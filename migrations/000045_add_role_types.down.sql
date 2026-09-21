-- The construction-site-manager rename is reversible. PostgreSQL cannot
-- safely remove the supplier enum value, so that unused label remains after a
-- rollback while its grants and translation are removed.

BEGIN;

DO $check$
BEGIN
	IF EXISTS (
		SELECT 1
		FROM public.users
		WHERE role_id = 'supplier'::public.role_types
	) THEN
		RAISE EXCEPTION
			'cannot roll back role types while users reference supplier';
	END IF;

	IF EXISTS (
		SELECT 1
		FROM public.main_menus
		WHERE role_id = 'supplier'::public.role_types
	) THEN
		RAISE EXCEPTION
			'cannot roll back role types while main menu items reference supplier';
	END IF;
END;
$check$;

DELETE FROM public.role_permissions
WHERE role_id = 'supplier'::public.role_types;

DO $rename$
DECLARE
	legacy_exists boolean;
	canonical_exists boolean;
BEGIN
	SELECT EXISTS (
		SELECT 1
		FROM pg_catalog.pg_enum AS enum_value
		WHERE enum_value.enumtypid = 'public.role_types'::regtype
			AND enum_value.enumlabel = 'constr_manager'
	)
	INTO legacy_exists;

	SELECT EXISTS (
		SELECT 1
		FROM pg_catalog.pg_enum AS enum_value
		WHERE enum_value.enumtypid = 'public.role_types'::regtype
			AND enum_value.enumlabel = 'construction_site_manager'
	)
	INTO canonical_exists;

	IF canonical_exists AND NOT legacy_exists THEN
		EXECUTE 'ALTER TYPE public.role_types RENAME VALUE '
			'''construction_site_manager'' TO ''constr_manager''';
	ELSIF legacy_exists AND canonical_exists THEN
		RAISE EXCEPTION
			'role_types contains both constr_manager and construction_site_manager';
	ELSIF NOT legacy_exists THEN
		RAISE EXCEPTION
			'role_types contains neither constr_manager nor construction_site_manager';
	END IF;
END;
$rename$;

CREATE OR REPLACE FUNCTION public.enum_role_types_val(
	public.role_types,
	public.locales
)
RETURNS text AS $function$
	SELECT CASE
		WHEN $1 = 'admin'::public.role_types
			AND $2 = 'ru'::public.locales
			THEN 'Администратор'
		WHEN $1 = 'constr_manager'::public.role_types
			AND $2 = 'ru'::public.locales
			THEN 'Прораб'
		WHEN $1 = 'accountant'::public.role_types
			AND $2 = 'ru'::public.locales
			THEN 'Бухгалтер'
		ELSE ''
	END;
$function$ LANGUAGE sql;

COMMIT;
