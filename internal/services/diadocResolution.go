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

func (s *DiadocImportService) Resolve(
	ctx context.Context,
	input models.DiadocResolutionInput,
) (*models.DiadocDocumentDetail, error) {
	user, err := s.require()
	if err != nil {
		return nil, err
	}
	if input.ID <= 0 || input.Request == nil {
		return nil, webapp.BadRequest("Diadoc resolution request is required", nil)
	}
	request := input.Request
	if request.Version <= 0 {
		return nil, webapp.BadRequest("version should be positive", nil)
	}
	if request.SupplierID <= 0 {
		return nil, webapp.BadRequest("supplier_id should be positive", nil)
	}
	request.ReceiptNumber = strings.TrimSpace(request.ReceiptNumber)
	request.ReceiptComment = strings.TrimSpace(request.ReceiptComment)
	if request.ReceiptNumber == "" {
		return nil, webapp.BadRequest("receipt_number is required", nil)
	}
	if request.ReceiptDate == nil || request.ReceiptDate.IsZero() {
		return nil, webapp.BadRequest("receipt_date is required", nil)
	}

	var result *models.DiadocDocumentDetail
	err = withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		if err := setAuditActor(ctx, tx, user); err != nil {
			return err
		}

		var currentVersion int64
		var status, boxID, senderBoxID, senderINN, senderKPP string
		if err := tx.QueryRow(ctx, `
			SELECT
				version,
				status,
				box_id,
				COALESCE(sender_box_id, ''),
				COALESCE(sender_inn, ''),
				COALESCE(sender_kpp, '')
			FROM integration_diadoc.documents
			WHERE id = $1
			FOR UPDATE
		`, input.ID).Scan(
			&currentVersion,
			&status,
			&boxID,
			&senderBoxID,
			&senderINN,
			&senderKPP,
		); err != nil {
			if errors.Is(err, ds.ErrNoRows) {
				return webapp.NotFound("Diadoc document not found", map[string]any{"id": input.ID})
			}
			return err
		}
		if currentVersion != request.Version {
			return diadocVersionConflict(input.ID, request.Version, currentVersion)
		}
		if status == "imported" || status == "ignored" || status == "revoked" || status == "superseded" {
			return webapp.Conflict("Diadoc document cannot be resolved in its current status", map[string]any{"status": status})
		}
		if err := requireActiveReference(ctx, tx, "public.suppliers", request.SupplierID, "supplier"); err != nil {
			return err
		}
		if request.ConstructionSiteID != nil {
			if err := requireActiveReference(ctx, tx, "public.construction_sites", *request.ConstructionSiteID, "construction site"); err != nil {
				return err
			}
		}

		if _, err := tx.Exec(ctx, `
			UPDATE integration_diadoc.documents
			SET
				supplier_id = $2,
				construction_site_id = $3,
				receipt_number = $4,
				receipt_date = $5,
				receipt_comment = NULLIF($6, ''),
				updated_at = now()
			WHERE id = $1
		`, input.ID, request.SupplierID, request.ConstructionSiteID, request.ReceiptNumber, request.ReceiptDate, request.ReceiptComment); err != nil {
			return err
		}

		if request.RememberSupplierMatch && senderBoxID != "" {
			if _, err := tx.Exec(ctx, `
				INSERT INTO integration_diadoc.supplier_matches (
					box_id,
					counteragent_box_id,
					source_inn,
					source_kpp,
					supplier_id
				)
				VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5)
				ON CONFLICT (box_id, counteragent_box_id) DO UPDATE
				SET
					source_inn = EXCLUDED.source_inn,
					source_kpp = EXCLUDED.source_kpp,
					supplier_id = EXCLUDED.supplier_id,
					updated_at = now()
			`, boxID, senderBoxID, senderINN, senderKPP, request.SupplierID); err != nil {
				return fmt.Errorf("save Diadoc supplier mapping: %w", err)
			}
			rows, err := tx.Query(ctx, `
				UPDATE integration_diadoc.documents
				SET
					supplier_id = $3,
					updated_at = now(),
					version = version + 1
				WHERE id <> $1
					AND box_id = $2
					AND sender_box_id = $4
					AND supplier_id IS NULL
					AND status IN ('received', 'needs_matching', 'ready', 'failed')
				RETURNING id
			`, input.ID, boxID, request.SupplierID, senderBoxID)
			if err != nil {
				return fmt.Errorf("apply Diadoc supplier mapping: %w", err)
			}
			mappedDocumentIDs := make([]int64, 0)
			for rows.Next() {
				var mappedDocumentID int64
				if err := rows.Scan(&mappedDocumentID); err != nil {
					rows.Close()
					return err
				}
				mappedDocumentIDs = append(mappedDocumentIDs, mappedDocumentID)
			}
			if err := rows.Err(); err != nil {
				rows.Close()
				return err
			}
			rows.Close()
			for _, mappedDocumentID := range mappedDocumentIDs {
				if err := recalculateDiadocDocumentTx(ctx, tx, mappedDocumentID, false); err != nil {
					return err
				}
			}
		}

		seenItems := make(map[int64]struct{}, len(request.Items))
		for index, item := range request.Items {
			if item == nil || item.ID <= 0 || item.MaterialID < 0 {
				return webapp.BadRequest(fmt.Sprintf("items[%d] is invalid", index), nil)
			}
			if _, exists := seenItems[item.ID]; exists {
				return webapp.BadRequest(fmt.Sprintf("items[%d].id is duplicated", index), nil)
			}
			seenItems[item.ID] = struct{}{}
			if err := resolveDiadocItemTx(ctx, tx, input.ID, request.SupplierID, item); err != nil {
				return err
			}
		}

		if err := recalculateDiadocDocumentTx(ctx, tx, input.ID, true); err != nil {
			return err
		}
		var fetchErr error
		result, fetchErr = fetchDiadocDocumentDetail(ctx, tx, input.ID)
		return fetchErr
	})
	if err != nil {
		return nil, fmt.Errorf("save Diadoc document resolution: %w", err)
	}
	return result, nil
}

