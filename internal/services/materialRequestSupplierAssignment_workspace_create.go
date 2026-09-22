package services

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/dronm/ds/v4"
	"github.com/dronm/skstruktura/internal/apperrors"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

const supplyManagerAssignmentMaxItems = 1000

type validatedSupplyManagerAssignment struct {
	requestIDs      []int
	requestVersions map[int]int64
	itemIDs         []int
	supplierIDs     []int
	itemsByID       map[int]int
}

func (s *MaterialRequestSupplierAssignmentService) SupplyManagerCreate(
	ctx context.Context,
	request *models.SupplyManagerCreateAssignmentRequest,
) (*models.SupplyManagerAssignmentHistoryRow, error) {
	user, err := s.currentSupplyManagerUser()
	if err != nil {
		return nil, err
	}
	if err := authorizeSupplyManagerRole(user, "materialRequestSupplierAssignment.create"); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}

	validated, err := validateSupplyManagerCreateAssignmentRequest(request)
	if err != nil {
		return nil, err
	}

	var result *models.SupplyManagerAssignmentHistoryRow
	if err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		newStatusID, err := materialRequestStatusID(ctx, tx, models.MaterialRequestStatusCodeNew)
		if err != nil {
			return err
		}
		supplierAssignedStatusID, err := materialRequestStatusID(
			ctx,
			tx,
			models.MaterialRequestStatusCodeSupplierAssigned,
		)
		if err != nil {
			return err
		}

		if err := lockAndValidateSupplyManagerRequests(ctx, tx, user, validated); err != nil {
			return err
		}
		if err := lockAndValidateSupplyManagerRequestItems(ctx, tx, validated); err != nil {
			return err
		}
		if err := validateActiveAssignmentSuppliers(ctx, tx, validated.supplierIDs); err != nil {
			return err
		}

		var assignmentID int
		if err := tx.QueryRow(ctx, `
			INSERT INTO public.material_request_supplier_assignments (
				date,
				supply_manager_id,
				comment
			)
			VALUES ($1, $2, $3)
			RETURNING id
		`, request.Date, user.ID, request.Comment).Scan(&assignmentID); err != nil {
			return err
		}

		for index, item := range request.Items {
			if _, err := tx.Exec(ctx, `
				INSERT INTO public.material_request_supplier_assignment_items (
					line_num,
					material_request_supplier_assignment_id,
					material_request_item_id,
					supplier_id
				)
				VALUES ($1, $2, $3, $4)
			`, index+1, assignmentID, item.MaterialRequestItemID, item.SupplierID); err != nil {
				return err
			}
		}

		updatedItems, err := tx.Exec(ctx, `
			WITH submitted AS (
				SELECT input.material_request_item_id, input.supplier_id
				FROM unnest($1::integer[], $2::integer[]) AS input(
					material_request_item_id,
					supplier_id
				)
			)
			UPDATE public.material_request_items AS item
			SET
				supplier_id = submitted.supplier_id,
				status_id = $3
			FROM submitted
			WHERE item.id = submitted.material_request_item_id
				AND item.status_id = $4
		`, validated.itemIDs, validated.supplierIDs, supplierAssignedStatusID, newStatusID)
		if err != nil {
			return err
		}
		if updatedItems.RowsAffected() != int64(len(validated.itemIDs)) {
			return webapp.Conflict(
				"material request lines changed while suppliers were being assigned",
				map[string]any{
					"expected_item_count": len(validated.itemIDs),
					"updated_item_count":  updatedItems.RowsAffected(),
				},
			)
		}

		updatedRequests, err := tx.Exec(ctx, `
			UPDATE public.material_requests
			SET
				status_id = $2,
				version = version + 1
			WHERE id = ANY($1::integer[])
				AND status_id = $3
		`, validated.requestIDs, supplierAssignedStatusID, newStatusID)
		if err != nil {
			return err
		}
		if updatedRequests.RowsAffected() != int64(len(validated.requestIDs)) {
			return webapp.Conflict(
				"material requests changed while suppliers were being assigned",
				map[string]any{
					"expected_request_count": len(validated.requestIDs),
					"updated_request_count":  updatedRequests.RowsAffected(),
				},
			)
		}

		result, err = fetchSupplyManagerAssignment(ctx, tx, assignmentID, user)
		return err
	}); err != nil {
		return nil, fmt.Errorf("create material request supplier assignment: %w", err)
	}

	return result, nil
}

