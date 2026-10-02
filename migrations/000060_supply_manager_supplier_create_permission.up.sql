BEGIN;

INSERT INTO public.role_permissions (role_id, permission_code)
VALUES ('supply_manager'::public.role_types, 'supplier.create')
ON CONFLICT DO NOTHING;

COMMIT;
