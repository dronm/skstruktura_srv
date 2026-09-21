# Diadoc integration and receipt import

The integration implements OAuth authorization, token refresh, incremental
`GetNewEvents V8` polling, `GetEntityContent V4` retrieval, a durable document
buffer, user-controlled matching, and synchronous material receipt import.

## Configuration

Add the following section to `config.json`:

```json
{
	"diadoc": {
		"entry_point": "https://diadoc-api.kontur.ru",
		"identity_entry_point": "https://identity.kontur.ru",
		"client_id": "...",
		"client_secret": "...",
		"redirect_url": "https://example.com/api/diadoc/auth/callback",
		"box_id": "",
		"poll_interval": "1m",
		"request_timeout": "30s",
		"event_timestamp_from": "",
		"event_limit": 100,
		"log_document_content": true,
		"max_logged_content_size": 262144
	}
}
```

Register the exact `redirect_url` in the Kontur integrator cabinet. The scope is
selected automatically from `entry_point`; it can be overridden with `scope`.

If `event_timestamp_from` is empty, the first run starts at application startup.
After migration `000029`, the database value is authoritative. Use the replay
endpoint to change the starting date safely.

If the authorized user has exactly one box, it is selected automatically. If
there are several boxes, the authorization callback returns their identifiers;
set the required one in `diadoc.box_id` and restart the application.

## Operation

1. Apply migrations through `000030_diadoc_import_gui`.
2. Log in to the application as an administrator.
3. Open `GET /api/diadoc/auth/start` in the same browser.
4. Complete authorization on the Kontur page.
5. Inspect `GET /api/diadoc/status`.
6. Use `POST /api/diadoc/sync` to perform an immediate bounded poll.

The background worker retrieves inbound `UniversalTransferDocument`
attachments. Each qualifying attachment is stored and parsed before its event
cursor is advanced. Replayed events are idempotent by Diadoc box, message, and
entity identity. Imported and ignored rows remain as durable tombstones.

The access token, refresh token, selected box, starting timestamp, and event
cursor are stored in `integration_diadoc.state`. Restrict direct database access
to this schema because OAuth tokens are sensitive.

## Buffer workflow

Buffered documents use these user-facing states:

- `needs_matching`: a supplier, construction site, or material is unresolved;
- `ready`: all server-side readiness checks passed;
- `failed`: content download or XML parsing failed and can be retried;
- `ignored`: removed from the active buffer without deleting its identity;
- `imported`: atomically converted into a material receipt.

Supplier matches and supplier-specific material matches can be remembered and
applied to later unresolved documents. The API calculates target quantities and
gross prices on the server with six decimal places. Source net amount, VAT
rate, VAT amount, and gross amount remain visible in the buffer. Imported
receipt `amount` and `price` are gross values including VAT; `vat_amount` is
stored separately as the included VAT portion. Converted quantity changes the
target unit price, but never the source gross amount or VAT values. Material
registers remain quantity-only.

Document detail keeps immutable Diadoc number/date fields separate from the
editable receipt-side `receipt_number`, `receipt_date`, and `receipt_comment`.
Its `totals` object contains `document`, `import`, and `excluded` groups. Every
group includes line count, net amount, VAT amount, and gross amount.

## Buffer endpoints

| Operation | Endpoint | Permission |
| --- | --- | --- |
| List buffered documents | `GET /api/diadoc/documents` | `diadocDocument.list` |
| Get matching detail | `GET /api/diadoc/documents/{id}` | `diadocDocument.detail` |
| Save complete resolution | `PUT /api/diadoc/documents/{id}/resolution` | `diadocDocument.resolve` |
| Exclude one line | `POST /api/diadoc/documents/{id}/items/{itemId}/exclude` | `diadocDocument.resolve` |
| Restore one line | `POST /api/diadoc/documents/{id}/items/{itemId}/restore` | `diadocDocument.resolve` |
| Ignore document | `DELETE /api/diadoc/documents/{id}` | `diadocDocument.ignore` |
| Restore ignored document | `POST /api/diadoc/documents/{id}/restore` | `diadocDocument.ignore` |
| Retry failed document | `POST /api/diadoc/documents/{id}/retry` | `diadocDocument.resolve` |
| Import material receipt | `POST /api/diadoc/documents/{id}/import` | `diadocDocument.import` and `materialReceipt.create` |

List query parameters are `status`, `date_from`, `date_to`, `supplier_id`,
`construction_site_id`, `search`, `from`, and `count`. The default status is
`active`, which includes received, matching, ready, and failed documents.

Resolution, line exclusion/restoration, ignore, restore, retry, and import
requests contain the current document `version`. Stale writes return `409
Conflict`. Import locks the buffer row, requires at least one included line,
ignores excluded lines, revalidates all included references and monetary
values, creates the receipt and VAT-aware items, posts quantity register
actions, and marks the buffer row imported in one PostgreSQL transaction.

## State and replay endpoints

| Operation | Endpoint | Permission |
| --- | --- | --- |
| Safe state view | `GET /api/diadoc/state` | `diadocState.view` |
| Enable or pause polling | `PATCH /api/diadoc/state` | `diadocState.update` |
| Reset cursor from a date | `POST /api/diadoc/state/replay` | `diadocState.update` |
| Synchronous bounded poll | `POST /api/diadoc/sync` | `diadoc.sync` |

The safe state response never contains OAuth tokens. Replay accepts an RFC3339
`event_timestamp_from`, clears the cursor, and wakes the poller. It can
optionally restore ignored documents and retry failed documents; it never
deletes imported receipts or user mappings.
