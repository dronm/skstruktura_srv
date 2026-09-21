BEGIN;

DROP VIEW public.materials_list;

CREATE VIEW public.materials_list AS
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
	) AS balances
FROM public.materials AS m
LEFT JOIN public.measure_units AS mu ON mu.id = m.measure_unit_id;

COMMENT ON VIEW public.materials_list IS
	'Material list including the current posted balance at every active construction site.';

DROP FUNCTION IF EXISTS public.material_types_ref(public.material_types);

ALTER TABLE public.materials
	DROP CONSTRAINT IF EXISTS materials_material_type_id_fkey;

ALTER TABLE public.materials
	ADD CONSTRAINT materials_material_type_id_fkey
	FOREIGN KEY (material_type_id)
	REFERENCES public.material_types (id);

COMMIT;
