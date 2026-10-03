export type RequestAction = 'Blocked' | 'Allowed'
export interface RequestLog {
  id: string
  time: string
  ip: string
  country: string
  code: string
  method: string
  path: string
  action: RequestAction
  rule: string
  status: number
  hostname: string
  durationMs: number
  requestBytes: number
  responseBytes: number
}
export interface TrafficPoint {
  label: string
  totalRequests: number
  blockedRequests: number
  blockRate: number
  averageLatencyMs: number
  requestBytes: number
  responseBytes: number
}
export type DashboardRange = '24h' | '7d' | '30d'
export type TrafficView = 'requests' | 'bandwidth'
export type RequestFilter = 'All requests' | RequestAction
export type DashboardSection = 'overview' | 'traffic' | 'threats' | 'logs' | 'blocked'
export interface DashboardMetric {
  label: string
  value: string
  unit: string
  icon: string
  color: string
  iconClass: string
  trend: string
  trendIcon: string
  data: number[]
}
export interface ThreatCategory {
  name: string
  value: number
  color: string
}
export interface BlockedSource {
  ip: string
  country: string
  code: string
  reason: string
  requests: number
}
