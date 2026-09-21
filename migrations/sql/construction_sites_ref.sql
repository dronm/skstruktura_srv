CREATE OR REPLACE FUNCTION public.construction_sites_ref(public.construction_sites)
RETURNS json AS
$BODY$
	SELECT CASE WHEN $1.id IS NULL THEN NULL ELSE json_build_object(
		'keys', json_build_object('id', $1.id),
		'descr', $1.name,
		'dataType', 'constructionSites'
	) END;
$BODY$
LANGUAGE sql STABLE;
