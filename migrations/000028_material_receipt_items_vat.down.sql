BEGIN;

ALTER TABLE material_receipt_items DROP COLUMN IF EXISTS vat_percent;
ALTER TABLE material_receipt_items DROP COLUMN IF EXISTS vat_amount;

COMMIT;

