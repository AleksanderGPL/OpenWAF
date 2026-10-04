<script setup lang="ts">
import type { FormSubmitEvent } from '@nuxt/ui'
import type { Service } from '~/types/service'
import type { AnomalyPolicy, AnomalySettingsView } from '~/types/investigation'
import * as z from 'zod'

const { t } = useI18n()
const auth = useAuthStore()
const toast = useToast()
const isAdmin = computed(() => auth.user?.role === 'admin')
const services = ref<Service[]>([])
const scope = ref('0')
const view = ref<AnomalySettingsView | null>(null)
const loading = ref(true)
const loadError = ref('')
const saving = ref(false)
const restoring = ref(false)
const saveError = ref('')
const state = reactive<AnomalyPolicy>({
  enabled: true,
  blockedThreshold: 10,
  suspiciousThreshold: 10,
  scanRequests: 30,
  scanPaths: 20,
  errorThreshold: 20,
  errorPercent: 50,
  surgeMinimum: 100,
  surgeMultiplier: 4,
  cooldownMinutes: 10,
  hourlyRunLimit: 20
})

const schema = computed(() => {
  const whole = (min: number, max: number, message: string) => z.coerce.number().int(message).min(min, message).max(max, message)
  const threshold = whole(1, 1000000, t('anomaly.validation.threshold'))
  return z.object({
    enabled: z.boolean(),
    blockedThreshold: threshold,
    suspiciousThreshold: threshold,
    scanRequests: threshold,
    scanPaths: whole(1, 100, t('anomaly.validation.scanPaths')),
    errorThreshold: threshold,
    errorPercent: whole(1, 100, t('anomaly.validation.percent')),
    surgeMinimum: threshold,
    surgeMultiplier: z.coerce.number().min(1.5, t('anomaly.validation.multiplier')).max(100, t('anomaly.validation.multiplier')),
    cooldownMinutes: whole(1, 1440, t('anomaly.validation.cooldown')),
    hourlyRunLimit: whole(1, 1000, t('anomaly.validation.hourly'))
  })
})
const scopeOptions = computed(() => [{
  label: t('anomaly.global'),
  value: '0'
}, ...services.value.map(service => ({
  label: `${service.name} · ${service.hostname}`,
  value: String(service.id)
}))])
const serviceScope = computed(() => scope.value !== '0')
const changed = computed(() => {
  const policy = view.value?.policy
  if (!policy) return false
  return (Object.keys(state) as (keyof AnomalyPolicy)[]).some(key => Number(state[key]) !== Number(policy[key]) && state[key] !== policy[key])
})

function apply(next: AnomalySettingsView) {
  view.value = next
  const policy = next.policy
  state.enabled = policy.enabled
  state.blockedThreshold = policy.blockedThreshold
  state.suspiciousThreshold = policy.suspiciousThreshold
  state.scanRequests = policy.scanRequests
  state.scanPaths = policy.scanPaths
  state.errorThreshold = policy.errorThreshold
  state.errorPercent = policy.errorPercent
  state.surgeMinimum = policy.surgeMinimum
  state.surgeMultiplier = policy.surgeMultiplier
  state.cooldownMinutes = policy.cooldownMinutes
  state.hourlyRunLimit = policy.hourlyRunLimit
  saveError.value = ''
}
async function load() {
  if (!isAdmin.value) { loading.value = false; return }
  loading.value = true
  loadError.value = ''
  try {
    const [settings, listed] = await Promise.all([
      $fetch<AnomalySettingsView>('/api/anomaly-settings', { query: serviceScope.value ? { serviceId: scope.value } : undefined }),
      services.value.length ? Promise.resolve(services.value) : $fetch<Service[]>('/api/services')
    ])
    services.value = listed
    apply(settings)
  } catch (error) {
    loadError.value = auth.authErrorMessage(error, t('anomaly.loadFailed'))
  } finally {
    loading.value = false
  }
}
function reset() {
  if (view.value) apply(view.value)
}
async function save(event: FormSubmitEvent<AnomalyPolicy>) {
  if (!isAdmin.value || saving.value || !changed.value) return
  saving.value = true
  saveError.value = ''
  try {
    const saved = await $fetch<AnomalySettingsView>('/api/anomaly-settings', {
      method: 'PUT',
      query: serviceScope.value ? { serviceId: scope.value } : undefined,
      body: event.data
    })
    apply(saved)
    toast.add({ title: t('anomaly.saved'), icon: 'i-lucide-circle-check', color: 'success' })
  } catch (error) {
    saveError.value = auth.authErrorMessage(error, t('anomaly.saveFailed'))
  } finally {
    saving.value = false
  }
}
async function restore() {
  if (!serviceScope.value || !view.value || view.value.inherited || restoring.value) return
  restoring.value = true
  saveError.value = ''
  try {
    await $fetch('/api/anomaly-settings', { method: 'DELETE', query: { serviceId: scope.value } })
    await load()
    toast.add({ title: t('anomaly.restored'), icon: 'i-lucide-circle-check', color: 'success' })
  } catch (error) {
    saveError.value = auth.authErrorMessage(error, t('anomaly.saveFailed'))
  } finally {
    restoring.value = false
  }
}

watch(scope, () => { void load() })
onMounted(load)
</script>

