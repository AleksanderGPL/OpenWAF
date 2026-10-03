<script setup lang="ts">
import type { TableColumn } from '@nuxt/ui'
import type { DashboardRange, RequestFilter, RequestLog } from '~/types/dashboard'
import type { LogPage } from '~/types/telemetry'
import { requestLogView } from '~/utils/telemetry'
const { t, locale } = useI18n()
const props = withDefaults(defineProps<{
  range: DashboardRange
  refreshToken: number
  pageSize?: number
  showRefresh?: boolean
}>(), {
  pageSize: 6,
  showRefresh: false
})
const logFilter = ref<RequestFilter>('All requests')
const search = ref('')
const searchQuery = ref('')
const page = ref(1)
const pageSize = computed(() => props.pageSize)
let searchTimer: ReturnType<typeof setTimeout> | undefined
watch(search, value => {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    searchQuery.value = value.trim()
  }, 300)
})
onUnmounted(() => clearTimeout(searchTimer))
watch([() => props.range, logFilter, searchQuery], () => {
  page.value = 1
})
const action = computed(() => logFilter.value === 'Blocked' ? 'blocked' : logFilter.value === 'Allowed' ? 'allowed' : undefined)
const query = computed(() => ({
  range: props.range,
  page: page.value,
  limit: pageSize.value,
  action: action.value,
  search: searchQuery.value || undefined
}))
const { data, error, status, refresh } = await useFetch<LogPage>('/api/logs', { query })
watch(() => props.refreshToken, () => {
  refresh()
})
const visibleLogs = computed(() => (data.value?.items ?? []).map(log => requestLogView(log, locale.value === 'pl' ? 'pl-PL' : 'en-GB')))
const total = computed(() => data.value?.total ?? 0)
const firstVisible = computed(() => total.value ? (page.value - 1) * pageSize.value + 1 : 0)
const lastVisible = computed(() => Math.min(page.value * pageSize.value, total.value))
const selectedLog = ref<RequestLog | null>(null)
const searchInput = useTemplateRef<{
  inputRef: HTMLInputElement
}>('searchInput')
const toast = useToast()
defineShortcuts({
  '/': () => {
    if (!selectedLog.value) searchInput.value?.inputRef?.focus()
  }
})
const logTabs = computed(() => [{
  label: t('logs.all'),
  value: 'All requests' as const
}, {
  label: t('logs.blocked'),
  value: 'Blocked' as const
}, {
  label: t('logs.allowed'),
  value: 'Allowed' as const
}])
const columns = computed<TableColumn<RequestLog>[]>(() => [{
  accessorKey: 'time',
  header: t('logs.time')
}, {
  accessorKey: 'ip',
  header: t('logs.sourceIp')
}, {
  id: 'request',
  header: t('logs.request')
}, {
  accessorKey: 'action',
  header: t('logs.action')
}, {
  accessorKey: 'rule',
  header: t('logs.rule')
}, {
  id: 'details',
  header: ''
}])
async function exportLogs() {
  try {
    const blob = await $fetch<Blob>('/api/logs/export', {
      query: {
        range: props.range,
        action: action.value,
        search: searchQuery.value || undefined
      },
      responseType: 'blob'
    })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = 'openwaf-request-logs.csv'
    link.click()
    setTimeout(() => URL.revokeObjectURL(url), 0)
    toast.add({
      title: t('logs.exported'),
      icon: 'i-lucide-circle-check',
      color: 'success'
    })
  } catch (exportError) {
    const message = (exportError as { data?: { message?: string } }).data?.message
    toast.add({
      title: message ? translateApiMessage(message) : t('logs.exportFailed'),
      icon: 'i-lucide-circle-alert',
      color: 'error'
    })
  }
}
</script>

