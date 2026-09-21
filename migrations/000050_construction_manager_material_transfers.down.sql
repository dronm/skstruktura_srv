BEGIN;

DELETE FROM public.role_permissions
WHERE permission_code IN (
	'constructionManager.materialTransfer.create',
	'constructionManager.materialTransfer.list'
);

DELETE FROM public.permissions
WHERE code IN (
	'constructionManager.materialTransfer.create',
	'constructionManager.materialTransfer.list'
);

COMMIT;
