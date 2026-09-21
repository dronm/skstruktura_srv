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

func (s *MaterialReceiptService) Create(
	ctx context.Context,
	document *models.MaterialReceiptDocument,
) (*models.MaterialReceiptDocument, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if err := validateMaterialReceiptDocument(document, true, 0); err != nil {
		return nil, err
	}

	var result *models.MaterialReceiptDocument
	if err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		if err := tx.QueryRow(ctx, `
			INSERT INTO public.material_receipts (
				date,
				construction_site_id,
				supplier_id,
				number,
				comment
			)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, version
		`,
			document.Date,
			document.ConstructionSiteID,
			document.SupplierID,
			document.Number,
			document.Comment,
		).Scan(&document.ID, &document.Version); err != nil {
			return err
		}
		if err := syncMaterialReceiptItems(ctx, tx, document.ID, document.Items); err != nil {
			return err
		}
		if err := rebuildMaterialRegisterActions(
			ctx,
			tx,
			materialReceiptRecorderType,
			document.ID,
		); err != nil {
			return err
		}

		var err error
		result, err = fetchMaterialReceiptDocument(ctx, tx, document.ID)
		return err
	}); err != nil {
		return nil, fmt.Errorf("create complete material receipt: %w", err)
	}

	return result, nil
}

func (s *MaterialReceiptService) Update(
	ctx context.Context,
	input models.UpdateMaterialReceiptDocumentRequest,
) (*models.MaterialReceiptDocument, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if err := validateMaterialReceiptDocument(input.Document, false, input.ID); err != nil {
		return nil, err
	}

	document := input.Document
	document.ID = input.ID
	var result *models.MaterialReceiptDocument
	if err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		if err := lockMaterialRecorders(ctx, tx, materialReceiptRecorderType, document.ID); err != nil {
			return err
		}

		var currentVersion int64
		if err := tx.QueryRow(ctx, `
			SELECT version
			FROM public.material_receipts
			WHERE id = $1
			FOR UPDATE
		`, document.ID).Scan(&currentVersion); err != nil {
			if errors.Is(err, ds.ErrNoRows) {
				return webapp.NotFound("material receipt not found", map[string]any{"id": document.ID})
			}
			return err
		}
		if currentVersion != document.Version {
			return webapp.Conflict(
				"material receipt was changed by another request",
				map[string]any{
					"id":               document.ID,
					"expected_version": document.Version,
					"current_version":  currentVersion,
				},
			)
		}

		if err := tx.QueryRow(ctx, `
			UPDATE public.material_receipts
			SET
				date = $2,
				construction_site_id = $3,
				supplier_id = $4,
				number = $5,
				comment = $6,
				version = version + 1
			WHERE id = $1
			RETURNING version
		`,
			document.ID,
			document.Date,
			document.ConstructionSiteID,
			document.SupplierID,
			document.Number,
			document.Comment,
		).Scan(&document.Version); err != nil {
			return err
		}
		if err := syncMaterialReceiptItems(ctx, tx, document.ID, document.Items); err != nil {
			return err
		}
		if err := rebuildMaterialRegisterActions(
			ctx,
			tx,
			materialReceiptRecorderType,
			document.ID,
		); err != nil {
			return err
		}

		var err error
		result, err = fetchMaterialReceiptDocument(ctx, tx, document.ID)
		return err
	}); err != nil {
		return nil, fmt.Errorf("update complete material receipt: %w", err)
	}

	return result, nil
}

func (s *MaterialReceiptService) DocumentDetail(
	ctx context.Context,
	id int,
) (*models.MaterialReceiptDocument, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, webapp.BadRequest("material receipt id is required", nil)
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return nil, fmt.Errorf("get primary connection for material receipt detail: %w", err)
	}
	defer s.DB.Release(poolConn, connID)

	result, err := fetchMaterialReceiptDocument(ctx, poolConn.Conn(), id)
	if err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, webapp.NotFound("material receipt not found", map[string]any{"id": id})
		}
		return nil, fmt.Errorf("fetch complete material receipt: %w", err)
	}
	return result, nil
}

func (s *MaterialReceiptService) Delete(
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
		return wmodels.RowsAffectedResponse{}, webapp.BadRequest("material receipt id is required", nil)
	}

	var rowsAffected int64
	if err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		if err := lockMaterialRecorders(ctx, tx, materialReceiptRecorderType, id); err != nil {
			return err
		}
		if err := removeMaterialRegisterActions(ctx, tx, materialReceiptRecorderType, id); err != nil {
			return err
		}

		result, err := tx.Exec(ctx, "DELETE FROM public.material_receipts WHERE id = $1", id)
		if err != nil {
			return err
		}
		rowsAffected = result.RowsAffected()

		return nil
	}); err != nil {
		return wmodels.RowsAffectedResponse{}, fmt.Errorf("delete material receipt: %w", err)
	}
	if rowsAffected == 0 {
		return wmodels.RowsAffectedResponse{}, webapp.NotFound(
			"material receipt not found",
			map[string]any{"id": id},
		)
	}

	return wmodels.RowsAffectedResponse{RowsAffected: rowsAffected}, nil
}
