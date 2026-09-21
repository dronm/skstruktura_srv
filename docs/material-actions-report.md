# Material actions report API

`GET /api/reports/material-actions` returns one hierarchy level at a time. The
timestamp range is inclusive at both boundaries. RFC 3339 offsets identify the
exact instants, so the database session and register business time zones do not
change the selected period.

Required query parameters:

- `date_from`, `date_to`: RFC 3339 timestamps, including an offset or `Z`
- `level`: `construction_site`, `material`, or `document`

Optional typed filters:

- `construction_site_ids`: repeated or comma-separated integer values
- `material_ids`: repeated or comma-separated integer values

Hierarchy parameters:

- `material` requires `parent_construction_site_id`
- `document` requires both `parent_construction_site_id` and
  `parent_material_id`

The endpoint also accepts collection `from`, `count`, and `sorter` parameters.
Generic collection `filter` expressions are rejected because the report has
typed filters. Page size is capped at 1000.

Examples:

```text
/api/reports/material-actions?date_from=2026-08-01T00:00:00Z&date_to=2026-08-31T23:59:59Z&level=construction_site&construction_site_ids=1,2
/api/reports/material-actions?date_from=2026-08-01T08:00:00Z&date_to=2026-08-31T18:00:00Z&level=material&parent_construction_site_id=1
/api/reports/material-actions?date_from=2026-08-01T08:00:00Z&date_to=2026-08-31T18:00:00Z&level=document&parent_construction_site_id=1&parent_material_id=10
```

Group rows contain `balance_start`, positive `income`, positive `outcome`, and
`balance_end`. Document rows contain only income/outcome; opening and closing
balances are `null`. `has_children` and `child_count` let the UI decide whether
to render an expand button and request the next level.

The material collection (`GET /api/material`) also exposes `balances`, a JSON
array with the current posted quantity for every active construction site,
including zero balances.
