import type { InvestigationEvent, InvestigationSummary } from '~/types/investigation'
import { EventStreamError, readSseStream } from '~/utils/sse'

const listEventTypes = new Set([
  'investigation.created',
  'investigation.updated',
  'investigation.queued',
  'investigation.started',
  'investigation.result_published',
  'investigation.completed',
  'investigation.failed',
  'investigation.cancellation_requested',
  'investigation.cancelled'
])

let connection: AbortController | undefined
let cursor = 0
let summaryTimer: ReturnType<typeof setTimeout> | undefined

function sleep(ms: number, signal: AbortSignal) {
  return new Promise<void>((resolve) => {
    const timer = setTimeout(resolve, ms)
    signal.addEventListener('abort', () => {
      clearTimeout(timer)
      resolve()
    }, { once: true })
  })
}

export function useInvestigationFeed() {
  const auth = useAuthStore()
  const summary = useState<InvestigationSummary | null>('investigations-summary', () => null)
  const revision = useState('investigations-revision', () => 0)
  const connectionState = useState<'connecting' | 'live' | 'offline'>('investigations-connection', () => 'connecting')
  const unread = computed(() => summary.value?.unread ?? 0)

  async function refreshSummary(prime = false) {
    if (auth.user?.role !== 'admin') return
    const next = await $fetch<InvestigationSummary>('/api/investigations/summary')
    summary.value = next
    if (prime && next.lastEventId > cursor) cursor = next.lastEventId
  }

  function start() {
    if (connection || auth.user?.role !== 'admin' || !import.meta.client) return
    const abort = new AbortController()
    connection = abort
    void run(abort.signal)
  }

  function stop() {
    connection?.abort()
    connection = undefined
    clearTimeout(summaryTimer)
    connectionState.value = 'offline'
  }

  function noteListEvent(id: number) {
    if (id > cursor) cursor = id
    revision.value += 1
    clearTimeout(summaryTimer)
    summaryTimer = setTimeout(() => { void refreshSummary().catch(() => {}) }, 400)
  }

  async function run(signal: AbortSignal) {
    try {
      await refreshSummary(true)
    } catch {
      // The investigations page surfaces its own load error. The badge can wait for the next event.
    }
    while (!signal.aborted) {
      try {
        connectionState.value = 'connecting'
        const response = await fetch(`/api/investigations/events?after=${cursor}`, {
          headers: { Accept: 'text/event-stream' },
          credentials: 'same-origin',
          signal
        })
        if (response.ok) connectionState.value = 'live'
        await readSseStream(response, (frame) => {
          let event: InvestigationEvent
          try {
            event = JSON.parse(frame.data) as InvestigationEvent
          } catch {
            return
          }
          if (listEventTypes.has(event.type)) noteListEvent(event.id)
          else if (event.id > cursor) cursor = event.id
        })
      } catch (error) {
        if (signal.aborted) return
        connectionState.value = 'offline'
        if (error instanceof EventStreamError && error.status >= 400 && error.status < 500) return
      }
      if (signal.aborted) return
      await sleep(1000, signal)
    }
  }

  return { summary, revision, unread, connectionState, refreshSummary, start, stop }
}
