-- Function: public.contacts_ref(public.contacts)

CREATE OR REPLACE FUNCTION public.contacts_ref(public.contacts)
RETURNS json AS
$BODY$
	SELECT
		CASE
			WHEN $1.id IS NULL THEN NULL
			ELSE json_build_object(
				'keys', json_build_object('id', $1.id),
				'descr', concat_ws(
					', ',
					NULLIF(btrim($1.name), ''),
					public.format_phone($1.phone),
					(
						SELECT NULLIF(btrim(employee_post.name), '')
						FROM public.employee_posts AS employee_post
						WHERE employee_post.id = $1.employee_post_id
					),
					NULLIF(btrim($1.email), '')
				),
				'dataType', 'contacts'
			)
		END;
$BODY$
LANGUAGE sql STABLE;
