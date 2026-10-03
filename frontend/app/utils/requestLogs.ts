import type { RequestLog } from '~/types/dashboard'
export function serializeRequestLogs(logs: readonly RequestLog[]): string {
  const headers = ['Request ID', 'Time', 'IP address', 'Country', 'Method', 'Path', 'Action', 'Rule', 'Status']
  const rows = logs.map(log => [log.id, log.time, log.ip, log.country, log.method, log.path, log.action, log.rule, log.status])
  return [headers, ...rows].map(row => row.map(value => '"' + String(value).replaceAll('"', '""') + '"').join(',')).join('\r\n')
}
export function downloadRequestLogs(logs: readonly RequestLog[]): void {
  if (!import.meta.client) return
  const blob = new Blob([serializeRequestLogs(logs)], {
    type: 'text/csv;charset=utf-8;'
  })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = 'openwaf-mock-request-logs.csv'
  link.click()
  // Keep the URL alive until the browser has started the download.
  setTimeout(() => URL.revokeObjectURL(url), 0)
}
