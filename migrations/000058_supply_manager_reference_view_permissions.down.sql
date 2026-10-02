BEGIN;

-- Keep measure unit view permissions granted before this migration by 000047.
DELETE FROM public.role_permissions
WHERE role_id = 'supply_manager'::public.role_types
	AND permission_code IN (
		'constructionSite.list',
		'constructionSite.detail',
		'materialType.list',
		'materialType.detail'
	);

COMMIT;
