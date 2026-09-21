BEGIN;

DELETE FROM public.role_permissions
WHERE permission_code = 'diadoc.manage';

DELETE FROM public.permissions
WHERE code = 'diadoc.manage';

DROP SCHEMA integration_diadoc CASCADE;

COMMIT;
