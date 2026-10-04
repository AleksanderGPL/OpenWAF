<script setup lang="ts">
import type { FormSubmitEvent, TableColumn } from '@nuxt/ui'
import type { Service } from '~/types/service'
import type {
  CatalogRule,
  PolicyInput,
  PolicyView,
  Rule,
  RuleFormValues,
  RuleTarget
} from '~/types/rule'
import {
  blankCondition,
  canDisableCatalogRule,
  createRuleFormSchema,
  formValuesFromRule,
  isRuleTarget,
  operatorsFor,
  ruleInput,
  sameRule
} from '~/utils/rules'

definePageMeta({ layout: 'dashboard' })
const { t } = useI18n()
useSeoMeta({
  title: () => t('seo.rulesTitle'),
  description: () => t('seo.rulesDescription')
})
const ruleFormSchema = computed(() => createRuleFormSchema(key => t(key)))

const auth = useAuthStore()
const { user } = storeToRefs(auth)
const isAdmin = computed(() => user.value?.role === 'admin')

type RulesPageData = {
  rules: Rule[]
  services: Service[]
  catalog: CatalogRule[]
  crsVersion: string
  policy: PolicyView
}

async function loadRules(): Promise<RulesPageData> {
  const [globalRules, services, catalogRes, policy] = await Promise.all([
    $fetch<Rule[]>('/api/rules'),
    $fetch<Service[]>('/api/services'),
    $fetch<{ crsVersion: string, rules: CatalogRule[] }>('/api/rules/catalog'),
    $fetch<PolicyView>('/api/rules/policy')
  ])
  const scoped = await Promise.all(services.map(service => $fetch<Rule[]>('/api/rules', { query: { serviceId: service.id } })))
  return {
    rules: [...globalRules, ...scoped.flat()].sort((left, right) => left.id - right.id),
    services,
    catalog: catalogRes.rules.filter(rule => canDisableCatalogRule(rule.id)).sort((left, right) => left.id - right.id),
    crsVersion: catalogRes.crsVersion,
    policy
  }
}

const { data, status, error: loadError, refresh } = await useAsyncData('dashboard-rules', loadRules, {
  immediate: isAdmin.value
})
const rules = computed(() => data.value?.rules ?? [])
const services = computed(() => data.value?.services ?? [])
const catalog = computed(() => data.value?.catalog ?? [])
const crsVersion = computed(() => data.value?.crsVersion ?? '')
const disabledCatalogIds = computed(() => new Set(data.value?.policy.policy.disabledRuleIds ?? []))

const activeSection = ref<'custom' | 'catalog'>('custom')
const search = ref('')
const query = computed(() => search.value.trim().toLowerCase())
const catalogSearch = ref('')
const catalogQuery = computed(() => catalogSearch.value.trim().toLowerCase())
const catalogFilter = ref<'all' | 'disabled'>('all')
const catalogPage = ref(1)
const catalogPageSize = 20
const sectionTabs = computed(() => [{
  label: t('rules.customHeading'),
  value: 'custom' as const,
  badge: rules.value.length || undefined
}, {
  label: t('rules.catalogHeading'),
  value: 'catalog' as const,
  badge: disabledCatalogIds.value.size || undefined
}])
const catalogFilterTabs = computed(() => [{
  label: t('rules.catalogFilterAll'),
  value: 'all' as const
}, {
  label: t('rules.catalogFilterDisabled'),
  value: 'disabled' as const
}])
const formScopeOptions = computed(() => [{
  label: t('rules.scopeGlobal'),
  value: 'global'
}, ...services.value.map(service => ({
  label: `${service.name} · ${service.hostname}`,
  value: String(service.id)
}))])
const actionOptions = computed(() => [{
  label: t('rules.actions.block'),
  value: 'block' as const
}, {
  label: t('rules.actions.log'),
  value: 'log' as const
}])
const targetOptions = computed(() => (['path', 'query', 'header', 'method', 'ip'] as const).map(value => ({
  label: t(`rules.targets.${value}`),
  value
})))

function serviceById(id: number | null) {
  if (id == null) return null
  return services.value.find(service => service.id === id) ?? null
}

