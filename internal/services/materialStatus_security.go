package services

import (
	"github.com/dronm/modelbind"
	"github.com/dronm/session"
	"github.com/dronm/skstruktura/internal/apperrors"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

func requireMaterialStatusAdminSession(currentSession session.Session, permission string) error {
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

func validateGenericMaterialStatusCreateInput(
	input modelbind.ModelInput[*models.MaterialStatus],
) error {
	if input.Model == nil {
		return webapp.BadRequest("material status input is required", nil)
	}
	if input.Model.ConstructionSiteID == nil || *input.Model.ConstructionSiteID <= 0 {
		return webapp.BadRequest(
			"construction_site_id is required for a new material status",
			nil,
		)
	}

	return nil
}

func validateGenericMaterialStatusUpdateInput(
	input modelbind.ModelInput[*models.MaterialStatus],
) error {
	if input.Model == nil {
		return webapp.BadRequest("material status input is required", nil)
	}
	if input.IsPresent("construction_site_id") &&
		(input.Model.ConstructionSiteID == nil || *input.Model.ConstructionSiteID <= 0) {
		return webapp.BadRequest(
			"construction_site_id cannot be cleared from a material status",
			nil,
		)
	}

	return nil
}
