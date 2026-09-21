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
