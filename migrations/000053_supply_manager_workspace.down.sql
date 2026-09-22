BEGIN;

DROP TRIGGER IF EXISTS audit_log_material_request_supplier_assignment_items
	ON public.material_request_supplier_assignment_items;

DROP TRIGGER IF EXISTS audit_log_material_request_supplier_assignments
	ON public.material_request_supplier_assignments;

DELETE FROM public.audit_column_aliases
WHERE (table_name, column_name) IN (
	('material_requests', 'status_id'),
	('material_request_supplier_assignments', 'id'),
	('material_request_supplier_assignments', 'date'),
	('material_request_supplier_assignments', 'supply_manager_id'),
	('material_request_supplier_assignments', 'comment'),
	('material_request_supplier_assignments', 'version'),
	('material_request_supplier_assignment_items', 'id'),
	('material_request_supplier_assignment_items', 'line_num'),
	(
		'material_request_supplier_assignment_items',
		'material_request_supplier_assignment_id'
	),
	('material_request_supplier_assignment_items', 'material_request_item_id'),
	('material_request_supplier_assignment_items', 'supplier_id')
);

DELETE FROM public.role_permissions
WHERE permission_code IN (
	'materialRequestSupplierAssignment.create',
	'materialRequestSupplierAssignment.list',
	'materialRequestSupplierAssignment.detail',
	'materialRequestSupplierAssignmentItem.list',
	'materialRequestSupplierAssignmentItem.detail'
);

DELETE FROM public.permissions
WHERE code IN (
	'materialRequestSupplierAssignment.create',
	'materialRequestSupplierAssignment.list',
	'materialRequestSupplierAssignment.detail',
	'materialRequestSupplierAssignmentItem.list',
	'materialRequestSupplierAssignmentItem.detail'
);

WITH required(permission_code) AS (
	VALUES
		('materialRequest.list'),
		('materialRequest.detail'),
		('materialRequest.update')
)
INSERT INTO public.role_permissions (role_id, permission_code)
SELECT 'supply_manager'::public.role_types, required.permission_code
FROM required
ON CONFLICT DO NOTHING;

DROP VIEW IF EXISTS public.material_request_supplier_assignment_items_list;
DROP VIEW IF EXISTS public.material_request_supplier_assignments_list;
DROP TABLE IF EXISTS public.material_request_supplier_assignment_items;
DROP TABLE IF EXISTS public.material_request_supplier_assignments;

DROP VIEW public.material_requests_list;

CREATE VIEW public.material_requests_list AS
SELECT
	request.id,
	request.date,
	request.construction_site_id,
	request.construction_manager_id,
	request.comment,
	request.version,
	public.construction_sites_ref(site) AS construction_site,
	public.users_ref(manager) AS construction_manager
FROM public.material_requests AS request
LEFT JOIN public.construction_sites AS site
	ON site.id = request.construction_site_id
LEFT JOIN public.users AS manager
	ON manager.id = request.construction_manager_id;

ALTER TABLE public.material_requests
	DROP COLUMN status_id;

ALTER TABLE public.material_request_statuses
	DROP CONSTRAINT IF EXISTS material_request_statuses_code_values_check;

UPDATE public.material_request_items AS item
SET status_id = submitted_status.id
FROM public.material_request_statuses AS assigned_status,
	public.material_request_statuses AS submitted_status
WHERE item.status_id = assigned_status.id
	AND assigned_status.code = 'supplier_assigned'
	AND submitted_status.code = 'new';

DELETE FROM public.material_request_statuses
WHERE code = 'supplier_assigned';

UPDATE public.material_request_statuses
SET
	code = CASE code
		WHEN 'new' THEN 'submitted'
		WHEN 'ordered' THEN 'in_progress'
		WHEN 'fulfilled' THEN 'completed'
		ELSE code
	END,
	name = CASE code
		WHEN 'new' THEN 'Отправлена'
		WHEN 'ordered' THEN 'В работе'
		WHEN 'fulfilled' THEN 'Выполнена'
		ELSE name
	END
WHERE code IN ('new', 'ordered', 'fulfilled');

ALTER TABLE public.material_request_statuses
	ADD CONSTRAINT material_request_statuses_code_values_check CHECK (
		code IN ('draft', 'submitted', 'in_progress', 'completed', 'cancelled')
	);

COMMIT;
