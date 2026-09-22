package services

import (
	"context"
	"fmt"

	"github.com/dronm/modelbind"
	"github.com/dronm/skstruktura/internal/apperrors"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
	wmodels "github.com/dronm/webapp/models"
)

const (
	constructionManagerMaterialRequestDefaultPageSize = 30
	constructionManagerMaterialRequestMaxPageSize     = 100
)

func (s *MaterialRequestService) ConstructionManagerList(
	ctx context.Context,
	input models.ConstructionManagerMaterialRequestInput,
) (wmodels.CollectionResponse[*models.MaterialRequestList], error) {
	user, err := s.constructionManagerMaterialRequestUser()
	if err != nil {
		return wmodels.CollectionResponse[*models.MaterialRequestList]{}, err
	}
	if err := authorizeConstructionManagerMaterialRequestRole(user); err != nil {
		return wmodels.CollectionResponse[*models.MaterialRequestList]{}, err
	}
	if err := s.requireDB(); err != nil {
		return wmodels.CollectionResponse[*models.MaterialRequestList]{}, err
	}

	query, params, err := validateConstructionManagerMaterialRequestInput(input)
	if err != nil {
		return wmodels.CollectionResponse[*models.MaterialRequestList]{}, err
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return wmodels.CollectionResponse[*models.MaterialRequestList]{}, fmt.Errorf(
			"get primary connection for construction manager material requests: %w",
			err,
		)
	}
	defer s.DB.Release(poolConn, connID)

	db := poolConn.Conn()
	available, err := materialBalanceSiteAvailable(ctx, db, user, query.ConstructionSiteID)
	if err != nil {
		return wmodels.CollectionResponse[*models.MaterialRequestList]{}, err
	}
	if !available {
		return wmodels.CollectionResponse[*models.MaterialRequestList]{}, webapp.Forbidden(
			"construction site is not available to the current user",
			map[string]any{
				"code":                 apperrors.CodeForbidden,
				"construction_site_id": query.ConstructionSiteID,
			},
		)
	}

	total := 0
	if err := db.QueryRow(ctx, `
		SELECT COUNT(*)::integer
		FROM public.material_requests_list
		WHERE construction_site_id = $1
	`, query.ConstructionSiteID).Scan(&total); err != nil {
		return wmodels.CollectionResponse[*models.MaterialRequestList]{}, fmt.Errorf(
			"count construction manager material requests: %w",
			err,
		)
	}

	rows, err := db.Query(ctx, `
		SELECT
			id,
			date,
			construction_site_id,
			construction_manager_id,
			comment,
			version,
			construction_site,
			construction_manager,
			status_id,
			status
		FROM public.material_requests_list
		WHERE construction_site_id = $1
		ORDER BY date DESC, id DESC
		LIMIT $2 OFFSET $3
	`, query.ConstructionSiteID, int(params.Count), int(params.From))
	if err != nil {
		return wmodels.CollectionResponse[*models.MaterialRequestList]{}, fmt.Errorf(
			"select construction manager material requests: %w",
			err,
		)
	}
	defer rows.Close()

	result := wmodels.CollectionResponse[*models.MaterialRequestList]{
		Rows: make([]*models.MaterialRequestList, 0),
		Agg:  &wmodels.TotCount{TotCount: total},
	}
	for rows.Next() {
		row := &models.MaterialRequestList{}
		if err := rows.Scan(
			&row.ID,
			&row.Date,
			&row.ConstructionSiteID,
			&row.ConstructionManagerID,
			&row.Comment,
			&row.Version,
			&row.ConstructionSite,
			&row.ConstructionManager,
			&row.StatusID,
			&row.Status,
		); err != nil {
			return wmodels.CollectionResponse[*models.MaterialRequestList]{}, fmt.Errorf(
				"scan construction manager material request: %w",
				err,
			)
		}
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return wmodels.CollectionResponse[*models.MaterialRequestList]{}, fmt.Errorf(
			"iterate construction manager material requests: %w",
			err,
		)
	}

	return result, nil
}

func (s *MaterialRequestService) constructionManagerMaterialRequestUser() (models.UserLogin, error) {
	if s.Session == nil {
		return models.UserLogin{}, apperrors.SessionRequired()
	}

	user := models.UserLogin{}
	if err := s.Session.Get("user", &user); err != nil || user.ID <= 0 {
		return models.UserLogin{}, apperrors.SessionRequired()
	}

	return user, nil
}

func authorizeConstructionManagerMaterialRequestRole(user models.UserLogin) error {
	switch user.RoleID {
	case models.RoleIDAdmin, models.RoleIDConstructionSiteManager:
		return nil
	default:
		return apperrors.Forbidden("materialRequest.list")
	}
}

func validateConstructionManagerMaterialRequestInput(
	input models.ConstructionManagerMaterialRequestInput,
) (*models.ConstructionManagerMaterialRequestQuery, modelbind.CollectionParams, error) {
	if input.Query == nil {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest(
			"construction manager material request query is required",
			nil,
		)
	}
	if input.Query.ConstructionSiteID <= 0 {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest(
			"construction_site_id should be positive",
			nil,
		)
	}

	params := input.Params
	if len(params.Filter) > 0 || len(params.Sorter) > 0 {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest(
			"generic filters and sorting are not supported for construction manager material requests",
			nil,
		)
	}
	if params.From < 0 {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest("from should not be negative", nil)
	}
	if params.Count < 0 {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest("count should not be negative", nil)
	}
	if params.Count == 0 {
		params.Count = constructionManagerMaterialRequestDefaultPageSize
	}
	if params.Count > constructionManagerMaterialRequestMaxPageSize {
		params.Count = constructionManagerMaterialRequestMaxPageSize
	}

	query := *input.Query
	return &query, params, nil
}
