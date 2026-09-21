BEGIN;

ALTER TABLE public.material_statuses
	ADD COLUMN construction_site_id integer
		REFERENCES public.construction_sites(id)
		ON DELETE RESTRICT
		ON UPDATE CASCADE;

COMMENT ON COLUMN public.material_statuses.construction_site_id IS
	'Construction site where the status change was recorded. Legacy rows remain NULL.';

CREATE INDEX material_statuses_material_latest_idx
	ON public.material_statuses (material_id, created_at DESC, id DESC)
	WHERE is_active;

CREATE INDEX material_statuses_site_history_idx
	ON public.material_statuses (construction_site_id, created_at DESC, id DESC)
	WHERE is_active AND construction_site_id IS NOT NULL;

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

WITH required(code, description) AS (
	VALUES
		(
			'constructionManager.materialStatus.list',
			'Просмотр текущих статусов материалов и истории в рабочем месте прораба'
		),
		(
			'constructionManager.materialStatus.create',
			'Изменение статуса материала в рабочем месте прораба'
		)
)
INSERT INTO public.permissions (code, description)
SELECT required.code, required.description
FROM required
ON CONFLICT (code) DO UPDATE
SET description = EXCLUDED.description;

WITH roles(role_id) AS (
	VALUES
		('admin'::public.role_types),
		('construction_site_manager'::public.role_types)
), required(permission_code) AS (
	VALUES
		('constructionManager.materialStatus.list'),
		('constructionManager.materialStatus.create')
)
INSERT INTO public.role_permissions (role_id, permission_code)
SELECT roles.role_id, required.permission_code
FROM roles
CROSS JOIN required
ON CONFLICT DO NOTHING;

COMMIT;
