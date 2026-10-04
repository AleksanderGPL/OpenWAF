import type { AgentMessage, AssistantConversation, AssistantEvent, AssistantMessage } from '~/types/assistant'
import { readAssistantStream } from '~/utils/assistantStream'
import { translateApiMessage } from '~/utils/i18n'

export function useInvestigationFollowUp(investigationId: Ref<string>) {
  const auth = useAuthStore()
  const { t } = useI18n()
  const messages = ref<AgentMessage[]>([])
  const prompt = ref('')
  const loading = ref(false)
  const opened = ref(false)
  const error = ref('')
  const status = ref<'ready' | 'submitted' | 'streaming'>('ready')
  const stopping = ref(false)
  const activeTool = ref('')
  const conversationId = ref<string>()
  const busy = computed(() => status.value !== 'ready')
  const canSend = computed(() => opened.value && !loading.value && !busy.value)
  let controller: AbortController | undefined
  let runStarted = false
  let disposed = false
  let pollTimer: ReturnType<typeof setTimeout> | undefined

  function errorText(cause: unknown) {
    return auth.authErrorMessage(cause, cause instanceof Error ? cause.message : t('agent.unreachable'))
  }
  function toMessage(message: AssistantMessage): AgentMessage {
    return { id: message.id, role: message.role, parts: [{ type: 'text', text: message.content }], metadata: { status: message.status } }
  }
  function upsert(message: AssistantMessage) {
    const index = messages.value.findIndex(item => item.id === message.id)
    if (index === -1) messages.value.push(toMessage(message))
    else messages.value[index] = toMessage(message)
  }
  async function loadMessages(id: string, reconcile = false) {
    const result = await $fetch<{ items: AssistantMessage[] }>(`/api/assistant/conversations/${id}/messages`)
    if (disposed || conversationId.value !== id || (!reconcile && status.value !== 'ready')) return
    messages.value = result.items.map(toMessage)
    clearTimeout(pollTimer)
    if (result.items.some(item => item.status === 'running')) {
      pollTimer = setTimeout(() => {
        if (!disposed && conversationId.value === id && status.value === 'ready') {
          void loadMessages(id).catch(cause => { error.value = errorText(cause) })
        }
      }, 2000)
    }
  }
  async function open() {
    if (loading.value || opened.value) return
    loading.value = true
    error.value = ''
    try {
      const conversation = await $fetch<AssistantConversation>(`/api/investigations/${investigationId.value}/follow-up`, { method: 'POST' })
      if (disposed) return
      conversationId.value = conversation.id
      opened.value = true
      await loadMessages(conversation.id)
    } catch (cause) {
      if (!disposed) error.value = errorText(cause)
    } finally {
      if (!disposed) loading.value = false
    }
  }
  async function cancelRun(id: string) {
    await $fetch(`/api/assistant/conversations/${id}/cancel`, { method: 'POST' })
  }
  async function stop() {
    if (!busy.value || stopping.value || !conversationId.value) return
    stopping.value = true
    if (!runStarted) return
    try {
      await cancelRun(conversationId.value)
    } catch (cause) {
      error.value = errorText(cause)
      stopping.value = false
    }
  }
  async function send(question = prompt.value) {
    const content = question.trim()
    const id = conversationId.value
    if (!content || !canSend.value || !id) return
    if (new TextEncoder().encode(content).length > 8000) {
      error.value = t('agent.messageTooLong')
      return
    }
    error.value = ''
    status.value = 'submitted'
    stopping.value = false
    runStarted = false
    const abort = new AbortController()
    controller = abort
    const optimisticId = crypto.randomUUID()
    let accepted = false
    try {
      messages.value.push({ id: optimisticId, role: 'user', parts: [{ type: 'text', text: content }], metadata: { status: 'completed' } })
      prompt.value = ''
      const response = await fetch(`/api/assistant/conversations/${id}/messages`, {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json', Accept: 'text/event-stream' },
        body: JSON.stringify({ content }),
        signal: abort.signal
      })
      accepted = response.ok
      await readAssistantStream(response, (event: AssistantEvent) => {
        if (event.type === 'run_started') {
          accepted = true
          runStarted = true
          status.value = 'streaming'
          if (event.message) upsert(event.message)
          if (stopping.value) void cancelRun(id).catch(cause => { error.value = errorText(cause); stopping.value = false })
        } else if (event.type === 'text_delta') {
          const message = messages.value.find(item => item.id === event.runId)
          if (message) message.parts[0]!.text += event.delta || ''
        } else if (event.type === 'tool_started') {
          activeTool.value = event.toolName || 'workspace'
        } else if (event.type === 'tool_finished') {
          activeTool.value = ''
        } else if (event.type === 'run_completed' || event.type === 'run_failed') {
          if (event.message) upsert(event.message)
          if (event.type === 'run_failed' && event.message?.status !== 'cancelled') {
            error.value = event.message?.status === 'timed_out' ? t('agent.timedOut') : (event.error ? translateApiMessage(event.error) : t('agent.incomplete'))
          }
        }
      })
    } catch (cause) {
      if (!disposed) {
        error.value = errorText(cause)
        if (!accepted && !abort.signal.aborted) {
          messages.value = messages.value.filter(item => item.id !== optimisticId)
          prompt.value = content
        }
      }
    } finally {
      if (!disposed) {
        try { await loadMessages(id, true) } catch (cause) { error.value ||= errorText(cause) }
      }
      controller = undefined
      runStarted = false
      activeTool.value = ''
      stopping.value = false
      status.value = 'ready'
    }
  }

  onBeforeUnmount(() => {
    disposed = true
    clearTimeout(pollTimer)
    controller?.abort()
    if (runStarted && conversationId.value) {
      void fetch(`/api/assistant/conversations/${conversationId.value}/cancel`, { method: 'POST', credentials: 'same-origin', keepalive: true }).catch(() => {})
    }
  })

  return { messages, prompt, loading, opened, error, status, stopping, activeTool, busy, canSend, open, send, stop }
}
