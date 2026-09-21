package config

import (
	"strings"
	"testing"
)

func TestMAXConfigValidation(t *testing.T) {
	cfg := defaultConfig().MAX
	cfg.BotToken = "token"
	cfg.WebhookURL = "http://example.test/webhook"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("Validate() error = %v, want HTTPS webhook error", err)
	}

	cfg.WebhookURL = "https://example.test/webhook"
	cfg.WebhookSecret = "bad secret"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "webhook_secret") {
		t.Fatalf("Validate() error = %v, want webhook secret error", err)
	}

	cfg.WebhookSecret = "secret_123"
	cfg.AuthRequestTTL = "0s"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "auth_request_ttl") {
		t.Fatalf("Validate() error = %v, want auth request TTL error", err)
	}

	cfg.AuthRequestTTL = "5m"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestMAXLongPollingConfigValidation(t *testing.T) {
	cfg := defaultConfig().MAX
	cfg.BotToken = "token"
	cfg.UpdateMode = MAXUpdateModeLongPolling
	cfg.WebhookURL = ""
	cfg.WebhookSecret = ""
	cfg.HTTPAddr = ""

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() long polling error = %v", err)
	}

	cfg.LongPollingTimeout = "91s"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "long_polling_timeout") {
		t.Fatalf("Validate() error = %v, want long polling timeout error", err)
	}

	cfg.LongPollingTimeout = "30s"
	cfg.LongPollingLimit = 1001
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "long_polling_limit") {
		t.Fatalf("Validate() error = %v, want long polling limit error", err)
	}
}

func TestMAXUpdateModeValidation(t *testing.T) {
	cfg := defaultConfig().MAX
	cfg.BotToken = "token"
	cfg.UpdateMode = "unknown"
	cfg.WebhookURL = "https://example.test/webhook"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "update_mode") {
		t.Fatalf("Validate() error = %v, want update mode error", err)
	}
}