<template>
  <UCard id="logs" class="scroll-mt-6" :ui="{ body: 'p-0 sm:p-0', footer: 'p-4 sm:px-6' }">
    <template #header>
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <div class="flex flex-wrap items-center gap-2">
            <h2 class="text-sm font-semibold text-highlighted">
              {{ t('logs.title') }}
            </h2>
            <UBadge
              :label="t(`logs.count.${pluralForm(total)}`, { count: total })"
              color="neutral"
              variant="subtle"
              size="sm"
            />
          </div>
          <p class="mt-1 text-xs text-muted">
            {{ t('logs.description') }}
          </p>
        </div>
        <div class="flex items-center gap-2">
          <UButton
            v-if="showRefresh"
            :label="t('logs.refresh')"
            icon="i-lucide-refresh-cw"
            color="neutral"
            variant="outline"
            size="sm"
            :loading="status === 'pending'"
            @click="refresh()"
          />
          <UButton
            :label="t('logs.export')"
            icon="i-lucide-download"
            color="neutral"
            variant="outline"
            size="sm"
            @click="exportLogs"
          />
        </div>
      </div>
    </template>
    <div class="flex flex-wrap items-center justify-between gap-4 p-4 sm:px-6">
      <UTabs
        v-model="logFilter"
        :items="logTabs"
        :content="false"
        variant="link"
        size="sm"
        :aria-label="t('logs.filter')"
        class="max-w-full"
      />
      <UInput
        ref="searchInput"
        v-model="search"
        icon="i-lucide-search"
        type="search"
        :placeholder="t('logs.searchPlaceholder')"
        :aria-label="t('logs.search')"
        class="w-full sm:w-64"
      >
        <template #trailing>
          <UKbd value="/" />
        </template>
      </UInput>
    </div>
    <UAlert
      v-if="error"
      class="mx-4 mb-4 sm:mx-6"
      color="error"
      variant="subtle"
      :title="t('logs.loadFailed')"
    />
    <UTable
      :data="visibleLogs"
      :columns="columns"
      :loading="status === 'pending'"
      :get-row-id="row => row.id"
      :empty="t('logs.empty')"
      :ui="{ th: 'bg-elevated/50 text-xs', td: 'text-xs', tr: 'hover:bg-elevated/30' }"
    >
      <template #time-cell="{ row }">
        <span class="font-mono text-muted">
          {{ row.original.time }}
        </span>
      </template>
      <template #ip-cell="{ row }">
        <p class="font-mono text-highlighted">
          {{ row.original.ip }}
        </p>
        <div class="mt-1 flex items-center gap-1.5 text-muted">
          <UBadge
            :label="row.original.code"
            color="neutral"
            variant="soft"
            size="sm"
          />
          <span v-if="row.original.country">
            {{ row.original.country }}
          </span>
        </div>
      </template>
      <template #request-cell="{ row }">
        <div class="flex items-center gap-2">
          <UBadge
            :label="row.original.method"
            :color="row.original.method === 'POST' ? 'primary' : 'info'"
            variant="soft"
            size="sm"
          />
          <span class="font-mono text-muted">
            {{ row.original.path }}
          </span>
        </div>
      </template>
      <template #action-cell="{ row }">
        <UBadge
          :label="t(row.original.action === 'Blocked' ? 'logs.blocked' : 'logs.allowed')"
          :color="getRequestActionColor(row.original.action)"
          variant="subtle"
          size="sm"
        />
      </template>
      <template #details-cell="{ row }">
        <UTooltip :text="t('logs.view')">
          <UButton
            icon="i-lucide-chevron-right"
            color="neutral"
            variant="ghost"
            size="sm"
            :aria-label="t('logs.viewId', { id: row.original.id })"
            @click="selectedLog = row.original"
          />
        </UTooltip>
      </template>
    </UTable>
    <template #footer>
      <div class="flex flex-wrap items-center justify-between gap-3">
        <p class="text-xs text-muted">
          {{ t('logs.showing', { from: firstVisible, to: lastVisible, total }) }}
        </p>
        <UPagination
          v-model:page="page"
          :items-per-page="pageSize"
          :total="total"
          :sibling-count="0"
          size="sm"
        />
      </div>
    </template>
  </UCard>
  <DashboardRequestDetails v-model="selectedLog" />
</template>
