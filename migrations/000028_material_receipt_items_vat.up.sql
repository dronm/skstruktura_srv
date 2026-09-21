BEGIN;

ALTER TABLE public.material_receipt_items
	ADD COLUMN vat_percent numeric(5, 2) NOT NULL DEFAULT 0,
	ADD COLUMN vat_amount numeric(15, 2) NOT NULL DEFAULT 0,
	ADD CONSTRAINT material_receipt_items_vat_percent_chk
		CHECK (vat_percent >= 0 AND vat_percent <= 100),
	ADD CONSTRAINT material_receipt_items_vat_amount_chk
		CHECK (vat_amount >= 0 AND vat_amount <= amount);

COMMENT ON COLUMN public.material_receipt_items.amount
	IS 'Total line amount including VAT.';

COMMENT ON COLUMN public.material_receipt_items.vat_percent
	IS 'VAT rate as a percentage.';

COMMENT ON COLUMN public.material_receipt_items.vat_amount
	IS 'VAT amount included in the total line amount.';

COMMIT;
