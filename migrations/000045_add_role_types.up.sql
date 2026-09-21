-- Replace the abbreviated legacy role name and add the supplier role.

BEGIN;

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

	IF legacy_exists AND NOT canonical_exists THEN
		EXECUTE 'ALTER TYPE public.role_types RENAME VALUE '
			'''constr_manager'' TO ''construction_site_manager''';
	ELSIF legacy_exists AND canonical_exists THEN
		RAISE EXCEPTION
			'role_types contains both constr_manager and construction_site_manager';
	ELSIF NOT canonical_exists THEN
		RAISE EXCEPTION
			'role_types contains neither constr_manager nor construction_site_manager';
	END IF;
END;
$rename$;

-- Keep the translation function valid after the rename is committed. The
-- supplier branch is added in the next transaction, after its enum value is
-- safe to use.
CREATE OR REPLACE FUNCTION public.enum_role_types_val(
	public.role_types,
	public.locales
)
RETURNS text AS $function$
	SELECT CASE
		WHEN $1 = 'admin'::public.role_types
			AND $2 = 'ru'::public.locales
			THEN 'Администратор'
		WHEN $1 = 'construction_site_manager'::public.role_types
			AND $2 = 'ru'::public.locales
			THEN 'Прораб'
		WHEN $1 = 'accountant'::public.role_types
			AND $2 = 'ru'::public.locales
			THEN 'Бухгалтер'
		ELSE ''
	END;
$function$ LANGUAGE sql;

ALTER TYPE public.role_types
	ADD VALUE IF NOT EXISTS 'supplier';

COMMIT;

-- PostgreSQL does not allow newly-added enum values to be used until the
-- transaction that added them has committed.
BEGIN;

CREATE OR REPLACE FUNCTION public.enum_role_types_val(
	public.role_types,
	public.locales
)
RETURNS text AS $function$
	SELECT CASE
		WHEN $1 = 'admin'::public.role_types
			AND $2 = 'ru'::public.locales
			THEN 'Администратор'
		WHEN $1 = 'construction_site_manager'::public.role_types
			AND $2 = 'ru'::public.locales
			THEN 'Прораб'
		WHEN $1 = 'accountant'::public.role_types
			AND $2 = 'ru'::public.locales
			THEN 'Бухгалтер'
		WHEN $1 = 'supplier'::public.role_types
			AND $2 = 'ru'::public.locales
			THEN 'Снабженец'
		ELSE ''
	END;
$function$ LANGUAGE sql;

-- The renamed construction-site-manager role retains its existing grants.
-- Give the new supplier role only the baseline authenticated-user permissions;
-- business permissions must be granted separately.
WITH required(permission_code) AS (
	VALUES
		('user.profile.detail'),
		('user.profile.update'),
		('user.profile.password'),
		('user.logout'),
		('mainMenu.for_user'),
		('progAbout.info')
)
INSERT INTO public.role_permissions (role_id, permission_code)
SELECT 'supplier'::public.role_types, required.permission_code
FROM required
ON CONFLICT DO NOTHING;

COMMIT;
