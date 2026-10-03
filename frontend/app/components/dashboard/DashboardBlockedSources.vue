<script setup lang="ts">
import type { BlockedSource } from '~/types/dashboard'
const props = defineProps<{
  sources: BlockedSource[]
  rangeLabel: string
  multiplier: number
}>()
const maximumRequests = computed(() => Math.max(1, ...props.sources.map(source => source.requests)))
</script>

<template>
  <UCard id="blocked" class="scroll-mt-6">
    <template #header>
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 class="text-sm font-semibold text-highlighted">
            Top blocked sources
          </h2>
          <p class="mt-1 text-xs text-muted">
            Repeat offenders stopped at the edge ·
            {{ rangeLabel.toLowerCase() }}
          </p>
        </div>
        <UBadge
          :label="`${sources.length} blocked IPs`"
          icon="i-lucide-ban"
          color="neutral"
          variant="subtle"
        />
      </div>
    </template>
    <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <UCard
        v-for="(source, index) in sources"
        :key="source.ip"
        variant="subtle"
        :ui="{ body: 'p-4 sm:p-4' }"
      >
        <div class="mb-4 flex items-center justify-between">
          <span class="font-mono text-xs text-dimmed">
            0
            {{ index + 1 }}
          </span>
          <UBadge
            label="Blocked"
            color="error"
            variant="soft"
            size="sm"
          />
        </div>
        <p class="font-mono text-sm font-medium text-highlighted">
          {{ source.ip }}
        </p>
        <p class="mt-2 flex items-center gap-1.5 text-xs text-muted">
          <UBadge
            :label="source.code"
            color="neutral"
            variant="outline"
            size="sm"
          />
          {{ source.country }}
        </p>
        <p class="mt-4 text-xs text-muted">
          {{ source.reason }}
        </p>
        <USeparator class="my-3" />
        <p class="mb-2 text-sm font-medium text-highlighted">
          {{ formatDashboardNumber(source.requests * multiplier) }}
          <span class="text-xs font-normal text-muted">
            requests stopped
          </span>
        </p>
        <UProgress
          :model-value="source.requests / maximumRequests * 100"
          :max="100"
          size="xs"
          :aria-label="`Blocked request volume for ${source.ip}`"
        />
      </UCard>
    </div>
  </UCard>
</template>
