BEGIN;

ALTER TABLE public.material_transfers
	DROP COLUMN IF EXISTS version;

ALTER TABLE public.material_consumptions
	DROP COLUMN IF EXISTS version;

ALTER TABLE public.material_receipts
	DROP COLUMN IF EXISTS version;

COMMIT;
