BEGIN;

ALTER TABLE public.materials
	DROP CONSTRAINT IF EXISTS materials_material_type_id_fkey;

ALTER TABLE public.materials
	ADD CONSTRAINT materials_material_type_id_fkey
	FOREIGN KEY (material_type_id)
	REFERENCES public.material_types (id)
	ON UPDATE CASCADE
	ON DELETE RESTRICT;

CREATE OR REPLACE FUNCTION public.material_types_ref(public.material_types)
RETURNS json AS
$BODY$
	SELECT CASE WHEN $1.id IS NULL THEN NULL ELSE json_build_object(
		'keys', json_build_object('id', $1.id),
		'descr', $1.name,
		'dataType', 'materialTypes'
	) END;
$BODY$
LANGUAGE sql STABLE;

CREATE OR REPLACE VIEW public.materials_list AS
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
LEFT JOIN public.material_types AS mt ON mt.id = m.material_type_id;

COMMENT ON VIEW public.materials_list IS
	'Material list including material type and the current posted balance at every active construction site.';

SELECT setval(
	pg_get_serial_sequence('public.material_types', 'id'),
	COALESCE(MAX(id), 1),
	MAX(id) IS NOT NULL
)
FROM public.material_types;

COMMIT;
