package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/dronm/ds/v4"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
	wmodels "github.com/dronm/webapp/models"
)

func (s *MaterialRequestService) Create(
	ctx context.Context,
	document *models.MaterialRequestDocument,
) (*models.MaterialRequestDocument, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if err := validateMaterialRequestDocument(document, true, 0); err != nil {
		return nil, err
	}

	var result *models.MaterialRequestDocument
	if err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		if err := prepareMaterialRequestReferences(ctx, tx, document); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO public.material_requests (
				date,
				construction_site_id,
				construction_manager_id,
				comment
			)
			VALUES ($1, $2, $3, $4)
			RETURNING id, version
		`,
			document.Date,
			document.ConstructionSiteID,
			document.ConstructionManagerID,
			document.Comment,
		).Scan(&document.ID, &document.Version); err != nil {
			return err
		}
		if err := syncMaterialRequestItems(ctx, tx, document.ID, document.Items); err != nil {
			return err
		}

		var err error
		result, err = fetchMaterialRequestDocument(ctx, tx, document.ID)
		return err
	}); err != nil {
		return nil, fmt.Errorf("create complete material request: %w", err)
	}

	return result, nil
}

func (s *MaterialRequestService) Update(
	ctx context.Context,
	input models.UpdateMaterialRequestDocumentRequest,
) (*models.MaterialRequestDocument, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if err := validateMaterialRequestDocument(input.Document, false, input.ID); err != nil {
		return nil, err
	}

	document := input.Document
	document.ID = input.ID
	var result *models.MaterialRequestDocument
	if err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		var currentVersion int64
		var currentStatus string
		if err := tx.QueryRow(ctx, `
			SELECT request.version, status.code
			FROM public.material_requests AS request
			JOIN public.material_request_statuses AS status
				ON status.id = request.status_id
			WHERE request.id = $1
			FOR UPDATE OF request
		`, document.ID).Scan(&currentVersion, &currentStatus); err != nil {
			if errors.Is(err, ds.ErrNoRows) {
				return webapp.NotFound("material request not found", map[string]any{"id": document.ID})
			}
			return err
		}
		if currentStatus != models.MaterialRequestStatusCodeDraft {
			return materialRequestNotDraftConflict(document.ID, currentStatus, "updated")
		}
		if currentVersion != document.Version {
			return materialRequestVersionConflict(document.ID, document.Version, currentVersion)
		}
		draftStatusID, err := materialRequestStatusID(ctx, tx, models.MaterialRequestStatusCodeDraft)
		if err != nil {
			return err
		}
		for index, item := range document.Items {
			if item.SupplierID != nil {
				return invalidMaterialDocumentItem(
					index,
					"supplier_id is managed by the supply manager workspace",
				)
			}
			if item.ID == 0 {
				item.StatusID = draftStatusID
				continue
			}
			if item.StatusID != draftStatusID {
				return invalidMaterialDocumentItem(
					index,
					"status_id is managed by the material request workflow",
				)
			}
		}
		if err := prepareMaterialRequestReferences(ctx, tx, document); err != nil {
			return err
		}

		if err := tx.QueryRow(ctx, `
			UPDATE public.material_requests
			SET
				date = $2,
				construction_site_id = $3,
				construction_manager_id = $4,
				comment = $5,
				version = version + 1
			WHERE id = $1
			RETURNING version
		`,
			document.ID,
			document.Date,
			document.ConstructionSiteID,
			document.ConstructionManagerID,
			document.Comment,
		).Scan(&document.Version); err != nil {
			return err
		}
		if err := syncMaterialRequestItems(ctx, tx, document.ID, document.Items); err != nil {
			return err
		}

		result, err = fetchMaterialRequestDocument(ctx, tx, document.ID)
		return err
	}); err != nil {
		return nil, fmt.Errorf("update complete material request: %w", err)
	}

	return result, nil
}

func (s *MaterialRequestService) DocumentDetail(
	ctx context.Context,
	id int,
) (*models.MaterialRequestDocument, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, webapp.BadRequest("material request id is required", nil)
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return nil, fmt.Errorf("get primary connection for material request detail: %w", err)
	}
	defer s.DB.Release(poolConn, connID)

	result, err := fetchMaterialRequestDocument(ctx, poolConn.Conn(), id)
	if err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, webapp.NotFound("material request not found", map[string]any{"id": id})
		}
		return nil, fmt.Errorf("fetch complete material request: %w", err)
	}
	return result, nil
}

func (s *MaterialRequestService) Submit(
	ctx context.Context,
	input models.SubmitMaterialRequestInput,
) (*models.MaterialRequestDocument, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if input.ID <= 0 {
		return nil, webapp.BadRequest("material request id is required", nil)
	}
	if input.Request == nil || input.Request.Version <= 0 {
		return nil, webapp.BadRequest("material request version should be positive", nil)
	}

	var result *models.MaterialRequestDocument
	if err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		var currentVersion int64
		if err := tx.QueryRow(ctx, `
			SELECT version
			FROM public.material_requests
			WHERE id = $1
			FOR UPDATE
		`, input.ID).Scan(&currentVersion); err != nil {
			if errors.Is(err, ds.ErrNoRows) {
				return webapp.NotFound("material request not found", map[string]any{"id": input.ID})
			}
			return err
		}
		if currentVersion != input.Request.Version {
			return materialRequestVersionConflict(input.ID, input.Request.Version, currentVersion)
		}

		draftStatusID, err := materialRequestStatusID(ctx, tx, models.MaterialRequestStatusCodeDraft)
		if err != nil {
			return err
		}
		newStatusID, err := materialRequestStatusID(ctx, tx, models.MaterialRequestStatusCodeNew)
		if err != nil {
			return err
		}

		var itemCount, draftItemCount int
		if err := tx.QueryRow(ctx, `
			SELECT
				count(*)::integer,
				count(*) FILTER (WHERE status_id = $2)::integer
			FROM public.material_request_items
			WHERE material_request_id = $1
		`, input.ID, draftStatusID).Scan(&itemCount, &draftItemCount); err != nil {
			return err
		}
		if itemCount == 0 {
			return webapp.Conflict(
				"material request without items cannot be submitted",
				map[string]any{"id": input.ID},
			)
		}
		if draftItemCount != itemCount {
			return webapp.Conflict(
				"only a material request whose items are all drafts can be submitted",
				map[string]any{
					"id":               input.ID,
					"item_count":       itemCount,
					"draft_item_count": draftItemCount,
				},
			)
		}

		if _, err := tx.Exec(ctx, `
			UPDATE public.material_request_items
			SET status_id = $2
			WHERE material_request_id = $1
				AND status_id = $3
		`, input.ID, newStatusID, draftStatusID); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `
			UPDATE public.material_requests
			SET
				status_id = $2,
				version = version + 1
			WHERE id = $1
			RETURNING version
		`, input.ID, newStatusID).Scan(&currentVersion); err != nil {
			return err
		}

		result, err = fetchMaterialRequestDocument(ctx, tx, input.ID)
		return err
	}); err != nil {
		return nil, fmt.Errorf("submit material request: %w", err)
	}

	return result, nil
}

func (s *MaterialRequestService) Delete(
	ctx context.Context,
	id int,
) (wmodels.RowsAffectedResponse, error) {
	if err := s.requireSession(); err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}
	if err := s.requireDB(); err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}
	if id <= 0 {
		return wmodels.RowsAffectedResponse{}, webapp.BadRequest("material request id is required", nil)
	}

	var rowsAffected int64
	if err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		var currentStatus string
		if err := tx.QueryRow(ctx, `
			SELECT status.code
			FROM public.material_requests AS request
			JOIN public.material_request_statuses AS status
				ON status.id = request.status_id
			WHERE request.id = $1
			FOR UPDATE OF request
		`, id).Scan(&currentStatus); err != nil {
			if errors.Is(err, ds.ErrNoRows) {
				return webapp.NotFound("material request not found", map[string]any{"id": id})
			}
			return err
		}
		if currentStatus != models.MaterialRequestStatusCodeDraft {
			return materialRequestNotDraftConflict(id, currentStatus, "deleted")
		}

		result, err := tx.Exec(ctx, "DELETE FROM public.material_requests WHERE id = $1", id)
		if err != nil {
			return err
		}
		rowsAffected = result.RowsAffected()
		return nil
	}); err != nil {
		return wmodels.RowsAffectedResponse{}, fmt.Errorf("delete material request: %w", err)
	}
	if rowsAffected == 0 {
		return wmodels.RowsAffectedResponse{}, webapp.NotFound(
			"material request not found",
			map[string]any{"id": id},
		)
	}

	return wmodels.RowsAffectedResponse{RowsAffected: rowsAffected}, nil
}

func materialRequestVersionConflict(id int, expected int64, current int64) error {
	return webapp.Conflict(
		"material request was changed by another request",
		map[string]any{
			"id":               id,
			"expected_version": expected,
			"current_version":  current,
		},
	)
}

func materialRequestNotDraftConflict(id int, status string, operation string) error {
	return webapp.Conflict(
		fmt.Sprintf("only a draft material request can be %s", operation),
		map[string]any{
			"id":          id,
			"status_code": status,
		},
	)
}
