import { afterEach, beforeEach, expect, test } from 'bun:test'
import { computed, ref } from 'vue'
import { useAssistantChat } from '../app/composables/useAssistantChat'

const globals = globalThis as unknown as Record<string, unknown>
const names = ['ref', 'computed', 'useAuthStore', '$fetch', 'onMounted', 'onBeforeUnmount', 'useI18n']
const messages: Record<string, string> = {
  'agent.newConversation': 'New conversation',
  'agent.investigationTitle': 'Traffic investigation',
  'agent.unreachable': 'Could not reach the assistant. Please try again.',
  'agent.messageTooLong': 'Your message must be at most 8000 bytes. Please shorten it.',
  'agent.timedOut': 'The assistant timed out. Try a narrower question.',
  'agent.incomplete': 'The assistant could not complete its response.'
}
const original = new Map(names.map(name => [name, globals[name]]))
let requests: { url: string, options?: { method?: string, body?: unknown } }[]
let respond: (url: string, options?: { method?: string, body?: unknown }) => unknown
const conversation = (id: string) => ({ id, title: id, createdAt: '2026-10-03T12:00:00Z', updatedAt: '2026-10-03T12:00:00Z' })

beforeEach(() => {
  requests = []
  respond = () => ({ items: [] })
  Object.assign(globals, {
    ref, computed,
    useAuthStore: () => ({ user: { role: 'admin' }, authErrorMessage: (_cause: unknown, fallback: string) => fallback }),
    useI18n: () => ({ t: (key: string) => messages[key] ?? key }),
    onMounted: () => {}, onBeforeUnmount: () => {},
    $fetch: async (url: string, options?: { method?: string, body?: unknown }) => {
      requests.push({ url, options })
      return respond(url, options)
    }
  })
})
afterEach(() => {
  for (const name of names) {
    if (original.get(name) === undefined) delete globals[name]
    else globals[name] = original.get(name)
  }
})

function savedChat() {
  const chat = useAssistantChat()
  chat.loading.value = false
  chat.conversations.value = [conversation('current'), conversation('older')]
  chat.conversationId.value = 'current'
  chat.messages.value = [{ id: 'message', role: 'user', parts: [{ type: 'text', text: 'Saved question' }], metadata: { status: 'completed' } }]
  return chat
}

test('new conversation persists immediately and preserves previous history', async () => {
  const chat = savedChat()
  respond = () => conversation('new')
  await chat.newConversation()
  expect(requests).toEqual([{ url: '/api/assistant/conversations', options: { method: 'POST', body: { title: 'New conversation' } } }])
  expect(chat.conversations.value.map(item => item.id)).toEqual(['new', 'current', 'older'])
  expect(chat.conversationId.value).toBe('new')
  expect(chat.messages.value).toEqual([])
  expect(chat.loading.value).toBe(false)
})

test('deletion loads the next saved conversation after the server succeeds', async () => {
  const chat = savedChat()
  respond = url => url.endsWith('/messages') ? { items: [{ id: 'older-message', conversationId: 'older', role: 'assistant', content: 'Previous answer', status: 'completed' }] } : { success: true }
  expect(await chat.deleteConversation()).toBe(true)
  expect(requests.map(item => [item.url, item.options?.method || 'GET'])).toEqual([
    ['/api/assistant/conversations/current', 'DELETE'],
    ['/api/assistant/conversations/older/messages', 'GET']
  ])
  expect(chat.conversations.value.map(item => item.id)).toEqual(['older'])
  expect(chat.conversationId.value).toBe('older')
  expect(chat.messages.value[0]?.parts[0]?.text).toBe('Previous answer')
})

test('deleting the last conversation leaves an empty chat', async () => {
  const chat = savedChat()
  chat.conversations.value = [conversation('current')]
  expect(await chat.deleteConversation()).toBe(true)
  expect(chat.conversationId.value).toBeUndefined()
  expect(chat.conversations.value).toEqual([])
  expect(chat.messages.value).toEqual([])
})

test('failed creation or deletion keeps the current conversation and messages', async () => {
  const chat = savedChat()
  respond = () => { throw new Error('Server unavailable') }
  await chat.newConversation()
  expect(await chat.deleteConversation()).toBe(false)
  expect(chat.conversationId.value).toBe('current')
  expect(chat.conversations.value).toHaveLength(2)
  expect(chat.messages.value[0]?.parts[0]?.text).toBe('Saved question')
  expect(chat.error.value).toBe('Server unavailable')
  expect(chat.loading.value).toBe(false)
})

test('conversation changes are blocked while a response is active', async () => {
  const chat = savedChat()
  chat.status.value = 'streaming'
  await chat.newConversation()
  expect(await chat.deleteConversation()).toBe(false)
  expect(requests).toEqual([])
  expect(chat.conversationId.value).toBe('current')
})
