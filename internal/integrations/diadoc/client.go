package diadoc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dronm/skstruktura/internal/config"
)

const maxErrorBodySize = 64 * 1024

type Client struct {
	apiEntryPoint      string
	identityEntryPoint string
	clientID           string
	clientSecret       string
	redirectURL        string
	httpClient         *http.Client
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	Scope        string `json:"scope"`
}

type OrganizationList struct {
	Organizations []Organization `json:"Organizations"`
}

type Organization struct {
	OrgID     string `json:"OrgId"`
	INN       string `json:"Inn"`
	KPP       string `json:"Kpp"`
	FullName  string `json:"FullName"`
	ShortName string `json:"ShortName"`
	IsActive  bool   `json:"IsActive"`
	IsTest    bool   `json:"IsTest"`
	Boxes     []Box  `json:"Boxes"`
}

type Box struct {
	BoxID     string `json:"BoxId"`
	BoxIDGUID string `json:"BoxIdGuid"`
	Title     string `json:"Title"`
}

type NewEventsResponse struct {
	Events         []BoxEvent `json:"Events"`
	TotalCount     int        `json:"TotalCount"`
	TotalCountType string     `json:"TotalCountType"`
}

type BoxEvent struct {
	EventID  string          `json:"EventId"`
	IndexKey string          `json:"IndexKey"`
	Message  *Message        `json:"Message"`
	Patch    json.RawMessage `json:"Patch"`
}

type Message struct {
	MessageID          string   `json:"MessageId"`
	TimestampTicks     int64    `json:"TimestampTicks"`
	FromBoxID          string   `json:"FromBoxId"`
	FromTitle          string   `json:"FromTitle"`
	ToBoxID            string   `json:"ToBoxId"`
	ToTitle            string   `json:"ToTitle"`
	MessageType        string   `json:"MessageType"`
	MessageIsDeleted   bool     `json:"MessageIsDeleted"`
	MessageIsDelivered bool     `json:"MessageIsDelivered"`
	Entities           []Entity `json:"Entities"`
}

type Entity struct {
	EntityType      string        `json:"EntityType"`
	EntityID        string        `json:"EntityId"`
	ParentEntityID  string        `json:"ParentEntityId"`
	AttachmentType  string        `json:"AttachmentType"`
	FileName        string        `json:"FileName"`
	DocumentInfo    *DocumentInfo `json:"DocumentInfo"`
	Content         EntityContent `json:"Content"`
	NeedRecipientSignature bool          `json:"NeedRecipientSignature"`
}

type EntityContent struct {
	Size int64 `json:"Size"`
}

type DocumentInfo struct {
	MessageID            string `json:"MessageId"`
	EntityID             string `json:"EntityId"`
	CounteragentBoxID    string `json:"CounteragentBoxId"`
	DocumentType         string `json:"DocumentType"`
	TypeNamedID          string `json:"TypeNamedId"`
	Function             string `json:"Function"`
	Version              string `json:"Version"`
	DocumentDate         string `json:"DocumentDate"`
	DocumentNumber       string `json:"DocumentNumber"`
	DocumentDirection    string `json:"DocumentDirection"`
	SenderSignatureStatus string `json:"SenderSignatureStatus"`
	IsDeleted            bool   `json:"IsDeleted"`
	IsTest               bool   `json:"IsTest"`
}

type NewEventsRequest struct {
	BoxID              string
	AfterIndexKey      string
	TimestampFromTicks int64
	Limit              int
}

func NewClient(cfg config.DiadocConfig, requestTimeout time.Duration) *Client {
	return &Client{
		apiEntryPoint:      normalizedEntryPoint(cfg.EntryPoint),
		identityEntryPoint: normalizedEntryPoint(cfg.IdentityEntryPoint),
		clientID:           cfg.ClientID,
		clientSecret:       cfg.ClientSecret,
		redirectURL:        cfg.RedirectURL,
		httpClient: &http.Client{
			Timeout: requestTimeout,
		},
	}
}

func (c *Client) ExchangeAuthorizationCode(
	ctx context.Context,
	code string,
) (TokenResponse, error) {
	values := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"redirect_uri":  {c.redirectURL},
	}
	return c.requestToken(ctx, values)
}

