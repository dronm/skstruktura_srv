package maxbot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool     *pgxpool.Pool
	botToken string
}

func NewStore(pool *pgxpool.Pool, botToken string) *Store {
	return &Store{
		pool:     pool,
		botToken: strings.TrimSpace(botToken),
	}
}

func (s *Store) SaveUpdate(ctx context.Context, raw json.RawMessage, update Update) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("MAX store is not initialized")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	maxUserID := updateMaxUserID(update)
	maxChatID := updateMaxChatID(update)
	if _, err := tx.Exec(ctx, `
		INSERT INTO public.max_in_messages (
			update_type,
			max_user_id,
			max_chat_id,
			message
		)
		VALUES ($1, $2, $3, $4::jsonb)
	`, update.UpdateType, maxUserID, maxChatID, string(raw)); err != nil {
		return fmt.Errorf("persist MAX incoming update: %w", err)
	}

	if update.UpdateType == "bot_stopped" {
		if user := updateActor(update); user != nil && user.UserID > 0 {
			if _, err := tx.Exec(ctx, `
				UPDATE public.max_users
				SET is_active = false
				WHERE max_user_id = $1
			`, user.UserID); err != nil {
				return fmt.Errorf("deactivate stopped MAX user: %w", err)
			}
		}
		return commitUpdate(ctx, tx)
	}

	actor := updateActor(update)
	if actor == nil || actor.UserID <= 0 {
		return commitUpdate(ctx, tx)
	}
	rawActor, err := extractRawActor(raw, update)
	if err != nil {
		return err
	}
	contactID, err := upsertMaxUser(ctx, tx, actor, rawActor)
	if err != nil {
		return err
	}

	switch update.UpdateType {
	case "bot_started":
		if contactID == nil {
			if err := queueContactRequest(ctx, tx, actor.UserID, ContactRequestText, update.UpdateType); err != nil {
				return err
			}
		} else {
			if err := queueSimpleMessage(ctx, tx, actor.UserID, WelcomeText, "bot_started", update.UpdateType); err != nil {
				return err
			}
		}
	case "message_created":
		if contactID != nil {
			break
		}
		contact, contactErr := ExtractVerifiedSharedContact(update, s.botToken)
		if contactErr != nil {
			slog.Warn("MAX shared contact rejected", "max_user_id", actor.UserID, "error", contactErr)
			if err := queueContactRequest(
				ctx,
				tx,
				actor.UserID,
				"Не удалось подтвердить номер телефона. Используйте кнопку ниже и поделитесь контактом, привязанным к вашему аккаунту MAX.",
				update.UpdateType,
			); err != nil {
				return err
			}
			break
		}
		if contact == nil {
			if err := queueContactRequest(ctx, tx, actor.UserID, ContactRequestText, update.UpdateType); err != nil {
				return err
			}
			break
		}
		createdContactID, err := findOrCreateContact(ctx, tx, *contact)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE public.max_users
			SET contact_id = $2
			WHERE max_user_id = $1
				AND contact_id IS NULL
		`, actor.UserID, createdContactID); err != nil {
			return fmt.Errorf("bind contact to MAX user: %w", err)
		}
		if err := queueSimpleMessage(ctx, tx, actor.UserID, ContactBoundText, "contact_bound", update.UpdateType); err != nil {
			return err
		}
	case "message_callback":
		if err := processMaxAuthCallback(ctx, tx, update, actor.UserID); err != nil {
			return err
		}
	}

	return commitUpdate(ctx, tx)
}

func upsertMaxUser(ctx context.Context, tx pgx.Tx, user *User, rawUser json.RawMessage) (*int, error) {
	var contactID *int
	avatarURL := normalized(user.AvatarURL)
	var rawUserWithPhoto any
	if avatarURL != nil || normalized(user.FullAvatarURL) != nil {
		rawUserWithPhoto = string(rawUser)
	}
	err := tx.QueryRow(ctx, `
		INSERT INTO public.max_users (
			max_user_id,
			username,
			avatar_url,
			raw_user,
			raw_user_with_photo,
			is_active
		)
		VALUES ($1, $2, $3, $4::jsonb, $5::jsonb, true)
		ON CONFLICT (max_user_id) DO UPDATE
		SET
			username = EXCLUDED.username,
			avatar_url = COALESCE(EXCLUDED.avatar_url, public.max_users.avatar_url),
			raw_user = EXCLUDED.raw_user,
			raw_user_with_photo = COALESCE(EXCLUDED.raw_user_with_photo, public.max_users.raw_user_with_photo),
			is_active = true
		RETURNING contact_id
	`, user.UserID, normalized(user.Username), avatarURL, string(rawUser), rawUserWithPhoto).Scan(&contactID)
	if err != nil {
		return nil, fmt.Errorf("upsert MAX user: %w", err)
	}
	return contactID, nil
}

func findOrCreateContact(ctx context.Context, tx pgx.Tx, contact SharedContact) (int, error) {
	name := strings.TrimSpace(contact.Name)
	if name == "" {
		name = "Пользователь MAX"
	}
	name = truncateRunes(name, 250)
	var contactID int
	if err := tx.QueryRow(ctx, `
		INSERT INTO public.contacts (
			name,
			phone,
			email,
			employee_post_id,
			is_active
		)
		VALUES ($1, $2, NULL, NULL, true)
		ON CONFLICT (phone) DO UPDATE
		SET phone = EXCLUDED.phone
		RETURNING id
	`, name, contact.Phone).Scan(&contactID); err != nil {
		return 0, fmt.Errorf("find/create contact from MAX phone: %w", err)
	}
	return contactID, nil
}

func queueContactRequest(ctx context.Context, tx pgx.Tx, maxUserID int64, text, updateType string) error {
	body, err := BuildContactRequestMessage(text)
	if err != nil {
		return err
	}
	return queueMessage(ctx, tx, maxUserID, body, "contact_request", updateType)
}

func queueSimpleMessage(ctx context.Context, tx pgx.Tx, maxUserID int64, text, source, updateType string) error {
	body, err := buildMessage(text, nil)
	if err != nil {
		return err
	}
	return queueMessage(ctx, tx, maxUserID, body, source, updateType)
}

func queueMessage(ctx context.Context, tx pgx.Tx, maxUserID int64, body json.RawMessage, source, updateType string) error {
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
				'source', $3::text,
				'update_type', $4::text
			)
		)
	`, maxUserID, string(body), source, updateType); err != nil {
		return fmt.Errorf("queue MAX message: %w", err)
	}
	return nil
}

