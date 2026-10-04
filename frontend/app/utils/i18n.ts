export function uiLocale() {
  return useNuxtApp().$i18n.locale.value === 'pl' ? 'pl-PL' : 'en-GB'
}

export function pluralForm(count: number) {
  const code = useNuxtApp().$i18n.locale.value === 'pl' ? 'pl' : 'en'
  const form = new Intl.PluralRules(code).select(count)
  if (form === 'one' || form === 'few' || form === 'many') return form
  return 'other'
}

const apiErrorKeys: Record<string, string> = {
  'Unauthorized': 'errors.unauthorized',
  'Invalid username or password': 'errors.invalidCredentials',
  'Setup has already been completed': 'errors.setupCompleted',
  'Username is already taken': 'errors.usernameTaken',
  'A service with this hostname already exists': 'errors.hostnameTaken',
  'Request log not found': 'errors.logNotFound',
  'Not Found': 'errors.notFound',
  'Forbidden': 'errors.forbidden',
  'Internal server error': 'errors.internal',
  'A name (up to 255 bytes) and a valid hostname or IP address without a port are required': 'errors.serviceInput',
  'Upstream URL must be an HTTP or HTTPS origin without credentials, a path, query, or fragment': 'services.validation.upstreamInvalid',
  'Upstream port must be between 1 and 65535': 'services.validation.portRange',
  'Upstream port cannot be empty': 'services.validation.emptyPort',
  'Invalid service ID': 'errors.invalidServiceId',
  'logRetentionDays must be between 1 and 3650': 'errors.retentionRange',
  'A rule requires a name (1..120 bytes), description up to 1000 bytes and 1..10 conditions': 'errors.ruleInput',
  'action must be block or log': 'errors.ruleAction',
  'key is only supported for query and header conditions': 'errors.ruleKeyUnused',
  'query and header conditions require a valid key': 'rules.validation.key',
  'Unknown condition target': 'errors.ruleTarget',
  'Condition value must be 1..512 bytes without control characters': 'rules.validation.value',
  'Invalid regular expression': 'rules.validation.regex',
  'cidr only supports the ip target': 'rules.validation.cidrTarget',
  'Invalid CIDR': 'rules.validation.cidr',
  'Unknown condition operator': 'errors.ruleOperator',
  'serviceId must be positive or null': 'rules.validation.scope',
  'serviceId must be a positive integer': 'rules.validation.scope',
  'At most 500 custom rules are permitted': 'errors.ruleLimit',
  'A rule\'s scope cannot be changed; create a new rule': 'errors.ruleScopeLocked',
  'Invalid rule ID': 'errors.invalidRuleId',
  'An investigation is already active': 'investigations.alreadyActive',
  'The result version is no longer available': 'investigations.versionGone',
  'AI investigation is not configured': 'investigations.aiNotConfigured',
  'Wait for the investigation to finish or cancel it before following up': 'investigations.followUpWaiting',
  'Investigation context is too large': 'investigations.contextTooLarge',
  'thresholds must be between 1 and 1000000': 'anomaly.validation.threshold',
  'invalid anomaly settings': 'anomaly.invalid',
  'global settings cannot be deleted': 'anomaly.globalLocked',
  'page must be 1–10000 and limit 1–100': 'errors.pagination'
}

export function translateApiMessage(message: string) {
  const key = apiErrorKeys[message]
  return key ? useNuxtApp().$i18n.t(key) : message
}
