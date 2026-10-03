<script setup lang="ts">
import type { FormSubmitEvent, TableColumn } from '@nuxt/ui'
import type { Service } from '~/types/service'
import { createServiceFormSchema, serviceInput, upstreamIssue, usesTls, type ServiceFormValues } from '~/utils/services'

definePageMeta({ layout: 'dashboard' })
const { t } = useI18n()
useSeoMeta({
  title: () => t('seo.servicesTitle'),
  description: () => t('seo.servicesDescription')
})
const serviceFormSchema = computed(() => createServiceFormSchema(key => t(key)))

const auth = useAuthStore()
const { user } = storeToRefs(auth)
const isAdmin = computed(() => user.value?.role === 'admin')
const { data: services, status, error: loadError, refresh } = await useFetch<Service[]>('/api/services', {
  immediate: isAdmin.value
})
const search = ref('')
const query = computed(() => search.value.trim().toLowerCase())
const visibleServices = computed(() => {
  const items = services.value ?? []
  if (!query.value) return items
  return items.filter(service => [service.name, service.hostname, service.upstreamUrl].some(value => value.toLowerCase().includes(query.value)))
})
const toast = useToast()
const editorForm = useTemplateRef<{ submit: () => Promise<void> }>('editorForm')
const editorOpen = ref(false)
const editing = ref<Service | null>(null)
const saving = ref(false)
const formError = ref('')
const state = reactive({
  name: '',
  hostname: '',
  upstreamUrl: '',
  skipTlsVerify: false,
  enabled: true
})
const pendingDelete = ref<Service | null>(null)
const deleting = ref(false)
const deleteError = ref('')
const togglingId = ref<number | null>(null)
const unchanged = computed(() => {
  const current = editing.value
  if (!current) return false
  return state.name === current.name
    && state.hostname === current.hostname
    && state.upstreamUrl === current.upstreamUrl
    && state.skipTlsVerify === current.skipTlsVerify
    && state.enabled === current.enabled
})
const httpsOrigin = computed(() => usesTls(state.upstreamUrl) && !upstreamIssue(state.upstreamUrl))

watch(state, () => { formError.value = '' })
watch(editorOpen, (open) => {
  if (!open) formError.value = ''
})

function blankForm() {
  state.name = ''
  state.hostname = ''
  state.upstreamUrl = ''
  state.skipTlsVerify = false
  state.enabled = true
  formError.value = ''
}

function openCreate() {
  editing.value = null
  blankForm()
  editorOpen.value = true
}

function openEdit(service: Service) {
  editing.value = service
  state.name = service.name
  state.hostname = service.hostname
  state.upstreamUrl = service.upstreamUrl
  state.skipTlsVerify = service.skipTlsVerify
  state.enabled = service.enabled
  formError.value = ''
  editorOpen.value = true
}

function replaceService(updated: Service) {
  services.value = (services.value ?? []).map(service => service.id === updated.id ? updated : service)
}

async function save(event: FormSubmitEvent<ServiceFormValues>) {
  if (!isAdmin.value || saving.value || unchanged.value) return
  saving.value = true
  formError.value = ''
  const body = event.data
  try {
    if (editing.value) {
      const updated = await $fetch<Service>(`/api/services/${editing.value.id}`, { method: 'PUT', body })
      replaceService(updated)
      toast.add({ title: t('services.updated'), description: t('services.updatedDescription'), icon: 'i-lucide-circle-check', color: 'success' })
    } else {
      const created = await $fetch<Service>('/api/services', { method: 'POST', body })
      services.value = [...(services.value ?? []), created]
      toast.add({ title: t('services.added'), description: t('services.addedDescription', { hostname: created.hostname }), icon: 'i-lucide-circle-check', color: 'success' })
    }
    editorOpen.value = false
  } catch (error) {
    formError.value = auth.authErrorMessage(error, t('services.saveFailed'))
  } finally {
    saving.value = false
  }
}

async function setEnabled(service: Service, enabled: boolean) {
  if (!isAdmin.value || togglingId.value !== null || service.enabled === enabled) return
  togglingId.value = service.id
  try {
    const updated = await $fetch<Service>(`/api/services/${service.id}`, {
      method: 'PUT',
      body: { ...serviceInput(service), enabled }
    })
    replaceService(updated)
    toast.add({
      title: updated.enabled ? t('services.enabledTitle') : t('services.disabledTitle'),
      description: t(updated.enabled ? 'services.enabledNotice' : 'services.disabledNotice', { hostname: updated.hostname }),
      icon: 'i-lucide-circle-check',
      color: 'success'
    })
  } catch (error) {
    toast.add({
      title: auth.authErrorMessage(error, t('services.updateFailed')),
      icon: 'i-lucide-circle-alert',
      color: 'error'
    })
  } finally {
    togglingId.value = null
  }
}

