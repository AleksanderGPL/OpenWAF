# Assistant API

The backend uses Eino ADK and OpenRouter (or another OpenAI-compatible chat completions provider) that supports tool calling. Configure these environment variables before starting OpenWAF:

```sh
export AI_KEY='your-provider-key'
export AI_MODEL='provider/model-id'
# Optional; defaults to OpenRouter. Override for another compatible provider.
export ASSISTANT_BASE_URL='https://openrouter.ai/api/v1'
```

Without an API key, OpenWAF starts with the assistant disabled. Chat creation and history remain available; sending returns HTTP 503. With a key, a model name is required. Keys are never returned to clients.

All routes require the existing admin session cookie. Conversations belong to their creator; another admin receives 404 when accessing them. Send JSON with `Content-Type: application/json` for create and send requests.

| Method | Path | Request / response |
|---|---|---|
| GET | `/api/assistant/status` | `{ "enabled": true }` |
| POST | `/api/assistant/conversations` | `{ "title": "Investigate traffic spike" }` → 201 conversation object; title optional |
| GET | `/api/assistant/conversations` | `{ "items": [...] }`, latest 100 |
| GET | `/api/assistant/conversations/:id/messages` | `{ "items": [...] }`, oldest first |
| POST | `/api/assistant/conversations/:id/messages` | `{ "content": "Investigate blocked traffic in the last hour" }` → SSE |
| POST | `/api/assistant/conversations/:id/cancel` | `{ "success": true }`; idempotent |
| DELETE | `/api/assistant/conversations/:id` | `{ "success": true }`; deletes messages too; 409 while active |

Conversation fields: `id`, `title`, `createdAt`, `updatedAt`.
Message fields: `id`, `conversationId`, `role` (`user` or `assistant`), `content`, `status`, `createdAt`.
Assistant message statuses: `running`, `completed`, `failed`, `cancelled`, `timed_out`, `interrupted` (process restarted during the run). User messages have status `completed`.

## Streaming

Use streaming `fetch` for the POST response. Native `EventSource` cannot send the JSON message body. Include credentials when calling through a different frontend origin; production is same origin.

```text
event: run_started
data: {"type":"run_started","runId":"...","conversationId":"...","message":{...}}

event: text_delta
data: {"type":"text_delta","runId":"...","conversationId":"...","delta":"The traffic..."}

event: tool_started
data: {"type":"tool_started","runId":"...","conversationId":"...","toolCallId":"...","toolName":"search_requests","arguments":"{...}"}

event: tool_finished
data: {"type":"tool_finished","runId":"...","conversationId":"...","toolCallId":"...","toolName":"search_requests","result":"{...}"}

event: run_completed
data: {"type":"run_completed","runId":"...","conversationId":"...","message":{...}}
```

`runId` equals the persisted assistant message ID. Append `delta` to the current answer. Associate tools using `toolCallId`; arguments and result are JSON strings. Tool errors may end the run before a `tool_finished` event. A terminal `run_failed` includes `message` with its status and a safe `error` string. Provider error details are logged server-side.

The final message is authoritative and saved before the terminal event. Render its `content` to reconcile streamed text. Heartbeats are SSE comments every 15 seconds. An example consumer:

```ts
async function sendMessage(id: string, content: string, onEvent: (event: any) => void) {
  const response = await fetch(`/api/assistant/conversations/${id}/messages`, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', Accept: 'text/event-stream' },
    body: JSON.stringify({ content }),
  })
  if (!response.ok) throw new Error((await response.json()).message)
  if (!response.body) throw new Error('Missing response stream')
  const reader = response.body.pipeThrough(new TextDecoderStream()).getReader()
  let pending = ''
  let terminal = false
  try {
    while (true) {
      const { value, done } = await reader.read()
      if (done) break
      pending += value
      let boundary: number
      while ((boundary = pending.indexOf('\n\n')) !== -1) {
        const frame = pending.slice(0, boundary)
        pending = pending.slice(boundary + 2)
        const data = frame.split('\n').filter(line => line.startsWith('data:'))
          .map(line => line.slice(5).trimStart()).join('\n')
        if (!data) continue // heartbeat
        const event = JSON.parse(data)
        terminal ||= event.type === 'run_completed' || event.type === 'run_failed'
        onEvent(event)
      }
    }
    if (!terminal) throw new Error('Stream interrupted; reload conversation messages')
  } finally {
    await reader.cancel()
    reader.releaseLock()
  }
}
```

Before the SSE stream starts, errors use the standard `{ "message": "..." }` JSON response: 400 invalid content, 401 unauthenticated, 403 non-admin, 404 missing/other operator's chat, 409 active conversation, 415 wrong content type, 429 global concurrency limit, 503 unconfigured assistant.

## Lifecycle and limits

One active run per conversation, four across the process. Each run has a three-minute deadline and at most eight model iterations. Queries allow at most 30 days and 100 log rows per page; tools are read-only. Output/context size is bounded. Recent complete turns are retained in model context (up to ten turns and approximately 256 KiB). Full visible chat history persists in SQLite.

Cancelling or disconnecting stops execution; disconnect detection happens on a stream write/heartbeat failure. Cancellation persists any answer text received so far. Failed turns are excluded from later model context. This API has no event replay: after reconnecting, fetch message history to recover the saved result. Runs are not durable across process restarts. Background scheduling and anomaly detection are future additions; telemetry tools and the Eino execution service are shared foundations.

Keep reverse proxy buffering disabled for these endpoints. No WAF rule changes or blocking actions are exposed to the model. Telemetry is treated as untrusted evidence; findings should cite request IDs and time windows.

Linked follow-up conversations for autonomous investigations use the same message and cancellation endpoints. See [investigations-api.md](investigations-api.md). Tools also support minute-level traffic and security-match summaries including allowed requests in detection mode. Runs are limited to 24 tool calls.
