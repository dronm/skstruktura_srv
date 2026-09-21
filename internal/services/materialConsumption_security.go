package services

import (
	"github.com/dronm/session"
	"github.com/dronm/skstruktura/internal/apperrors"
	"github.com/dronm/skstruktura/internal/models"
)

func requireMaterialConsumptionAdminSession(currentSession session.Session, permission string) error {
	if currentSession == nil {
		return apperrors.SessionRequired()
	}

	user := models.UserLogin{}
	if err := currentSession.Get("user", &user); err != nil || user.ID <= 0 {
		return apperrors.SessionRequired()
	}
	if user.RoleID != models.RoleIDAdmin {
		return apperrors.Forbidden(permission)
	}

	return nil
}
