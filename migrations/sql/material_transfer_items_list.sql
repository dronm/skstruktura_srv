CREATE OR REPLACE VIEW public.material_transfer_items_list AS
SELECT
	row.id,
	row.line_num,
	row.material_transfer_id,
	row.material_id,
	row.measure_unit_id,
	row.quant,
	public.materials_ref(material_ref_row) AS material,
	public.measure_units_ref(measure_unit_ref_row) AS measure_unit
FROM public.material_transfer_items AS row
LEFT JOIN public.materials AS material_ref_row ON material_ref_row.id = row.material_id
LEFT JOIN public.measure_units AS measure_unit_ref_row ON measure_unit_ref_row.id = row.measure_unit_id;