func resolveDiadocItemTx(
	ctx context.Context,
	tx ds.Querier,
	documentID int64,
	supplierID int,
	request *models.DiadocResolutionItem,
) error {
	var productCode, article, gtin, sourceName, sourceOKEI string
	if err := tx.QueryRow(ctx, `
		SELECT
			COALESCE(source_product_code, ''),
			COALESCE(source_article, ''),
			COALESCE(source_gtin, ''),
			source_name,
			COALESCE(source_okei_code, '')
		FROM integration_diadoc.document_items
		WHERE id = $1
			AND document_id = $2
		FOR UPDATE
	`, request.ID, documentID).Scan(
		&productCode,
		&article,
		&gtin,
		&sourceName,
		&sourceOKEI,
	); err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return webapp.BadRequest("Diadoc item does not belong to the document", map[string]any{"item_id": request.ID})
		}
		return err
	}

	if request.ConstructionSiteID != nil {
		if err := requireActiveReference(ctx, tx, "public.construction_sites", *request.ConstructionSiteID, "construction site"); err != nil {
			return err
		}
	}

	if request.MaterialID == 0 {
		// Save the site even while this item is waiting for material matching.
		_, err := tx.Exec(ctx, `
			UPDATE integration_diadoc.document_items
			SET construction_site_id = $3,
				material_id = NULL, measure_unit_id = NULL,
				conversion_factor = NULL, import_quant = NULL, import_price = NULL,
				mapping_source = NULL
			WHERE id = $1 AND document_id = $2
		`, request.ID, documentID, request.ConstructionSiteID)
		return err
	}

	var measureUnitID int
	var targetOKEI string
	if err := tx.QueryRow(ctx, `
		SELECT material.measure_unit_id, COALESCE(unit.okei_code, '')
		FROM public.materials AS material
		JOIN public.measure_units AS unit
			ON unit.id = material.measure_unit_id
			AND unit.is_active
		WHERE material.id = $1
			AND material.is_active
	`, request.MaterialID).Scan(&measureUnitID, &targetOKEI); err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return webapp.BadRequest("selected material or its measurement unit is inactive", map[string]any{"material_id": request.MaterialID})
		}
		return err
	}

	factor := strings.TrimSpace(request.ConversionFactor)
	if factor == "" && sourceOKEI != "" && sourceOKEI == targetOKEI {
		factor = "1"
	}
	if !positiveDecimal(factor) {
		return webapp.BadRequest("conversion_factor should be a positive decimal", map[string]any{"item_id": request.ID})
	}

	if _, err := tx.Exec(ctx, `
		UPDATE integration_diadoc.document_items
		SET
			material_id = $3,
			measure_unit_id = $4,
			conversion_factor = $5::numeric,
			construction_site_id = $6,
			mapping_source = 'manual',
			last_error = NULL
		WHERE id = $1
			AND document_id = $2
	`, request.ID, documentID, request.MaterialID, measureUnitID, factor, request.ConstructionSiteID); err != nil {
		return fmt.Errorf("save Diadoc item resolution: %w", err)
	}

	if request.RememberMaterialMatch {
		kind, key := preferredMaterialMatchKey(productCode, article, gtin, sourceName)
		if key == "" {
			return webapp.BadRequest("item has no reusable material matching key", map[string]any{"item_id": request.ID})
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO integration_diadoc.supplier_material_matches (
				supplier_id,
				source_key_kind,
				source_key,
				source_name,
				source_okei_code,
				material_id,
				conversion_factor
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7::numeric)
			ON CONFLICT (supplier_id, source_key_kind, source_key, source_okei_code) DO UPDATE
			SET
				source_name = EXCLUDED.source_name,
				material_id = EXCLUDED.material_id,
				conversion_factor = EXCLUDED.conversion_factor,
				updated_at = now()
		`, supplierID, kind, key, sourceName, sourceOKEI, request.MaterialID, factor); err != nil {
			return fmt.Errorf("save Diadoc material mapping: %w", err)
		}
		rows, err := tx.Query(ctx, `
			UPDATE integration_diadoc.document_items AS item
			SET
				material_id = $6,
				measure_unit_id = $7,
				conversion_factor = $8::numeric,
				mapping_source = 'cached',
				last_error = NULL
			FROM integration_diadoc.documents AS document
			WHERE document.id = item.document_id
				AND document.id <> $1
				AND document.supplier_id = $2
				AND document.status IN ('received', 'needs_matching', 'ready', 'failed')
				AND item.material_id IS NULL
				AND COALESCE(item.source_okei_code, '') = $5
				AND CASE $3
					WHEN 'product_code' THEN COALESCE(item.source_product_code, '')
					WHEN 'article' THEN COALESCE(item.source_article, '')
					WHEN 'gtin' THEN COALESCE(item.source_gtin, '')
					ELSE regexp_replace(btrim(item.source_name), '[[:space:]]+', ' ', 'g')
				END = $4
			RETURNING item.document_id
		`, documentID, supplierID, kind, key, sourceOKEI, request.MaterialID, measureUnitID, factor)
		if err != nil {
			return fmt.Errorf("apply Diadoc material mapping: %w", err)
		}
		affected := make(map[int64]struct{})
		for rows.Next() {
			var affectedDocumentID int64
			if err := rows.Scan(&affectedDocumentID); err != nil {
				rows.Close()
				return err
			}
			affected[affectedDocumentID] = struct{}{}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		for affectedDocumentID := range affected {
			if err := recalculateDiadocDocumentTx(ctx, tx, affectedDocumentID, true); err != nil {
				return err
			}
		}
	}
	return nil
}

func recalculateDiadocDocumentTx(
	ctx context.Context,
	tx ds.Querier,
	documentID int64,
	incrementVersion bool,
) error {
	if _, err := tx.Exec(ctx, `
		UPDATE integration_diadoc.document_items AS item
		SET
			import_quant = round(item.source_quant * item.conversion_factor, 4),
			import_price = CASE
				WHEN item.source_quant * item.conversion_factor > 0 THEN
					round(item.source_amount_with_vat / (item.source_quant * item.conversion_factor), 6)
				ELSE NULL
			END,
			import_amount = item.source_amount_with_vat,
			import_vat_percent = item.source_vat_percent,
			import_vat_amount = item.source_vat_amount
		WHERE item.document_id = $1
			AND item.material_id IS NOT NULL
			AND item.measure_unit_id IS NOT NULL
			AND item.conversion_factor > 0
	`, documentID); err != nil {
		return fmt.Errorf("calculate Diadoc import values: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE integration_diadoc.documents AS document
		SET
			status = CASE WHEN
				document.supplier_id IS NOT NULL
				AND EXISTS (
					SELECT 1
					FROM public.suppliers AS supplier
					WHERE supplier.id = document.supplier_id
						AND supplier.is_active
				)

				AND document.receipt_date IS NOT NULL
				AND COALESCE(document.receipt_number, '') <> ''
				AND EXISTS (
					SELECT 1
					FROM integration_diadoc.document_items AS item
					WHERE item.document_id = document.id
						AND NOT item.is_excluded
				)
				AND NOT EXISTS (
					SELECT 1
					FROM integration_diadoc.document_items AS item
					LEFT JOIN public.materials AS material
						ON material.id = item.material_id
					LEFT JOIN public.measure_units AS unit
						ON unit.id = item.measure_unit_id
					WHERE item.document_id = document.id
						AND NOT item.is_excluded
						AND (
							item.material_id IS NULL
							OR NOT EXISTS (
								SELECT 1 FROM public.construction_sites AS effective_site
								WHERE effective_site.id = COALESCE(item.construction_site_id, document.construction_site_id)
									AND effective_site.is_active
							)
							OR material.id IS NULL
							OR NOT material.is_active
							OR item.measure_unit_id IS NULL
							OR unit.id IS NULL
							OR NOT unit.is_active
							OR material.measure_unit_id <> item.measure_unit_id
							OR item.conversion_factor IS NULL
							OR item.conversion_factor <= 0
							OR item.import_quant IS NULL
							OR item.import_quant <= 0
							OR item.import_price IS NULL
							OR item.import_price < 0
							OR item.import_amount IS DISTINCT FROM item.source_amount_with_vat
							OR item.import_vat_percent IS DISTINCT FROM item.source_vat_percent
							OR item.import_vat_amount IS DISTINCT FROM item.source_vat_amount
						)
				) THEN 'ready' ELSE 'needs_matching' END,
			updated_at = now(),
			version = version + CASE WHEN $2 THEN 1 ELSE 0 END
		WHERE document.id = $1
			AND document.status NOT IN ('imported', 'ignored', 'revoked', 'superseded')
	`, documentID, incrementVersion); err != nil {
		return fmt.Errorf("calculate Diadoc document readiness: %w", err)
	}
	return nil
}

func requireActiveReference(
	ctx context.Context,
	tx ds.Querier,
	table string,
	id int,
	description string,
) error {
	var exists bool
	query := fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM %s WHERE id = $1 AND is_active)", table)
	if err := tx.QueryRow(ctx, query, id).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return webapp.BadRequest(description+" is missing or inactive", map[string]any{"id": id})
	}
	return nil
}

func preferredMaterialMatchKey(productCode, article, gtin, name string) (string, string) {
	for _, candidate := range []struct {
		kind string
		key  string
	}{
		{kind: "product_code", key: productCode},
		{kind: "article", key: article},
		{kind: "gtin", key: gtin},
		{kind: "name", key: strings.Join(strings.Fields(name), " ")},
	} {
		if candidate.key != "" {
			return candidate.kind, candidate.key
		}
	}
	return "", ""
}

func diadocVersionConflict(id int64, expected int64, current int64) error {
	return webapp.Conflict(
		"Diadoc document was changed by another request",
		map[string]any{
			"id":               id,
			"expected_version": expected,
			"current_version":  current,
		},
	)
}
