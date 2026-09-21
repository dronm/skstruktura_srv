BEGIN;

DELETE FROM public.role_permissions
WHERE permission_code IN (
	'constructionManager.materialStatus.list',
	'constructionManager.materialStatus.create'
);

DELETE FROM public.permissions
WHERE code IN (
	'constructionManager.materialStatus.list',
	'constructionManager.materialStatus.create'
);

DROP VIEW public.material_statuses_list;

DROP INDEX IF EXISTS public.material_statuses_site_history_idx;
DROP INDEX IF EXISTS public.material_statuses_material_latest_idx;

ALTER TABLE public.material_statuses
	DROP COLUMN construction_site_id;

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

COMMIT;
