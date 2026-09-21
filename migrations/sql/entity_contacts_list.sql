-- View: public.entity_contacts_list

CREATE OR REPLACE VIEW public.entity_contacts_list AS
	SELECT
		e_ct.id,
		e_ct.entity_type,
		e_ct.entity_id,
		CASE
			WHEN e_ct.entity_type = 'users' THEN users_ref(u)
			WHEN e_ct.entity_type = 'suppliers' THEN suppliers_ref(spl)
			ELSE NULL
		END AS entity,
		e_ct.contact_id,
		contacts_ref(ct) AS contact,
		json_build_object(
			'name', ct.name,
			'phone', ct.phone,
			'email', ct.email,
			'employee_post', p.name
		) AS contact_attrs,
		e_ct.is_active
	FROM public.entity_contacts AS e_ct
	LEFT JOIN public.users AS u
		ON e_ct.entity_type = 'users' AND u.id = e_ct.entity_id
	LEFT JOIN public.suppliers AS spl
		ON e_ct.entity_type = 'suppliers' AND spl.id = e_ct.entity_id
	LEFT JOIN public.contacts AS ct
		ON ct.id = e_ct.contact_id
	LEFT JOIN public.employee_posts AS p
		ON p.id = ct.employee_post_id
	ORDER BY
		e_ct.entity_type,
		CASE
			WHEN e_ct.entity_type = 'users' THEN users_ref(u)->>'descr'
			WHEN e_ct.entity_type = 'suppliers' THEN suppliers_ref(spl)->>'descr'
			ELSE NULL
		END,
		ct.name,
		e_ct.id;
