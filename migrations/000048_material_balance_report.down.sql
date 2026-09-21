BEGIN;

DELETE FROM public.main_menus AS menu
USING public.application_routes AS route
WHERE menu.route_id = route.id
	AND route.name = 'materialBalance';

DELETE FROM public.application_routes
WHERE name = 'materialBalance';

DELETE FROM public.role_permissions
WHERE permission_code = 'materialBalance.list';

DELETE FROM public.permissions
WHERE code = 'materialBalance.list';

DROP VIEW IF EXISTS public.material_balances_list;

COMMIT;
