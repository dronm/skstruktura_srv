package diadoc

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/dronm/skstruktura/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	authorizationStateLifetime = 10 * time.Minute
	tokenRefreshAdvance        = time.Minute
	dotNetUnixEpochTicks       = int64(621355968000000000)
)

var (
	ErrNotAuthorized  = errors.New("Diadoc integration is not authorized")
	ErrBoxNotSelected = errors.New("Diadoc box is not selected")
	ErrSyncDisabled   = errors.New("Diadoc synchronization is disabled")
)

type Manager struct {
	cfg          config.DiadocConfig
	client       *Client
	store        *Store
	logger       *slog.Logger
	pollInterval time.Duration

	stateMu        sync.Mutex
	state          State
	authState      string
	authStateUntil time.Time

	pollMu sync.Mutex
	wake   chan struct{}
	wg     sync.WaitGroup
}

type BoxChoice struct {
	BoxID    string `json:"box_id"`
	Title    string `json:"title"`
	INN      string `json:"inn"`
	KPP      string `json:"kpp"`
	IsActive bool   `json:"is_active"`
	IsTest   bool   `json:"is_test"`
}

type AuthorizationResult struct {
	Authorized           bool        `json:"authorized"`
	BoxID                string      `json:"box_id,omitempty"`
	RequiresBoxSelection bool        `json:"requires_box_selection"`
	Boxes                []BoxChoice `json:"boxes,omitempty"`
}

type Status struct {
	Configured              bool      `json:"configured"`
	Authorized              bool      `json:"authorized"`
	BoxID                   string    `json:"box_id,omitempty"`
	AccessTokenExpiresAt    time.Time `json:"access_token_expires_at,omitempty"`
	AfterIndexKey           string    `json:"after_index_key,omitempty"`
	EventTimestampFromTicks int64     `json:"event_timestamp_from_ticks"`
}

type SyncResult struct {
	EventsProcessed  int  `json:"events_processed"`
	DocumentsAdded   int  `json:"documents_added"`
	DocumentsUpdated int  `json:"documents_updated"`
	DocumentsFailed  int  `json:"documents_failed"`
	HasMore          bool `json:"has_more"`
}

func NewManager(
	ctx context.Context,
	cfg config.DiadocConfig,
	pool *pgxpool.Pool,
	logger *slog.Logger,
) (*Manager, error) {
	pollInterval, err := cfg.PollIntervalDuration()
	if err != nil {
		return nil, err
	}
	requestTimeout, err := cfg.RequestTimeoutDuration()
	if err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}

	store := NewStore(pool)
	state, err := store.Load(ctx)
	if err != nil {
		return nil, err
	}
	changed := false
	if cfg.BoxID != "" && cfg.BoxID != state.BoxID {
		state.BoxID = cfg.BoxID
		state.AfterIndexKey = ""
		state.EventTimestampFromTicks = 0
		changed = true
	}
	if state.EventTimestampFromTicks == 0 {
		from := time.Now().UTC()
		if cfg.EventTimestampFrom != "" {
			from, err = time.Parse(time.RFC3339, cfg.EventTimestampFrom)
			if err != nil {
				return nil, fmt.Errorf("parse Diadoc event start timestamp: %w", err)
			}
		}
		state.EventTimestampFromTicks = dotNetTicks(from)
		state.EventTimestampFrom = from
		changed = true
	}
	if state.EventTimestampFrom.IsZero() {
		state.EventTimestampFrom = time.Now().UTC()
		changed = true
	}
	if changed {
		if err := store.Save(ctx, state); err != nil {
			return nil, err
		}
	}

	return &Manager{
		cfg:          cfg,
		client:       NewClient(cfg, requestTimeout),
		store:        store,
		logger:       logger,
		pollInterval: pollInterval,
		state:        state,
		wake:         make(chan struct{}, 1),
	}, nil
}

func (m *Manager) Start(ctx context.Context) {
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.run(ctx)
	}()
}

func (m *Manager) Wait() {
	m.wg.Wait()
}

