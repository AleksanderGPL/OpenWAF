import type { RequestLogRecord } from './telemetry'

export const investigationStates = ['open', 'acknowledged', 'resolved', 'dismissed'] as const
export const investigationStatuses = ['detected', 'queued', 'running', 'completed', 'failed', 'cancelled'] as const
export const investigationSeverities = ['critical', 'high', 'medium', 'low', 'info'] as const
export const investigationAssessments = ['likely_malicious', 'likely_benign', 'inconclusive'] as const
export const investigationDetectors = ['repeated_blocks', 'repeated_security_matches', 'path_scanning', 'error_surge', 'traffic_surge'] as const

export type InvestigationState = typeof investigationStates[number]
export type InvestigationStatus = typeof investigationStatuses[number]
export type InvestigationSeverity = typeof investigationSeverities[number]
export type InvestigationAssessment = typeof investigationAssessments[number]
export type InvestigationDetector = typeof investigationDetectors[number]

export interface InvestigationTrigger {
  detector: string
  explanation: string
  from: string
  to: string
  observed: number
  threshold: number
  baseline?: number
  requestIds?: number[]
  coverage: string
}

export interface InvestigationCard {
  id: string
  serviceId: number
  ip?: string
  title: string
  summary: string
  severity?: InvestigationSeverity | ''
  assessment?: InvestigationAssessment | ''
  state: InvestigationState
  status: InvestigationStatus
  read: boolean
  trigger: InvestigationTrigger
  firstSeen: string
  lastSeen: string
  startedAt?: string | null
  finishedAt?: string | null
  cancellationRequested: boolean
  error?: string
  resultAvailable: boolean
  resultVersion: number
}

export interface EvidenceRuleMatch {
  ruleId: string
  source: string
  message: string
  severity?: string
  tags?: string[]
  variables?: string[]
}

export interface EvidenceRequest extends RequestLogRecord {
  ruleMatches?: EvidenceRuleMatch[]
  securitySignals?: string[]
}

export interface InvestigationEvidence {
  request: EvidenceRequest
  available: boolean
}

export interface InvestigationResult {
  title: string
  summary: string
  severity: InvestigationSeverity
  assessment: InvestigationAssessment
  explanation: string
  patterns: string[] | null
  recommendations: string[] | null
  limitations: string[] | null
  trigger: InvestigationTrigger
  publishedAt: string
  evidence: InvestigationEvidence[] | null
}

export interface InvestigationEvent {
  id: number
  investigationId: string
  type: string
  activityId?: string
  attempt: number
  provisional: boolean
  activity?: string
  delta?: string
  createdAt: string
}

export interface InvestigationDetail extends InvestigationCard {
  result: InvestigationResult | null
  events: InvestigationEvent[]
  lastEventId: number
  eventsTruncated: boolean
}

export interface InvestigationList {
  items: InvestigationCard[]
  total: number
  page: number
  limit: number
  lastEventId: number
}

export interface InvestigationSummary {
  total: number
  unread: number
  bySeverity: Record<string, number>
  byStatus: Record<string, number>
  lastEventId: number
}

export interface AnomalyPolicy {
  enabled: boolean
  blockedThreshold: number
  suspiciousThreshold: number
  scanRequests: number
  scanPaths: number
  errorThreshold: number
  errorPercent: number
  surgeMinimum: number
  surgeMultiplier: number
  cooldownMinutes: number
  hourlyRunLimit: number
}

export interface AnomalySettingsView {
  policy: AnomalyPolicy & { scope?: number }
  inherited: boolean
  aiEnabled: boolean
}