function summarize(condition: Rule['conditions'][number]) {
  const target = t(`rules.targets.${condition.target}`)
  const operator = t(`rules.operators.${condition.operator}`)
  const subject = condition.key ? `${target} ${condition.key}` : target
  return `${subject} ${operator} ${condition.value}`
}

function conditionsLabel(rule: Rule) {
  const [first, ...rest] = rule.conditions
  if (!first) return '—'
  const summary = summarize(first)
  if (!rest.length) return summary
  return `${summary} ${t('rules.moreConditions', { count: rest.length })}`
}

function scopeLabel(serviceId: number | null) {
  if (serviceId == null) return t('rules.scopeGlobal')
  const service = serviceById(serviceId)
  return service?.hostname ?? t('rules.scopeMissing', { id: serviceId })
}

const visibleRules = computed(() => rules.value.filter((rule) => {
  if (!query.value) return true
  const service = serviceById(rule.serviceId)
  const haystack = [
    rule.name,
    rule.description,
    rule.serviceId == null ? t('rules.scopeGlobal') : `${service?.name ?? ''} ${service?.hostname ?? ''}`,
    ...rule.conditions.map(summarize)
  ].join(' ').toLowerCase()
  return haystack.includes(query.value)
}))

const filteredCatalog = computed(() => catalog.value.filter((rule) => {
  if (catalogFilter.value === 'disabled' && !disabledCatalogIds.value.has(rule.id)) return false
  if (!catalogQuery.value) return true
  const haystack = [String(rule.id), rule.source, rule.message, rule.file ?? '', ...rule.tags].join(' ').toLowerCase()
  return haystack.includes(catalogQuery.value)
}))
const catalogTotal = computed(() => filteredCatalog.value.length)
const catalogPageCount = computed(() => Math.max(1, Math.ceil(catalogTotal.value / catalogPageSize)))
const visibleCatalog = computed(() => {
  const start = (catalogPage.value - 1) * catalogPageSize
  return filteredCatalog.value.slice(start, start + catalogPageSize)
})
const catalogFrom = computed(() => catalogTotal.value === 0 ? 0 : (catalogPage.value - 1) * catalogPageSize + 1)
const catalogTo = computed(() => Math.min(catalogPage.value * catalogPageSize, catalogTotal.value))

watch([catalogQuery, catalogFilter], () => { catalogPage.value = 1 })
watch(catalogPageCount, (count) => {
  if (catalogPage.value > count) catalogPage.value = count
})

const toast = useToast()
const editorForm = useTemplateRef<{ submit: () => Promise<void> }>('editorForm')
const editorOpen = ref(false)
const editing = ref<Rule | null>(null)
const saving = ref(false)
const formError = ref('')
const state = reactive<RuleFormValues>({
  name: '',
  description: '',
  action: 'block',
  enabled: true,
  scope: 'global',
  conditions: [blankCondition()]
})
const pendingDelete = ref<Rule | null>(null)
const deleting = ref(false)
const deleteError = ref('')
const togglingId = ref<number | null>(null)
const togglingCatalogId = ref<number | null>(null)
const unchanged = computed(() => {
  const current = editing.value
  if (!current) return false
  return sameRule(ruleInput(state), ruleInput(formValuesFromRule(current)))
})
const conditionLimitReached = computed(() => state.conditions.length >= 10)

watch(state, () => { formError.value = '' })
watch(editorOpen, (open) => {
  if (!open) formError.value = ''
})

function applyForm(values: RuleFormValues) {
  state.name = values.name
  state.description = values.description
  state.action = values.action
  state.enabled = values.enabled
  state.scope = values.scope
  state.conditions = values.conditions.map(condition => ({ ...condition }))
  formError.value = ''
}

function openCreate() {
  editing.value = null
  applyForm({
    name: '',
    description: '',
    action: 'block',
    enabled: true,
    scope: 'global',
    conditions: [blankCondition()]
  })
  editorOpen.value = true
}

function openEdit(rule: Rule) {
  editing.value = rule
  applyForm(formValuesFromRule(rule))
  editorOpen.value = true
}

function replaceRule(updated: Rule) {
  if (!data.value) return
  data.value = { ...data.value, rules: data.value.rules.map(rule => rule.id === updated.id ? updated : rule) }
}

function needsKey(target: RuleTarget) {
  return target === 'query' || target === 'header'
}

