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
