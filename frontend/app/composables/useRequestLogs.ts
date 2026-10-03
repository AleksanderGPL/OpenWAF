import { computed, ref, toValue, watch, type MaybeRefOrGetter } from 'vue'
import type { RequestFilter, RequestLog } from '~/types/dashboard'
export function useRequestLogs(logs: MaybeRefOrGetter<readonly RequestLog[]>) {
  const logFilter = ref<RequestFilter>('All requests')
  const search = ref('')
  const page = ref(1)
  const pageSize = 6
  const filteredLogs = computed(() => {
    const query = search.value.trim().toLowerCase()
    return toValue(logs).filter(log => {
      const matchesAction = logFilter.value === 'All requests' || log.action === logFilter.value
      const matchesSearch = !query || [log.ip, log.path, log.country, log.rule, log.id].some(value => value.toLowerCase().includes(query))
      return matchesAction && matchesSearch
    })
  })
  const visibleLogs = computed(() => filteredLogs.value.slice((page.value - 1) * pageSize, page.value * pageSize))
  const firstVisible = computed(() => filteredLogs.value.length ? (page.value - 1) * pageSize + 1 : 0)
  const lastVisible = computed(() => Math.min(page.value * pageSize, filteredLogs.value.length))
  watch([search, logFilter], () => {
    page.value = 1
  })
  watch(() => filteredLogs.value.length, total => {
    page.value = Math.min(page.value, Math.max(1, Math.ceil(total / pageSize)))
  })
  return {
    logFilter,
    search,
    page,
    pageSize,
    filteredLogs,
    visibleLogs,
    firstVisible,
    lastVisible
  }
}
