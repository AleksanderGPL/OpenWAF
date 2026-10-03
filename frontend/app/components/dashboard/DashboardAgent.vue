<script setup lang="ts">
import { createReusableTemplate, useMediaQuery } from '@vueuse/core'
import { agentSuggestions } from '~/data/assistant'
import type { AssistantMessageStatus } from '~/types/assistant'
import { uiLocale } from '~/utils/i18n'

const { t, te } = useI18n()
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
  label: `${item.title} · ${new Date(item.createdAt).toLocaleString(uiLocale())}`, value: item.id
})))
function toolLabel(name: string) {
  const key = `agent.tools.${name}`
  return te(key) ? t(key) : name.replaceAll('_', ' ')
}
function statusLabel(value: AssistantMessageStatus) {
  const key = `agent.status.${value}`
  return te(key) ? t(key) : value.replaceAll('_', ' ')
}
async function confirmDelete() {
  deleting.value = true
  try {
    if (await deleteConversation()) deleteOpen.value = false
  } finally {
    deleting.value = false
  }
}
const chatStatus = computed(() => busy.value && status.value === 'ready' ? 'submitted' : status.value)
const activity = computed(() => stopping.value ? t('agent.stopping') : activeTool.value ? t('agent.checking', { tool: toolLabel(activeTool.value) }) : busy.value ? t('agent.investigating') : t('agent.sendHint'))

watch(desktop, value => { if (value) open.value = false })
</script>

