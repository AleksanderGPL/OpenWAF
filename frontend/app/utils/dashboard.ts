import type { RequestAction } from '~/types/dashboard'
import { uiLocale } from '~/utils/i18n'
const requestActionColors = {
  Blocked: 'error',
  Allowed: 'success'
} as const
export function formatDashboardNumber(value: number): string {
  return new Intl.NumberFormat(uiLocale()).format(value)
}
export function formatLatency(value: number): string {
  return new Intl.NumberFormat(uiLocale(), { maximumFractionDigits: 1 }).format(value)
}
export function formatBytes(value: number): string {
  if (value < 1024) return `${formatDashboardNumber(value)} B`
  if (value < 1024 * 1024) return `${formatLatency(value / 1024)} KB`
  return `${formatLatency(value / 1024 / 1024)} MB`
}
export function getRequestActionColor(action: RequestAction) {
  return requestActionColors[action]
}
