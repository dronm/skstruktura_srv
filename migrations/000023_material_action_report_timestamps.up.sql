BEGIN;

CREATE FUNCTION public.material_actions_report_totals(
	in_date_from timestamptz,
	in_date_to timestamptz,
	in_construction_site_ids integer[] DEFAULT NULL,
	in_material_ids integer[] DEFAULT NULL
)
RETURNS TABLE (
	construction_site_id integer,
	material_id integer,
	balance_start numeric(19, 4),
	income numeric(19, 4),
	outcome numeric(19, 4),
	balance_end numeric(19, 4),
	document_count bigint
)
LANGUAGE sql
STABLE
AS $function$
	WITH boundary AS (
		SELECT
			in_date_from AS from_at,
			in_date_to AS to_at
	), opening_movements AS (
		SELECT
			period.construction_site_id,
			period.material_id,
			period.quant
		FROM public.rg_materials_period AS period
		CROSS JOIN boundary
		WHERE period.period_start < public.register_month_start(boundary.from_at)
			AND (
				in_construction_site_ids IS NULL
				OR cardinality(in_construction_site_ids) = 0
				OR period.construction_site_id = ANY(in_construction_site_ids)
			)
			AND (
				in_material_ids IS NULL
				OR cardinality(in_material_ids) = 0
				OR period.material_id = ANY(in_material_ids)
			)

		UNION ALL

		SELECT
			action.construction_site_id,
			action.material_id,
			action.quant
		FROM public.ra_materials AS action
		CROSS JOIN boundary
		WHERE action.effective_at >= public.register_month_start_at(boundary.from_at)
			AND action.effective_at < boundary.from_at
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
	), opening AS (
		SELECT
			movement.construction_site_id,
			movement.material_id,
			SUM(movement.quant) AS balance_start
		FROM opening_movements AS movement
		GROUP BY movement.construction_site_id, movement.material_id
	), turnover AS (
		SELECT
			action.construction_site_id,
			action.material_id,
			COALESCE(SUM(action.quant) FILTER (WHERE action.quant > 0), 0::numeric) AS income,
			COALESCE(SUM(-action.quant) FILTER (WHERE action.quant < 0), 0::numeric) AS outcome,
			COUNT(DISTINCT (action.recorder_type, action.recorder_id)) AS document_count
		FROM public.ra_materials AS action
		CROSS JOIN boundary
		WHERE action.effective_at >= boundary.from_at
			AND action.effective_at <= boundary.to_at
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
		GROUP BY action.construction_site_id, action.material_id
	), totals AS (
		SELECT
			COALESCE(opening.construction_site_id, turnover.construction_site_id) AS construction_site_id,
			COALESCE(opening.material_id, turnover.material_id) AS material_id,
			COALESCE(opening.balance_start, 0::numeric) AS balance_start,
			COALESCE(turnover.income, 0::numeric) AS income,
			COALESCE(turnover.outcome, 0::numeric) AS outcome,
			COALESCE(turnover.document_count, 0::bigint) AS document_count
		FROM opening
		FULL JOIN turnover
			ON turnover.construction_site_id = opening.construction_site_id
			AND turnover.material_id = opening.material_id
	)
	SELECT
		totals.construction_site_id,
		totals.material_id,
		totals.balance_start::numeric(19, 4),
		totals.income::numeric(19, 4),
		totals.outcome::numeric(19, 4),
		(totals.balance_start + totals.income - totals.outcome)::numeric(19, 4) AS balance_end,
		totals.document_count
	FROM totals
	WHERE totals.balance_start <> 0
		OR totals.income <> 0
		OR totals.outcome <> 0;
$function$;

COMMENT ON FUNCTION public.material_actions_report_totals(timestamptz, timestamptz, integer[], integer[]) IS 'Returns opening balance, positive income/outcome turnovers, closing balance, and document count by construction site and material for an inclusive timestamp period.';

CREATE FUNCTION public.material_actions_report_documents(
	in_date_from timestamptz,
	in_date_to timestamptz,
	in_construction_site_ids integer[] DEFAULT NULL,
	in_material_ids integer[] DEFAULT NULL
)
RETURNS TABLE (
	construction_site_id integer,
	material_id integer,
	recorder_type text,
	recorder_id bigint,
	document_date timestamptz,
	document_number text,
	income numeric(19, 4),
	outcome numeric(19, 4)
)
LANGUAGE sql
STABLE
AS $function$
	SELECT
		action.construction_site_id,
		action.material_id,
		action.recorder_type,
		action.recorder_id,
		COALESCE(recorder.document_date, MIN(action.effective_at)) AS document_date,
		recorder.document_number,
		COALESCE(SUM(action.quant) FILTER (WHERE action.quant > 0), 0::numeric)::numeric(19, 4) AS income,
		COALESCE(SUM(-action.quant) FILTER (WHERE action.quant < 0), 0::numeric)::numeric(19, 4) AS outcome
	FROM public.ra_materials AS action
	LEFT JOIN public.material_register_recorders AS recorder
		ON recorder.recorder_type = action.recorder_type
		AND recorder.recorder_id = action.recorder_id
	WHERE action.effective_at >= in_date_from
		AND action.effective_at <= in_date_to
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

COMMENT ON FUNCTION public.material_actions_report_documents(timestamptz, timestamptz, integer[], integer[]) IS 'Returns material register movements grouped by construction site, material, and source document for an inclusive timestamp period.';

COMMIT;
