package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dronm/ds/v4"
	"github.com/dronm/modelbind"
	"github.com/dronm/skstruktura/internal/apperrors"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
	wmodels "github.com/dronm/webapp/models"
)

const (
	supplyManagerAssignmentCreatePermission = "materialRequestSupplierAssignment.create"
	supplyManagerAssignmentListPermission   = "materialRequestSupplierAssignment.list"
	supplyManagerAssignmentDetailPermission = "materialRequestSupplierAssignment.detail"

	supplyManagerMaterialRequestDefaultPageSize = 30
	supplyManagerMaterialRequestMaxPageSize     = 100
	supplyManagerAssignmentDefaultPageSize      = 30
	supplyManagerAssignmentMaxPageSize          = 100
)

func (s *MaterialRequestSupplierAssignmentService) SupplyManagerSites(
	ctx context.Context,
) (models.SupplyManagerSitesResponse, error) {
	user, err := s.currentSupplyManagerUser()
	if err != nil {
		return models.SupplyManagerSitesResponse{}, err
	}
	if err := authorizeSupplyManagerRole(user, supplyManagerAssignmentCreatePermission); err != nil {
		return models.SupplyManagerSitesResponse{}, err
	}
	if err := s.requireDB(); err != nil {
		return models.SupplyManagerSitesResponse{}, err
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return models.SupplyManagerSitesResponse{}, fmt.Errorf(
			"get primary connection for supply manager sites: %w",
			err,
		)
	}
	defer s.DB.Release(poolConn, connID)

	rows, err := poolConn.Conn().Query(ctx, `
		SELECT site.id, site.name
		FROM public.construction_sites AS site
		WHERE site.is_active
			AND (
				$1::boolean
				OR EXISTS (
					SELECT 1
					FROM public.user_construction_sites AS assignment
					WHERE assignment.user_id = $2
						AND assignment.construction_site_id = site.id
				)
			)
		ORDER BY lower(site.name), site.id
	`, user.RoleID == models.RoleIDAdmin, user.ID)
	if err != nil {
		return models.SupplyManagerSitesResponse{}, fmt.Errorf(
			"select supply manager sites: %w",
			err,
		)
	}
	defer rows.Close()

	result := models.SupplyManagerSitesResponse{
		Rows: make([]*models.SupplyManagerSite, 0),
	}
	for rows.Next() {
		row := &models.SupplyManagerSite{}
		if err := rows.Scan(&row.ID, &row.Name); err != nil {
			return models.SupplyManagerSitesResponse{}, fmt.Errorf(
				"scan supply manager site: %w",
				err,
			)
		}
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return models.SupplyManagerSitesResponse{}, fmt.Errorf(
			"iterate supply manager sites: %w",
			err,
		)
	}
	result.Total = int64(len(result.Rows))

	return result, nil
}

func (s *MaterialRequestSupplierAssignmentService) SupplyManagerMaterialRequests(
	ctx context.Context,
	input models.SupplyManagerMaterialRequestInput,
) (wmodels.CollectionResponse[*models.MaterialRequestDocument], error) {
	user, err := s.currentSupplyManagerUser()
	if err != nil {
		return wmodels.CollectionResponse[*models.MaterialRequestDocument]{}, err
	}
	if err := authorizeSupplyManagerRole(user, supplyManagerAssignmentCreatePermission); err != nil {
		return wmodels.CollectionResponse[*models.MaterialRequestDocument]{}, err
	}
	if err := s.requireDB(); err != nil {
		return wmodels.CollectionResponse[*models.MaterialRequestDocument]{}, err
	}

	query, params, err := validateSupplyManagerMaterialRequestInput(input)
	if err != nil {
		return wmodels.CollectionResponse[*models.MaterialRequestDocument]{}, err
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return wmodels.CollectionResponse[*models.MaterialRequestDocument]{}, fmt.Errorf(
			"get primary connection for supply manager material requests: %w",
			err,
		)
	}
	defer s.DB.Release(poolConn, connID)

	db := poolConn.Conn()
	if query.ConstructionSiteID != nil {
		available, err := supplyManagerSiteAvailable(ctx, db, user, *query.ConstructionSiteID, true)
		if err != nil {
			return wmodels.CollectionResponse[*models.MaterialRequestDocument]{}, err
		}
		if !available {
			return wmodels.CollectionResponse[*models.MaterialRequestDocument]{},
				supplyManagerSiteForbidden(*query.ConstructionSiteID)
		}
	}

	requestIDs, total, err := fetchSupplyManagerMaterialRequestIDs(ctx, db, user, query, params)
	if err != nil {
		return wmodels.CollectionResponse[*models.MaterialRequestDocument]{}, err
	}
	requests, err := fetchSupplyManagerMaterialRequestDocuments(ctx, db, requestIDs)
	if err != nil {
		return wmodels.CollectionResponse[*models.MaterialRequestDocument]{}, err
	}

	return wmodels.CollectionResponse[*models.MaterialRequestDocument]{
		Rows: requests,
		Agg:  &wmodels.TotCount{TotCount: total},
	}, nil
}

