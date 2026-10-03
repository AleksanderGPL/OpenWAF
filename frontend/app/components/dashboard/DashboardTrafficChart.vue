<script setup lang="ts">
import type { DashboardRange, TrafficPoint, TrafficView } from '~/types/dashboard'
const props = defineProps<{
  range: DashboardRange
  points: TrafficPoint[]
}>()
const trafficView = ref<TrafficView>('requests')
const trafficTabs = [{
  label: 'Requests',
  value: 'requests'
}, {
  label: 'Bandwidth',
  value: 'bandwidth'
}]
const trafficOption = computed(() => createTrafficChartOption(props.points, trafficView.value))
</script>

<template>
  <UCard id="traffic" class="scroll-mt-6 xl:col-span-2" :ui="{ body: 'p-4 sm:p-5' }">
    <template #header>
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 class="text-sm font-semibold text-highlighted">
            Traffic overview
          </h2>
          <p class="mt-1 text-xs text-muted">
            Request activity across your applications
          </p>
        </div>
        <UTabs
          v-model="trafficView"
          :items="trafficTabs"
          :content="false"
          size="sm"
          aria-label="Traffic chart metric"
        />
      </div>
    </template>
    <div class="mb-2 flex flex-wrap gap-5 text-xs text-muted">
      <span class="flex items-center gap-2">
        <span class="size-2 rounded-full bg-indigo-500" />
        {{ trafficView === 'requests' ? 'Total requests' : 'Request body' }}
      </span>
      <span class="flex items-center gap-2">
        <span class="size-2 rounded-full bg-amber-500" />
        {{ trafficView === 'requests' ? 'Blocked requests' : 'Response body' }}
      </span>
    </div>
    <VChart
      class="traffic-chart"
      :option="trafficOption"
      autoresize
      :aria-label="`Traffic ${trafficView} for ${range}`"
    />
  </UCard>
</template>

<style scoped>
.traffic-chart { width: 100%; height: 290px; }
</style>
