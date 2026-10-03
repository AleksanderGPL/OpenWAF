<script setup lang="ts">
import * as z from 'zod'

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

const inputUi = { base: 'rounded-lg' }

const registerFields = [{
  name: 'username',
  type: 'text' as const,
  label: 'Username',
  size: 'lg' as const,
  autocomplete: 'username',
  defaultValue: '',
  ui: inputUi
}, {
  name: 'password',
  type: 'password' as const,
  label: 'Password',
  size: 'lg' as const,
  autocomplete: 'new-password',
  defaultValue: '',
  ui: inputUi
}]

const loginFields = [{
  name: 'username',
  type: 'text' as const,
  label: 'Username',
  size: 'lg' as const,
  autocomplete: 'username',
  defaultValue: savedUsername,
  ui: inputUi
}, {
  name: 'password',
  type: 'password' as const,
  label: 'Password',
  size: 'lg' as const,
  autocomplete: 'current-password',
  defaultValue: '',
  ui: inputUi
}, {
  name: 'remember',
  type: 'checkbox' as const,
  label: 'Save username on this device',
  color: 'neutral' as const,
  defaultValue: true,
  class: 'mt-1',
  ui: { label: 'font-normal' }
}]

const formUi = {
  header: 'mb-2',
  form: 'space-y-6',
  footer: 'text-center text-sm text-muted'
}

const registerSchema = z.object({
  username: z.string().trim().min(1, 'Enter a username').max(255, 'Username must be 255 characters or fewer'),
  password: z.string().min(8, 'Password must be at least 8 characters').max(1024, 'Password is too long')
})

const loginSchema = z.object({
  username: z.string().trim().min(1, 'Enter your username').max(255, 'Username must be 255 characters or fewer'),
  password: z.string().min(1, 'Enter your password').max(1024, 'Password is too long'),
  remember: z.boolean()
})

async function onRegister(event: { data: z.output<typeof registerSchema> }) {
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
    errorMessage.value = auth.authErrorMessage(error, registered.value ? 'Account created, but sign-in failed' : 'Could not create the account')
  } finally {
    pending.value = false
  }
}

async function onLogin(event: { data: z.output<typeof loginSchema> }) {
  errorMessage.value = ''
  pending.value = true
  try {
    const username = event.data.username
    await auth.signIn(username, event.data.password)
    auth.rememberUsername(event.data.remember ? username : null)
    await navigateTo('/dash/overview')
  } catch (error) {
    errorMessage.value = auth.authErrorMessage(error, 'Could not sign in')
  } finally {
    pending.value = false
  }
}
</script>

<template>
  <AuthFrame>
    <div v-if="setupError" class="space-y-4 text-center">
      <h1 class="text-3xl font-bold tracking-tight text-highlighted">
        Welcome
      </h1>
      <UAlert
        color="error"
        variant="subtle"
        title="Could not reach the server"
        description="Check that the API is running, then reload this page."
      />
    </div>

    <UAuthForm
      v-else-if="!registered"
      :fields="registerFields"
      :schema="registerSchema"
      :submit="{ label: 'Create account', size: 'xl', class: 'h-12 rounded-lg font-semibold' }"
      :loading="pending"
      novalidate
      :ui="formUi"
      @submit="onRegister"
    >
      <template #header>
        <h1 class="text-center text-3xl font-bold tracking-tight text-highlighted">
          Welcome
        </h1>
        <p class="mt-2 text-center text-muted">
          Create the administrator account for this OpenWAF instance.
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
      :submit="{ label: 'Sign in', size: 'xl', class: 'h-12 rounded-lg font-semibold' }"
      :loading="pending"
      novalidate
      :ui="formUi"
      @submit="onLogin"
    >
      <template #header>
        <h1 class="text-center text-3xl font-bold tracking-tight text-highlighted">
          Sign in to OpenWAF
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
