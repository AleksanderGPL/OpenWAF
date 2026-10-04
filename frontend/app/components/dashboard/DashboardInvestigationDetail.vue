<script setup lang="ts">
import type { Service } from '~/types/service'
import type { InvestigationCard, InvestigationDetail, InvestigationEvent, InvestigationState } from '~/types/investigation'
import { EventStreamError, readSseStream } from '~/utils/sse'

const props = defineProps<{
  id: string
  services: Service[]
  aiEnabled: boolean
}>()
const emit = defineEmits<{
  close: []
  changed: [item: InvestigationCard]
}>()
const { t, te } = useI18n()
const auth = useAuthStore()
const toast = useToast()
const open = ref(true)
const detail = ref<InvestigationDetail | null>(null)
const loading = ref(true)
const loadError = ref('')
const actionError = ref('')
const pending = ref('')
const expandedEvidence = ref<number | null>(null)
const seenVersion = ref<number | null>(null)
let streamAbort: AbortController | undefined
let reloadTimer: ReturnType<typeof setTimeout> | undefined
let disposed = false

const documentEvents = new Set([
  'investigation.updated',
  'investigation.queued',
  'investigation.started',
  'investigation.result_published',
  'investigation.completed',
  'investigation.failed',
  'investigation.cancelled',
  'investigation.cancellation_requested'
])
const eventLabels: Record<string, string> = {
  'investigation.created': 'created',
  'investigation.updated': 'updated',
  'investigation.queued': 'queued',
  'investigation.started': 'started',
  'activity.started': 'activityStarted',
  'activity.completed': 'activityCompleted',
  'investigation.result_published': 'resultPublished',
  'investigation.completed': 'completed',
  'investigation.failed': 'failed',
  'investigation.cancellation_requested': 'cancellationRequested',
  'investigation.cancelled': 'cancelled'
}
const eventIcons: Record<string, string> = {
  'investigation.created': 'i-lucide-plus',
  'investigation.updated': 'i-lucide-pencil',
  'investigation.queued': 'i-lucide-clock',
  'investigation.started': 'i-lucide-play',
  'activity.started': 'i-lucide-search',
  'activity.completed': 'i-lucide-check',
  'investigation.result_published': 'i-lucide-file-check',
  'investigation.completed': 'i-lucide-circle-check',
  'investigation.failed': 'i-lucide-circle-alert',
  'investigation.cancellation_requested': 'i-lucide-ban',
  'investigation.cancelled': 'i-lucide-circle-off'
}

const progress = computed(() => groupInvestigationProgress(detail.value?.events ?? []))
const evidence = computed(() => detail.value?.result?.evidence ?? [])
const patterns = computed(() => detail.value?.result?.patterns ?? [])
const recommendations = computed(() => detail.value?.result?.recommendations ?? [])
const limitations = computed(() => detail.value?.result?.limitations ?? [])
const running = computed(() => !!detail.value && ['queued', 'running'].includes(detail.value.status))
const staleResult = computed(() => !!detail.value?.result && detail.value.status !== 'completed')
const newerResult = computed(() => seenVersion.value != null && !!detail.value && detail.value.resultVersion !== seenVersion.value)
const serviceLabel = computed(() => {
  const service = props.services.find(item => item.id === detail.value?.serviceId)
  return service ? `${service.name} · ${service.hostname}` : t('investigations.serviceMissing', { id: detail.value?.serviceId ?? '' })
})
const stateActions = computed(() => {
  const state = detail.value?.state
  if (state === 'resolved' || state === 'dismissed') return ['open'] as const
  if (state === 'acknowledged') return ['resolved', 'dismissed', 'open'] as const
  if (state === 'open') return ['acknowledged', 'resolved', 'dismissed'] as const
  return []
})

