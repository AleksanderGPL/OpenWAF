<script setup lang="ts">
import type { AssistantMessageStatus } from '~/types/assistant'

const props = defineProps<{
  investigationId: string
  enabled: boolean
}>()
const { t, te } = useI18n()
const id = computed(() => props.investigationId)
const {
  messages, prompt, loading, opened, error, status, stopping, activeTool, busy, canSend, open, send, stop
} = useInvestigationFollowUp(id)
const chatStatus = computed(() => busy.value && status.value === 'ready' ? 'submitted' : status.value)
const suggestions = [{
  label: 'investigations.suggestions.otherService',
  prompt: 'investigations.suggestions.otherServicePrompt'
}, {
  label: 'investigations.suggestions.evidence',
  prompt: 'investigations.suggestions.evidencePrompt'
}]
function toolLabel(name: string) {
  const key = `agent.tools.${name}`
  return te(key) ? t(key) : name.replaceAll('_', ' ')
}
function statusLabel(value: AssistantMessageStatus) {
  const key = `agent.status.${value}`
  return te(key) ? t(key) : value.replaceAll('_', ' ')
}
const activity = computed(() => {
  if (stopping.value) return t('agent.stopping')
  if (activeTool.value) return t('agent.checking', { tool: toolLabel(activeTool.value) })
  if (busy.value) return t('investigations.followingUp')
  return t('agent.sendHint')
})
</script>

<template>
  <section class="space-y-3">
    <div>
      <h3 class="text-sm font-semibold text-highlighted">
        {{ t('investigations.followUp') }}
      </h3>
      <p class="mt-1 text-sm text-muted">
        {{ t('investigations.followUpDescription') }}
      </p>
    </div>
    <UAlert
      v-if="!enabled"
      :title="t('investigations.followUpWaiting')"
      icon="i-lucide-clock"
      color="neutral"
      variant="subtle"
    />
    <div v-else-if="!opened" class="space-y-3">
      <UAlert v-if="error" :title="error" color="error" variant="subtle" />
      <UButton
        :label="t('investigations.followUpStart')"
        icon="i-lucide-sparkles"
        color="primary"
        variant="soft"
        :loading="loading"
        @click="open"
      />
    </div>
    <div v-else class="overflow-hidden rounded-lg border border-default">
      <div class="h-80 overflow-y-auto px-4 py-4">
        <div v-if="loading" class="flex items-center gap-2 text-sm text-muted" role="status">
          <UIcon name="i-lucide-loader-circle" class="size-4 animate-spin" />
          {{ t('agent.loading') }}
        </div>
        <div v-else-if="!messages.length && !error" class="py-8 text-center">
          <UIcon name="i-lucide-sparkles" class="size-6 text-primary" />
          <p class="mt-2 text-sm text-muted">
            {{ t('investigations.followUpEmpty') }}
          </p>
        </div>
        <UChatMessages
          v-else
          :messages="messages"
          :status="chatStatus"
          should-auto-scroll
          compact
          :aria-label="t('investigations.followUp')"
        >
          <template #content="{ message }">
            <template v-for="(part, index) in message.parts" :key="index">
              <DashboardAgentMarkdown
                v-if="message.role === 'assistant' && part.type === 'text'"
                :text="part.text"
                :streaming="status === 'streaming' && message.metadata?.status === 'running'"
              />
              <p v-else-if="part.type === 'text'" class="whitespace-pre-wrap break-words">
                {{ part.text }}
              </p>
            </template>
          </template>
          <template #actions="{ message }">
            <span v-if="message.metadata?.status && !['completed', 'running'].includes(message.metadata.status)" class="text-xs text-muted">
              {{ statusLabel(message.metadata.status) }}
            </span>
          </template>
        </UChatMessages>
      </div>
      <div class="space-y-3 border-t border-default p-3">
        <p v-if="error" class="text-sm text-error" role="alert">
          {{ error }}
        </p>
        <div class="flex flex-wrap gap-2">
          <UButton
            v-for="suggestion in suggestions"
            :key="suggestion.label"
            :label="t(suggestion.label)"
            size="xs"
            color="neutral"
            variant="outline"
            :disabled="!canSend"
            @click="send(t(suggestion.prompt))"
          />
        </div>
        <UChatPrompt
          v-model="prompt"
          :placeholder="t('investigations.followUpPlaceholder')"
          :disabled="!canSend"
          :rows="2"
          :maxrows="5"
          :ui="{ base: 'text-sm' }"
          :aria-label="t('investigations.followUp')"
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
  </section>
</template>
