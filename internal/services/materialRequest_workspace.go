package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/dronm/ds/v4"
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

func (s *MaterialRequestService) ConstructionManagerDetail(
	ctx context.Context,
	id int,
) (*models.MaterialRequestDocument, error) {
	user, err := s.constructionManagerMaterialRequestUser()
	if err != nil {
		return nil, err
	}
	if err := authorizeConstructionManagerMaterialRequestRole(user); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, webapp.BadRequest("material request id should be positive", nil)
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return nil, fmt.Errorf(
			"get primary connection for construction manager material request detail: %w",
			err,
		)
	}
	defer s.DB.Release(poolConn, connID)

	db := poolConn.Conn()
	if err := requireAssignedSiteMaterialRequestAccess(ctx, db, user, id); err != nil {
		return nil, err
	}

	document, err := fetchMaterialRequestDocument(ctx, db, id)
	if err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, materialRequestDocumentNotFound(id)
		}
		return nil, fmt.Errorf("fetch construction manager material request detail: %w", err)
	}
	return document, nil
}

func requireAssignedSiteMaterialRequestAccess(
	ctx context.Context,
	db ds.Querier,
	user models.UserLogin,
	id int,
) error {
	var available bool
	if err := db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM public.material_requests AS request
			JOIN public.construction_sites AS site
				ON site.id = request.construction_site_id
			WHERE request.id = $1
				AND (
					$2::boolean
					OR (
						site.is_active
						AND EXISTS (
							SELECT 1
							FROM public.user_construction_sites AS assignment
							WHERE assignment.user_id = $3
								AND assignment.construction_site_id = site.id
						)
					)
				)
		)
	`, id, user.RoleID == models.RoleIDAdmin, user.ID).Scan(&available); err != nil {
		return fmt.Errorf("check assigned-site material request access: %w", err)
	}
	if !available {
		return materialRequestDocumentNotFound(id)
	}
	return nil
}

func materialRequestDocumentNotFound(id int) error {
	return webapp.NotFound("material request not found", map[string]any{"id": id})
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
