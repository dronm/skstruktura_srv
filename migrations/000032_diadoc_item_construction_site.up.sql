BEGIN;

ALTER TABLE integration_diadoc.document_items
	ADD COLUMN construction_site_id integer REFERENCES public.construction_sites(id)
	ON UPDATE CASCADE ON DELETE RESTRICT;
CREATE INDEX diadoc_document_items_construction_site_id_idx
	ON integration_diadoc.document_items (construction_site_id);
COMMENT ON COLUMN integration_diadoc.document_items.construction_site_id IS
	'Optional item site; NULL inherits the document construction_site_id.';

COMMIT;
