BEGIN;

DELETE FROM public.audit_column_aliases
WHERE (table_name, column_name) IN (
	('material_receipt_items', 'vat_percent'),
	('material_receipt_items', 'vat_amount'),
	('measure_units', 'okei_code')
);

DROP TRIGGER IF EXISTS integration_diadoc_supplier_material_matches_audit
	ON integration_diadoc.supplier_material_matches;
DROP TRIGGER IF EXISTS integration_diadoc_supplier_matches_audit
	ON integration_diadoc.supplier_matches;
DROP TRIGGER IF EXISTS integration_diadoc_state_audit
	ON integration_diadoc.state;
DROP TRIGGER IF EXISTS integration_diadoc_document_items_audit
	ON integration_diadoc.document_items;
DROP TRIGGER IF EXISTS integration_diadoc_documents_audit
	ON integration_diadoc.documents;
DROP FUNCTION IF EXISTS integration_diadoc.audit_selected_columns();

DROP TABLE IF EXISTS integration_diadoc.supplier_material_matches;
DROP TABLE IF EXISTS integration_diadoc.supplier_matches;
DROP TABLE IF EXISTS integration_diadoc.document_items;
DROP TABLE IF EXISTS integration_diadoc.documents;

ALTER TABLE integration_diadoc.state
	DROP CONSTRAINT IF EXISTS integration_diadoc_state_version_chk,
	DROP COLUMN IF EXISTS version,
	DROP COLUMN IF EXISTS cursor_reset_by,
	DROP COLUMN IF EXISTS cursor_reset_at,
	DROP COLUMN IF EXISTS last_sync_error,
	DROP COLUMN IF EXISTS last_sync_finished_at,
	DROP COLUMN IF EXISTS last_sync_started_at,
	DROP COLUMN IF EXISTS event_timestamp_from,
	DROP COLUMN IF EXISTS enabled;

DROP INDEX IF EXISTS public.audit_log_object_history_idx;
ALTER TABLE public.audit_log
	DROP COLUMN IF EXISTS schema_name;
CREATE INDEX audit_log_object_history_idx
	ON public.audit_log (table_name, record_id, changed_at DESC, id DESC);

DROP INDEX IF EXISTS public.measure_units_okei_code_idx;
ALTER TABLE public.measure_units
	DROP COLUMN IF EXISTS okei_code;

DELETE FROM public.role_permissions
WHERE permission_code IN (
	'diadocDocument.list',
	'diadocDocument.detail',
	'diadocDocument.resolve',
	'diadocDocument.ignore',
	'diadocDocument.import',
	'diadocState.view',
	'diadocState.update',
	'diadoc.sync'
);

DELETE FROM public.permissions
WHERE code IN (
	'diadocDocument.list',
	'diadocDocument.detail',
	'diadocDocument.resolve',
	'diadocDocument.ignore',
	'diadocDocument.import',
	'diadocState.view',
	'diadocState.update',
	'diadoc.sync'
);

COMMIT;
