package diadoc

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type StageDocumentInput struct {
	BoxID            string
	MessageID        string
	EntityID         string
	ParentEntityID   string
	EventID          string
	EventIndexKey    string
	EventTimestamp   time.Time
	AttachmentType   string
	TypeNamedID      string
	DocumentFunction string
	DocumentVersion  string
	DocumentNumber   string
	DocumentDate     *time.Time
	FileName         string
	SenderBoxID      string
	SenderName       string
	SenderINN        string
	SenderKPP        string
	Content          []byte
	Parsed           *ParsedDocument
	StageError       error
}

type StageDocumentResult struct {
	DocumentID int64
	Added      bool
	Updated    bool
	Failed     bool
}

type BufferedDocumentSource struct {
	ID               int64
	Version          int64
	BoxID            string
	MessageID        string
	EntityID         string
	ParentEntityID   string
	EventID          string
	EventIndexKey    string
	EventTimestamp   time.Time
	AttachmentType   string
	TypeNamedID      string
	DocumentFunction string
	DocumentVersion  string
	DocumentNumber   string
	DocumentDate     *time.Time
	FileName         string
	SenderBoxID      string
	SenderName       string
	SenderINN        string
	SenderKPP        string
	Status           string
}

type preservedItemResolution struct {
	MaterialID       *int
	MeasureUnitID    *int
	ConversionFactor string
	MappingSource    string
	IsExcluded       bool
}

