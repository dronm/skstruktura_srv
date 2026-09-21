
CREATE OR REPLACE VIEW public.material_statuses_list AS
SELECT
	row.id,
	row.created_at,
	row.material_id,
	public.materials_ref(mat) AS material,
	row.status,
	row.is_active,
	row.construction_site_id,
	public.construction_sites_ref(site) AS construction_site
FROM public.material_statuses AS row
LEFT JOIN public.materials AS mat ON mat.id = row.material_id
LEFT JOIN public.construction_sites AS site ON site.id = row.construction_site_id
;
