<script setup lang="ts">
import type { DashboardRange } from '~/types/dashboard'
import { attacks, blockedSources } from '~/data/dashboard'
definePageMeta({
  layout: 'dashboard'
})
useSeoMeta({
  title: 'Overview · OpenWAF',
  description: 'An example OpenWAF security dashboard with local mock traffic, threats, and request logs.'
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
  multiplier,
  totalRequests,
  totalBlocked,
  metrics
} = useDashboardMetrics(range)
const refreshedAt = ref('14:33:00')
const toast = useToast()
function refresh() {
  refreshedAt.value = new Date().toLocaleTimeString('en-GB')
  toast.add({
    title: 'Mock snapshot refreshed. All data is local.',
    icon: 'i-lucide-circle-check',
    color: 'success'
  })
}
</script>

<template>
  <main id="overview" class="mx-auto flex w-full max-w-7xl flex-col gap-6">
    <div class="flex flex-wrap items-center justify-between gap-4">
      <div>
        <p class="mb-2 text-xs font-medium tracking-widest text-dimmed">
          YOUR SECURITY, AT A GLANCE
        </p>
        <h1 class="text-3xl font-semibold tracking-tight text-highlighted">
          Overview<span class="text-primary">.</span>
        </h1>
        <p class="mt-2 text-sm text-muted">
          A clear view of your traffic and the threats we stop along the way.
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
        <UTooltip text="Refresh mock snapshot">
          <UButton
            icon="i-lucide-refresh-cw"
            color="neutral"
            variant="outline"
            aria-label="Refresh mock snapshot"
            @click="refresh"
          />
        </UTooltip>
      </div>
    </div>
    <UAlert
      title="Looking good. Your applications are protected."
      description="No critical incidents in this mock snapshot."
      icon="i-lucide-shield-check"
      color="success"
      variant="subtle"
    />
    <DashboardMetrics :metrics="metrics" />
    <div class="grid grid-cols-1 gap-6 xl:grid-cols-3">
      <DashboardTrafficChart :range="range" :total-requests="totalRequests" :total-blocked="totalBlocked" />
      <DashboardThreatChart
        :range="range"
        :attacks="attacks"
        :multiplier="multiplier"
        :total-blocked="totalBlocked"
      />
    </div>
    <DashboardRequests />
    <DashboardBlockedSources :sources="blockedSources" :range-label="rangeLabel" :multiplier="multiplier" />
    <footer class="flex flex-wrap items-center justify-between gap-2 pb-2 text-xs text-dimmed">
      <span class="flex items-center gap-2">
        <UIcon name="i-lucide-circle-check" class="size-3.5 text-success" />
        All systems operational · Updated
        {{ refreshedAt }}
      </span>
      <span>
        OpenWAF / Example dashboard · Synthetic data
      </span>
    </footer>
  </main>
</template>
