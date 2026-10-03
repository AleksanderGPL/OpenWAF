<script setup lang="ts">
import type { DashboardMetric } from '~/types/dashboard'
defineProps<{
  metrics: DashboardMetric[]
}>()
const { t } = useI18n()
</script>

<template>
  <section class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4" :aria-label="t('metrics.aria')">
    <UCard v-for="metric in metrics" :key="metric.label" :ui="{ body: 'p-5 sm:p-5' }">
      <div class="flex items-center justify-between gap-3">
        <p class="text-sm text-muted">
          {{ metric.label }}
        </p>
        <span class="flex size-8 items-center justify-center rounded-lg" :class="metric.iconClass">
          <UIcon :name="metric.icon" class="size-4" />
        </span>
      </div>
      <p class="mt-3 text-3xl font-semibold tracking-tight text-highlighted">
        {{ metric.value }}
        <span v-if="metric.unit" class="ml-1 text-xl font-normal text-muted">
          {{ metric.unit }}
        </span>
      </p>
      <div class="mt-4 flex items-center justify-between gap-3">
        <div>
          <UBadge
            :label="metric.trend"
            :icon="metric.trendIcon"
            color="success"
            variant="soft"
            size="sm"
          />
          <p class="mt-1.5 text-xs text-dimmed">
            {{ t('metrics.previousPeriod') }}
          </p>
        </div>
        <VChart
          class="metric-sparkline"
          :option="createSparklineOption(metric.data, metric.color)"
          autoresize
          :aria-label="t('metrics.trend', { label: metric.label })"
        />
      </div>
    </UCard>
  </section>
</template>

<style scoped>
.metric-sparkline { width: 88px; height: 36px; flex-shrink: 0; }
</style>
