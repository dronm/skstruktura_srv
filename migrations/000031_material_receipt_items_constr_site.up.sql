BEGIN;

ALTER TABLE public.material_receipt_items
	ADD COLUMN IF NOT EXISTS construction_site_id integer;
ALTER TABLE public.material_receipt_items
	DROP CONSTRAINT IF EXISTS material_receipt_items_construction_site_id_fkey;
ALTER TABLE public.material_receipt_items
	ADD CONSTRAINT material_receipt_items_construction_site_id_fkey
	FOREIGN KEY (construction_site_id) REFERENCES public.construction_sites(id)
	ON UPDATE CASCADE ON DELETE RESTRICT;
ALTER TABLE public.material_receipts
	ALTER COLUMN construction_site_id DROP NOT NULL;
CREATE INDEX IF NOT EXISTS material_receipt_items_construction_site_id_idx
	ON public.material_receipt_items (construction_site_id);
COMMENT ON COLUMN public.material_receipt_items.construction_site_id IS
	'Optional line site. NULL inherits material_receipts.construction_site_id.';

-- Source: migrations/sql/construction_sites_ref.sql
CREATE OR REPLACE FUNCTION public.construction_sites_ref(public.construction_sites)
RETURNS json AS
$BODY$
	SELECT CASE WHEN $1.id IS NULL THEN NULL ELSE json_build_object(
		'keys', json_build_object('id', $1.id),
		'descr', $1.name,
		'dataType', 'constructionSites'
	) END;
$BODY$
LANGUAGE sql STABLE;

-- Source: migrations/sql/measure_units_ref.sql
CREATE OR REPLACE FUNCTION public.measure_units_ref(public.measure_units)
RETURNS json AS
$BODY$
	SELECT CASE WHEN $1.id IS NULL THEN NULL ELSE json_build_object(
		'keys', json_build_object('id', $1.id),
		'descr', $1.name,
		'dataType', 'measureUnits'
	) END;
$BODY$
LANGUAGE sql STABLE;

-- Source: migrations/sql/materials_ref.sql
CREATE OR REPLACE FUNCTION public.materials_ref(public.materials)
RETURNS json AS
$BODY$
	SELECT CASE WHEN $1.id IS NULL THEN NULL ELSE json_build_object(
		'keys', json_build_object('id', $1.id),
		'descr', $1.name,
		'dataType', 'materials'
	) END;
$BODY$
LANGUAGE sql STABLE;

-- Source: migrations/sql/suppliers_ref.sql
CREATE OR REPLACE FUNCTION public.suppliers_ref(public.suppliers)
RETURNS json AS
$BODY$
	SELECT CASE WHEN $1.id IS NULL THEN NULL ELSE json_build_object(
		'keys', json_build_object('id', $1.id),
		'descr', $1.name,
		'dataType', 'suppliers'
	) END;
$BODY$
LANGUAGE sql STABLE;

-- Source: migrations/sql/materials_list.sql
-- View: public.materials_list

-- DROP VIEW public.materials_list;

CREATE OR REPLACE VIEW public.materials_list
 AS
SELECT 
	m.id,
	m.name,
	m.name_full,
	m.measure_unit_id,
	public.measure_units_ref(mu) AS measure_unit,
	m.is_active,
	COALESCE(
		(
			SELECT jsonb_agg(
				jsonb_build_object(
					'construction_site_id', site.id,
					'construction_site', jsonb_build_object(
						'keys', jsonb_build_object('id', site.id),
						'descr', site.name,
						'dataType', 'constructionSites'
					),
					'quant', COALESCE(balance.quant, 0::numeric)
				)
				ORDER BY lower(site.name), site.id
			)
			FROM public.construction_sites AS site
			LEFT JOIN public.rg_materials_current AS balance
				ON balance.construction_site_id = site.id
				AND balance.material_id = m.id
			WHERE site.is_active
		),
		'[]'::jsonb
	) AS balances
FROM public.materials AS m
LEFT JOIN public.measure_units AS mu ON mu.id = m.measure_unit_id
;

