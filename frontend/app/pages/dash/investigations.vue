<script setup lang="ts">
import type { Service } from '~/types/service'
import type { AnomalySettingsView, InvestigationCard, InvestigationList } from '~/types/investigation'
import { investigationAssessments, investigationSeverities, investigationStates, investigationStatuses } from '~/types/investigation'

definePageMeta({ layout: 'dashboard' })
const { t } = useI18n()
useSeoMeta({
  title: () => t('seo.investigationsTitle'),
  description: () => t('seo.investigationsDescription')
})

const auth = useAuthStore()
const isAdmin = computed(() => auth.user?.role === 'admin')
const route = useRoute()
const { summary, revision, connectionState, refreshSummary } = useInvestigationFeed()
const page = ref(1)
const serviceFilter = ref<string>()
const stateFilter = ref('all')
const statusFilter = ref<string>()
const severityFilter = ref<string>()
const assessmentFilter = ref<string>()
const unreadOnly = ref(false)
const range = ref<'all' | '24h' | '7d' | '30d'>('all')
const rangeFrom = ref<string>()

watch([serviceFilter, stateFilter, statusFilter, severityFilter, assessmentFilter, unreadOnly, range], () => {
  page.value = 1
  rangeFrom.value = investigationRangeStart(range.value)
}, { flush: 'sync' })

const query = computed(() => ({
  page: page.value,
  limit: 25,
  serviceId: serviceFilter.value,
  state: stateFilter.value === 'all' ? undefined : stateFilter.value,
  status: statusFilter.value,
  severity: severityFilter.value,
  assessment: assessmentFilter.value,
  unread: unreadOnly.value ? true : undefined,
  from: rangeFrom.value
}))
const { data, status, error: loadError, refresh } = await useFetch<InvestigationList>('/api/investigations', {
  query,
  immediate: isAdmin.value,
  default: () => ({ items: [], total: 0, page: 1, limit: 25, lastEventId: 0 })
})
const { data: services } = await useFetch<Service[]>('/api/services', {
  immediate: isAdmin.value,
  default: () => []
})
const { data: anomaly } = await useFetch<AnomalySettingsView>('/api/anomaly-settings', {
  immediate: isAdmin.value
})

const items = computed(() => data.value?.items ?? [])
const total = computed(() => data.value?.total ?? 0)
const limit = computed(() => data.value?.limit || 25)
const fromItem = computed(() => total.value ? (page.value - 1) * limit.value + 1 : 0)
const toItem = computed(() => Math.min(page.value * limit.value, total.value))
const filtered = computed(() => !!serviceFilter.value || stateFilter.value !== 'all' || !!statusFilter.value || !!severityFilter.value || !!assessmentFilter.value || unreadOnly.value || range.value !== 'all')
const selectedId = computed(() => typeof route.query.id === 'string' ? route.query.id : '')
const activeCount = computed(() => (summary.value?.byStatus?.queued ?? 0) + (summary.value?.byStatus?.running ?? 0))
const severeCount = computed(() => (summary.value?.bySeverity?.high ?? 0) + (summary.value?.bySeverity?.critical ?? 0))
const summaryCards = computed(() => [{
  label: t('investigations.summaryTotal'),
  value: formatDashboardNumber(summary.value?.total ?? total.value),
  icon: 'i-lucide-radar',
  iconClass: 'bg-primary/10 text-primary'
}, {
  label: t('investigations.summaryUnread'),
  value: formatDashboardNumber(summary.value?.unread ?? 0),
  icon: 'i-lucide-mail',
  iconClass: 'bg-info/10 text-info'
}, {
  label: t('investigations.summaryActive'),
  value: formatDashboardNumber(activeCount.value),
  detail: t('investigations.activeDetail', { queued: summary.value?.byStatus?.queued ?? 0, running: summary.value?.byStatus?.running ?? 0 }),
  icon: 'i-lucide-loader-circle',
  iconClass: 'bg-warning/10 text-warning'
}, {
  label: t('investigations.summarySevere'),
  value: formatDashboardNumber(severeCount.value),
  detail: t('investigations.severeDetail', { high: summary.value?.bySeverity?.high ?? 0, critical: summary.value?.bySeverity?.critical ?? 0 }),
  icon: 'i-lucide-shield-alert',
  iconClass: 'bg-error/10 text-error'
}])
const aiEnabled = computed(() => anomaly.value?.aiEnabled ?? true)

