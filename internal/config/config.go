// Package config contains application configuration parameters.
package config

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dronm/webapp"
	websession "github.com/dronm/webapp/session"
)

const DefaultConfigPath = "config.json"

type Config struct {
	HTTP     HTTPConfig     `json:"http"`
	Database DatabaseConfig `json:"database"`
	CORS     CORSConfig     `json:"cors"`
	Session  SessionConfig  `json:"session"`
	Debug    DebugConfig    `json:"debug"`
	MainMenu MainMenuConfig `json:"main_menu"`
	Diadoc   DiadocConfig   `json:"diadoc"`
	MAX      MAXConfig      `json:"max"`
}

type HTTPConfig struct {
	Addr           string `json:"addr"`
	ReadTimeout    string `json:"read_timeout"`
	WriteTimeout   string `json:"write_timeout"`
	IdleTimeout    string `json:"idle_timeout"`
	RequestTimeout string `json:"request_timeout"`
}

type DatabaseConfig struct {
	Primary     string            `json:"primary"`
	Secondaries map[string]string `json:"secondaries"`
}

type CORSConfig struct {
	AllowedOrigins   []string `json:"allowed_origins"`
	AllowedMethods   []string `json:"allowed_methods"`
	AllowedHeaders   []string `json:"allowed_headers"`
	ExposedHeaders   []string `json:"exposed_headers"`
	AllowCredentials bool     `json:"allow_credentials"`
	MaxAgeSeconds    int      `json:"max_age_seconds"`
}

type SessionConfig struct {
	Enabled        bool        `json:"enabled"`
	CookieName     string      `json:"cookie_name"`
	CookieDomain   string      `json:"cookie_domain"`
	Secure         bool        `json:"secure"`
	SameSite       string      `json:"same_site"`
	MaxLifeTime    int64       `json:"max_life_time"`
	MaxIdleTime    int64       `json:"max_idle_time"`
	DestroyAllTime string      `json:"destroy_all_time"`
	Redis          RedisConfig `json:"redis"`
}

type RedisConfig struct {
	Connect   string `json:"connect"`
	Namespace string `json:"namespace"`
}

type MainMenuConfig struct {
	CacheEnabled bool   `json:"cache_enabled"`
	CacheTTL     string `json:"cache_ttl"`
}

type DebugConfig struct {
	LogConfig  bool   `json:"log_config"`
	LogLevel   string `json:"log_level"`
	LogFormat  string `json:"log_format"`
	SQLQueries bool   `json:"sql_queries"`
}

const (
	MAXUpdateModeWebhook     = "webhook"
	MAXUpdateModeLongPolling = "long_polling"
)

type MAXConfig struct {
	HTTPAddr                      string `json:"http_addr"`
	UpdateMode                    string `json:"update_mode"`
	BotToken                      string `json:"bot_token"`
	WebhookURL                    string `json:"webhook_url"`
	WebhookSecret                 string `json:"webhook_secret"`
	AuthRequestTTL                string `json:"auth_request_ttl"`
	LongPollingTimeout            string `json:"long_polling_timeout"`
	LongPollingLimit              int    `json:"long_polling_limit"`
	SenderPollInterval            string `json:"sender_poll_interval"`
	SenderNotifyReconnectInterval string `json:"sender_notify_reconnect_interval"`
	SenderLockTimeout             string `json:"sender_lock_timeout"`
	SenderRetryBaseDelay          string `json:"sender_retry_base_delay"`
	SenderRetryMaxDelay           string `json:"sender_retry_max_delay"`
	SenderMaxAttempts             int    `json:"sender_max_attempts"`
}

type DiadocConfig struct {
	EntryPoint           string `json:"entry_point"`
	IdentityEntryPoint   string `json:"identity_entry_point"`
	ClientID             string `json:"client_id"`
	ClientSecret         string `json:"client_secret"`
	RedirectURL          string `json:"redirect_url"`
	Scope                string `json:"scope"`
	BoxID                string `json:"box_id"`
	PollInterval         string `json:"poll_interval"`
	RequestTimeout       string `json:"request_timeout"`
	EventTimestampFrom   string `json:"event_timestamp_from"`
	EventLimit           int    `json:"event_limit"`
	LogDocumentContent   bool   `json:"log_document_content"`
	MaxLoggedContentSize int64  `json:"max_logged_content_size"`
}

