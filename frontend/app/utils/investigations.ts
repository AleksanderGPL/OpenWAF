import type { InvestigationEvent, InvestigationSeverity, InvestigationStatus } from '../types/investigation'

export interface ExplanationBlock {
  kind: 'explanation'
  id: number
  attempt: number
  text: string
  provisional: boolean
  createdAt: string
}

export interface MilestoneBlock {
  kind: 'event'
  event: InvestigationEvent
}

export type ProgressBlock = ExplanationBlock | MilestoneBlock

const severityColors: Record<InvestigationSeverity, 'error' | 'warning' | 'success' | 'info' | 'neutral'> = {
  critical: 'error',
  high: 'error',
  medium: 'warning',
  low: 'success',
  info: 'info'
}

const statusColors: Record<InvestigationStatus, 'error' | 'warning' | 'success' | 'info' | 'primary' | 'neutral'> = {
  detected: 'neutral',
  queued: 'info',
  running: 'primary',
  completed: 'success',
  failed: 'error',
  cancelled: 'warning'
}

const stateColors = {
  open: 'primary',
  acknowledged: 'info',
  resolved: 'success',
  dismissed: 'neutral'
} as const

const assessmentColors = {
  likely_malicious: 'error',
  likely_benign: 'success',
  inconclusive: 'warning'
} as const

export function severityColor(severity?: string) {
  return severityColors[severity as InvestigationSeverity] ?? 'neutral'
}

export function statusColor(status?: string) {
  return statusColors[status as InvestigationStatus] ?? 'neutral'
}

export function stateColor(state?: string) {
  return stateColors[state as keyof typeof stateColors] ?? 'neutral'
}

export function assessmentColor(assessment?: string) {
  return assessmentColors[assessment as keyof typeof assessmentColors] ?? 'neutral'
}

export function mergeInvestigationEvents(current: InvestigationEvent[], incoming: InvestigationEvent[]) {
  const byId = new Map<number, InvestigationEvent>()
  for (const event of current) byId.set(event.id, event)
  for (const event of incoming) byId.set(event.id, event)
  return [...byId.values()].sort((left, right) => left.id - right.id)
}

export function groupInvestigationProgress(events: InvestigationEvent[]): ProgressBlock[] {
  const blocks: ProgressBlock[] = []
  for (const event of events) {
    if (event.type !== 'explanation.delta') {
      blocks.push({ kind: 'event', event })
      continue
    }
    const previous = blocks.at(-1)
    if (previous?.kind === 'explanation' && previous.attempt === event.attempt) {
      previous.text += event.delta ?? ''
      previous.provisional = event.provisional
      previous.id = event.id
      continue
    }
    blocks.push({
      kind: 'explanation',
      id: event.id,
      attempt: event.attempt,
      text: event.delta ?? '',
      provisional: event.provisional,
      createdAt: event.createdAt
    })
  }
  return blocks
}

export function formatInvestigationTime(value: string | null | undefined, locale: string) {
  if (!value || value.startsWith('0001-')) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return date.toLocaleString(locale, { dateStyle: 'medium', timeStyle: 'short' })
}

export function investigationRangeStart(range: 'all' | '24h' | '7d' | '30d', now = Date.now()) {
  const day = 24 * 60 * 60 * 1000
  if (range === '24h') return new Date(now - day).toISOString()
  if (range === '7d') return new Date(now - 7 * day).toISOString()
  if (range === '30d') return new Date(now - 30 * day).toISOString()
  return undefined
}
