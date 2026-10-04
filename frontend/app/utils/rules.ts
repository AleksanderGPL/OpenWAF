import * as z from 'zod'
import type { Rule, RuleAction, RuleCondition, RuleFormValues, RuleInput, RuleOperator, RuleTarget } from '~/types/rule'

type Translate = (key: string) => string

const targets = ['path', 'query', 'header', 'method', 'ip'] as const
const operators = ['equals', 'contains', 'prefix', 'suffix', 'regex', 'cidr'] as const
const actions = ['block', 'log'] as const

export function isRuleTarget(value: string): value is RuleTarget {
  return (targets as readonly string[]).includes(value)
}

export function isRuleOperator(value: string): value is RuleOperator {
  return (operators as readonly string[]).includes(value)
}

export function isRuleAction(value: string): value is RuleAction {
  return (actions as readonly string[]).includes(value)
}

export function formatRuleMessage(message: string) {
  if (!message.includes('%{')) return message
  return message
    .replace(/\s*\(Total Score: %\{[^}]+\}\)/g, '')
    .replace(/%\{(?:TX\.)?([^}]+)\}/gi, (_, name: string) => name.replace(/_/g, ' ').toLowerCase())
    .replace(/[ ]{2,}/g, ' ')
    .trim()
}

/** Matches backend validatePolicy: only known CRS detection / patch IDs may be disabled. */
export function canDisableCatalogRule(id: number) {
  if (id >= 911000 && id < 949000) return true
  return id >= 1000000
}

function byteLength(value: string) {
  return new TextEncoder().encode(value).length
}

function hasForbiddenControl(value: string) {
  return /[\u0000\r\n]/.test(value)
}

function isIpv4(host: string) {
  const parts = host.split('.')
  if (parts.length !== 4) return false
  return parts.every((part) => {
    if (!/^\d{1,3}$/.test(part) || (part.length > 1 && part.startsWith('0'))) return false
    return Number(part) <= 255
  })
}

function isIpv6(host: string) {
  if (!host.includes(':') || host.includes('%')) return false
  try {
    const parsed = new URL(`http://[${host}]`)
    return parsed.hostname.replace(/^\[|\]$/g, '').length > 0
  } catch {
    return false
  }
}

export function isCidr(value: string) {
  const slash = value.indexOf('/')
  if (slash <= 0 || slash !== value.lastIndexOf('/')) return false
  const address = value.slice(0, slash)
  const prefix = value.slice(slash + 1)
  if (!/^\d{1,3}$/.test(prefix)) return false
  const bits = Number(prefix)
  if (address.includes(':')) return isIpv6(address) && bits <= 128
  return isIpv4(address) && bits <= 32
}

export function operatorsFor(target: RuleTarget): RuleOperator[] {
  if (target === 'ip') return [...operators]
  return operators.filter(operator => operator !== 'cidr')
}

export function blankCondition(): RuleFormValues['conditions'][number] {
  return { target: 'path', key: '', operator: 'contains', value: '' }
}

export function createRuleFormSchema(translate: Translate) {
  const condition = z.object({
    target: z.enum(targets),
    key: z.string(),
    operator: z.enum(operators),
    value: z.string()
  }).superRefine((item, context) => {
    const needsKey = item.target === 'query' || item.target === 'header'
    if (needsKey && (!item.key || byteLength(item.key) > 128 || hasForbiddenControl(item.key))) {
      context.addIssue({ code: 'custom', message: translate('rules.validation.key'), path: ['key'] })
    }
    if (!item.value || byteLength(item.value) > 512 || hasForbiddenControl(item.value)) {
      context.addIssue({ code: 'custom', message: translate('rules.validation.value'), path: ['value'] })
    } else if (item.operator === 'cidr') {
      if (item.target !== 'ip') context.addIssue({ code: 'custom', message: translate('rules.validation.cidrTarget'), path: ['operator'] })
      else if (!isCidr(item.value)) context.addIssue({ code: 'custom', message: translate('rules.validation.cidr'), path: ['value'] })
    } else if (item.operator === 'regex') {
      try {
        RegExp(item.value)
      } catch {
        context.addIssue({ code: 'custom', message: translate('rules.validation.regex'), path: ['value'] })
      }
    }
  })

  return z.object({
    name: z.string().trim().min(1, translate('rules.validation.nameRequired')).refine(
      name => byteLength(name) <= 120,
      translate('rules.validation.nameLength')
    ),
    description: z.string().refine(
      description => byteLength(description) <= 1000,
      translate('rules.validation.noteLength')
    ),
    action: z.enum(actions),
    enabled: z.boolean(),
    scope: z.string().refine(value => value === 'global' || /^[1-9]\d*$/.test(value), translate('rules.validation.scope')),
    conditions: z.array(condition).min(1, translate('rules.validation.conditions')).max(10, translate('rules.validation.conditions'))
  })
}

export function ruleInput(values: RuleFormValues): RuleInput {
  return {
    serviceId: values.scope === 'global' ? null : Number(values.scope),
    name: values.name.trim(),
    description: values.description,
    enabled: values.enabled,
    action: values.action,
    conditions: values.conditions.map(condition => ({
      target: condition.target,
      operator: condition.operator,
      value: condition.value,
      ...(condition.target === 'query' || condition.target === 'header' ? { key: condition.key } : {})
    }))
  }
}

export function formValuesFromRule(rule: Rule): RuleFormValues {
  return {
    name: rule.name,
    description: rule.description ?? '',
    action: rule.action,
    enabled: rule.enabled,
    scope: rule.serviceId == null ? 'global' : String(rule.serviceId),
    conditions: (rule.conditions ?? []).map(condition => ({
      target: condition.target,
      key: condition.key ?? '',
      operator: condition.operator,
      value: condition.value
    }))
  }
}

export function sameRule(left: RuleInput, right: RuleInput) {
  return JSON.stringify(normalizeRule(left)) === JSON.stringify(normalizeRule(right))
}

function normalizeRule(input: RuleInput) {
  return {
    ...input,
    serviceId: input.serviceId ?? null,
    conditions: input.conditions.map(condition => normalizeCondition(condition))
  }
}

function normalizeCondition(condition: RuleCondition) {
  return {
    target: condition.target,
    operator: condition.operator,
    value: condition.value,
    key: condition.target === 'query' || condition.target === 'header' ? condition.key ?? '' : ''
  }
}
