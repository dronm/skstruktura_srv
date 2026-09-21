-- View: public.max_users_list

CREATE OR REPLACE VIEW public.max_users_list AS
SELECT
	u.id,
	CASE WHEN u.contact_id IS NULL THEN NULL ELSE contacts_ref(contacts) END AS contact,
	u.max_user_id,
	u.username,
	u.avatar_url,
	u.raw_user,
	u.is_active
FROM public.max_users u
LEFT JOIN public.contacts ON contacts.id = u.contact_id;
