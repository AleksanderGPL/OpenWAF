import * as z from 'zod'
import type { Service, ServiceInput } from '~/types/service'

type Translate = (key: string) => string
export type UpstreamIssue = '' | 'required' | 'port' | 'emptyPort' | 'invalid'

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

export function isHostname(value: string) {
  const host = value.trim().toLowerCase().replace(/\.$/, '')
  if (!host || host.length > 253) return false
  if (isIpv4(host) || isIpv6(host)) return true
  if (host.includes(':') || host.includes('/')) return false
  return host.split('.').every((label) => {
    if (!label || label.length > 63 || label.startsWith('-') || label.endsWith('-')) return false
    return /^[a-z0-9-]+$/.test(label)
  })
}

const upstreamIssueKeys: Record<Exclude<UpstreamIssue, ''>, string> = {
  required: 'services.validation.upstreamRequired',
  port: 'services.validation.portRange',
  emptyPort: 'services.validation.emptyPort',
  invalid: 'services.validation.upstreamInvalid'
}

export function upstreamIssue(value: string): UpstreamIssue {
  const raw = value.trim()
  if (!raw) return 'required'
  const explicitPort = raw.match(/:(\d+)(?:\/)?$/)
  if (explicitPort) {
    const port = Number(explicitPort[1])
    if (!Number.isInteger(port) || port < 1 || port > 65535) return 'port'
  }
  if (raw.endsWith(':')) return 'emptyPort'
  let target: URL
  try {
    target = new URL(raw)
  } catch {
    return 'invalid'
  }
  if ((target.protocol !== 'http:' && target.protocol !== 'https:') || target.username || target.password || (target.pathname !== '' && target.pathname !== '/') || target.search || target.hash) {
    return 'invalid'
  }
  if (target.port) {
    const port = Number(target.port)
    if (!Number.isInteger(port) || port < 1 || port > 65535) return 'port'
  }
  const host = target.hostname.replace(/^\[|\]$/g, '')
  if (!isHostname(host)) return 'invalid'
  return ''
}

export function createServiceFormSchema(translate: Translate) {
  return z.object({
    name: z.string().trim().min(1, translate('services.validation.nameRequired')).refine(
      name => new TextEncoder().encode(name).length <= 255,
      translate('services.validation.nameLength')
    ),
    hostname: z.string().trim().min(1, translate('services.validation.hostname')).refine(isHostname, translate('services.validation.hostname')),
    upstreamUrl: z.string().trim().superRefine((value, context) => {
      const issue = upstreamIssue(value)
      if (issue) context.addIssue(translate(upstreamIssueKeys[issue]))
    }),
    skipTlsVerify: z.boolean(),
    enabled: z.boolean()
  })
}

export type ServiceFormValues = {
  name: string
  hostname: string
  upstreamUrl: string
  skipTlsVerify: boolean
  enabled: boolean
}

export function serviceInput(service: Pick<Service, 'name' | 'hostname' | 'upstreamUrl' | 'skipTlsVerify' | 'enabled'>): ServiceInput {
  return {
    name: service.name,
    hostname: service.hostname,
    upstreamUrl: service.upstreamUrl,
    skipTlsVerify: service.skipTlsVerify,
    enabled: service.enabled
  }
}

export function usesTls(upstreamUrl: string) {
  return upstreamUrl.trim().toLowerCase().startsWith('https:')
}
