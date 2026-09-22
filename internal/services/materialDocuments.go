package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dronm/ds/v4"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

const materialDocumentMaxItems = 1000

func validateMaterialReceiptDocument(document *models.MaterialReceiptDocument, create bool, pathID int) error {
	if document == nil {
		return webapp.BadRequest("material receipt document is required", nil)
	}
	if err := validateMaterialDocumentIdentity(document.ID, document.Version, create, pathID); err != nil {
		return err
	}
	if document.Date.IsZero() {
		return webapp.BadRequest("material receipt date is required", nil)
	}
	if document.ConstructionSiteID != nil && *document.ConstructionSiteID <= 0 {
		return webapp.BadRequest("material receipt construction_site_id should be positive", nil)
	}
	if document.SupplierID <= 0 {
		return webapp.BadRequest("material receipt supplier_id should be positive", nil)
	}
	document.Number = strings.TrimSpace(document.Number)
	if document.Number == "" {
		return webapp.BadRequest("material receipt number is required", nil)
	}
	if err := validateMaterialDocumentItemCount(len(document.Items)); err != nil {
		return err
	}

	seenIDs := make(map[int]struct{}, len(document.Items))
	for index, item := range document.Items {
		if item == nil {
			return invalidMaterialDocumentItem(index, "item is required")
		}
		if err := validateSubmittedItemID(item.ID, create, index, seenIDs); err != nil {
			return err
		}
		if item.ConstructionSiteID != nil && *item.ConstructionSiteID <= 0 {
			return invalidMaterialDocumentItem(index, "construction_site_id should be positive")
		}
		if item.ConstructionSiteID == nil && document.ConstructionSiteID == nil {
			return invalidMaterialDocumentItem(index, "construction_site_id is required on the item or receipt header")
		}
		item.LineNum = index + 1
		if item.MaterialID <= 0 {
			return invalidMaterialDocumentItem(index, "material_id should be positive")
		}
		if item.MeasureUnitID <= 0 {
			return invalidMaterialDocumentItem(index, "measure_unit_id should be positive")
		}
		if item.Quant <= 0 {
			return invalidMaterialDocumentItem(index, "quant should be greater than zero")
		}
		if item.Price < 0 {
			return invalidMaterialDocumentItem(index, "price should not be negative")
		}
		if item.Amount < 0 {
			return invalidMaterialDocumentItem(index, "amount should not be negative")
		}
		if item.VatPercent < 0 || item.VatPercent > 100 {
			return invalidMaterialDocumentItem(index, "vat_percent should be between zero and 100")
		}
		if item.VatAmount < 0 || item.VatAmount > item.Amount {
			return invalidMaterialDocumentItem(index, "vat_amount should be between zero and amount")
		}
	}

	return nil
}

func validateMaterialConsumptionDocument(document *models.MaterialConsumptionDocument, create bool, pathID int) error {
	if document == nil {
		return webapp.BadRequest("material consumption document is required", nil)
	}
	if err := validateMaterialDocumentIdentity(document.ID, document.Version, create, pathID); err != nil {
		return err
	}
	if document.Date.IsZero() {
		return webapp.BadRequest("material consumption date is required", nil)
	}
	if document.ConstructionSiteID <= 0 {
		return webapp.BadRequest("material consumption construction_site_id should be positive", nil)
	}
	if err := validateMaterialDocumentItemCount(len(document.Items)); err != nil {
		return err
	}

	seenIDs := make(map[int]struct{}, len(document.Items))
	for index, item := range document.Items {
		if item == nil {
			return invalidMaterialDocumentItem(index, "item is required")
		}
		if err := validateSubmittedItemID(item.ID, create, index, seenIDs); err != nil {
			return err
		}
		item.LineNum = index + 1
		if item.MaterialID <= 0 {
			return invalidMaterialDocumentItem(index, "material_id should be positive")
		}
		if item.MeasureUnitID <= 0 {
			return invalidMaterialDocumentItem(index, "measure_unit_id should be positive")
		}
		if item.Quant <= 0 {
			return invalidMaterialDocumentItem(index, "quant should be greater than zero")
		}
	}

	return nil
}

