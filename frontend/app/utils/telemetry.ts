import type { DashboardMetric, ThreatCategory, BlockedSource, RequestLog, TrafficPoint } from '~/types/dashboard'
import type { BlockedSourceItem, RequestLogRecord, RequestMetrics, StatsSummary, ThreatItem, TrafficResponse } from '~/types/telemetry'
import { formatDashboardNumber, formatLatency } from '~/utils/dashboard'
import { uiLocale } from '~/utils/i18n'
import { formatRuleMessage } from '~/utils/rules'

function formatRate(value: number) {
  return new Intl.NumberFormat('en-US', { maximumFractionDigits: 2 }).format(value)
}

const threatColors = ['#6366f1', '#a78bfa', '#38bdf8', '#fbbf24', '#f87171', '#14b8a6', '#fb7185', '#94a3b8']

type Translate = (key: string) => string

function trend(current: number, previous: number, kind: 'ratio' | 'points' | 'ms', translate: Translate) {
  const delta = current - previous
  const trendIcon = delta > 0 ? 'i-lucide-trending-up' : delta < 0 ? 'i-lucide-trending-down' : 'i-lucide-minus'
  if (kind === 'ratio') {
    if (previous === 0) return { trend: current === 0 ? '0%' : translate('metrics.new'), trendIcon }
    return { trend: `${Math.abs(delta / previous * 100).toFixed(1)}%`, trendIcon }
  }
  if (kind === 'points') return { trend: `${Math.abs(delta).toFixed(2)}%`, trendIcon }
  return { trend: `${formatLatency(Math.abs(delta))}ms`, trendIcon }
}

export function trafficPoints(traffic: TrafficResponse | null | undefined, locale = uiLocale()): TrafficPoint[] {
  return (traffic?.buckets ?? []).map(bucket => {
    const date = new Date(bucket.timestamp)
    const label = traffic?.interval === 'day'
      ? date.toLocaleDateString(locale, { day: 'numeric', month: 'short' })
      : date.toLocaleTimeString(locale, { hour: '2-digit', minute: '2-digit' })
    return {
      label,
      totalRequests: bucket.totalRequests,
      blockedRequests: bucket.blockedRequests,
      blockRate: bucket.blockRate,
      averageLatencyMs: bucket.averageLatencyMs,
      requestBytes: bucket.requestBytes,
      responseBytes: bucket.responseBytes
    }
  })
}

export function dashboardMetrics(summary: StatsSummary | null | undefined, points: TrafficPoint[], translate: Translate): DashboardMetric[] {
  const current: RequestMetrics = summary ?? {
    totalRequests: 0,
    allowedRequests: 0,
    blockedRequests: 0,
    errorRequests: 0,
    blockRate: 0,
    averageLatencyMs: 0,
    requestBytes: 0,
    responseBytes: 0
  }
  const previous = summary?.previous
  const requests = trend(current.totalRequests, previous?.totalRequests ?? 0, 'ratio', translate)
  const blocked = trend(current.blockedRequests, previous?.blockedRequests ?? 0, 'ratio', translate)
  const rate = trend(current.blockRate, previous?.blockRate ?? 0, 'points', translate)
  const latency = trend(current.averageLatencyMs, previous?.averageLatencyMs ?? 0, 'ms', translate)
  const value = (text: string) => summary ? text : '—'
  return [{
    label: translate('metrics.totalRequests'),
    value: value(formatDashboardNumber(current.totalRequests)),
    unit: '',
    icon: 'i-lucide-activity',
    color: '#6366f1',
    iconClass: 'text-primary bg-primary/10',
    ...requests,
    data: points.map(point => point.totalRequests)
  }, {
    label: translate('metrics.blockedThreats'),
    value: value(formatDashboardNumber(current.blockedRequests)),
    unit: '',
    icon: 'i-lucide-shield-check',
    color: '#f87171',
    iconClass: 'text-error bg-error/10',
    ...blocked,
    data: points.map(point => point.blockedRequests)
  }, {
    label: translate('metrics.blockRate'),
    value: value(formatRate(current.blockRate)),
    unit: summary ? '%' : '',
    icon: 'i-lucide-ban',
    color: '#fbbf24',
    iconClass: 'text-warning bg-warning/10',
    ...rate,
    data: points.map(point => point.blockRate)
  }, {
    label: translate('metrics.averageLatency'),
    value: value(formatLatency(current.averageLatencyMs)),
    unit: summary ? 'ms' : '',
    icon: 'i-lucide-zap',
    color: '#14b8a6',
    iconClass: 'text-success bg-success/10',
    ...latency,
    data: points.map(point => point.averageLatencyMs)
  }]
}

export function threatCategories(items: ThreatItem[] | null | undefined, translate: Translate): ThreatCategory[] {
  return (items ?? []).map((item, index) => ({
    name: formatRuleMessage(item.reason || item.ruleId || translate('metrics.unknownRule')),
    value: item.requests,
    color: threatColors[index % threatColors.length]!
  }))
}

export function blockedSourceCards(items: BlockedSourceItem[] | null | undefined, translate: Translate): BlockedSource[] {
  return (items ?? []).map(item => ({
    ip: item.ip,
    country: '',
    code: item.countryCode || '—',
    reason: formatRuleMessage(item.reason || item.ruleId || translate('logs.blocked')),
    requests: item.requests
  }))
}

export function requestLogView(log: RequestLogRecord, locale = uiLocale()): RequestLog {
  const action = log.action === 'blocked' ? 'Blocked' : 'Allowed'
  return {
    id: String(log.id),
    time: new Date(log.timestamp).toLocaleString(locale, { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit', second: '2-digit' }),
    ip: log.ip,
    country: '',
    code: log.countryCode || '—',
    method: log.method,
    path: log.path,
    action,
    rule: formatRuleMessage(log.reason || log.ruleId || '—'),
    status: log.status,
    hostname: log.hostname,
    durationMs: log.durationMs,
    requestBytes: log.requestBytes,
    responseBytes: log.responseBytes
  }
}
