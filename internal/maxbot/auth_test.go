package maxbot

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildMaxAuthRequestMessage(t *testing.T) {
	requestID := strings.Repeat("a", 64)
	body, err := BuildMaxAuthRequestMessage(requestID)
	if err != nil {
		t.Fatalf("BuildMaxAuthRequestMessage(): %v", err)
	}

	var decoded struct {
		Text        string `json:"text"`
		Attachments []struct {
			Type    string `json:"type"`
			Payload struct {
				Buttons [][]struct {
					Type    string `json:"type"`
					Text    string `json:"text"`
					Payload string `json:"payload"`
				} `json:"buttons"`
			} `json:"payload"`
		} `json:"attachments"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode auth request: %v", err)
	}
	if decoded.Text != MaxAuthRequestText {
		t.Fatalf("text = %q", decoded.Text)
	}
	if len(decoded.Attachments) != 1 || decoded.Attachments[0].Type != "inline_keyboard" {
		t.Fatalf("attachments = %#v", decoded.Attachments)
	}
	buttons := decoded.Attachments[0].Payload.Buttons
	if len(buttons) != 1 || len(buttons[0]) != 2 {
		t.Fatalf("buttons = %#v", buttons)
	}
	if buttons[0][0].Type != "callback" || buttons[0][0].Text != MaxAuthApproveButton {
		t.Fatalf("approve button = %#v", buttons[0][0])
	}
	if buttons[0][1].Type != "callback" || buttons[0][1].Text != MaxAuthDeclineButton {
		t.Fatalf("decline button = %#v", buttons[0][1])
	}
	if decision, gotID, ok := ParseMaxAuthCallbackPayload(buttons[0][0].Payload); !ok || decision != MaxAuthApprove || gotID != requestID {
		t.Fatalf("approve payload parsed as decision=%q id=%q ok=%v", decision, gotID, ok)
	}
	if decision, gotID, ok := ParseMaxAuthCallbackPayload(buttons[0][1].Payload); !ok || decision != MaxAuthDecline || gotID != requestID {
		t.Fatalf("decline payload parsed as decision=%q id=%q ok=%v", decision, gotID, ok)
	}
}

func TestParseMaxAuthCallbackPayloadRejectsInvalidValues(t *testing.T) {
	invalid := []string{
		"",
		"other:approve:" + strings.Repeat("a", 64),
		"max_auth:unknown:" + strings.Repeat("a", 64),
		"max_auth:approve:short",
		"max_auth:approve:" + strings.Repeat("z", 64),
	}
	for _, value := range invalid {
		if _, _, ok := ParseMaxAuthCallbackPayload(value); ok {
			t.Fatalf("ParseMaxAuthCallbackPayload(%q) accepted invalid value", value)
		}
	}
}
