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

func (s *MaterialConsumptionService) Create(
	ctx context.Context,
	document *models.MaterialConsumptionDocument,
) (*models.MaterialConsumptionDocument, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if err := validateMaterialConsumptionDocument(document, true, 0); err != nil {
		return nil, err
	}

	var result *models.MaterialConsumptionDocument
	if err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		if err := tx.QueryRow(ctx, `
			INSERT INTO public.material_consumptions (
				date,
				construction_site_id,
				comment
			)
			VALUES ($1, $2, $3)
			RETURNING id, version
		`,
			document.Date,
			document.ConstructionSiteID,
			document.Comment,
		).Scan(&document.ID, &document.Version); err != nil {
			return err
		}
		if err := syncMaterialConsumptionItems(ctx, tx, document.ID, document.Items); err != nil {
			return err
		}
		if err := rebuildMaterialRegisterActions(
			ctx,
			tx,
			materialConsumptionRecorderType,
			document.ID,
		); err != nil {
			return err
		}

		var err error
		result, err = fetchMaterialConsumptionDocument(ctx, tx, document.ID)
		return err
	}); err != nil {
		return nil, fmt.Errorf("create complete material consumption: %w", err)
	}

	return result, nil
}

func (s *MaterialConsumptionService) Update(
	ctx context.Context,
	input models.UpdateMaterialConsumptionDocumentRequest,
) (*models.MaterialConsumptionDocument, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if err := validateMaterialConsumptionDocument(input.Document, false, input.ID); err != nil {
		return nil, err
	}

	document := input.Document
	document.ID = input.ID
	var result *models.MaterialConsumptionDocument
	if err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		if err := lockMaterialRecorders(ctx, tx, materialConsumptionRecorderType, document.ID); err != nil {
			return err
		}

		var currentVersion int64
		if err := tx.QueryRow(ctx, `
			SELECT version
			FROM public.material_consumptions
			WHERE id = $1
			FOR UPDATE
		`, document.ID).Scan(&currentVersion); err != nil {
			if errors.Is(err, ds.ErrNoRows) {
				return webapp.NotFound("material consumption not found", map[string]any{"id": document.ID})
			}
			return err
		}
		if currentVersion != document.Version {
			return webapp.Conflict(
				"material consumption was changed by another request",
				map[string]any{
					"id":               document.ID,
					"expected_version": document.Version,
					"current_version":  currentVersion,
				},
			)
		}

		if err := tx.QueryRow(ctx, `
			UPDATE public.material_consumptions
			SET
				date = $2,
				construction_site_id = $3,
				comment = $4,
				version = version + 1
			WHERE id = $1
			RETURNING version
		`,
			document.ID,
			document.Date,
			document.ConstructionSiteID,
			document.Comment,
		).Scan(&document.Version); err != nil {
			return err
		}
		if err := syncMaterialConsumptionItems(ctx, tx, document.ID, document.Items); err != nil {
			return err
		}
		if err := rebuildMaterialRegisterActions(
			ctx,
			tx,
			materialConsumptionRecorderType,
			document.ID,
		); err != nil {
			return err
		}

		var err error
		result, err = fetchMaterialConsumptionDocument(ctx, tx, document.ID)
		return err
	}); err != nil {
		return nil, fmt.Errorf("update complete material consumption: %w", err)
	}

	return result, nil
}

func (s *MaterialConsumptionService) DocumentDetail(
	ctx context.Context,
	id int,
) (*models.MaterialConsumptionDocument, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, webapp.BadRequest("material consumption id is required", nil)
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return nil, fmt.Errorf("get primary connection for material consumption detail: %w", err)
	}
	defer s.DB.Release(poolConn, connID)

	result, err := fetchMaterialConsumptionDocument(ctx, poolConn.Conn(), id)
	if err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, webapp.NotFound("material consumption not found", map[string]any{"id": id})
		}
		return nil, fmt.Errorf("fetch complete material consumption: %w", err)
	}
	return result, nil
}

func (s *MaterialConsumptionService) Delete(
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
		return wmodels.RowsAffectedResponse{}, webapp.BadRequest("material consumption id is required", nil)
	}

	var rowsAffected int64
	if err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		if err := lockMaterialRecorders(ctx, tx, materialConsumptionRecorderType, id); err != nil {
			return err
		}
		if err := removeMaterialRegisterActions(ctx, tx, materialConsumptionRecorderType, id); err != nil {
			return err
		}

		result, err := tx.Exec(ctx, "DELETE FROM public.material_consumptions WHERE id = $1", id)
		if err != nil {
			return err
		}
		rowsAffected = result.RowsAffected()
		return nil
	}); err != nil {
		return wmodels.RowsAffectedResponse{}, fmt.Errorf("delete material consumption: %w", err)
	}
	if rowsAffected == 0 {
		return wmodels.RowsAffectedResponse{}, webapp.NotFound(
			"material consumption not found",
			map[string]any{"id": id},
		)
	}

	return wmodels.RowsAffectedResponse{RowsAffected: rowsAffected}, nil
}