func Load(path string) (Config, error) {
	if path == "" {
		path = DefaultConfigPath
	}

	body, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}

	cfg := defaultConfig()
	if err := json.Unmarshal(body, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %q: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func defaultConfig() Config {
	return Config{
		HTTP: HTTPConfig{
			Addr:           "127.0.0.1:8080",
			ReadTimeout:    "15s",
			WriteTimeout:   "15s",
			IdleTimeout:    "60s",
			RequestTimeout: "30s",
		},
		CORS: CORSConfig{
			AllowedOrigins: []string{"http://localhost:3000"},
			AllowedMethods: []string{
				http.MethodGet,
				http.MethodPost,
				http.MethodPut,
				http.MethodPatch,
				http.MethodDelete,
				http.MethodOptions,
			},
			AllowedHeaders: []string{
				"Content-Type",
				"Authorization",
				webapp.DefaultQueryIDHeader,
			},
			ExposedHeaders: []string{
				webapp.DefaultQueryIDHeader,
			},
			AllowCredentials: true,
			MaxAgeSeconds:    86400,
		},
		Debug: DebugConfig{
			LogLevel:  "info",
			LogFormat: "text",
		},
		Session: SessionConfig{
			CookieName:     "_s",
			SameSite:       "lax",
			MaxLifeTime:    86400,
			MaxIdleTime:    3600,
			DestroyAllTime: "03:00",
			Redis: RedisConfig{
				Connect:   "127.0.0.1:6379",
				Namespace: "supplier-test",
			},
		},
		MainMenu: MainMenuConfig{
			CacheEnabled: true,
			CacheTTL:     "15m",
		},
		Diadoc: DiadocConfig{
			EntryPoint:           "https://diadoc-api.kontur.ru",
			IdentityEntryPoint:   "https://identity.kontur.ru",
			PollInterval:         "1m",
			RequestTimeout:       "30s",
			EventLimit:           100,
			LogDocumentContent:   true,
			MaxLoggedContentSize: 256 * 1024,
		},
		MAX: MAXConfig{
			HTTPAddr:                      "127.0.0.1:59001",
			UpdateMode:                    MAXUpdateModeWebhook,
			AuthRequestTTL:                "5m",
			LongPollingTimeout:            "30s",
			LongPollingLimit:              100,
			SenderPollInterval:            "2s",
			SenderNotifyReconnectInterval: "5s",
			SenderLockTimeout:             "2m",
			SenderRetryBaseDelay:          "5s",
			SenderRetryMaxDelay:           "5m",
			SenderMaxAttempts:             10,
		},
	}
}

func (c Config) Validate() error {
	if c.HTTP.Addr == "" {
		return fmt.Errorf("http.addr is required")
	}
	if c.Database.Primary == "" {
		return fmt.Errorf("database.primary is required")
	}
	if _, err := c.HTTPReadTimeout(); err != nil {
		return err
	}
	if _, err := c.HTTPWriteTimeout(); err != nil {
		return err
	}
	if _, err := c.HTTPIdleTimeout(); err != nil {
		return err
	}
	if _, err := c.RequestTimeout(); err != nil {
		return err
	}
	if c.MainMenu.CacheEnabled {
		cacheTTL, err := c.MainMenu.CacheTTLDuration()
		if err != nil {
			return err
		}
		if cacheTTL <= 0 {
			return fmt.Errorf("main_menu.cache_ttl should be positive")
		}
	}
	if err := c.Diadoc.Validate(); err != nil {
		return err
	}
	if err := c.MAX.Validate(); err != nil {
		return err
	}
	return nil
}

func (c DatabaseConfig) GetPrimary() string {
	return c.Primary
}

func (c DatabaseConfig) GetSecondaries() map[string]string {
	return c.Secondaries
}

func (c Config) HTTPReadTimeout() (time.Duration, error) {
	return parseDuration("http.read_timeout", c.HTTP.ReadTimeout)
}

func (c Config) HTTPWriteTimeout() (time.Duration, error) {
	return parseDuration("http.write_timeout", c.HTTP.WriteTimeout)
}

func (c Config) HTTPIdleTimeout() (time.Duration, error) {
	return parseDuration("http.idle_timeout", c.HTTP.IdleTimeout)
}

func (c Config) RequestTimeout() (time.Duration, error) {
	return parseDuration("http.request_timeout", c.HTTP.RequestTimeout)
}

func (c MainMenuConfig) CacheTTLDuration() (time.Duration, error) {
	return parseDuration("main_menu.cache_ttl", c.CacheTTL)
}

func (c MAXConfig) Configured() bool {
	return strings.TrimSpace(c.BotToken) != "" ||
		strings.TrimSpace(c.WebhookURL) != "" ||
		strings.TrimSpace(c.WebhookSecret) != ""
}

func (c MAXConfig) Validate() error {
	if !c.Configured() {
		return nil
	}
	if strings.TrimSpace(c.BotToken) == "" {
		return fmt.Errorf("max.bot_token is required")
	}

	mode := strings.TrimSpace(c.UpdateMode)
	switch mode {
	case MAXUpdateModeWebhook:
		if strings.TrimSpace(c.HTTPAddr) == "" {
			return fmt.Errorf("max.http_addr is required in webhook mode")
		}
		webhookURL := strings.TrimSpace(c.WebhookURL)
		if webhookURL == "" {
			return fmt.Errorf("max.webhook_url is required in webhook mode")
		}
		if !strings.HasPrefix(webhookURL, "https://") {
			return fmt.Errorf("max.webhook_url should use https")
		}
	case MAXUpdateModeLongPolling:
		// webhook_url/webhook_secret may stay configured while development uses
		// Long Polling. They are ignored until update_mode is switched back.
	default:
		return fmt.Errorf("max.update_mode should be %q or %q", MAXUpdateModeWebhook, MAXUpdateModeLongPolling)
	}

	if secret := strings.TrimSpace(c.WebhookSecret); secret != "" {
		if len(secret) < 5 || len(secret) > 256 || !validMAXWebhookSecret(secret) {
			return fmt.Errorf("max.webhook_secret should be 5-256 characters containing only letters, digits, underscore and hyphen")
		}
	}

	longPollingTimeout, err := c.LongPollingTimeoutDuration()
	if err != nil {
		return err
	}
	if longPollingTimeout <= 0 || longPollingTimeout > 90*time.Second {
		return fmt.Errorf("max.long_polling_timeout should be greater than 0 and no more than 90s")
	}
	if longPollingTimeout%time.Second != 0 {
		return fmt.Errorf("max.long_polling_timeout should be a whole number of seconds")
	}
	if c.LongPollingLimit < 1 || c.LongPollingLimit > 1000 {
		return fmt.Errorf("max.long_polling_limit should be between 1 and 1000")
	}

	durations := []struct {
		name  string
		value string
	}{
		{name: "max.auth_request_ttl", value: c.AuthRequestTTL},
		{name: "max.sender_poll_interval", value: c.SenderPollInterval},
		{name: "max.sender_notify_reconnect_interval", value: c.SenderNotifyReconnectInterval},
		{name: "max.sender_lock_timeout", value: c.SenderLockTimeout},
		{name: "max.sender_retry_base_delay", value: c.SenderRetryBaseDelay},
		{name: "max.sender_retry_max_delay", value: c.SenderRetryMaxDelay},
	}
	for _, item := range durations {
		duration, err := parseDuration(item.name, item.value)
		if err != nil {
			return err
		}
		if duration <= 0 {
			return fmt.Errorf("%s should be positive", item.name)
		}
	}
	if c.SenderMaxAttempts <= 0 {
		return fmt.Errorf("max.sender_max_attempts should be positive")
	}
	return nil
}

func (c MAXConfig) AuthRequestTTLDuration() (time.Duration, error) {
	return parseDuration("max.auth_request_ttl", c.AuthRequestTTL)
}

func (c MAXConfig) LongPollingTimeoutDuration() (time.Duration, error) {
	return parseDuration("max.long_polling_timeout", c.LongPollingTimeout)
}

func (c MAXConfig) SenderPollIntervalDuration() (time.Duration, error) {
	return parseDuration("max.sender_poll_interval", c.SenderPollInterval)
}

func (c MAXConfig) SenderNotifyReconnectIntervalDuration() (time.Duration, error) {
	return parseDuration("max.sender_notify_reconnect_interval", c.SenderNotifyReconnectInterval)
}

func (c MAXConfig) SenderLockTimeoutDuration() (time.Duration, error) {
	return parseDuration("max.sender_lock_timeout", c.SenderLockTimeout)
}

func (c MAXConfig) SenderRetryBaseDelayDuration() (time.Duration, error) {
	return parseDuration("max.sender_retry_base_delay", c.SenderRetryBaseDelay)
}

func (c MAXConfig) SenderRetryMaxDelayDuration() (time.Duration, error) {
	return parseDuration("max.sender_retry_max_delay", c.SenderRetryMaxDelay)
}

func validMAXWebhookSecret(value string) bool {
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '_' || char == '-' {
			continue
		}
		return false
	}
	return true
}