func (s *MaterialRequestSupplierAssignmentService) SupplyManagerHistory(
	ctx context.Context,
	input models.SupplyManagerAssignmentHistoryInput,
) (wmodels.CollectionResponse[*models.SupplyManagerAssignmentHistoryRow], error) {
	user, err := s.currentSupplyManagerUser()
	if err != nil {
		return wmodels.CollectionResponse[*models.SupplyManagerAssignmentHistoryRow]{}, err
	}
	if err := authorizeSupplyManagerRole(user, supplyManagerAssignmentListPermission); err != nil {
		return wmodels.CollectionResponse[*models.SupplyManagerAssignmentHistoryRow]{}, err
	}
	if err := s.requireDB(); err != nil {
		return wmodels.CollectionResponse[*models.SupplyManagerAssignmentHistoryRow]{}, err
	}

	query, params, err := validateSupplyManagerAssignmentHistoryInput(input)
	if err != nil {
		return wmodels.CollectionResponse[*models.SupplyManagerAssignmentHistoryRow]{}, err
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return wmodels.CollectionResponse[*models.SupplyManagerAssignmentHistoryRow]{}, fmt.Errorf(
			"get primary connection for supply manager assignment history: %w",
			err,
		)
	}
	defer s.DB.Release(poolConn, connID)

	db := poolConn.Conn()
	if query.ConstructionSiteID != nil {
		available, err := supplyManagerSiteAvailable(ctx, db, user, *query.ConstructionSiteID, false)
		if err != nil {
			return wmodels.CollectionResponse[*models.SupplyManagerAssignmentHistoryRow]{}, err
		}
		if !available {
			return wmodels.CollectionResponse[*models.SupplyManagerAssignmentHistoryRow]{},
				supplyManagerSiteForbidden(*query.ConstructionSiteID)
		}
	}
	assignmentIDs, total, err := fetchSupplyManagerAssignmentHistoryIDs(ctx, db, user, query, params)
	if err != nil {
		return wmodels.CollectionResponse[*models.SupplyManagerAssignmentHistoryRow]{}, err
	}
	assignments, err := fetchSupplyManagerAssignments(ctx, db, assignmentIDs, user)
	if err != nil {
		return wmodels.CollectionResponse[*models.SupplyManagerAssignmentHistoryRow]{}, err
	}

	return wmodels.CollectionResponse[*models.SupplyManagerAssignmentHistoryRow]{
		Rows: assignments,
		Agg:  &wmodels.TotCount{TotCount: total},
	}, nil
}

func (s *MaterialRequestSupplierAssignmentService) SupplyManagerDetail(
	ctx context.Context,
	id int,
) (*models.SupplyManagerAssignmentHistoryRow, error) {
	user, err := s.currentSupplyManagerUser()
	if err != nil {
		return nil, err
	}
	if err := authorizeSupplyManagerRole(user, supplyManagerAssignmentDetailPermission); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, webapp.BadRequest("supplier assignment id should be positive", nil)
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return nil, fmt.Errorf("get primary connection for supply manager assignment detail: %w", err)
	}
	defer s.DB.Release(poolConn, connID)

	result, err := fetchSupplyManagerAssignment(ctx, poolConn.Conn(), id, user)
	if err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, webapp.NotFound("supplier assignment not found", map[string]any{"id": id})
		}
		return nil, fmt.Errorf("fetch supply manager assignment detail: %w", err)
	}
	return result, nil
}

