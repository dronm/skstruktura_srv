package maxbot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const apiBaseURL = "https://platform-api2.max.ru"

const (
	ContactRequestText   = "Для привязки аккаунта MAX поделитесь номером телефона, привязанным к вашему аккаунту MAX."
	ContactRequestButton = "Поделиться контактом"
	ContactBoundText     = "Номер телефона подтверждён. Контакт привязан."
	WelcomeText          = "MAX подключён. Вы сможете получать уведомления и подтверждать вход в приложение."
)

var updateTypes = []string{
	"bot_started",
	"bot_stopped",
	"message_created",
	"message_callback",
}

type Client struct {
	token    string
	baseURL  string
	http     *http.Client
	pollHTTP *http.Client
}

type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("MAX API HTTP %d: %s", e.StatusCode, e.Body)
}

type WebhookSubscription struct {
	URL string `json:"url"`
}

type LongPollPage struct {
	Updates []json.RawMessage
	Marker  *int64
}

func NewClient(token string) (*Client, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("MAX bot token is required")
	}
	return &Client{
		token:    token,
		baseURL:  apiBaseURL,
		http:     &http.Client{Timeout: 15 * time.Second},
		pollHTTP: &http.Client{Timeout: 100 * time.Second},
	}, nil
}

func (c *Client) ConfigureWebhook(ctx context.Context, webhookURL, secret string) error {
	webhookURL = strings.TrimSpace(webhookURL)
	if webhookURL == "" {
		return fmt.Errorf("MAX webhook URL is required")
	}
	body := map[string]any{
		"url":          webhookURL,
		"update_types": updateTypes,
	}
	if secret = strings.TrimSpace(secret); secret != "" {
		body["secret"] = secret
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode MAX webhook subscription: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/subscriptions", bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("configure MAX webhook: %w", err)
	}
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(responseBody))}
	}
	if err := validateSuccessResponse(responseBody, "configure MAX webhook"); err != nil {
		return err
	}
	return nil
}

func (c *Client) WebhookSubscriptions(ctx context.Context) ([]WebhookSubscription, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/subscriptions", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list MAX webhook subscriptions: %w", err)
	}
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(responseBody))}
	}

	var result struct {
		Subscriptions []WebhookSubscription `json:"subscriptions"`
	}
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return nil, fmt.Errorf("decode MAX webhook subscriptions: %w", err)
	}
	return result.Subscriptions, nil
}

func (c *Client) DeleteWebhook(ctx context.Context, webhookURL string) error {
	webhookURL = strings.TrimSpace(webhookURL)
	if webhookURL == "" {
		return nil
	}

	endpoint := c.baseURL + "/subscriptions?url=" + url.QueryEscape(webhookURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("delete MAX webhook subscription: %w", err)
	}
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(responseBody))}
	}
	if err := validateSuccessResponse(responseBody, "delete MAX webhook subscription"); err != nil {
		return err
	}
	return nil
}

func (c *Client) RemoveWebhookSubscriptions(ctx context.Context) (int, error) {
	subscriptions, err := c.WebhookSubscriptions(ctx)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, subscription := range subscriptions {
		webhookURL := strings.TrimSpace(subscription.URL)
		if webhookURL == "" {
			continue
		}
		if err := c.DeleteWebhook(ctx, webhookURL); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

func (c *Client) GetUpdates(
	ctx context.Context,
	marker *int64,
	timeout time.Duration,
	limit int,
) (LongPollPage, error) {
	if timeout <= 0 || timeout > 90*time.Second {
		return LongPollPage{}, fmt.Errorf("MAX long polling timeout should be greater than 0 and no more than 90 seconds")
	}
	if timeout%time.Second != 0 {
		return LongPollPage{}, fmt.Errorf("MAX long polling timeout should be a whole number of seconds")
	}
	if limit < 1 || limit > 1000 {
		return LongPollPage{}, fmt.Errorf("MAX long polling limit should be between 1 and 1000")
	}

	query := url.Values{}
	query.Set("limit", strconv.Itoa(limit))
	query.Set("timeout", strconv.Itoa(int(timeout/time.Second)))
	query.Set("types", strings.Join(updateTypes, ","))
	if marker != nil {
		query.Set("marker", strconv.FormatInt(*marker, 10))
	}

	endpoint := c.baseURL + "/updates?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return LongPollPage{}, err
	}
	req.Header.Set("Authorization", c.token)

	resp, err := c.pollHTTP.Do(req)
	if err != nil {
		return LongPollPage{}, fmt.Errorf("poll MAX updates: %w", err)
	}
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return LongPollPage{}, &APIError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(responseBody))}
	}

	var result struct {
		Updates []json.RawMessage `json:"updates"`
		Marker  *int64            `json:"marker"`
	}
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return LongPollPage{}, fmt.Errorf("decode MAX long polling response: %w", err)
	}
	return LongPollPage{
		Updates: result.Updates,
		Marker:  result.Marker,
	}, nil
}

func (c *Client) AnswerCallback(ctx context.Context, callbackID, text string) error {
	callbackID = strings.TrimSpace(callbackID)
	if callbackID == "" {
		return fmt.Errorf("MAX callback id is required")
	}

	body, err := json.Marshal(map[string]any{
		"message": map[string]any{
			"text": strings.TrimSpace(text),
		},
	})
	if err != nil {
		return fmt.Errorf("encode MAX callback answer: %w", err)
	}

	endpoint := c.baseURL + "/answers?callback_id=" + url.QueryEscape(callbackID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("answer MAX callback: %w", err)
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(responseBody))}
	}
	if err := validateSuccessResponse(responseBody, "answer MAX callback"); err != nil {
		return err
	}
	return nil
}

func (c *Client) SendMessage(ctx context.Context, userID int64, body json.RawMessage) error {
	if userID <= 0 {
		return fmt.Errorf("MAX user id should be positive")
	}
	if len(body) == 0 || !json.Valid(body) {
		return fmt.Errorf("MAX message body should be valid JSON")
	}

	endpoint := c.baseURL + "/messages?user_id=" + url.QueryEscape(fmt.Sprintf("%d", userID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("send MAX message: %w", err)
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(responseBody)),
		}
	}
	return nil
}

func BuildContactRequestMessage(text string) (json.RawMessage, error) {
	if strings.TrimSpace(text) == "" {
		text = ContactRequestText
	}
	return buildMessage(text, [][]any{{
		map[string]any{
			"type": "request_contact",
			"text": ContactRequestButton,
		},
	}})
}

func BuildContactBoundMessage() (json.RawMessage, error) {
	return buildMessage(ContactBoundText, nil)
}

func BuildWelcomeMessage() (json.RawMessage, error) {
	return buildMessage(WelcomeText, nil)
}

func buildMessage(text string, buttons [][]any) (json.RawMessage, error) {
	body := map[string]any{"text": text}
	if len(buttons) > 0 {
		body["attachments"] = []any{
			map[string]any{
				"type": "inline_keyboard",
				"payload": map[string]any{
					"buttons": buttons,
				},
			},
		}
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode MAX message: %w", err)
	}
	return encoded, nil
}

func validateSuccessResponse(body []byte, operation string) error {
	if len(body) == 0 {
		return nil
	}
	var result struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("decode %s response: %w", operation, err)
	}
	if !result.Success {
		message := strings.TrimSpace(result.Message)
		if message == "" {
			message = "MAX API returned success=false"
		}
		return fmt.Errorf("%s: %s", operation, message)
	}
	return nil
}
