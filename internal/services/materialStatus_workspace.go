package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dronm/ds/v4"
	"github.com/dronm/modelbind"
	"github.com/dronm/skstruktura/internal/apperrors"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
	wmodels "github.com/dronm/webapp/models"
)

const (
	constructionManagerMaterialStatusListPermission   = "constructionManager.materialStatus.list"
	constructionManagerMaterialStatusCreatePermission = "constructionManager.materialStatus.create"

	constructionManagerMaterialStatusDefaultPageSize = 30
	constructionManagerMaterialStatusMaxPageSize     = 100
	constructionManagerMaterialStatusFutureAllowance = 5 * time.Minute
)

type constructionManagerMaterialStatusLockedMaterial struct {
	Name             string
	MaterialTypeID   int
	MaterialTypeName string
}

type constructionManagerMaterialStatusLatest struct {
	ID        *int
	CreatedAt *time.Time
	Status    models.MaterialStatusType
}

func (s *MaterialStatusService) ConstructionManagerCurrent(
	ctx context.Context,
	input models.ConstructionManagerMaterialStatusInput,
) (wmodels.CollectionResponse[*models.ConstructionManagerMaterialStatusCurrentRow], error) {
	user, err := s.constructionManagerMaterialStatusUser()
	if err != nil {
		return wmodels.CollectionResponse[*models.ConstructionManagerMaterialStatusCurrentRow]{}, err
	}
	if err := authorizeConstructionManagerMaterialStatusRole(
		user,
		constructionManagerMaterialStatusListPermission,
	); err != nil {
		return wmodels.CollectionResponse[*models.ConstructionManagerMaterialStatusCurrentRow]{}, err
	}
	if err := s.requireDB(); err != nil {
		return wmodels.CollectionResponse[*models.ConstructionManagerMaterialStatusCurrentRow]{}, err
	}

	query, params, err := validateConstructionManagerMaterialStatusInput(input)
	if err != nil {
		return wmodels.CollectionResponse[*models.ConstructionManagerMaterialStatusCurrentRow]{}, err
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return wmodels.CollectionResponse[*models.ConstructionManagerMaterialStatusCurrentRow]{}, fmt.Errorf(
			"get primary connection for current construction manager material statuses: %w",
			err,
		)
	}
	defer s.DB.Release(poolConn, connID)

	db := poolConn.Conn()
	available, err := materialBalanceSiteAvailable(ctx, db, user, query.ConstructionSiteID)
	if err != nil {
		return wmodels.CollectionResponse[*models.ConstructionManagerMaterialStatusCurrentRow]{}, err
	}
	if !available {
		return wmodels.CollectionResponse[*models.ConstructionManagerMaterialStatusCurrentRow]{},
			constructionManagerMaterialStatusSiteForbidden(query.ConstructionSiteID)
	}

	var total int
	if err := db.QueryRow(ctx, `
		SELECT COUNT(*)::integer
		FROM public.materials AS material
		JOIN public.material_types AS material_type
			ON material_type.id = material.material_type_id
		JOIN public.rg_materials_current AS balance
			ON balance.construction_site_id = $1
			AND balance.material_id = material.id
		WHERE material.is_active
			AND balance.quant > 0
			AND ($2::integer IS NULL OR material.material_type_id = $2)
	`, query.ConstructionSiteID, query.MaterialTypeID).Scan(&total); err != nil {
		return wmodels.CollectionResponse[*models.ConstructionManagerMaterialStatusCurrentRow]{}, fmt.Errorf(
			"count current construction manager material statuses: %w",
			err,
		)
	}

	rows, err := db.Query(ctx, `
		SELECT
			material.id,
			material.name,
			material.material_type_id,
			material_type.name,
			COALESCE(latest.status, 'at_work'::public.material_status_types),
			latest.id,
			latest.created_at
		FROM public.materials AS material
		JOIN public.material_types AS material_type
			ON material_type.id = material.material_type_id
		JOIN public.rg_materials_current AS balance
			ON balance.construction_site_id = $1
			AND balance.material_id = material.id
		LEFT JOIN LATERAL (
			SELECT status_row.id, status_row.created_at, status_row.status
			FROM public.material_statuses AS status_row
			WHERE status_row.material_id = material.id
				AND status_row.is_active
			ORDER BY status_row.created_at DESC, status_row.id DESC
			LIMIT 1
		) AS latest ON true
		WHERE material.is_active
			AND balance.quant > 0
			AND ($2::integer IS NULL OR material.material_type_id = $2)
		ORDER BY lower(material.name), material.id
		LIMIT $3 OFFSET $4
	`, query.ConstructionSiteID, query.MaterialTypeID, int(params.Count), int(params.From))
	if err != nil {
		return wmodels.CollectionResponse[*models.ConstructionManagerMaterialStatusCurrentRow]{}, fmt.Errorf(
			"select current construction manager material statuses: %w",
			err,
		)
	}
	defer rows.Close()

	result := wmodels.CollectionResponse[*models.ConstructionManagerMaterialStatusCurrentRow]{
		Rows: make([]*models.ConstructionManagerMaterialStatusCurrentRow, 0),
		Agg:  &wmodels.TotCount{TotCount: total},
	}
	for rows.Next() {
		row := &models.ConstructionManagerMaterialStatusCurrentRow{}
		if err := rows.Scan(
			&row.MaterialID,
			&row.MaterialName,
			&row.MaterialTypeID,
			&row.MaterialTypeName,
			&row.Status,
			&row.StatusRecordID,
			&row.StatusChangedAt,
		); err != nil {
			return wmodels.CollectionResponse[*models.ConstructionManagerMaterialStatusCurrentRow]{}, fmt.Errorf(
				"scan current construction manager material status: %w",
				err,
			)
		}
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return wmodels.CollectionResponse[*models.ConstructionManagerMaterialStatusCurrentRow]{}, fmt.Errorf(
			"iterate current construction manager material statuses: %w",
			err,
		)
	}

	return result, nil
}

