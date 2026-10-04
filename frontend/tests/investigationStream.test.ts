import { describe, expect, test } from 'bun:test'
import { parseSseFrame, readSseStream } from '../app/utils/sse'
import { groupInvestigationProgress, mergeInvestigationEvents } from '../app/utils/investigations'
import type { InvestigationEvent } from '../app/types/investigation'

function responseFromChunks(chunks: Uint8Array[], status = 200) {
  return new Response(new ReadableStream({
    start(controller) {
      for (const chunk of chunks) controller.enqueue(chunk)
      controller.close()
    }
  }), { status, headers: { 'Content-Type': 'text/event-stream; charset=utf-8' } })
}

const encode = (text: string) => new TextEncoder().encode(text)

function event(partial: Partial<InvestigationEvent> & Pick<InvestigationEvent, 'id' | 'type'>): InvestigationEvent {
  return {
    investigationId: 'inv',
    attempt: 1,
    provisional: false,
    createdAt: '2026-10-04T00:00:00Z',
    ...partial
  }
}

describe('investigation event stream', () => {
  test('parses ids, named events, heartbeats and split UTF-8 frames', async () => {
    const payload = { id: 42, investigationId: 'inv', type: 'explanation.delta', attempt: 1, provisional: true, delta: 'Żółć 🛡️', createdAt: '2026-10-04T00:00:00Z' }
    const wire = encode(': heartbeat\n\n' + `id: 42\nevent: explanation.delta\ndata: ${JSON.stringify(payload)}\n\n`)
    const frames: { id: string, event: string, data: string }[] = []
    await readSseStream(responseFromChunks(Array.from(wire, byte => new Uint8Array([byte]))), frame => frames.push(frame))
    expect(frames).toHaveLength(1)
    expect(frames[0]).toMatchObject({ id: '42', event: 'explanation.delta' })
    expect(JSON.parse(frames[0]!.data).delta).toBe('Żółć 🛡️')
  })

  test('accepts CRLF and multiline data', () => {
    const frame = parseSseFrame('id: 7\r\nevent: investigation.failed\r\ndata: {"error":\r\ndata: "timed out"}\r\n')
    expect(frame).toEqual({ id: '7', event: 'investigation.failed', data: '{"error":\n"timed out"}' })
  })

  test('ignores comment-only frames', () => {
    expect(parseSseFrame(': heartbeat')).toBeNull()
  })

  test('returns when the stream closes after a full frame', async () => {
    const frames: string[] = []
    await readSseStream(responseFromChunks([encode('id: 1\nevent: investigation.completed\ndata: {"id":1}\n\n')]), frame => frames.push(frame.event))
    expect(frames).toEqual(['investigation.completed'])
  })
})

describe('investigation progress', () => {
  test('merges adjacent explanation deltas from the same attempt', () => {
    const blocks = groupInvestigationProgress([
      event({ id: 1, type: 'activity.started', activity: 'Searching relevant requests' }),
      event({ id: 2, type: 'explanation.delta', provisional: true, delta: 'Comparing ' }),
      event({ id: 3, type: 'explanation.delta', provisional: true, delta: 'this source.' }),
      event({ id: 4, type: 'investigation.started', attempt: 2 }),
      event({ id: 5, type: 'explanation.delta', attempt: 2, provisional: true, delta: 'Second look.' })
    ])
    expect(blocks.map(block => block.kind)).toEqual(['event', 'explanation', 'event', 'explanation'])
    const narrative = blocks[1]
    expect(narrative?.kind).toBe('explanation')
    if (narrative?.kind === 'explanation') {
      expect(narrative.text).toBe('Comparing this source.')
      expect(narrative.provisional).toBe(true)
      expect(narrative.id).toBe(3)
    }
  })

  test('deduplicates events by id and keeps chronological order', () => {
    const merged = mergeInvestigationEvents(
      [event({ id: 2, type: 'activity.completed' }), event({ id: 1, type: 'activity.started' })],
      [event({ id: 2, type: 'activity.completed', activity: 'done' }), event({ id: 3, type: 'investigation.completed' })]
    )
    expect(merged.map(item => item.id)).toEqual([1, 2, 3])
    expect(merged[1]?.activity).toBe('done')
  })
})
