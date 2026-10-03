<script setup lang="ts">
import type { DashboardRange } from '~/types/dashboard'

definePageMeta({ layout: 'dashboard' })
const { t } = useI18n()
useSeoMeta({
  title: () => t('seo.logsTitle'),
  description: () => t('seo.logsDescription')
})
const range = ref<DashboardRange>('24h')
const ranges = computed(() => [
  { value: '24h', label: t('overview.range24h') },
  { value: '7d', label: t('overview.range7d') },
  { value: '30d', label: t('overview.range30d') }
])
</script>

<template>
  <main class="mx-auto flex w-full max-w-7xl flex-col gap-6">
    <div class="flex flex-wrap items-center justify-between gap-4">
      <USelect
        v-model="range"
        :items="ranges"
        value-key="value"
        icon="i-lucide-clock"
        :aria-label="t('logs.timeRange')"
        class="w-44"
      />
    </div>
    <DashboardRequests :range="range" :refresh-token="0" :page-size="25" show-refresh />
  </main>
</template>
