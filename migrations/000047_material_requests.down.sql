BEGIN;

DROP TRIGGER IF EXISTS audit_log_material_request_items ON public.material_request_items;
DROP TRIGGER IF EXISTS audit_log_material_requests ON public.material_requests;
DROP TRIGGER IF EXISTS audit_log_material_request_statuses ON public.material_request_statuses;
DROP TRIGGER IF EXISTS audit_log_order_importances ON public.order_importances;

WITH aliases(table_name, column_name, column_alias) AS (
	VALUES
		('order_importances', 'id', 'Идентификатор'),
		('order_importances', 'name', 'Наименование'),
		('order_importances', 'sort_order', 'Порядок'),
		('order_importances', 'is_active', 'Активна'),
		('material_request_statuses', 'id', 'Идентификатор'),
		('material_request_statuses', 'code', 'Код'),
		('material_request_statuses', 'name', 'Наименование'),
		('material_requests', 'id', 'Идентификатор'),
		('material_requests', 'date', 'Дата и время'),
		('material_requests', 'construction_site_id', 'Объект строительства'),
		('material_requests', 'construction_manager_id', 'Ответственный прораб'),
		('material_requests', 'comment', 'Комментарий'),
		('material_requests', 'version', 'Версия'),
		('material_request_items', 'id', 'Идентификатор'),
		('material_request_items', 'line_num', 'Номер строки'),
		('material_request_items', 'material_request_id', 'Заявка на материалы'),
		('material_request_items', 'material_id', 'Материал'),
		('material_request_items', 'measure_unit_id', 'Единица измерения'),
		('material_request_items', 'quant', 'Количество'),
		('material_request_items', 'supplier_id', 'Поставщик'),
		('material_request_items', 'required_date', 'Требуемая дата'),
		('material_request_items', 'order_importance_id', 'Важность'),
		('material_request_items', 'status_id', 'Статус')
)
DELETE FROM public.audit_column_aliases AS target
USING aliases
WHERE target.table_name = aliases.table_name
	AND target.column_name = aliases.column_name
	AND target.column_alias = aliases.column_alias;

DELETE FROM public.main_menus AS menu
USING public.application_routes AS route
WHERE menu.route_id = route.id
	AND menu.user_id IS NULL
	AND menu.role_id IN (
		'admin'::public.role_types,
		'construction_site_manager'::public.role_types,
		'supplier'::public.role_types
	)
	AND route.name IN (
		'orderImportances',
		'materialRequestStatuses',
		'materialRequests'
	);

DELETE FROM public.main_menus AS parent
WHERE parent.user_id IS NULL
	AND parent.parent_id IS NULL
	AND (
		(parent.role_id = 'admin'::public.role_types AND parent.caption = 'Заявки на материалы')
		OR (
			parent.role_id IN (
				'construction_site_manager'::public.role_types,
				'supplier'::public.role_types
			)
			AND parent.caption = 'Материалы'
		)
	)
	AND NOT EXISTS (
		SELECT 1
		FROM public.main_menus AS child
		WHERE child.parent_id = parent.id
	);

DELETE FROM public.application_routes AS route
WHERE route.name IN (
	'orderImportances',
	'materialRequestStatuses',
	'materialRequests'
)
	AND NOT EXISTS (
		SELECT 1
		FROM public.main_menus AS menu
		WHERE menu.route_id = route.id
	);

DELETE FROM public.role_permissions
WHERE permission_code IN (
	'orderImportance.create',
	'orderImportance.list',
	'orderImportance.detail',
	'orderImportance.update',
	'orderImportance.delete',
	'materialRequestStatus.list',
	'materialRequestStatus.detail',
	'materialRequestStatus.update',
	'materialRequest.create',
	'materialRequest.list',
	'materialRequest.detail',
	'materialRequest.update',
	'materialRequest.delete',
	'materialRequest.submit',
	'materialRequestItem.create',
	'materialRequestItem.list',
	'materialRequestItem.detail',
	'materialRequestItem.update',
	'materialRequestItem.delete'
);

DELETE FROM public.role_permissions
WHERE role_id = 'construction_site_manager'::public.role_types
	AND permission_code IN (
		'constructionSite.list',
		'constructionSite.detail',
		'material.list',
		'material.detail',
		'measureUnit.list',
		'measureUnit.detail'
	);

DELETE FROM public.role_permissions
WHERE role_id = 'supplier'::public.role_types
	AND permission_code IN (
		'supplier.list',
		'supplier.detail',
		'material.list',
		'material.detail',
		'measureUnit.list',
		'measureUnit.detail'
	);

DELETE FROM public.permissions
WHERE code IN (
	'orderImportance.create',
	'orderImportance.list',
	'orderImportance.detail',
	'orderImportance.update',
	'orderImportance.delete',
	'materialRequestStatus.list',
	'materialRequestStatus.detail',
	'materialRequestStatus.update',
	'materialRequest.create',
	'materialRequest.list',
	'materialRequest.detail',
	'materialRequest.update',
	'materialRequest.delete',
	'materialRequest.submit',
	'materialRequestItem.create',
	'materialRequestItem.list',
	'materialRequestItem.detail',
	'materialRequestItem.update',
	'materialRequestItem.delete'
);

DROP VIEW IF EXISTS public.material_request_items_list;
DROP VIEW IF EXISTS public.material_requests_list;

DROP FUNCTION IF EXISTS public.material_request_statuses_ref(public.material_request_statuses);
DROP FUNCTION IF EXISTS public.order_importances_ref(public.order_importances);

DROP TABLE IF EXISTS public.material_request_items;
DROP TABLE IF EXISTS public.material_requests;
DROP TABLE IF EXISTS public.material_request_statuses;
DROP TABLE IF EXISTS public.order_importances;

COMMIT;
