package maxbot

import (
	"crypto/hmac"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

const maxWebhookBodySize = 1 << 20

type Handler struct {
	Processor *Processor
	Secret    string
}

func NewHandler(processor *Processor, secret string) *Handler {
	return &Handler{
		Processor: processor,
		Secret:    strings.TrimSpace(secret),
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.Secret != "" && !hmac.Equal([]byte(r.Header.Get("X-Max-Bot-Api-Secret")), []byte(h.Secret)) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if h.Processor == nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBodySize+1))
	if err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if len(body) == 0 || len(body) > maxWebhookBodySize {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	if err := h.Processor.ProcessRaw(r.Context(), body); err != nil {
		slog.Error("process MAX webhook update", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func normalized(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