func (m *Manager) PollNow() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

func (m *Manager) LockSynchronization() func() {
	m.pollMu.Lock()
	return m.pollMu.Unlock
}

func (m *Manager) AuthorizationURL() (string, error) {
	state, err := randomURLToken(32)
	if err != nil {
		return "", fmt.Errorf("generate Diadoc authorization state: %w", err)
	}
	nonce, err := randomURLToken(32)
	if err != nil {
		return "", fmt.Errorf("generate Diadoc authorization nonce: %w", err)
	}

	m.stateMu.Lock()
	m.authState = state
	m.authStateUntil = time.Now().Add(authorizationStateLifetime)
	m.stateMu.Unlock()

	query := url.Values{
		"response_type": {"code"},
		"client_id":     {m.cfg.ClientID},
		"scope":         {m.cfg.ScopeValue()},
		"redirect_uri":  {m.cfg.RedirectURL},
		"state":         {state},
		"nonce":         {nonce},
	}
	return normalizedEntryPoint(m.cfg.IdentityEntryPoint) + "/connect/authorize?" + query.Encode(), nil
}

func (m *Manager) CompleteAuthorization(
	ctx context.Context,
	state string,
	code string,
) (AuthorizationResult, error) {
	if code == "" {
		return AuthorizationResult{}, fmt.Errorf("authorization code is required")
	}

	m.stateMu.Lock()
	stateValid := state != "" &&
		state == m.authState &&
		time.Now().Before(m.authStateUntil)
	m.authState = ""
	m.authStateUntil = time.Time{}
	m.stateMu.Unlock()
	if !stateValid {
		return AuthorizationResult{}, fmt.Errorf("Diadoc authorization state is invalid or expired")
	}

	token, err := m.client.ExchangeAuthorizationCode(ctx, code)
	if err != nil {
		return AuthorizationResult{}, err
	}
	if err := m.saveToken(ctx, token); err != nil {
		return AuthorizationResult{}, err
	}

	organizations, err := m.client.GetMyOrganizations(ctx, token.AccessToken)
	if err != nil {
		return AuthorizationResult{}, err
	}
	choices := organizationBoxChoices(organizations)
	boxID, found := selectBox(m.configuredBoxID(), choices)
	if !found {
		return AuthorizationResult{
			Authorized:           true,
			RequiresBoxSelection: true,
			Boxes:                choices,
		}, nil
	}
	if err := m.setBoxID(ctx, boxID); err != nil {
		return AuthorizationResult{}, err
	}

	m.logger.Info("Diadoc authorization completed", "box_id", boxID)
	m.PollNow()
	return AuthorizationResult{
		Authorized: true,
		BoxID:      boxID,
		Boxes:      choices,
	}, nil
}

func (m *Manager) Status() Status {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	return Status{
		Configured:              m.cfg.Configured(),
		Authorized:              m.state.RefreshToken != "" || validAccessToken(m.state),
		BoxID:                   m.state.BoxID,
		AccessTokenExpiresAt:    m.state.AccessTokenExpiresAt,
		AfterIndexKey:           m.state.AfterIndexKey,
		EventTimestampFromTicks: m.state.EventTimestampFromTicks,
	}
}

func (m *Manager) run(ctx context.Context) {
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	if err := m.RunOnce(ctx); err != nil &&
		!errors.Is(err, ErrNotAuthorized) &&
		!errors.Is(err, ErrSyncDisabled) {
		m.logger.Error("initial Diadoc synchronization failed", "error", err)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-m.wake:
		}
		if err := m.RunOnce(ctx); err != nil {
			if errors.Is(err, ErrNotAuthorized) ||
				errors.Is(err, ErrBoxNotSelected) ||
				errors.Is(err, ErrSyncDisabled) {
				m.logger.Debug("Diadoc synchronization is waiting", "reason", err)
				continue
			}
			m.logger.Error("Diadoc synchronization failed", "error", err)
		}
	}
}

