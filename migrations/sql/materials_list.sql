-- View: public.materials_list

-- DROP VIEW public.materials_list;

CREATE OR REPLACE VIEW public.materials_list
 AS
SELECT 
	m.id,
	m.name,
	m.name_full,
	m.measure_unit_id,
	public.measure_units_ref(mu) AS measure_unit,
	m.is_active,
	COALESCE(
		(
			SELECT jsonb_agg(
				jsonb_build_object(
					'construction_site_id', site.id,
					'construction_site', jsonb_build_object(
						'keys', jsonb_build_object('id', site.id),
						'descr', site.name,
						'dataType', 'constructionSites'
					),
					'quant', COALESCE(balance.quant, 0::numeric)
				)
				ORDER BY lower(site.name), site.id
			)
			FROM public.construction_sites AS site
			LEFT JOIN public.rg_materials_current AS balance
				ON balance.construction_site_id = site.id
				AND balance.material_id = m.id
			WHERE site.is_active
		),
		'[]'::jsonb
	) AS balances,
	m.material_type_id,
	public.material_types_ref(mt) AS material_type
FROM public.materials AS m
LEFT JOIN public.measure_units AS mu ON mu.id = m.measure_unit_id
LEFT JOIN public.material_types AS mt ON mt.id = m.material_type_id
;