func validateSupplyManagerCreateAssignmentRequest(
	request *models.SupplyManagerCreateAssignmentRequest,
) (*validatedSupplyManagerAssignment, error) {
	if request == nil {
		return nil, webapp.BadRequest("supplier assignment request is required", nil)
	}
	if request.Date.IsZero() {
		return nil, webapp.BadRequest("supplier assignment date is required", nil)
	}
	if len(request.Requests) == 0 {
		return nil, webapp.BadRequest("supplier assignment should contain at least one material request", nil)
	}
	if len(request.Requests) > supplyManagerAssignmentMaxItems {
		return nil, webapp.BadRequest(
			"supplier assignment contains too many material requests",
			map[string]any{"maximum": supplyManagerAssignmentMaxItems, "count": len(request.Requests)},
		)
	}
	if len(request.Items) == 0 {
		return nil, webapp.BadRequest("supplier assignment should contain at least one item", nil)
	}
	if len(request.Items) > supplyManagerAssignmentMaxItems {
		return nil, webapp.BadRequest(
			"supplier assignment contains too many items",
			map[string]any{"maximum": supplyManagerAssignmentMaxItems, "count": len(request.Items)},
		)
	}

	if request.Comment != nil {
		trimmed := strings.TrimSpace(*request.Comment)
		if trimmed == "" {
			request.Comment = nil
		} else {
			request.Comment = &trimmed
		}
	}

	result := &validatedSupplyManagerAssignment{
		requestIDs:      make([]int, 0, len(request.Requests)),
		requestVersions: make(map[int]int64, len(request.Requests)),
		itemIDs:         make([]int, 0, len(request.Items)),
		supplierIDs:     make([]int, 0, len(request.Items)),
		itemsByID:       make(map[int]int, len(request.Items)),
	}
	for index, requestRef := range request.Requests {
		if requestRef == nil {
			return nil, invalidSupplyManagerAssignmentEntry("requests", index, "request is required")
		}
		if requestRef.ID <= 0 {
			return nil, invalidSupplyManagerAssignmentEntry("requests", index, "id should be positive")
		}
		if requestRef.Version <= 0 {
			return nil, invalidSupplyManagerAssignmentEntry("requests", index, "version should be positive")
		}
		if _, exists := result.requestVersions[requestRef.ID]; exists {
			return nil, invalidSupplyManagerAssignmentEntry("requests", index, "id is duplicated")
		}
		result.requestIDs = append(result.requestIDs, requestRef.ID)
		result.requestVersions[requestRef.ID] = requestRef.Version
	}

	for index, item := range request.Items {
		if item == nil {
			return nil, invalidSupplyManagerAssignmentEntry("items", index, "item is required")
		}
		if item.MaterialRequestItemID <= 0 {
			return nil, invalidSupplyManagerAssignmentEntry(
				"items",
				index,
				"material_request_item_id should be positive",
			)
		}
		if item.SupplierID <= 0 {
			return nil, invalidSupplyManagerAssignmentEntry("items", index, "supplier_id should be positive")
		}
		if _, exists := result.itemsByID[item.MaterialRequestItemID]; exists {
			return nil, invalidSupplyManagerAssignmentEntry(
				"items",
				index,
				"material_request_item_id is duplicated",
			)
		}
		result.itemIDs = append(result.itemIDs, item.MaterialRequestItemID)
		result.supplierIDs = append(result.supplierIDs, item.SupplierID)
		result.itemsByID[item.MaterialRequestItemID] = item.SupplierID
	}

	sort.Ints(result.requestIDs)
	return result, nil
}

func invalidSupplyManagerAssignmentEntry(field string, index int, message string) error {
	return webapp.BadRequest(
		fmt.Sprintf("%s[%d]: %s", field, index, message),
		map[string]any{"field": field, "index": index},
	)
}

