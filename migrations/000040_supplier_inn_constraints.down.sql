BEGIN;

DROP INDEX IF EXISTS public.suppliers_inn_kpp_idx;

ALTER TABLE public.suppliers
	DROP CONSTRAINT IF EXISTS suppliers_inn_not_empty,
	ALTER COLUMN inn DROP NOT NULL;

COMMIT;
