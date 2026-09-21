BEGIN;

DELETE FROM public.role_permissions
WHERE permission_code = 'materialActionReport.list';

DELETE FROM public.permissions
WHERE code = 'materialActionReport.list';

DROP FUNCTION IF EXISTS public.material_actions_report_documents(date, date, integer[], integer[]);
DROP FUNCTION IF EXISTS public.material_actions_report_totals(date, date, integer[], integer[]);
DROP VIEW IF EXISTS public.material_register_recorders;
DROP VIEW IF EXISTS public.materials_list;

CREATE VIEW public.materials_list AS
SELECT
	m.id,
	m.name,
	m.name_full,
	m.measure_unit_id,
	CASE
		WHEN mu.id IS NULL THEN NULL
		ELSE json_build_object(
			'keys', json_build_object('id', mu.id),
			'descr', mu.name,
			'dataType', 'measureUnits'
		)
	END AS measure_unit,
	m.is_active
FROM public.materials AS m
LEFT JOIN public.measure_units AS mu ON mu.id = m.measure_unit_id;

COMMIT;