const serviceOptions = computed(() => (services.value ?? []).map(service => ({
  label: `${service.name} · ${service.hostname}`,
  value: String(service.id)
})))
const stateTabs = computed(() => [{
  label: t('investigations.all'),
  value: 'all'
}, ...investigationStates.map(value => ({
  label: t(`investigations.states.${value}`),
  value
}))])
function countedLabel(label: string, count: number) {
  return count > 0 ? `${label} · ${formatDashboardNumber(count)}` : label
}
const statusOptions = computed(() => investigationStatuses.map(value => ({
  label: countedLabel(t(`investigations.statuses.${value}`), summary.value?.byStatus?.[value] ?? 0),
  value
})))
const severityOptions = computed(() => investigationSeverities.map(value => ({
  label: countedLabel(t(`investigations.severities.${value}`), summary.value?.bySeverity?.[value] ?? 0),
  value
})))
const assessmentOptions = computed(() => investigationAssessments.map(value => ({
  label: t(`investigations.assessments.${value}`),
  value
})))
const rangeOptions = computed(() => [{
  label: t('investigations.rangeAll'),
  value: 'all'
}, {
  label: t('overview.range24h'),
  value: '24h'
}, {
  label: t('overview.range7d'),
  value: '7d'
}, {
  label: t('overview.range30d'),
  value: '30d'
}])
function serviceLabel(id: number) {
  const service = services.value?.find(item => item.id === id)
  return service?.hostname ?? t('investigations.serviceMissing', { id })
}
function titleOf(value: string) {
  return value === 'Traffic investigation' ? t('agent.investigationTitle') : value
}
function clearFilters() {
  stateFilter.value = 'all'
  serviceFilter.value = undefined
  statusFilter.value = undefined
  severityFilter.value = undefined
  assessmentFilter.value = undefined
  unreadOnly.value = false
  range.value = 'all'
}
async function openInvestigation(id: string) {
  await navigateTo({ path: '/dash/investigations', query: { ...route.query, id } })
}
async function closeInvestigation() {
  const next = { ...route.query }
  delete next.id
  await navigateTo({ path: '/dash/investigations', query: next })
}
function onChanged(item: InvestigationCard) {
  if (!data.value) return
  data.value.items = data.value.items.map(current => current.id === item.id ? { ...current, ...item } : current)
  void refreshSummary()
}
let refreshTimer: ReturnType<typeof setTimeout> | undefined
watch(revision, () => {
  clearTimeout(refreshTimer)
  refreshTimer = setTimeout(() => { void refresh() }, 400)
})
onUnmounted(() => clearTimeout(refreshTimer))
</script>

