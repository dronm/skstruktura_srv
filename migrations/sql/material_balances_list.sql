CREATE OR REPLACE VIEW public.material_balances_list AS
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
	balance.quant::numeric(19, 4) AS balance,
	balance.amount::numeric(19, 4) AS amount,
	CASE
		WHEN balance.amount IS NULL OR balance.quant = 0 THEN NULL
		ELSE (balance.amount / balance.quant)::numeric(19, 6)
	END AS average_cost,
	balance.pending_count > 0 AS amount_pending
FROM public.rg_materials_current AS balance
JOIN public.construction_sites AS site
	ON site.id = balance.construction_site_id
JOIN public.materials AS material
	ON material.id = balance.material_id
JOIN public.material_types AS material_type
	ON material_type.id = material.material_type_id
JOIN public.measure_units AS measure_unit
	ON measure_unit.id = material.measure_unit_id
WHERE balance.quant <> 0
	OR balance.pending_count > 0
	OR COALESCE(balance.amount, 0) <> 0;

COMMENT ON VIEW public.material_balances_list IS
	'Current material quantity and monetary balances with material-type grouping metadata.';
