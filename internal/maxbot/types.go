package maxbot

import (
	"encoding/json"
	"time"
)

type User struct {
	UserID        int64   `json:"user_id"`
	FirstName     string  `json:"first_name"`
	LastName      *string `json:"last_name"`
	Name          *string `json:"name"`
	Username      *string `json:"username"`
	AvatarURL     *string `json:"avatar_url"`
	FullAvatarURL *string `json:"full_avatar_url"`
}

type Recipient struct {
	ChatID *int64 `json:"chat_id"`
	UserID *int64 `json:"user_id"`
}

type Attachment struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type MessageBody struct {
	Text        *string      `json:"text"`
	Attachments []Attachment `json:"attachments"`
}

type Message struct {
	Sender    *User        `json:"sender"`
	Recipient *Recipient   `json:"recipient"`
	Body      *MessageBody `json:"body"`
}

type Callback struct {
	CallbackID string `json:"callback_id"`
	Payload    string `json:"payload"`
	User       *User  `json:"user"`
}

type Update struct {
	UpdateType string          `json:"update_type"`
	ChatID     *int64          `json:"chat_id"`
	User       *User           `json:"user"`
	Message    *Message        `json:"message"`
	Callback   *Callback       `json:"callback"`
	RawUser    json.RawMessage `json:"-"`
}

type ContactAttachmentPayload struct {
	VCFInfo string          `json:"vcf_info"`
	Hash    string          `json:"hash"`
	MaxInfo json.RawMessage `json:"max_info"`
}

type SharedContact struct {
	Phone string
	Name  string
}

type OutMessage struct {
	ID           int64
	MaxUserID    int64
	Message      json.RawMessage
	AttemptCount int
}

type SenderConfig struct {
	DSN                     string
	PollInterval            time.Duration
	NotifyReconnectInterval time.Duration
	LockTimeout             time.Duration
	RetryBaseDelay          time.Duration
	RetryMaxDelay           time.Duration
	MaxAttempts             int
}
