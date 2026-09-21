BEGIN;

DROP FUNCTION public.material_actions_report_documents(date, date, integer[], integer[]);
DROP VIEW public.material_register_recorders;

ALTER TABLE public.material_receipts
	ALTER COLUMN date TYPE date
	USING (date AT TIME ZONE public.register_business_timezone())::date;

ALTER TABLE public.material_consumptions
	ALTER COLUMN date TYPE date
	USING (date AT TIME ZONE public.register_business_timezone())::date;

ALTER TABLE public.material_transfers
	ALTER COLUMN date TYPE date
	USING (date AT TIME ZONE public.register_business_timezone())::date;

COMMENT ON COLUMN public.material_receipts.date IS 'Document date.';
COMMENT ON COLUMN public.material_consumptions.date IS 'Document date.';
COMMENT ON COLUMN public.material_transfers.date IS 'Document date.';

CREATE VIEW public.material_register_recorders AS
SELECT
	'MaterialReceipt'::text AS recorder_type,
	receipt.id::bigint AS recorder_id,
	receipt.date AS document_date,
	receipt.number AS document_number
FROM public.material_receipts AS receipt

UNION ALL

SELECT
	'MaterialConsumption'::text AS recorder_type,
	consumption.id::bigint AS recorder_id,
	consumption.date AS document_date,
	NULL::text AS document_number
FROM public.material_consumptions AS consumption

UNION ALL

SELECT
	'MaterialTransfer'::text AS recorder_type,
	transfer.id::bigint AS recorder_id,
	transfer.date AS document_date,
	NULL::text AS document_number
FROM public.material_transfers AS transfer;

COMMENT ON VIEW public.material_register_recorders IS 'Normalizes material document headers used as ra_materials recorders.';

CREATE FUNCTION public.material_actions_report_documents(
	in_date_from date,
	in_date_to date,
	in_construction_site_ids integer[] DEFAULT NULL,
	in_material_ids integer[] DEFAULT NULL
)
RETURNS TABLE (
	construction_site_id integer,
	material_id integer,
	recorder_type text,
	recorder_id bigint,
	document_date date,
	document_number text,
	income numeric(19, 4),
	outcome numeric(19, 4)
)
LANGUAGE sql
STABLE
AS $function$
	WITH boundary AS (
		SELECT
			in_date_from::timestamp AT TIME ZONE public.register_business_timezone() AS from_at,
			(in_date_to + 1)::timestamp AT TIME ZONE public.register_business_timezone() AS to_at
	)
	SELECT
		action.construction_site_id,
		action.material_id,
		action.recorder_type,
		action.recorder_id,
		COALESCE(
			recorder.document_date,
			(MIN(action.effective_at) AT TIME ZONE public.register_business_timezone())::date
		) AS document_date,
		recorder.document_number,
		COALESCE(SUM(action.quant) FILTER (WHERE action.quant > 0), 0::numeric)::numeric(19, 4) AS income,
		COALESCE(SUM(-action.quant) FILTER (WHERE action.quant < 0), 0::numeric)::numeric(19, 4) AS outcome
	FROM public.ra_materials AS action
	CROSS JOIN boundary
	LEFT JOIN public.material_register_recorders AS recorder
		ON recorder.recorder_type = action.recorder_type
		AND recorder.recorder_id = action.recorder_id
	WHERE action.effective_at >= boundary.from_at
		AND action.effective_at < boundary.to_at
		AND (
			in_construction_site_ids IS NULL
			OR cardinality(in_construction_site_ids) = 0
			OR action.construction_site_id = ANY(in_construction_site_ids)
		)
		AND (
			in_material_ids IS NULL
			OR cardinality(in_material_ids) = 0
			OR action.material_id = ANY(in_material_ids)
		)
	GROUP BY
		action.construction_site_id,
		action.material_id,
		action.recorder_type,
		action.recorder_id,
		recorder.document_date,
		recorder.document_number;
$function$;

COMMENT ON FUNCTION public.material_actions_report_documents(date, date, integer[], integer[]) IS 'Returns period material register movements grouped by construction site, material, and source document.';

COMMIT;