func (c *Client) RefreshToken(ctx context.Context, refreshToken string) (TokenResponse, error) {
	values := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"refresh_token": {refreshToken},
	}
	return c.requestToken(ctx, values)
}

func (c *Client) requestToken(ctx context.Context, values url.Values) (TokenResponse, error) {
	endpoint := c.identityEntryPoint + "/connect/token"
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint,
		strings.NewReader(values.Encode()),
	)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("create Diadoc token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	var result TokenResponse
	if err := c.doJSON(req, &result); err != nil {
		return TokenResponse{}, fmt.Errorf("request Diadoc token: %w", err)
	}
	if result.AccessToken == "" || result.ExpiresIn <= 0 {
		return TokenResponse{}, fmt.Errorf("Diadoc token response is incomplete")
	}
	return result, nil
}

func (c *Client) GetMyOrganizations(
	ctx context.Context,
	accessToken string,
) (OrganizationList, error) {
	req, err := c.authorizedRequest(
		ctx,
		http.MethodGet,
		c.apiEntryPoint+"/GetMyOrganizations?autoRegister=false",
		accessToken,
		nil,
	)
	if err != nil {
		return OrganizationList{}, err
	}

	var result OrganizationList
	if err := c.doJSON(req, &result); err != nil {
		return OrganizationList{}, fmt.Errorf("get Diadoc organizations: %w", err)
	}
	return result, nil
}

func (c *Client) GetNewEvents(
	ctx context.Context,
	accessToken string,
	input NewEventsRequest,
) (NewEventsResponse, error) {
	query := url.Values{
		"boxId":             {input.BoxID},
		"messageType":       {"Letter"},
		"typeNamedId":       {"UniversalTransferDocument"},
		"documentDirection": {"Inbound"},
		"orderBy":           {"Ascending"},
		"limit":             {fmt.Sprintf("%d", input.Limit)},
	}
	if input.AfterIndexKey != "" {
		query.Set("afterIndexKey", input.AfterIndexKey)
	}
	if input.TimestampFromTicks > 0 {
		query.Set("timestampFromTicks", fmt.Sprintf("%d", input.TimestampFromTicks))
	}

	req, err := c.authorizedRequest(
		ctx,
		http.MethodGet,
		c.apiEntryPoint+"/V8/GetNewEvents?"+query.Encode(),
		accessToken,
		nil,
	)
	if err != nil {
		return NewEventsResponse{}, err
	}

	var result NewEventsResponse
	if err := c.doJSON(req, &result); err != nil {
		return NewEventsResponse{}, fmt.Errorf("get new Diadoc events: %w", err)
	}
	return result, nil
}

func (c *Client) GetEntityContent(
	ctx context.Context,
	accessToken string,
	boxID string,
	messageID string,
	entityID string,
) ([]byte, error) {
	query := url.Values{
		"boxId":    {boxID},
		"messageId": {messageID},
		"entityId":  {entityID},
	}
	req, err := c.authorizedRequest(
		ctx,
		http.MethodGet,
		c.apiEntryPoint+"/V4/GetEntityContent?"+query.Encode(),
		accessToken,
		nil,
	)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute Diadoc entity content request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, responseError(resp)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read Diadoc entity content: %w", err)
	}
	return body, nil
}

func (c *Client) authorizedRequest(
	ctx context.Context,
	method string,
	requestURL string,
	accessToken string,
	body []byte,
) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, requestURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create Diadoc API request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json; charset=utf-8")
	return req, nil
}

func (c *Client) doJSON(req *http.Request, result any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return responseError(resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func responseError(resp *http.Response) error {
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodySize))
	if err != nil {
		return fmt.Errorf("Diadoc returned %s and its response could not be read: %w", resp.Status, err)
	}
	message := strings.TrimSpace(string(body))
	if message == "" {
		message = http.StatusText(resp.StatusCode)
	}
	return fmt.Errorf("Diadoc returned %s: %s", resp.Status, message)
}

func normalizedEntryPoint(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}
