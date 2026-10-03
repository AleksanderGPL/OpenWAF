export interface RequestMetrics {
  totalRequests: number
  allowedRequests: number
  blockedRequests: number
  errorRequests: number
  blockRate: number
  averageLatencyMs: number
  requestBytes: number
  responseBytes: number
}

export interface TimeWindow {
  from: string
  to: string
}

export interface StatsSummary extends RequestMetrics, TimeWindow {
  previousWindow: TimeWindow
  previous: RequestMetrics
  collectionFailures: number
  collectionStartedAt: string
  logRetentionDays: number
}

export interface TrafficBucket extends RequestMetrics {
  timestamp: string
}

export interface TrafficResponse extends TimeWindow {
  interval: 'hour' | 'day'
  buckets: TrafficBucket[]
}

export interface ThreatItem {
  ruleId: string
  reason: string
  requests: number
}

export interface ThreatsResponse extends TimeWindow {
  items: ThreatItem[]
}

export interface BlockedSourceItem {
  ip: string
  countryCode: string | null
  requests: number
  ruleId: string
  reason: string
  lastSeen: string
}

export interface SourcesResponse extends TimeWindow {
  items: BlockedSourceItem[]
}

export interface RequestLogRecord {
  id: number
  timestamp: string
  serviceId: number | null
  hostname: string
  ip: string
  countryCode: string | null
  method: string
  path: string
  action: 'allowed' | 'blocked'
  ruleId: string
  reason: string
  status: number
  errorCategory: string
  durationMs: number
  requestBytes: number
  responseBytes: number
}

export interface LogPage {
  items: RequestLogRecord[]
  total: number
  page: number
  limit: number
}