<template>
  <DefineChat>
    <div class="flex min-h-0 flex-1 flex-col">
      <div v-if="isAdmin" class="shrink-0 space-y-2 border-b border-default px-4 py-3">
        <div class="flex items-center justify-between gap-2">
          <UButton :label="t('agent.newConversation')" icon="i-lucide-square-pen" color="neutral" variant="outline" size="sm" :disabled="loading || busy" @click="newConversation" />
          <UTooltip :text="busy ? t('agent.deleteDisabled') : t('agent.deleteConversation')">
            <UButton icon="i-lucide-trash-2" color="error" variant="ghost" size="sm" :aria-label="t('agent.deleteConversation')" :disabled="!conversationId || loading || busy" @click="deleteOpen = true" />
          </UTooltip>
        </div>
        <USelect
          v-if="conversations.length"
          :model-value="conversationId"
          :items="conversationItems"
          :placeholder="t('agent.newConversation')"
          :disabled="loading || busy"
          icon="i-lucide-messages-square"
          :aria-label="t('agent.history')"
          class="w-full"
          @update:model-value="selectConversation($event)"
        />
      </div>

      <div class="min-h-0 flex-1 overflow-y-auto overscroll-contain px-4 py-5">
        <div v-if="loading" class="flex items-center gap-2 text-sm text-muted" role="status">
          <UIcon name="i-lucide-loader-circle" class="size-4 animate-spin" />
          {{ t('agent.loading') }}
        </div>
        <UAlert v-else-if="!isAdmin" :title="t('settings.adminRequired')" :description="t('agent.adminRequiredDescription')" color="neutral" variant="subtle" />
        <UAlert
          v-else-if="configuration && !configuration.enabled"
          :title="t('agent.unavailable')"
          :description="t('agent.unavailableDescription')"
          color="neutral"
          variant="subtle"
          class="mb-4"
        />
        <div v-else-if="!messages.length && !error" class="space-y-3 py-6 text-center">
          <UIcon name="i-lucide-sparkles" class="size-8 text-primary" />
          <h2 class="font-semibold text-highlighted">{{ t('agent.emptyTitle') }}</h2>
          <p class="text-sm text-muted">{{ t('agent.emptyDescription') }}</p>
        </div>
        <UChatMessages
          :messages="messages"
          :status="chatStatus"
          should-auto-scroll
          compact
          :aria-label="t('agent.conversation')"
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
            <span v-if="message.metadata?.status && !['completed', 'running'].includes(message.metadata.status)" class="text-xs text-muted">
              {{ statusLabel(message.metadata.status) }}
            </span>
          </template>
        </UChatMessages>
      </div>

      <div class="shrink-0 space-y-3 border-t border-default bg-default p-4">
        <div v-if="error" class="space-y-2" role="alert">
          <p class="text-sm text-error">{{ error }}</p>
          <UButton :label="t('agent.reload')" icon="i-lucide-refresh-cw" color="neutral" variant="ghost" size="xs" :disabled="loading || busy" @click="initialize" />
        </div>
        <div class="flex flex-wrap gap-2">
          <UButton
            v-for="suggestion in agentSuggestions"
            :key="suggestion.labelKey"
            :label="t(suggestion.labelKey)"
            :icon="suggestion.icon"
            :disabled="!canSend"
            size="xs"
            color="neutral"
            variant="outline"
            @click="send(t(suggestion.promptKey))"
          />
        </div>
        <UChatPrompt
          v-model="prompt"
          :placeholder="t('agent.placeholder')"
          :disabled="!canSend"
          :autofocus="false"
          :rows="2"
          :maxrows="5"
          :ui="{ base: 'text-sm' }"
          :aria-label="t('agent.message')"
          @submit="send()"
        >
          <template #footer>
            <span class="text-xs text-dimmed" role="status">{{ activity }}</span>
            <UChatPromptSubmit
              :status="chatStatus"
              :disabled="!canSend || !prompt.trim()"
              :aria-label="busy ? t('agent.stop') : t('agent.send')"
              size="sm"
              @stop="stop"
            />
          </template>
        </UChatPrompt>
      </div>
    </div>
  </DefineChat>

  <aside v-if="desktop" v-show="visible" id="openwaf-agent-panel" class="flex w-[380px] shrink-0 flex-col border-l border-default bg-default" :aria-label="t('agent.panel')">
    <UDashboardNavbar :title="t('agent.title')" :ui="{ root: 'shrink-0 px-4', title: 'text-sm font-semibold' }">
      <template #leading>
        <UIcon name="i-lucide-sparkles" class="size-5 text-primary" />
      </template>
      <template #right>
        <UTooltip :text="t('agent.hide')">
          <UButton icon="i-lucide-panel-right-close" color="neutral" variant="ghost" size="sm" :aria-label="t('agent.hide')" @click="visible = false" />
        </UTooltip>
      </template>
    </UDashboardNavbar>
    <ReuseChat />
  </aside>
  <USlideover
    v-else
    v-model:open="open"
    :title="t('agent.title')"
    :description="t('agent.slideoverDescription')"
    :content="{ id: 'openwaf-agent-panel' }"
    :ui="{ content: 'max-w-[420px]', body: 'flex min-h-0 flex-1 flex-col overflow-hidden p-0 sm:p-0' }"
  >
    <template #body>
      <ReuseChat />
    </template>
  </USlideover>
  <UModal
    v-model:open="deleteOpen"
    :title="t('agent.deleteTitle')"
    :description="t('agent.deleteDescription', { title: selectedConversation?.title || t('agent.thisConversation') })"
    :dismissible="!deleting"
    :close="!deleting"
  >
    <template #body>
      <p class="text-sm text-muted">{{ t('agent.cannotUndo') }}</p>
      <p v-if="error" class="mt-3 text-sm text-error" role="alert">{{ error }}</p>
    </template>
    <template #footer>
      <div class="flex w-full justify-end gap-2">
        <UButton :label="t('common.cancel')" color="neutral" variant="outline" :disabled="deleting" @click="deleteOpen = false" />
        <UButton :label="t('agent.deleteConversation')" icon="i-lucide-trash-2" color="error" :loading="deleting" :disabled="loading || busy || !conversationId" @click="confirmDelete" />
      </div>
    </template>
  </UModal>
</template>
