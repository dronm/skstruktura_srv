BEGIN;

DELETE FROM public.role_permissions
WHERE role_id = 'supply_manager'::public.role_types
	AND permission_code IN (
		'materialType.create',
		'measureUnit.create'
	);

COMMIT;
