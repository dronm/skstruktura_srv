BEGIN;

ALTER TABLE public.suppliers
	ALTER COLUMN inn SET NOT NULL,
	ADD CONSTRAINT suppliers_inn_not_empty CHECK (btrim(inn) <> '');

CREATE UNIQUE INDEX suppliers_inn_kpp_idx
	ON public.suppliers USING btree (inn, (COALESCE(kpp, '')));

COMMIT;
