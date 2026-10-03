<script setup lang="ts">
import type { DashboardRange, TrafficPoint, TrafficView } from '~/types/dashboard'
const props = defineProps<{
  range: DashboardRange
  points: TrafficPoint[]
}>()
const { t } = useI18n()
const trafficView = ref<TrafficView>('requests')
const trafficTabs = computed(() => [{
  label: t('traffic.requests'),
  value: 'requests' as const
}, {
  label: t('traffic.bandwidth'),
  value: 'bandwidth' as const
}])
const trafficOption = computed(() => createTrafficChartOption(props.points, trafficView.value, {
  totalRequests: t('metrics.totalRequests'),
  blockedRequests: t('traffic.blockedRequests'),
  requestBody: t('traffic.requestBody'),
  responseBody: t('traffic.responseBody')
}))
</script>

<template>
  <UCard id="traffic" class="scroll-mt-6 xl:col-span-2" :ui="{ body: 'p-4 sm:p-5' }">
    <template #header>
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 class="text-sm font-semibold text-highlighted">
            {{ t('traffic.title') }}
          </h2>
          <p class="mt-1 text-xs text-muted">
            {{ t('traffic.description') }}
          </p>
        </div>
        <UTabs
          v-model="trafficView"
          :items="trafficTabs"
          :content="false"
          size="sm"
          :aria-label="t('traffic.metric')"
        />
      </div>
    </template>
    <div class="mb-2 flex flex-wrap gap-5 text-xs text-muted">
      <span class="flex items-center gap-2">
        <span class="size-2 rounded-full bg-indigo-500" />
        {{ trafficView === 'requests' ? t('metrics.totalRequests') : t('traffic.requestBody') }}
      </span>
      <span class="flex items-center gap-2">
        <span class="size-2 rounded-full bg-amber-500" />
        {{ trafficView === 'requests' ? t('traffic.blockedRequests') : t('traffic.responseBody') }}
      </span>
    </div>
    <VChart
      class="traffic-chart"
      :option="trafficOption"
      autoresize
      :aria-label="t('traffic.chart', { view: trafficView === 'requests' ? t('traffic.requests') : t('traffic.bandwidth'), range: t(range === '24h' ? 'overview.rangeShort24h' : range === '7d' ? 'overview.rangeShort7d' : 'overview.rangeShort30d') })"
    />
  </UCard>
</template>

<style scoped>
.traffic-chart { width: 100%; height: 290px; }
</style>
