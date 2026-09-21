CREATE OR REPLACE FUNCTION public.material_types_ref(public.material_types)
RETURNS json AS
$BODY$
	SELECT CASE WHEN $1.id IS NULL THEN NULL ELSE json_build_object(
		'keys', json_build_object('id', $1.id),
		'descr', $1.name,
		'dataType', 'materialTypes'
	) END;
$BODY$
LANGUAGE sql STABLE;
