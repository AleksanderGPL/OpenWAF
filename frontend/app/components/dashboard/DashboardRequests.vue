<script setup lang="ts">
import type { TableColumn } from '@nuxt/ui'
import type { DashboardRange, RequestFilter, RequestLog } from '~/types/dashboard'
import type { LogPage } from '~/types/telemetry'
import { requestLogView } from '~/utils/telemetry'
const props = defineProps<{
  range: DashboardRange
  refreshToken: number
}>()
const logFilter = ref<RequestFilter>('All requests')
const search = ref('')
const searchQuery = ref('')
const page = ref(1)
const pageSize = 6
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
  limit: pageSize,
  action: action.value,
  search: searchQuery.value || undefined
}))
const { data, error, refresh } = await useFetch<LogPage>('/api/logs', { query })
watch(() => props.refreshToken, () => {
  refresh()
})
const visibleLogs = computed(() => (data.value?.items ?? []).map(requestLogView))
const total = computed(() => data.value?.total ?? 0)
const firstVisible = computed(() => total.value ? (page.value - 1) * pageSize + 1 : 0)
const lastVisible = computed(() => Math.min(page.value * pageSize, total.value))
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
const logTabs = ['All requests', 'Blocked', 'Allowed'].map(label => ({
  label,
  value: label
}))
const columns: TableColumn<RequestLog>[] = [{
  accessorKey: 'time',
  header: 'Time'
}, {
  accessorKey: 'ip',
  header: 'Source IP'
}, {
  id: 'request',
  header: 'Request'
}, {
  accessorKey: 'action',
  header: 'Action'
}, {
  accessorKey: 'rule',
  header: 'Matched rule'
}, {
  id: 'details',
  header: ''
}]
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
      title: 'Request logs exported',
      icon: 'i-lucide-circle-check',
      color: 'success'
    })
  } catch (exportError) {
    const message = (exportError as { data?: { message?: string } }).data?.message
    toast.add({
      title: message || 'Could not export logs',
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
              Recent requests
            </h2>
            <UBadge
              :label="`${total} requests`"
              color="neutral"
              variant="subtle"
              size="sm"
            />
          </div>
          <p class="mt-1 text-xs text-muted">
            The latest activity passing through your firewall
          </p>
        </div>
        <UButton
          label="Export logs"
          icon="i-lucide-download"
          color="neutral"
          variant="outline"
          size="sm"
          @click="exportLogs"
        />
      </div>
    </template>
    <div class="flex flex-wrap items-center justify-between gap-4 p-4 sm:px-6">
      <UTabs
        v-model="logFilter"
        :items="logTabs"
        :content="false"
        variant="link"
        size="sm"
        aria-label="Filter request action"
        class="max-w-full"
      />
      <UInput
        ref="searchInput"
        v-model="search"
        icon="i-lucide-search"
        type="search"
        placeholder="Search IP, path, or rule…"
        aria-label="Search request logs"
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
      title="Could not load request logs"
    />
    <UTable
      :data="visibleLogs"
      :columns="columns"
      :get-row-id="row => row.id"
      empty="No requests match your filters. Try another IP, path, or rule."
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
          :label="row.original.action"
          :color="getRequestActionColor(row.original.action)"
          variant="subtle"
          size="sm"
        />
      </template>
      <template #details-cell="{ row }">
        <UTooltip text="View request details">
          <UButton
            icon="i-lucide-chevron-right"
            color="neutral"
            variant="ghost"
            size="sm"
            :aria-label="`View request ${row.original.id}`"
            @click="selectedLog = row.original"
          />
        </UTooltip>
      </template>
    </UTable>
    <template #footer>
      <div class="flex flex-wrap items-center justify-between gap-3">
        <p class="text-xs text-muted">
          Showing
          <span class="font-medium text-highlighted">
            {{ firstVisible }}
            –
            {{ lastVisible }}
          </span>
          of
          <span class="font-medium text-highlighted">
            {{ total }}
          </span>
          requests
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
