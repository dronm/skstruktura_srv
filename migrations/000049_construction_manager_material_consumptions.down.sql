BEGIN;

DELETE FROM public.role_permissions
WHERE permission_code IN (
	'constructionManager.materialConsumption.create',
	'constructionManager.materialConsumption.list'
);

DELETE FROM public.permissions
WHERE code IN (
	'constructionManager.materialConsumption.create',
	'constructionManager.materialConsumption.list'
);

COMMIT;
