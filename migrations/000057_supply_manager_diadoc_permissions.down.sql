BEGIN;

DELETE FROM public.role_permissions
WHERE role_id = 'supply_manager'::public.role_types
	AND permission_code IN (
		'diadoc.manage',
		'diadoc.sync',
		'diadocDocument.list',
		'diadocDocument.detail',
		'diadocDocument.resolve',
		'diadocDocument.ignore',
		'diadocDocument.import',
		'diadocState.view',
		'diadocState.update'
	);

COMMIT;
