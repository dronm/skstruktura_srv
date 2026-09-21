package maxbot

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	maxAuthCallbackPrefix = "max_auth"

	MaxAuthApprove = "approve"
	MaxAuthDecline = "decline"

	MaxAuthRequestText   = "Выполнен запрос на вход в приложение СК Структура. Разрешить вход?"
	MaxAuthApproveButton = "Войти"
	MaxAuthDeclineButton = "Отклонить"
	MaxAuthApprovedText  = "Вход подтверждён. Вернитесь в приложение."
	MaxAuthDeclinedText  = "Вход отклонён."
	MaxAuthExpiredText   = "Запрос на вход недействителен или уже истёк."
	MaxAuthConsumedText  = "Вход уже выполнен."
)

type CallbackAnswer struct {
	CallbackID string
	Text       string
}

func BuildMaxAuthRequestMessage(requestID string) (json.RawMessage, error) {
	requestID = strings.TrimSpace(requestID)
	if !validAuthRequestID(requestID) {
		return nil, fmt.Errorf("invalid MAX authentication request id")
	}

	return buildMessage(MaxAuthRequestText, [][]any{{
		map[string]any{
			"type":    "callback",
			"text":    MaxAuthApproveButton,
			"payload": buildMaxAuthCallbackPayload(MaxAuthApprove, requestID),
		},
		map[string]any{
			"type":    "callback",
			"text":    MaxAuthDeclineButton,
			"payload": buildMaxAuthCallbackPayload(MaxAuthDecline, requestID),
		},
	}})
}

func ParseMaxAuthCallbackPayload(payload string) (decision, requestID string, ok bool) {
	parts := strings.SplitN(strings.TrimSpace(payload), ":", 3)
	if len(parts) != 3 || parts[0] != maxAuthCallbackPrefix {
		return "", "", false
	}
	decision = parts[1]
	requestID = parts[2]
	if decision != MaxAuthApprove && decision != MaxAuthDecline {
		return "", "", false
	}
	if !validAuthRequestID(requestID) {
		return "", "", false
	}
	return decision, requestID, true
}

func buildMaxAuthCallbackPayload(decision, requestID string) string {
	return maxAuthCallbackPrefix + ":" + decision + ":" + requestID
}

func validAuthRequestID(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}
