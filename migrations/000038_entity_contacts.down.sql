BEGIN;

DELETE FROM public.role_permissions
WHERE permission_code IN (
	'entityContact.create',
	'entityContact.list',
	'entityContact.detail',
	'entityContact.update',
	'entityContact.delete'
);

DELETE FROM public.permissions
WHERE code IN (
	'entityContact.create',
	'entityContact.list',
	'entityContact.detail',
	'entityContact.update',
	'entityContact.delete'
);

DROP VIEW IF EXISTS public.entity_contacts_list;
DROP TABLE IF EXISTS public.entity_contacts;

COMMIT;
