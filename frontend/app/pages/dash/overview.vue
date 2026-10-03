<script setup lang="ts">
import type { DashboardRange } from '~/types/dashboard'
definePageMeta({
  layout: 'dashboard'
})
const { t } = useI18n()
useSeoMeta({
  title: () => t('seo.overviewTitle'),
  description: () => t('seo.overviewDescription')
})
const range = ref<DashboardRange>('24h')
const ranges = computed(() => [{
  value: '24h' as const,
  label: t('overview.range24h')
}, {
  value: '7d' as const,
  label: t('overview.range7d')
}, {
  value: '30d' as const,
  label: t('overview.range30d')
}])
const rangeLabel = computed(() => ranges.value.find(item => item.value === range.value)?.label ?? '')
const {
  summary,
  points,
  metrics,
  attacks,
  blockedSources,
  loadError,
  refresh: refreshStats
} = await useDashboardStats(range)
const refreshToken = ref(0)
const refreshedAt = ref('')
watch(summary, value => {
  if (value) refreshedAt.value = new Date().toLocaleTimeString('en-GB')
}, { immediate: true })
const toast = useToast()
async function refresh() {
  refreshToken.value += 1
  await refreshStats()
  if (loadError.value) {
    toast.add({
      title: t('overview.refreshFailed'),
      icon: 'i-lucide-circle-alert',
      color: 'error'
    })
    return
  }
  toast.add({
    title: t('overview.refreshed'),
    icon: 'i-lucide-circle-check',
    color: 'success'
  })
}
</script>

<template>
  <main id="overview" class="mx-auto flex w-full max-w-7xl flex-col gap-6">
    <div class="flex flex-wrap items-center justify-between gap-4">
      <div class="flex items-center gap-3">
        <UBadge
          :label="loadError ? t('overview.unavailable') : t('overview.live')"
          :color="loadError ? 'error' : 'success'"
          variant="soft"
          icon="i-lucide-shield-check"
        />
        <p v-if="summary" class="text-sm text-muted">
          {{ t('overview.summary', { blocked: formatDashboardNumber(summary.blockedRequests), requests: formatDashboardNumber(summary.totalRequests) }) }}
        </p>
      </div>
      <div class="flex items-center gap-2">
        <USelect
          v-model="range"
          :items="ranges"
          value-key="value"
          icon="i-lucide-clock"
          :aria-label="t('overview.timeRange')"
          class="w-44"
        />
        <UTooltip :text="t('overview.refresh')">
          <UButton
            icon="i-lucide-refresh-cw"
            color="neutral"
            variant="outline"
            :aria-label="t('overview.refresh')"
            @click="refresh"
          />
        </UTooltip>
      </div>
    </div>
    <UAlert
      v-if="loadError"
      color="error"
      variant="subtle"
      :title="t('overview.loadFailed')"
      :description="t('overview.loadFailedDescription')"
    />
    <DashboardMetrics :metrics="metrics" />
    <div class="grid grid-cols-1 gap-6 xl:grid-cols-3">
      <DashboardTrafficChart :range="range" :points="points" />
      <DashboardThreatChart :range="range" :attacks="attacks" :total-blocked="summary?.blockedRequests ?? 0" />
    </div>
    <DashboardRequests :range="range" :refresh-token="refreshToken" />
    <DashboardBlockedSources :sources="blockedSources" :range-label="rangeLabel" />
    <footer class="flex flex-wrap items-center justify-between gap-2 pb-2 text-xs text-dimmed">
      <span class="flex items-center gap-2">
        <UIcon name="i-lucide-circle-check" class="size-3.5 text-success" />
        {{ loadError ? t('overview.statusUnavailable') : t('overview.live') }}
        <template v-if="refreshedAt">
          {{ t('overview.updated', { time: refreshedAt }) }}
        </template>
        <template v-if="summary && summary.collectionFailures > 0">
          {{ t(`overview.logWritesFailed.${pluralForm(summary.collectionFailures)}`, { count: formatDashboardNumber(summary.collectionFailures) }) }}
        </template>
      </span>
      <span v-if="summary">
        {{ t('overview.logsKept', { days: summary.logRetentionDays }) }}
      </span>
    </footer>
  </main>
</template>
