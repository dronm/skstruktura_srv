BEGIN;

INSERT INTO public.role_permissions (role_id, permission_code)
VALUES
	('supply_manager'::public.role_types, 'materialType.create'),
	('supply_manager'::public.role_types, 'measureUnit.create')
ON CONFLICT DO NOTHING;

COMMIT;
