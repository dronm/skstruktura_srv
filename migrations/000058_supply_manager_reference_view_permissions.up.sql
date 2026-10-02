BEGIN;

-- Reference autocomplete uses the list endpoint; detail resolves a selected value.
-- Measure unit view permissions were already granted by migration 000047.
WITH required(permission_code) AS (
	VALUES
		('constructionSite.list'),
		('constructionSite.detail'),
		('materialType.list'),
		('materialType.detail'),
		('measureUnit.list'),
		('measureUnit.detail')
)
INSERT INTO public.role_permissions (role_id, permission_code)
SELECT 'supply_manager'::public.role_types, required.permission_code
FROM required
ON CONFLICT DO NOTHING;

COMMIT;
