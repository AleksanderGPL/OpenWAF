<script setup lang="ts">
import type { DashboardRange } from '~/types/dashboard'
definePageMeta({
  layout: 'dashboard'
})
useSeoMeta({
  title: 'Overview · OpenWAF',
  description: 'OpenWAF request statistics, traffic, threats, and request logs.'
})
const range = ref<DashboardRange>('24h')
const ranges = [{
  value: '24h',
  label: 'Last 24 hours'
}, {
  value: '7d',
  label: 'Last 7 days'
}, {
  value: '30d',
  label: 'Last 30 days'
}]
const rangeLabel = computed(() => ranges.find(item => item.value === range.value)?.label ?? '')
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
      title: 'Could not refresh stats',
      icon: 'i-lucide-circle-alert',
      color: 'error'
    })
    return
  }
  toast.add({
    title: 'Stats refreshed',
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
          :label="loadError ? 'Unavailable' : 'Live'"
          :color="loadError ? 'error' : 'success'"
          variant="soft"
          icon="i-lucide-shield-check"
        />
        <p v-if="summary" class="text-sm text-muted">
          {{ formatDashboardNumber(summary.blockedRequests) }} blocked
          ·
          {{ formatDashboardNumber(summary.totalRequests) }} requests
        </p>
      </div>
      <div class="flex items-center gap-2">
        <USelect
          v-model="range"
          :items="ranges"
          value-key="value"
          icon="i-lucide-clock"
          aria-label="Dashboard time range"
          class="w-44"
        />
        <UTooltip text="Refresh stats">
          <UButton
            icon="i-lucide-refresh-cw"
            color="neutral"
            variant="outline"
            aria-label="Refresh stats"
            @click="refresh"
          />
        </UTooltip>
      </div>
    </div>
    <UAlert
      v-if="loadError"
      color="error"
      variant="subtle"
      title="Could not load stats"
      description="Request statistics are unavailable. Try refreshing."
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
        {{ loadError ? 'Status unavailable' : 'Live' }}
        <template v-if="refreshedAt">
          · Updated {{ refreshedAt }}
        </template>
        <template v-if="summary && summary.collectionFailures > 0">
          · {{ formatDashboardNumber(summary.collectionFailures) }} log writes failed
        </template>
      </span>
      <span v-if="summary">
        Logs kept {{ summary.logRetentionDays }} days
      </span>
    </footer>
  </main>
</template>
