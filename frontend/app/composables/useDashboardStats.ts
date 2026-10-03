import type { Ref } from 'vue'
import type { DashboardRange } from '~/types/dashboard'
import type { SourcesResponse, StatsSummary, ThreatsResponse, TrafficResponse } from '~/types/telemetry'
import { blockedSourceCards, dashboardMetrics, threatCategories, trafficPoints } from '~/utils/telemetry'

export async function useDashboardStats(range: Ref<DashboardRange>) {
  const query = computed(() => ({ range: range.value }))
  const stats = useFetch<StatsSummary>('/api/stats', { query })
  const traffic = useFetch<TrafficResponse>('/api/stats/traffic', { query })
  const threats = useFetch<ThreatsResponse>('/api/stats/threats', { query })
  const sources = useFetch<SourcesResponse>('/api/stats/blocked-sources', { query })
  await Promise.all([stats, traffic, threats, sources])
  const points = computed(() => trafficPoints(traffic.data.value))
  const metrics = computed(() => dashboardMetrics(stats.data.value, points.value))
  const attacks = computed(() => threatCategories(threats.data.value?.items))
  const blockedSources = computed(() => blockedSourceCards(sources.data.value?.items))
  const summary = computed(() => stats.data.value)
  const loadError = computed(() => Boolean(stats.error.value || traffic.error.value || threats.error.value || sources.error.value))
  async function refresh() {
    await Promise.all([stats.refresh(), traffic.refresh(), threats.refresh(), sources.refresh()])
  }
  return {
    summary,
    points,
    metrics,
    attacks,
    blockedSources,
    loadError,
    refresh
  }
}
