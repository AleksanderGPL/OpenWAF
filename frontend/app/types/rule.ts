export type RuleTarget = 'path' | 'query' | 'header' | 'method' | 'ip'
export type RuleOperator = 'equals' | 'contains' | 'prefix' | 'suffix' | 'regex' | 'cidr'
export type RuleAction = 'block' | 'log'

export interface RuleCondition {
  target: RuleTarget
  key?: string
  operator: RuleOperator
  value: string
}

export interface Rule {
  id: number
  serviceId: number | null
  name: string
  description: string
  enabled: boolean
  action: RuleAction
  conditions: RuleCondition[]
  createdAt: string
  updatedAt: string
}

export interface RuleInput {
  serviceId: number | null
  name: string
  description: string
  enabled: boolean
  action: RuleAction
  conditions: RuleCondition[]
}

export interface RuleConditionForm {
  target: RuleTarget
  key: string
  operator: RuleOperator
  value: string
}

export interface RuleFormValues {
  name: string
  description: string
  action: RuleAction
  enabled: boolean
  scope: string
  conditions: RuleConditionForm[]
}

export interface CatalogRule {
  id: number
  source: string
  file?: string
  message: string
  tags: string[]
}

export interface RulePolicy {
  serviceId?: number | null
  mode: 'blocking' | 'detection' | 'off'
  blockingParanoiaLevel: number
  detectionParanoiaLevel: number
  inboundThreshold: number
  maxBodyBytes: number
  rateLimitPerMinute: number
  rateLimitAction: 'block' | 'log'
  disabledRuleIds: number[]
  updatedAt?: string
}

export interface PolicyView {
  policy: RulePolicy
  inherited: boolean
  configuredPolicy: RulePolicy | null
}

export interface PolicyInput {
  mode: RulePolicy['mode']
  blockingParanoiaLevel: number
  detectionParanoiaLevel: number
  inboundThreshold: number
  maxBodyBytes: number
  rateLimitPerMinute: number
  rateLimitAction: RulePolicy['rateLimitAction']
  disabledRuleIds: number[]
}
