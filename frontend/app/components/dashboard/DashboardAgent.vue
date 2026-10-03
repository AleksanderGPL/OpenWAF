<script setup lang="ts">
import { createReusableTemplate, useMediaQuery } from '@vueuse/core'
import { agentSuggestions } from '~/data/assistant'

const open = defineModel<boolean>('open', { default: false })
const visible = defineModel<boolean>('visible', { default: true })
const desktop = useMediaQuery('(min-width: 1280px)')
const [DefineChat, ReuseChat] = createReusableTemplate()
const {
  configuration, conversations, conversationId, messages, prompt, loading, error,
  status, stopping, activeTool, isAdmin, busy, canSend,
  initialize, selectConversation, newConversation, deleteConversation, send, stop
} = useAssistantChat()
const deleteOpen = ref(false)
const deleting = ref(false)
const selectedConversation = computed(() => conversations.value.find(item => item.id === conversationId.value))
const conversationItems = computed(() => conversations.value.map(item => ({
  label: `${item.title} · ${new Date(item.createdAt).toLocaleString()}`, value: item.id
})))
async function confirmDelete() {
  deleting.value = true
  try {
    if (await deleteConversation()) deleteOpen.value = false
  } finally {
    deleting.value = false
  }
}
const chatStatus = computed(() => busy.value && status.value === 'ready' ? 'submitted' : status.value)
const activity = computed(() => stopping.value ? 'Stopping response…' : activeTool.value ? `Checking ${activeTool.value}…` : busy.value ? 'Investigating your workspace…' : 'Enter to send · Shift + Enter for a new line')

watch(desktop, value => { if (value) open.value = false })
</script>