func (s *MaterialStatusService) ConstructionManagerHistory(
	ctx context.Context,
	input models.ConstructionManagerMaterialStatusInput,
) (wmodels.CollectionResponse[*models.ConstructionManagerMaterialStatusHistoryRow], error) {
	user, err := s.constructionManagerMaterialStatusUser()
	if err != nil {
		return wmodels.CollectionResponse[*models.ConstructionManagerMaterialStatusHistoryRow]{}, err
	}
	if err := authorizeConstructionManagerMaterialStatusRole(
		user,
		constructionManagerMaterialStatusListPermission,
	); err != nil {
		return wmodels.CollectionResponse[*models.ConstructionManagerMaterialStatusHistoryRow]{}, err
	}
	if err := s.requireDB(); err != nil {
		return wmodels.CollectionResponse[*models.ConstructionManagerMaterialStatusHistoryRow]{}, err
	}

	query, params, err := validateConstructionManagerMaterialStatusInput(input)
	if err != nil {
		return wmodels.CollectionResponse[*models.ConstructionManagerMaterialStatusHistoryRow]{}, err
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return wmodels.CollectionResponse[*models.ConstructionManagerMaterialStatusHistoryRow]{}, fmt.Errorf(
			"get primary connection for construction manager material status history: %w",
			err,
		)
	}
	defer s.DB.Release(poolConn, connID)

	db := poolConn.Conn()
	available, err := materialBalanceSiteAvailable(ctx, db, user, query.ConstructionSiteID)
	if err != nil {
		return wmodels.CollectionResponse[*models.ConstructionManagerMaterialStatusHistoryRow]{}, err
	}
	if !available {
		return wmodels.CollectionResponse[*models.ConstructionManagerMaterialStatusHistoryRow]{},
			constructionManagerMaterialStatusSiteForbidden(query.ConstructionSiteID)
	}

	var total int
	if err := db.QueryRow(ctx, `
		SELECT COUNT(*)::integer
		FROM public.material_statuses AS status_row
		JOIN public.materials AS material ON material.id = status_row.material_id
		JOIN public.material_types AS material_type
			ON material_type.id = material.material_type_id
		WHERE status_row.is_active
			AND status_row.construction_site_id = $1
			AND ($2::integer IS NULL OR material.material_type_id = $2)
	`, query.ConstructionSiteID, query.MaterialTypeID).Scan(&total); err != nil {
		return wmodels.CollectionResponse[*models.ConstructionManagerMaterialStatusHistoryRow]{}, fmt.Errorf(
			"count construction manager material status history: %w",
			err,
		)
	}

	rows, err := db.Query(ctx, `
		SELECT
			status_row.id,
			status_row.created_at,
			status_row.construction_site_id,
			material.id,
			material.name,
			material.material_type_id,
			material_type.name,
			status_row.status
		FROM public.material_statuses AS status_row
		JOIN public.materials AS material ON material.id = status_row.material_id
		JOIN public.material_types AS material_type
			ON material_type.id = material.material_type_id
		WHERE status_row.is_active
			AND status_row.construction_site_id = $1
			AND ($2::integer IS NULL OR material.material_type_id = $2)
		ORDER BY status_row.created_at DESC, status_row.id DESC
		LIMIT $3 OFFSET $4
	`, query.ConstructionSiteID, query.MaterialTypeID, int(params.Count), int(params.From))
	if err != nil {
		return wmodels.CollectionResponse[*models.ConstructionManagerMaterialStatusHistoryRow]{}, fmt.Errorf(
			"select construction manager material status history: %w",
			err,
		)
	}
	defer rows.Close()

	result := wmodels.CollectionResponse[*models.ConstructionManagerMaterialStatusHistoryRow]{
		Rows: make([]*models.ConstructionManagerMaterialStatusHistoryRow, 0),
		Agg:  &wmodels.TotCount{TotCount: total},
	}
	for rows.Next() {
		row := &models.ConstructionManagerMaterialStatusHistoryRow{}
		if err := rows.Scan(
			&row.ID,
			&row.CreatedAt,
			&row.ConstructionSiteID,
			&row.MaterialID,
			&row.MaterialName,
			&row.MaterialTypeID,
			&row.MaterialTypeName,
			&row.Status,
		); err != nil {
			return wmodels.CollectionResponse[*models.ConstructionManagerMaterialStatusHistoryRow]{}, fmt.Errorf(
				"scan construction manager material status history: %w",
				err,
			)
		}
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return wmodels.CollectionResponse[*models.ConstructionManagerMaterialStatusHistoryRow]{}, fmt.Errorf(
			"iterate construction manager material status history: %w",
			err,
		)
	}

	return result, nil
}

