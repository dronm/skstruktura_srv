package services

import (
	"context"
	"fmt"
	"sort"

	"github.com/dronm/ds/v4"
)

const (
	materialReceiptRecorderType     = "MaterialReceipt"
	materialConsumptionRecorderType = "MaterialConsumption"
	materialTransferRecorderType    = "MaterialTransfer"
)

func withPrimaryTransaction(
	ctx context.Context,
	db ds.Provider,
	fn func(ds.Tx) error,
) error {
	poolConn, connID, err := db.GetPrimary(ctx)
	if err != nil {
		return fmt.Errorf("get primary connection: %w", err)
	}
	defer db.Release(poolConn, connID)

	if err := ds.WithTx(ctx, poolConn.Conn(), func(ctx context.Context, tx ds.Tx) error {
		return fn(tx)
	}); err != nil {
		return err
	}

	return nil
}

func lockMaterialRecorders(
	ctx context.Context,
	tx ds.Querier,
	recorderType string,
	recorderIDs ...int,
) error {
	ids := append([]int(nil), recorderIDs...)
	sort.Ints(ids)

	previousID := 0
	for _, recorderID := range ids {
		if recorderID <= 0 || recorderID == previousID {
			continue
		}

		if _, err := tx.Exec(
			ctx,
			"SELECT pg_advisory_xact_lock(hashtext($1), $2)",
			recorderType,
			recorderID,
		); err != nil {
			return fmt.Errorf(
				"lock %s register recorder %d: %w",
				recorderType,
				recorderID,
				err,
			)
		}

		previousID = recorderID
	}

	return nil
}

func removeMaterialRegisterActions(
	ctx context.Context,
	tx ds.Querier,
	recorderType string,
	recorderID int,
) error {
	if _, err := tx.Exec(
		ctx,
		"SELECT public.ra_materials_remove_acts($1, $2)",
		recorderType,
		recorderID,
	); err != nil {
		return fmt.Errorf(
			"remove %s register actions for recorder %d: %w",
			recorderType,
			recorderID,
			err,
		)
	}

	return nil
}

func rebuildMaterialRegisterActions(
	ctx context.Context,
	tx ds.Querier,
	recorderType string,
	recorderID int,
) error {
	if err := removeMaterialRegisterActions(ctx, tx, recorderType, recorderID); err != nil {
		return err
	}

	var query string
	switch recorderType {
	case materialReceiptRecorderType:
		query = `
			SELECT public.ra_materials_add_act(
				receipt.date,
				$1,
				receipt.id,
				COALESCE(item.construction_site_id, receipt.construction_site_id),
				item.material_id,
				item.quant
			)
			FROM public.material_receipts AS receipt
			JOIN public.material_receipt_items AS item
				ON item.material_receipt_id = receipt.id
			WHERE receipt.id = $2
			ORDER BY item.line_num, item.id
		`
	case materialConsumptionRecorderType:
		query = `
			SELECT public.ra_materials_add_act(
				consumption.date,
				$1,
				consumption.id,
				consumption.construction_site_id,
				item.material_id,
				-item.quant
			)
			FROM public.material_consumptions AS consumption
			JOIN public.material_consumption_items AS item
				ON item.material_consumption_id = consumption.id
			WHERE consumption.id = $2
			ORDER BY item.line_num, item.id
		`
	case materialTransferRecorderType:
		query = `
			SELECT public.ra_materials_add_act(
				transfer.date,
				$1,
				transfer.id,
				movement.construction_site_id,
				movement.material_id,
				movement.quant
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
			WHERE transfer.id = $2
			ORDER BY movement.line_num, movement.id, movement.movement_order
		`
	default:
		return fmt.Errorf("unsupported material register recorder type %q", recorderType)
	}

	if _, err := tx.Exec(ctx, query, recorderType, recorderID); err != nil {
		return fmt.Errorf(
			"write %s register actions for recorder %d: %w",
			recorderType,
			recorderID,
			err,
		)
	}

	return nil
}
