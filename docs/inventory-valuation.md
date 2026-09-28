# Material inventory valuation

Migration `000054_material_inventory_valuation` extends the materials accumulation register from quantity-only accounting to quantity plus monetary value.

## Cost rules

- Receipt movements use the receipt line amount as their acquisition value.
- `register_settings.materials_exclude_vat_from_cost` controls whether recoverable VAT is excluded (`amount - vat_amount`). This setting is initialization-only and cannot be changed after the first material register movement.
- Consumption and transfer source movements use deterministic weighted-average cost.
- Exact depletion uses the exact remaining balance amount, avoiding rounding residue.
- Transfer destination movements receive exactly the calculated value removed from the source site.

## Negative stock

`register_settings.materials_allow_negative_open_period` controls whether open-period quantity may become negative.

When negative quantity is allowed, an outgoing movement that cannot be completely valued is stored with `amount = NULL` and `amount_pending = true`. A later valued receipt can resolve the shortage during `materials_revalue()`. Transfer values propagate over repeated deterministic replay passes.

A closed period may not contain any negative running quantity or unresolved monetary movement.

## Closing and reopening

`register_settings.materials_closed_through` is an inclusive business date. Material documents and their items cannot be inserted, changed, or deleted on or before that date.

Closing creates a row in `material_valuation_closures` and a balance snapshot in `material_valuation_snapshots`. Reopening clears the active cutoff but retains closure/snapshot history. A later close creates a new snapshot.

The management API is exposed under `/api/inventory-valuation` and the frontend page under `/inventory-valuation`.
