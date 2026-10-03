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
  'logRetentionDays must be between 1 and 3650': 'errors.retentionRange'
}

export function translateApiMessage(message: string) {
  const key = apiErrorKeys[message]
  return key ? useNuxtApp().$i18n.t(key) : message
}
