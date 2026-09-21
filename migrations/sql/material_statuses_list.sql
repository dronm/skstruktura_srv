
CREATE OR REPLACE VIEW public.material_statuses_list AS
SELECT
	row.id,
	row.created_at,
	row.material_id,
	public.materials_ref(mat) AS material,
	row.status,
	row.is_active
FROM public.material_statuses AS row
LEFT JOIN public.materials AS mat ON mat.id = row.material_id
;
