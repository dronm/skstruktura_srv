package services

import (
	"context"
	"fmt"

	"github.com/dronm/ds/v4"
	"github.com/dronm/skstruktura/internal/models"
)

func fetchDiadocDocumentDetail(
	ctx context.Context,
	db ds.Querier,
	id int64,
) (*models.DiadocDocumentDetail, error) {
	document := &models.DiadocDocumentDetail{
		Items: make([]*models.DiadocDocumentItem, 0),
		Readiness: models.DiadocReadiness{
			Missing:  make([]*models.DiadocReadinessIssue, 0),
			Warnings: make([]string, 0),
		},
	}
	var supplierName, siteName string
	var supplierActive, siteActive bool
	if err := db.QueryRow(ctx, `
		SELECT
			document.id,
			document.version,
			document.status,
			document.message_id,
			document.entity_id,
			COALESCE(document.document_number, ''),
			document.document_date,
			COALESCE(document.document_function, ''),
			COALESCE(document.document_version, ''),
			COALESCE(document.sender_box_id, ''),
			COALESCE(document.sender_name, ''),
			COALESCE(document.sender_inn, ''),
			COALESCE(document.sender_kpp, ''),
			document.supplier_id,
			COALESCE(supplier.name, ''),
			COALESCE(supplier.is_active, false),
			document.construction_site_id,
			COALESCE(site.name, ''),
			COALESCE(site.is_active, false),
			COALESCE(document.receipt_number, ''),
			document.receipt_date,
			COALESCE(document.receipt_comment, ''),
			document.line_count,
			document.amount_without_vat::text,
			document.vat_amount::text,
			document.amount_with_vat::text,
			document.material_receipt_id,
			COALESCE(document.ignored_reason, ''),
			document.ignored_at,
			COALESCE(document.ignored_by, ''),
			document.imported_at,
			COALESCE(document.imported_by, ''),
			COALESCE(document.last_error, ''),
			document.created_at,
			document.updated_at
		FROM integration_diadoc.documents AS document
		LEFT JOIN public.suppliers AS supplier ON supplier.id = document.supplier_id
		LEFT JOIN public.construction_sites AS site ON site.id = document.construction_site_id
		WHERE document.id = $1
	`, id).Scan(
		&document.ID,
		&document.Version,
		&document.Status,
		&document.MessageID,
		&document.EntityID,
		&document.DocumentNumber,
		&document.DocumentDate,
		&document.DocumentFunction,
		&document.DocumentVersion,
		&document.SenderBoxID,
		&document.SenderName,
		&document.SenderINN,
		&document.SenderKPP,
		&document.SupplierID,
		&supplierName,
		&supplierActive,
		&document.ConstructionSiteID,
		&siteName,
		&siteActive,
		&document.ReceiptNumber,
		&document.ReceiptDate,
		&document.ReceiptComment,
		&document.Totals.Document.LineCount,
		&document.AmountWithoutVAT,
		&document.VATAmount,
		&document.AmountWithVAT,
		&document.MaterialReceiptID,
		&document.IgnoredReason,
		&document.IgnoredAt,
		&document.IgnoredBy,
		&document.ImportedAt,
		&document.ImportedBy,
		&document.LastError,
		&document.CreatedAt,
		&document.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if document.SupplierID != nil {
		document.Supplier = reportRef(*document.SupplierID, supplierName)
	}
	if document.ConstructionSiteID != nil {
		document.ConstructionSite = reportRef(*document.ConstructionSiteID, siteName)
	}
	document.Totals.Document.AmountWithoutVAT = document.AmountWithoutVAT
	document.Totals.Document.VATAmount = document.VATAmount
	document.Totals.Document.AmountWithVAT = document.AmountWithVAT

	if err := db.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE NOT is_excluded)::integer,
			COALESCE(sum(source_amount_without_vat) FILTER (WHERE NOT is_excluded), 0)::text,
			COALESCE(sum(source_vat_amount) FILTER (WHERE NOT is_excluded), 0)::text,
			COALESCE(sum(source_amount_with_vat) FILTER (WHERE NOT is_excluded), 0)::text,
			count(*) FILTER (WHERE is_excluded)::integer,
			COALESCE(sum(source_amount_without_vat) FILTER (WHERE is_excluded), 0)::text,
			COALESCE(sum(source_vat_amount) FILTER (WHERE is_excluded), 0)::text,
			COALESCE(sum(source_amount_with_vat) FILTER (WHERE is_excluded), 0)::text
		FROM integration_diadoc.document_items
		WHERE document_id = $1
	`, id).Scan(
		&document.Totals.Import.LineCount,
		&document.Totals.Import.AmountWithoutVAT,
		&document.Totals.Import.VATAmount,
		&document.Totals.Import.AmountWithVAT,
		&document.Totals.Excluded.LineCount,
		&document.Totals.Excluded.AmountWithoutVAT,
		&document.Totals.Excluded.VATAmount,
		&document.Totals.Excluded.AmountWithVAT,
	); err != nil {
		return nil, fmt.Errorf("calculate Diadoc document totals: %w", err)
	}

	rows, err := db.Query(ctx, `
		SELECT
			item.id,
			item.line_num,
			COALESCE(item.source_product_code, ''),
			COALESCE(item.source_article, ''),
			COALESCE(item.source_gtin, ''),
			item.source_name,
			COALESCE(item.source_okei_code, ''),
			COALESCE(item.source_unit_name, ''),
			item.source_quant::text,
			item.source_price::text,
			item.source_amount_without_vat::text,
			item.source_vat_percent::text,
			item.source_vat_amount::text,
			item.source_amount_with_vat::text,
			item.construction_site_id,
			COALESCE(item_site.name, ''),
			COALESCE(item_site.is_active, false),
			item.material_id,
			COALESCE(material.name, ''),
			COALESCE(material.is_active, false),
			item.measure_unit_id,
			COALESCE(unit.name, ''),
			COALESCE(unit.is_active, false),
			COALESCE(material.measure_unit_id = item.measure_unit_id, false),
			item.conversion_factor::text,
			item.import_quant::text,
			item.import_price::text,
			item.import_amount::text,
			item.import_vat_percent::text,
			item.import_vat_amount::text,
			COALESCE(item.import_price >= 0, false),
			item.import_amount IS NOT DISTINCT FROM item.source_amount_with_vat,
			item.import_vat_percent IS NOT DISTINCT FROM item.source_vat_percent,
			item.import_vat_amount IS NOT DISTINCT FROM item.source_vat_amount,
			item.is_excluded,
			COALESCE(item.mapping_source, ''),
			COALESCE(item.last_error, '')
		FROM integration_diadoc.document_items AS item
		LEFT JOIN public.construction_sites AS item_site ON item_site.id = item.construction_site_id
		LEFT JOIN public.materials AS material ON material.id = item.material_id
		LEFT JOIN public.measure_units AS unit ON unit.id = item.measure_unit_id
		WHERE item.document_id = $1
		ORDER BY item.line_num, item.id
	`, id)
	if err != nil {
		return nil, fmt.Errorf("select Diadoc document items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		item := &models.DiadocDocumentItem{Issues: make([]string, 0)}
		var materialName, unitName, itemSiteName string
		var itemSiteActive bool
		var materialActive, unitActive, unitMatchesMaterial bool
		var importPriceValid, importAmountStable, importVATPercentStable, importVATAmountStable bool
		if err := rows.Scan(
			&item.ID,
			&item.LineNum,
			&item.SourceProductCode,
			&item.SourceArticle,
			&item.SourceGTIN,
			&item.SourceName,
			&item.SourceOKEI,
			&item.SourceUnitName,
			&item.SourceQuant,
			&item.SourcePrice,
			&item.SourceAmountWithoutVAT,
			&item.SourceVATPercent,
			&item.SourceVATAmount,
			&item.SourceAmountWithVAT,
			&item.ConstructionSiteID,
			&itemSiteName,
			&itemSiteActive,
			&item.MaterialID,
			&materialName,
			&materialActive,
			&item.MeasureUnitID,
			&unitName,
			&unitActive,
			&unitMatchesMaterial,
			&item.ConversionFactor,
			&item.ImportQuant,
			&item.ImportPrice,
			&item.ImportAmount,
			&item.ImportVATPercent,
			&item.ImportVATAmount,
			&importPriceValid,
			&importAmountStable,
			&importVATPercentStable,
			&importVATAmountStable,
			&item.IsExcluded,
			&item.MappingSource,
			&item.LastError,
		); err != nil {
			return nil, err
		}
		if item.ConstructionSiteID != nil {
			item.ConstructionSite = reportRef(*item.ConstructionSiteID, itemSiteName)
		}
		if !item.IsExcluded {
			if item.ConstructionSiteID == nil && document.ConstructionSiteID == nil {
				appendDiadocItemIssue(document, item, "construction_site_required", "construction_site_id")
			} else if (item.ConstructionSiteID != nil && !itemSiteActive) || (item.ConstructionSiteID == nil && !siteActive) {
				appendDiadocItemIssue(document, item, "construction_site_inactive", "construction_site_id")
			}
		}
		if item.MaterialID != nil {
			item.Material = reportRef(*item.MaterialID, materialName)
			if !item.IsExcluded && !materialActive {
				appendDiadocItemIssue(document, item, "material_inactive", "material_id")
			}
		} else if !item.IsExcluded {
			appendDiadocItemIssue(document, item, "material_required", "material_id")
		}
		if item.MeasureUnitID != nil {
			item.MeasureUnit = reportRef(*item.MeasureUnitID, unitName)
			if !item.IsExcluded && !unitActive {
				appendDiadocItemIssue(document, item, "measure_unit_inactive", "measure_unit_id")
			} else if !item.IsExcluded && !unitMatchesMaterial {
				appendDiadocItemIssue(document, item, "material_measure_unit_mismatch", "measure_unit_id")
			}
		} else if !item.IsExcluded {
			appendDiadocItemIssue(document, item, "measure_unit_required", "measure_unit_id")
		}
		if !item.IsExcluded && item.ConversionFactor == nil {
			appendDiadocItemIssue(document, item, "conversion_factor_required", "conversion_factor")
		}
		if !item.IsExcluded && item.ImportQuant == nil {
			appendDiadocItemIssue(document, item, "import_quantity_required", "import_quant")
		}
		if !item.IsExcluded && !importPriceValid {
			appendDiadocItemIssue(document, item, "import_price_required", "import_price")
		}
		if !item.IsExcluded && !importAmountStable {
			appendDiadocItemIssue(document, item, "import_amount_invalid", "import_amount")
		}
		if !item.IsExcluded && !importVATPercentStable {
			appendDiadocItemIssue(document, item, "import_vat_percent_invalid", "import_vat_percent")
		}
		if !item.IsExcluded && !importVATAmountStable {
			appendDiadocItemIssue(document, item, "import_vat_amount_invalid", "import_vat_amount")
		}
		if item.LastError != "" {
			item.Issues = append(item.Issues, "item_error")
		}
		document.Items = append(document.Items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if document.SupplierID == nil {
		document.Readiness.Missing = append(document.Readiness.Missing, &models.DiadocReadinessIssue{
			Code:  "supplier_required",
			Field: "supplier_id",
		})
	} else if !supplierActive {
		document.Readiness.Missing = append(document.Readiness.Missing, &models.DiadocReadinessIssue{
			Code:  "supplier_inactive",
			Field: "supplier_id",
		})
	}
	if document.ReceiptDate == nil {
		document.Readiness.Missing = append(document.Readiness.Missing, &models.DiadocReadinessIssue{
			Code:  "receipt_date_required",
			Field: "receipt_date",
		})
	}
	if document.ReceiptNumber == "" {
		document.Readiness.Missing = append(document.Readiness.Missing, &models.DiadocReadinessIssue{
			Code:  "receipt_number_required",
			Field: "receipt_number",
		})
	}
	if document.Totals.Import.LineCount == 0 {
		document.Readiness.Missing = append(document.Readiness.Missing, &models.DiadocReadinessIssue{
			Code:  "included_items_required",
			Field: "items",
		})
	}

	if document.SupplierID != nil && document.ReceiptDate != nil && document.ReceiptNumber != "" {
		var duplicateCount int
		if err := db.QueryRow(ctx, `
			SELECT count(*)::integer
			FROM public.material_receipts
			WHERE supplier_id = $1
				AND number = $2
				AND date::date = $3::date
				AND ($4::integer IS NULL OR id <> $4)
		`, document.SupplierID, document.ReceiptNumber, document.ReceiptDate, document.MaterialReceiptID).Scan(&duplicateCount); err != nil {
			return nil, err
		}
		if duplicateCount > 0 {
			document.Readiness.Warnings = append(document.Readiness.Warnings, "possible_duplicate_receipt")
		}
	}

	document.Readiness.Ready = len(document.Readiness.Missing) == 0 && document.Status == "ready"
	return document, nil
}

func appendDiadocItemIssue(
	document *models.DiadocDocumentDetail,
	item *models.DiadocDocumentItem,
	code string,
	field string,
) {
	item.Issues = append(item.Issues, code)
	itemID := item.ID
	document.Readiness.Missing = append(document.Readiness.Missing, &models.DiadocReadinessIssue{
		Code:   code,
		Field:  field,
		ItemID: &itemID,
	})
}