func (c DiadocConfig) Configured() bool {
	return c.ClientID != "" || c.ClientSecret != ""
}

func (c DiadocConfig) Validate() error {
	if !c.Configured() {
		return nil
	}
	if c.ClientID == "" {
		return fmt.Errorf("diadoc.client_id is required")
	}
	if c.ClientSecret == "" {
		return fmt.Errorf("diadoc.client_secret is required")
	}
	if c.EntryPoint == "" {
		return fmt.Errorf("diadoc.entry_point is required")
	}
	if c.IdentityEntryPoint == "" {
		return fmt.Errorf("diadoc.identity_entry_point is required")
	}
	if c.RedirectURL == "" {
		return fmt.Errorf("diadoc.redirect_url is required")
	}
	pollInterval, err := c.PollIntervalDuration()
	if err != nil {
		return err
	}
	if pollInterval <= 0 {
		return fmt.Errorf("diadoc.poll_interval should be positive")
	}
	requestTimeout, err := c.RequestTimeoutDuration()
	if err != nil {
		return err
	}
	if requestTimeout <= 0 {
		return fmt.Errorf("diadoc.request_timeout should be positive")
	}
	if c.EventLimit < 1 || c.EventLimit > 500 {
		return fmt.Errorf("diadoc.event_limit should be between 1 and 500")
	}
	if c.EventTimestampFrom != "" {
		if _, err := time.Parse(time.RFC3339, c.EventTimestampFrom); err != nil {
			return fmt.Errorf("diadoc.event_timestamp_from: %w", err)
		}
	}
	if c.MaxLoggedContentSize < 0 {
		return fmt.Errorf("diadoc.max_logged_content_size should not be negative")
	}
	return nil
}

