BEGIN;

CREATE OR REPLACE VIEW public.material_balances_list AS
SELECT
	material.id,
	balance.construction_site_id,
	public.construction_sites_ref(site) AS construction_site,
	material.material_type_id,
	public.material_types_ref(material_type) AS material_type,
	material_type.name AS material_type_name,
	balance.material_id,
	public.materials_ref(material) AS material,
	material.name AS material_name,
	material.measure_unit_id,
	public.measure_units_ref(measure_unit) AS measure_unit,
	balance.quant::numeric(19, 4) AS balance
FROM public.rg_materials_current AS balance
JOIN public.construction_sites AS site
	ON site.id = balance.construction_site_id
JOIN public.materials AS material
	ON material.id = balance.material_id
JOIN public.material_types AS material_type
	ON material_type.id = material.material_type_id
JOIN public.measure_units AS measure_unit
	ON measure_unit.id = material.measure_unit_id
WHERE balance.quant <> 0;

COMMENT ON VIEW public.material_balances_list IS
	'Current non-zero material balances with material-type grouping metadata.';

INSERT INTO public.permissions (code, description)
VALUES ('materialBalance.list', 'Просмотр текущих остатков материалов')
ON CONFLICT (code) DO UPDATE
SET description = EXCLUDED.description;

WITH roles(role_id) AS (
	VALUES
		('admin'::public.role_types),
		('construction_site_manager'::public.role_types)
)
INSERT INTO public.role_permissions (role_id, permission_code)
SELECT roles.role_id, 'materialBalance.list'
FROM roles
ON CONFLICT DO NOTHING;

INSERT INTO public.application_routes (
	name,
	path,
	descr,
	section,
	icon,
	menu_available,
	is_active
)
VALUES (
	'materialBalance',
	'/material-balance',
	'Остатки материалов',
	'Отчёты',
	'pi pi-chart-bar',
	true,
	true
)
ON CONFLICT (name) DO UPDATE
SET
	path = EXCLUDED.path,
	descr = EXCLUDED.descr,
	section = EXCLUDED.section,
	icon = EXCLUDED.icon,
	menu_available = EXCLUDED.menu_available,
	is_active = true;

WITH roles(role_id) AS (
	VALUES
		('admin'::public.role_types),
		('construction_site_manager'::public.role_types)
)
INSERT INTO public.main_menus (
	role_id,
	caption,
	icon,
	sort_order,
	is_active
)
SELECT roles.role_id, 'Материалы', NULL, 40, true
FROM roles
WHERE NOT EXISTS (
	SELECT 1
	FROM public.main_menus AS existing
	WHERE existing.role_id = roles.role_id
		AND existing.user_id IS NULL
		AND existing.parent_id IS NULL
		AND existing.caption = 'Материалы'
);

WITH roles(role_id) AS (
	VALUES
		('admin'::public.role_types),
		('construction_site_manager'::public.role_types)
)
UPDATE public.main_menus AS existing
SET
	parent_id = parent.id,
	caption = 'Остатки материалов',
	icon = NULL,
	sort_order = 50,
	is_active = true
FROM roles
JOIN public.application_routes AS route
	ON route.name = 'materialBalance'
JOIN LATERAL (
	SELECT menu.id
	FROM public.main_menus AS menu
	WHERE menu.role_id = roles.role_id
		AND menu.user_id IS NULL
		AND menu.parent_id IS NULL
		AND menu.caption = 'Материалы'
	ORDER BY menu.id
	LIMIT 1
) AS parent ON true
WHERE existing.role_id = roles.role_id
	AND existing.user_id IS NULL
	AND existing.route_id = route.id;

WITH roles(role_id) AS (
	VALUES
		('admin'::public.role_types),
		('construction_site_manager'::public.role_types)
)
INSERT INTO public.main_menus (
	role_id,
	parent_id,
	caption,
	route_id,
	icon,
	sort_order,
	is_active
)
SELECT
	roles.role_id,
	parent.id,
	'Остатки материалов',
	route.id,
	NULL,
	50,
	true
FROM roles
JOIN public.application_routes AS route
	ON route.name = 'materialBalance'
JOIN LATERAL (
	SELECT menu.id
	FROM public.main_menus AS menu
	WHERE menu.role_id = roles.role_id
		AND menu.user_id IS NULL
		AND menu.parent_id IS NULL
		AND menu.caption = 'Материалы'
	ORDER BY menu.id
	LIMIT 1
) AS parent ON true
WHERE NOT EXISTS (
	SELECT 1
	FROM public.main_menus AS existing
	WHERE existing.role_id = roles.role_id
		AND existing.user_id IS NULL
		AND existing.route_id = route.id
);

COMMIT;
