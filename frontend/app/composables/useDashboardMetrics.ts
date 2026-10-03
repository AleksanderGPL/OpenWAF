import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import type { DashboardRange, DashboardMetric } from '~/types/dashboard'
import { attacks, hourlyTraffic, hourlyBlocked } from '~/data/dashboard'
import { formatDashboardNumber } from '~/utils/dashboard'

export function useDashboardMetrics(range: MaybeRefOrGetter<DashboardRange>) {
  const multiplier = computed(() => toValue(range) === '7d' ? 7 : toValue(range) === '30d' ? 30 : 1)
  const totalRequests = computed(() => hourlyTraffic.reduce((sum, value) => sum + value, 0) * multiplier.value)
  const totalBlocked = computed(() => attacks.reduce((sum, attack) => sum + attack.value, 0) * multiplier.value)
  const blockRate = computed(() => (totalBlocked.value / totalRequests.value * 100).toFixed(2))
  const metrics = computed<DashboardMetric[]>(() => [{
    label: 'Total requests',
    value: formatDashboardNumber(totalRequests.value),
    unit: '',
    icon: 'i-lucide-activity',
    color: '#6366f1',
    iconClass: 'text-primary bg-primary/10',
    trend: '12.8%',
    trendIcon: 'i-lucide-trending-up',
    data: hourlyTraffic
  }, {
    label: 'Blocked threats',
    value: formatDashboardNumber(totalBlocked.value),
    unit: '',
    icon: 'i-lucide-shield-check',
    color: '#f87171',
    iconClass: 'text-error bg-error/10',
    trend: '8.2%',
    trendIcon: 'i-lucide-trending-down',
    data: hourlyBlocked
  }, {
    label: 'Block rate',
    value: blockRate.value,
    unit: '%',
    icon: 'i-lucide-ban',
    color: '#fbbf24',
    iconClass: 'text-warning bg-warning/10',
    trend: '0.14%',
    trendIcon: 'i-lucide-trending-down',
    data: [9, 7, 8, 5, 6, 4, 5, 3, 4, 2]
  }, {
    label: 'Average latency',
    value: '24',
    unit: 'ms',
    icon: 'i-lucide-zap',
    color: '#14b8a6',
    iconClass: 'text-success bg-success/10',
    trend: '3ms',
    trendIcon: 'i-lucide-trending-down',
    data: [32, 28, 30, 26, 27, 25, 28, 24, 26, 24]
  }])
  return {
    multiplier,
    totalRequests,
    totalBlocked,
    blockRate,
    metrics
  }
}