func validateMaterialTransferDocument(document *models.MaterialTransferDocument, create bool, pathID int) error {
	if document == nil {
		return webapp.BadRequest("material transfer document is required", nil)
	}
	if err := validateMaterialDocumentIdentity(document.ID, document.Version, create, pathID); err != nil {
		return err
	}
	if document.Date.IsZero() {
		return webapp.BadRequest("material transfer date is required", nil)
	}
	if document.SourceConstructionSiteID <= 0 {
		return webapp.BadRequest("material transfer source_construction_site_id should be positive", nil)
	}
	if document.DestinationConstructionSiteID <= 0 {
		return webapp.BadRequest("material transfer destination_construction_site_id should be positive", nil)
	}
	if document.SourceConstructionSiteID == document.DestinationConstructionSiteID {
		return webapp.BadRequest("material transfer source and destination should differ", nil)
	}
	if err := validateMaterialDocumentItemCount(len(document.Items)); err != nil {
		return err
	}

	seenIDs := make(map[int]struct{}, len(document.Items))
	for index, item := range document.Items {
		if item == nil {
			return invalidMaterialDocumentItem(index, "item is required")
		}
		if err := validateSubmittedItemID(item.ID, create, index, seenIDs); err != nil {
			return err
		}
		item.LineNum = index + 1
		if item.MaterialID <= 0 {
			return invalidMaterialDocumentItem(index, "material_id should be positive")
		}
		if item.MeasureUnitID <= 0 {
			return invalidMaterialDocumentItem(index, "measure_unit_id should be positive")
		}
		if item.Quant <= 0 {
			return invalidMaterialDocumentItem(index, "quant should be greater than zero")
		}
	}

	return nil
}

func validateMaterialRequestDocument(document *models.MaterialRequestDocument, create bool, pathID int) error {
	if document == nil {
		return webapp.BadRequest("material request document is required", nil)
	}
	if err := validateMaterialDocumentIdentity(document.ID, document.Version, create, pathID); err != nil {
		return err
	}
	if document.Date.IsZero() {
		return webapp.BadRequest("material request date is required", nil)
	}
	if document.ConstructionSiteID <= 0 {
		return webapp.BadRequest("material request construction_site_id should be positive", nil)
	}
	if document.ConstructionManagerID <= 0 {
		return webapp.BadRequest("material request construction_manager_id should be positive", nil)
	}
	if err := validateMaterialDocumentItemCount(len(document.Items)); err != nil {
		return err
	}

	seenIDs := make(map[int]struct{}, len(document.Items))
	for index, item := range document.Items {
		if item == nil {
			return invalidMaterialDocumentItem(index, "item is required")
		}
		if err := validateSubmittedItemID(item.ID, create, index, seenIDs); err != nil {
			return err
		}
		item.LineNum = index + 1
		if item.MaterialID <= 0 {
			return invalidMaterialDocumentItem(index, "material_id should be positive")
		}
		if item.MeasureUnitID <= 0 {
			return invalidMaterialDocumentItem(index, "measure_unit_id should be positive")
		}
		if item.Quant <= 0 {
			return invalidMaterialDocumentItem(index, "quant should be greater than zero")
		}
		if create && item.SupplierID != nil {
			return invalidMaterialDocumentItem(
				index,
				"supplier_id is managed by the supply manager workspace",
			)
		}
		if item.RequiredDate != nil {
			if _, err := time.Parse(time.DateOnly, item.RequiredDate.String()); err != nil {
				return invalidMaterialDocumentItem(index, "required_date should use YYYY-MM-DD format")
			}
		}
		if item.OrderImportanceID <= 0 {
			return invalidMaterialDocumentItem(index, "order_importance_id should be positive")
		}
		if item.ID == 0 {
			if item.StatusID < 0 {
				return invalidMaterialDocumentItem(index, "status_id should not be negative")
			}
		} else if item.StatusID <= 0 {
			return invalidMaterialDocumentItem(index, "status_id should be positive")
		}
	}

	return nil
}

func validateMaterialDocumentIdentity(bodyID int, version int64, create bool, pathID int) error {
	if create {
		if bodyID != 0 {
			return webapp.BadRequest("a new document should not contain id", map[string]any{"id": bodyID})
		}
		if version != 0 {
			return webapp.BadRequest(
				"a new document should not contain version",
				map[string]any{"version": version},
			)
		}
		return nil
	}

	if pathID <= 0 {
		return webapp.BadRequest("document id should be positive", nil)
	}
	if bodyID != 0 && bodyID != pathID {
		return webapp.BadRequest(
			"document body id does not match path id",
			map[string]any{"path_id": pathID, "body_id": bodyID},
		)
	}
	if version <= 0 {
		return webapp.BadRequest("document version should be positive", nil)
	}

	return nil
}

func validateMaterialDocumentItemCount(count int) error {
	if count == 0 {
		return webapp.BadRequest("document should contain at least one item", nil)
	}
	if count > materialDocumentMaxItems {
		return webapp.BadRequest(
			"document contains too many items",
			map[string]any{"maximum": materialDocumentMaxItems, "count": count},
		)
	}
	return nil
}