func lockAndValidateSupplyManagerRequests(
	ctx context.Context,
	tx ds.Querier,
	user models.UserLogin,
	validated *validatedSupplyManagerAssignment,
) error {
	rows, err := tx.Query(ctx, `
		SELECT
			request.id,
			request.version,
			request.construction_site_id,
			site.is_active,
			status.code,
			EXISTS (
				SELECT 1
				FROM public.user_construction_sites AS site_assignment
				WHERE site_assignment.user_id = $2
					AND site_assignment.construction_site_id = request.construction_site_id
			) AS site_available
		FROM public.material_requests AS request
		JOIN public.construction_sites AS site
			ON site.id = request.construction_site_id
		JOIN public.material_request_statuses AS status
			ON status.id = request.status_id
		WHERE request.id = ANY($1::integer[])
			AND (
				$3::boolean
				OR EXISTS (
					SELECT 1
					FROM public.user_construction_sites AS site_assignment
					WHERE site_assignment.user_id = $2
						AND site_assignment.construction_site_id = request.construction_site_id
				)
			)
		ORDER BY request.id
		FOR UPDATE OF request
		FOR SHARE OF site
	`, validated.requestIDs, user.ID, user.RoleID == models.RoleIDAdmin)
	if err != nil {
		return err
	}
	defer rows.Close()

	found := make(map[int]struct{}, len(validated.requestIDs))
	constructionSiteIDs := make(map[int]struct{}, len(validated.requestIDs))
	for rows.Next() {
		var id, constructionSiteID int
		var version int64
		var siteActive bool
		var status string
		var siteAvailable bool
		if err := rows.Scan(
			&id,
			&version,
			&constructionSiteID,
			&siteActive,
			&status,
			&siteAvailable,
		); err != nil {
			return err
		}
		found[id] = struct{}{}
		constructionSiteIDs[constructionSiteID] = struct{}{}
		if !siteActive {
			return webapp.Conflict(
				"supplier assignments cannot be submitted for an inactive construction site",
				map[string]any{
					"material_request_id":  id,
					"construction_site_id": constructionSiteID,
				},
			)
		}
		if user.RoleID == models.RoleIDSupplyManager && !siteAvailable {
			return webapp.Forbidden(
				"construction site is not available to the current supply manager",
				map[string]any{
					"code":                 apperrors.CodeForbidden,
					"material_request_id":  id,
					"construction_site_id": constructionSiteID,
				},
			)
		}
		if status != models.MaterialRequestStatusCodeNew {
			return webapp.Conflict(
				"only a new material request can receive supplier assignments",
				map[string]any{"material_request_id": id, "status_code": status},
			)
		}
		if expected := validated.requestVersions[id]; version != expected {
			return materialRequestVersionConflict(id, expected, version)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if len(found) != len(validated.requestIDs) {
		missing := make([]int, 0)
		for _, id := range validated.requestIDs {
			if _, exists := found[id]; !exists {
				missing = append(missing, id)
			}
		}
		return webapp.NotFound("material request not found", map[string]any{"ids": missing})
	}
	if user.RoleID == models.RoleIDSupplyManager {
		if err := lockSupplyManagerSiteAssignments(
			ctx,
			tx,
			user.ID,
			constructionSiteIDs,
		); err != nil {
			return err
		}
	}

	return nil
}

func lockSupplyManagerSiteAssignments(
	ctx context.Context,
	tx ds.Querier,
	userID int,
	constructionSiteIDSet map[int]struct{},
) error {
	constructionSiteIDs := make([]int, 0, len(constructionSiteIDSet))
	for constructionSiteID := range constructionSiteIDSet {
		constructionSiteIDs = append(constructionSiteIDs, constructionSiteID)
	}
	sort.Ints(constructionSiteIDs)

	rows, err := tx.Query(ctx, `
		SELECT construction_site_id
		FROM public.user_construction_sites
		WHERE user_id = $1
			AND construction_site_id = ANY($2::integer[])
		ORDER BY construction_site_id
		FOR SHARE
	`, userID, constructionSiteIDs)
	if err != nil {
		return err
	}
	defer rows.Close()

	locked := make(map[int]struct{}, len(constructionSiteIDs))
	for rows.Next() {
		var constructionSiteID int
		if err := rows.Scan(&constructionSiteID); err != nil {
			return err
		}
		locked[constructionSiteID] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, constructionSiteID := range constructionSiteIDs {
		if _, exists := locked[constructionSiteID]; !exists {
			return webapp.Forbidden(
				"construction site is not available to the current supply manager",
				map[string]any{
					"code":                 apperrors.CodeForbidden,
					"construction_site_id": constructionSiteID,
				},
			)
		}
	}

	return nil
}

func lockAndValidateSupplyManagerRequestItems(
	ctx context.Context,
	tx ds.Querier,
	validated *validatedSupplyManagerAssignment,
) error {
	rows, err := tx.Query(ctx, `
		SELECT
			item.id,
			item.material_request_id,
			item.supplier_id,
			status.code,
			EXISTS (
				SELECT 1
				FROM public.material_request_supplier_assignment_items AS assignment_item
				WHERE assignment_item.material_request_item_id = item.id
			) AS already_assigned
		FROM public.material_request_items AS item
		JOIN public.material_request_statuses AS status
			ON status.id = item.status_id
		WHERE item.material_request_id = ANY($1::integer[])
		ORDER BY item.material_request_id, item.id
		FOR UPDATE OF item
	`, validated.requestIDs)
	if err != nil {
		return err
	}
	defer rows.Close()

	foundItems := make(map[int]struct{}, len(validated.itemIDs))
	requestItemCounts := make(map[int]int, len(validated.requestIDs))
	for rows.Next() {
		var itemID, requestID int
		var supplierID *int
		var status string
		var alreadyAssigned bool
		if err := rows.Scan(&itemID, &requestID, &supplierID, &status, &alreadyAssigned); err != nil {
			return err
		}
		requestItemCounts[requestID]++
		if _, submitted := validated.itemsByID[itemID]; !submitted {
			return webapp.Conflict(
				"supplier assignment should include every line of each material request",
				map[string]any{"material_request_id": requestID, "material_request_item_id": itemID},
			)
		}
		foundItems[itemID] = struct{}{}
		if status != models.MaterialRequestStatusCodeNew {
			return webapp.Conflict(
				"only a new material request line can receive a supplier assignment",
				map[string]any{"material_request_item_id": itemID, "status_code": status},
			)
		}
		if alreadyAssigned {
			return webapp.Conflict(
				"material request line already has a supplier assignment",
				map[string]any{"material_request_item_id": itemID},
			)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, requestID := range validated.requestIDs {
		if requestItemCounts[requestID] == 0 {
			return webapp.Conflict(
				"material request without items cannot receive supplier assignments",
				map[string]any{"material_request_id": requestID},
			)
		}
	}
	if len(foundItems) != len(validated.itemsByID) {
		return webapp.Conflict(
			"supplier assignment items do not match the selected material requests",
			map[string]any{
				"expected_item_count":  len(foundItems),
				"submitted_item_count": len(validated.itemsByID),
			},
		)
	}

	return nil
}

func validateActiveAssignmentSuppliers(
	ctx context.Context,
	tx ds.Querier,
	supplierIDs []int,
) error {
	unique := make(map[int]struct{}, len(supplierIDs))
	for _, supplierID := range supplierIDs {
		unique[supplierID] = struct{}{}
	}
	ids := make([]int, 0, len(unique))
	for supplierID := range unique {
		ids = append(ids, supplierID)
	}
	sort.Ints(ids)

	rows, err := tx.Query(ctx, `
		SELECT id
		FROM public.suppliers
		WHERE id = ANY($1::integer[])
			AND is_active
		ORDER BY id
		FOR SHARE
	`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()

	active := make(map[int]struct{}, len(ids))
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return err
		}
		active[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		if _, exists := active[id]; !exists {
			return webapp.BadRequest(
				"supplier_id should reference an active supplier",
				map[string]any{"supplier_id": id},
			)
		}
	}

	return nil
}
