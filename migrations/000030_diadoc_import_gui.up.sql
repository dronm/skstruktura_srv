BEGIN;

ALTER TABLE public.material_receipt_items
	ALTER COLUMN price TYPE numeric(19, 6);

ALTER TABLE integration_diadoc.documents
	ADD COLUMN receipt_number text,
	ADD COLUMN receipt_date timestamptz,
	ADD COLUMN receipt_comment text;

UPDATE integration_diadoc.documents
SET
	receipt_number = document_number,
	receipt_date = document_date::timestamp AT TIME ZONE public.register_business_timezone(),
	receipt_comment = 'Imported from Diadoc buffer document ' || id::text;

ALTER TABLE integration_diadoc.document_items
	ALTER COLUMN source_price TYPE numeric(19, 6),
	ALTER COLUMN import_price TYPE numeric(19, 6),
	ADD COLUMN is_excluded boolean NOT NULL DEFAULT false,
	ADD CONSTRAINT integration_diadoc_document_items_import_values_chk CHECK (
		(import_price IS NULL OR import_price >= 0)
		AND (import_amount IS NULL OR import_amount >= 0)
		AND (import_vat_percent IS NULL OR (import_vat_percent >= 0 AND import_vat_percent <= 100))
		AND (import_vat_amount IS NULL OR import_vat_amount >= 0)
		AND (import_amount IS NULL OR import_vat_amount IS NULL OR import_vat_amount <= import_amount)
	);

COMMENT ON COLUMN integration_diadoc.documents.receipt_number IS
	'User-editable number of the material receipt created by import.';
COMMENT ON COLUMN integration_diadoc.documents.receipt_date IS
	'User-editable date and time of the material receipt created by import.';
COMMENT ON COLUMN integration_diadoc.documents.receipt_comment IS
	'User-editable comment of the material receipt created by import.';
COMMENT ON COLUMN integration_diadoc.document_items.is_excluded IS
	'Whether the source line is retained in the buffer but excluded from receipt import.';
COMMENT ON COLUMN integration_diadoc.document_items.import_price IS
	'Gross target-unit price derived from the immutable source gross amount and converted quantity.';

DROP TRIGGER IF EXISTS integration_diadoc_documents_audit
	ON integration_diadoc.documents;
CREATE TRIGGER integration_diadoc_documents_audit
	AFTER UPDATE ON integration_diadoc.documents
	FOR EACH ROW EXECUTE FUNCTION integration_diadoc.audit_selected_columns(
		'status',
		'supplier_id',
		'construction_site_id',
		'receipt_number',
		'receipt_date',
		'receipt_comment',
		'material_receipt_id',
		'ignored_reason',
		'ignored_at',
		'ignored_by',
		'imported_at',
		'imported_by',
		'last_error'
	);

DROP TRIGGER IF EXISTS integration_diadoc_document_items_audit
	ON integration_diadoc.document_items;
CREATE TRIGGER integration_diadoc_document_items_audit
	AFTER UPDATE ON integration_diadoc.document_items
	FOR EACH ROW EXECUTE FUNCTION integration_diadoc.audit_selected_columns(
		'material_id',
		'measure_unit_id',
		'conversion_factor',
		'import_quant',
		'import_price',
		'import_amount',
		'import_vat_percent',
		'import_vat_amount',
		'is_excluded',
		'mapping_source',
		'last_error'
	);

COMMIT;
