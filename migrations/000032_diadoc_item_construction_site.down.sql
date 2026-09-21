BEGIN;
ALTER TABLE integration_diadoc.document_items DROP COLUMN construction_site_id;
COMMIT;
