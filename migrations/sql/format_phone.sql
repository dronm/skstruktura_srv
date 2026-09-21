-- Function: public.format_phone(text)

CREATE OR REPLACE FUNCTION public.format_phone(text)
RETURNS text AS
$BODY$
	SELECT
		CASE
			WHEN NULLIF(btrim($1), '') IS NULL THEN NULL
			WHEN char_length($1) < 10 THEN $1
			ELSE
				'+' || substr($1, 1, 1) ||
				'(' || substr($1, 2, 3) || ')-' ||
				substr($1, 5, 3) || '-' ||
				substr($1, 8, 2) || '-' ||
				substr($1, 10, 2)
		END;
$BODY$
LANGUAGE sql STABLE;
