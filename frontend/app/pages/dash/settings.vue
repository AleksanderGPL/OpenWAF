<script setup lang="ts">
import type { FormSubmitEvent } from '@nuxt/ui'
import * as z from 'zod'

definePageMeta({ layout: 'dashboard' })
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
useSeoMeta({
  title: () => t('seo.settingsTitle'),
  description: () => t('seo.settingsDescription')
})

type Settings = { logRetentionDays: number }
const auth = useAuthStore()
const { user } = storeToRefs(auth)
const isAdmin = computed(() => user.value?.role === 'admin')
const { data: settings, status, error: loadError, refresh } = await useFetch<Settings>('/api/settings', {
  immediate: isAdmin.value
})
const schema = computed(() => z.object({
  logRetentionDays: z.coerce.number()
    .int(t('settings.daysWhole'))
    .min(1, t('settings.daysMin'))
    .max(3650, t('settings.daysMax'))
}))
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

async function save(event: FormSubmitEvent<{ logRetentionDays: number }>) {
  if (!isAdmin.value || saving.value || !changed.value) return
  saving.value = true
  saveError.value = ''
  try {
    settings.value = await $fetch<Settings>('/api/settings', {
      method: 'PUT',
      body: { logRetentionDays: event.data.logRetentionDays }
    })
    toast.add({ title: t('settings.saved'), description: t('settings.savedDescription'), icon: 'i-lucide-circle-check', color: 'success' })
  } catch (error) {
    saveError.value = auth.authErrorMessage(error, t('settings.saveFailed'))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <main class="mx-auto flex w-full max-w-4xl flex-col gap-4">
    <section class="space-y-5 rounded-xl border border-default bg-default p-5 shadow-sm sm:p-6">
      <header class="flex items-start gap-3">
        <span class="mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
          <UIcon name="i-lucide-database" class="size-4" />
        </span>
        <div>
          <h2 class="text-lg font-semibold text-highlighted">{{ t('settings.retentionTitle') }}</h2>
          <p class="mt-1 text-sm text-muted">{{ t('settings.retentionDescription') }}</p>
        </div>
      </header>

      <UAlert
        v-if="!isAdmin"
        :title="t('settings.adminRequired')"
        :description="t('settings.adminRetention')"
        icon="i-lucide-lock"
        color="neutral"
        variant="subtle"
      />
      <div v-else-if="status === 'pending'" class="flex items-center gap-2 text-sm text-muted" role="status">
        <UIcon name="i-lucide-loader-circle" class="size-4 animate-spin" />
        {{ t('settings.loading') }}
      </div>
      <div v-else-if="loadError || !settings" class="space-y-4">
        <UAlert :title="t('settings.loadFailed')" :description="t('common.checkConnection')" color="error" variant="subtle" />
        <UButton :label="t('common.tryAgain')" icon="i-lucide-refresh-cw" color="neutral" variant="outline" @click="refresh()" />
      </div>
      <UForm v-else :schema="schema" :state="state" class="space-y-5" @submit="save">
        <UFormField :label="t('settings.keepLogs')" name="logRetentionDays" :description="t('settings.keepLogsDescription')" required>
          <div class="flex items-center gap-3">
            <UInput v-model.number="state.logRetentionDays" type="number" :min="1" :max="3650" :step="1" :disabled="saving" class="w-36" />
            <span class="text-sm text-muted">{{ t('common.days') }}</span>
          </div>
        </UFormField>
        <p class="text-sm text-muted">
          {{ t('settings.retentionNote') }}
        </p>
        <UAlert
          v-if="reducingRetention"
          :title="t('settings.reducingTitle')"
          :description="t('settings.reducingDescription')"
          icon="i-lucide-triangle-alert"
          color="warning"
          variant="subtle"
        />
        <UAlert v-if="saveError" :title="saveError" color="error" variant="subtle" />
        <div class="flex flex-wrap items-center justify-between gap-3 border-t border-default pt-5">
          <span class="text-xs text-muted" aria-live="polite">{{ changed ? t('settings.unsaved') : t('settings.savedState') }}</span>
          <div class="flex gap-2">
            <UButton :label="t('common.reset')" color="neutral" variant="ghost" :disabled="!changed || saving" @click="reset" />
            <UButton type="submit" :label="t('common.saveChanges')" icon="i-lucide-check" :loading="saving" :disabled="!changed" />
          </div>
        </div>
      </UForm>
    </section>

    <section class="space-y-5 rounded-xl border border-default bg-default p-5 shadow-sm sm:p-6">
      <header class="flex items-center gap-3">
        <span class="flex size-9 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
          <UIcon name="i-lucide-palette" class="size-4" />
        </span>
        <h2 class="text-lg font-semibold text-highlighted">{{ t('settings.appearance') }}</h2>
      </header>
      <div class="flex flex-wrap items-center justify-between gap-4">
        <div>
          <p class="text-sm font-medium text-highlighted">{{ t('settings.theme') }}</p>
          <p class="mt-1 text-sm text-muted">{{ t('settings.themeDescription') }}</p>
        </div>
        <UColorModeSelect :aria-label="t('settings.theme')" class="w-40" />
      </div>
      <div class="flex flex-wrap items-center justify-between gap-4">
        <div>
          <p class="text-sm font-medium text-highlighted">{{ t('settings.language') }}</p>
          <p class="mt-1 text-sm text-muted">{{ t('settings.languageDescription') }}</p>
        </div>
        <ULocaleSelect v-model="selectedLocale" :locales="localeOptions" class="w-40" :aria-label="t('settings.language')" />
      </div>
    </section>

    <section class="space-y-5 rounded-xl border border-default bg-default p-5 shadow-sm sm:p-6">
      <header class="flex items-center gap-3">
        <span class="flex size-9 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
          <UIcon name="i-lucide-user-round" class="size-4" />
        </span>
        <h2 class="text-lg font-semibold text-highlighted">{{ t('settings.account') }}</h2>
      </header>
      <dl class="grid gap-5 sm:grid-cols-3">
        <div>
          <dt class="text-xs text-muted">{{ t('settings.username') }}</dt>
          <dd class="mt-1 break-words text-sm font-medium text-highlighted">{{ user?.username }}</dd>
        </div>
        <div>
          <dt class="text-xs text-muted">{{ t('settings.displayName') }}</dt>
          <dd class="mt-1 break-words text-sm font-medium text-highlighted">{{ user?.name || user?.username }}</dd>
        </div>
        <div>
          <dt class="text-xs text-muted">{{ t('settings.role') }}</dt>
          <dd class="mt-1"><UBadge :label="isAdmin ? t('common.administrator') : t('common.user')" color="neutral" variant="subtle" /></dd>
        </div>
      </dl>
    </section>
  </main>
</template>
