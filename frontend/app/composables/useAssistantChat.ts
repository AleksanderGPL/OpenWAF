import type { AgentMessage, AssistantConversation, AssistantEvent, AssistantMessage } from '~/types/assistant'
import { readAssistantStream } from '../utils/assistantStream'

export function useAssistantChat() {
  const auth = useAuthStore()
  const isAdmin = computed(() => auth.user?.role === 'admin')
  const configuration = ref<{ enabled: boolean} | null>(null)
  const conversations = ref<AssistantConversation[]>([])
  const conversationId = ref<string>()
  const messages = ref<AgentMessage[]>([])
  const prompt = ref('')
  const loading = ref(true)
  const error = ref('')
  const status = ref<'ready' | 'submitted' | 'streaming'>('ready')
  const stopping = ref(false)
  const activeTool = ref('')
  const externalRun = computed(() => status.value === 'ready' && messages.value.some(item => item.metadata.status === 'running'))
  const busy = computed(() => status.value !== 'ready' || externalRun.value)
  const canSend = computed(() => isAdmin.value && configuration.value?.enabled && !loading.value && !busy.value)
  let controller: AbortController | undefined
  let runStarted = false
  let disposed = false
  let loadVersion = 0
  let pollTimer: ReturnType<typeof setTimeout> | undefined

  function errorText(cause: unknown) {
    return auth.authErrorMessage(cause, cause instanceof Error ? cause.message : 'Could not reach the assistant. Please try again.')
  }
  function toMessage(message: AssistantMessage): AgentMessage {
    return { id: message.id, role: message.role, parts: [{ type: 'text', text: message.content }], metadata: { status: message.status } }
  }
  function upsert(message: AssistantMessage) {
    const index = messages.value.findIndex(item => item.id === message.id)
    if (index === -1) messages.value.push(toMessage(message))
    else messages.value[index] = toMessage(message)
  }
  async function refreshConversations() {
    const result = await $fetch<{ items: AssistantConversation[] }>('/api/assistant/conversations')
    if (!disposed) conversations.value = result.items
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
  async function selectConversation(id: string) {
    if (loading.value || status.value !== 'ready') return
    clearTimeout(pollTimer)
    conversationId.value = id
    messages.value = []
    prompt.value = ''
    error.value = ''
    loading.value = true
    try {
      await loadMessages(id)
    } catch (cause) {
      error.value = errorText(cause)
    } finally {
      loading.value = false
    }
  }
  async function initialize() {
    const version = ++loadVersion
    loading.value = true
    error.value = ''
    if (!isAdmin.value) { loading.value = false; return }
    try {
      const [config] = await Promise.all([
        $fetch<{ enabled: boolean }>('/api/assistant/status'),
        refreshConversations()
      ])
      if (disposed || version !== loadVersion) return
      configuration.value = config
      conversationId.value ||= conversations.value[0]?.id
      if (conversationId.value) await loadMessages(conversationId.value)
    } catch (cause) {
      if (!disposed) error.value = errorText(cause)
    } finally {
      if (!disposed && version === loadVersion) loading.value = false
    }
  }
  async function newConversation() {
    if (!isAdmin.value || loading.value || busy.value) return
    loading.value = true
    error.value = ''
    try {
      const conversation = await $fetch<AssistantConversation>('/api/assistant/conversations', {
        method: 'POST', body: { title: 'New conversation' }
      })
      if (disposed) return
      clearTimeout(pollTimer)
      conversations.value.unshift(conversation)
      conversationId.value = conversation.id
      messages.value = []
      prompt.value = ''
    } catch (cause) {
      if (!disposed) error.value = errorText(cause)
    } finally {
      if (!disposed) loading.value = false
    }
  }
  async function deleteConversation() {
    const id = conversationId.value
    if (!id || !isAdmin.value || loading.value || busy.value) return false
    loading.value = true
    error.value = ''
    clearTimeout(pollTimer)
    try {
      await $fetch(`/api/assistant/conversations/${id}`, { method: 'DELETE' })
      if (disposed) return false
      conversations.value = conversations.value.filter(item => item.id !== id)
      conversationId.value = conversations.value[0]?.id
      messages.value = []
      prompt.value = ''
      if (conversationId.value) {
        try { await loadMessages(conversationId.value) } catch (cause) { error.value = errorText(cause) }
      }
      return true
    } catch (cause) {
      if (!disposed) error.value = errorText(cause)
      return false
    } finally {
      if (!disposed) loading.value = false
    }
  }
  async function cancelRun(id: string) {
    await $fetch(`/api/assistant/conversations/${id}/cancel`, { method: 'POST' })
  }
  async function stop() {
    if (!busy.value || stopping.value) return
    stopping.value = true
    if (!conversationId.value || (!runStarted && !externalRun.value)) return
    try {
      await cancelRun(conversationId.value)
      if (externalRun.value) await loadMessages(conversationId.value)
    } catch (cause) {
      error.value = errorText(cause)
      stopping.value = false
    }
    if (status.value === 'ready') stopping.value = false
  }
  async function send(question = prompt.value) {
    const content = question.trim()
    if (!content || !canSend.value) return
    if (new TextEncoder().encode(content).length > 8000) {
      error.value = 'Your message must be at most 8000 bytes. Please shorten it.'
      return
    }
    error.value = ''
    status.value = 'submitted'
    stopping.value = false
    runStarted = false
    const abort = new AbortController()
    controller = abort
    const optimisticId = crypto.randomUUID()
    let id = conversationId.value
    let accepted = false
    try {
      if (!id) {
        const conversation = await $fetch<AssistantConversation>('/api/assistant/conversations', {
          method: 'POST', body: { title: 'Traffic investigation' }, signal: abort.signal
        })
        if (disposed) return
        id = conversation.id
        conversationId.value = id
        conversations.value.unshift(conversation)
      }
      if (stopping.value || disposed) return
      messages.value.push({ id: optimisticId, role: 'user', parts: [{ type: 'text', text: content }], metadata: { status: 'completed' } })
      prompt.value = ''
      const response = await fetch(`/api/assistant/conversations/${id}/messages`, {
        method: 'POST', credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json', Accept: 'text/event-stream' },
        body: JSON.stringify({ content }), signal: abort.signal
      })
      accepted = response.ok
      await readAssistantStream(response, (event: AssistantEvent) => {
        if (event.type === 'run_started') {
          accepted = true
          runStarted = true
          status.value = 'streaming'
          if (event.message) upsert(event.message)
          if (stopping.value) void cancelRun(id!).catch(cause => { error.value = errorText(cause); stopping.value = false })
        } else if (event.type === 'text_delta') {
          const message = messages.value.find(item => item.id === event.runId)
          if (message) message.parts[0]!.text += event.delta || ''
        } else if (event.type === 'tool_started') {
          activeTool.value = event.toolName?.replace(/_/g, ' ') || 'workspace data'
        } else if (event.type === 'tool_finished') {
          activeTool.value = ''
        } else if (event.type === 'run_completed' || event.type === 'run_failed') {
          if (event.message) upsert(event.message)
          if (event.type === 'run_failed' && event.message?.status !== 'cancelled') {
            error.value = event.message?.status === 'timed_out' ? 'The assistant timed out. Try a narrower question.' : (event.error || 'The assistant could not complete its response.')
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
      // Replace optimistic IDs and partial text with the persisted server history.
      if (id && !disposed) {
        try { await loadMessages(id, true); await refreshConversations() } catch (cause) {
          error.value ||= errorText(cause)
        }
      }
      controller = undefined
      runStarted = false
      activeTool.value = ''
      stopping.value = false
      status.value = 'ready'
    }
  }

  onMounted(initialize)
  onBeforeUnmount(() => {
    disposed = true
    clearTimeout(pollTimer)
    controller?.abort()
    if (runStarted && conversationId.value) {
      void fetch(`/api/assistant/conversations/${conversationId.value}/cancel`, { method: 'POST', credentials: 'same-origin', keepalive: true }).catch(() => {})
    }
  })
  return { configuration, conversations, conversationId, messages, prompt, loading, error, status, stopping, activeTool, isAdmin, busy, canSend, initialize, selectConversation, newConversation, deleteConversation, send, stop }
}
