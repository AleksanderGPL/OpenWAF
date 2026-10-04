import type { Ref } from 'vue'
import type { DashboardRange } from '~/types/dashboard'
import type { CountriesResponse, SourcesResponse, StatsSummary, ThreatsResponse, TrafficResponse } from '~/types/telemetry'
import { blockedSourceCards, dashboardMetrics, threatCategories, trafficPoints } from '~/utils/telemetry'

export async function useDashboardStats(range: Ref<DashboardRange>) {
  const query = computed(() => ({ range: range.value }))
  const stats = useFetch<StatsSummary>('/api/stats', { query })
  const traffic = useFetch<TrafficResponse>('/api/stats/traffic', { query })
  const threats = useFetch<ThreatsResponse>('/api/stats/threats', { query })
  const sources = useFetch<SourcesResponse>('/api/stats/blocked-sources', { query })
  const countries = useFetch<CountriesResponse>('/api/stats/countries', { query: computed(() => ({ ...query.value, mode: 'all' })) })
  const blockedCountries = useFetch<CountriesResponse>('/api/stats/countries', { query: computed(() => ({ ...query.value, mode: 'blocked' })) })
  const { t, locale } = useI18n()
  await Promise.all([stats, traffic, threats, sources, countries, blockedCountries])
  const points = computed(() => trafficPoints(traffic.data.value, locale.value === 'pl' ? 'pl-PL' : 'en-GB'))
  const metrics = computed(() => dashboardMetrics(stats.data.value, points.value, key => t(key)))
  const attacks = computed(() => threatCategories(threats.data.value?.items, key => t(key)))
  const blockedSources = computed(() => blockedSourceCards(sources.data.value?.items, key => t(key)))
  const countryItems = computed(() => countries.data.value?.items ?? [])
  const blockedCountryItems = computed(() => blockedCountries.data.value?.items ?? [])
  const countryTotal = computed(() => countries.data.value?.totalRequests ?? 0)
  const blockedCountryTotal = computed(() => blockedCountries.data.value?.totalRequests ?? 0)
  const summary = computed(() => stats.data.value)
  const loadError = computed(() => Boolean(stats.error.value || traffic.error.value || threats.error.value || sources.error.value || countries.error.value || blockedCountries.error.value))
  async function refresh() {
    await Promise.all([stats.refresh(), traffic.refresh(), threats.refresh(), sources.refresh(), countries.refresh(), blockedCountries.refresh()])
  }
  return {
    summary,
    points,
    metrics,
    attacks,
    blockedSources,
    countryItems,
    blockedCountryItems,
    countryTotal,
    blockedCountryTotal,
    loadError,
    refresh
  }
}