func (m *Manager) RunOnce(ctx context.Context) error {
	_, err := m.Sync(ctx, 0)
	return err
}

func (m *Manager) Sync(ctx context.Context, maxPages int) (result SyncResult, syncErr error) {
	m.pollMu.Lock()
	defer m.pollMu.Unlock()
	if err := m.store.MarkSyncStarted(ctx); err != nil {
		return SyncResult{}, err
	}
	defer func() {
		if err := m.store.MarkSyncFinished(context.Background(), syncErr); err != nil {
			m.logger.Error("record Diadoc synchronization result", "error", err)
		}
		if state, err := m.store.Load(context.Background()); err == nil {
			m.stateMu.Lock()
			m.state.LastSyncStartedAt = state.LastSyncStartedAt
			m.state.LastSyncFinishedAt = state.LastSyncFinishedAt
			m.state.LastSyncError = state.LastSyncError
			m.stateMu.Unlock()
		}
	}()

	accessToken, err := m.accessToken(ctx)
	if err != nil {
		return SyncResult{}, err
	}
	state := m.stateSnapshot()
	if !state.Enabled {
		return SyncResult{}, ErrSyncDisabled
	}
	if state.BoxID == "" {
		return SyncResult{}, ErrBoxNotSelected
	}

	pages := 0
	for {
		if maxPages > 0 && pages >= maxPages {
			result.HasMore = true
			return result, nil
		}
		response, err := m.client.GetNewEvents(ctx, accessToken, NewEventsRequest{
			BoxID:              state.BoxID,
			AfterIndexKey:      state.AfterIndexKey,
			TimestampFromTicks: state.EventTimestampFromTicks,
			Limit:              m.cfg.EventLimit,
		})
		if err != nil {
			return result, err
		}
		pages++
		m.logger.Info(
			"Diadoc events page retrieved",
			"box_id", state.BoxID,
			"count", len(response.Events),
			"total_count", response.TotalCount,
			"total_count_type", response.TotalCountType,
		)
		if len(response.Events) == 0 {
			return result, nil
		}

		for _, event := range response.Events {
			eventResult, err := m.processEvent(ctx, accessToken, state.BoxID, event)
			if err != nil {
				return result, err
			}
			result.EventsProcessed++
			result.DocumentsAdded += eventResult.DocumentsAdded
			result.DocumentsUpdated += eventResult.DocumentsUpdated
			result.DocumentsFailed += eventResult.DocumentsFailed
			if event.IndexKey == "" {
				return result, fmt.Errorf("Diadoc event %q has no index key", event.EventID)
			}
			if err := m.setAfterIndexKey(ctx, event.IndexKey); err != nil {
				return result, err
			}
			state.AfterIndexKey = event.IndexKey
		}
		if len(response.Events) < m.cfg.EventLimit {
			return result, nil
		}
	}
}

