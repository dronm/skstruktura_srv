-- Function: public.users_ref(users)

-- DROP FUNCTION public.users_ref(users);

CREATE OR REPLACE FUNCTION public.users_ref(users)
  RETURNS json AS
$BODY$
	SELECT json_build_object(
		'keys',json_build_object(
			'id',$1.id    
			),	
		'descr',$1.name,
		'dataType','users'
	);
$BODY$
  LANGUAGE sql VOLATILE
  COST 100;