func (s *Store) StageDocument(ctx context.Context, input StageDocumentInput) (StageDocumentResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return StageDocumentResult{}, fmt.Errorf("begin Diadoc document staging: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	result, err := stageDocumentTx(ctx, tx, input)
	if err != nil {
		return StageDocumentResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return StageDocumentResult{}, fmt.Errorf("commit Diadoc document staging: %w", err)
	}
	return result, nil
}

func stageDocumentTx(
	ctx context.Context,
	tx pgx.Tx,
	input StageDocumentInput,
) (StageDocumentResult, error) {
	if strings.TrimSpace(input.BoxID) == "" || strings.TrimSpace(input.MessageID) == "" || strings.TrimSpace(input.EntityID) == "" {
		return StageDocumentResult{}, fmt.Errorf("Diadoc document identity is incomplete")
	}

	var existingID int64
	var existingStatus string
	var existingSupplierID *int
	var existingSiteID *int
	err := tx.QueryRow(ctx, `
		SELECT id, status, supplier_id, construction_site_id
		FROM integration_diadoc.documents
		WHERE box_id = $1
			AND message_id = $2
			AND entity_id = $3
		FOR UPDATE
	`, input.BoxID, input.MessageID, input.EntityID).Scan(
		&existingID,
		&existingStatus,
		&existingSupplierID,
		&existingSiteID,
	)
	if err != nil && err != pgx.ErrNoRows {
		return StageDocumentResult{}, fmt.Errorf("lock buffered Diadoc document: %w", err)
	}

	added := err == pgx.ErrNoRows
	protected := !added && (existingStatus == "imported" || existingStatus == "ignored")
	contentHash := sha256.Sum256(input.Content)
	var contentHashValue any
	if len(input.Content) > 0 {
		contentHashValue = contentHash[:]
	}
	var parsedPayload []byte
	if input.Parsed != nil {
		parsedPayload, err = json.Marshal(input.Parsed)
		if err != nil {
			return StageDocumentResult{}, fmt.Errorf("serialize parsed Diadoc document: %w", err)
		}
	}

	status := "needs_matching"
	lastError := ""
	if input.StageError != nil {
		status = "failed"
		lastError = input.StageError.Error()
	}
	if protected {
		status = existingStatus
	}

	if existingSupplierID == nil && input.Parsed != nil {
		existingSupplierID, err = resolveSupplierTx(
			ctx,
			tx,
			input.BoxID,
			input.SenderBoxID,
			firstNonEmpty(input.Parsed.Sender.INN, input.SenderINN),
			firstNonEmpty(input.Parsed.Sender.KPP, input.SenderKPP),
		)
		if err != nil {
			return StageDocumentResult{}, err
		}
	}

	lineCount := 0
	amountWithoutVAT := "0"
	vatAmount := "0"
	amountWithVAT := "0"
	if input.Parsed != nil {
		lineCount = len(input.Parsed.Items)
		amountWithoutVAT = input.Parsed.Totals.AmountWithoutVAT
		vatAmount = input.Parsed.Totals.VATAmount
		amountWithVAT = input.Parsed.Totals.AmountWithVAT
	}

	var documentID int64
	if added {
		err = tx.QueryRow(ctx, `
			INSERT INTO integration_diadoc.documents (
				box_id,
				message_id,
				entity_id,
				parent_entity_id,
				event_id,
				event_index_key,
				event_timestamp,
				status,
				attachment_type,
				type_named_id,
				document_function,
				document_version,
				document_number,
				document_date,
				receipt_number,
				receipt_date,
				file_name,
				sender_box_id,
				sender_name,
				sender_inn,
				sender_kpp,
				supplier_id,
				construction_site_id,
				raw_content,
				content_sha256,
				parsed_payload,
				line_count,
				amount_without_vat,
				vat_amount,
				amount_with_vat,
				last_error
			)
			VALUES (
				$1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''), $7, $8,
				$9, NULLIF($10, ''), NULLIF($11, ''), NULLIF($12, ''), NULLIF($13, ''),
				$14, NULLIF($13, ''), $14::date::timestamp AT TIME ZONE public.register_business_timezone(),
				NULLIF($15, ''), NULLIF($16, ''), NULLIF($17, ''), NULLIF($18, ''),
				NULLIF($19, ''), $20, $21, $22, $23, $24, $25, $26, $27, $28, NULLIF($29, '')
			)
			RETURNING id
		`,
			input.BoxID,
			input.MessageID,
			input.EntityID,
			input.ParentEntityID,
			input.EventID,
			input.EventIndexKey,
			nullableTime(input.EventTimestamp),
			status,
			input.AttachmentType,
			input.TypeNamedID,
			input.DocumentFunction,
			input.DocumentVersion,
			input.DocumentNumber,
			input.DocumentDate,
			input.FileName,
			input.SenderBoxID,
			firstNonEmpty(parsedSenderName(input.Parsed), input.SenderName),
			firstNonEmpty(parsedSenderINN(input.Parsed), input.SenderINN),
			firstNonEmpty(parsedSenderKPP(input.Parsed), input.SenderKPP),
			existingSupplierID,
			existingSiteID,
			nullableBytes(input.Content),
			contentHashValue,
			nullableJSON(parsedPayload),
			lineCount,
			amountWithoutVAT,
			vatAmount,
			amountWithVAT,
			lastError,
		).Scan(&documentID)
		if err == nil {
			_, err = tx.Exec(ctx, `
				UPDATE integration_diadoc.documents
				SET receipt_comment = 'Импортировано из Диадока'
				WHERE id = $1
			`, documentID)
		}
	} else {
		documentID = existingID
		_, err = tx.Exec(ctx, `
			UPDATE integration_diadoc.documents
			SET
				parent_entity_id = NULLIF($2, ''),
				event_id = NULLIF($3, ''),
				event_index_key = NULLIF($4, ''),
				event_timestamp = COALESCE($5, event_timestamp),
				status = $6,
				attachment_type = $7,
				type_named_id = NULLIF($8, ''),
				document_function = NULLIF($9, ''),
				document_version = NULLIF($10, ''),
				document_number = NULLIF($11, ''),
				document_date = $12,
				receipt_number = COALESCE(receipt_number, NULLIF($11, '')),
				receipt_date = COALESCE(
					receipt_date,
					$12::date::timestamp AT TIME ZONE public.register_business_timezone()
				),
				receipt_comment = COALESCE(
					receipt_comment,
					'Импортировано из Диадока'
				),
				file_name = NULLIF($13, ''),
				sender_box_id = NULLIF($14, ''),
				sender_name = NULLIF($15, ''),
				sender_inn = NULLIF($16, ''),
				sender_kpp = NULLIF($17, ''),
				supplier_id = COALESCE(supplier_id, $18),
				raw_content = COALESCE($19, raw_content),
				content_sha256 = CASE WHEN $19 IS NULL THEN content_sha256 ELSE $20 END,
				parsed_payload = COALESCE($21, parsed_payload),
				line_count = CASE WHEN $21 IS NULL THEN line_count ELSE $22 END,
				amount_without_vat = CASE WHEN $21 IS NULL THEN amount_without_vat ELSE $23 END,
				vat_amount = CASE WHEN $21 IS NULL THEN vat_amount ELSE $24 END,
				amount_with_vat = CASE WHEN $21 IS NULL THEN amount_with_vat ELSE $25 END,
				last_error = CASE WHEN $27 THEN last_error ELSE NULLIF($26, '') END,
				updated_at = now(),
				version = CASE WHEN $27 THEN version ELSE version + 1 END
			WHERE id = $1
		`,
			documentID,
			input.ParentEntityID,
			input.EventID,
			input.EventIndexKey,
			nullableTime(input.EventTimestamp),
			status,
			input.AttachmentType,
			input.TypeNamedID,
			input.DocumentFunction,
			input.DocumentVersion,
			input.DocumentNumber,
			input.DocumentDate,
			input.FileName,
			input.SenderBoxID,
			firstNonEmpty(parsedSenderName(input.Parsed), input.SenderName),
			firstNonEmpty(parsedSenderINN(input.Parsed), input.SenderINN),
			firstNonEmpty(parsedSenderKPP(input.Parsed), input.SenderKPP),
			existingSupplierID,
			nullableBytes(input.Content),
			contentHashValue,
			nullableJSON(parsedPayload),
			lineCount,
			amountWithoutVAT,
			vatAmount,
			amountWithVAT,
			lastError,
			protected,
		)
	}
	if err != nil {
		return StageDocumentResult{}, fmt.Errorf("store buffered Diadoc document: %w", err)
	}

	if input.Parsed != nil && !protected {
		ready, stageErr := replaceDocumentItemsTx(
			ctx,
			tx,
			documentID,
			existingSupplierID,
			existingSiteID,
			input.Parsed.Items,
		)
		if stageErr != nil {
			return StageDocumentResult{}, stageErr
		}
		status = "needs_matching"
		if ready {
			status = "ready"
		}
		if _, err := tx.Exec(ctx, `
			UPDATE integration_diadoc.documents
			SET status = $2, last_error = NULL, updated_at = now()
			WHERE id = $1
		`, documentID, status); err != nil {
			return StageDocumentResult{}, fmt.Errorf("update buffered Diadoc readiness: %w", err)
		}
	}

	return StageDocumentResult{
		DocumentID: documentID,
		Added:      added,
		Updated:    !added,
		Failed:     input.StageError != nil,
	}, nil
}

func replaceDocumentItemsTx(
	ctx context.Context,
	tx pgx.Tx,
	documentID int64,
	supplierID *int,
	constructionSiteID *int,
	items []ParsedItem,
) (bool, error) {
	preserved := make(map[int]preservedItemResolution)
	rows, err := tx.Query(ctx, `
		SELECT
			line_num,
			material_id,
			measure_unit_id,
			COALESCE(conversion_factor::text, ''),
			COALESCE(mapping_source, ''),
			is_excluded
		FROM integration_diadoc.document_items
		WHERE document_id = $1
	`, documentID)
	if err != nil {
		return false, fmt.Errorf("load buffered Diadoc item resolutions: %w", err)
	}
	for rows.Next() {
		var line int
		var value preservedItemResolution
		if err := rows.Scan(
			&line,
			&value.MaterialID,
			&value.MeasureUnitID,
			&value.ConversionFactor,
			&value.MappingSource,
			&value.IsExcluded,
		); err != nil {
			rows.Close()
			return false, err
		}
		preserved[line] = value
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return false, err
	}
	rows.Close()

	ready := supplierID != nil && *supplierID > 0
	lineNumbers := make([]int, 0, len(items))
	for _, source := range items {
		lineNumbers = append(lineNumbers, source.LineNum)
		resolution := preserved[source.LineNum]
		if resolution.MaterialID == nil && supplierID != nil {
			isExcluded := resolution.IsExcluded
			resolution, err = resolveMaterialTx(ctx, tx, *supplierID, source)
			if err != nil {
				return false, err
			}
			resolution.IsExcluded = isExcluded
		}

		var importQuant any
		var importPrice any
		var importAmount any
		var importVATPercent any
		var importVATAmount any
		if resolution.MaterialID != nil && resolution.MeasureUnitID != nil && resolution.ConversionFactor != "" {
			quant := multiplyDecimals(source.Quant, resolution.ConversionFactor, 4)
			price := divideDecimals(source.AmountWithVAT, quant, 6)
			importQuant = quant
			importPrice = price
			importAmount = source.AmountWithVAT
			importVATPercent = source.VATPercent
			importVATAmount = source.VATAmount
		} else if !resolution.IsExcluded {
			ready = false
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO integration_diadoc.document_items (
				document_id,
				line_num,
				source_product_code,
				source_article,
				source_gtin,
				source_name,
				source_okei_code,
				source_unit_name,
				source_quant,
				source_price,
				source_amount_without_vat,
				source_vat_percent,
				source_vat_amount,
				source_amount_with_vat,
				material_id,
				measure_unit_id,
				conversion_factor,
				import_quant,
				import_price,
				import_amount,
				import_vat_percent,
				import_vat_amount,
				mapping_source
			)
			VALUES (
				$1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), $6,
				NULLIF($7, ''), NULLIF($8, ''), $9, $10, $11, $12, $13, $14,
				$15, $16, NULLIF($17, '')::numeric, $18, $19, $20, $21, $22, NULLIF($23, '')
			)
			ON CONFLICT (document_id, line_num) DO UPDATE
			SET
				source_product_code = EXCLUDED.source_product_code,
				source_article = EXCLUDED.source_article,
				source_gtin = EXCLUDED.source_gtin,
				source_name = EXCLUDED.source_name,
				source_okei_code = EXCLUDED.source_okei_code,
				source_unit_name = EXCLUDED.source_unit_name,
				source_quant = EXCLUDED.source_quant,
				source_price = EXCLUDED.source_price,
				source_amount_without_vat = EXCLUDED.source_amount_without_vat,
				source_vat_percent = EXCLUDED.source_vat_percent,
				source_vat_amount = EXCLUDED.source_vat_amount,
				source_amount_with_vat = EXCLUDED.source_amount_with_vat,
				material_id = EXCLUDED.material_id,
				measure_unit_id = EXCLUDED.measure_unit_id,
				conversion_factor = EXCLUDED.conversion_factor,
				import_quant = EXCLUDED.import_quant,
				import_price = EXCLUDED.import_price,
				import_amount = EXCLUDED.import_amount,
				import_vat_percent = EXCLUDED.import_vat_percent,
				import_vat_amount = EXCLUDED.import_vat_amount,
				mapping_source = EXCLUDED.mapping_source,
				last_error = NULL
		`,
			documentID,
			source.LineNum,
			source.ProductCode,
			source.Article,
			source.GTIN,
			source.Name,
			source.OKEI,
			source.UnitName,
			source.Quant,
			source.Price,
			source.AmountWithoutVAT,
			source.VATPercent,
			source.VATAmount,
			source.AmountWithVAT,
			resolution.MaterialID,
			resolution.MeasureUnitID,
			resolution.ConversionFactor,
			importQuant,
			importPrice,
			importAmount,
			importVATPercent,
			importVATAmount,
			resolution.MappingSource,
		)
		if err != nil {
			return false, fmt.Errorf("store buffered Diadoc item %d: %w", source.LineNum, err)
		}
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM integration_diadoc.document_items
		WHERE document_id = $1
			AND NOT (line_num = ANY($2::integer[]))
	`, documentID, lineNumbers); err != nil {
		return false, fmt.Errorf("remove stale buffered Diadoc items: %w", err)
	}
	if ready {
		if err := tx.QueryRow(ctx, `
			SELECT
				EXISTS (
					SELECT 1
					FROM integration_diadoc.documents AS document
					JOIN public.suppliers AS supplier
						ON supplier.id = document.supplier_id
						AND supplier.is_active
					WHERE document.id = $1
						AND document.receipt_date IS NOT NULL
						AND COALESCE(document.receipt_number, '') <> ''
				)
				AND EXISTS (
					SELECT 1
					FROM integration_diadoc.document_items AS included_item
					WHERE included_item.document_id = $1
						AND NOT included_item.is_excluded
				)
				AND NOT EXISTS (
					SELECT 1
					FROM integration_diadoc.document_items AS item
					LEFT JOIN public.materials AS material
						ON material.id = item.material_id
					LEFT JOIN public.measure_units AS unit
						ON unit.id = item.measure_unit_id
					WHERE item.document_id = $1
						AND NOT item.is_excluded
						AND (
							material.id IS NULL
							OR NOT EXISTS (
								SELECT 1 FROM public.construction_sites AS effective_site
								JOIN integration_diadoc.documents AS document ON document.id = item.document_id
								WHERE effective_site.id = COALESCE(item.construction_site_id, document.construction_site_id)
									AND effective_site.is_active
							)
							OR NOT material.is_active
							OR unit.id IS NULL
							OR NOT unit.is_active
							OR material.measure_unit_id <> item.measure_unit_id
						)
				)
		`, documentID).Scan(&ready); err != nil {
			return false, err
		}
	}
	return ready, nil
}

func resolveSupplierTx(
	ctx context.Context,
	tx pgx.Tx,
	boxID string,
	counteragentBoxID string,
	inn string,
	kpp string,
) (*int, error) {
	var supplierID int
	err := tx.QueryRow(ctx, `
		SELECT supplier.id
		FROM integration_diadoc.supplier_matches AS match
		JOIN public.suppliers AS supplier
			ON supplier.id = match.supplier_id
			AND supplier.is_active
		WHERE match.box_id = $1
			AND match.counteragent_box_id = $2
	`, boxID, counteragentBoxID).Scan(&supplierID)
	if err == nil {
		return &supplierID, nil
	}
	if err != pgx.ErrNoRows {
		return nil, fmt.Errorf("resolve Diadoc supplier mapping: %w", err)
	}
	if strings.TrimSpace(inn) == "" {
		return nil, nil
	}

	rows, err := tx.Query(ctx, `
		SELECT id
		FROM public.suppliers
		WHERE is_active
			AND inn = $1
			AND ($2 = '' OR COALESCE(kpp, '') = $2)
		ORDER BY id
		LIMIT 2
	`, strings.TrimSpace(inn), strings.TrimSpace(kpp))
	if err != nil {
		return nil, fmt.Errorf("resolve Diadoc supplier by INN/KPP: %w", err)
	}
	defer rows.Close()
	ids := make([]int, 0, 2)
	for rows.Next() {
		if err := rows.Scan(&supplierID); err != nil {
			return nil, err
		}
		ids = append(ids, supplierID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 1 {
		return &ids[0], nil
	}
	return nil, nil
}

func resolveMaterialTx(
	ctx context.Context,
	tx pgx.Tx,
	supplierID int,
	item ParsedItem,
) (preservedItemResolution, error) {
	type candidate struct {
		kind string
		key  string
	}
	candidates := []candidate{
		{kind: "product_code", key: item.ProductCode},
		{kind: "article", key: item.Article},
		{kind: "gtin", key: item.GTIN},
		{kind: "name", key: normalizeMatchKey(item.Name)},
	}
	for _, candidate := range candidates {
		if candidate.key == "" {
			continue
		}
		var result preservedItemResolution
		var materialID, measureUnitID int
		err := tx.QueryRow(ctx, `
			SELECT
				match.material_id,
				material.measure_unit_id,
				match.conversion_factor::text
			FROM integration_diadoc.supplier_material_matches AS match
			JOIN public.materials AS material
				ON material.id = match.material_id
				AND material.is_active
			WHERE match.supplier_id = $1
				AND match.source_key_kind = $2
				AND match.source_key = $3
				AND COALESCE(match.source_okei_code, '') = $4
		`, supplierID, candidate.kind, candidate.key, item.OKEI).Scan(
			&materialID,
			&measureUnitID,
			&result.ConversionFactor,
		)
		if err == pgx.ErrNoRows {
			continue
		}
		if err != nil {
			return preservedItemResolution{}, fmt.Errorf("resolve Diadoc material mapping: %w", err)
		}
		result.MaterialID = &materialID
		result.MeasureUnitID = &measureUnitID
		result.MappingSource = "cached"
		return result, nil
	}
	return preservedItemResolution{}, nil
}

func (s *Store) BufferedDocumentSource(ctx context.Context, id int64) (BufferedDocumentSource, error) {
	var result BufferedDocumentSource
	err := s.pool.QueryRow(ctx, `
		SELECT
			id,
			version,
			box_id,
			message_id,
			entity_id,
			COALESCE(parent_entity_id, ''),
			COALESCE(event_id, ''),
			COALESCE(event_index_key, ''),
			COALESCE(event_timestamp, to_timestamp(0)),
			attachment_type,
			COALESCE(type_named_id, ''),
			COALESCE(document_function, ''),
			COALESCE(document_version, ''),
			COALESCE(document_number, ''),
			document_date,
			COALESCE(file_name, ''),
			COALESCE(sender_box_id, ''),
			COALESCE(sender_name, ''),
			COALESCE(sender_inn, ''),
			COALESCE(sender_kpp, ''),
			status
		FROM integration_diadoc.documents
		WHERE id = $1
	`, id).Scan(
		&result.ID,
		&result.Version,
		&result.BoxID,
		&result.MessageID,
		&result.EntityID,
		&result.ParentEntityID,
		&result.EventID,
		&result.EventIndexKey,
		&result.EventTimestamp,
		&result.AttachmentType,
		&result.TypeNamedID,
		&result.DocumentFunction,
		&result.DocumentVersion,
		&result.DocumentNumber,
		&result.DocumentDate,
		&result.FileName,
		&result.SenderBoxID,
		&result.SenderName,
		&result.SenderINN,
		&result.SenderKPP,
		&result.Status,
	)
	if err != nil {
		return BufferedDocumentSource{}, err
	}
	return result, nil
}

func nullableBytes(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}

func nullableJSON(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return string(value)
}

func parsedSenderName(document *ParsedDocument) string {
	if document == nil {
		return ""
	}
	return document.Sender.Name
}

func parsedSenderINN(document *ParsedDocument) string {
	if document == nil {
		return ""
	}
	return document.Sender.INN
}

func parsedSenderKPP(document *ParsedDocument) string {
	if document == nil {
		return ""
	}
	return document.Sender.KPP
}

func multiplyDecimals(left string, right string, scale int) string {
	result := new(big.Rat).Mul(decimalRat(left), decimalRat(right))
	return ratDecimal(result, scale)
}

func divideDecimals(numerator string, denominator string, scale int) string {
	divisor := decimalRat(denominator)
	if divisor.Sign() == 0 {
		return "0"
	}
	result := new(big.Rat).Quo(decimalRat(numerator), divisor)
	return ratDecimal(result, scale)
}
