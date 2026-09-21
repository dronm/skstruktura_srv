-- Function: public.contacts_process()

CREATE OR REPLACE FUNCTION public.contacts_process()
RETURNS trigger AS
$BODY$
BEGIN
	NEW.search := concat_ws(
		' ',
		NULLIF(btrim(NEW.name), ''),
		(
			SELECT NULLIF(btrim(employee_post.name), '')
			FROM public.employee_posts AS employee_post
			WHERE employee_post.id = NEW.employee_post_id
		),
		NULLIF(btrim(NEW.email), ''),
		NULLIF(btrim(NEW.phone), ''),
		public.format_phone(NEW.phone)
	);

	RETURN NEW;
END;
$BODY$
LANGUAGE plpgsql VOLATILE;
