CREATE OR REPLACE VIEW public.material_transfers_list AS
SELECT
	row.id,
	row.date,
	row.source_construction_site_id,
	row.destination_construction_site_id,
	row.comment,
	row.version,
	public.construction_sites_ref(source_construction_site_ref_row) AS source_construction_site,
	public.construction_sites_ref(destination_construction_site_ref_row) AS destination_construction_site
FROM public.material_transfers AS row
LEFT JOIN public.construction_sites AS source_construction_site_ref_row ON source_construction_site_ref_row.id = row.source_construction_site_id
LEFT JOIN public.construction_sites AS destination_construction_site_ref_row ON destination_construction_site_ref_row.id = row.destination_construction_site_id;
