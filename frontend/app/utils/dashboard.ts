import type { RequestAction } from '~/types/dashboard'
const dashboardNumberFormatter = new Intl.NumberFormat('en-US')
const requestActionColors = {
  Blocked: 'error',
  Allowed: 'success'
} as const
export function formatDashboardNumber(value: number): string {
  return dashboardNumberFormatter.format(value)
}
export function formatLatency(value: number): string {
  return new Intl.NumberFormat('en-US', { maximumFractionDigits: 1 }).format(value)
}
export function formatBytes(value: number): string {
  if (value < 1024) return `${formatDashboardNumber(value)} B`
  if (value < 1024 * 1024) return `${formatLatency(value / 1024)} KB`
  return `${formatLatency(value / 1024 / 1024)} MB`
}
export function getRequestActionColor(action: RequestAction) {
  return requestActionColors[action]
}