func (s *MaterialRequestSupplierAssignmentService) currentSupplyManagerUser() (models.UserLogin, error) {
	if s.Session == nil {
		return models.UserLogin{}, apperrors.SessionRequired()
	}

	user := models.UserLogin{}
	if err := s.Session.Get("user", &user); err != nil || user.ID <= 0 {
		return models.UserLogin{}, apperrors.SessionRequired()
	}

	return user, nil
}

func authorizeSupplyManagerRole(user models.UserLogin, permission string) error {
	switch user.RoleID {
	case models.RoleIDAdmin, models.RoleIDSupplyManager:
		return nil
	default:
		return apperrors.Forbidden(permission)
	}
}

func validateSupplyManagerMaterialRequestInput(
	input models.SupplyManagerMaterialRequestInput,
) (*models.SupplyManagerMaterialRequestQuery, modelbind.CollectionParams, error) {
	if input.Query == nil {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest(
			"supply manager material request query is required",
			nil,
		)
	}

	query := *input.Query
	if query.ConstructionSiteID != nil && *query.ConstructionSiteID <= 0 {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest(
			"construction_site_id should be positive",
			nil,
		)
	}
	if query.DateFrom != nil && query.DateFrom.IsZero() {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest("date_from should be a timestamp", nil)
	}
	if query.DateTo != nil && query.DateTo.IsZero() {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest("date_to should be a timestamp", nil)
	}
	if query.DateFrom != nil && query.DateTo != nil && query.DateFrom.After(*query.DateTo) {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest(
			"date_from should not be after date_to",
			nil,
		)
	}
	if query.OrderImportanceID != nil && *query.OrderImportanceID <= 0 {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest(
			"order_importance_id should be positive",
			nil,
		)
	}
	if query.MaterialSearch != nil {
		materialSearch := strings.TrimSpace(*query.MaterialSearch)
		if materialSearch == "" {
			query.MaterialSearch = nil
		} else {
			query.MaterialSearch = &materialSearch
		}
	}

	params := input.Params
	if len(params.Filter) > 0 || len(params.Sorter) > 0 {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest(
			"generic filters and sorting are not supported for supply manager material requests",
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
		params.Count = supplyManagerMaterialRequestDefaultPageSize
	}
	if params.Count > supplyManagerMaterialRequestMaxPageSize {
		params.Count = supplyManagerMaterialRequestMaxPageSize
	}

	return &query, params, nil
}

func validateSupplyManagerAssignmentHistoryInput(
	input models.SupplyManagerAssignmentHistoryInput,
) (*models.SupplyManagerAssignmentHistoryQuery, modelbind.CollectionParams, error) {
	if input.Query == nil {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest(
			"supply manager assignment history query is required",
			nil,
		)
	}
	query := *input.Query
	if query.ConstructionSiteID != nil && *query.ConstructionSiteID <= 0 {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest(
			"construction_site_id should be positive",
			nil,
		)
	}

	params := input.Params
	if len(params.Filter) > 0 || len(params.Sorter) > 0 {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest(
			"generic filters and sorting are not supported for supply manager assignment history",
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
		params.Count = supplyManagerAssignmentDefaultPageSize
	}
	if params.Count > supplyManagerAssignmentMaxPageSize {
		params.Count = supplyManagerAssignmentMaxPageSize
	}

	return &query, params, nil
}

func supplyManagerSiteAvailable(
	ctx context.Context,
	db ds.Querier,
	user models.UserLogin,
	constructionSiteID int,
	requireActive bool,
) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM public.construction_sites AS site
			WHERE site.id = $1
				AND (NOT $2::boolean OR site.is_active)
	`
	args := []any{constructionSiteID, requireActive}
	if user.RoleID != models.RoleIDAdmin {
		query += `
				AND EXISTS (
					SELECT 1
					FROM public.user_construction_sites AS assignment
					WHERE assignment.user_id = $3
						AND assignment.construction_site_id = site.id
				)
		`
		args = append(args, user.ID)
	}
	query += `
		)
	`

	var available bool
	if err := db.QueryRow(ctx, query, args...).Scan(&available); err != nil {
		return false, fmt.Errorf("check supply manager construction site: %w", err)
	}
	return available, nil
}

func supplyManagerSiteForbidden(constructionSiteID int) error {
	return webapp.Forbidden(
		"construction site is not available to the current user",
		map[string]any{
			"code":                 apperrors.CodeForbidden,
			"construction_site_id": constructionSiteID,
		},
	)
}

func fetchSupplyManagerMaterialRequestIDs(
	ctx context.Context,
	db ds.Querier,
	user models.UserLogin,
	query *models.SupplyManagerMaterialRequestQuery,
	params modelbind.CollectionParams,
) ([]int, int, error) {
	args := supplyManagerMaterialRequestQueryArgs(user, query)

	var total int
	if err := db.QueryRow(ctx, `
		SELECT COUNT(*)::integer
		FROM public.material_requests AS request
		JOIN public.construction_sites AS site
			ON site.id = request.construction_site_id
		JOIN public.material_request_statuses AS request_status
			ON request_status.id = request.status_id
		WHERE site.is_active
			AND request_status.code = $8
			AND EXISTS (
				SELECT 1
				FROM public.material_request_items AS request_item
				WHERE request_item.material_request_id = request.id
			)
			AND NOT EXISTS (
				SELECT 1
				FROM public.material_request_items AS request_item
				JOIN public.material_request_statuses AS item_status
					ON item_status.id = request_item.status_id
				WHERE request_item.material_request_id = request.id
					AND (
						item_status.code <> $8
						OR EXISTS (
							SELECT 1
							FROM public.material_request_supplier_assignment_items AS assignment_item
							WHERE assignment_item.material_request_item_id = request_item.id
						)
					)
			)
			AND ($1::boolean OR EXISTS (
				SELECT 1
				FROM public.user_construction_sites AS assignment
				WHERE assignment.user_id = $2
					AND assignment.construction_site_id = request.construction_site_id
			))
			AND ($3::integer IS NULL OR request.construction_site_id = $3)
			AND ($4::timestamptz IS NULL OR request.date >= $4)
			AND ($5::timestamptz IS NULL OR request.date <= $5)
			AND (
				($6::text IS NULL AND $7::integer IS NULL)
				OR EXISTS (
					SELECT 1
					FROM public.material_request_items AS filter_item
					JOIN public.materials AS filter_material
						ON filter_material.id = filter_item.material_id
					WHERE filter_item.material_request_id = request.id
						AND ($6::text IS NULL OR filter_material.name ILIKE '%' || $6 || '%')
						AND ($7::integer IS NULL OR filter_item.order_importance_id = $7)
				)
			)
	`, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count supply manager material requests: %w", err)
	}

	rows, err := db.Query(ctx, `
		SELECT request.id
		FROM public.material_requests AS request
		JOIN public.construction_sites AS site
			ON site.id = request.construction_site_id
		JOIN public.material_request_statuses AS request_status
			ON request_status.id = request.status_id
		WHERE site.is_active
			AND request_status.code = $8
			AND EXISTS (
				SELECT 1
				FROM public.material_request_items AS request_item
				WHERE request_item.material_request_id = request.id
			)
			AND NOT EXISTS (
				SELECT 1
				FROM public.material_request_items AS request_item
				JOIN public.material_request_statuses AS item_status
					ON item_status.id = request_item.status_id
				WHERE request_item.material_request_id = request.id
					AND (
						item_status.code <> $8
						OR EXISTS (
							SELECT 1
							FROM public.material_request_supplier_assignment_items AS assignment_item
							WHERE assignment_item.material_request_item_id = request_item.id
						)
					)
			)
			AND ($1::boolean OR EXISTS (
				SELECT 1
				FROM public.user_construction_sites AS assignment
				WHERE assignment.user_id = $2
					AND assignment.construction_site_id = request.construction_site_id
			))
			AND ($3::integer IS NULL OR request.construction_site_id = $3)
			AND ($4::timestamptz IS NULL OR request.date >= $4)
			AND ($5::timestamptz IS NULL OR request.date <= $5)
			AND (
				($6::text IS NULL AND $7::integer IS NULL)
				OR EXISTS (
					SELECT 1
					FROM public.material_request_items AS filter_item
					JOIN public.materials AS filter_material
						ON filter_material.id = filter_item.material_id
					WHERE filter_item.material_request_id = request.id
						AND ($6::text IS NULL OR filter_material.name ILIKE '%' || $6 || '%')
						AND ($7::integer IS NULL OR filter_item.order_importance_id = $7)
				)
			)
		ORDER BY request.date, request.id
		LIMIT $9 OFFSET $10
	`, append(args, int(params.Count), int(params.From))...)
	if err != nil {
		return nil, 0, fmt.Errorf("select supply manager material request ids: %w", err)
	}
	defer rows.Close()

	requestIDs := make([]int, 0)
	for rows.Next() {
		var requestID int
		if err := rows.Scan(&requestID); err != nil {
			return nil, 0, fmt.Errorf("scan supply manager material request id: %w", err)
		}
		requestIDs = append(requestIDs, requestID)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate supply manager material request ids: %w", err)
	}

	return requestIDs, total, nil
}

func supplyManagerMaterialRequestQueryArgs(
	user models.UserLogin,
	query *models.SupplyManagerMaterialRequestQuery,
) []any {
	return []any{
		user.RoleID == models.RoleIDAdmin,
		user.ID,
		query.ConstructionSiteID,
		query.DateFrom,
		query.DateTo,
		query.MaterialSearch,
		query.OrderImportanceID,
		models.MaterialRequestStatusCodeNew,
	}
}

func fetchSupplyManagerMaterialRequestDocuments(
	ctx context.Context,
	db ds.Querier,
	requestIDs []int,
) ([]*models.MaterialRequestDocument, error) {
	result := make([]*models.MaterialRequestDocument, 0, len(requestIDs))
	if len(requestIDs) == 0 {
		return result, nil
	}

	rows, err := db.Query(ctx, `
		SELECT
			request.id,
			request.version,
			request.date,
			request.construction_site_id,
			request.construction_manager_id,
			request.comment,
			request.status_id,
			request.status,
			item.id,
			item.line_num,
			item.material_id,
			item.measure_unit_id,
			item.quant::double precision,
			item.supplier_id,
			item.required_date::text,
			item.order_importance_id,
			item.status_id,
			request.construction_site,
			request.construction_manager,
			item.material,
			item.measure_unit,
			item.supplier,
			item.order_importance,
			item.status
		FROM public.material_requests_list AS request
		LEFT JOIN public.material_request_items_list AS item
			ON item.material_request_id = request.id
		WHERE request.id = ANY($1::integer[])
		ORDER BY request.date, request.id, item.line_num, item.id
	`, requestIDs)
	if err != nil {
		return nil, fmt.Errorf("select nested supply manager material requests: %w", err)
	}
	defer rows.Close()

	documentsByID := make(map[int]*models.MaterialRequestDocument, len(requestIDs))
	for rows.Next() {
		var documentID, constructionSiteID, constructionManagerID, requestStatusID int
		var version int64
		var date time.Time
		var comment *string
		var itemID, lineNum, materialID, measureUnitID, orderImportanceID, itemStatusID *int
		var supplierID *int
		var requiredDate *string
		var quant *float64
		var constructionSite, constructionManager, requestStatus *models.Ref
		var material, measureUnit, supplier, orderImportance, itemStatus *models.Ref
		if err := rows.Scan(
			&documentID,
			&version,
			&date,
			&constructionSiteID,
			&constructionManagerID,
			&comment,
			&requestStatusID,
			&requestStatus,
			&itemID,
			&lineNum,
			&materialID,
			&measureUnitID,
			&quant,
			&supplierID,
			&requiredDate,
			&orderImportanceID,
			&itemStatusID,
			&constructionSite,
			&constructionManager,
			&material,
			&measureUnit,
			&supplier,
			&orderImportance,
			&itemStatus,
		); err != nil {
			return nil, fmt.Errorf("scan nested supply manager material request: %w", err)
		}

		document := documentsByID[documentID]
		if document == nil {
			document = &models.MaterialRequestDocument{
				ID:                    documentID,
				Version:               version,
				Date:                  date,
				ConstructionSiteID:    constructionSiteID,
				ConstructionManagerID: constructionManagerID,
				Comment:               comment,
				StatusID:              requestStatusID,
				Items:                 make([]*models.MaterialRequestDocumentItem, 0),
				ConstructionSite:      constructionSite,
				ConstructionManager:   constructionManager,
				Status:                requestStatus,
			}
			documentsByID[documentID] = document
		}
		if itemID == nil {
			continue
		}

		var itemRequiredDate *models.DateOnly
		if requiredDate != nil {
			value := models.DateOnly(*requiredDate)
			itemRequiredDate = &value
		}
		document.Items = append(document.Items, &models.MaterialRequestDocumentItem{
			ID:                *itemID,
			LineNum:           *lineNum,
			MaterialID:        *materialID,
			MeasureUnitID:     *measureUnitID,
			Quant:             *quant,
			SupplierID:        supplierID,
			RequiredDate:      itemRequiredDate,
			OrderImportanceID: *orderImportanceID,
			StatusID:          *itemStatusID,
			Material:          material,
			MeasureUnit:       measureUnit,
			Supplier:          supplier,
			OrderImportance:   orderImportance,
			Status:            itemStatus,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate nested supply manager material requests: %w", err)
	}

	for _, requestID := range requestIDs {
		if document := documentsByID[requestID]; document != nil {
			result = append(result, document)
		}
	}
	return result, nil
}

func fetchSupplyManagerAssignmentHistoryIDs(
	ctx context.Context,
	db ds.Querier,
	user models.UserLogin,
	query *models.SupplyManagerAssignmentHistoryQuery,
	params modelbind.CollectionParams,
) ([]int, int, error) {
	var total int
	if err := db.QueryRow(ctx, `
		SELECT COUNT(*)::integer
		FROM public.material_request_supplier_assignments AS assignment
		WHERE ($1::boolean
			OR (
				EXISTS (
					SELECT 1
					FROM public.material_request_supplier_assignment_items AS assignment_item
					WHERE assignment_item.material_request_supplier_assignment_id = assignment.id
				)
				AND NOT EXISTS (
					SELECT 1
					FROM public.material_request_supplier_assignment_items AS assignment_item
					JOIN public.material_request_items AS request_item
						ON request_item.id = assignment_item.material_request_item_id
					JOIN public.material_requests AS request
						ON request.id = request_item.material_request_id
					WHERE assignment_item.material_request_supplier_assignment_id = assignment.id
						AND NOT EXISTS (
							SELECT 1
							FROM public.user_construction_sites AS site_assignment
							WHERE site_assignment.user_id = $2
								AND site_assignment.construction_site_id = request.construction_site_id
						)
				)
			))
			AND ($3::integer IS NULL OR EXISTS (
				SELECT 1
				FROM public.material_request_supplier_assignment_items AS assignment_item
				JOIN public.material_request_items AS request_item
					ON request_item.id = assignment_item.material_request_item_id
				JOIN public.material_requests AS request
					ON request.id = request_item.material_request_id
				WHERE assignment_item.material_request_supplier_assignment_id = assignment.id
					AND request.construction_site_id = $3
			))
	`, user.RoleID == models.RoleIDAdmin, user.ID, query.ConstructionSiteID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count supply manager assignment history: %w", err)
	}

	rows, err := db.Query(ctx, `
		SELECT assignment.id
		FROM public.material_request_supplier_assignments AS assignment
		WHERE ($1::boolean
			OR (
				EXISTS (
					SELECT 1
					FROM public.material_request_supplier_assignment_items AS assignment_item
					WHERE assignment_item.material_request_supplier_assignment_id = assignment.id
				)
				AND NOT EXISTS (
					SELECT 1
					FROM public.material_request_supplier_assignment_items AS assignment_item
					JOIN public.material_request_items AS request_item
						ON request_item.id = assignment_item.material_request_item_id
					JOIN public.material_requests AS request
						ON request.id = request_item.material_request_id
					WHERE assignment_item.material_request_supplier_assignment_id = assignment.id
						AND NOT EXISTS (
							SELECT 1
							FROM public.user_construction_sites AS site_assignment
							WHERE site_assignment.user_id = $2
								AND site_assignment.construction_site_id = request.construction_site_id
						)
				)
			))
			AND ($3::integer IS NULL OR EXISTS (
				SELECT 1
				FROM public.material_request_supplier_assignment_items AS assignment_item
				JOIN public.material_request_items AS request_item
					ON request_item.id = assignment_item.material_request_item_id
				JOIN public.material_requests AS request
					ON request.id = request_item.material_request_id
				WHERE assignment_item.material_request_supplier_assignment_id = assignment.id
					AND request.construction_site_id = $3
			))
		ORDER BY assignment.date DESC, assignment.id DESC
		LIMIT $4 OFFSET $5
	`, user.RoleID == models.RoleIDAdmin, user.ID, query.ConstructionSiteID, int(params.Count), int(params.From))
	if err != nil {
		return nil, 0, fmt.Errorf("select supply manager assignment history ids: %w", err)
	}
	defer rows.Close()

	assignmentIDs := make([]int, 0)
	for rows.Next() {
		var assignmentID int
		if err := rows.Scan(&assignmentID); err != nil {
			return nil, 0, fmt.Errorf("scan supply manager assignment history id: %w", err)
		}
		assignmentIDs = append(assignmentIDs, assignmentID)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate supply manager assignment history ids: %w", err)
	}

	return assignmentIDs, total, nil
}

func fetchSupplyManagerAssignment(
	ctx context.Context,
	db ds.Querier,
	id int,
	user models.UserLogin,
) (*models.SupplyManagerAssignmentHistoryRow, error) {
	rows, err := fetchSupplyManagerAssignments(ctx, db, []int{id}, user)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ds.ErrNoRows
	}
	return rows[0], nil
}

func fetchSupplyManagerAssignments(
	ctx context.Context,
	db ds.Querier,
	assignmentIDs []int,
	user models.UserLogin,
) ([]*models.SupplyManagerAssignmentHistoryRow, error) {
	result := make([]*models.SupplyManagerAssignmentHistoryRow, 0, len(assignmentIDs))
	if len(assignmentIDs) == 0 {
		return result, nil
	}

	headerRows, err := db.Query(ctx, `
		SELECT
			assignment.id,
			assignment.date,
			assignment.supply_manager_id,
			assignment.comment,
			assignment.version,
			public.users_ref(supply_manager)
		FROM public.material_request_supplier_assignments AS assignment
		JOIN public.users AS supply_manager
			ON supply_manager.id = assignment.supply_manager_id
		WHERE assignment.id = ANY($1::integer[])
			AND (
				$2::boolean
				OR (
					EXISTS (
						SELECT 1
						FROM public.material_request_supplier_assignment_items AS assignment_item
						WHERE assignment_item.material_request_supplier_assignment_id = assignment.id
					)
					AND NOT EXISTS (
						SELECT 1
						FROM public.material_request_supplier_assignment_items AS assignment_item
						JOIN public.material_request_items AS request_item
							ON request_item.id = assignment_item.material_request_item_id
						JOIN public.material_requests AS request
							ON request.id = request_item.material_request_id
						WHERE assignment_item.material_request_supplier_assignment_id = assignment.id
							AND NOT EXISTS (
								SELECT 1
								FROM public.user_construction_sites AS site_assignment
								WHERE site_assignment.user_id = $3
									AND site_assignment.construction_site_id = request.construction_site_id
							)
					)
				)
			)
	`, assignmentIDs, user.RoleID == models.RoleIDAdmin, user.ID)
	if err != nil {
		return nil, fmt.Errorf("select supply manager assignment headers: %w", err)
	}

	assignmentsByID := make(map[int]*models.SupplyManagerAssignmentHistoryRow, len(assignmentIDs))
	for headerRows.Next() {
		row := &models.SupplyManagerAssignmentHistoryRow{
			Items: make([]*models.SupplyManagerAssignmentHistoryItem, 0),
		}
		if err := headerRows.Scan(
			&row.ID,
			&row.Date,
			&row.SupplyManagerID,
			&row.Comment,
			&row.Version,
			&row.SupplyManager,
		); err != nil {
			_ = headerRows.Close()
			return nil, fmt.Errorf("scan supply manager assignment header: %w", err)
		}
		assignmentsByID[row.ID] = row
	}
	if err := headerRows.Err(); err != nil {
		_ = headerRows.Close()
		return nil, fmt.Errorf("iterate supply manager assignment headers: %w", err)
	}
	if err := headerRows.Close(); err != nil {
		return nil, fmt.Errorf("close supply manager assignment headers: %w", err)
	}

	visibleIDs := make([]int, 0, len(assignmentIDs))
	for _, assignmentID := range assignmentIDs {
		if assignment := assignmentsByID[assignmentID]; assignment != nil {
			visibleIDs = append(visibleIDs, assignmentID)
			result = append(result, assignment)
		}
	}
	if len(visibleIDs) == 0 {
		return result, nil
	}

	itemRows, err := db.Query(ctx, `
		SELECT
			assignment_item.id,
			assignment_item.line_num,
			assignment_item.material_request_supplier_assignment_id,
			assignment_item.material_request_item_id,
			request_item.material_request_id,
			request.date,
			request.construction_site_id,
			request_item.material_id,
			request_item.measure_unit_id,
			request_item.quant::double precision,
			assignment_item.supplier_id,
			request_item.required_date::text,
			request_item.order_importance_id,
			public.construction_sites_ref(site),
			public.materials_ref(material),
			public.measure_units_ref(measure_unit),
			public.suppliers_ref(supplier),
			public.order_importances_ref(order_importance)
		FROM public.material_request_supplier_assignment_items AS assignment_item
		JOIN public.material_request_items AS request_item
			ON request_item.id = assignment_item.material_request_item_id
		JOIN public.material_requests AS request
			ON request.id = request_item.material_request_id
		JOIN public.construction_sites AS site
			ON site.id = request.construction_site_id
		JOIN public.materials AS material
			ON material.id = request_item.material_id
		JOIN public.measure_units AS measure_unit
			ON measure_unit.id = request_item.measure_unit_id
		JOIN public.suppliers AS supplier
			ON supplier.id = assignment_item.supplier_id
		JOIN public.order_importances AS order_importance
			ON order_importance.id = request_item.order_importance_id
		WHERE assignment_item.material_request_supplier_assignment_id = ANY($1::integer[])
		ORDER BY
			assignment_item.material_request_supplier_assignment_id,
			assignment_item.line_num,
			assignment_item.id
	`, visibleIDs)
	if err != nil {
		return nil, fmt.Errorf("select supply manager assignment items: %w", err)
	}
	defer itemRows.Close()

	for itemRows.Next() {
		row := &models.SupplyManagerAssignmentHistoryItem{}
		var requiredDate *string
		if err := itemRows.Scan(
			&row.ID,
			&row.LineNum,
			&row.MaterialRequestSupplierAssignmentID,
			&row.MaterialRequestItemID,
			&row.MaterialRequestID,
			&row.RequestDate,
			&row.ConstructionSiteID,
			&row.MaterialID,
			&row.MeasureUnitID,
			&row.Quant,
			&row.SupplierID,
			&requiredDate,
			&row.OrderImportanceID,
			&row.ConstructionSite,
			&row.Material,
			&row.MeasureUnit,
			&row.Supplier,
			&row.OrderImportance,
		); err != nil {
			return nil, fmt.Errorf("scan supply manager assignment item: %w", err)
		}
		if requiredDate != nil {
			value := models.DateOnly(*requiredDate)
			row.RequiredDate = &value
		}
		assignment := assignmentsByID[row.MaterialRequestSupplierAssignmentID]
		if assignment != nil {
			assignment.Items = append(assignment.Items, row)
		}
	}
	if err := itemRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate supply manager assignment items: %w", err)
	}

	return result, nil
}
