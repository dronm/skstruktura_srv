BEGIN;

DELETE FROM public.main_menus AS menu
USING public.application_routes AS route
WHERE menu.route_id = route.id
	AND route.name = 'inventoryValuation';
DELETE FROM public.application_routes WHERE name = 'inventoryValuation';
DELETE FROM public.role_permissions WHERE permission_code IN ('inventoryValuation.view', 'inventoryValuation.manage');
DELETE FROM public.role_permissions WHERE role_id = 'accountant'::public.role_types AND permission_code = 'materialBalance.list';
DELETE FROM public.permissions WHERE code IN ('inventoryValuation.view', 'inventoryValuation.manage');

DROP VIEW IF EXISTS public.material_balances_list;
CREATE VIEW public.material_balances_list AS
SELECT
	material.id,
	balance.construction_site_id,
	public.construction_sites_ref(site) AS construction_site,
	material.material_type_id,
	public.material_types_ref(material_type) AS material_type,
	material_type.name AS material_type_name,
	balance.material_id,
	public.materials_ref(material) AS material,
	material.name AS material_name,
	material.measure_unit_id,
	public.measure_units_ref(measure_unit) AS measure_unit,
	balance.quant::numeric(19, 4) AS balance
FROM public.rg_materials_current AS balance
JOIN public.construction_sites AS site ON site.id = balance.construction_site_id
JOIN public.materials AS material ON material.id = balance.material_id
JOIN public.material_types AS material_type ON material_type.id = material.material_type_id
JOIN public.measure_units AS measure_unit ON measure_unit.id = material.measure_unit_id
WHERE balance.quant <> 0;

DROP TRIGGER IF EXISTS material_receipt_items_closed_period_guard_trigger ON public.material_receipt_items;
DROP TRIGGER IF EXISTS material_consumption_items_closed_period_guard_trigger ON public.material_consumption_items;
DROP TRIGGER IF EXISTS material_transfer_items_closed_period_guard_trigger ON public.material_transfer_items;
DROP FUNCTION IF EXISTS public.material_document_item_closed_period_guard();
DROP TRIGGER IF EXISTS material_receipts_closed_period_guard_trigger ON public.material_receipts;
DROP TRIGGER IF EXISTS material_consumptions_closed_period_guard_trigger ON public.material_consumptions;
DROP TRIGGER IF EXISTS material_transfers_closed_period_guard_trigger ON public.material_transfers;
DROP FUNCTION IF EXISTS public.material_document_closed_period_guard();
DROP TRIGGER IF EXISTS ra_materials_closed_period_guard_trigger ON public.ra_materials;
DROP FUNCTION IF EXISTS public.ra_materials_closed_period_guard();
DROP FUNCTION IF EXISTS public.materials_assert_open_date(timestamptz);

DROP FUNCTION IF EXISTS public.materials_reopen_period(integer);
DROP FUNCTION IF EXISTS public.materials_close_period(date, integer);
DROP FUNCTION IF EXISTS public.materials_revalue();

DROP FUNCTION IF EXISTS public.rg_materials_balance(timestamptz, integer[], integer[]);
DROP FUNCTION IF EXISTS public.rg_materials_balance(integer[], integer[]);
DROP FUNCTION IF EXISTS public.rg_materials_rebuild();
DROP FUNCTION IF EXISTS public.ra_materials_add_act(timestamptz, text, bigint, bigint, smallint, integer, integer, numeric, numeric);

DROP TRIGGER IF EXISTS ra_materials_reject_update_trigger ON public.ra_materials;
DROP FUNCTION IF EXISTS public.ra_materials_reject_update();
DROP TRIGGER IF EXISTS ra_materials_process_trigger ON public.ra_materials;
DROP FUNCTION IF EXISTS public.ra_materials_process();
DROP FUNCTION IF EXISTS public.rg_materials_apply_delta(timestamptz, integer, integer, numeric, numeric, integer);

DROP TABLE IF EXISTS public.material_valuation_snapshots;
DROP TABLE IF EXISTS public.material_valuation_closures;

ALTER TABLE public.rg_materials_current
	DROP COLUMN IF EXISTS pending_count,
	DROP COLUMN IF EXISTS amount;
ALTER TABLE public.rg_materials_period
	DROP COLUMN IF EXISTS pending_count,
	DROP COLUMN IF EXISTS amount;

