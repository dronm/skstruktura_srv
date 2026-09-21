BEGIN;

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
		'mapping_source',
		'last_error'
	);

DROP TRIGGER IF EXISTS integration_diadoc_documents_audit
	ON integration_diadoc.documents;
CREATE TRIGGER integration_diadoc_documents_audit
	AFTER UPDATE ON integration_diadoc.documents
	FOR EACH ROW EXECUTE FUNCTION integration_diadoc.audit_selected_columns(
		'status',
		'supplier_id',
		'construction_site_id',
		'material_receipt_id',
		'ignored_reason',
		'ignored_at',
		'ignored_by',
		'imported_at',
		'imported_by',
		'last_error'
	);

ALTER TABLE integration_diadoc.document_items
	DROP CONSTRAINT integration_diadoc_document_items_import_values_chk,
	DROP COLUMN is_excluded,
	ALTER COLUMN source_price TYPE numeric(15, 6),
	ALTER COLUMN import_price TYPE numeric(15, 2);

ALTER TABLE integration_diadoc.documents
	DROP COLUMN receipt_comment,
	DROP COLUMN receipt_date,
	DROP COLUMN receipt_number;

ALTER TABLE public.material_receipt_items
	ALTER COLUMN price TYPE numeric(15, 2);

COMMIT;