func commitUpdate(ctx context.Context, tx pgx.Tx) error {
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit MAX incoming update: %w", err)
	}
	return nil
}

func (s *Store) RequeueStale(ctx context.Context, lockTimeout time.Duration) error {
	if lockTimeout <= 0 {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE public.max_out_messages
		SET
			status = 'pending',
			locked_at = NULL,
			next_attempt_at = now(),
			error_str = COALESCE(error_str, 'sender lock expired')
		WHERE status = 'sending'
			AND locked_at IS NOT NULL
			AND locked_at < now() - $1::interval
	`, durationInterval(lockTimeout))
	if err != nil {
		return fmt.Errorf("requeue stale MAX messages: %w", err)
	}
	return nil
}

func (s *Store) ClaimNext(ctx context.Context) (*OutMessage, error) {
	var result OutMessage
	err := s.pool.QueryRow(ctx, `
		WITH next_message AS (
			SELECT id
			FROM public.max_out_messages
			WHERE status = 'pending'
				AND (next_attempt_at IS NULL OR next_attempt_at <= now())
			ORDER BY created_at, id
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE public.max_out_messages AS m
		SET
			status = 'sending',
			locked_at = now(),
			attempt_count = m.attempt_count + 1,
			error_str = NULL
		FROM next_message
		WHERE m.id = next_message.id
		RETURNING m.id, m.max_user_id, m.message, m.attempt_count
	`).Scan(&result.ID, &result.MaxUserID, &result.Message, &result.AttemptCount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("claim MAX outgoing message: %w", err)
	}
	return &result, nil
}

func (s *Store) MarkSent(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE public.max_out_messages
		SET
			status = 'sent',
			sent_at = now(),
			locked_at = NULL,
			next_attempt_at = NULL,
			error_str = NULL
		WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("mark MAX message sent: %w", err)
	}
	return nil
}

func (s *Store) MarkRetry(ctx context.Context, id int64, delay time.Duration, sendErr error) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE public.max_out_messages
		SET
			status = 'pending',
			locked_at = NULL,
			next_attempt_at = now() + $2::interval,
			error_str = $3
		WHERE id = $1
	`, id, durationInterval(delay), truncateError(sendErr))
	if err != nil {
		return fmt.Errorf("schedule MAX message retry: %w", err)
	}
	return nil
}

func (s *Store) MarkFailed(ctx context.Context, id int64, sendErr error) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE public.max_out_messages
		SET
			status = 'failed',
			locked_at = NULL,
			next_attempt_at = NULL,
			error_str = $2
		WHERE id = $1
	`, id, truncateError(sendErr))
	if err != nil {
		return fmt.Errorf("mark MAX message failed: %w", err)
	}
	return nil
}

func extractRawActor(raw json.RawMessage, update Update) (json.RawMessage, error) {
	var envelope struct {
		User    json.RawMessage `json:"user"`
		Message struct {
			Sender json.RawMessage `json:"sender"`
		} `json:"message"`
		Callback struct {
			User json.RawMessage `json:"user"`
		} `json:"callback"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decode MAX update actor: %w", err)
	}
	if update.User != nil && len(envelope.User) > 0 && string(envelope.User) != "null" {
		return envelope.User, nil
	}
	if update.Callback != nil && update.Callback.User != nil && len(envelope.Callback.User) > 0 && string(envelope.Callback.User) != "null" {
		return envelope.Callback.User, nil
	}
	if update.Message != nil && update.Message.Sender != nil && len(envelope.Message.Sender) > 0 && string(envelope.Message.Sender) != "null" {
		return envelope.Message.Sender, nil
	}
	return nil, fmt.Errorf("MAX update does not contain user JSON")
}

func updateActor(update Update) *User {
	if update.User != nil && update.User.UserID > 0 {
		return update.User
	}
	if update.Callback != nil && update.Callback.User != nil && update.Callback.User.UserID > 0 {
		return update.Callback.User
	}
	if update.Message != nil && update.Message.Sender != nil && update.Message.Sender.UserID > 0 {
		return update.Message.Sender
	}
	return nil
}

func updateMaxUserID(update Update) any {
	if user := updateActor(update); user != nil {
		return user.UserID
	}
	return nil
}

func updateMaxChatID(update Update) any {
	if update.ChatID != nil && *update.ChatID != 0 {
		return *update.ChatID
	}
	if update.Message != nil && update.Message.Recipient != nil && update.Message.Recipient.ChatID != nil && *update.Message.Recipient.ChatID != 0 {
		return *update.Message.Recipient.ChatID
	}
	return nil
}

func truncateRunes(value string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

func durationInterval(value time.Duration) string {
	return fmt.Sprintf("%f seconds", value.Seconds())
}

func truncateError(err error) string {
	if err == nil {
		return ""
	}
	value := err.Error()
	const maxLength = 4000
	if len(value) > maxLength {
		return value[:maxLength]
	}
	return value
}
