BEGIN;

-- Refuse rollback when the old header-only model cannot represent the data.
DO $BODY$
BEGIN
	IF EXISTS (SELECT 1 FROM public.material_receipts WHERE construction_site_id IS NULL)
		OR EXISTS (
			SELECT 1 FROM public.material_receipt_items item
			JOIN public.material_receipts receipt ON receipt.id = item.material_receipt_id
			WHERE item.construction_site_id IS NOT NULL
				AND item.construction_site_id IS DISTINCT FROM receipt.construction_site_id
		) THEN
		RAISE EXCEPTION 'Cannot roll back: assign receipt header sites and resolve line site overrides first';
	END IF;
END;
$BODY$;

DROP VIEW IF EXISTS public.material_receipts_list;
DROP VIEW IF EXISTS public.material_consumptions_list;
DROP VIEW IF EXISTS public.material_transfers_list;
DROP VIEW IF EXISTS public.material_receipt_items_list;
DROP VIEW IF EXISTS public.material_consumption_items_list;
DROP VIEW IF EXISTS public.material_transfer_items_list;
ALTER TABLE public.material_receipts ALTER COLUMN construction_site_id SET NOT NULL;
ALTER TABLE public.material_receipt_items DROP COLUMN IF EXISTS construction_site_id;

-- Keep reusable reference helpers and the pre-existing materials_list view.
COMMIT;