function titleOf(value: string) {
  return value === 'Traffic investigation' ? t('agent.investigationTitle') : value
}
function detectorLabel(detector: string) {
  const key = `investigations.detectors.${detector}`
  return te(key) ? t(key) : detector
}
function eventText(event: InvestigationEvent) {
  if (event.type === 'investigation.updated' && event.activity) {
    const key = `investigations.states.${event.activity}`
    if (te(key)) return t(key)
  }
  if (event.activity) return event.activity
  const key = eventLabels[event.type]
  return key ? t(`investigations.events.${key}`) : event.type
}
function publish(next: InvestigationDetail) {
  detail.value = next
  emit('changed', next)
}
function sleep(ms: number, signal: AbortSignal) {
  return new Promise<void>((resolve) => {
    const timer = setTimeout(resolve, ms)
    signal.addEventListener('abort', () => { clearTimeout(timer); resolve() }, { once: true })
  })
}
function appendEvent(event: InvestigationEvent) {
  if (!detail.value || detail.value.events.some(item => item.id === event.id)) return
  detail.value.events = mergeInvestigationEvents(detail.value.events, [event])
  detail.value.lastEventId = Math.max(detail.value.lastEventId, event.id)
}
async function reload() {
  const next = await $fetch<InvestigationDetail>(`/api/investigations/${props.id}`)
  if (disposed || !detail.value) return
  next.events = mergeInvestigationEvents(detail.value.events, next.events ?? [])
  publish(next)
}
function scheduleReload() {
  clearTimeout(reloadTimer)
  reloadTimer = setTimeout(() => { void reload().catch(() => {}) }, 300)
}
async function subscribe(after: number) {
  streamAbort?.abort()
  const abort = new AbortController()
  streamAbort = abort
  let cursor = after
  while (!abort.signal.aborted && !disposed) {
    try {
      const response = await fetch(`/api/investigations/${props.id}/events?after=${cursor}`, {
        headers: { Accept: 'text/event-stream' },
        credentials: 'same-origin',
        signal: abort.signal
      })
      await readSseStream(response, (frame) => {
        let event: InvestigationEvent
        try {
          event = JSON.parse(frame.data) as InvestigationEvent
        } catch {
          return
        }
        if (event.id > cursor) cursor = event.id
        appendEvent(event)
        if (documentEvents.has(event.type)) scheduleReload()
      })
    } catch (error) {
      if (abort.signal.aborted || disposed) return
      if (error instanceof EventStreamError && error.status >= 400 && error.status < 500) return
    }
    if (abort.signal.aborted || disposed) return
    await sleep(1000, abort.signal)
  }
}
async function load() {
  loading.value = true
  loadError.value = ''
  try {
    const next = await $fetch<InvestigationDetail>(`/api/investigations/${props.id}`)
    if (disposed) return
    next.events = next.events ?? []
    if (next.result) {
      next.result.patterns = next.result.patterns ?? []
      next.result.recommendations = next.result.recommendations ?? []
      next.result.limitations = next.result.limitations ?? []
      next.result.evidence = next.result.evidence ?? []
    }
    seenVersion.value = next.resultVersion
    publish(next)
    if (!next.read) {
      try {
        const updated = await $fetch<InvestigationCard>(`/api/investigations/${props.id}`, {
          method: 'PATCH',
          body: { read: true, resultVersion: next.resultVersion }
        })
        if (!disposed && detail.value) {
          publish({ ...detail.value, ...updated })
          if (updated.resultVersion !== next.resultVersion) await reload()
        }
      } catch {
        // A newer assessment can land before this read is stored. Leave it unread.
      }
    }
    void subscribe(next.lastEventId)
  } catch (error) {
    if (!disposed) loadError.value = auth.authErrorMessage(error, t('investigations.detailFailed'))
  } finally {
    if (!disposed) loading.value = false
  }
}
async function patch(body: { state?: InvestigationState, read?: boolean, resultVersion?: number }, action: string, success: string) {
  if (!detail.value || pending.value) return
  pending.value = action
  actionError.value = ''
  try {
    const updated = await $fetch<InvestigationCard>(`/api/investigations/${props.id}`, { method: 'PATCH', body })
    if (detail.value) publish({ ...detail.value, ...updated })
    if (action === 'read') seenVersion.value = updated.resultVersion
    toast.add({ title: success, icon: 'i-lucide-circle-check', color: 'success' })
  } catch (error) {
    actionError.value = auth.authErrorMessage(error, t('investigations.actionFailed'))
  } finally {
    pending.value = ''
  }
}
async function retry() {
  if (!detail.value || pending.value || running.value || !props.aiEnabled) return
  pending.value = 'retry'
  actionError.value = ''
  try {
    const next = await $fetch<InvestigationDetail>(`/api/investigations/${props.id}/retry`, { method: 'POST' })
    next.events = mergeInvestigationEvents(detail.value.events, next.events ?? [])
    publish(next)
    toast.add({ title: t('investigations.retryQueued'), icon: 'i-lucide-circle-check', color: 'success' })
  } catch (error) {
    actionError.value = auth.authErrorMessage(error, t('investigations.actionFailed'))
  } finally {
    pending.value = ''
  }
}
async function cancel() {
  if (!detail.value || pending.value || !running.value) return
  pending.value = 'cancel'
  actionError.value = ''
  try {
    await $fetch(`/api/investigations/${props.id}/cancel`, { method: 'POST' })
    await reload()
    toast.add({ title: t('investigations.cancelRequested'), icon: 'i-lucide-circle-check', color: 'success' })
  } catch (error) {
    actionError.value = auth.authErrorMessage(error, t('investigations.actionFailed'))
  } finally {
    pending.value = ''
  }
}
function stateLabel(state: string) {
  if (state === 'open' && detail.value?.state !== 'open') return t('investigations.reopen')
  if (state === 'acknowledged') return t('investigations.acknowledge')
  if (state === 'resolved') return t('investigations.resolve')
  if (state === 'dismissed') return t('investigations.dismiss')
  return t('investigations.reopen')
}
function actionColor(action: string) {
  return action === 'blocked' ? 'error' as const : 'success' as const
}