function setTarget(index: number, value: unknown) {
  if (typeof value !== 'string' || !isRuleTarget(value)) return
  const condition = state.conditions[index]
  if (!condition) return
  condition.target = value
  if (!needsKey(value)) condition.key = ''
  if (!operatorsFor(value).includes(condition.operator)) condition.operator = 'equals'
}

function operatorOptions(target: RuleTarget) {
  return operatorsFor(target).map(value => ({ label: t(`rules.operators.${value}`), value }))
}

function valuePlaceholder(condition: RuleFormValues['conditions'][number]) {
  if (condition.operator === 'cidr') return t('rules.placeholders.cidr')
  return t(`rules.placeholders.${condition.target}`)
}

function keyPlaceholder(target: RuleTarget) {
  return t(target === 'query' ? 'rules.placeholders.queryKey' : 'rules.keyPlaceholder')
}

function addCondition() {
  if (conditionLimitReached.value || saving.value) return
  state.conditions.push(blankCondition())
}

function removeCondition(index: number) {
  if (state.conditions.length <= 1 || saving.value) return
  state.conditions.splice(index, 1)
}

async function save(event: FormSubmitEvent<RuleFormValues>) {
  if (!isAdmin.value || saving.value || unchanged.value) return
  saving.value = true
  formError.value = ''
  const body = ruleInput(event.data)
  try {
    if (editing.value) {
      const updated = await $fetch<Rule>(`/api/rules/${editing.value.id}`, { method: 'PUT', body })
      replaceRule(updated)
      toast.add({ title: t('rules.updated'), description: t('rules.updatedDescription'), icon: 'i-lucide-circle-check', color: 'success' })
    } else {
      const created = await $fetch<Rule>('/api/rules', { method: 'POST', body })
      if (data.value) data.value = { ...data.value, rules: [...data.value.rules, created].sort((left, right) => left.id - right.id) }
      toast.add({ title: t('rules.added'), description: t('rules.addedDescription', { name: created.name }), icon: 'i-lucide-circle-check', color: 'success' })
    }
    editorOpen.value = false
  } catch (error) {
    formError.value = auth.authErrorMessage(error, t('rules.saveFailed'))
  } finally {
    saving.value = false
  }
}

async function setEnabled(rule: Rule, enabled: boolean) {
  if (!isAdmin.value || togglingId.value !== null || rule.enabled === enabled) return
  togglingId.value = rule.id
  try {
    const updated = await $fetch<Rule>(`/api/rules/${rule.id}`, {
      method: 'PUT',
      body: {
        ...ruleInput(formValuesFromRule(rule)),
        serviceId: rule.serviceId,
        enabled
      }
    })
    replaceRule(updated)
    toast.add({
      title: updated.enabled ? t('rules.enabledTitle') : t('rules.disabledTitle'),
      description: t(updated.enabled ? 'rules.enabledNotice' : 'rules.disabledNotice', { name: updated.name }),
      icon: 'i-lucide-circle-check',
      color: 'success'
    })
  } catch (error) {
    toast.add({
      title: auth.authErrorMessage(error, t('rules.updateFailed')),
      icon: 'i-lucide-circle-alert',
      color: 'error'
    })
  } finally {
    togglingId.value = null
  }
}

function policyInput(policy: PolicyView['policy'], disabledRuleIds: number[]): PolicyInput {
  return {
    mode: policy.mode,
    blockingParanoiaLevel: policy.blockingParanoiaLevel,
    detectionParanoiaLevel: policy.detectionParanoiaLevel,
    inboundThreshold: policy.inboundThreshold,
    maxBodyBytes: policy.maxBodyBytes,
    rateLimitPerMinute: policy.rateLimitPerMinute,
    rateLimitAction: policy.rateLimitAction,
    disabledRuleIds
  }
}

function catalogEnabled(rule: CatalogRule) {
  return !disabledCatalogIds.value.has(rule.id)
}

