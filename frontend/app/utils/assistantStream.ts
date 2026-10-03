import type { AssistantEvent } from '../types/assistant'

// The backend sends its own SSE events, rather than the AI SDK stream protocol.
export async function readAssistantStream(response: Response, onEvent: (event: AssistantEvent) => void) {
  if (!response.ok) {
    const body = await response.json().catch(() => null)
    throw new Error(body?.message || `Assistant request failed (${response.status})`)
  }
  if (!response.headers.get('content-type')?.includes('text/event-stream') || !response.body) {
    throw new Error('The server did not return an assistant stream')
  }
  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let pending = ''
  let terminal = false
  try {
    while (!terminal) {
      const { value, done } = await reader.read()
      pending += done ? decoder.decode() : decoder.decode(value, { stream: true })
      let boundary: RegExpExecArray | null
      while ((boundary = /\r?\n\r?\n/.exec(pending))) {
        const frame = pending.slice(0, boundary.index)
        pending = pending.slice(boundary.index + boundary[0].length)
        const data = frame.split(/\r?\n/).filter(line => line.startsWith('data:'))
          .map(line => line.slice(5).replace(/^ /, '')).join('\n')
        if (!data) continue
        const event = JSON.parse(data) as AssistantEvent
        onEvent(event)
        terminal = event.type === 'run_completed' || event.type === 'run_failed'
        if (terminal) break
      }
      if (done) break
    }
    if (!terminal) throw new Error('The response was interrupted. Reload the conversation to recover its saved messages.')
  } finally {
    await reader.cancel().catch(() => {})
    reader.releaseLock()
  }
}
