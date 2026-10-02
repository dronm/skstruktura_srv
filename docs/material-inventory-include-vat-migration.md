# Include VAT in material inventory cost

Migration `000056_material_inventory_include_vat` changes
`register_settings.materials_exclude_vat_from_cost` to `false` and rebuilds the
entire materials register from receipts, consumptions and transfers. Receipt
acquisition cost becomes the full line `amount`, including `vat_amount`.
Consumption and transfer values are recalculated using the existing
weighted-average valuation rules.

## Apply

1. Copy both migration files into the project's `migrations/` directory. This
	package follows migration `000055` in the supplied backend.
2. Make a database backup and stop the backend and any background processes that
	write material documents.
3. From the project directory, use your existing database configuration:

	```bash
	make migver
	make migup
	```

	The supplied Makefile applies one migration per `make migup`. If the database
	is already at version `55`, this runs migration `56`. If it is behind, apply
	the earlier migrations first.
4. Start the backend and check the materials balance report.

## Behavior

- The flag and its column default become `false`.
- The register movements and current/monthly balances are rebuilt. Source
	documents and their quantity, amount and VAT fields are not edited.
- An open period remains open. Unresolved open-period shortages follow the
	existing valuation rules and can remain pending.
- If a period is closed, the migration reopens it, rebuilds the register and
	closes it at the same date. Existing closure/snapshot history is retained;
	a new snapshot records VAT-inclusive values. The migration's reopen/close
	operations have no application user attribution.
- Normal VAT-basis and closed-period protection triggers remain enabled.
- Unsupported recorder types cause an error before the register is cleared.
- The complete operation runs in one transaction. Replay or closing errors roll
	back the setting, register data, default and closure changes together.
- Register action IDs are regenerated, matching the existing full-rebuild
	behavior. Source document and item IDs remain unchanged.
- The migration holds exclusive table locks until completion, so run it during
	maintenance; the duration depends on the document volume.

## Roll back

With the backend/background writers stopped and the database at version `56`:

```bash
make migdown
```

The down migration sets the flag/default to `true`, rebuilds all movements with
VAT-exclusive receipt costs and restores the same closed cutoff if one exists.
It also retains historical snapshots and creates a new closing snapshot.

Changing only the flag or calling `materials_revalue()` on the old movements is
insufficient: receipt costs are stored in immutable movement `source_amount`.
The service's current settings update also changes the flag before rebuilding,
which the database rejects when register movements already exist. This
migration clears the register inside the locked transaction before changing
the flag, then reconstructs every movement from its source document.