DROP INDEX IF EXISTS public.ra_materials_valuation_order_idx;
ALTER TABLE public.ra_materials
	DROP CONSTRAINT IF EXISTS ra_materials_source_amount_chk,
	DROP CONSTRAINT IF EXISTS ra_materials_movement_order_chk,
	DROP COLUMN IF EXISTS amount_pending,
	DROP COLUMN IF EXISTS amount,
	DROP COLUMN IF EXISTS source_amount,
	DROP COLUMN IF EXISTS movement_order,
	DROP COLUMN IF EXISTS recorder_item_id;

ALTER TABLE public.register_settings
	DROP COLUMN IF EXISTS materials_closed_through,
	DROP COLUMN IF EXISTS materials_exclude_vat_from_cost,
	DROP COLUMN IF EXISTS materials_allow_negative_open_period;

CREATE OR REPLACE FUNCTION public.register_settings_validate()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
BEGIN
	BEGIN
		PERFORM timezone(NEW.business_timezone, CURRENT_TIMESTAMP);
	EXCEPTION
		WHEN invalid_parameter_value THEN
			RAISE EXCEPTION 'Invalid PostgreSQL time zone: %', NEW.business_timezone
				USING ERRCODE = '22023';
	END;

	RETURN NEW;
END;
$function$;

DROP TRIGGER IF EXISTS register_settings_validate_trigger ON public.register_settings;
CREATE TRIGGER register_settings_validate_trigger
	BEFORE INSERT OR UPDATE OF business_timezone
	ON public.register_settings
	FOR EACH ROW
	EXECUTE FUNCTION public.register_settings_validate();

CREATE OR REPLACE FUNCTION public.rg_materials_apply_delta(
	in_effective_at timestamptz,
	in_construction_site_id integer,
	in_material_id integer,
	in_delta_quant numeric(19, 4)
)
RETURNS void
LANGUAGE plpgsql
AS $function$
DECLARE
	v_period_start date;
BEGIN
	IF in_delta_quant = 0 THEN
		RETURN;
	END IF;

	v_period_start := public.register_month_start(in_effective_at);

	INSERT INTO public.rg_materials_period (
		period_start,
		construction_site_id,
		material_id,
		quant
	)
	VALUES (
		v_period_start,
		in_construction_site_id,
		in_material_id,
		in_delta_quant
	)
	ON CONFLICT (construction_site_id, material_id, period_start)
	DO UPDATE
	SET quant = rg_materials_period.quant + EXCLUDED.quant;

	DELETE FROM public.rg_materials_period
	WHERE period_start = v_period_start
		AND construction_site_id = in_construction_site_id
		AND material_id = in_material_id
		AND quant = 0;

	INSERT INTO public.rg_materials_current (
		construction_site_id,
		material_id,
		quant
	)
	VALUES (
		in_construction_site_id,
		in_material_id,
		in_delta_quant
	)
	ON CONFLICT (construction_site_id, material_id)
	DO UPDATE
	SET quant = rg_materials_current.quant + EXCLUDED.quant;

	DELETE FROM public.rg_materials_current
	WHERE construction_site_id = in_construction_site_id
		AND material_id = in_material_id
		AND quant = 0;
END;
$function$;

CREATE OR REPLACE FUNCTION public.ra_materials_process()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
BEGIN
	IF TG_OP = 'INSERT' THEN
		PERFORM public.rg_materials_apply_delta(
			NEW.effective_at,
			NEW.construction_site_id,
			NEW.material_id,
			NEW.quant
		);

		RETURN NEW;
	END IF;

	IF TG_OP = 'DELETE' THEN
		PERFORM public.rg_materials_apply_delta(
			OLD.effective_at,
			OLD.construction_site_id,
			OLD.material_id,
			-OLD.quant
		);

		RETURN OLD;
	END IF;

	RAISE EXCEPTION 'Unsupported operation % on ra_materials', TG_OP;
END;
$function$;

CREATE TRIGGER ra_materials_process_trigger
	AFTER INSERT OR DELETE
	ON public.ra_materials
	FOR EACH ROW
	EXECUTE FUNCTION public.ra_materials_process();

CREATE OR REPLACE FUNCTION public.ra_materials_reject_update()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
BEGIN
	RAISE EXCEPTION 'ra_materials rows are immutable; delete the old register actions and insert replacement actions instead'
		USING ERRCODE = '55000';
END;
$function$;

CREATE TRIGGER ra_materials_reject_update_trigger
	BEFORE UPDATE
	ON public.ra_materials
	FOR EACH ROW
	EXECUTE FUNCTION public.ra_materials_reject_update();

CREATE OR REPLACE FUNCTION public.ra_materials_add_act(
	in_effective_at timestamptz,
	in_recorder_type text,
	in_recorder_id bigint,
	in_construction_site_id integer,
	in_material_id integer,
	in_quant numeric(19, 4)
)
RETURNS bigint
LANGUAGE plpgsql
AS $function$
DECLARE
	v_id bigint;
