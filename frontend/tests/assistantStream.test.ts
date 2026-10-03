import { describe, expect, test } from 'bun:test'
import { readAssistantStream } from '../app/utils/assistantStream'
import type { AssistantEvent } from '../app/types/assistant'

function responseFromChunks(chunks: Uint8Array[]) {
  return new Response(new ReadableStream({
    start(controller) {
      for (const chunk of chunks) controller.enqueue(chunk)
      controller.close()
    }
  }), { headers: { 'Content-Type': 'text/event-stream; charset=utf-8' } })
}
const encode = (text: string) => new TextEncoder().encode(text)
const frame = (event: object) => `data: ${JSON.stringify(event)}\n\n`

describe('assistant backend stream', () => {
  test('handles split frames, split UTF-8, heartbeats and authoritative final text', async () => {
    const finalMessage = { id: 'run', conversationId: 'chat', role: 'assistant', content: 'Final answer', status: 'completed', createdAt: '' }
    const wire = encode(': heartbeat\n\n' +
      frame({ type: 'run_started', runId: 'run', conversationId: 'chat' }) +
      frame({ type: 'text_delta', runId: 'run', conversationId: 'chat', delta: 'Żółć 🛡️' }) +
      frame({ type: 'tool_started', runId: 'run', conversationId: 'chat', toolName: 'search_requests' }) +
      frame({ type: 'tool_finished', runId: 'run', conversationId: 'chat', toolName: 'search_requests' }) +
      frame({ type: 'run_completed', runId: 'run', conversationId: 'chat', message: finalMessage }))
    const events: AssistantEvent[] = []
    // One byte per chunk exercises boundaries inside both JSON and Unicode.
    await readAssistantStream(responseFromChunks(Array.from(wire, byte => new Uint8Array([byte]))), event => events.push(event))
    expect(events.map(event => event.type)).toEqual(['run_started', 'text_delta', 'tool_started', 'tool_finished', 'run_completed'])
    expect(events[1]?.delta).toBe('Żółć 🛡️')
    expect(events[4]?.message?.content).toBe('Final answer')
  })

  test('accepts CRLF and multiline data frames', async () => {
    const wire = 'event: run_failed\r\ndata: {"type":"run_failed",\r\ndata: "error":"Timed out"}\r\n\r\n'
    const events: AssistantEvent[] = []
    await readAssistantStream(responseFromChunks([encode(wire)]), event => events.push(event))
    expect(events[0]?.error).toBe('Timed out')
  })

  test('reports interruptions instead of treating a partial answer as completed', async () => {
    const response = responseFromChunks([encode(frame({ type: 'text_delta', delta: 'Partial answer' }))])
    await expect(readAssistantStream(response, () => {})).rejects.toThrow('interrupted')
  })

  test('surfaces backend configuration and authentication errors', async () => {
    for (const [status, message] of [[503, 'Assistant is not configured'], [401, 'Unauthorized']] as const) {
      const response = new Response(JSON.stringify({ message }), { status, headers: { 'Content-Type': 'application/json' } })
      await expect(readAssistantStream(response, () => {})).rejects.toThrow(message)
    }
  })

  test('rejects a successful response that is not an SSE stream', async () => {
    await expect(readAssistantStream(new Response('<html>Fallback page</html>', { headers: { 'Content-Type': 'text/html' } }), () => {})).rejects.toThrow('did not return an assistant stream')
  })

  test('stops reading immediately after a cancelled terminal event', async () => {
    let cancelled = false
    const response = new Response(new ReadableStream({
      start(controller) {
        controller.enqueue(encode(frame({ type: 'run_failed', message: { status: 'cancelled', content: 'Partial answer' } })))
      },
      cancel() { cancelled = true }
    }), { headers: { 'Content-Type': 'text/event-stream' } })
    await readAssistantStream(response, () => {})
    expect(cancelled).toBe(true)
  })
})
