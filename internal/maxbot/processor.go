package maxbot

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

// Processor contains update handling shared by Webhook and Long Polling.
type Processor struct {
	Store  *Store
	Client *Client
}

func NewProcessor(store *Store, client *Client) *Processor {
	return &Processor{
		Store:  store,
		Client: client,
	}
}

func (p *Processor) ProcessRaw(ctx context.Context, raw json.RawMessage) error {
	if p == nil || p.Store == nil {
		return fmt.Errorf("MAX update processor is not initialized")
	}
	if len(raw) == 0 || !json.Valid(raw) {
		return fmt.Errorf("MAX update should be valid JSON")
	}

	var update Update
	if err := json.Unmarshal(raw, &update); err != nil {
		return fmt.Errorf("decode MAX update: %w", err)
	}
	update.UpdateType = strings.TrimSpace(update.UpdateType)
	if update.UpdateType == "" {
		return fmt.Errorf("MAX update_type is required")
	}

	if err := p.Store.SaveUpdate(ctx, raw, update); err != nil {
		return fmt.Errorf("persist/process MAX update %q: %w", update.UpdateType, err)
	}

	if update.UpdateType == "message_callback" && p.Client != nil {
		answer, err := p.Store.CallbackAnswer(ctx, update)
		if err != nil {
			return fmt.Errorf("resolve MAX callback answer: %w", err)
		}
		if answer != nil {
			if err := p.Client.AnswerCallback(ctx, answer.CallbackID, answer.Text); err != nil {
				// The authentication decision itself is already committed. Failing to
				// refresh the callback UI must not replay the update and decision.
				slog.Error("answer MAX callback", "callback_id", answer.CallbackID, "error", err)
			}
		}
	}

	return nil
}
