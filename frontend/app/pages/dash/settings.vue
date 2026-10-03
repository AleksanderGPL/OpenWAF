<script setup lang="ts">
import type { FormSubmitEvent } from '@nuxt/ui'
import * as z from 'zod'

definePageMeta({ layout: 'dashboard' })
useSeoMeta({
  title: 'Settings · OpenWAF',
  description: 'Manage request log retention and your dashboard preferences.'
})

type Settings = { logRetentionDays: number }
const auth = useAuthStore()
const { user } = storeToRefs(auth)
const isAdmin = computed(() => user.value?.role === 'admin')
const { data: settings, status, error: loadError, refresh } = await useFetch<Settings>('/api/settings', {
  immediate: isAdmin.value
})
const schema = z.object({
  logRetentionDays: z.coerce.number()
    .int('Enter a whole number of days')
    .min(1, 'Keep logs for at least 1 day')
    .max(3650, 'Keep logs for at most 3650 days')
})
const state = reactive({ logRetentionDays: 30 })
const saving = ref(false)
const saveError = ref('')
const toast = useToast()
const changed = computed(() => !!settings.value && Number(state.logRetentionDays) !== settings.value.logRetentionDays)
const reducingRetention = computed(() => !!settings.value && Number(state.logRetentionDays) < settings.value.logRetentionDays)

watch(settings, value => {
  if (value) state.logRetentionDays = value.logRetentionDays
}, { immediate: true })
watch(() => state.logRetentionDays, () => { saveError.value = '' })

function reset() {
  if (settings.value) state.logRetentionDays = settings.value.logRetentionDays
  saveError.value = ''
}

async function save(event: FormSubmitEvent<z.output<typeof schema>>) {
  if (!isAdmin.value || saving.value || !changed.value) return
  saving.value = true
  saveError.value = ''
  try {
    settings.value = await $fetch<Settings>('/api/settings', {
      method: 'PUT',
      body: { logRetentionDays: event.data.logRetentionDays }
    })
    toast.add({ title: 'Settings saved', description: 'Your log retention policy has been updated.', icon: 'i-lucide-circle-check', color: 'success' })
  } catch (error) {
    saveError.value = auth.authErrorMessage(error, 'Could not save settings. Please try again.')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <main class="mx-auto flex w-full max-w-4xl flex-col gap-6">
    <div>
      <h1 class="text-2xl font-semibold tracking-tight text-highlighted">
        Workspace settings
      </h1>
      <p class="mt-1 text-sm text-muted">
        Manage your request logs and personalize your dashboard.
      </p>
    </div>

    <UCard>
      <template #header>
        <div class="flex items-center gap-3">
          <UIcon name="i-lucide-database" class="size-5 text-primary" />
          <div>
            <h2 class="font-semibold text-highlighted">Request log retention</h2>
            <p class="mt-1 text-sm text-muted">Choose how long OpenWAF keeps request history.</p>
          </div>
        </div>
      </template>

      <UAlert
        v-if="!isAdmin"
        title="Administrator access required"
        description="Only administrators can view and change the workspace retention policy."
        icon="i-lucide-lock"
        color="neutral"
        variant="subtle"
      />
      <div v-else-if="status === 'pending'" class="flex items-center gap-2 text-sm text-muted" role="status">
        <UIcon name="i-lucide-loader-circle" class="size-4 animate-spin" />
        Loading settings…
      </div>
      <div v-else-if="loadError || !settings" class="space-y-4">
        <UAlert title="Could not load settings" description="Check your connection and try again." color="error" variant="subtle" />
        <UButton label="Try again" icon="i-lucide-refresh-cw" color="neutral" variant="outline" @click="refresh()" />
      </div>
      <UForm v-else :schema="schema" :state="state" class="space-y-5" @submit="save">
        <UFormField label="Keep logs for" name="logRetentionDays" description="Enter a whole number between 1 and 3650 days. The default is 30 days." required>
          <div class="flex items-center gap-3">
            <UInput v-model.number="state.logRetentionDays" type="number" :min="1" :max="3650" :step="1" :disabled="saving" class="w-36" />
            <span class="text-sm text-muted">days</span>
          </div>
        </UFormField>
        <p class="text-sm text-muted">
          Expired logs are removed automatically. Dashboard statistics only include retained logs.
        </p>
        <UAlert
          v-if="reducingRetention"
          title="Older logs will be deleted"
          description="Saving a shorter retention period immediately removes logs outside that period. Deleted logs cannot be recovered."
          icon="i-lucide-triangle-alert"
          color="warning"
          variant="subtle"
        />
        <UAlert v-if="saveError" :title="saveError" color="error" variant="subtle" />
        <div class="flex flex-wrap items-center justify-between gap-3 border-t border-default pt-5">
          <span class="text-xs text-muted" aria-live="polite">{{ changed ? 'You have unsaved changes' : 'All changes saved' }}</span>
          <div class="flex gap-2">
            <UButton label="Reset" color="neutral" variant="ghost" :disabled="!changed || saving" @click="reset" />
            <UButton type="submit" label="Save changes" icon="i-lucide-check" :loading="saving" :disabled="!changed" />
          </div>
        </div>
      </UForm>
    </UCard>

    <UCard>
      <template #header>
        <div class="flex items-center gap-3">
          <UIcon name="i-lucide-palette" class="size-5 text-primary" />
          <h2 class="font-semibold text-highlighted">Appearance</h2>
        </div>
      </template>
      <div class="flex flex-wrap items-center justify-between gap-4">
        <div>
          <p class="text-sm font-medium text-highlighted">Color theme</p>
          <p class="mt-1 text-sm text-muted">Use light, dark, or your system theme. Saved on this device.</p>
        </div>
        <UColorModeSelect aria-label="Color theme" class="w-40" />
      </div>
    </UCard>

    <UCard>
      <template #header>
        <div class="flex items-center gap-3">
          <UIcon name="i-lucide-user-round" class="size-5 text-primary" />
          <h2 class="font-semibold text-highlighted">Your account</h2>
        </div>
      </template>
      <dl class="grid gap-5 sm:grid-cols-3">
        <div>
          <dt class="text-xs text-muted">Username</dt>
          <dd class="mt-1 break-words text-sm font-medium text-highlighted">{{ user?.username }}</dd>
        </div>
        <div>
          <dt class="text-xs text-muted">Display name</dt>
          <dd class="mt-1 break-words text-sm font-medium text-highlighted">{{ user?.name || user?.username }}</dd>
        </div>
        <div>
          <dt class="text-xs text-muted">Role</dt>
          <dd class="mt-1"><UBadge :label="isAdmin ? 'Administrator' : 'User'" color="neutral" variant="subtle" /></dd>
        </div>
      </dl>
    </UCard>
  </main>
</template>
