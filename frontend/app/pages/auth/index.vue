<script setup lang="ts">
import * as z from 'zod'

const { t, locale, locales, setLocale } = useI18n()
const selectedLocale = computed({
  get: () => locale.value,
  set: (code: string) => {
    setLocale(code as typeof locale.value)
  }
})
const localeOptions = computed(() => locales.value.map(item => ({
  code: item.code,
  name: item.name ?? item.code,
  dir: item.dir === 'rtl' ? 'rtl' as const : 'ltr' as const,
  messages: {}
})))
const auth = useAuthStore()
const { user } = storeToRefs(auth)
const { data: setup, error: setupError } = await useFetch<{ completed: boolean }>('/api/auth/setup')

const pending = ref(false)
const errorMessage = ref('')
const registered = ref(!!setup.value?.completed)
const savedUsername = auth.rememberedUsername()

await auth.fetchUser()
if (user.value) {
  await navigateTo('/dash/overview')
}

const registerFields = computed(() => [{
  name: 'username',
  type: 'text' as const,
  label: t('auth.username'),
  size: 'lg' as const,
  autocomplete: 'username',
  defaultValue: ''
}, {
  name: 'password',
  type: 'password' as const,
  label: t('auth.password'),
  size: 'lg' as const,
  autocomplete: 'new-password',
  defaultValue: ''
}])

const loginFields = computed(() => [{
  name: 'username',
  type: 'text' as const,
  label: t('auth.username'),
  size: 'lg' as const,
  autocomplete: 'username',
  defaultValue: savedUsername
}, {
  name: 'password',
  type: 'password' as const,
  label: t('auth.password'),
  size: 'lg' as const,
  autocomplete: 'current-password',
  defaultValue: ''
}, {
  name: 'remember',
  type: 'checkbox' as const,
  label: t('auth.rememberUsername'),
  color: 'neutral' as const,
  defaultValue: true,
  class: 'mt-1',
  ui: { label: 'font-normal' }
}])

const formUi = {
  header: 'mb-2',
  form: 'space-y-6',
  footer: 'text-center text-sm text-muted'
}

const registerSchema = computed(() => z.object({
  username: z.string().trim().min(1, t('auth.usernameRequired')).max(255, t('auth.usernameLength')),
  password: z.string().min(8, t('auth.passwordLength')).max(1024, t('auth.passwordTooLong'))
}))

const loginSchema = computed(() => z.object({
  username: z.string().trim().min(1, t('auth.usernameSignInRequired')).max(255, t('auth.usernameLength')),
  password: z.string().min(1, t('auth.passwordRequired')).max(1024, t('auth.passwordTooLong')),
  remember: z.boolean()
}))

async function onRegister(event: { data: { username: string, password: string } }) {
  errorMessage.value = ''
  pending.value = true
  const username = event.data.username
  try {
    await auth.signUp({ username, password: event.data.password, name: '' })
    registered.value = true
    await auth.signIn(username, event.data.password)
    auth.rememberUsername(username)
    await navigateTo('/dash/overview')
  } catch (error) {
    errorMessage.value = auth.authErrorMessage(error, registered.value ? t('auth.createdButSignInFailed') : t('auth.createFailed'))
  } finally {
    pending.value = false
  }
}

async function onLogin(event: { data: { username: string, password: string, remember: boolean } }) {
  errorMessage.value = ''
  pending.value = true
  try {
    const username = event.data.username
    await auth.signIn(username, event.data.password)
    auth.rememberUsername(event.data.remember ? username : null)
    await navigateTo('/dash/overview')
  } catch (error) {
    errorMessage.value = auth.authErrorMessage(error, t('auth.signInFailed'))
  } finally {
    pending.value = false
  }
}
</script>

<template>
  <AuthFrame>
    <div class="mb-6 flex justify-end">
      <ULocaleSelect v-model="selectedLocale" :locales="localeOptions" class="w-40" :aria-label="t('settings.language')" />
    </div>
    <div v-if="setupError" class="space-y-4 text-center">
      <h1 class="text-3xl font-bold tracking-tight text-highlighted">
        {{ t('auth.welcome') }}
      </h1>
      <UAlert
        color="error"
        variant="subtle"
        :title="t('auth.serverUnreachable')"
        :description="t('auth.serverUnreachableDescription')"
      />
    </div>

    <UAuthForm
      v-else-if="!registered"
      :fields="registerFields"
      :schema="registerSchema"
      :submit="{ label: t('auth.createAccount'), size: 'xl', class: 'h-12 font-semibold' }"
      :loading="pending"
      novalidate
      :ui="formUi"
      @submit="onRegister"
    >
      <template #header>
        <h1 class="text-center text-3xl font-bold tracking-tight text-highlighted">
          {{ t('auth.welcome') }}
        </h1>
        <p class="mt-2 text-center text-muted">
          {{ t('auth.createAdmin') }}
        </p>
      </template>

      <template #validation>
        <UAlert
          v-if="errorMessage"
          color="error"
          variant="subtle"
          :title="errorMessage"
        />
      </template>
    </UAuthForm>

    <UAuthForm
      v-else
      :fields="loginFields"
      :schema="loginSchema"
      :submit="{ label: t('auth.signIn'), size: 'xl', class: 'h-12 font-semibold' }"
      :loading="pending"
      novalidate
      :ui="formUi"
      @submit="onLogin"
    >
      <template #header>
        <h1 class="text-center text-3xl font-bold tracking-tight text-highlighted">
          {{ t('auth.signInTitle') }}
        </h1>
      </template>

      <template #validation>
        <UAlert
          v-if="errorMessage"
          color="error"
          variant="subtle"
          :title="errorMessage"
        />
      </template>
    </UAuthForm>
  </AuthFrame>
</template>
