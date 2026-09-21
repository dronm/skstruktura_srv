# Material document API

Material receipts, consumptions, and transfers are saved as complete
master-detail aggregates. The header, all item changes, and all corresponding
`ra_materials` actions are committed in one database transaction.

## Endpoints

Each document type has the same endpoint shape:

| Operation | Receipt | Consumption | Transfer |
| --- | --- | --- | --- |
| Create | `POST /api/material-receipts` | `POST /api/material-consumptions` | `POST /api/material-transfers` |
| List headers | `GET /api/material-receipts` | `GET /api/material-consumptions` | `GET /api/material-transfers` |
| Complete detail | `GET /api/material-receipts/{id}` | `GET /api/material-consumptions/{id}` | `GET /api/material-transfers/{id}` |
| Replace complete document | `PUT /api/material-receipts/{id}` | `PUT /api/material-consumptions/{id}` | `PUT /api/material-transfers/{id}` |
| Delete | `DELETE /api/material-receipts/{id}` | `DELETE /api/material-consumptions/{id}` | `DELETE /api/material-transfers/{id}` |

Standalone item CRUD endpoints are not exposed. Header list endpoints remain
lightweight; use a detail endpoint when the item collection is needed.

## Receipt example

Create requests omit server-generated document and item identifiers, the
document version, and line numbers:

```json
{
	"date": "2026-08-22T08:45:00+03:00",
	"construction_site_id": 10,
	"supplier_id": 20,
	"number": "R-42",
	"comment": "Optional comment",
	"items": [
		{
			"material_id": 30,
			"measure_unit_id": 40,
			"quant": 5.5,
			"price": 12.25,
			"amount": 67.38,
			"vat_percent": 20,
			"vat_amount": 11.23
		}
	]
}
```

For receipt items, `amount` is the gross line total including VAT.
`vat_amount` is the VAT portion already included in that total, and
`vat_percent` is stored as a percentage. VAT is receipt-level financial data;
the material register continues to store quantity movements only.

The response contains the complete saved document. It includes `id`,
`version`, each item `id`, and one-based `line_num` values assigned from array
order.

Document `date` values are exact timestamps. Clients send an RFC 3339 value
with an offset (or its equivalent UTC value), and PostgreSQL stores it as
`timestamptz`.

## Complete replacement and item identity

A `PUT` request sends the complete desired state and the `version` returned by
the last create, update, or detail request.

- An item with its existing positive `id` is updated and may be reordered.
- An item with an omitted or zero `id` is inserted.
- An existing item omitted from the array is deleted.
- An item ID belonging to another document rejects the entire request.
- Array order becomes `line_num`; client-provided line numbers are ignored.

Every successful replacement increments the document version. A stale version
returns HTTP `409 Conflict`, preventing one editor from silently overwriting a
newer document.

All writes are atomic: validation, header changes, item reconciliation, removal
of old register actions, and creation of replacement actions either all commit
or all roll back. Receipt quantities produce positive actions, consumption
quantities produce negative actions, and transfers produce one negative source
action plus one positive destination action per item.
