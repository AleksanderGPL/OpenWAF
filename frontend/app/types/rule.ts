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