function askDelete(service: Service) {
  deleteError.value = ''
  pendingDelete.value = service
}

async function confirmDelete() {
  const service = pendingDelete.value
  if (!service || deleting.value) return
  deleting.value = true
  deleteError.value = ''
  try {
    await $fetch(`/api/services/${service.id}`, { method: 'DELETE' })
    services.value = (services.value ?? []).filter(item => item.id !== service.id)
    pendingDelete.value = null
    toast.add({ title: t('services.deleted'), description: t('services.deletedDescription', { hostname: service.hostname }), icon: 'i-lucide-circle-check', color: 'success' })
  } catch (error) {
    deleteError.value = auth.authErrorMessage(error, t('services.deleteFailed'))
  } finally {
    deleting.value = false
  }
}

const columns = computed<TableColumn<Service>[]>(() => [{
  accessorKey: 'name',
  header: t('services.name')
}, {
  accessorKey: 'hostname',
  header: t('services.hostname')
}, {
  accessorKey: 'upstreamUrl',
  header: t('services.upstream')
}, {
  id: 'tls',
  header: t('services.tls')
}, {
  id: 'enabled',
  header: t('services.enabled')
}, {
  id: 'actions',
  header: ''
}])
</script>

<template>
  <main class="mx-auto flex w-full max-w-7xl flex-col gap-6">
    <UCard :ui="{ body: 'p-0 sm:p-0' }">
      <template #header>
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div class="flex items-start gap-3">
            <UIcon name="i-lucide-server" class="mt-0.5 size-5 text-primary" />
            <div>
              <div class="flex flex-wrap items-center gap-2">
                <h2 class="font-semibold text-highlighted">{{ t('services.title') }}</h2>
                <UBadge
                  v-if="isAdmin && services"
                  :label="t(`services.count.${pluralForm(services.length)}`, { count: services.length })"
                  color="neutral"
                  variant="subtle"
                  size="sm"
                />
              </div>
              <p class="mt-1 text-sm text-muted">
                {{ t('services.description') }}
              </p>
            </div>
          </div>
          <div v-if="isAdmin && services" class="flex items-center gap-2">
            <UTooltip :text="t('services.refresh')">
              <UButton
                icon="i-lucide-refresh-cw"
                color="neutral"
                variant="outline"
                :aria-label="t('services.refresh')"
                @click="refresh()"
              />
            </UTooltip>
            <UButton v-if="services.length > 0" :label="t('services.add')" icon="i-lucide-plus" @click="openCreate" />
          </div>
        </div>
      </template>

      <div class="p-4 sm:p-6">
        <UAlert
          v-if="!isAdmin"
          :title="t('settings.adminRequired')"
          :description="t('services.adminOnly')"
          icon="i-lucide-lock"
          color="neutral"
          variant="subtle"
        />
        <div v-else-if="status === 'pending'" class="flex items-center gap-2 text-sm text-muted" role="status">
          <UIcon name="i-lucide-loader-circle" class="size-4 animate-spin" />
          {{ t('services.loading') }}
        </div>
        <div v-else-if="loadError || !services" class="space-y-4">
          <UAlert :title="t('services.loadFailed')" :description="t('common.checkConnection')" color="error" variant="subtle" />
          <UButton :label="t('common.tryAgain')" icon="i-lucide-refresh-cw" color="neutral" variant="outline" @click="refresh()" />
        </div>
        <UEmpty
          v-else-if="services.length === 0"
          icon="i-lucide-server"
          :title="t('services.emptyTitle')"
          :description="t('services.emptyDescription')"
          class="py-8"
        >
          <template #actions>
            <UButton :label="t('services.add')" icon="i-lucide-plus" @click="openCreate" />
          </template>
        </UEmpty>
        <div v-else class="space-y-4">
          <UInput
            v-model="search"
            icon="i-lucide-search"
            type="search"
            :placeholder="t('services.searchPlaceholder')"
            :aria-label="t('services.search')"
            class="w-full sm:max-w-sm"
          />
          <UTable
            :data="visibleServices"
            :columns="columns"
            :get-row-id="row => String(row.id)"
            :empty="query ? t('services.emptySearch') : t('services.emptyTitle')"
            :ui="{ th: 'bg-elevated/50 text-xs', td: 'text-sm', tr: 'hover:bg-elevated/30' }"
          >
            <template #name-cell="{ row }">
              <p class="font-medium text-highlighted">{{ row.original.name }}</p>
            </template>
            <template #hostname-cell="{ row }">
              <span class="font-mono text-xs text-muted">{{ row.original.hostname }}</span>
            </template>
            <template #upstreamUrl-cell="{ row }">
              <UTooltip :text="row.original.upstreamUrl">
                <span class="block max-w-72 truncate font-mono text-xs text-muted">{{ row.original.upstreamUrl }}</span>
              </UTooltip>
            </template>
            <template #tls-cell="{ row }">
              <UBadge
                v-if="usesTls(row.original.upstreamUrl) && row.original.skipTlsVerify"
                :label="t('services.verificationOff')"
                color="warning"
                variant="subtle"
                size="sm"
              />
              <UBadge
                v-else-if="usesTls(row.original.upstreamUrl)"
                :label="t('services.verified')"
                color="success"
                variant="subtle"
                size="sm"
              />
              <span v-else class="text-xs text-dimmed">{{ t('services.tlsUnused') }}</span>
            </template>
            <template #enabled-cell="{ row }">
              <USwitch
                :model-value="row.original.enabled"
                :loading="togglingId === row.original.id"
                :disabled="togglingId !== null"
                :aria-label="t(row.original.enabled ? 'services.disable' : 'services.enable', { name: row.original.name })"
                @update:model-value="setEnabled(row.original, $event)"
              />
            </template>
            <template #actions-cell="{ row }">
              <div class="flex justify-end gap-1">
                <UTooltip :text="t('services.edit')">
                  <UButton
                    icon="i-lucide-pencil"
                    color="neutral"
                    variant="ghost"
                    size="sm"
                    :aria-label="t('services.editNamed', { name: row.original.name })"
                    @click="openEdit(row.original)"
                  />
                </UTooltip>
                <UTooltip :text="t('services.delete')">
                  <UButton
                    icon="i-lucide-trash-2"
                    color="error"
                    variant="ghost"
                    size="sm"
                    :aria-label="t('services.deleteNamed', { name: row.original.name })"
                    @click="askDelete(row.original)"
                  />
                </UTooltip>
              </div>
            </template>
          </UTable>
        </div>
      </div>
    </UCard>

    <UModal
      v-model:open="editorOpen"
      :title="editing ? t('services.editTitle') : t('services.add')"
      :description="editing ? t('services.editDescription') : t('services.addDescription')"
      :dismissible="!saving"
      scrollable
    >
      <template #body>
        <UForm ref="editorForm" :schema="serviceFormSchema" :state="state" class="space-y-5" @submit="save">
          <UFormField :label="t('services.name')" name="name" :description="t('services.nameDescription')" required>
            <UInput v-model="state.name" :placeholder="t('services.namePlaceholder')" :disabled="saving" class="w-full" />
          </UFormField>
          <UFormField :label="t('services.hostname')" name="hostname" :description="t('services.hostnameDescription')" required>
            <UInput v-model="state.hostname" :placeholder="t('services.hostnamePlaceholder')" autocomplete="off" :disabled="saving" class="w-full" />
          </UFormField>
          <UFormField :label="t('services.upstream')" name="upstreamUrl" :description="t('services.upstreamDescription')" required>
            <UInput v-model="state.upstreamUrl" :placeholder="t('services.upstreamPlaceholder')" autocomplete="off" :disabled="saving" class="w-full" />
          </UFormField>
          <UFormField :label="t('services.skipTls')" name="skipTlsVerify" :description="t('services.skipTlsDescription')">
            <USwitch v-model="state.skipTlsVerify" :disabled="saving" />
          </UFormField>
          <UAlert
            v-if="state.skipTlsVerify && httpsOrigin"
            :title="t('services.tlsWarningTitle')"
            :description="t('services.tlsWarningDescription')"
            icon="i-lucide-triangle-alert"
            color="warning"
            variant="subtle"
          />
          <UFormField :label="t('services.enabled')" name="enabled" :description="t('services.enabledDescription')">
            <USwitch v-model="state.enabled" :disabled="saving" />
          </UFormField>
          <UAlert v-if="formError" :title="formError" color="error" variant="subtle" />
        </UForm>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton :label="t('common.cancel')" color="neutral" variant="ghost" :disabled="saving" @click="editorOpen = false" />
          <UButton :label="editing ? t('common.saveChanges') : t('services.add')" icon="i-lucide-check" :loading="saving" :disabled="unchanged" @click="editorForm?.submit()" />
        </div>
      </template>
    </UModal>

    <UModal
      :open="!!pendingDelete"
      :title="t('services.delete')"
      :description="pendingDelete ? t('services.deleteDescription', { name: pendingDelete.name, hostname: pendingDelete.hostname }) : ''"
      :dismissible="!deleting"
      @update:open="open => { if (!open && !deleting) pendingDelete = null }"
    >
      <template #footer>
        <div class="flex w-full flex-col gap-3">
          <UAlert v-if="deleteError" :title="deleteError" color="error" variant="subtle" />
          <div class="flex justify-end gap-2">
            <UButton :label="t('common.cancel')" color="neutral" variant="ghost" :disabled="deleting" @click="pendingDelete = null" />
            <UButton :label="t('services.delete')" icon="i-lucide-trash-2" color="error" :loading="deleting" @click="confirmDelete" />
          </div>
        </div>
      </template>
    </UModal>
  </main>
</template>
