# Object history API

The common object-history endpoint reads audit rows from `public.audit_log`.

```text
GET /api/object-history
```

Query parameters:

| Parameter | Required | Description |
| --- | --- | --- |
| `object_type` | yes | Unqualified audited table name, for example `construction_sites` |
| `object_id` | yes | Value stored in `audit_log.record_id`; composite IDs remain strings |
| `from` | no | Zero-based row offset |
| `count` | no | Page size; defaults to 50 and is limited to 500 |

Rows are always returned newest first. Generic collection filters and sorters are
not accepted because the endpoint has a fixed object scope and chronology.

Example:

```text
GET /api/object-history?object_type=construction_sites&object_id=1&from=0&count=50
```

Response:

```json
{
	"rows": [
		{
			"id": 2,
			"changed_at": "2026-08-22T08:10:27.949924+05:00",
			"operation": "U",
			"changed_by": "skstruktura",
			"changes": [
				{
					"column": "name",
					"field": "Наименование",
					"old": "Старое наименование",
					"new": "Новое наименование"
				}
			]
		}
	],
	"total": 1
}
```

`field` uses the stored audit alias and falls back to the database column name.
Foreign-key values include `old_description` and `new_description` when the audit
trigger resolved them. Credential and secret columns are omitted from API output.

The endpoint requires the `objectHistory.list` permission. Migration 24 grants it
to the administrator role.
