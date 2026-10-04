# Country request statistics

`GET /api/stats/countries` requires an admin session cookie and returns
`Cache-Control: no-store`. It aggregates retained request logs by country for a
future map display. Country lookup does not happen during this query; it uses the
country codes stored when requests were recorded.

Query parameters:

| Parameter | Values | Default |
| --- | --- | --- |
| `mode` | `all` or `blocked` | `all` |
| `range` | `24h`, `7d`, or `30d` | `24h` |
| `from`, `to` | RFC 3339 timestamps, supplied together without `range` | — |
| `serviceId` | Positive service ID | All services |

The time window includes `from` and excludes `to`. Explicit windows must be
positive and no longer than 366 days. Invalid parameters return HTTP 400;
unauthenticated requests return 401 and non-admin sessions return 403.

For example, `GET /api/stats/countries?mode=blocked&range=7d` returns:

```json
{
  "from": "2026-09-27T12:00:00Z",
  "to": "2026-10-04T12:00:00Z",
  "mode": "blocked",
  "totalRequests": 125,
  "items": [
    { "countryCode": "US", "requests": 80 },
    { "countryCode": "PL", "requests": 40 },
    { "countryCode": null, "requests": 5 }
  ]
}
```

Each count includes only requests matching the selected mode, time window, and
service. All country groups with matching requests are returned, with no top-N
limit. Codes are uppercase ISO 3166-1 alpha-2. Null, empty, and whitespace-only
codes form one unknown-country bucket (`countryCode: null`); this bucket counts
toward `totalRequests` but should not be plotted on a map. Items are ordered by
request count descending, then country code ascending, with the unknown bucket
last in a tie. An empty result has `totalRequests: 0` and `items: []`.
