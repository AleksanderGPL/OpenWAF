<script setup lang="ts">
import type { BlockedSource } from '~/types/dashboard'
const props = defineProps<{
  sources: BlockedSource[]
  rangeLabel: string
}>()
const { t } = useI18n()
const maximumRequests = computed(() => Math.max(1, ...props.sources.map(source => source.requests)))
</script>

<template>
  <section id="blocked" class="scroll-mt-6 space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 class="text-sm font-semibold text-highlighted">
          {{ t('sources.title') }}
        </h2>
        <p class="mt-1 text-xs text-muted">
          {{ t('sources.description', { range: rangeLabel }) }}
        </p>
      </div>
      <UBadge
        :label="t(`sources.count.${pluralForm(sources.length)}`, { count: sources.length })"
        icon="i-lucide-ban"
        color="neutral"
        variant="subtle"
      />
    </div>
    <p v-if="sources.length === 0" class="text-sm text-muted">
      {{ t('sources.empty') }}
    </p>
    <div v-else class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <article
        v-for="(source, index) in sources"
        :key="source.ip"
        class="space-y-3 border-t border-default pt-4"
      >
        <div class="flex items-center justify-between">
          <span class="font-mono text-xs text-dimmed">
            {{ String(index + 1).padStart(2, '0') }}
          </span>
          <UBadge
            :label="t('logs.blocked')"
            color="error"
            variant="soft"
            size="sm"
          />
        </div>
        <p class="font-mono text-sm font-medium text-highlighted">
          {{ source.ip }}
        </p>
        <p class="flex items-center gap-1.5 text-xs text-muted">
          <CountryFlag :code="source.code" :label="source.country || source.code" />
          <span v-if="source.country">
            {{ source.country }}
          </span>
        </p>
        <p class="text-xs text-muted">
          {{ source.reason }}
        </p>
        <p class="text-sm font-medium text-highlighted">
          {{ formatDashboardNumber(source.requests) }}
          <span class="text-xs font-normal text-muted">
            {{ t('sources.stopped') }}
          </span>
        </p>
        <UProgress
          :model-value="source.requests / maximumRequests * 100"
          :max="100"
          size="xs"
          :aria-label="t('sources.volume', { ip: source.ip })"
        />
      </article>
    </div>
  </section>
</template>