BEGIN
	INSERT INTO public.ra_materials (
		effective_at,
		recorder_type,
		recorder_id,
		construction_site_id,
		material_id,
		quant
	)
	VALUES (
		in_effective_at,
		in_recorder_type,
		in_recorder_id,
		in_construction_site_id,
		in_material_id,
		in_quant
	)
	RETURNING id INTO v_id;

	RETURN v_id;
END;
$function$;

COMMENT ON FUNCTION public.ra_materials_add_act(timestamptz, text, bigint, integer, integer, numeric) IS 'Adds one immutable materials register action and returns its action id.';


CREATE OR REPLACE FUNCTION public.rg_materials_rebuild()
RETURNS void
LANGUAGE plpgsql
AS $function$
BEGIN
	TRUNCATE TABLE public.rg_materials_period, public.rg_materials_current;

	INSERT INTO public.rg_materials_period (
		period_start,
		construction_site_id,
		material_id,
		quant
	)
	SELECT
		public.register_month_start(action.effective_at),
		action.construction_site_id,
		action.material_id,
		SUM(action.quant)::numeric(19, 4)
	FROM public.ra_materials AS action
	GROUP BY
		public.register_month_start(action.effective_at),
		action.construction_site_id,
		action.material_id
	HAVING SUM(action.quant) <> 0;

	INSERT INTO public.rg_materials_current (
		construction_site_id,
		material_id,
		quant
	)
	SELECT
		action.construction_site_id,
		action.material_id,
		SUM(action.quant)::numeric(19, 4)
	FROM public.ra_materials AS action
	GROUP BY
		action.construction_site_id,
		action.material_id
	HAVING SUM(action.quant) <> 0;
END;
$function$;

COMMENT ON FUNCTION public.rg_materials_rebuild() IS 'Rebuilds all materials register aggregates from the authoritative ra_materials ledger. Run after changing register_settings.business_timezone.';

CREATE OR REPLACE FUNCTION public.rg_materials_balance(
	in_construction_site_ids integer[] DEFAULT NULL,
	in_material_ids integer[] DEFAULT NULL
)
RETURNS TABLE (
	construction_site_id integer,
	material_id integer,
	quant numeric(19, 4)
)
LANGUAGE sql
STABLE
AS $function$
	SELECT
		balance.construction_site_id,
		balance.material_id,
		balance.quant
	FROM public.rg_materials_current AS balance
	WHERE (
		in_construction_site_ids IS NULL
		OR cardinality(in_construction_site_ids) = 0
		OR balance.construction_site_id = ANY(in_construction_site_ids)
	)
	AND (
		in_material_ids IS NULL
		OR cardinality(in_material_ids) = 0
		OR balance.material_id = ANY(in_material_ids)
	)
	AND balance.quant <> 0
	ORDER BY
		balance.construction_site_id,
		balance.material_id;
$function$;

COMMENT ON FUNCTION public.rg_materials_balance(integer[], integer[]) IS 'Returns the fast current posted balance of the materials register.';

CREATE OR REPLACE FUNCTION public.rg_materials_balance(
	in_effective_at timestamptz,
	in_construction_site_ids integer[] DEFAULT NULL,
	in_material_ids integer[] DEFAULT NULL
)
RETURNS TABLE (
	construction_site_id integer,
	material_id integer,
	quant numeric(19, 4)
)
LANGUAGE sql
STABLE
AS $function$
	WITH boundary AS (
		SELECT
			public.register_month_start(in_effective_at) AS period_start,
			public.register_month_start_at(in_effective_at) AS period_start_at
	), movements AS (
		SELECT
			period.construction_site_id,
			period.material_id,
			period.quant
		FROM public.rg_materials_period AS period
		CROSS JOIN boundary
		WHERE period.period_start < boundary.period_start
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
		WHERE action.effective_at >= boundary.period_start_at
			AND action.effective_at <= in_effective_at
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
	)
	SELECT
		movements.construction_site_id,
		movements.material_id,
		SUM(movements.quant)::numeric(19, 4) AS quant
	FROM movements
	GROUP BY
		movements.construction_site_id,
		movements.material_id
	HAVING SUM(movements.quant) <> 0
	ORDER BY
		movements.construction_site_id,
		movements.material_id;
$function$;

COMMENT ON FUNCTION public.rg_materials_balance(timestamptz, integer[], integer[]) IS 'Returns materials balance at an exact instant using completed monthly turnovers plus raw actions from the requested month.';


COMMIT;
