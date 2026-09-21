BEGIN;

CREATE TABLE public.register_settings (
	id smallint NOT NULL PRIMARY KEY DEFAULT 1,
	business_timezone text NOT NULL DEFAULT 'UTC',
	CONSTRAINT register_settings_singleton_chk CHECK (id = 1),
	CONSTRAINT register_settings_business_timezone_chk CHECK (btrim(business_timezone) <> '')
);

COMMENT ON TABLE public.register_settings IS 'Application-wide settings shared by accumulation registers.';
COMMENT ON COLUMN public.register_settings.business_timezone IS 'IANA/PostgreSQL time zone used to map timestamptz register actions to local aggregation periods.';

CREATE OR REPLACE FUNCTION public.register_settings_validate()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
BEGIN
	BEGIN
		PERFORM timezone(NEW.business_timezone, CURRENT_TIMESTAMP);
	EXCEPTION
		WHEN invalid_parameter_value THEN
			RAISE EXCEPTION 'Invalid PostgreSQL time zone: %', NEW.business_timezone
				USING ERRCODE = '22023';
	END;

	RETURN NEW;
END;
$function$;

CREATE TRIGGER register_settings_validate_trigger
	BEFORE INSERT OR UPDATE OF business_timezone
	ON public.register_settings
	FOR EACH ROW
	EXECUTE FUNCTION public.register_settings_validate();

INSERT INTO public.register_settings (id, business_timezone)
VALUES (1, 'Asia/Yekaterinburg');

CREATE OR REPLACE FUNCTION public.register_business_timezone()
RETURNS text
LANGUAGE sql
STABLE
AS $function$
	SELECT settings.business_timezone
	FROM public.register_settings AS settings
	WHERE settings.id = 1;
$function$;

COMMENT ON FUNCTION public.register_business_timezone() IS 'Returns the application business time zone used by register period calculations.';

CREATE OR REPLACE FUNCTION public.register_month_start(
	in_effective_at timestamptz
)
RETURNS date
LANGUAGE sql
STABLE
STRICT
AS $function$
	SELECT date_trunc(
		'month',
		in_effective_at AT TIME ZONE public.register_business_timezone()
	)::date;
$function$;

COMMENT ON FUNCTION public.register_month_start(timestamptz) IS 'Returns the first local calendar date of the month containing an action instant in the configured business time zone.';

CREATE OR REPLACE FUNCTION public.register_month_start_at(
	in_effective_at timestamptz
)
RETURNS timestamptz
LANGUAGE sql
STABLE
STRICT
AS $function$
	SELECT date_trunc(
		'month',
		in_effective_at AT TIME ZONE public.register_business_timezone()
	) AT TIME ZONE public.register_business_timezone();
$function$;

COMMENT ON FUNCTION public.register_month_start_at(timestamptz) IS 'Returns the exact timestamptz instant corresponding to local midnight at the start of the action month in the configured business time zone.';

COMMIT;