<template>
  <section v-if="isAdmin" class="space-y-5 rounded-xl border border-default bg-default p-5 shadow-sm sm:p-6">
    <header class="flex items-start gap-3">
      <span class="mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
        <UIcon name="i-lucide-radar" class="size-4" />
      </span>
      <div>
        <h2 class="text-lg font-semibold text-highlighted">{{ t('anomaly.title') }}</h2>
        <p class="mt-1 text-sm text-muted">{{ t('anomaly.description') }}</p>
      </div>
    </header>

    <div v-if="loading" class="flex items-center gap-2 text-sm text-muted" role="status">
      <UIcon name="i-lucide-loader-circle" class="size-4 animate-spin" />
      {{ t('anomaly.loading') }}
    </div>
    <div v-else-if="loadError || !view" class="space-y-4">
      <UAlert :title="loadError || t('anomaly.loadFailed')" :description="t('common.checkConnection')" color="error" variant="subtle" />
      <UButton :label="t('common.tryAgain')" icon="i-lucide-refresh-cw" color="neutral" variant="outline" @click="load" />
    </div>
    <UForm v-else :schema="schema" :state="state" class="space-y-5" @submit="save">
      <div class="space-y-2">
        <p class="text-sm font-medium text-highlighted">{{ t('anomaly.scope') }}</p>
        <USelect v-model="scope" :items="scopeOptions" value-key="value" :disabled="saving || restoring || changed" :aria-label="t('anomaly.scope')" class="w-full sm:max-w-sm" />
        <p v-if="changed" class="text-xs text-muted">{{ t('anomaly.scopeLocked') }}</p>
      </div>
      <UAlert v-if="!view.aiEnabled" :title="t('anomaly.aiOff')" :description="t('anomaly.aiOffDescription')" icon="i-lucide-sparkles" color="warning" variant="subtle" />
      <UAlert v-if="serviceScope && view.inherited" :title="t('anomaly.inherited')" :description="t('anomaly.inheritedDescription')" icon="i-lucide-link" color="info" variant="subtle" />
      <div v-else-if="serviceScope" class="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-default px-4 py-3">
        <p class="text-sm text-muted">{{ t('anomaly.overrideDescription') }}</p>
        <UButton :label="t('anomaly.restore')" icon="i-lucide-undo-2" color="neutral" variant="outline" :loading="restoring" :disabled="saving || changed" @click="restore" />
      </div>

      <UFormField :label="t('anomaly.enabled')" name="enabled" :description="t('anomaly.enabledDescription')">
        <USwitch v-model="state.enabled" :disabled="saving" />
      </UFormField>

      <div class="grid gap-5 sm:grid-cols-2">
        <UFormField :label="t('anomaly.blockedThreshold')" name="blockedThreshold" :description="t('anomaly.blockedDescription')" required>
          <UInput v-model.number="state.blockedThreshold" type="number" :min="1" :max="1000000" :disabled="saving" class="w-full" />
        </UFormField>
        <UFormField :label="t('anomaly.suspiciousThreshold')" name="suspiciousThreshold" :description="t('anomaly.suspiciousDescription')" required>
          <UInput v-model.number="state.suspiciousThreshold" type="number" :min="1" :max="1000000" :disabled="saving" class="w-full" />
        </UFormField>
        <UFormField :label="t('anomaly.scanRequests')" name="scanRequests" :description="t('anomaly.scanRequestsDescription')" required>
          <UInput v-model.number="state.scanRequests" type="number" :min="1" :max="1000000" :disabled="saving" class="w-full" />
        </UFormField>
        <UFormField :label="t('anomaly.scanPaths')" name="scanPaths" :description="t('anomaly.scanPathsDescription')" required>
          <UInput v-model.number="state.scanPaths" type="number" :min="1" :max="100" :disabled="saving" class="w-full" />
        </UFormField>
        <UFormField :label="t('anomaly.errorThreshold')" name="errorThreshold" :description="t('anomaly.errorDescription')" required>
          <UInput v-model.number="state.errorThreshold" type="number" :min="1" :max="1000000" :disabled="saving" class="w-full" />
        </UFormField>
        <UFormField :label="t('anomaly.errorPercent')" name="errorPercent" :description="t('anomaly.errorPercentDescription')" required>
          <UInput v-model.number="state.errorPercent" type="number" :min="1" :max="100" :disabled="saving" class="w-full" />
        </UFormField>
        <UFormField :label="t('anomaly.surgeMinimum')" name="surgeMinimum" :description="t('anomaly.surgeDescription')" required>
          <UInput v-model.number="state.surgeMinimum" type="number" :min="1" :max="1000000" :disabled="saving" class="w-full" />
        </UFormField>
        <UFormField :label="t('anomaly.surgeMultiplier')" name="surgeMultiplier" :description="t('anomaly.surgeMultiplierDescription')" required>
          <UInput v-model.number="state.surgeMultiplier" type="number" :min="1.5" :max="100" :step="0.1" :disabled="saving" class="w-full" />
        </UFormField>
        <UFormField :label="t('anomaly.cooldown')" name="cooldownMinutes" :description="t('anomaly.cooldownDescription')" required>
          <UInput v-model.number="state.cooldownMinutes" type="number" :min="1" :max="1440" :disabled="saving" class="w-full" />
        </UFormField>
        <UFormField :label="t('anomaly.hourlyLimit')" name="hourlyRunLimit" :description="t('anomaly.hourlyDescription')" required>
          <UInput v-model.number="state.hourlyRunLimit" type="number" :min="1" :max="1000" :disabled="saving" class="w-full" />
        </UFormField>
      </div>

      <UAlert v-if="saveError" :title="saveError" color="error" variant="subtle" />
      <div class="flex flex-wrap items-center justify-between gap-3 border-t border-default pt-5">
        <span class="text-xs text-muted" aria-live="polite">{{ changed ? t('settings.unsaved') : t('settings.savedState') }}</span>
        <div class="flex gap-2">
          <UButton :label="t('common.reset')" color="neutral" variant="ghost" :disabled="!changed || saving" @click="reset" />
          <UButton type="submit" :label="t('common.saveChanges')" icon="i-lucide-check" :loading="saving" :disabled="!changed || restoring" />
        </div>
      </div>
    </UForm>
  </section>
</template>
