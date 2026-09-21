package maxbot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestBuildContactRequestMessage(t *testing.T) {
	body, err := BuildContactRequestMessage("")
	if err != nil {
		t.Fatalf("BuildContactRequestMessage(): %v", err)
	}
	var decoded struct {
		Text        string `json:"text"`
		Attachments []struct {
			Type    string `json:"type"`
			Payload struct {
				Buttons [][]struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"buttons"`
			} `json:"payload"`
		} `json:"attachments"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode contact request: %v", err)
	}
	if decoded.Text != ContactRequestText {
		t.Fatalf("text = %q", decoded.Text)
	}
	if len(decoded.Attachments) != 1 || decoded.Attachments[0].Type != "inline_keyboard" {
		t.Fatalf("attachments = %#v", decoded.Attachments)
	}
	if len(decoded.Attachments[0].Payload.Buttons) != 1 || len(decoded.Attachments[0].Payload.Buttons[0]) != 1 {
		t.Fatalf("buttons = %#v", decoded.Attachments[0].Payload.Buttons)
	}
	button := decoded.Attachments[0].Payload.Buttons[0][0]
	if button.Type != "request_contact" || button.Text != ContactRequestButton {
		t.Fatalf("button = %#v", button)
	}
}

func TestUpdateTypesIncludeAuthenticationEvents(t *testing.T) {
	wanted := map[string]bool{
		"bot_started":      false,
		"bot_stopped":      false,
		"message_created":  false,
		"message_callback": false,
	}
	for _, updateType := range updateTypes {
		if _, ok := wanted[updateType]; ok {
			wanted[updateType] = true
		}
	}
	for updateType, found := range wanted {
		if !found {
			t.Fatalf("MAX update type %q is missing", updateType)
		}
	}
}

func TestGetUpdates(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodGet || r.URL.Path != "/updates" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "token" {
			t.Fatalf("Authorization = %q", got)
		}
		if got := r.URL.Query().Get("timeout"); got != "30" {
			t.Fatalf("timeout = %q", got)
		}
		if got := r.URL.Query().Get("limit"); got != "100" {
			t.Fatalf("limit = %q", got)
		}
		gotTypes := strings.Split(r.URL.Query().Get("types"), ",")
		sort.Strings(gotTypes)
		wantTypes := append([]string(nil), updateTypes...)
		sort.Strings(wantTypes)
		if !reflect.DeepEqual(gotTypes, wantTypes) {
			t.Fatalf("types = %#v, want %#v", gotTypes, wantTypes)
		}
		if requests == 1 {
			if got := r.URL.Query().Get("marker"); got != "" {
				t.Fatalf("initial marker = %q", got)
			}
		} else if got := r.URL.Query().Get("marker"); got != "42" {
			t.Fatalf("next marker = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"updates":[{"update_type":"bot_started"}],"marker":42}`))
	}))
	defer server.Close()

	client := &Client{
		token:    "token",
		baseURL:  server.URL,
		http:     server.Client(),
		pollHTTP: server.Client(),
	}
	page, err := client.GetUpdates(context.Background(), nil, 30*time.Second, 100)
	if err != nil {
		t.Fatalf("GetUpdates(): %v", err)
	}
	if page.Marker == nil || *page.Marker != 42 || len(page.Updates) != 1 {
		t.Fatalf("page = %#v", page)
	}
	if _, err := client.GetUpdates(context.Background(), page.Marker, 30*time.Second, 100); err != nil {
		t.Fatalf("GetUpdates(marker): %v", err)
	}
}

func TestRemoveWebhookSubscriptions(t *testing.T) {
	deleted := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if r.URL.Path != "/subscriptions" {
				t.Fatalf("GET path = %q", r.URL.Path)
			}
			_, _ = w.Write([]byte(`{"subscriptions":[{"url":"https://one.example/webhook"},{"url":"https://two.example/webhook"}]}`))
		case http.MethodDelete:
			deleted = append(deleted, r.URL.Query().Get("url"))
			_, _ = w.Write([]byte(`{"success":true}`))
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	client := &Client{
		token:    "token",
		baseURL:  server.URL,
		http:     server.Client(),
		pollHTTP: server.Client(),
	}
	count, err := client.RemoveWebhookSubscriptions(context.Background())
	if err != nil {
		t.Fatalf("RemoveWebhookSubscriptions(): %v", err)
	}
	if count != 2 {
		t.Fatalf("removed count = %d", count)
	}
	want := []string{"https://one.example/webhook", "https://two.example/webhook"}
	if !reflect.DeepEqual(deleted, want) {
		t.Fatalf("deleted = %#v, want %#v", deleted, want)
	}
}
