# Autonomous investigations API

An investigation is the single operator-facing resource. It contains why analysis started, execution state, live progress, the resulting assessment and evidence, and follow-up chat. Its ID stays the same across retries and updated results. All endpoints require an admin session cookie.

The backend uses Eino with the existing `AI_KEY`, `AI_MODEL`, and optional `ASSISTANT_BASE_URL`. Provider/model configuration and raw tool arguments/results are not returned by this API. The agent reads telemetry and cannot block traffic or change rules.

## Endpoints

| Method | Path | Purpose |
|---|---|---|
| GET | `/api/investigations` | Paginated investigation cards, including active investigations and completed results |
| GET | `/api/investigations/summary` | Total, unread, severity and status counts |
| GET | `/api/investigations/events` | SSE updates across investigations |
| GET | `/api/investigations/:id` | Trigger, execution state, result, retained evidence and recent progress |
| GET | `/api/investigations/:id/events` | Replayable live progress for this investigation |
| PATCH | `/api/investigations/:id` | Mark read/unread, acknowledge, resolve, dismiss or reopen |
| POST | `/api/investigations/:id/retry` | Queue another execution; returns 202 with the same investigation ID |
| POST | `/api/investigations/:id/cancel` | Cancel queued/running execution; idempotent |
| POST | `/api/investigations/:id/follow-up` | Create/get your linked assistant conversation |
| GET | `/api/anomaly-settings` | Effective global/service anomaly settings |
| PUT | `/api/anomaly-settings` | Replace complete global/service settings |
| DELETE | `/api/anomaly-settings?serviceId=N` | Restore service inheritance |

The previous separate incident and finding endpoints are removed. Execution records remain internal for retries and recovery; clients use the investigation ID for every action.

## List and detail views

The list returns `{items, total, page, limit, lastEventId}`. Defaults: page 1, limit 25; maximum limit 100. Filters: `serviceId`, `state`, `status`, `severity`, `assessment`, `unread=true/false`, `from`, `to`. Dates are RFC3339, from inclusive/to exclusive, applied to lastSeen.

Each card contains:

- `id`, `serviceId`, optional `ip`, `firstSeen`, `lastSeen`.
- `title`, `summary`, and, when a result exists, `severity` and `assessment`.
- `state`: `open`, `acknowledged`, `resolved`, `dismissed`.
- `status`: `detected`, `queued`, `running`, `completed`, `failed`, `cancelled`.
- `trigger`: explanation, detector, observed values, thresholds/baseline, time window, evidence IDs and coverage.
- `read`, `resultAvailable`, `resultVersion`.
- Execution timestamps, `cancellationRequested`, and a safe error when applicable.

`detected` means suspicious activity was recorded but no execution has been admitted yet, for example because the hourly allowance was reached. Queued work can wait while AI is unavailable. Title/summary show the trigger before a result exists, then the latest published assessment.

Detail returns the same card fields plus `result`, `events`, `lastEventId`, and `eventsTruncated`. `result` is null until analysis publishes an assessment. A result contains title, summary, severity, assessment, explanation, possible attack patterns, recommendations, limitations, its trigger snapshot, publication timestamp and evidence.

Severity: `info`, `low`, `medium`, `high`, `critical`. Assessment: `likely_malicious`, `likely_benign`, `inconclusive`. These are judgments supported by telemetry, not proof that an attack succeeded. Cited request IDs must have been returned by telemetry tools and still exist at publication; this checks references rather than proving every narrative interpretation. An empty evidence list requires an inconclusive assessment with explicit limitations.

The root trigger describes the latest execution, or the initial detection if no execution exists. The result retains its own trigger/window. A previous result remains available during a retry, cancellation or failure; its publication timestamp and version identify which assessment is displayed. Failed/cancelled execution does not publish a new result.

`result.evidence` contains `{request, available}` entries. Each request is a retained snapshot; available indicates whether the original log can still be fetched. Snapshots bound paths to 1024 bytes, reasons to 512 bytes, and rule details to five matches with bounded messages/tags/variables. Normal request-log retention does not remove published snapshots. Investigation progress/results currently remain until database removal.

Detail includes the latest 200 progress events in chronological order. `eventsTruncated` indicates older events exist; use the stream with `after=0` to replay them. The detail and its event cursor are read in one transaction.

## Operator state and unread counts

```json
{"state":"acknowledged","read":true,"resultVersion":1}
```

State is shared across operators. Read state is per operator. Investigations start unread; marking the current result version read also works before publication using version 0. Every newly published result increments `resultVersion` and makes the investigation unread again. Sending the version displayed by the frontend prevents a newly published result from being marked read accidentally. `{"read":false}` marks the investigation unread.

`GET /api/investigations/summary` returns `{total, unread, bySeverity, byStatus, lastEventId}`. Severity counts describe the latest results; status counts describe current execution. No-result investigations contribute to total/status/unread counts without receiving an invented severity.

## SSE

Use EventSource with the admin cookie, or streaming fetch. SSE events have numeric IDs. Reconnect using `Last-Event-ID` or `?after=N`.

- The global stream defaults to subscribing after the current cursor. Pass the list/summary's `lastEventId` to cover updates between fetching a snapshot and subscribing.
- The per-investigation stream defaults to replaying that investigation from the beginning, including earlier executions. `after=0` explicitly requests historical replay.

```text
id: 42
event: activity.started
data: {"id":42,"investigationId":"...","type":"activity.started","activityId":"...","attempt":1,"provisional":false,"activity":"Searching relevant requests","createdAt":"..."}

id: 43
event: explanation.delta
data: {"id":43,"investigationId":"...","type":"explanation.delta","attempt":1,"provisional":true,"delta":"I am comparing this source with the previous window.","createdAt":"..."}
```

