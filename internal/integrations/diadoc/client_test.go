package diadoc

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dronm/skstruktura/internal/config"
)

func TestClientRefreshAndRetrieveEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/connect/token":
			if err := r.ParseForm(); err != nil {
				t.Errorf("ParseForm() error = %v", err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != "refresh" {
				t.Errorf("unexpected token form: %v", r.Form)
				http.Error(w, "unexpected token form", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"access_token":"access","refresh_token":"next","expires_in":3600}`)
		case "/V8/GetNewEvents":
			if r.Header.Get("Authorization") != "Bearer access" {
				t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
				http.Error(w, "unexpected authorization", http.StatusUnauthorized)
				return
			}
			if r.URL.Query().Get("afterIndexKey") != "cursor" ||
				r.URL.Query().Get("documentDirection") != "Inbound" ||
				r.URL.Query().Get("typeNamedId") != "UniversalTransferDocument" {
				t.Errorf("unexpected query: %v", r.URL.Query())
				http.Error(w, "unexpected query", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"Events":[{"EventId":"event","IndexKey":"next"}],"TotalCount":1}`)
		case "/V4/GetEntityContent":
			_, _ = io.WriteString(w, `<document id="1"/>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(config.DiadocConfig{
		EntryPoint:         server.URL,
		IdentityEntryPoint: server.URL,
		ClientID:           "client",
		ClientSecret:       "secret",
		RedirectURL:        "https://example.com/callback",
	}, time.Second)

	token, err := client.RefreshToken(context.Background(), "refresh")
	if err != nil {
		t.Fatalf("RefreshToken() error = %v", err)
	}
	if token.AccessToken != "access" || token.RefreshToken != "next" {
		t.Fatalf("RefreshToken() = %#v", token)
	}

	events, err := client.GetNewEvents(context.Background(), token.AccessToken, NewEventsRequest{
		BoxID:         "box",
		AfterIndexKey: "cursor",
		Limit:         100,
	})
	if err != nil {
		t.Fatalf("GetNewEvents() error = %v", err)
	}
	if len(events.Events) != 1 || events.Events[0].IndexKey != "next" {
		t.Fatalf("GetNewEvents() = %#v", events)
	}

	content, err := client.GetEntityContent(context.Background(), token.AccessToken, "box", "message", "entity")
	if err != nil {
		t.Fatalf("GetEntityContent() error = %v", err)
	}
	if string(content) != `<document id="1"/>` {
		t.Fatalf("GetEntityContent() = %q", content)
	}
}
