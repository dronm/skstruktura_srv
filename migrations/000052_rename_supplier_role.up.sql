BEGIN;

DO $rename$
DECLARE
	supplier_exists boolean;
	supply_manager_exists boolean;
BEGIN
	SELECT EXISTS (
		SELECT 1
		FROM pg_catalog.pg_enum AS enum_value
		WHERE enum_value.enumtypid = 'public.role_types'::regtype
			AND enum_value.enumlabel = 'supplier'
	)
	INTO supplier_exists;

	SELECT EXISTS (
		SELECT 1
		FROM pg_catalog.pg_enum AS enum_value
		WHERE enum_value.enumtypid = 'public.role_types'::regtype
			AND enum_value.enumlabel = 'supply_manager'
	)
	INTO supply_manager_exists;

	IF supplier_exists AND NOT supply_manager_exists THEN
		EXECUTE 'ALTER TYPE public.role_types RENAME VALUE '
			'''supplier'' TO ''supply_manager''';
	ELSIF supplier_exists AND supply_manager_exists THEN
		RAISE EXCEPTION
			'role_types contains both supplier and supply_manager';
	ELSIF NOT supply_manager_exists THEN
		RAISE EXCEPTION
			'role_types contains neither supplier nor supply_manager';
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
		WHEN $1 = 'construction_site_manager'::public.role_types
			AND $2 = 'ru'::public.locales
			THEN 'Прораб'
		WHEN $1 = 'accountant'::public.role_types
			AND $2 = 'ru'::public.locales
			THEN 'Бухгалтер'
		WHEN $1 = 'supply_manager'::public.role_types
			AND $2 = 'ru'::public.locales
			THEN 'Снабженец'
		ELSE ''
	END;
$function$ LANGUAGE sql;

COMMENT ON COLUMN public.material_request_items.supplier_id IS
	'Optional supplier assigned by a supply manager.';

COMMIT;