func (c DiadocConfig) PollIntervalDuration() (time.Duration, error) {
	return parseDuration("diadoc.poll_interval", c.PollInterval)
}

func (c DiadocConfig) RequestTimeoutDuration() (time.Duration, error) {
	return parseDuration("diadoc.request_timeout", c.RequestTimeout)
}

func (c DiadocConfig) ScopeValue() string {
	if c.Scope != "" {
		return c.Scope
	}
	if strings.Contains(strings.ToLower(c.EntryPoint), "staging") {
		return "openid profile email offline_access Diadoc.PublicAPI.Staging"
	}
	return "openid profile email offline_access Diadoc.PublicAPI"
}

func (c CORSConfig) WebappConfig() webapp.CORSConfig {
	return webapp.CORSConfig{
		AllowedOrigins:   c.AllowedOrigins,
		AllowedMethods:   c.AllowedMethods,
		AllowedHeaders:   c.AllowedHeaders,
		ExposedHeaders:   c.ExposedHeaders,
		AllowCredentials: c.AllowCredentials,
		MaxAgeSeconds:    c.MaxAgeSeconds,
	}
}

func (c SessionConfig) WebappConfig() websession.Config {
	return websession.Config{
		CookieName:     c.CookieName,
		CookieDomain:   c.CookieDomain,
		Secure:         c.Secure,
		HTTPOnly:       true,
		SameSite:       parseSameSite(c.SameSite),
		MaxLifeTime:    c.MaxLifeTime,
		MaxIdleTime:    c.MaxIdleTime,
		DestroyAllTime: c.DestroyAllTime,
	}
}

func (c RedisConfig) WebappConfig() websession.RedisConfig {
	return websession.RedisConfig{
		Connect:   c.Connect,
		Namespace: c.Namespace,
	}
}

func parseDuration(name string, value string) (time.Duration, error) {
	if value == "" {
		return 0, nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}

	return duration, nil
}

func parseSameSite(value string) http.SameSite {
	switch value {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	case "lax", "":
		return http.SameSiteLaxMode
	default:
		return http.SameSiteLaxMode
	}
}