func (m *Manager) processEvent(
	ctx context.Context,
	accessToken string,
	boxID string,
	event BoxEvent,
) (SyncResult, error) {
	result := SyncResult{}
	if event.Message == nil {
		m.logger.Info(
			"Diadoc event retrieved",
			"event_id", event.EventID,
			"index_key", event.IndexKey,
			"has_patch", len(event.Patch) > 0,
		)
		return result, nil
	}

	message := event.Message
	m.logger.Info(
		"Diadoc message retrieved",
		"event_id", event.EventID,
		"index_key", event.IndexKey,
		"message_id", message.MessageID,
		"from_box_id", message.FromBoxID,
		"from_title", message.FromTitle,
		"to_box_id", message.ToBoxID,
		"entity_count", len(message.Entities),
	)
	for _, entity := range message.Entities {
		if entity.EntityType != "Attachment" || entity.AttachmentType != "UniversalTransferDocument" {
			continue
		}
		attributes := []any{
			"message_id", message.MessageID,
			"entity_id", entity.EntityID,
			"attachment_type", entity.AttachmentType,
			"file_name", entity.FileName,
			"declared_content_size", entity.Content.Size,
		}
		if entity.DocumentInfo != nil {
			attributes = append(attributes,
				"document_number", entity.DocumentInfo.DocumentNumber,
				"document_date", entity.DocumentInfo.DocumentDate,
				"document_function", entity.DocumentInfo.Function,
				"document_version", entity.DocumentInfo.Version,
				"sender_signature_status", entity.DocumentInfo.SenderSignatureStatus,
			)
		}
		m.logger.Info("Diadoc document metadata retrieved", attributes...)

		input := stageInputFromEvent(boxID, event, *message, entity)
		content, err := m.client.GetEntityContent(
			ctx,
			accessToken,
			boxID,
			message.MessageID,
			entity.EntityID,
		)
		if err != nil {
			input.StageError = fmt.Errorf(
				"get content of Diadoc message %s entity %s: %w",
				message.MessageID,
				entity.EntityID,
				err,
			)
			staged, stageErr := m.store.StageDocument(ctx, input)
			if stageErr != nil {
				return result, stageErr
			}
			applyStageResult(&result, staged)
			continue
		}
		input.Content = content
		m.logger.Info(
			"Diadoc document content retrieved",
			"message_id", message.MessageID,
			"entity_id", entity.EntityID,
			"content_size", len(content),
		)
		if m.cfg.LogDocumentContent {
			loggedContent, truncated := limitedContent(content, m.cfg.MaxLoggedContentSize)
			m.logger.Debug(
				"Diadoc document XML",
				"message_id", message.MessageID,
				"entity_id", entity.EntityID,
				"truncated", truncated,
				"content", string(loggedContent),
			)
		}
		parsed, parseErr := ParseUTD(content)
		if parseErr != nil {
			input.StageError = parseErr
		} else {
			input.Parsed = &parsed
		}
		staged, err := m.store.StageDocument(ctx, input)
		if err != nil {
			return result, err
		}
		applyStageResult(&result, staged)
	}
	return result, nil
}

func (m *Manager) RetryDocument(ctx context.Context, id int64, version int64) (StageDocumentResult, error) {
	m.pollMu.Lock()
	defer m.pollMu.Unlock()

	source, err := m.store.BufferedDocumentSource(ctx, id)
	if err != nil {
		return StageDocumentResult{}, err
	}
	if source.Version != version {
		return StageDocumentResult{}, fmt.Errorf("buffered Diadoc document version changed")
	}
	if source.Status != "failed" {
		return StageDocumentResult{}, fmt.Errorf("only failed Diadoc documents can be retried")
	}
	accessToken, err := m.accessToken(ctx)
	if err != nil {
		return StageDocumentResult{}, err
	}
	content, err := m.client.GetEntityContent(
		ctx,
		accessToken,
		source.BoxID,
		source.MessageID,
		source.EntityID,
	)
	input := StageDocumentInput{
		BoxID:            source.BoxID,
		MessageID:        source.MessageID,
		EntityID:         source.EntityID,
		ParentEntityID:   source.ParentEntityID,
		EventID:          source.EventID,
		EventIndexKey:    source.EventIndexKey,
		EventTimestamp:   source.EventTimestamp,
		AttachmentType:   source.AttachmentType,
		TypeNamedID:      source.TypeNamedID,
		DocumentFunction: source.DocumentFunction,
		DocumentVersion:  source.DocumentVersion,
		DocumentNumber:   source.DocumentNumber,
		DocumentDate:     source.DocumentDate,
		FileName:         source.FileName,
		SenderBoxID:      source.SenderBoxID,
		SenderName:       source.SenderName,
		SenderINN:        source.SenderINN,
		SenderKPP:        source.SenderKPP,
		Content:          content,
		StageError:       err,
	}
	if err == nil {
		parsed, parseErr := ParseUTD(content)
		if parseErr != nil {
			input.StageError = parseErr
		} else {
			input.Parsed = &parsed
		}
	}
	return m.store.StageDocument(ctx, input)
}