watch(open, (value) => { if (!value) emit('close') })
onMounted(load)
onBeforeUnmount(() => {
  disposed = true
  streamAbort?.abort()
  clearTimeout(reloadTimer)
})
</script>

<template>
  <USlideover
    v-model:open="open"
    :title="detail ? titleOf(detail.title) : t('investigations.loading')"
    :description="detail ? serviceLabel : ''"
    :ui="{ content: 'max-w-3xl w-full' }"
  >
    <template #body>
      <div v-if="loading" class="flex items-center gap-2 text-sm text-muted" role="status">
        <UIcon name="i-lucide-loader-circle" class="size-4 animate-spin" />
        {{ t('investigations.loading') }}
      </div>
      <div v-else-if="loadError || !detail" class="space-y-4">
        <UAlert :title="loadError || t('investigations.detailFailed')" :description="t('common.checkConnection')" color="error" variant="subtle" />
        <UButton :label="t('common.tryAgain')" icon="i-lucide-refresh-cw" color="neutral" variant="outline" @click="load" />
      </div>
      <div v-else class="space-y-8">
        <div class="flex flex-wrap items-center gap-2">
          <UBadge v-if="!detail.read" :label="t('investigations.unread')" color="primary" variant="subtle" />
          <UBadge v-if="detail.severity" :label="t(`investigations.severities.${detail.severity}`)" :color="severityColor(detail.severity)" :variant="detail.severity === 'critical' ? 'solid' : 'subtle'" />
          <UBadge v-if="detail.assessment" :label="t(`investigations.assessments.${detail.assessment}`)" :color="assessmentColor(detail.assessment)" variant="subtle" />
          <UBadge :label="t(`investigations.statuses.${detail.status}`)" :color="statusColor(detail.status)" variant="subtle" />
          <UBadge :label="t(`investigations.states.${detail.state}`)" :color="stateColor(detail.state)" variant="outline" />
          <UBadge v-if="detail.cancellationRequested && running" :label="t('investigations.cancelRequested')" color="warning" variant="subtle" />
        </div>

        <p class="text-sm text-muted">
          {{ detail.summary }}
        </p>
        <p class="text-xs text-dimmed">
          {{ detail.ip || t('investigations.unknownSource') }}
          · {{ serviceLabel }}
          · {{ t('investigations.lastSeen', { time: formatInvestigationTime(detail.lastSeen, uiLocale()) }) }}
          · {{ t('investigations.version', { version: detail.resultVersion }) }}
        </p>

        <UAlert v-if="newerResult" :title="t('investigations.newResult')" :description="t('investigations.newResultDescription')" icon="i-lucide-bell" color="primary" variant="subtle" />
        <UAlert v-if="detail.status === 'detected'" :title="t('investigations.detectedHint')" icon="i-lucide-hourglass" color="neutral" variant="subtle" />
        <UAlert v-if="detail.status === 'failed' && detail.error" :title="t('investigations.analysisError')" :description="detail.error" icon="i-lucide-circle-alert" color="error" variant="subtle" />
        <UAlert v-if="actionError" :title="actionError" color="error" variant="subtle" />

        <section class="space-y-3">
          <h3 class="text-sm font-semibold text-highlighted">
            {{ t('investigations.trigger') }}
          </h3>
          <div class="rounded-lg border border-default p-4">
            <div class="flex flex-wrap items-center gap-2">
              <UBadge :label="detectorLabel(detail.trigger.detector)" color="neutral" variant="subtle" icon="i-lucide-radar" />
              <span class="text-xs text-dimmed">
                {{ formatInvestigationTime(detail.trigger.from, uiLocale()) }} – {{ formatInvestigationTime(detail.trigger.to, uiLocale()) }}
              </span>
            </div>
            <p class="mt-3 text-sm text-toned">
              {{ detail.trigger.explanation }}
            </p>
            <dl class="mt-4 grid gap-3 sm:grid-cols-3">
              <div>
                <dt class="text-xs text-muted">{{ t('investigations.observed') }}</dt>
                <dd class="mt-1 text-sm font-medium text-highlighted">{{ formatLatency(detail.trigger.observed) }}</dd>
              </div>
              <div>
                <dt class="text-xs text-muted">{{ t('investigations.threshold') }}</dt>
                <dd class="mt-1 text-sm font-medium text-highlighted">{{ formatLatency(detail.trigger.threshold) }}</dd>
              </div>
              <div v-if="detail.trigger.baseline != null">
                <dt class="text-xs text-muted">{{ t('investigations.baseline') }}</dt>
                <dd class="mt-1 text-sm font-medium text-highlighted">{{ formatLatency(detail.trigger.baseline) }}</dd>
              </div>
            </dl>
            <p v-if="detail.trigger.requestIds?.length" class="mt-3 text-xs text-dimmed">
              {{ t(`investigations.sampled.${pluralForm(detail.trigger.requestIds.length)}`, { count: detail.trigger.requestIds.length }) }}
            </p>
            <p v-if="detail.trigger.coverage" class="mt-2 text-xs text-dimmed">
              {{ detail.trigger.coverage }}
            </p>
          </div>
        </section>

        <section class="space-y-3">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <h3 class="text-sm font-semibold text-highlighted">
              {{ t('investigations.result') }}
            </h3>
            <span v-if="detail.result" class="text-xs text-dimmed">
              {{ t('investigations.published', { time: formatInvestigationTime(detail.result.publishedAt, uiLocale()) }) }}
            </span>
          </div>
          <UAlert v-if="!detail.result" :title="t('investigations.noResult')" color="neutral" variant="subtle" />
          <div v-else class="space-y-4">
            <UAlert v-if="staleResult" :title="t('investigations.previousResult')" icon="i-lucide-history" color="neutral" variant="subtle" />
            <p class="text-sm font-medium text-highlighted">
              {{ detail.result.title }}
            </p>
            <p class="text-sm text-muted">
              {{ detail.result.summary }}
            </p>
            <DashboardAgentMarkdown v-if="detail.result.explanation" :text="detail.result.explanation" :streaming="false" />
            <div v-if="patterns.length">
              <h4 class="text-xs font-medium text-muted">{{ t('investigations.patterns') }}</h4>
              <ul class="mt-2 list-disc space-y-1 pl-5 text-sm text-toned">
                <li v-for="item in patterns" :key="item">{{ item }}</li>
              </ul>
            </div>
            <div v-if="recommendations.length">
              <h4 class="text-xs font-medium text-muted">{{ t('investigations.recommendations') }}</h4>
              <ul class="mt-2 list-disc space-y-1 pl-5 text-sm text-toned">
                <li v-for="item in recommendations" :key="item">{{ item }}</li>
              </ul>
            </div>
            <div v-if="limitations.length">
              <h4 class="text-xs font-medium text-muted">{{ t('investigations.limitations') }}</h4>
              <ul class="mt-2 list-disc space-y-1 pl-5 text-sm text-toned">
                <li v-for="item in limitations" :key="item">{{ item }}</li>
              </ul>
            </div>
          </div>
        </section>

        <section v-if="detail.result" class="space-y-3">
          <h3 class="text-sm font-semibold text-highlighted">
            {{ t('investigations.evidence') }}
          </h3>
          <p v-if="!evidence.length" class="text-sm text-muted">
            {{ t('investigations.evidenceEmpty') }}
          </p>
          <div v-else class="divide-y divide-default overflow-hidden rounded-lg border border-default">
            <div v-for="item in evidence" :key="item.request.id">
              <button
                type="button"
                class="flex w-full items-center gap-2 px-3 py-3 text-left hover:bg-elevated/40"
                :aria-expanded="expandedEvidence === item.request.id"
                @click="expandedEvidence = expandedEvidence === item.request.id ? null : item.request.id"
              >
                <UBadge :label="item.request.method" :color="item.request.method === 'POST' ? 'primary' : 'info'" variant="soft" size="sm" />
                <span class="min-w-0 flex-1 truncate font-mono text-xs text-toned">{{ item.request.path }}</span>
                <UBadge :label="t(item.request.action === 'blocked' ? 'logs.blocked' : 'logs.allowed')" :color="actionColor(item.request.action)" variant="subtle" size="sm" />
                <UBadge v-if="!item.available" :label="t('investigations.snapshotOnly')" color="warning" variant="subtle" size="sm" />
                <UIcon :name="expandedEvidence === item.request.id ? 'i-lucide-chevron-up' : 'i-lucide-chevron-down'" class="size-4 shrink-0 text-muted" />
              </button>
              <dl v-if="expandedEvidence === item.request.id" class="grid gap-3 border-t border-default bg-elevated/30 px-3 py-3 text-sm sm:grid-cols-2">
                <div>
                  <dt class="text-xs text-muted">{{ t('request.id') }}</dt>
                  <dd class="mt-1 font-mono text-xs">{{ item.request.id }}</dd>
                </div>
                <div>
                  <dt class="text-xs text-muted">{{ t('request.time') }}</dt>
                  <dd class="mt-1">{{ formatInvestigationTime(item.request.timestamp, uiLocale()) }}</dd>
                </div>
                <div>
                  <dt class="text-xs text-muted">{{ t('request.sourceIp') }}</dt>
                  <dd class="mt-1 font-mono text-xs">{{ item.request.ip }}</dd>
                </div>
                <div>
                  <dt class="text-xs text-muted">{{ t('request.status') }}</dt>
                  <dd class="mt-1">{{ item.request.status }}</dd>
                </div>
                <div class="sm:col-span-2">
                  <dt class="text-xs text-muted">{{ t('request.rule') }}</dt>
                  <dd class="mt-1">{{ formatRuleMessage(item.request.reason || item.request.ruleId || '—') }}</dd>
                </div>
                <div v-if="item.request.ruleMatches?.length" class="sm:col-span-2 space-y-2">
                  <dt class="text-xs text-muted">{{ t('investigations.ruleMatches') }}</dt>
                  <dd v-for="match in item.request.ruleMatches" :key="`${match.ruleId}-${match.message}`" class="rounded-md border border-default px-3 py-2">
                    <p class="text-sm text-highlighted">{{ formatRuleMessage(match.message || match.ruleId) }}</p>
                    <p class="mt-1 text-xs text-dimmed">{{ match.ruleId }}<span v-if="match.severity"> · {{ match.severity }}</span></p>
                  </dd>
                </div>
              </dl>
            </div>
          </div>
        </section>

        <section class="space-y-3">
          <h3 class="text-sm font-semibold text-highlighted">
            {{ t('investigations.progress') }}
          </h3>
          <UAlert v-if="detail.eventsTruncated" :title="t('investigations.olderEvents')" color="neutral" variant="subtle" />
          <p v-if="!progress.length" class="text-sm text-muted">
            {{ t('investigations.progressEmpty') }}
          </p>
          <ol v-else class="space-y-3">
            <li v-for="block in progress" :key="block.kind === 'event' ? block.event.id : `explanation-${block.id}`" class="flex gap-3">
              <span class="mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-md bg-elevated text-muted">
                <UIcon :name="block.kind === 'explanation' ? 'i-lucide-sparkles' : (eventIcons[block.event.type] || 'i-lucide-circle')" class="size-3.5" />
              </span>
              <div class="min-w-0 flex-1">
                <template v-if="block.kind === 'explanation'">
                  <div class="flex flex-wrap items-center gap-2">
                    <p class="text-sm font-medium text-highlighted">{{ t('investigations.narrative') }}</p>
                    <UBadge v-if="block.provisional" :label="t('investigations.provisional')" color="primary" variant="subtle" size="sm" />
                    <span v-if="block.attempt > 1" class="text-xs text-dimmed">{{ t('investigations.attempt', { attempt: block.attempt }) }}</span>
                  </div>
                  <div class="mt-2">
                    <DashboardAgentMarkdown :text="block.text" :streaming="block.provisional && running" />
                  </div>
                </template>
                <template v-else>
                  <p class="text-sm text-highlighted">{{ eventText(block.event) }}</p>
                  <p class="mt-0.5 text-xs text-dimmed">
                    {{ formatInvestigationTime(block.event.createdAt, uiLocale()) }}
                    <span v-if="block.event.attempt > 1"> · {{ t('investigations.attempt', { attempt: block.event.attempt }) }}</span>
                  </p>
                </template>
              </div>
            </li>
          </ol>
        </section>

        <DashboardInvestigationFollowUp :investigation-id="detail.id" :enabled="!running" />
      </div>
    </template>
    <template v-if="detail && !loading" #footer>
      <div class="flex w-full flex-wrap justify-end gap-2">
        <UButton
          :label="t(detail.read ? 'investigations.markUnread' : 'investigations.markRead')"
          :icon="detail.read ? 'i-lucide-mail' : 'i-lucide-mail-check'"
          color="neutral"
          variant="ghost"
          :loading="pending === 'read'"
          :disabled="!!pending"
          @click="patch(detail.read ? { read: false } : { read: true, resultVersion: detail.resultVersion }, 'read', t(detail.read ? 'investigations.markedUnread' : 'investigations.markedRead'))"
        />
        <UButton
          v-for="state in stateActions"
          :key="state"
          :label="stateLabel(state)"
          color="neutral"
          variant="outline"
          :loading="pending === state"
          :disabled="!!pending"
          @click="patch({ state }, state, t('investigations.updated'))"
        />
        <UButton
          v-if="running"
          :label="t('investigations.cancelRun')"
          icon="i-lucide-ban"
          color="error"
          variant="outline"
          :loading="pending === 'cancel'"
          :disabled="!!pending"
          @click="cancel"
        />
        <UTooltip v-else :text="aiEnabled ? t('investigations.retry') : t('investigations.aiNotConfigured')">
          <UButton
            :label="t('investigations.retry')"
            icon="i-lucide-refresh-cw"
            color="primary"
            variant="soft"
            :loading="pending === 'retry'"
            :disabled="!!pending || !aiEnabled"
            @click="retry"
          />
        </UTooltip>
      </div>
    </template>
  </USlideover>
</template>
