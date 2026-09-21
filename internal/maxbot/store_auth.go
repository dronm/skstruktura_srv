package maxbot

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func processMaxAuthCallback(ctx context.Context, tx pgx.Tx, update Update, actorMaxUserID int64) error {
	if update.Callback == nil {
		return nil
	}
	decision, requestID, ok := ParseMaxAuthCallbackPayload(update.Callback.Payload)
	if !ok {
		return nil
	}

	if _, err := tx.Exec(ctx, `
		UPDATE public.max_auth_requests
		SET status = 'expired'
		WHERE request_id = $1
			AND max_user_id = $2
			AND status = 'pending'
			AND expires_at <= now()
	`, requestID, actorMaxUserID); err != nil {
		return fmt.Errorf("expire MAX authentication request: %w", err)
	}

	if decision == MaxAuthDecline {
		if _, err := tx.Exec(ctx, `
			UPDATE public.max_auth_requests
			SET
				status = 'declined',
				decided_at = now()
			WHERE request_id = $1
				AND max_user_id = $2
				AND status = 'pending'
				AND expires_at > now()
		`, requestID, actorMaxUserID); err != nil {
			return fmt.Errorf("decline MAX authentication request: %w", err)
		}
		return nil
	}

	tag, err := tx.Exec(ctx, `
		UPDATE public.max_auth_requests AS request
		SET
			status = 'approved',
			decided_at = now()
		WHERE request.request_id = $1
			AND request.max_user_id = $2
			AND request.status = 'pending'
			AND request.expires_at > now()
			AND EXISTS (
				SELECT 1
				FROM public.max_users AS max_user
				WHERE max_user.max_user_id = request.max_user_id
					AND max_user.contact_id = request.contact_id
					AND max_user.is_active
			)
			AND EXISTS (
				SELECT 1
				FROM public.entity_contacts AS entity_contact
				WHERE entity_contact.contact_id = request.contact_id
					AND entity_contact.entity_type = 'users'
					AND entity_contact.entity_id = request.user_id
					AND entity_contact.is_active
			)
			AND EXISTS (
				SELECT 1
				FROM public.users AS app_user
				WHERE app_user.id = request.user_id
					AND NOT COALESCE(app_user.banned, false)
			)
	`, requestID, actorMaxUserID)
	if err != nil {
		return fmt.Errorf("approve MAX authentication request: %w", err)
	}
	if tag.RowsAffected() > 0 {
		return nil
	}

	// The callback came from the expected MAX account but the relationship used
	// to create the request is no longer valid. Do not leave such a request
	// pending: it cannot safely be approved anymore.
	if _, err := tx.Exec(ctx, `
		UPDATE public.max_auth_requests
		SET status = 'expired'
		WHERE request_id = $1
			AND max_user_id = $2
			AND status = 'pending'
	`, requestID, actorMaxUserID); err != nil {
		return fmt.Errorf("invalidate MAX authentication request: %w", err)
	}
	return nil
}

func (s *Store) CallbackAnswer(ctx context.Context, update Update) (*CallbackAnswer, error) {
	if s == nil || s.pool == nil || update.Callback == nil {
		return nil, nil
	}
	_, requestID, ok := ParseMaxAuthCallbackPayload(update.Callback.Payload)
	if !ok || update.Callback.CallbackID == "" {
		return nil, nil
	}
	actor := updateActor(update)
	if actor == nil || actor.UserID <= 0 {
		return nil, nil
	}

	var status string
	err := s.pool.QueryRow(ctx, `
		SELECT
			CASE
				WHEN status = 'pending' AND expires_at <= now() THEN 'expired'
				ELSE status
			END
		FROM public.max_auth_requests
		WHERE request_id = $1
			AND max_user_id = $2
	`, requestID, actor.UserID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		status = "expired"
	} else if err != nil {
		return nil, fmt.Errorf("read MAX authentication callback state: %w", err)
	}

	text := MaxAuthExpiredText
	switch status {
	case "approved":
		text = MaxAuthApprovedText
	case "declined":
		text = MaxAuthDeclinedText
	case "consumed":
		text = MaxAuthConsumedText
	}

	return &CallbackAnswer{
		CallbackID: update.Callback.CallbackID,
		Text:       text,
	}, nil
}