<template>
  <DefineChat>
    <div class="flex min-h-0 flex-1 flex-col">
      <div v-if="isAdmin" class="shrink-0 space-y-2 border-b border-default px-4 py-3">
        <div class="flex items-center justify-between gap-2">
          <UButton label="New conversation" icon="i-lucide-square-pen" color="neutral" variant="outline" size="sm" :disabled="loading || busy" @click="newConversation" />
          <UTooltip :text="busy ? 'Stop the response before deleting' : 'Delete conversation'">
            <UButton icon="i-lucide-trash-2" color="error" variant="ghost" size="sm" aria-label="Delete conversation" :disabled="!conversationId || loading || busy" @click="deleteOpen = true" />
          </UTooltip>
        </div>
        <USelect
          v-if="conversations.length"
          :model-value="conversationId"
          :items="conversationItems"
          placeholder="New conversation"
          :disabled="loading || busy"
          icon="i-lucide-messages-square"
          aria-label="Conversation history"
          class="w-full"
          @update:model-value="selectConversation($event)"
        />
      </div>

      <div class="min-h-0 flex-1 overflow-y-auto overscroll-contain px-4 py-5">
        <div v-if="loading" class="flex items-center gap-2 text-sm text-muted" role="status">
          <UIcon name="i-lucide-loader-circle" class="size-4 animate-spin" />
          Loading assistant…
        </div>
        <UAlert v-else-if="!isAdmin" title="Administrator access required" description="Sign in as an administrator to use the workspace assistant." color="neutral" variant="subtle" />
        <UAlert
          v-else-if="configuration && !configuration.enabled"
          title="Assistant is unavailable"
          description="The assistant has not been configured on the server. Saved conversations are still available."
          color="neutral"
          variant="subtle"
          class="mb-4"
        />
        <div v-else-if="!messages.length && !error" class="space-y-3 py-6 text-center">
          <UIcon name="i-lucide-sparkles" class="size-8 text-primary" />
          <h2 class="font-semibold text-highlighted">Investigate your workspace</h2>
          <p class="text-sm text-muted">Ask about traffic, blocked requests, or log retention. Answers use your retained workspace data.</p>
        </div>
        <UChatMessages
          :messages="messages"
          :status="chatStatus"
          should-auto-scroll
          compact
          aria-label="Agent conversation"
        >
          <template #content="{ message }">
            <template v-for="(part, index) in message.parts" :key="index">
              <DashboardAgentMarkdown
                v-if="message.role === 'assistant' && part.type === 'text'"
                :text="part.text"
                :streaming="status === 'streaming' && message.metadata?.status === 'running'"
              />
              <p v-else-if="part.type === 'text'" class="whitespace-pre-wrap break-words">{{ part.text }}</p>
            </template>
          </template>
          <template #actions="{ message }">
            <span v-if="message.metadata?.status && !['completed', 'running'].includes(message.metadata.status)" class="text-xs capitalize text-muted">
              {{ message.metadata.status.replace(/_/g, ' ') }}
            </span>
          </template>
        </UChatMessages>
      </div>

      <div class="shrink-0 space-y-3 border-t border-default bg-default p-4">
        <div v-if="error" class="space-y-2" role="alert">
          <p class="text-sm text-error">{{ error }}</p>
          <UButton label="Reload assistant" icon="i-lucide-refresh-cw" color="neutral" variant="ghost" size="xs" :disabled="loading || busy" @click="initialize" />
        </div>
        <div class="flex flex-wrap gap-2">
          <UButton
            v-for="suggestion in agentSuggestions"
            :key="suggestion.label"
            :label="suggestion.label"
            :icon="suggestion.icon"
            :disabled="!canSend"
            size="xs"
            color="neutral"
            variant="outline"
            @click="send(suggestion.prompt)"
          />
        </div>
        <UChatPrompt
          v-model="prompt"
          placeholder="Ask about your workspace…"
          :disabled="!canSend"
          :autofocus="false"
          :rows="2"
          :maxrows="5"
          :ui="{ base: 'text-sm' }"
          aria-label="Message the OpenWAF agent"
          @submit="send()"
        >
          <template #footer>
            <span class="text-xs text-dimmed" role="status">{{ activity }}</span>
            <UChatPromptSubmit
              :status="chatStatus"
              :disabled="!canSend || !prompt.trim()"
              :aria-label="busy ? 'Stop response' : 'Send message'"
              size="sm"
              @stop="stop"
            />
          </template>
        </UChatPrompt>
      </div>
    </div>
  </DefineChat>

  <aside v-if="desktop" v-show="visible" id="openwaf-agent-panel" class="flex w-[380px] shrink-0 flex-col border-l border-default bg-default" aria-label="OpenWAF agent">
    <UDashboardNavbar title="OpenWAF agent" :ui="{ root: 'shrink-0 px-4', title: 'text-sm font-semibold' }">
      <template #leading>
        <UIcon name="i-lucide-sparkles" class="size-5 text-primary" />
      </template>
      <template #right>
        <UTooltip text="Hide agent">
          <UButton icon="i-lucide-panel-right-close" color="neutral" variant="ghost" size="sm" aria-label="Hide agent" @click="visible = false" />
        </UTooltip>
      </template>
    </UDashboardNavbar>
    <ReuseChat />
  </aside>
  <USlideover
    v-else
    v-model:open="open"
    title="OpenWAF agent"
    description="Investigate your workspace traffic and blocked requests."
    :content="{ id: 'openwaf-agent-panel' }"
    :ui="{ content: 'max-w-[420px]', body: 'flex min-h-0 flex-1 flex-col overflow-hidden p-0 sm:p-0' }"
  >
    <template #body>
      <ReuseChat />
    </template>
  </USlideover>
  <UModal
    v-model:open="deleteOpen"
    title="Delete conversation?"
    :description="`This permanently deletes “${selectedConversation?.title || 'this conversation'}” and all its messages.`"
    :dismissible="!deleting"
    :close="!deleting"
  >
    <template #body>
      <p class="text-sm text-muted">This action cannot be undone.</p>
      <p v-if="error" class="mt-3 text-sm text-error" role="alert">{{ error }}</p>
    </template>
    <template #footer>
      <div class="flex w-full justify-end gap-2">
        <UButton label="Cancel" color="neutral" variant="outline" :disabled="deleting" @click="deleteOpen = false" />
        <UButton label="Delete conversation" icon="i-lucide-trash-2" color="error" :loading="deleting" :disabled="loading || busy || !conversationId" @click="confirmDelete" />
      </div>
    </template>
  </UModal>
</template>
