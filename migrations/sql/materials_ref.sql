CREATE OR REPLACE FUNCTION public.materials_ref(public.materials)
RETURNS json AS
$BODY$
	SELECT CASE WHEN $1.id IS NULL THEN NULL ELSE json_build_object(
		'keys', json_build_object('id', $1.id),
		'descr', $1.name,
		'dataType', 'materials'
	) END;
$BODY$
LANGUAGE sql STABLE;
