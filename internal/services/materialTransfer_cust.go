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

func (s *MaterialTransferService) Create(
	ctx context.Context,
	document *models.MaterialTransferDocument,
) (*models.MaterialTransferDocument, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if err := validateMaterialTransferDocument(document, true, 0); err != nil {
		return nil, err
	}

	var result *models.MaterialTransferDocument
	if err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		var err error
		result, err = createMaterialTransferDocument(ctx, tx, document)
		return err
	}); err != nil {
		return nil, fmt.Errorf("create complete material transfer: %w", err)
	}

	return result, nil
}

func createMaterialTransferDocument(
	ctx context.Context,
	tx ds.Querier,
	document *models.MaterialTransferDocument,
) (*models.MaterialTransferDocument, error) {
	if err := tx.QueryRow(ctx, `
		INSERT INTO public.material_transfers (
			date,
			source_construction_site_id,
			destination_construction_site_id,
			comment
		)
		VALUES ($1, $2, $3, $4)
		RETURNING id, version
	`,
		document.Date,
		document.SourceConstructionSiteID,
		document.DestinationConstructionSiteID,
		document.Comment,
	).Scan(&document.ID, &document.Version); err != nil {
		return nil, err
	}
	if err := syncMaterialTransferItems(ctx, tx, document.ID, document.Items); err != nil {
		return nil, err
	}
	if err := rebuildMaterialRegisterActions(
		ctx,
		tx,
		materialTransferRecorderType,
		document.ID,
	); err != nil {
		return nil, err
	}

	return fetchMaterialTransferDocument(ctx, tx, document.ID)
}

func (s *MaterialTransferService) Update(
	ctx context.Context,
	input models.UpdateMaterialTransferDocumentRequest,
) (*models.MaterialTransferDocument, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if err := validateMaterialTransferDocument(input.Document, false, input.ID); err != nil {
		return nil, err
	}

	document := input.Document
	document.ID = input.ID
	var result *models.MaterialTransferDocument
	if err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		if err := lockMaterialRecorders(ctx, tx, materialTransferRecorderType, document.ID); err != nil {
			return err
		}

		var currentVersion int64
		if err := tx.QueryRow(ctx, `
			SELECT version
			FROM public.material_transfers
			WHERE id = $1
			FOR UPDATE
		`, document.ID).Scan(&currentVersion); err != nil {
			if errors.Is(err, ds.ErrNoRows) {
				return webapp.NotFound("material transfer not found", map[string]any{"id": document.ID})
			}
			return err
		}
		if currentVersion != document.Version {
			return webapp.Conflict(
				"material transfer was changed by another request",
				map[string]any{
					"id":               document.ID,
					"expected_version": document.Version,
					"current_version":  currentVersion,
				},
			)
		}

		if err := tx.QueryRow(ctx, `
			UPDATE public.material_transfers
			SET
				date = $2,
				source_construction_site_id = $3,
				destination_construction_site_id = $4,
				comment = $5,
				version = version + 1
			WHERE id = $1
			RETURNING version
		`,
			document.ID,
			document.Date,
			document.SourceConstructionSiteID,
			document.DestinationConstructionSiteID,
			document.Comment,
		).Scan(&document.Version); err != nil {
			return err
		}
		if err := syncMaterialTransferItems(ctx, tx, document.ID, document.Items); err != nil {
			return err
		}
		if err := rebuildMaterialRegisterActions(
			ctx,
			tx,
			materialTransferRecorderType,
			document.ID,
		); err != nil {
			return err
		}

		var err error
		result, err = fetchMaterialTransferDocument(ctx, tx, document.ID)
		return err
	}); err != nil {
		return nil, fmt.Errorf("update complete material transfer: %w", err)
	}

	return result, nil
}

func (s *MaterialTransferService) DocumentDetail(
	ctx context.Context,
	id int,
) (*models.MaterialTransferDocument, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, webapp.BadRequest("material transfer id is required", nil)
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return nil, fmt.Errorf("get primary connection for material transfer detail: %w", err)
	}
	defer s.DB.Release(poolConn, connID)

	result, err := fetchMaterialTransferDocument(ctx, poolConn.Conn(), id)
	if err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, webapp.NotFound("material transfer not found", map[string]any{"id": id})
		}
		return nil, fmt.Errorf("fetch complete material transfer: %w", err)
	}
	return result, nil
}

func (s *MaterialTransferService) Delete(
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
		return wmodels.RowsAffectedResponse{}, webapp.BadRequest("material transfer id is required", nil)
	}

	var rowsAffected int64
	if err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		if err := lockMaterialRecorders(ctx, tx, materialTransferRecorderType, id); err != nil {
			return err
		}
		if err := removeMaterialRegisterActions(ctx, tx, materialTransferRecorderType, id); err != nil {
			return err
		}

		result, err := tx.Exec(ctx, "DELETE FROM public.material_transfers WHERE id = $1", id)
		if err != nil {
			return err
		}
		rowsAffected = result.RowsAffected()
		return nil
	}); err != nil {
		return wmodels.RowsAffectedResponse{}, fmt.Errorf("delete material transfer: %w", err)
	}
	if rowsAffected == 0 {
		return wmodels.RowsAffectedResponse{}, webapp.NotFound(
			"material transfer not found",
			map[string]any{"id": id},
		)
	}

	return wmodels.RowsAffectedResponse{RowsAffected: rowsAffected}, nil
}
