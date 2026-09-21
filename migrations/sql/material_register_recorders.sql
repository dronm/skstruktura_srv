
-- DROP VIEW public.material_register_recorders;

CREATE OR REPLACE VIEW public.material_register_recorders AS
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