func (s *MaterialStatusService) ConstructionManagerCreate(
	ctx context.Context,
	request *models.ConstructionManagerMaterialStatusChangeRequest,
) (*models.ConstructionManagerMaterialStatusHistoryRow, error) {
	user, err := s.constructionManagerMaterialStatusUser()
	if err != nil {
		return nil, err
	}
	if err := authorizeConstructionManagerMaterialStatusRole(
		user,
		constructionManagerMaterialStatusCreatePermission,
	); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if err := validateConstructionManagerMaterialStatusChange(request, time.Now().UTC()); err != nil {
		return nil, err
	}

	var result *models.ConstructionManagerMaterialStatusHistoryRow
	if err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		available, err := materialBalanceSiteAvailable(ctx, tx, user, request.ConstructionSiteID)
		if err != nil {
			return err
		}
		if !available {
			return constructionManagerMaterialStatusSiteForbidden(request.ConstructionSiteID)
		}

		material, err := lockConstructionManagerMaterialStatusMaterial(
			ctx,
			tx,
			request.ConstructionSiteID,
			request.MaterialID,
		)
		if err != nil {
			return err
		}

		// This lookup deliberately runs after the row locks in a separate statement.
		// Under READ COMMITTED it observes a status change committed while this request
		// was waiting for the material lock.
		latest, err := fetchConstructionManagerMaterialStatusLatest(ctx, tx, request.MaterialID)
		if err != nil {
			return err
		}
		if err := validateConstructionManagerMaterialStatusTransition(request, latest); err != nil {
			return err
		}
		if err := validateConstructionManagerMaterialStatusHistoricalBalance(ctx, tx, request); err != nil {
			return err
		}

		row := &models.ConstructionManagerMaterialStatusHistoryRow{
			CreatedAt:          request.CreatedAt,
			ConstructionSiteID: request.ConstructionSiteID,
			MaterialID:         request.MaterialID,
			MaterialName:       material.Name,
			MaterialTypeID:     material.MaterialTypeID,
			MaterialTypeName:   material.MaterialTypeName,
			Status:             request.TargetStatus,
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO public.material_statuses (
				created_at,
				material_id,
				construction_site_id,
				status,
				is_active
			)
			VALUES ($1, $2, $3, $4, true)
			RETURNING id, created_at
		`,
			request.CreatedAt,
			request.MaterialID,
			request.ConstructionSiteID,
			request.TargetStatus,
		).Scan(&row.ID, &row.CreatedAt); err != nil {
			return fmt.Errorf("insert construction manager material status: %w", err)
		}

		result = row
		return nil
	}); err != nil {
		return nil, fmt.Errorf("create construction manager material status: %w", err)
	}

	return result, nil
}

func (s *MaterialStatusService) constructionManagerMaterialStatusUser() (models.UserLogin, error) {
	if s.Session == nil {
		return models.UserLogin{}, apperrors.SessionRequired()
	}

	user := models.UserLogin{}
	if err := s.Session.Get("user", &user); err != nil || user.ID <= 0 {
		return models.UserLogin{}, apperrors.SessionRequired()
	}

	return user, nil
}

func authorizeConstructionManagerMaterialStatusRole(user models.UserLogin, permission string) error {
	switch user.RoleID {
	case models.RoleIDAdmin, models.RoleIDConstructionSiteManager:
		return nil
	default:
		return apperrors.Forbidden(permission)
	}
}

func validateConstructionManagerMaterialStatusInput(
	input models.ConstructionManagerMaterialStatusInput,
) (*models.ConstructionManagerMaterialStatusQuery, modelbind.CollectionParams, error) {
	if input.Query == nil {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest(
			"construction manager material status query is required",
			nil,
		)
	}
	if input.Query.ConstructionSiteID <= 0 {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest(
			"construction_site_id should be positive",
			nil,
		)
	}
	if input.Query.MaterialTypeID != nil && *input.Query.MaterialTypeID <= 0 {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest(
			"material_type_id should be positive",
			nil,
		)
	}

	params := input.Params
	if len(params.Filter) > 0 || len(params.Sorter) > 0 {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest(
			"generic filters and sorting are not supported for construction manager material statuses",
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
		params.Count = constructionManagerMaterialStatusDefaultPageSize
	}
	if params.Count > constructionManagerMaterialStatusMaxPageSize {
		params.Count = constructionManagerMaterialStatusMaxPageSize
	}

	query := *input.Query
	return &query, params, nil
}

func validateConstructionManagerMaterialStatusChange(
	request *models.ConstructionManagerMaterialStatusChangeRequest,
	now time.Time,
) error {
	if request == nil {
		return webapp.BadRequest("material status change request is required", nil)
	}
	if request.ConstructionSiteID <= 0 {
		return webapp.BadRequest("construction_site_id should be positive", nil)
	}
	if request.MaterialID <= 0 {
		return webapp.BadRequest("material_id should be positive", nil)
	}
	if request.CreatedAt.IsZero() {
		return webapp.BadRequest("created_at is required", nil)
	}
	if request.CreatedAt.After(now.Add(constructionManagerMaterialStatusFutureAllowance)) {
		return webapp.BadRequest(
			"created_at is too far in the future",
			map[string]any{"maximum_created_at": now.Add(constructionManagerMaterialStatusFutureAllowance)},
		)
	}
	if request.ExpectedStatusRecordID != nil && *request.ExpectedStatusRecordID <= 0 {
		return webapp.BadRequest("expected_status_record_id should be positive when provided", nil)
	}
	if !request.ExpectedStatus.IsValid() {
		return webapp.BadRequest("expected_status is invalid", map[string]any{"status": request.ExpectedStatus})
	}
	if !request.TargetStatus.IsValid() {
		return webapp.BadRequest("target_status is invalid", map[string]any{"status": request.TargetStatus})
	}
	if request.TargetStatus != reverseMaterialStatus(request.ExpectedStatus) {
		return webapp.BadRequest(
			"target_status must be the reverse of expected_status",
			map[string]any{
				"expected_status": request.ExpectedStatus,
				"target_status":   request.TargetStatus,
			},
		)
	}

	return nil
}

func lockConstructionManagerMaterialStatusMaterial(
	ctx context.Context,
	tx ds.Querier,
	constructionSiteID int,
	materialID int,
) (constructionManagerMaterialStatusLockedMaterial, error) {
	result := constructionManagerMaterialStatusLockedMaterial{}
	if err := tx.QueryRow(ctx, `
		SELECT material.name, material.material_type_id, material_type.name
		FROM public.materials AS material
		JOIN public.material_types AS material_type
			ON material_type.id = material.material_type_id
		JOIN public.rg_materials_current AS balance
			ON balance.construction_site_id = $1
			AND balance.material_id = material.id
		WHERE material.id = $2
			AND material.is_active
			AND balance.quant > 0
		FOR UPDATE OF material, balance
	`, constructionSiteID, materialID).Scan(
		&result.Name,
		&result.MaterialTypeID,
		&result.MaterialTypeName,
	); err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return constructionManagerMaterialStatusLockedMaterial{}, webapp.BadRequest(
				"material is inactive or has no positive balance at the construction site",
				map[string]any{
					"construction_site_id": constructionSiteID,
					"material_id":          materialID,
				},
			)
		}
		return constructionManagerMaterialStatusLockedMaterial{}, fmt.Errorf(
			"lock construction manager material status material: %w",
			err,
		)
	}

	return result, nil
}

func fetchConstructionManagerMaterialStatusLatest(
	ctx context.Context,
	tx ds.Querier,
	materialID int,
) (constructionManagerMaterialStatusLatest, error) {
	result := constructionManagerMaterialStatusLatest{Status: models.MaterialStatusTypeAtWork}
	var id int
	var createdAt time.Time
	var status models.MaterialStatusType
	err := tx.QueryRow(ctx, `
		SELECT status_row.id, status_row.created_at, status_row.status
		FROM public.material_statuses AS status_row
		WHERE status_row.material_id = $1
			AND status_row.is_active
		ORDER BY status_row.created_at DESC, status_row.id DESC
		LIMIT 1
	`, materialID).Scan(&id, &createdAt, &status)
	if errors.Is(err, ds.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return constructionManagerMaterialStatusLatest{}, fmt.Errorf(
			"fetch latest construction manager material status: %w",
			err,
		)
	}

	result.ID = &id
	result.CreatedAt = &createdAt
	result.Status = status
	return result, nil
}

func validateConstructionManagerMaterialStatusTransition(
	request *models.ConstructionManagerMaterialStatusChangeRequest,
	latest constructionManagerMaterialStatusLatest,
) error {
	if !equalOptionalInt(request.ExpectedStatusRecordID, latest.ID) ||
		request.ExpectedStatus != latest.Status {
		return webapp.Conflict(
			"material status was changed by another request",
			map[string]any{
				"material_id":               request.MaterialID,
				"expected_status_record_id": request.ExpectedStatusRecordID,
				"current_status_record_id":  latest.ID,
				"expected_status":           request.ExpectedStatus,
				"current_status":            latest.Status,
			},
		)
	}
	if request.TargetStatus != reverseMaterialStatus(latest.Status) {
		return webapp.BadRequest(
			"target_status must reverse the current material status",
			map[string]any{
				"current_status": latest.Status,
				"target_status":  request.TargetStatus,
			},
		)
	}
	if latest.CreatedAt != nil && request.CreatedAt.Before(*latest.CreatedAt) {
		return webapp.BadRequest(
			"created_at cannot precede the latest material status",
			map[string]any{
				"created_at":               request.CreatedAt,
				"latest_status_created_at": latest.CreatedAt,
			},
		)
	}

	return nil
}

func validateConstructionManagerMaterialStatusHistoricalBalance(
	ctx context.Context,
	tx ds.Querier,
	request *models.ConstructionManagerMaterialStatusChangeRequest,
) error {
	var balance float64
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE((
			SELECT historical.quant::double precision
			FROM public.rg_materials_balance(
				$1::timestamptz,
				ARRAY[$2]::integer[],
				ARRAY[$3]::integer[]
			) AS historical
			WHERE historical.construction_site_id = $2
				AND historical.material_id = $3
			LIMIT 1
		), 0::double precision)
	`, request.CreatedAt, request.ConstructionSiteID, request.MaterialID).Scan(&balance); err != nil {
		return fmt.Errorf("check historical material balance for status change: %w", err)
	}
	if balance <= 0 {
		return webapp.BadRequest(
			"material had no positive balance at the construction site at created_at",
			map[string]any{
				"construction_site_id": request.ConstructionSiteID,
				"material_id":          request.MaterialID,
				"created_at":           request.CreatedAt,
			},
		)
	}

	return nil
}

func reverseMaterialStatus(status models.MaterialStatusType) models.MaterialStatusType {
	if status == models.MaterialStatusTypeAtWork {
		return models.MaterialStatusTypeOnMaintenance
	}
	if status == models.MaterialStatusTypeOnMaintenance {
		return models.MaterialStatusTypeAtWork
	}
	return ""
}

func equalOptionalInt(left *int, right *int) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func constructionManagerMaterialStatusSiteForbidden(constructionSiteID int) error {
	return webapp.Forbidden(
		"construction site is unavailable for the current construction manager",
		map[string]any{
			"permission":           constructionManagerMaterialStatusListPermission,
			"construction_site_id": constructionSiteID,
		},
	)
}