func (m *Manager) ReloadState(ctx context.Context) error {
	state, err := m.store.Load(ctx)
	if err != nil {
		return err
	}
	m.stateMu.Lock()
	m.state = state
	m.stateMu.Unlock()
	return nil
}

func (m *Manager) accessToken(ctx context.Context) (string, error) {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	if validAccessToken(m.state) {
		return m.state.AccessToken, nil
	}
	if m.state.RefreshToken == "" {
		return "", ErrNotAuthorized
	}

	result, err := m.client.RefreshToken(ctx, m.state.RefreshToken)
	if err != nil {
		return "", fmt.Errorf("refresh Diadoc access token: %w", err)
	}
	if result.RefreshToken == "" {
		result.RefreshToken = m.state.RefreshToken
	}
	m.applyToken(result)
	if err := m.store.Save(ctx, m.state); err != nil {
		return "", err
	}
	m.logger.Info("Diadoc access token refreshed", "expires_at", m.state.AccessTokenExpiresAt)
	return m.state.AccessToken, nil
}

func (m *Manager) saveToken(ctx context.Context, token TokenResponse) error {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	if token.RefreshToken == "" {
		token.RefreshToken = m.state.RefreshToken
	}
	m.applyToken(token)
	return m.store.Save(ctx, m.state)
}

func (m *Manager) applyToken(token TokenResponse) {
	m.state.AccessToken = token.AccessToken
	m.state.RefreshToken = token.RefreshToken
	m.state.AccessTokenExpiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
}

func (m *Manager) setBoxID(ctx context.Context, boxID string) error {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	if m.state.BoxID != "" && m.state.BoxID != boxID {
		m.state.AfterIndexKey = ""
	}
	m.state.BoxID = boxID
	return m.store.Save(ctx, m.state)
}

func (m *Manager) setAfterIndexKey(ctx context.Context, indexKey string) error {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	m.state.AfterIndexKey = indexKey
	return m.store.Save(ctx, m.state)
}

func (m *Manager) stateSnapshot() State {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	return m.state
}

func (m *Manager) configuredBoxID() string {
	if m.cfg.BoxID != "" {
		return m.cfg.BoxID
	}
	return m.stateSnapshot().BoxID
}

func validAccessToken(state State) bool {
	return state.AccessToken != "" && time.Now().Add(tokenRefreshAdvance).Before(state.AccessTokenExpiresAt)
}

func organizationBoxChoices(list OrganizationList) []BoxChoice {
	result := make([]BoxChoice, 0)
	for _, organization := range list.Organizations {
		for _, box := range organization.Boxes {
			result = append(result, BoxChoice{
				BoxID:    box.BoxID,
				Title:    firstNonEmpty(box.Title, organization.ShortName, organization.FullName),
				INN:      organization.INN,
				KPP:      organization.KPP,
				IsActive: organization.IsActive,
				IsTest:   organization.IsTest,
			})
		}
	}
	return result
}

func selectBox(configuredBoxID string, choices []BoxChoice) (string, bool) {
	if configuredBoxID != "" {
		for _, choice := range choices {
			if sameBoxID(configuredBoxID, choice.BoxID) {
				return choice.BoxID, true
			}
		}
		return "", false
	}
	if len(choices) == 1 {
		return choices[0].BoxID, true
	}
	return "", false
}

func sameBoxID(left string, right string) bool {
	normalize := func(value string) string {
		value = strings.ToLower(strings.TrimSpace(value))
		value = strings.ReplaceAll(value, "-", "")
		value = strings.TrimSuffix(value, "@diadoc.ru")
		return value
	}
	return normalize(left) == normalize(right)
}

func randomURLToken(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func dotNetTicks(value time.Time) int64 {
	value = value.UTC()
	return dotNetUnixEpochTicks + value.Unix()*10_000_000 + int64(value.Nanosecond()/100)
}

func limitedContent(content []byte, maximum int64) ([]byte, bool) {
	if maximum <= 0 {
		return nil, len(content) > 0
	}
	if int64(len(content)) <= maximum {
		return content, false
	}
	return content[:maximum], true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
