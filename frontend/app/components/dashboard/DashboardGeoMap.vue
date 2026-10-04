<script setup lang="ts">
import type { CountryRequests } from '~/types/telemetry'
import type { GeoMapMode } from '~/utils/geoMap'

const props = defineProps<{
  countries: CountryRequests[]
  blockedCountries: CountryRequests[]
  totalRequests: number
  blockedRequests: number
}>()
const { t, locale } = useI18n()
const mode = ref<GeoMapMode>('all')
const mapReady = ref(false)
const mapError = ref(false)
const modes = computed(() => [{
  label: t('geo.all'),
  value: 'all' as const
}, {
  label: t('geo.blocked'),
  value: 'blocked' as const
}])
const numberFormat = computed(() => new Intl.NumberFormat(locale.value === 'pl' ? 'pl-PL' : 'en-GB'))
const percentFormat = computed(() => new Intl.NumberFormat(locale.value === 'pl' ? 'pl-PL' : 'en-GB', { style: 'percent', maximumFractionDigits: 1 }))
const regionNames = computed(() => new Intl.DisplayNames([locale.value === 'pl' ? 'pl' : 'en'], { type: 'region' }))
const activeItems = computed(() => mode.value === 'blocked' ? props.blockedCountries : props.countries)
const activeTotal = computed(() => mode.value === 'blocked' ? props.blockedRequests : props.totalRequests)
const ranked = computed(() => activeItems.value.map(item => ({
  code: item.countryCode,
  name: countryName(item.countryCode),
  requests: item.requests,
  share: activeTotal.value > 0 ? item.requests / activeTotal.value : 0
})))
const visible = computed(() => ranked.value.slice(0, 8))
const hiddenCount = computed(() => Math.max(0, ranked.value.length - visible.value.length))
const maximum = computed(() => Math.max(1, ...ranked.value.map(item => item.requests)))
const located = computed(() => mapReady.value && activeItems.value.some(item => mapHasCountry(item.countryCode)))
const option = computed(() => mapReady.value
  ? createGeoMapOption(activeItems.value, mode.value, code => countryName(code), {
      requests: t('geo.requests'),
      format: value => numberFormat.value.format(value)
    })
  : undefined)

function countryName(code: string | null) {
  if (!code) return t('geo.unknown')
  try {
    return regionNames.value.of(code) || code
  } catch {
    return code
  }
}

async function loadMap() {
  mapError.value = false
  try {
    await loadWorldMap()
    mapReady.value = true
  } catch {
    mapReady.value = false
    mapError.value = true
  }
}

onMounted(() => {
  void loadMap()
})
</script>

<template>
  <UCard id="geo" class="scroll-mt-6 overflow-hidden" :ui="{ body: 'p-0 sm:p-0' }">
    <template #header>
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 class="text-sm font-semibold text-highlighted">
            {{ t('geo.title') }}
          </h2>
          <p class="mt-1 text-xs text-muted">
            {{ t('geo.description') }}
          </p>
        </div>
        <UTabs
          v-model="mode"
          :items="modes"
          :content="false"
          size="sm"
          :aria-label="t('geo.mode')"
        />
      </div>
    </template>
    <div class="grid grid-cols-1 sm:grid-cols-[minmax(0,1fr)_17.5rem]">
      <div class="geo-stage relative">
        <VChart
          v-if="option"
          class="geo-chart"
          :option="option"
          autoresize
          :update-options="{ notMerge: true }"
          :aria-label="t('geo.chart', { mode: mode === 'blocked' ? t('geo.blocked') : t('geo.all') })"
        />
        <div v-if="mapError" class="geo-message">
          <p>{{ t('geo.mapFailed') }}</p>
          <UButton
            class="mt-3"
            size="sm"
            color="neutral"
            variant="outline"
            :label="t('common.tryAgain')"
            @click="loadMap"
          />
        </div>
        <p v-else-if="mapReady && !located" class="pointer-events-none absolute inset-x-6 bottom-4 text-center text-sm text-slate-400">
          {{ activeItems.length ? t('geo.unlocated') : t('geo.empty') }}
        </p>
      </div>
      <aside class="flex flex-col gap-4 border-t border-default p-4 sm:border-t-0 sm:border-l" :aria-label="t('geo.top')">
        <h3 class="text-xs font-medium text-muted">
          {{ t('geo.top') }}
        </h3>
        <p v-if="visible.length === 0" class="text-sm text-muted">
          {{ t('geo.empty') }}
        </p>
        <ol v-else class="space-y-3">
          <li v-for="entry in visible" :key="entry.code ?? 'unknown'" class="space-y-1.5">
            <div class="flex items-center justify-between gap-3">
              <p class="flex min-w-0 items-center gap-2">
                <CountryFlag :code="entry.code" :label="entry.name" />
                <span class="truncate text-sm text-highlighted">
                  {{ entry.name }}
                </span>
              </p>
              <span class="shrink-0 text-xs tabular-nums text-muted">
                {{ percentFormat.format(entry.share) }}
              </span>
            </div>
            <div class="h-1 overflow-hidden rounded-full bg-elevated" :aria-hidden="true">
              <div
                class="h-full rounded-full"
                :class="mode === 'blocked' ? 'bg-orange-400' : 'bg-sky-400'"
                :style="{ width: `${entry.requests / maximum * 100}%` }"
              />
            </div>
          </li>
        </ol>
        <p v-if="hiddenCount > 0" class="text-xs text-dimmed">
          {{ t(`geo.more.${pluralForm(hiddenCount)}`, { count: hiddenCount }) }}
        </p>
      </aside>
    </div>
  </UCard>
</template>

<style scoped>
.geo-stage {
  background: #070d16;
}
.geo-chart {
  width: 100%;
  height: auto;
  aspect-ratio: 1.39 / 1;
}
.geo-message {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 1.5rem;
  text-align: center;
  font-size: 0.875rem;
  color: #94a3b8;
  pointer-events: none;
}
.geo-message :deep(button) { pointer-events: auto; }
</style>
