import type { RequestAction } from '~/types/dashboard'
const dashboardNumberFormatter = new Intl.NumberFormat('en-US')
const requestActionColors = {
  Blocked: 'error',
  Allowed: 'success',
  Challenged: 'warning'
} as const
export function formatDashboardNumber(value: number): string {
  return dashboardNumberFormatter.format(value)
}
export function getRequestActionColor(action: RequestAction) {
  return requestActionColors[action]
}