<template>
  <main class="mx-auto flex w-full max-w-7xl flex-col gap-6">
    <UAlert
      v-if="!isAdmin"
      :title="t('settings.adminRequired')"
      :description="t('investigations.adminOnly')"
      icon="i-lucide-lock"
      color="neutral"
      variant="subtle"
    />
    <template v-else>
      <div class="flex flex-wrap items-center justify-between gap-4">
        <div class="flex items-center gap-3">
          <UBadge
            :label="connectionState === 'live' ? t('overview.live') : connectionState === 'offline' ? t('overview.unavailable') : t('investigations.connecting')"
            :color="connectionState === 'live' ? 'success' : 'neutral'"
            variant="soft"
            icon="i-lucide-radar"
          />
          <p class="text-sm text-muted">
            {{ t('investigations.intro') }}
          </p>
        </div>
        <UTooltip :text="t('investigations.refresh')">
          <UButton
            icon="i-lucide-refresh-cw"
            color="neutral"
            variant="outline"
            :aria-label="t('investigations.refresh')"
            :loading="status === 'pending'"
            @click="rangeFrom = investigationRangeStart(range); refresh(); refreshSummary()"
          />
        </UTooltip>
      </div>

      <UAlert
        v-if="anomaly && !anomaly.aiEnabled"
        :title="t('anomaly.aiOff')"
        :description="t('anomaly.aiOffDescription')"
        icon="i-lucide-sparkles"
        color="warning"
        variant="subtle"
      />

      <section class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4" :aria-label="t('investigations.summaryLabel')">
        <UCard v-for="metric in summaryCards" :key="metric.label" :ui="{ body: 'p-5 sm:p-5' }">
          <div class="flex items-center justify-between gap-3">
            <p class="text-sm text-muted">{{ metric.label }}</p>
            <span class="flex size-8 items-center justify-center rounded-lg" :class="metric.iconClass">
              <UIcon :name="metric.icon" class="size-4" />
            </span>
          </div>
          <p class="mt-3 text-3xl font-semibold tracking-tight text-highlighted">{{ metric.value }}</p>
          <p v-if="metric.detail" class="mt-2 text-xs text-dimmed">{{ metric.detail }}</p>
        </UCard>
      </section>

      <UCard :ui="{ body: 'p-0 sm:p-0', footer: 'p-4 sm:px-6' }">
        <template #header>
          <div class="flex flex-wrap items-center gap-2">
            <h2 class="text-sm font-semibold text-highlighted">{{ t('nav.investigations') }}</h2>
            <UBadge :label="t(`investigations.count.${pluralForm(total)}`, { count: formatDashboardNumber(total) })" color="neutral" variant="subtle" size="sm" />
          </div>
        </template>
        <div class="space-y-3 border-b border-default px-4 py-3 sm:px-6">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <UTabs
              v-model="stateFilter"
              :items="stateTabs"
              :content="false"
              variant="link"
              size="sm"
              :aria-label="t('investigations.state')"
              class="min-w-0 max-w-full"
            />
            <div class="flex flex-wrap items-center gap-2">
              <USelect
                v-model="range"
                :items="rangeOptions"
                value-key="value"
                icon="i-lucide-clock"
                size="sm"
                :aria-label="t('investigations.timeRange')"
                class="w-40"
              />
              <UButton
                :label="t('investigations.unread')"
                icon="i-lucide-mail"
                size="sm"
                :color="unreadOnly ? 'primary' : 'neutral'"
                :variant="unreadOnly ? 'soft' : 'ghost'"
                :aria-pressed="unreadOnly"
                @click="unreadOnly = !unreadOnly"
              />
              <UButton
                v-if="filtered"
                :label="t('investigations.clearFilters')"
                icon="i-lucide-x"
                size="sm"
                color="neutral"
                variant="ghost"
                @click="clearFilters"
              />
            </div>
          </div>
          <div class="grid grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-4">
            <USelect
              v-model="serviceFilter"
              :items="serviceOptions"
              value-key="value"
              icon="i-lucide-server"
              size="sm"
              :placeholder="t('investigations.service')"
              :aria-label="t('investigations.service')"
              class="w-full"
            >
              <template v-if="serviceFilter" #trailing>
                <UButton icon="i-lucide-x" size="xs" color="neutral" variant="link" :aria-label="t('investigations.clearFilters')" @pointerdown.stop.prevent="serviceFilter = undefined" />
              </template>
            </USelect>
            <USelect
              v-model="statusFilter"
              :items="statusOptions"
              value-key="value"
              icon="i-lucide-activity"
              size="sm"
              :placeholder="t('investigations.statusShort')"
              :aria-label="t('investigations.status')"
              class="w-full"
            >
              <template v-if="statusFilter" #trailing>
                <UButton icon="i-lucide-x" size="xs" color="neutral" variant="link" :aria-label="t('investigations.clearFilters')" @pointerdown.stop.prevent="statusFilter = undefined" />
              </template>
            </USelect>
            <USelect
              v-model="severityFilter"
              :items="severityOptions"
              value-key="value"
              icon="i-lucide-shield-alert"
              size="sm"
              :placeholder="t('investigations.severity')"
              :aria-label="t('investigations.severity')"
              class="w-full"
            >
              <template v-if="severityFilter" #trailing>
                <UButton icon="i-lucide-x" size="xs" color="neutral" variant="link" :aria-label="t('investigations.clearFilters')" @pointerdown.stop.prevent="severityFilter = undefined" />
              </template>
            </USelect>
            <USelect
              v-model="assessmentFilter"
              :items="assessmentOptions"
              value-key="value"
              icon="i-lucide-scale"
              size="sm"
              :placeholder="t('investigations.assessment')"
              :aria-label="t('investigations.assessment')"
              class="w-full"
            >
              <template v-if="assessmentFilter" #trailing>
                <UButton icon="i-lucide-x" size="xs" color="neutral" variant="link" :aria-label="t('investigations.clearFilters')" @pointerdown.stop.prevent="assessmentFilter = undefined" />
              </template>
            </USelect>
          </div>
        </div>
        <UAlert v-if="loadError" class="m-4 sm:mx-6" :title="t('investigations.loadFailed')" :description="t('common.checkConnection')" color="error" variant="subtle" />
        <div v-else-if="status === 'pending' && !items.length" class="flex items-center gap-2 p-6 text-sm text-muted" role="status">
          <UIcon name="i-lucide-loader-circle" class="size-4 animate-spin" />
          {{ t('investigations.loading') }}
        </div>
        <UEmpty
          v-else-if="!items.length"
          :icon="filtered ? 'i-lucide-list-filter' : 'i-lucide-radar'"
          :title="filtered ? t('investigations.emptyFiltered') : t('investigations.emptyTitle')"
          :description="filtered ? t('investigations.emptyFilteredDescription') : t('investigations.emptyDescription')"
          class="py-10"
        />
        <div v-else class="divide-y divide-default">
          <button
            v-for="item in items"
            :key="item.id"
            type="button"
            class="flex w-full flex-col gap-3 px-4 py-4 text-left transition-colors hover:bg-elevated/40 sm:px-6"
            :class="selectedId === item.id ? 'bg-elevated/50' : ''"
            @click="openInvestigation(item.id)"
          >
            <div class="flex flex-wrap items-start justify-between gap-3">
              <div class="min-w-0">
                <div class="flex items-center gap-2">
                  <span v-if="!item.read" class="size-2 shrink-0 rounded-full bg-primary" :aria-label="t('investigations.unread')" />
                  <h3 class="truncate text-sm font-medium text-highlighted" :class="item.read ? '' : 'font-semibold'">{{ titleOf(item.title) }}</h3>
                </div>
                <p class="mt-1 line-clamp-2 text-sm text-muted">{{ item.summary }}</p>
              </div>
              <div class="flex shrink-0 flex-wrap justify-end gap-1.5">
                <UBadge v-if="item.severity" :label="t(`investigations.severities.${item.severity}`)" :color="severityColor(item.severity)" :variant="item.severity === 'critical' ? 'solid' : 'subtle'" size="sm" />
                <UBadge v-if="item.assessment" :label="t(`investigations.assessments.${item.assessment}`)" :color="assessmentColor(item.assessment)" variant="subtle" size="sm" />
                <UBadge :label="t(`investigations.statuses.${item.status}`)" :color="statusColor(item.status)" variant="subtle" size="sm" />
                <UBadge :label="t(`investigations.states.${item.state}`)" :color="stateColor(item.state)" variant="outline" size="sm" />
              </div>
            </div>
            <p class="text-xs text-dimmed">
              {{ item.ip || t('investigations.unknownSource') }}
              · {{ serviceLabel(item.serviceId) }}
              · {{ formatInvestigationTime(item.lastSeen, uiLocale()) }}
            </p>
            <p v-if="item.status === 'failed' && item.error" class="line-clamp-2 text-xs text-error">{{ item.error }}</p>
            <p v-else-if="item.status === 'detected'" class="text-xs text-dimmed">{{ t('investigations.detectedHint') }}</p>
          </button>
        </div>
        <template v-if="total > limit" #footer>
          <div class="flex flex-wrap items-center justify-between gap-3">
            <p class="text-xs text-muted">{{ t('investigations.showing', { from: fromItem, to: toItem, total: formatDashboardNumber(total) }) }}</p>
            <UPagination v-model:page="page" :total="total" :items-per-page="limit" show-edges />
          </div>
        </template>
      </UCard>

      <DashboardInvestigationDetail
        v-if="selectedId"
        :key="selectedId"
        :id="selectedId"
        :services="services ?? []"
        :ai-enabled="aiEnabled"
        @close="closeInvestigation"
        @changed="onChanged"
      />
    </template>
  </main>
</template>
