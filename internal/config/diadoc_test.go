package config

import (
	"strings"
	"testing"
)

func TestDiadocConfigUsesStagingScope(t *testing.T) {
	cfg := defaultConfig().Diadoc
	cfg.EntryPoint = "https://diadoc-api-staging.kontur.ru"
	if got := cfg.ScopeValue(); !strings.Contains(got, "Diadoc.PublicAPI.Staging") {
		t.Fatalf("ScopeValue() = %q", got)
	}
}

func TestDiadocConfigRequiresCompleteOAuthSettings(t *testing.T) {
	cfg := defaultConfig().Diadoc
	cfg.ClientID = "client"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "client_secret") {
		t.Fatalf("Validate() error = %v, want missing client_secret", err)
	}

	cfg.ClientSecret = "secret"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "redirect_url") {
		t.Fatalf("Validate() error = %v, want missing redirect_url", err)
	}

	cfg.RedirectURL = "https://example.com/api/diadoc/auth/callback"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
