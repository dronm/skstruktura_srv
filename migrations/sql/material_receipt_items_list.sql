CREATE OR REPLACE VIEW public.material_receipt_items_list AS
SELECT
	row.id,
	row.line_num,
	row.material_receipt_id,
	row.material_id,
	row.measure_unit_id,
	row.construction_site_id,
	row.quant,
	row.price,
	row.amount,
	row.vat_percent,
	row.vat_amount,
	public.materials_ref(material_ref_row) AS material,
	public.measure_units_ref(measure_unit_ref_row) AS measure_unit,
	public.construction_sites_ref(construction_site_ref_row) AS construction_site
FROM public.material_receipt_items AS row
LEFT JOIN public.materials AS material_ref_row ON material_ref_row.id = row.material_id
LEFT JOIN public.measure_units AS measure_unit_ref_row ON measure_unit_ref_row.id = row.measure_unit_id
LEFT JOIN public.construction_sites AS construction_site_ref_row ON construction_site_ref_row.id = row.construction_site_id;
