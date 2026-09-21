CREATE OR REPLACE FUNCTION public.measure_units_ref(public.measure_units)
RETURNS json AS
$BODY$
	SELECT CASE WHEN $1.id IS NULL THEN NULL ELSE json_build_object(
		'keys', json_build_object('id', $1.id),
		'descr', $1.name,
		'dataType', 'measureUnits'
	) END;
$BODY$
LANGUAGE sql STABLE;