Event types:

- `investigation.created`, `investigation.updated`.
- `investigation.queued`, `investigation.started`.
- `activity.started`, `activity.completed`, `explanation.delta`.
- `investigation.result_published`, `investigation.completed`.
- `investigation.failed`, `investigation.cancellation_requested`, `investigation.cancelled`.

Every event references the same `investigationId`. Fetch `/api/investigations/:id` after result publication or state changes. `activityId` correlates activity updates without exposing tool payloads. `attempt` identifies automatic retries within an execution; `investigation.started` starts a new progress sequence. Explanation deltas are provisional operator-facing narrative, not private model reasoning. Persisted event IDs are suitable for deduplication.

Heartbeats occur every 15 seconds. Streams close after 15 minutes and may reconnect; session expiry/sign-out also closes streaming. Disconnecting does not cancel execution. Cancellation retains the trigger, recorded progress and any previously published result. Text deltas are buffered into small durable events instead of writing a row per token.

## Follow-up chat

After execution finishes or is cancelled:

1. POST `/api/investigations/:id/follow-up` without a body.
2. Receive your assistant conversation including its `id`. First creation returns 201; later calls return the same conversation with 200.
3. POST `{"content":"Could this activity affect another service?"}` to `/api/assistant/conversations/:id/messages` and consume its existing SSE response.

The same conversation remains linked across investigation retries. Calling follow-up refreshes its context with the latest trigger, result, retained evidence and recent progress while preserving chat history. Each operator has a private conversation. Investigation context remains available across later chat turns. Follow-up also works when analysis failed, was cancelled, or has not yet been admitted. Interactive cancellation/disconnect behavior follows the existing assistant API. Follow-up does not automatically modify or republish the investigation result.

## Detection and limits

Aggregation reads new persisted request IDs in batches of 250, normally every second, updating minute buckets and its cursor atomically. Evaluation runs every 15 seconds using completed buckets. First startup begins at the latest request; historical logs do not generate a flood of investigations. Restarts continue from the saved cursor. Relative detection warms up again after a collection gap exceeding two minutes or a change in collection failures.

| Detector | Default trigger |
|---|---|
| Repeated blocks | At least 10 blocked requests from one IP/service in five minutes |
| Security matches | At least 10 suspicious requests from one IP/service in five minutes, including allowed requests with rule matches |
| Path scanning | At least 30 requests and 20 distinct paths in five minutes, plus three suspicious requests or three 400/404/405 responses |
| Upstream errors | At least 20 allowed requests with upstream errors, representing at least 50% of service traffic in five minutes |
| Traffic surge | At least 100 requests in the latest completed minute and four times the preceding 30-minute average |

The surge baseline becomes available after approximately 31 minutes of continuous collection. Fixed thresholds operate during warmup; trigger coverage explains partial windows. Distinct paths are bounded to 100 hashes per source/minute and 500 per evaluated group. Initial evidence IDs are a bounded sample. Unknown-service requests are excluded. Evaluation waits for the cursor to catch up; more than 50,000 source/minute buckets in the range defers evaluation and records an operational error in server logs. This version uses the existing SQLite connection.

Related activity shares one investigation by detector family, service and source IP. One execution is queued/running per investigation. Default cooldown: 10 minutes. A measurement doubling can bypass the cooldown after one minute. Resolved/dismissed investigations suppress related activity while it remains within the correlation window; activity after that window creates a new investigation. A manual retry reopens a resolved/dismissed investigation. Stored observations update as activity continues; each execution preserves its trigger snapshot.

`GET /api/anomaly-settings[?serviceId=N]` returns `{policy, inherited, aiEnabled}`. Service overrides replace the complete policy; deleting one restores global inheritance.

```json
{
  "enabled": true,
  "blockedThreshold": 10,
  "suspiciousThreshold": 10,
  "scanRequests": 30,
  "scanPaths": 20,
  "errorThreshold": 20,
  "errorPercent": 50,
  "surgeMinimum": 100,
  "surgeMultiplier": 4,
  "cooldownMinutes": 10,
  "hourlyRunLimit": 20
}
```

Disabling detection does not cancel queued work. One autonomous worker preserves interactive assistant capacity. Attempts are bounded to three minutes, eight model iterations and 24 tool calls, with two attempts maximum. Failures retry after one minute; crashes recover after a five-minute lease expires using a fresh attempt. The queue is capped at 100 active executions. The global hourly allowance limits admission and started attempts; service overrides additionally limit admitted executions for that service. These are execution limits, not a currency/token cap.

Telemetry tools include minute-level traffic, security matches including detection-mode traffic, request search/detail, statistics and blocked sources. Minute traffic queries are limited to two hours.

Existing data from the previous split API is migrated on startup: the former incident ID becomes the investigation ID, its latest finding becomes the embedded result, and read state/follow-up links are preserved. Legacy result records remain stored; the public API no longer exposes separate incident, finding or execution IDs.


## Live AI verification

With `AI_KEY` and `AI_MODEL` configured in the root `.env` or environment, run:

```sh
go test -tags dev ./internal/investigation -run '^TestLiveInvestigation$' -count=1 -v -timeout=15m -args -live-investigation
```

This calls the configured AI provider using a temporary database and an isolated HTTP server. It seeds 60 requests: 30 blocked SQL injection probes, 10 allowed probes with security matches, and 20 benign requests. It checks detection, streamed activity, evidence publication, list/detail/read APIs, tool-assisted follow-up, SSE replay, and cancellation preserving the previous result. Regular test runs skip provider calls.

Invalid investigation tool arguments produce corrective feedback for the agent. Cancellation, timeouts, and the tool-call budget still stop execution.
