BEGIN;

WITH required(code, description) AS (
	VALUES
		(
			'constructionManager.materialConsumption.create',
			'Создание списания материалов в рабочем месте прораба'
		),
		(
			'constructionManager.materialConsumption.list',
			'Просмотр списаний материалов в рабочем месте прораба'
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
		('constructionManager.materialConsumption.create'),
		('constructionManager.materialConsumption.list')
)
INSERT INTO public.role_permissions (role_id, permission_code)
SELECT roles.role_id, required.permission_code
FROM roles
CROSS JOIN required
ON CONFLICT DO NOTHING;

COMMIT;
