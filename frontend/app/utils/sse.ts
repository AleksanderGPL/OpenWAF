export class EventStreamError extends Error {
  status: number

  constructor(message: string, status = 0) {
    super(message)
    this.name = 'EventStreamError'
    this.status = status
  }
}

export interface SseFrame {
  id: string
  event: string
  data: string
}

export function parseSseFrame(frame: string): SseFrame | null {
  let id = ''
  let event = ''
  const data: string[] = []
  for (const line of frame.split(/\r?\n/)) {
    if (!line || line.startsWith(':')) continue
    const separator = line.indexOf(':')
    const field = separator === -1 ? line : line.slice(0, separator)
    let value = separator === -1 ? '' : line.slice(separator + 1)
    if (value.startsWith(' ')) value = value.slice(1)
    if (field === 'id') id = value
    else if (field === 'event') event = value
    else if (field === 'data') data.push(value)
  }
  if (!data.length) return null
  return { id, event, data: data.join('\n') }
}

export async function readSseStream(response: Response, onFrame: (frame: SseFrame) => void) {
  if (!response.ok) {
    const body = await response.json().catch(() => null) as { message?: string } | null
    throw new EventStreamError(body?.message || `Request failed (${response.status})`, response.status)
  }
  if (!response.headers.get('content-type')?.includes('text/event-stream') || !response.body) {
    throw new EventStreamError('The server did not return an event stream')
  }
  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let pending = ''
  try {
    while (true) {
      const { value, done } = await reader.read()
      pending += done ? decoder.decode() : decoder.decode(value, { stream: true })
      let boundary: RegExpExecArray | null
      while ((boundary = /\r?\n\r?\n/.exec(pending))) {
        const frame = parseSseFrame(pending.slice(0, boundary.index))
        pending = pending.slice(boundary.index + boundary[0].length)
        if (frame) onFrame(frame)
      }
      if (done) break
    }
  } finally {
    await reader.cancel().catch(() => {})
    reader.releaseLock()
  }
}