-- Source: migrations/sql/material_receipts_list.sql
CREATE OR REPLACE VIEW public.material_receipts_list AS
SELECT
	row.id,
	row.date,
	row.construction_site_id,
	row.supplier_id,
	row.number,
	row.comment,
	row.version,
	public.construction_sites_ref(construction_site_ref_row) AS construction_site,
	public.suppliers_ref(supplier_ref_row) AS supplier
FROM public.material_receipts AS row
LEFT JOIN public.construction_sites AS construction_site_ref_row ON construction_site_ref_row.id = row.construction_site_id
LEFT JOIN public.suppliers AS supplier_ref_row ON supplier_ref_row.id = row.supplier_id;

-- Source: migrations/sql/material_consumptions_list.sql
CREATE OR REPLACE VIEW public.material_consumptions_list AS
SELECT
	row.id,
	row.date,
	row.construction_site_id,
	row.comment,
	row.version,
	public.construction_sites_ref(construction_site_ref_row) AS construction_site
FROM public.material_consumptions AS row
LEFT JOIN public.construction_sites AS construction_site_ref_row ON construction_site_ref_row.id = row.construction_site_id;

-- Source: migrations/sql/material_transfers_list.sql
CREATE OR REPLACE VIEW public.material_transfers_list AS
SELECT
	row.id,
	row.date,
	row.source_construction_site_id,
	row.destination_construction_site_id,
	row.comment,
	row.version,
	public.construction_sites_ref(source_construction_site_ref_row) AS source_construction_site,
	public.construction_sites_ref(destination_construction_site_ref_row) AS destination_construction_site
FROM public.material_transfers AS row
LEFT JOIN public.construction_sites AS source_construction_site_ref_row ON source_construction_site_ref_row.id = row.source_construction_site_id
LEFT JOIN public.construction_sites AS destination_construction_site_ref_row ON destination_construction_site_ref_row.id = row.destination_construction_site_id;

-- Source: migrations/sql/material_receipt_items_list.sql
CREATE OR REPLACE VIEW public.material_receipt_items_list AS
SELECT
	row.id,
	row.line_num,
	row.material_receipt_id,
	row.material_id,
	row.measure_unit_id,
	row.construction_site_id,
	row.quant,
	row.price,
	row.amount,
	row.vat_percent,
	row.vat_amount,
	public.materials_ref(material_ref_row) AS material,
	public.measure_units_ref(measure_unit_ref_row) AS measure_unit,
	public.construction_sites_ref(construction_site_ref_row) AS construction_site
FROM public.material_receipt_items AS row
LEFT JOIN public.materials AS material_ref_row ON material_ref_row.id = row.material_id
LEFT JOIN public.measure_units AS measure_unit_ref_row ON measure_unit_ref_row.id = row.measure_unit_id
LEFT JOIN public.construction_sites AS construction_site_ref_row ON construction_site_ref_row.id = row.construction_site_id;

-- Source: migrations/sql/material_consumption_items_list.sql
CREATE OR REPLACE VIEW public.material_consumption_items_list AS
SELECT
	row.id,
	row.line_num,
	row.material_consumption_id,
	row.material_id,
	row.measure_unit_id,
	row.quant,
	public.materials_ref(material_ref_row) AS material,
	public.measure_units_ref(measure_unit_ref_row) AS measure_unit
FROM public.material_consumption_items AS row
LEFT JOIN public.materials AS material_ref_row ON material_ref_row.id = row.material_id
LEFT JOIN public.measure_units AS measure_unit_ref_row ON measure_unit_ref_row.id = row.measure_unit_id;

-- Source: migrations/sql/material_transfer_items_list.sql
CREATE OR REPLACE VIEW public.material_transfer_items_list AS
SELECT
	row.id,
	row.line_num,
	row.material_transfer_id,
	row.material_id,
	row.measure_unit_id,
	row.quant,
	public.materials_ref(material_ref_row) AS material,
	public.measure_units_ref(measure_unit_ref_row) AS measure_unit
FROM public.material_transfer_items AS row
LEFT JOIN public.materials AS material_ref_row ON material_ref_row.id = row.material_id
LEFT JOIN public.measure_units AS measure_unit_ref_row ON measure_unit_ref_row.id = row.measure_unit_id;

COMMIT;
