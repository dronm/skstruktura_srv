package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/dronm/ds/v4"
	"github.com/dronm/skstruktura/internal/apperrors"
	"github.com/dronm/skstruktura/internal/maxbot"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
	wmodels "github.com/dronm/webapp/models"
)

const maxAuthRequestTokenBytes = 32

func (s *UserService) RequestMaxLogin(
	ctx context.Context,
	input models.UserMaxAuthRequest,
) (models.UserMaxAuthRequestResponse, error) {
	if s.Session == nil {
		return models.UserMaxAuthRequestResponse{}, apperrors.SessionRequired()
	}
	if err := s.requireDB(); err != nil {
		return models.UserMaxAuthRequestResponse{}, err
	}
	if !s.MaxCfg.Configured() {
		return models.UserMaxAuthRequestResponse{}, webapp.BadRequest("MAX authentication is not configured", nil)
	}

	phone, err := maxbot.NormalizeRussianPhone(input.Phone)
	if err != nil {
		return models.UserMaxAuthRequestResponse{}, webapp.BadRequest("invalid phone", map[string]any{
			"field": "phone",
		})
	}
	ttl, err := s.MaxCfg.AuthRequestTTLDuration()
	if err != nil {
		return models.UserMaxAuthRequestResponse{}, fmt.Errorf("MAX authentication request TTL: %w", err)
	}
	requestID, err := newMaxAuthRequestID()
	if err != nil {
		return models.UserMaxAuthRequestResponse{}, fmt.Errorf("generate MAX authentication request id: %w", err)
	}
	expiresAt := time.Now().UTC().Add(ttl)
	sessionID := s.Session.SessionID()
	if sessionID == "" {
		return models.UserMaxAuthRequestResponse{}, apperrors.SessionRequired()
	}

	message, err := maxbot.BuildMaxAuthRequestMessage(requestID)
	if err != nil {
		return models.UserMaxAuthRequestResponse{}, err
	}

	if err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		if _, err := tx.Exec(ctx, `
			SELECT pg_advisory_xact_lock(hashtext('max_auth'), hashtext($1))
		`, sessionID); err != nil {
			return fmt.Errorf("lock MAX authentication session: %w", err)
		}

		var existingRequestID string
		var existingExpiresAt time.Time
		existingErr := tx.QueryRow(ctx, `
			SELECT request_id, expires_at
			FROM public.max_auth_requests
			WHERE session_id = $1
				AND phone = $2
				AND max_user_id IS NOT NULL
				AND status IN ('pending', 'approved')
				AND expires_at > now()
			ORDER BY created_at DESC
			LIMIT 1
		`, sessionID, phone).Scan(&existingRequestID, &existingExpiresAt)
		if existingErr == nil {
			requestID = existingRequestID
			expiresAt = existingExpiresAt
			return nil
		}
		if !errors.Is(existingErr, ds.ErrNoRows) {
			return fmt.Errorf("read existing MAX authentication request: %w", existingErr)
		}

		if _, err := tx.Exec(ctx, `
			UPDATE public.max_auth_requests
			SET status = 'expired'
			WHERE session_id = $1
				AND status IN ('pending', 'approved')
		`, sessionID); err != nil {
			return fmt.Errorf("expire previous MAX authentication requests: %w", err)
		}

		var contactID int
		var maxUserID int64
		var userID int
		candidateErr := tx.QueryRow(ctx, `
			WITH candidates AS (
				SELECT DISTINCT
					contact.id AS contact_id,
					max_user.max_user_id,
					app_user.id AS user_id
				FROM public.contacts AS contact
				JOIN public.max_users AS max_user
					ON max_user.contact_id = contact.id
					AND max_user.is_active
				JOIN public.entity_contacts AS entity_contact
					ON entity_contact.contact_id = contact.id
					AND entity_contact.entity_type = 'users'
					AND entity_contact.is_active
				JOIN public.users AS app_user
					ON app_user.id = entity_contact.entity_id
				WHERE contact.phone = $1
					AND contact.is_active
					AND NOT COALESCE(app_user.banned, false)
			), resolved AS (
				SELECT
					min(contact_id) AS contact_id,
					min(max_user_id) AS max_user_id,
					min(user_id) AS user_id
				FROM candidates
				HAVING count(*) = 1
			)
			SELECT contact_id, max_user_id, user_id
			FROM resolved
		`, phone).Scan(&contactID, &maxUserID, &userID)

		resolved := candidateErr == nil
		if candidateErr != nil && !errors.Is(candidateErr, ds.ErrNoRows) {
			return fmt.Errorf("resolve MAX authentication target: %w", candidateErr)
		}

		var contactValue any
		var maxUserValue any
		var userValue any
		if resolved {
			contactValue = contactID
			maxUserValue = maxUserID
			userValue = userID
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO public.max_auth_requests (
				request_id,
				session_id,
				phone,
				contact_id,
				max_user_id,
				user_id,
				expires_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, requestID, sessionID, phone, contactValue, maxUserValue, userValue, expiresAt); err != nil {
			return fmt.Errorf("insert MAX authentication request: %w", err)
		}

		if !resolved {
			return nil
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO public.max_out_messages (
				max_user_id,
				message,
				metadata
			)
			VALUES (
				$1,
				$2::jsonb,
				jsonb_build_object(
					'source', 'max_auth',
					'request_id', $3::text
				)
			)
		`, maxUserID, string(message), requestID); err != nil {
			return fmt.Errorf("queue MAX authentication request: %w", err)
		}
		return nil
	}); err != nil {
		return models.UserMaxAuthRequestResponse{}, err
	}

	return models.UserMaxAuthRequestResponse{
		RequestID: requestID,
		Status:    models.UserMaxAuthStatusPending,
		ExpiresAt: expiresAt,
	}, nil
}

func (s *UserService) CompleteMaxLogin(
	ctx context.Context,
	input models.UserMaxAuthCompleteInput,
) (models.UserMaxAuthCompleteResponse, error) {
	if s.Session == nil {
		return models.UserMaxAuthCompleteResponse{}, apperrors.SessionRequired()
	}
	if err := s.requireDB(); err != nil {
		return models.UserMaxAuthCompleteResponse{}, err
	}
	requestID := input.Model.RequestID
	if !isMaxAuthRequestID(requestID) {
		return models.UserMaxAuthCompleteResponse{}, webapp.BadRequest("invalid MAX authentication request id", map[string]any{
			"field": "request_id",
		})
	}
	sessionID := s.Session.SessionID()
	if sessionID == "" {
		return models.UserMaxAuthCompleteResponse{}, apperrors.SessionRequired()
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return models.UserMaxAuthCompleteResponse{}, err
	}
	defer s.DB.Release(poolConn, connID)
	conn := poolConn.Conn()

	if _, err := conn.Exec(ctx, `
		SELECT pg_advisory_lock(hashtext('max_auth'), hashtext($1))
	`, sessionID); err != nil {
		return models.UserMaxAuthCompleteResponse{}, fmt.Errorf("lock MAX authentication session: %w", err)
	}
	defer func() {
		if _, unlockErr := conn.Exec(context.Background(), `
			SELECT pg_advisory_unlock(hashtext('max_auth'), hashtext($1))
		`, sessionID); unlockErr != nil {
			slog.Error("unlock MAX authentication session", "session_id", sessionID, "error", unlockErr)
		}
	}()

	if _, err := conn.Exec(ctx, `
		UPDATE public.max_auth_requests
		SET status = 'expired'
		WHERE request_id = $1
			AND session_id = $2
			AND status = 'pending'
			AND expires_at <= now()
	`, requestID, sessionID); err != nil {
		return models.UserMaxAuthCompleteResponse{}, fmt.Errorf("expire MAX authentication request: %w", err)
	}

	var status string
	var userID *int
	err = conn.QueryRow(ctx, `
		SELECT status, user_id
		FROM public.max_auth_requests
		WHERE request_id = $1
			AND session_id = $2
	`, requestID, sessionID).Scan(&status, &userID)
	if errors.Is(err, ds.ErrNoRows) {
		return maxAuthStatusResponse(models.UserMaxAuthStatusExpired), nil
	}
	if err != nil {
		return models.UserMaxAuthCompleteResponse{}, fmt.Errorf("read MAX authentication request: %w", err)
	}

	switch status {
	case models.UserMaxAuthStatusPending,
		models.UserMaxAuthStatusDeclined,
		models.UserMaxAuthStatusExpired:
		return maxAuthStatusResponse(status), nil
	case models.UserMaxAuthStatusConsumed:
		if userID != nil {
			var sessionUser models.UserLogin
			if err := s.Session.Get("user", &sessionUser); err == nil && sessionUser.ID == *userID {
				return s.maxAuthenticatedResponse(&sessionUser), nil
			}
		}
		return maxAuthStatusResponse(models.UserMaxAuthStatusConsumed), nil
	case models.UserMaxAuthStatusApproved:
		// Continue below.
	default:
		return models.UserMaxAuthCompleteResponse{}, fmt.Errorf("unsupported MAX authentication status %q", status)
	}

	userRow := models.UserLogin{}
	err = conn.QueryRow(ctx, `
		SELECT
			app_user.id,
			app_user.name,
			app_user.role_id,
			app_user.create_dt,
			COALESCE(app_user.pwd, ''),
			COALESCE(app_user.banned, false),
			(
				SELECT string_agg(ban.hash, ',')
				FROM public.login_device_bans AS ban
				WHERE ban.user_id = app_user.id
			) AS ban_hash
		FROM public.max_auth_requests AS request
		JOIN public.users AS app_user
			ON app_user.id = request.user_id
		JOIN public.max_users AS max_user
			ON max_user.max_user_id = request.max_user_id
			AND max_user.contact_id = request.contact_id
			AND max_user.is_active
		JOIN public.entity_contacts AS entity_contact
			ON entity_contact.contact_id = request.contact_id
			AND entity_contact.entity_type = 'users'
			AND entity_contact.entity_id = request.user_id
			AND entity_contact.is_active
		WHERE request.request_id = $1
			AND request.session_id = $2
			AND request.status = 'approved'
			AND request.expires_at > now()
		LIMIT 1
	`, requestID, sessionID).Scan(
		&userRow.ID,
		&userRow.Name,
		&userRow.RoleID,
		&userRow.CreateDt,
		&userRow.Pwd,
		&userRow.Banned,
		&userRow.BanHash,
	)
	if errors.Is(err, ds.ErrNoRows) {
		if _, updateErr := conn.Exec(ctx, `
			UPDATE public.max_auth_requests
			SET status = 'expired'
			WHERE request_id = $1
				AND session_id = $2
				AND status = 'approved'
		`, requestID, sessionID); updateErr != nil {
			slog.Error("invalidate MAX authentication request", "request_id", requestID, "error", updateErr)
		}
		return maxAuthStatusResponse(models.UserMaxAuthStatusExpired), nil
	}
	if err != nil {
		return models.UserMaxAuthCompleteResponse{}, fmt.Errorf("load MAX authentication user: %w", err)
	}

	if err := loginUser(ctx, s.Session, input.UserInf, conn, &userRow); err != nil {
		return models.UserMaxAuthCompleteResponse{}, err
	}

	if _, err := conn.Exec(ctx, `
		UPDATE public.max_auth_requests
		SET
			status = 'consumed',
			consumed_at = now()
		WHERE request_id = $1
			AND session_id = $2
			AND status = 'approved'
	`, requestID, sessionID); err != nil {
		return models.UserMaxAuthCompleteResponse{}, fmt.Errorf("consume MAX authentication request: %w", err)
	}

	return s.maxAuthenticatedResponse(&userRow), nil
}

func (s *UserService) maxAuthenticatedResponse(user *models.UserLogin) models.UserMaxAuthCompleteResponse {
	tokenExpires := time.Time{}
	if s.SessCfg.MaxLifeTime > 0 {
		tokenExpires = time.Now().Add(time.Duration(s.SessCfg.MaxLifeTime) * time.Second)
	}
	return models.UserMaxAuthCompleteResponse{
		Status: models.UserMaxAuthStatusAuthenticated,
		User:   user,
		Auth: &wmodels.Auth{
			Token:        s.Session.SessionID(),
			TokenRefresh: "",
			Expires:      tokenExpires,
		},
	}
}

func maxAuthStatusResponse(status string) models.UserMaxAuthCompleteResponse {
	return models.UserMaxAuthCompleteResponse{Status: status}
}

func newMaxAuthRequestID() (string, error) {
	buffer := make([]byte, maxAuthRequestTokenBytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func isMaxAuthRequestID(value string) bool {
	if len(value) != maxAuthRequestTokenBytes*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
