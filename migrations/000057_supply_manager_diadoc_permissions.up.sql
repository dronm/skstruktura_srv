BEGIN;

WITH required(permission_code) AS (
	VALUES
		('diadoc.manage'),
		('diadoc.sync'),
		('diadocDocument.list'),
		('diadocDocument.detail'),
		('diadocDocument.resolve'),
		('diadocDocument.ignore'),
		('diadocDocument.import'),
		('diadocState.view'),
		('diadocState.update')
)
INSERT INTO public.role_permissions (role_id, permission_code)
SELECT 'supply_manager'::public.role_types, required.permission_code
FROM required
ON CONFLICT DO NOTHING;

COMMIT;
