BEGIN;

ALTER TABLE public.material_receipts
	ADD COLUMN version bigint NOT NULL DEFAULT 1;

ALTER TABLE public.material_consumptions
	ADD COLUMN version bigint NOT NULL DEFAULT 1;

ALTER TABLE public.material_transfers
	ADD COLUMN version bigint NOT NULL DEFAULT 1;

COMMENT ON COLUMN public.material_receipts.version IS 'Optimistic concurrency version incremented after every complete document update.';
COMMENT ON COLUMN public.material_consumptions.version IS 'Optimistic concurrency version incremented after every complete document update.';
COMMENT ON COLUMN public.material_transfers.version IS 'Optimistic concurrency version incremented after every complete document update.';

COMMIT;
