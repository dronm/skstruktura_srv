package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dronm/ds/v4"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

func (s *DiadocImportService) Ignore(
	ctx context.Context,
	input models.DiadocIgnoreInput,
) (*models.DiadocDocumentDetail, error) {
	user, err := s.require()
	if err != nil {
		return nil, err
	}
	if input.ID <= 0 || input.Request == nil || input.Request.Version <= 0 {
		return nil, webapp.BadRequest("valid Diadoc document id and version are required", nil)
	}

	var result *models.DiadocDocumentDetail
	err = withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		if err := setAuditActor(ctx, tx, user); err != nil {
			return err
		}
		status, currentVersion, err := lockDiadocDocument(ctx, tx, input.ID)
		if err != nil {
			return err
		}
		if currentVersion != input.Request.Version {
			return diadocVersionConflict(input.ID, input.Request.Version, currentVersion)
		}
		if status == "imported" {
			return webapp.Conflict("an imported Diadoc document cannot be ignored", nil)
		}
		if status == "ignored" {
			return webapp.Conflict("Diadoc document is already ignored", nil)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE integration_diadoc.documents
			SET
				status = 'ignored',
				ignored_reason = NULLIF($2, ''),
				ignored_at = now(),
				ignored_by = $3,
				updated_at = now(),
				version = version + 1
			WHERE id = $1
		`, input.ID, strings.TrimSpace(input.Request.Reason), user.Name); err != nil {
			return err
		}
		result, err = fetchDiadocDocumentDetail(ctx, tx, input.ID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("ignore Diadoc document: %w", err)
	}
	return result, nil
}

func (s *DiadocImportService) Restore(
	ctx context.Context,
	input models.DiadocVersionInput,
) (*models.DiadocDocumentDetail, error) {
	user, err := s.require()
	if err != nil {
		return nil, err
	}
	if input.ID <= 0 || input.Request == nil || input.Request.Version <= 0 {
		return nil, webapp.BadRequest("valid Diadoc document id and version are required", nil)
	}

	var result *models.DiadocDocumentDetail
	err = withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		if err := setAuditActor(ctx, tx, user); err != nil {
			return err
		}
		status, currentVersion, err := lockDiadocDocument(ctx, tx, input.ID)
		if err != nil {
			return err
		}
		if currentVersion != input.Request.Version {
			return diadocVersionConflict(input.ID, input.Request.Version, currentVersion)
		}
		if status != "ignored" {
			return webapp.Conflict("only an ignored Diadoc document can be restored", map[string]any{"status": status})
		}
		if _, err := tx.Exec(ctx, `
			UPDATE integration_diadoc.documents
			SET
				status = 'needs_matching',
				ignored_reason = NULL,
				ignored_at = NULL,
				ignored_by = NULL,
				updated_at = now()
			WHERE id = $1
		`, input.ID); err != nil {
			return err
		}
		if err := recalculateDiadocDocumentTx(ctx, tx, input.ID, true); err != nil {
			return err
		}
		result, err = fetchDiadocDocumentDetail(ctx, tx, input.ID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("restore Diadoc document: %w", err)
	}
	return result, nil
}

func (s *DiadocImportService) Retry(
	ctx context.Context,
	input models.DiadocVersionInput,
) (*models.DiadocDocumentDetail, error) {
	if _, err := s.require(); err != nil {
		return nil, err
	}
	if input.ID <= 0 || input.Request == nil || input.Request.Version <= 0 {
		return nil, webapp.BadRequest("valid Diadoc document id and version are required", nil)
	}
	if s.Manager == nil {
		return nil, webapp.Internal("Diadoc integration is not configured", nil)
	}
	document, err := s.Detail(ctx, input.ID)
	if err != nil {
		return nil, err
	}
	if document.Version != input.Request.Version {
		return nil, diadocVersionConflict(input.ID, input.Request.Version, document.Version)
	}
	if document.Status != "failed" {
		return nil, webapp.Conflict("only a failed Diadoc document can be retried", map[string]any{"status": document.Status})
	}
	if _, err := s.Manager.RetryDocument(ctx, input.ID, input.Request.Version); err != nil {
		return nil, fmt.Errorf("retry Diadoc document: %w", err)
	}
	return s.Detail(ctx, input.ID)
}

func (s *DiadocImportService) ExcludeItem(
	ctx context.Context,
	input models.DiadocItemVersionInput,
) (*models.DiadocDocumentDetail, error) {
	return s.setItemExcluded(ctx, input, true)
}

func (s *DiadocImportService) RestoreItem(
	ctx context.Context,
	input models.DiadocItemVersionInput,
) (*models.DiadocDocumentDetail, error) {
	return s.setItemExcluded(ctx, input, false)
}

func (s *DiadocImportService) setItemExcluded(
	ctx context.Context,
	input models.DiadocItemVersionInput,
	excluded bool,
) (*models.DiadocDocumentDetail, error) {
	user, err := s.require()
	if err != nil {
		return nil, err
	}
	if input.ID <= 0 || input.ItemID <= 0 || input.Request == nil || input.Request.Version <= 0 {
		return nil, webapp.BadRequest("valid Diadoc document id, item id, and version are required", nil)
	}

	var result *models.DiadocDocumentDetail
	err = withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		if err := setAuditActor(ctx, tx, user); err != nil {
			return err
		}
		status, currentVersion, err := lockDiadocDocument(ctx, tx, input.ID)
		if err != nil {
			return err
		}
		if currentVersion != input.Request.Version {
			return diadocVersionConflict(input.ID, input.Request.Version, currentVersion)
		}
		if status == "imported" || status == "ignored" || status == "revoked" || status == "superseded" {
			return webapp.Conflict("Diadoc document items cannot be changed in its current status", map[string]any{"status": status})
		}

		var currentExcluded bool
		if err := tx.QueryRow(ctx, `
			SELECT is_excluded
			FROM integration_diadoc.document_items
			WHERE id = $1
				AND document_id = $2
			FOR UPDATE
		`, input.ItemID, input.ID).Scan(&currentExcluded); err != nil {
			if errors.Is(err, ds.ErrNoRows) {
				return webapp.NotFound("Diadoc document item not found", map[string]any{"item_id": input.ItemID})
			}
			return err
		}
		if currentExcluded == excluded {
			return webapp.Conflict("Diadoc document item already has the requested import state", map[string]any{"is_excluded": excluded})
		}
		if _, err := tx.Exec(ctx, `
			UPDATE integration_diadoc.document_items
			SET is_excluded = $3
			WHERE id = $1
				AND document_id = $2
		`, input.ItemID, input.ID, excluded); err != nil {
			return err
		}
		if err := recalculateDiadocDocumentTx(ctx, tx, input.ID, true); err != nil {
			return err
		}
		result, err = fetchDiadocDocumentDetail(ctx, tx, input.ID)
		return err
	})
	if err != nil {
		action := "restore"
		if excluded {
			action = "exclude"
		}
		return nil, fmt.Errorf("%s Diadoc document item: %w", action, err)
	}
	return result, nil
}

func (s *DiadocImportService) Import(
	ctx context.Context,
	input models.DiadocVersionInput,
) (models.DiadocImportResponse, error) {
	user, err := s.require()
	if err != nil {
		return models.DiadocImportResponse{}, err
	}
	if input.ID <= 0 || input.Request == nil || input.Request.Version <= 0 {
		return models.DiadocImportResponse{}, webapp.BadRequest("valid Diadoc document id and version are required", nil)
	}
	if s.Authorizer == nil {
		return models.DiadocImportResponse{}, webapp.Internal("Diadoc import authorizer is not initialized", nil)
	}
	if _, err := s.Authorizer.Require(s.Session, "materialReceipt.create"); err != nil {
		return models.DiadocImportResponse{}, err
	}

	result := models.DiadocImportResponse{DocumentID: input.ID}
	err = withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		if err := setAuditActor(ctx, tx, user); err != nil {
			return err
		}
		status, currentVersion, err := lockDiadocDocument(ctx, tx, input.ID)
		if err != nil {
			return err
		}
		if currentVersion != input.Request.Version {
			return diadocVersionConflict(input.ID, input.Request.Version, currentVersion)
		}
		if status != "ready" {
			return webapp.Conflict("Diadoc document is not ready for import", map[string]any{"status": status})
		}

		var supplierID int
		var constructionSiteID *int
		var receiptNumber string
		var receiptDatePresent bool
		var materialReceiptID *int
		if err := tx.QueryRow(ctx, `
			SELECT
				COALESCE(supplier_id, 0),
				construction_site_id,
				COALESCE(receipt_number, ''),
				receipt_date IS NOT NULL,
				material_receipt_id
			FROM integration_diadoc.documents
			WHERE id = $1
		`, input.ID).Scan(
			&supplierID,
			&constructionSiteID,
			&receiptNumber,
			&receiptDatePresent,
			&materialReceiptID,
		); err != nil {
			return err
		}
		if materialReceiptID != nil {
			return webapp.Conflict("Diadoc document was already imported", map[string]any{"material_receipt_id": *materialReceiptID})
		}
		if supplierID <= 0 || receiptNumber == "" || !receiptDatePresent {
			return webapp.Conflict("Diadoc document header is incomplete", nil)
		}
		if err := requireActiveReference(ctx, tx, "public.suppliers", supplierID, "supplier"); err != nil {
			return err
		}

		var invalidItems int
		if err := tx.QueryRow(ctx, `
			SELECT count(*)::integer
			FROM integration_diadoc.document_items AS item
			LEFT JOIN public.materials AS material
				ON material.id = item.material_id
			LEFT JOIN public.measure_units AS unit
				ON unit.id = item.measure_unit_id
			WHERE item.document_id = $1
				AND NOT item.is_excluded
				AND (
					item.material_id IS NULL
					OR NOT EXISTS (
						SELECT 1 FROM public.construction_sites AS effective_site
						WHERE effective_site.id = COALESCE(item.construction_site_id, $2::integer)
							AND effective_site.is_active
					)
					OR material.id IS NULL
					OR NOT material.is_active
					OR item.measure_unit_id IS NULL
					OR unit.id IS NULL
					OR NOT unit.is_active
					OR material.measure_unit_id <> item.measure_unit_id
					OR item.import_quant IS NULL
					OR item.import_quant <= 0
					OR item.import_price IS NULL
					OR item.import_price < 0
					OR item.import_amount IS NULL
					OR item.import_amount IS DISTINCT FROM item.source_amount_with_vat
					OR item.import_vat_percent IS NULL
					OR item.import_vat_percent IS DISTINCT FROM item.source_vat_percent
					OR item.import_vat_percent < 0
					OR item.import_vat_percent > 100
					OR item.import_vat_amount IS NULL
					OR item.import_vat_amount IS DISTINCT FROM item.source_vat_amount
					OR item.import_vat_amount < 0
					OR item.import_vat_amount > item.import_amount
				)
		`, input.ID, constructionSiteID).Scan(&invalidItems); err != nil {
			return err
		}
		var itemCount int
		if err := tx.QueryRow(ctx, `
			SELECT count(*)::integer
			FROM integration_diadoc.document_items
			WHERE document_id = $1
				AND NOT is_excluded
		`, input.ID).Scan(&itemCount); err != nil {
			return err
		}
		if itemCount == 0 || invalidItems != 0 {
			return webapp.Conflict("Diadoc document items are not ready for import", map[string]any{"invalid_items": invalidItems})
		}

		if err := tx.QueryRow(ctx, `
			INSERT INTO public.material_receipts (
				date,
				construction_site_id,
				supplier_id,
				number,
				comment
			)
			SELECT
				document.receipt_date,
				document.construction_site_id,
				document.supplier_id,
				document.receipt_number,
				CASE
					WHEN document.receipt_comment LIKE 'Imported from Diadoc buffer document %'
						THEN 'Импортировано из Диадока'
					ELSE document.receipt_comment
				END
			FROM integration_diadoc.documents AS document
			WHERE document.id = $1
			RETURNING id
		`, input.ID).Scan(&result.MaterialReceiptID); err != nil {
			return fmt.Errorf("create material receipt from Diadoc: %w", err)
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO public.material_receipt_items (
				line_num,
				material_receipt_id,
				construction_site_id,
				material_id,
				measure_unit_id,
				quant,
				price,
				amount,
				vat_percent,
				vat_amount
			)
			SELECT
				row_number() OVER (ORDER BY item.line_num, item.id)::integer,
				$2,
				item.construction_site_id,
				item.material_id,
				item.measure_unit_id,
				item.import_quant,
				item.import_price,
				item.import_amount,
				item.import_vat_percent,
				item.import_vat_amount
			FROM integration_diadoc.document_items AS item
			WHERE item.document_id = $1
				AND NOT item.is_excluded
			ORDER BY item.line_num, item.id
		`, input.ID, result.MaterialReceiptID); err != nil {
			return fmt.Errorf("create material receipt items from Diadoc: %w", err)
		}

		if err := rebuildMaterialRegisterActions(
			ctx,
			tx,
			materialReceiptRecorderType,
			result.MaterialReceiptID,
		); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
			UPDATE integration_diadoc.documents
			SET
				status = 'imported',
				material_receipt_id = $2,
				imported_at = now(),
				imported_by = $3,
				updated_at = now(),
				version = version + 1
			WHERE id = $1
		`, input.ID, result.MaterialReceiptID, user.Name); err != nil {
			return err
		}
		result.Status = "imported"
		return nil
	})
	if err != nil {
		return models.DiadocImportResponse{}, fmt.Errorf("import Diadoc document: %w", err)
	}
	return result, nil
}

func lockDiadocDocument(
	ctx context.Context,
	tx ds.Querier,
	id int64,
) (string, int64, error) {
	var status string
	var version int64
	if err := tx.QueryRow(ctx, `
		SELECT status, version
		FROM integration_diadoc.documents
		WHERE id = $1
		FOR UPDATE
	`, id).Scan(&status, &version); err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return "", 0, webapp.NotFound("Diadoc document not found", map[string]any{"id": id})
		}
		return "", 0, err
	}
	return status, version, nil
}