func validateSubmittedItemID(id int, create bool, index int, seen map[int]struct{}) error {
	if id < 0 {
		return invalidMaterialDocumentItem(index, "id should not be negative")
	}
	if create && id != 0 {
		return invalidMaterialDocumentItem(index, "a new document item should not contain id")
	}
	if id == 0 {
		return nil
	}
	if _, exists := seen[id]; exists {
		return invalidMaterialDocumentItem(index, "id is duplicated")
	}
	seen[id] = struct{}{}
	return nil
}

func invalidMaterialDocumentItem(index int, message string) error {
	return webapp.BadRequest(
		fmt.Sprintf("items[%d]: %s", index, message),
		map[string]any{"item_index": index},
	)
}

func fetchMaterialReceiptDocument(
	ctx context.Context,
	db ds.Querier,
	id int,
) (*models.MaterialReceiptDocument, error) {
	rows, err := db.Query(ctx, `
		SELECT
			receipt.id,
			receipt.version,
			receipt.date,
			receipt.construction_site_id,
			receipt.supplier_id,
			receipt.number,
			receipt.comment,
			item.id,
			item.line_num,
			item.material_id,
			item.measure_unit_id,
			item.quant::double precision,
			item.price::double precision,
			item.amount::double precision,
			item.vat_percent::double precision,
			item.vat_amount::double precision,
			receipt.construction_site,
			receipt.supplier,
			item.material,
			item.measure_unit,
			item.construction_site,
			item.construction_site_id
		FROM public.material_receipts_list AS receipt
		LEFT JOIN public.material_receipt_items_list AS item
			ON item.material_receipt_id = receipt.id
		WHERE receipt.id = $1
		ORDER BY item.line_num, item.id
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var document *models.MaterialReceiptDocument
	for rows.Next() {
		var documentID, supplierID int
		var constructionSiteID, itemConstructionSiteID *int
		var version int64
		var date time.Time
		var number string
		var comment *string
		var itemID, lineNum, materialID, measureUnitID *int
		var quant, price, amount, vatPercent, vatAmount *float64
		var constructionSiteRef, supplierRef, itemMaterial, itemMeasureUnit, itemConstructionSite *models.Ref
		if err := rows.Scan(
			&documentID,
			&version,
			&date,
			&constructionSiteID,
			&supplierID,
			&number,
			&comment,
			&itemID,
			&lineNum,
			&materialID,
			&measureUnitID,
			&quant,
			&price,
			&amount,
			&vatPercent,
			&vatAmount,
			&constructionSiteRef,
			&supplierRef,
			&itemMaterial,
			&itemMeasureUnit,
			&itemConstructionSite,
			&itemConstructionSiteID,
		); err != nil {
			return nil, err
		}
		if document == nil {
			document = &models.MaterialReceiptDocument{
				ConstructionSite:   constructionSiteRef,
				Supplier:           supplierRef,
				ID:                 documentID,
				Version:            version,
				Date:               date,
				ConstructionSiteID: constructionSiteID,
				SupplierID:         supplierID,
				Number:             number,
				Comment:            comment,
				Items:              make([]*models.MaterialReceiptDocumentItem, 0),
			}
		}
		if itemID != nil {
			document.Items = append(document.Items, &models.MaterialReceiptDocumentItem{
				Material:           itemMaterial,
				MeasureUnit:        itemMeasureUnit,
				ConstructionSite:   itemConstructionSite,
				ConstructionSiteID: itemConstructionSiteID,
				ID:                 *itemID,
				LineNum:            *lineNum,
				MaterialID:         *materialID,
				MeasureUnitID:      *measureUnitID,
				Quant:              *quant,
				Price:              *price,
				Amount:             *amount,
				VatPercent:         *vatPercent,
				VatAmount:          *vatAmount,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if document == nil {
		return nil, ds.ErrNoRows
	}
	return document, nil
}

func fetchMaterialConsumptionDocument(
	ctx context.Context,
	db ds.Querier,
	id int,
) (*models.MaterialConsumptionDocument, error) {
	rows, err := db.Query(ctx, `
		SELECT
			consumption.id,
			consumption.version,
			consumption.date,
			consumption.construction_site_id,
			consumption.comment,
			item.id,
			item.line_num,
			item.material_id,
			item.measure_unit_id,
			item.quant::double precision,
			consumption.construction_site,
			item.material,
			item.measure_unit
		FROM public.material_consumptions_list AS consumption
		LEFT JOIN public.material_consumption_items_list AS item
			ON item.material_consumption_id = consumption.id
		WHERE consumption.id = $1
		ORDER BY item.line_num, item.id
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var document *models.MaterialConsumptionDocument
	for rows.Next() {
		var documentID, constructionSiteID int
		var version int64
		var date time.Time
		var comment *string
		var itemID, lineNum, materialID, measureUnitID *int
		var quant *float64
		var constructionSiteRef, itemMaterial, itemMeasureUnit *models.Ref
		if err := rows.Scan(
			&documentID,
			&version,
			&date,
			&constructionSiteID,
			&comment,
			&itemID,
			&lineNum,
			&materialID,
			&measureUnitID,
			&quant,
			&constructionSiteRef,
			&itemMaterial,
			&itemMeasureUnit,
		); err != nil {
			return nil, err
		}
		if document == nil {
			document = &models.MaterialConsumptionDocument{
				ConstructionSite:   constructionSiteRef,
				ID:                 documentID,
				Version:            version,
				Date:               date,
				ConstructionSiteID: constructionSiteID,
				Comment:            comment,
				Items:              make([]*models.MaterialConsumptionDocumentItem, 0),
			}
		}
		if itemID != nil {
			document.Items = append(document.Items, &models.MaterialConsumptionDocumentItem{
				Material:      itemMaterial,
				MeasureUnit:   itemMeasureUnit,
				ID:            *itemID,
				LineNum:       *lineNum,
				MaterialID:    *materialID,
				MeasureUnitID: *measureUnitID,
				Quant:         *quant,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if document == nil {
		return nil, ds.ErrNoRows
	}
	return document, nil
}

func fetchMaterialTransferDocument(
	ctx context.Context,
	db ds.Querier,
	id int,
) (*models.MaterialTransferDocument, error) {
	rows, err := db.Query(ctx, `
		SELECT
			transfer.id,
			transfer.version,
			transfer.date,
			transfer.source_construction_site_id,
			transfer.destination_construction_site_id,
			transfer.comment,
			item.id,
			item.line_num,
			item.material_id,
			item.measure_unit_id,
			item.quant::double precision,
			transfer.source_construction_site,
			transfer.destination_construction_site,
			item.material,
			item.measure_unit
		FROM public.material_transfers_list AS transfer
		LEFT JOIN public.material_transfer_items_list AS item
			ON item.material_transfer_id = transfer.id
		WHERE transfer.id = $1
		ORDER BY item.line_num, item.id
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var document *models.MaterialTransferDocument
	for rows.Next() {
		var documentID, sourceSiteID, destinationSiteID int
		var version int64
		var date time.Time
		var comment *string
		var itemID, lineNum, materialID, measureUnitID *int
		var quant *float64
		var sourceConstructionSiteRef, destinationConstructionSiteRef, itemMaterial, itemMeasureUnit *models.Ref
		if err := rows.Scan(
			&documentID,
			&version,
			&date,
			&sourceSiteID,
			&destinationSiteID,
			&comment,
			&itemID,
			&lineNum,
			&materialID,
			&measureUnitID,
			&quant,
			&sourceConstructionSiteRef,
			&destinationConstructionSiteRef,
			&itemMaterial,
			&itemMeasureUnit,
		); err != nil {
			return nil, err
		}
		if document == nil {
			document = &models.MaterialTransferDocument{
				SourceConstructionSite:        sourceConstructionSiteRef,
				DestinationConstructionSite:   destinationConstructionSiteRef,
				ID:                            documentID,
				Version:                       version,
				Date:                          date,
				SourceConstructionSiteID:      sourceSiteID,
				DestinationConstructionSiteID: destinationSiteID,
				Comment:                       comment,
				Items:                         make([]*models.MaterialTransferDocumentItem, 0),
			}
		}
		if itemID != nil {
			document.Items = append(document.Items, &models.MaterialTransferDocumentItem{
				Material:      itemMaterial,
				MeasureUnit:   itemMeasureUnit,
				ID:            *itemID,
				LineNum:       *lineNum,
				MaterialID:    *materialID,
				MeasureUnitID: *measureUnitID,
				Quant:         *quant,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if document == nil {
		return nil, ds.ErrNoRows
	}
	return document, nil
}

func fetchMaterialRequestDocument(
	ctx context.Context,
	db ds.Querier,
	id int,
) (*models.MaterialRequestDocument, error) {
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
			to_char(item.required_date, 'YYYY-MM-DD'),
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
		WHERE request.id = $1
		ORDER BY item.line_num, item.id
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var document *models.MaterialRequestDocument
	for rows.Next() {
		var documentID, constructionSiteID, constructionManagerID int
		var version int64
		var date time.Time
		var comment *string
		var requestStatusID int
		var itemID, lineNum, materialID, measureUnitID, orderImportanceID, statusID *int
		var supplierID *int
		var requiredDate *string
		var quant *float64
		var constructionSiteRef, constructionManagerRef, requestStatusRef *models.Ref
		var itemMaterial, itemMeasureUnit, itemSupplier, itemOrderImportance, itemStatus *models.Ref
		if err := rows.Scan(
			&documentID,
			&version,
			&date,
			&constructionSiteID,
			&constructionManagerID,
			&comment,
			&requestStatusID,
			&requestStatusRef,
			&itemID,
			&lineNum,
			&materialID,
			&measureUnitID,
			&quant,
			&supplierID,
			&requiredDate,
			&orderImportanceID,
			&statusID,
			&constructionSiteRef,
			&constructionManagerRef,
			&itemMaterial,
			&itemMeasureUnit,
			&itemSupplier,
			&itemOrderImportance,
			&itemStatus,
		); err != nil {
			return nil, err
		}
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
				ConstructionSite:      constructionSiteRef,
				ConstructionManager:   constructionManagerRef,
				Status:                requestStatusRef,
			}
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
			StatusID:          *statusID,
			Material:          itemMaterial,
			MeasureUnit:       itemMeasureUnit,
			Supplier:          itemSupplier,
			OrderImportance:   itemOrderImportance,
			Status:            itemStatus,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if document == nil {
		return nil, ds.ErrNoRows
	}
	return document, nil
}

func syncMaterialReceiptItems(
	ctx context.Context,
	tx ds.Querier,
	documentID int,
	items []*models.MaterialReceiptDocumentItem,
) error {
	existing, err := materialDocumentItemIDs(ctx, tx, `
		SELECT id
		FROM public.material_receipt_items
		WHERE material_receipt_id = $1
		FOR UPDATE
	`, documentID)
	if err != nil {
		return err
	}
	if err := stageMaterialDocumentItemLines(ctx, tx, `
		WITH line_offset AS (
			SELECT COALESCE(MAX(line_num), 0) + $2 AS value
			FROM public.material_receipt_items
			WHERE material_receipt_id = $1
		)
		UPDATE public.material_receipt_items AS item
		SET line_num = item.line_num + line_offset.value
		FROM line_offset
		WHERE item.material_receipt_id = $1
	`, documentID); err != nil {
		return err
	}

	for index, item := range items {
		if item.ID == 0 {
			if err := tx.QueryRow(ctx, `
				INSERT INTO public.material_receipt_items (
					line_num,
					material_receipt_id,
					material_id,
					measure_unit_id,
					quant,
					price,
					amount,
					vat_percent,
					vat_amount,
					construction_site_id
				)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
				RETURNING id
			`,
				index+1,
				documentID,
				item.MaterialID,
				item.MeasureUnitID,
				item.Quant,
				item.Price,
				item.Amount,
				item.VatPercent,
				item.VatAmount,
				item.ConstructionSiteID,
			).Scan(&item.ID); err != nil {
				return err
			}
			continue
		}

		if _, ok := existing[item.ID]; !ok {
			return invalidMaterialDocumentItem(index, "id does not belong to this document")
		}
		if _, err := tx.Exec(ctx, `
			UPDATE public.material_receipt_items
			SET
				line_num = $3,
				material_id = $4,
				measure_unit_id = $5,
				quant = $6,
				price = $7,
				amount = $8,
				vat_percent = $9,
				vat_amount = $10,
				construction_site_id = $11
			WHERE id = $1
				AND material_receipt_id = $2
		`,
			item.ID,
			documentID,
			index+1,
			item.MaterialID,
			item.MeasureUnitID,
			item.Quant,
			item.Price,
			item.Amount,
			item.VatPercent,
			item.VatAmount,
			item.ConstructionSiteID,
		); err != nil {
			return err
		}
		delete(existing, item.ID)
	}

	return deleteMaterialDocumentItems(ctx, tx, "public.material_receipt_items", "material_receipt_id", documentID, existing)
}

func syncMaterialConsumptionItems(
	ctx context.Context,
	tx ds.Querier,
	documentID int,
	items []*models.MaterialConsumptionDocumentItem,
) error {
	existing, err := materialDocumentItemIDs(ctx, tx, `
		SELECT id
		FROM public.material_consumption_items
		WHERE material_consumption_id = $1
		FOR UPDATE
	`, documentID)
	if err != nil {
		return err
	}
	if err := stageMaterialDocumentItemLines(ctx, tx, `
		WITH line_offset AS (
			SELECT COALESCE(MAX(line_num), 0) + $2 AS value
			FROM public.material_consumption_items
			WHERE material_consumption_id = $1
		)
		UPDATE public.material_consumption_items AS item
		SET line_num = item.line_num + line_offset.value
		FROM line_offset
		WHERE item.material_consumption_id = $1
	`, documentID); err != nil {
		return err
	}

	for index, item := range items {
		if item.ID == 0 {
			if err := tx.QueryRow(ctx, `
				INSERT INTO public.material_consumption_items (
					line_num,
					material_consumption_id,
					material_id,
					measure_unit_id,
					quant
				)
				VALUES ($1, $2, $3, $4, $5)
				RETURNING id
			`, index+1, documentID, item.MaterialID, item.MeasureUnitID, item.Quant).Scan(&item.ID); err != nil {
				return err
			}
			continue
		}

		if _, ok := existing[item.ID]; !ok {
			return invalidMaterialDocumentItem(index, "id does not belong to this document")
		}
		if _, err := tx.Exec(ctx, `
			UPDATE public.material_consumption_items
			SET
				line_num = $3,
				material_id = $4,
				measure_unit_id = $5,
				quant = $6
			WHERE id = $1
				AND material_consumption_id = $2
		`, item.ID, documentID, index+1, item.MaterialID, item.MeasureUnitID, item.Quant); err != nil {
			return err
		}
		delete(existing, item.ID)
	}

	return deleteMaterialDocumentItems(ctx, tx, "public.material_consumption_items", "material_consumption_id", documentID, existing)
}

func syncMaterialTransferItems(
	ctx context.Context,
	tx ds.Querier,
	documentID int,
	items []*models.MaterialTransferDocumentItem,
) error {
	existing, err := materialDocumentItemIDs(ctx, tx, `
		SELECT id
		FROM public.material_transfer_items
		WHERE material_transfer_id = $1
		FOR UPDATE
	`, documentID)
	if err != nil {
		return err
	}
	if err := stageMaterialDocumentItemLines(ctx, tx, `
		WITH line_offset AS (
			SELECT COALESCE(MAX(line_num), 0) + $2 AS value
			FROM public.material_transfer_items
			WHERE material_transfer_id = $1
		)
		UPDATE public.material_transfer_items AS item
		SET line_num = item.line_num + line_offset.value
		FROM line_offset
		WHERE item.material_transfer_id = $1
	`, documentID); err != nil {
		return err
	}

	for index, item := range items {
		if item.ID == 0 {
			if err := tx.QueryRow(ctx, `
				INSERT INTO public.material_transfer_items (
					line_num,
					material_transfer_id,
					material_id,
					measure_unit_id,
					quant
				)
				VALUES ($1, $2, $3, $4, $5)
				RETURNING id
			`, index+1, documentID, item.MaterialID, item.MeasureUnitID, item.Quant).Scan(&item.ID); err != nil {
				return err
			}
			continue
		}

		if _, ok := existing[item.ID]; !ok {
			return invalidMaterialDocumentItem(index, "id does not belong to this document")
		}
		if _, err := tx.Exec(ctx, `
			UPDATE public.material_transfer_items
			SET
				line_num = $3,
				material_id = $4,
				measure_unit_id = $5,
				quant = $6
			WHERE id = $1
				AND material_transfer_id = $2
		`, item.ID, documentID, index+1, item.MaterialID, item.MeasureUnitID, item.Quant); err != nil {
			return err
		}
		delete(existing, item.ID)
	}

	return deleteMaterialDocumentItems(ctx, tx, "public.material_transfer_items", "material_transfer_id", documentID, existing)
}

func syncMaterialRequestItems(
	ctx context.Context,
	tx ds.Querier,
	documentID int,
	items []*models.MaterialRequestDocumentItem,
) error {
	existing, err := materialDocumentItemIDs(ctx, tx, `
		SELECT id
		FROM public.material_request_items
		WHERE material_request_id = $1
		FOR UPDATE
	`, documentID)
	if err != nil {
		return err
	}
	if err := stageMaterialDocumentItemLines(ctx, tx, `
		WITH line_offset AS (
			SELECT COALESCE(MAX(line_num), 0) + $2 AS value
			FROM public.material_request_items
			WHERE material_request_id = $1
		)
		UPDATE public.material_request_items AS item
		SET line_num = item.line_num + line_offset.value
		FROM line_offset
		WHERE item.material_request_id = $1
	`, documentID); err != nil {
		return err
	}

	for index, item := range items {
		var requiredDate any
		if item.RequiredDate != nil {
			requiredDate = item.RequiredDate.String()
		}

		if item.ID == 0 {
			if err := tx.QueryRow(ctx, `
				INSERT INTO public.material_request_items (
					line_num,
					material_request_id,
					material_id,
					measure_unit_id,
					quant,
					supplier_id,
					required_date,
					order_importance_id,
					status_id
				)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
				RETURNING id
			`,
				index+1,
				documentID,
				item.MaterialID,
				item.MeasureUnitID,
				item.Quant,
				item.SupplierID,
				requiredDate,
				item.OrderImportanceID,
				item.StatusID,
			).Scan(&item.ID); err != nil {
				return err
			}
			continue
		}

		if _, ok := existing[item.ID]; !ok {
			return invalidMaterialDocumentItem(index, "id does not belong to this document")
		}
		if _, err := tx.Exec(ctx, `
			UPDATE public.material_request_items
			SET
				line_num = $3,
				material_id = $4,
				measure_unit_id = $5,
				quant = $6,
				supplier_id = $7,
				required_date = $8,
				order_importance_id = $9,
				status_id = $10
			WHERE id = $1
				AND material_request_id = $2
		`,
			item.ID,
			documentID,
			index+1,
			item.MaterialID,
			item.MeasureUnitID,
			item.Quant,
			item.SupplierID,
			requiredDate,
			item.OrderImportanceID,
			item.StatusID,
		); err != nil {
			return err
		}
		delete(existing, item.ID)
	}

	return deleteMaterialDocumentItems(ctx, tx, "public.material_request_items", "material_request_id", documentID, existing)
}

func prepareMaterialRequestReferences(
	ctx context.Context,
	tx ds.Querier,
	document *models.MaterialRequestDocument,
) error {
	draftStatusID, err := materialRequestStatusID(ctx, tx, models.MaterialRequestStatusCodeDraft)
	if err != nil {
		return err
	}
	for _, item := range document.Items {
		if item.ID == 0 {
			item.StatusID = draftStatusID
		}
	}

	var managerRole string
	var constructionSiteExists bool
	if err := tx.QueryRow(ctx, `
		SELECT
			manager.role_id::text,
			(site.id IS NOT NULL)
		FROM public.users AS manager
		LEFT JOIN public.construction_sites AS site
			ON site.id = $2
		WHERE manager.id = $1
	`, document.ConstructionManagerID, document.ConstructionSiteID).Scan(
		&managerRole,
		&constructionSiteExists,
	); err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return webapp.BadRequest(
				"material request construction_manager_id does not reference a user",
				map[string]any{"construction_manager_id": document.ConstructionManagerID},
			)
		}
		return err
	}
	if managerRole != models.RoleIDConstructionSiteManager.String() {
		return webapp.BadRequest(
			"material request construction manager should have construction_site_manager role",
			map[string]any{
				"construction_manager_id": document.ConstructionManagerID,
				"role_id":                 managerRole,
			},
		)
	}
	if !constructionSiteExists {
		return webapp.BadRequest(
			"material request construction_site_id does not reference a construction site",
			map[string]any{"construction_site_id": document.ConstructionSiteID},
		)
	}

	materialIDs := make([]int, len(document.Items))
	measureUnitIDs := make([]int, len(document.Items))
	importanceIDs := make([]int, len(document.Items))
	statusIDs := make([]int, len(document.Items))
	supplierIDs := make([]int, len(document.Items))
	itemIDs := make([]int, len(document.Items))
	for index, item := range document.Items {
		materialIDs[index] = item.MaterialID
		measureUnitIDs[index] = item.MeasureUnitID
		importanceIDs[index] = item.OrderImportanceID
		statusIDs[index] = item.StatusID
		itemIDs[index] = item.ID
		if item.SupplierID != nil {
			supplierIDs[index] = *item.SupplierID
		}
	}

	var index, materialID, measureUnitID, importanceID, statusID, supplierID int
	var expectedMeasureUnitID *int
	var materialExists, importanceExists, inactiveChangedImportance, statusExists, supplierExists bool
	err = tx.QueryRow(ctx, `
		WITH submitted AS (
			SELECT
				input.material_id,
				input.measure_unit_id,
				input.order_importance_id,
				input.status_id,
				input.supplier_id,
				input.item_id,
				input.ordinality::integer AS item_index
			FROM unnest(
				$1::integer[],
				$2::integer[],
				$3::integer[],
				$4::integer[],
				$5::integer[],
				$6::integer[]
			) WITH ORDINALITY AS input(
				material_id,
				measure_unit_id,
				order_importance_id,
				status_id,
				supplier_id,
				item_id,
				ordinality
			)
		)
		SELECT
			submitted.item_index - 1,
			submitted.material_id,
			submitted.measure_unit_id,
			submitted.order_importance_id,
			submitted.status_id,
			submitted.supplier_id,
			material.measure_unit_id,
			(material.id IS NOT NULL),
			(importance.id IS NOT NULL),
			(
				importance.id IS NOT NULL
				AND importance.is_active IS NOT TRUE
				AND (
					submitted.item_id = 0
					OR existing_item.order_importance_id IS DISTINCT FROM submitted.order_importance_id
				)
			),
			(status.id IS NOT NULL),
			(submitted.supplier_id = 0 OR supplier.id IS NOT NULL)
		FROM submitted
		LEFT JOIN public.materials AS material
			ON material.id = submitted.material_id
		LEFT JOIN public.order_importances AS importance
			ON importance.id = submitted.order_importance_id
		LEFT JOIN public.material_request_items AS existing_item
			ON existing_item.id = submitted.item_id
		LEFT JOIN public.material_request_statuses AS status
			ON status.id = submitted.status_id
		LEFT JOIN public.suppliers AS supplier
			ON supplier.id = NULLIF(submitted.supplier_id, 0)
		WHERE material.id IS NULL
			OR material.measure_unit_id <> submitted.measure_unit_id
			OR importance.id IS NULL
			OR (
				importance.is_active IS NOT TRUE
				AND (
					submitted.item_id = 0
					OR existing_item.order_importance_id IS DISTINCT FROM submitted.order_importance_id
				)
			)
			OR status.id IS NULL
			OR (submitted.supplier_id <> 0 AND supplier.id IS NULL)
		ORDER BY submitted.item_index
		LIMIT 1
	`, materialIDs, measureUnitIDs, importanceIDs, statusIDs, supplierIDs, itemIDs).Scan(
		&index,
		&materialID,
		&measureUnitID,
		&importanceID,
		&statusID,
		&supplierID,
		&expectedMeasureUnitID,
		&materialExists,
		&importanceExists,
		&inactiveChangedImportance,
		&statusExists,
		&supplierExists,
	)
	if errors.Is(err, ds.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if !materialExists {
		return invalidMaterialDocumentItem(index, fmt.Sprintf("material_id %d does not reference a material", materialID))
	}
	if expectedMeasureUnitID == nil || *expectedMeasureUnitID != measureUnitID {
		return webapp.BadRequest(
			fmt.Sprintf("items[%d]: measure_unit_id does not match the selected material", index),
			map[string]any{
				"item_index":               index,
				"material_id":              materialID,
				"measure_unit_id":          measureUnitID,
				"expected_measure_unit_id": expectedMeasureUnitID,
			},
		)
	}
	if !importanceExists {
		return invalidMaterialDocumentItem(index, fmt.Sprintf("order_importance_id %d does not reference an order importance", importanceID))
	}
	if inactiveChangedImportance {
		return invalidMaterialDocumentItem(index, fmt.Sprintf("order_importance_id %d is inactive", importanceID))
	}
	if !statusExists {
		return invalidMaterialDocumentItem(index, fmt.Sprintf("status_id %d does not reference a material request status", statusID))
	}
	if !supplierExists {
		return invalidMaterialDocumentItem(index, fmt.Sprintf("supplier_id %d does not reference a supplier", supplierID))
	}

	return nil
}

func materialRequestStatusID(ctx context.Context, tx ds.Querier, code string) (int, error) {
	var id int
	if err := tx.QueryRow(ctx, `
		SELECT id
		FROM public.material_request_statuses
		WHERE code = $1
	`, code).Scan(&id); err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return 0, webapp.Internal(
				"material request workflow status is not configured",
				map[string]any{"status_code": code},
			)
		}
		return 0, err
	}
	return id, nil
}

func materialDocumentItemIDs(
	ctx context.Context,
	tx ds.Querier,
	query string,
	documentID int,
) (map[int]struct{}, error) {
	rows, err := tx.Query(ctx, query, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int]struct{})
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// stageMaterialDocumentItemLines moves the current lines outside the range
// assigned by this request. This makes arbitrary reordering safe with the
// per-document unique line number constraints.
func stageMaterialDocumentItemLines(
	ctx context.Context,
	tx ds.Querier,
	query string,
	documentID int,
) error {
	_, err := tx.Exec(ctx, query, documentID, materialDocumentMaxItems+1)
	return err
}

func deleteMaterialDocumentItems(
	ctx context.Context,
	tx ds.Querier,
	table string,
	parentColumn string,
	documentID int,
	itemIDs map[int]struct{},
) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1 AND %s = $2", table, parentColumn)
	for itemID := range itemIDs {
		if _, err := tx.Exec(ctx, query, itemID, documentID); err != nil {
			return err
		}
	}
	return nil
}