async function setCatalogEnabled(rule: CatalogRule, enabled: boolean) {
  if (!isAdmin.value || !data.value || togglingCatalogId.value !== null) return
  const currentlyEnabled = catalogEnabled(rule)
  if (currentlyEnabled === enabled) return
  const disabled = new Set(data.value.policy.policy.disabledRuleIds)
  if (enabled) disabled.delete(rule.id)
  else {
    if (disabled.size >= 100) {
      toast.add({ title: t('rules.catalogLimit'), icon: 'i-lucide-circle-alert', color: 'warning' })
      return
    }
    disabled.add(rule.id)
  }
  togglingCatalogId.value = rule.id
  try {
    const policy = await $fetch<PolicyView>('/api/rules/policy', {
      method: 'PUT',
      body: policyInput(data.value.policy.policy, [...disabled].sort((left, right) => left - right))
    })
    data.value = { ...data.value, policy }
    toast.add({
      title: enabled ? t('rules.catalogEnabledTitle') : t('rules.catalogDisabledTitle'),
      description: t(enabled ? 'rules.catalogEnabledNotice' : 'rules.catalogDisabledNotice', { id: rule.id }),
      icon: 'i-lucide-circle-check',
      color: 'success'
    })
  } catch (error) {
    toast.add({
      title: auth.authErrorMessage(error, t('rules.catalogUpdateFailed')),
      icon: 'i-lucide-circle-alert',
      color: 'error'
    })
  } finally {
    togglingCatalogId.value = null
  }
}

function askDelete(rule: Rule) {
  deleteError.value = ''
  pendingDelete.value = rule
}

async function confirmDelete() {
  const rule = pendingDelete.value
  if (!rule || deleting.value || !data.value) return
  deleting.value = true
  deleteError.value = ''
  try {
    await $fetch(`/api/rules/${rule.id}`, { method: 'DELETE' })
    data.value = { ...data.value, rules: data.value.rules.filter(item => item.id !== rule.id) }
    pendingDelete.value = null
    toast.add({ title: t('rules.deleted'), description: t('rules.deletedDescription', { name: rule.name }), icon: 'i-lucide-circle-check', color: 'success' })
  } catch (error) {
    deleteError.value = auth.authErrorMessage(error, t('rules.deleteFailed'))
  } finally {
    deleting.value = false
  }
}

const columns = computed<TableColumn<Rule>[]>(() => [{
  accessorKey: 'name',
  header: t('rules.name')
}, {
  id: 'scope',
  header: t('rules.scope')
}, {
  id: 'action',
  header: t('rules.action')
}, {
  id: 'conditions',
  header: t('rules.conditions')
}, {
  id: 'enabled',
  header: t('rules.enabled')
}, {
  id: 'actions',
  header: ''
}])

const emptyMessage = computed(() => query.value ? t('rules.emptySearch') : t('rules.emptyTitle'))
const catalogEmptyMessage = computed(() => {
  if (catalogQuery.value) return t('rules.catalogEmptySearch')
  if (catalogFilter.value === 'disabled') return t('rules.catalogFilterDisabled')
  return t('rules.catalogEmpty')
})
const catalogColumns = computed<TableColumn<CatalogRule>[]>(() => [{
  accessorKey: 'id',
  header: t('rules.catalogId')
}, {
  accessorKey: 'source',
  header: t('rules.catalogSource')
}, {
  accessorKey: 'message',
  header: t('rules.catalogMessage')
}, {
  id: 'enabled',
  header: t('rules.enabled')
}])
const tableUi = {
  th: 'bg-elevated/80 text-xs font-medium text-muted',
  td: 'py-3 text-sm text-highlighted',
  tr: 'border-b border-default/80 last:border-b-0 hover:bg-elevated/40',
  separator: 'bg-default'
}
</script>

