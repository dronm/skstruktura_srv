BEGIN;

-- Run during maintenance with application/background document writers stopped.
-- Keep source documents, register movements, balances and closing history stable
-- until the VAT basis change and the complete replay have committed together.
LOCK TABLE
	public.material_receipts,
	public.material_receipt_items,
	public.material_consumptions,
	public.material_consumption_items,
	public.material_transfers,
	public.material_transfer_items,
	public.register_settings,
	public.ra_materials,
	public.rg_materials_period,
	public.rg_materials_current,
	public.material_valuation_closures,
	public.material_valuation_snapshots
IN ACCESS EXCLUSIVE MODE;

ALTER TABLE public.register_settings
	ALTER COLUMN materials_exclude_vat_from_cost SET DEFAULT false;

DO $migration$
DECLARE
	v_closed_through date;
BEGIN
	SELECT materials_closed_through
	INTO STRICT v_closed_through
	FROM public.register_settings
	WHERE id = 1;

	-- Only these recorder types can be reconstructed from the source documents.
	IF EXISTS (
		SELECT 1
		FROM public.ra_materials
		WHERE recorder_type NOT IN ('MaterialReceipt', 'MaterialConsumption', 'MaterialTransfer')
	) THEN
		RAISE EXCEPTION 'Cannot change material VAT basis: register contains unsupported recorder types'
			USING ERRCODE = '55000';
	END IF;

	-- Retain the old closure/snapshots as history. Closing below creates a fresh
	-- snapshot on the new cost basis and restores the same inclusive cutoff.
	IF v_closed_through IS NOT NULL THEN
		PERFORM public.materials_reopen_period(NULL);
	END IF;

	TRUNCATE TABLE
		public.ra_materials,
		public.rg_materials_period,
		public.rg_materials_current
	RESTART IDENTITY;

	-- The initialization-only validation trigger remains enabled: the ledger is
	-- empty now, so the change is allowed without weakening normal protection.
	UPDATE public.register_settings
	SET materials_exclude_vat_from_cost = false
	WHERE id = 1;

	PERFORM public.ra_materials_add_act(
		receipt.date,
		'MaterialReceipt',
		receipt.id,
		item.id,
		1::smallint,
		COALESCE(item.construction_site_id, receipt.construction_site_id),
		item.material_id,
		item.quant,
		CASE
			WHEN settings.materials_exclude_vat_from_cost THEN item.amount - item.vat_amount
			ELSE item.amount
		END
	)
	FROM public.material_receipts AS receipt
	JOIN public.material_receipt_items AS item
		ON item.material_receipt_id = receipt.id
	CROSS JOIN public.register_settings AS settings
	WHERE settings.id = 1
	ORDER BY receipt.date, receipt.id, item.line_num, item.id;

	PERFORM public.ra_materials_add_act(
		consumption.date,
		'MaterialConsumption',
		consumption.id,
		item.id,
		1::smallint,
		consumption.construction_site_id,
		item.material_id,
		-item.quant,
		NULL
	)
	FROM public.material_consumptions AS consumption
	JOIN public.material_consumption_items AS item
		ON item.material_consumption_id = consumption.id
	ORDER BY consumption.date, consumption.id, item.line_num, item.id;

	PERFORM public.ra_materials_add_act(
		transfer.date,
		'MaterialTransfer',
		transfer.id,
		movement.id,
		movement.movement_order::smallint,
		movement.construction_site_id,
		movement.material_id,
		movement.quant,
		NULL
	)
	FROM public.material_transfers AS transfer
	JOIN LATERAL (
		SELECT
			item.line_num,
			item.id,
			transfer.source_construction_site_id AS construction_site_id,
			item.material_id,
			-item.quant AS quant,
			1 AS movement_order
		FROM public.material_transfer_items AS item
		WHERE item.material_transfer_id = transfer.id

		UNION ALL

		SELECT
			item.line_num,
			item.id,
			transfer.destination_construction_site_id AS construction_site_id,
			item.material_id,
			item.quant AS quant,
			2 AS movement_order
		FROM public.material_transfer_items AS item
		WHERE item.material_transfer_id = transfer.id
	) AS movement ON true
	ORDER BY transfer.date, transfer.id, movement.line_num, movement.id, movement.movement_order;

	IF v_closed_through IS NULL THEN
		PERFORM public.materials_revalue();
	ELSE
		-- This function also runs materials_revalue(), rejects pending/negative
		-- closed history, writes a new snapshot and restores the closing cutoff.
		PERFORM public.materials_close_period(v_closed_through, NULL);
	END IF;
END;
$migration$;

COMMIT;
