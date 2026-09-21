package models

import (
	"time"

	wmodels "github.com/dronm/webapp/models"
)

const (
	UserMaxAuthStatusPending       = "pending"
	UserMaxAuthStatusApproved      = "approved"
	UserMaxAuthStatusDeclined      = "declined"
	UserMaxAuthStatusExpired       = "expired"
	UserMaxAuthStatusConsumed      = "consumed"
	UserMaxAuthStatusAuthenticated = "authenticated"
)

type UserMaxAuthRequest struct {
	Phone string `json:"phone" required:"true" maxLen:"30"`
}

type UserMaxAuthRequestResponse struct {
	RequestID string    `json:"request_id"`
	Status    string    `json:"status"`
	ExpiresAt time.Time `json:"expires_at"`
}

type UserMaxAuthCompleteRequest struct {
	RequestID string `json:"request_id" required:"true" maxLen:"64"`
}

type UserMaxAuthCompleteInput struct {
	UserInf UserLoginInf
	Model   UserMaxAuthCompleteRequest
}

type UserMaxAuthCompleteResponse struct {
	Status string        `json:"status"`
	User   *UserLogin    `json:"user,omitempty"`
	Auth   *wmodels.Auth `json:"auth,omitempty"`
}