<template>
  <main class="mx-auto flex w-full max-w-7xl flex-col gap-4">
    <UAlert
      v-if="!isAdmin"
      :title="t('settings.adminRequired')"
      :description="t('rules.adminOnly')"
      icon="i-lucide-lock"
      color="neutral"
      variant="subtle"
    />
    <div v-else-if="status === 'pending'" class="flex items-center gap-2 text-sm text-muted" role="status">
      <UIcon name="i-lucide-loader-circle" class="size-4 animate-spin" />
      {{ t('rules.loading') }}
    </div>
    <div v-else-if="loadError || !data" class="space-y-4">
      <UAlert :title="t('rules.loadFailed')" :description="t('common.checkConnection')" color="error" variant="subtle" />
      <UButton :label="t('common.tryAgain')" icon="i-lucide-refresh-cw" color="neutral" variant="outline" @click="refresh()" />
    </div>
    <div v-else class="overflow-hidden rounded-xl border border-default bg-default shadow-sm">
      <div class="flex flex-wrap items-center justify-between gap-3 border-b border-default px-4 py-3 sm:px-5">
        <UTabs
          v-model="activeSection"
          :items="sectionTabs"
          :content="false"
          variant="link"
          size="sm"
          :aria-label="t('rules.sectionNav')"
          class="min-w-0"
        />
        <div class="flex items-center gap-2">
          <UTooltip :text="t('rules.refresh')">
            <UButton
              icon="i-lucide-refresh-cw"
              color="neutral"
              variant="ghost"
              size="sm"
              :aria-label="t('rules.refresh')"
              @click="refresh()"
            />
          </UTooltip>
          <UButton
            v-if="activeSection === 'custom'"
            :label="t('rules.add')"
            icon="i-lucide-plus"
            size="sm"
            @click="openCreate"
          />
        </div>
      </div>

      <template v-if="activeSection === 'custom'">
        <UEmpty
          v-if="rules.length === 0"
          icon="i-lucide-shield"
          :title="t('rules.emptyTitle')"
          :description="t('rules.emptyDescription')"
          class="py-12"
        >
          <template #actions>
            <UButton :label="t('rules.add')" icon="i-lucide-plus" @click="openCreate" />
          </template>
        </UEmpty>
        <template v-else>
          <div class="border-b border-default px-4 py-3 sm:px-5">
            <UInput
              v-model="search"
              icon="i-lucide-search"
              type="search"
              size="sm"
              :placeholder="t('rules.searchPlaceholder')"
              :aria-label="t('rules.search')"
              class="w-full sm:max-w-xs"
            />
          </div>
          <UTable
            :data="visibleRules"
            :columns="columns"
            :get-row-id="row => String(row.id)"
            :empty="emptyMessage"
            :ui="tableUi"
          >
            <template #name-cell="{ row }">
              <UTooltip v-if="row.original.description" :text="row.original.description">
                <p class="font-medium text-highlighted">{{ row.original.name }}</p>
              </UTooltip>
              <p v-else class="font-medium text-highlighted">{{ row.original.name }}</p>
            </template>
            <template #scope-cell="{ row }">
              <span class="font-mono text-xs text-toned">{{ scopeLabel(row.original.serviceId) }}</span>
            </template>
            <template #action-cell="{ row }">
              <UBadge
                :label="t(`rules.actions.${row.original.action}`)"
                :color="row.original.action === 'block' ? 'warning' : 'success'"
                variant="subtle"
                size="sm"
              />
            </template>
            <template #conditions-cell="{ row }">
              <UTooltip :text="row.original.conditions.map(summarize).join('\n')">
                <span class="block max-w-72 truncate font-mono text-xs text-toned">{{ conditionsLabel(row.original) }}</span>
              </UTooltip>
            </template>
            <template #enabled-cell="{ row }">
              <USwitch
                :model-value="row.original.enabled"
                :loading="togglingId === row.original.id"
                :disabled="togglingId !== null"
                size="sm"
                :aria-label="t(row.original.enabled ? 'rules.disable' : 'rules.enable', { name: row.original.name })"
                @update:model-value="setEnabled(row.original, $event)"
              />
            </template>
            <template #actions-cell="{ row }">
              <div class="flex justify-end gap-1">
                <UTooltip :text="t('rules.edit')">
                  <UButton
                    icon="i-lucide-pencil"
                    color="neutral"
                    variant="ghost"
                    size="sm"
                    :aria-label="t('rules.editNamed', { name: row.original.name })"
                    @click="openEdit(row.original)"
                  />
                </UTooltip>
                <UTooltip :text="t('rules.delete')">
                  <UButton
                    icon="i-lucide-trash-2"
                    color="error"
                    variant="ghost"
                    size="sm"
                    :aria-label="t('rules.deleteNamed', { name: row.original.name })"
                    @click="askDelete(row.original)"
                  />
                </UTooltip>
              </div>
            </template>
          </UTable>
        </template>
      </template>

      <template v-else>
        <div class="flex flex-col gap-3 border-b border-default px-4 py-3 sm:flex-row sm:items-center sm:justify-between sm:px-5">
          <div class="flex min-w-0 flex-wrap items-center gap-2">
            <UInput
              v-model="catalogSearch"
              icon="i-lucide-search"
              type="search"
              size="sm"
              :placeholder="t('rules.catalogSearchPlaceholder')"
              :aria-label="t('rules.catalogSearch')"
              class="w-full sm:w-64"
            />
            <UTabs
              v-model="catalogFilter"
              :items="catalogFilterTabs"
              :content="false"
              variant="pill"
              size="xs"
              :aria-label="t('rules.catalogFilter')"
            />
          </div>
          <div class="flex shrink-0 items-center gap-2">
            <UBadge
              v-if="crsVersion"
              :label="t('rules.catalogVersion', { version: crsVersion })"
              color="neutral"
              variant="outline"
              size="sm"
            />
            <UBadge
              v-if="disabledCatalogIds.size"
              :label="t(`rules.catalogDisabledCount.${pluralForm(disabledCatalogIds.size)}`, { count: disabledCatalogIds.size })"
              color="warning"
              variant="subtle"
              size="sm"
            />
          </div>
        </div>
        <UTable
          :data="visibleCatalog"
          :columns="catalogColumns"
          :get-row-id="row => String(row.id)"
          :empty="catalogEmptyMessage"
          :ui="tableUi"
        >
          <template #id-cell="{ row }">
            <span class="font-mono text-xs tabular-nums text-toned">{{ row.original.id }}</span>
          </template>
          <template #source-cell="{ row }">
            <span class="text-xs uppercase tracking-wide text-muted">{{ row.original.source }}</span>
          </template>
          <template #message-cell="{ row }">
            <div class="min-w-0 max-w-2xl">
              <p class="truncate text-sm text-highlighted">
                {{ row.original.message || row.original.file?.split('/').pop() || '—' }}
              </p>
              <p v-if="row.original.message && row.original.file" class="mt-0.5 truncate font-mono text-[11px] text-dimmed">
                {{ row.original.file.split('/').pop() }}
              </p>
            </div>
          </template>
          <template #enabled-cell="{ row }">
            <USwitch
              :model-value="catalogEnabled(row.original)"
              :loading="togglingCatalogId === row.original.id"
              :disabled="togglingCatalogId !== null"
              size="sm"
              :aria-label="t(catalogEnabled(row.original) ? 'rules.catalogDisable' : 'rules.catalogEnable', { id: row.original.id })"
              @update:model-value="setCatalogEnabled(row.original, $event)"
            />
          </template>
        </UTable>
        <div
          v-if="catalogTotal > 0"
          class="flex flex-wrap items-center justify-between gap-3 border-t border-default px-4 py-3 sm:px-5"
        >
          <p class="text-xs text-muted">
            {{ t('rules.catalogShowing', { from: catalogFrom, to: catalogTo, total: catalogTotal }) }}
          </p>
          <UPagination
            v-model:page="catalogPage"
            :items-per-page="catalogPageSize"
            :total="catalogTotal"
            :sibling-count="0"
            size="sm"
          />
        </div>
      </template>
    </div>

    <UModal
      v-model:open="editorOpen"
      :title="editing ? t('rules.editTitle') : t('rules.add')"
      :description="editing ? t('rules.editDescription') : t('rules.addDescription')"
      :dismissible="!saving"
      scrollable
    >
      <template #body>
        <UForm ref="editorForm" :schema="ruleFormSchema" :state="state" class="space-y-5" @submit="save">
          <UFormField :label="t('rules.name')" name="name" :description="t('rules.nameDescription')" required>
            <UInput v-model="state.name" :placeholder="t('rules.namePlaceholder')" :disabled="saving" class="w-full" />
          </UFormField>
          <UFormField :label="t('rules.note')" name="description" :description="t('rules.noteDescription')">
            <UInput v-model="state.description" :placeholder="t('rules.notePlaceholder')" autocomplete="off" :disabled="saving" class="w-full" />
          </UFormField>
          <UFormField :label="t('rules.scope')" name="scope" :description="editing ? t('rules.scopeLocked') : t('rules.scopeDescription')" required>
            <USelect
              v-model="state.scope"
              :items="formScopeOptions"
              value-key="value"
              :disabled="saving || !!editing"
              class="w-full"
            />
          </UFormField>
          <UFormField :label="t('rules.action')" name="action" :description="t('rules.actionDescription')" required>
            <USelect v-model="state.action" :items="actionOptions" value-key="value" :disabled="saving" class="w-full" />
          </UFormField>
          <UFormField :label="t('rules.enabled')" name="enabled" :description="t('rules.enabledDescription')">
            <USwitch v-model="state.enabled" :disabled="saving" />
          </UFormField>

          <div class="space-y-5 border-t border-default pt-5">
            <div>
              <h3 class="text-sm font-medium text-highlighted">{{ t('rules.conditionsTitle') }}</h3>
              <p class="mt-1 text-sm text-muted">{{ t('rules.conditionsDescription') }}</p>
            </div>
            <div
              v-for="(condition, index) in state.conditions"
              :key="index"
              class="space-y-5"
            >
              <div class="flex items-center justify-between gap-2">
                <p class="text-sm font-medium text-highlighted">{{ t('rules.conditionLabel', { index: index + 1 }) }}</p>
                <UButton
                  type="button"
                  icon="i-lucide-trash-2"
                  color="error"
                  variant="ghost"
                  size="sm"
                  :aria-label="t('rules.removeCondition')"
                  :disabled="state.conditions.length === 1 || saving"
                  @click="removeCondition(index)"
                />
              </div>
              <UFormField :label="t('rules.target')" :name="`conditions.${index}.target`" required>
                <USelect
                  :model-value="condition.target"
                  :items="targetOptions"
                  value-key="value"
                  :disabled="saving"
                  class="w-full"
                  @update:model-value="setTarget(index, $event)"
                />
              </UFormField>
              <UFormField
                v-if="needsKey(condition.target)"
                :label="t('rules.key')"
                :name="`conditions.${index}.key`"
                :description="t('rules.keyDescription')"
                required
              >
                <UInput v-model="condition.key" :placeholder="keyPlaceholder(condition.target)" autocomplete="off" :disabled="saving" class="w-full" />
              </UFormField>
              <UFormField :label="t('rules.operator')" :name="`conditions.${index}.operator`" required>
                <USelect
                  v-model="condition.operator"
                  :items="operatorOptions(condition.target)"
                  value-key="value"
                  :disabled="saving"
                  class="w-full"
                />
              </UFormField>
              <UFormField :label="t('rules.value')" :name="`conditions.${index}.value`" required>
                <UInput v-model="condition.value" :placeholder="valuePlaceholder(condition)" autocomplete="off" :disabled="saving" class="w-full" />
              </UFormField>
            </div>
            <UTooltip :text="conditionLimitReached ? t('rules.conditionLimit') : t('rules.addCondition')">
              <UButton
                type="button"
                :label="t('rules.addCondition')"
                icon="i-lucide-plus"
                color="neutral"
                variant="outline"
                :disabled="conditionLimitReached || saving"
                @click="addCondition"
              />
            </UTooltip>
          </div>
          <UAlert v-if="formError" :title="formError" color="error" variant="subtle" />
        </UForm>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton :label="t('common.cancel')" color="neutral" variant="ghost" :disabled="saving" @click="editorOpen = false" />
          <UButton
            :label="editing ? t('common.saveChanges') : t('rules.add')"
            icon="i-lucide-check"
            :loading="saving"
            :disabled="unchanged"
            @click="editorForm?.submit()"
          />
        </div>
      </template>
    </UModal>

    <UModal
      :open="!!pendingDelete"
      :title="t('rules.delete')"
      :description="pendingDelete ? t('rules.deleteDescription', { name: pendingDelete.name }) : ''"
      :dismissible="!deleting"
      @update:open="open => { if (!open && !deleting) pendingDelete = null }"
    >
      <template #footer>
        <div class="flex w-full flex-col gap-3">
          <UAlert v-if="deleteError" :title="deleteError" color="error" variant="subtle" />
          <div class="flex justify-end gap-2">
            <UButton :label="t('common.cancel')" color="neutral" variant="ghost" :disabled="deleting" @click="pendingDelete = null" />
            <UButton :label="t('rules.delete')" icon="i-lucide-trash-2" color="error" :loading="deleting" @click="confirmDelete" />
          </div>
        </div>
      </template>
    </UModal>
  </main>
</template>
