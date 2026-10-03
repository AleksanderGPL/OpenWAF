<script setup lang="ts">
import type { DashboardRange, ThreatCategory } from '~/types/dashboard'
const props = defineProps<{
  range: DashboardRange
  attacks: ThreatCategory[]
  totalBlocked: number
}>()
const attackOption = computed(() => createThreatChartOption(props.attacks))
const threatTotal = computed(() => props.attacks.reduce((sum, attack) => sum + attack.value, 0))
</script>

<template>
  <UCard id="threats" class="scroll-mt-6" :ui="{ body: 'p-5 sm:p-5' }">
    <template #header>
      <div class="flex items-center justify-between">
        <div>
          <h2 class="text-sm font-semibold text-highlighted">
            Threat breakdown
          </h2>
          <p class="mt-1 text-xs text-muted">
            What we're keeping out
          </p>
        </div>
        <UBadge :label="range" color="neutral" variant="subtle" />
      </div>
    </template>
    <div class="relative h-44">
      <VChart
        class="threat-chart"
        :option="attackOption"
        autoresize
        aria-label="Blocked threats by attack type"
      />
      <div class="pointer-events-none absolute inset-0 flex flex-col items-center justify-center">
        <strong class="text-2xl font-semibold tracking-tight text-highlighted">
          {{ formatDashboardNumber(totalBlocked) }}
        </strong>
        <span class="mt-1 text-xs text-muted">
          threats blocked
        </span>
      </div>
    </div>
    <div class="mt-5 space-y-3">
      <div v-for="(attack, index) in attacks" :key="`${attack.name}-${index}`" class="flex items-center gap-2 text-xs">
        <span class="size-2 shrink-0 rounded-full" :style="{ backgroundColor: attack.color }" />
        <span class="text-muted">
          {{ attack.name }}
        </span>
        <strong class="ml-auto font-medium text-highlighted">
          {{ formatDashboardNumber(attack.value) }}
        </strong>
        <span class="w-11 text-right text-dimmed">
          {{ threatTotal ? (attack.value / threatTotal * 100).toFixed(1) : '0.0' }}%
        </span>
      </div>
    </div>
  </UCard>
</template>

<style scoped>
.threat-chart { width: 100%; height: 100%; }
</style>
