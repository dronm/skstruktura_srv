-- View: public.audit_log_list

-- DROP VIEW public.audit_log_dialog;
-- DROP VIEW public.audit_log_list

CREATE OR REPLACE VIEW public.audit_log_list
 AS
SELECT 
	t.id,
	t.table_name,
	t.record_id,
	t.operation,
	t.changed_at,
	t.changed_by,
	CASE
		WHEN t.table_name = 'users' THEN users_ref(u)
		WHEN t.table_name = 'construction_sites' THEN construction_sites_ref(cs)
	ELSE NULL
	END AS object_ref,

	CASE
		WHEN t.table_name = 'users' THEN 'Пользователи'
		WHEN t.table_name = 'construction_sites' THEN 'Объекты строительства'
	ELSE NULL
	END AS type_descr,

	CASE
		WHEN t.operation = 'I' THEN 'Добавление'
		WHEN t.operation = 'U' THEN 'Изменение'
		WHEN t.operation = 'D' THEN 'Удаление'
	ELSE NULL
	END AS operation_descr

FROM public.audit_log AS t
LEFT JOIN users AS u ON t.table_name = 'users' AND t.record_id = u.id::text
LEFT JOIN construction_sites AS cs ON t.table_name = 'construction_sites' AND t.record_id = cs.id::text
ORDER BY t.changed_at DESC;

