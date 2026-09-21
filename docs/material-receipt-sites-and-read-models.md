# Receipt line sites and reference read models

Receipt posting now uses `COALESCE(item.construction_site_id, receipt.construction_site_id)`.

- The receipt header site and each item site are nullable (`*int` in Go).
- Every item must have a positive effective site: set the item site, or set the header site.
- A non-null item site overrides the header. Null stays null in storage and API responses so later header edits still affect inherited lines.
- Zero and negative IDs are rejected, including zero supplied as an attempted fallback.
- Complete-document create/update saves line overrides and reposts the document in the existing transaction. Changing the header, changing an override, or clearing an override removes the previous register actions and rebuilds them using the new effective sites.
- Consumption and transfer site rules are unchanged.

## Database update

The existing last migration, `000031_material_receipt_items_constr_site`, was updated in place as requested. Its up script adds the nullable item column, a foreign key with `ON UPDATE CASCADE ON DELETE RESTRICT`, an index, and makes the header column nullable. Deleting a construction site cannot silently delete receipt lines.

The same migration installs the reference functions and all read views. Their canonical SQL is also in `migrations/sql`. The migration embeds ordinary SQL so it works with golang-migrate; it does not depend on psql include commands or a separate `make runsql` step.

If the database is at version 30, apply migration 31 normally:

```bash
make migup
```

If the earlier version of migration 31 has already been applied, a normal migration run will not rerun the edited file. Run the revised up script once against that database:

```bash
psql "$DB_CONN" -v ON_ERROR_STOP=1 -f migrations/000031_material_receipt_items_constr_site.up.sql
```

That script can be reapplied. No migration-version change is required when the database is already at version 31. Apply the SQL before starting the updated backend.

Existing register actions are not rewritten by the migration. Receipts that already contained populated line-site overrides before this update need to be saved/reposted through the document service to bring their existing actions into agreement with those overrides.

Rollback refuses to discard data if a header is null or a line overrides its header with a different site. Resolve these documents through the service before rolling back. Compatible rollback retains reusable reference helpers and the pre-existing materials view.

## Read API

The original models still describe writable tables. YAML `list` sections generate the following read models; both service `List()` and `Detail()` use these view-backed models:

| Entity | Read model | Reference fields |
| --- | --- | --- |
| Material | MaterialList | measure_unit; existing balances retained |
| MaterialReceipt | MaterialReceiptList | construction_site, supplier |
| MaterialConsumption | MaterialConsumptionList | construction_site |
| MaterialTransfer | MaterialTransferList | source_construction_site, destination_construction_site |
| MaterialReceiptItem | MaterialReceiptItemList | material, measure_unit, construction_site |
| MaterialConsumptionItem | MaterialConsumptionItemList | material, measure_unit |
| MaterialTransferItem | MaterialTransferItemList | material, measure_unit |

All ID fields remain available. References use the existing `Ref` shape with `keys.id` and `descr`. Unset receipt header and item site references are JSON null. An item's site reference describes its explicit override; it does not copy the header reference into the item.

`construction_sites_ref()` supplies site references. Unit, material, and supplier references have analogous null-safe SQL functions. The pre-existing incorrect `dataType: users` values in the construction-site and measure-unit SQL functions were corrected.

The document HTTP detail routes already invoke `DocumentDetail()`, returning a complete document with items. That path, along with create/update responses, now retrieves header and item references through the new views in the existing single joined query. It preserves the complete-document API shape and optimistic version handling. No per-reference queries are added. List pagination retains the framework's existing aggregate/count behavior.

`Material.name_full` is now nullable in both the writable and read models, matching the existing database column and avoiding scan failures for materials without a full name.

## Code generation

The current public codegen source supports `list.model`, `list.table`, and `list.fields`, but does not define a separate top-level detail projection setting. Consequently:

- The seven entity YAML files define their read projections under `list`.
- They declare `detail` under `service.manualMethods`.
- Small `*_read.go` files implement `Detail()` against the generated read models and view relations.
- The `.gen.go` files were regenerated from YAML, not hand-patched.
- Existing custom aggregate create/update/delete methods remain manual.

This makes the backend changes safe to regenerate without requiring a generator upgrade. List and detail share each entity's read model; they do not need two duplicate struct definitions.

Generation and drift checks passed for the seven changed schemas using backend-only generation. Full-project validation with the current public generator stops on a pre-existing issue: `MeasureUnit`'s frontend form does not include the required writable create field `okei_code`. This unrelated form configuration is unchanged. It must be addressed before full-project generation with that generator version.

## Verification

- `go test ./...`: passed, including new cases for missing sites, per-line sites, mixed inheritance/overrides, invalid IDs, and nullable-header JSON binding.
- Backend build: passed.
- Codegen generation and drift check: passed for the seven affected schemas.
- PostgreSQL execution using PGlite: migration 31, all views/reference functions, actual receipt posting SQL, override clearing, header changes, reference values, restricted site deletion, rollback guards, down/up reapplication: passed.

Go checks used temporary local copies of the public dependency sources because the `/home/andrey/go/...` replacements are specific to your machine. Your `go.mod` and `go.sum` were preserved. No live application database or authenticated API server was used during verification.
