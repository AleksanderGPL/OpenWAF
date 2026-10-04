<script setup lang="ts">
import type { DashboardSection } from '~/types/dashboard'
import { useMediaQuery } from '@vueuse/core'

const { t } = useI18n()
const auth = useAuthStore()
const { user } = storeToRefs(auth)
const route = useRoute()

const sidebarOpen = ref(false)
const agentOpen = ref(false)
const agentVisible = ref(true)
const desktop = useMediaQuery('(min-width: 1280px)')
function toggleAgent() {
  if (desktop.value) agentVisible.value = !agentVisible.value
  else agentOpen.value = !agentOpen.value
}
const activeSection = ref<DashboardSection>('overview')
const routedPages = {
  '/dash/logs': 'logs',
  '/dash/settings': 'settings',
  '/dash/services': 'services',
  '/dash/rules': 'rules'
} as const
const routedPage = computed(() => routedPages[route.path as keyof typeof routedPages] ?? null)
const pageTitle = computed(() => {
  if (routedPage.value === 'logs') return t('nav.logs')
  if (routedPage.value === 'settings') return t('nav.settings')
  if (routedPage.value === 'services') return t('nav.services')
  if (routedPage.value === 'rules') return t('nav.rules')
  return t('nav.overview')
})
async function navigate(section: DashboardSection) {
  activeSection.value = section
  sidebarOpen.value = false
  if (route.path !== '/dash/overview') {
    await navigateTo({ path: '/dash/overview', hash: `#${section}` })
    return
  }
  await nextTick()
  document.getElementById(section)?.scrollIntoView({
    behavior: 'smooth',
    block: 'start'
  })
}
</script>

<template>
  <UDashboardGroup v-if="user" unit="px" storage="local" storage-key="openwaf-dashboard">
    <DashboardSidebar v-model:open="sidebarOpen" :active-section="routedPage ?? activeSection" @navigate="navigate" />
    <UDashboardPanel id="overview-panel" :ui="{ body: 'bg-muted' }">
      <template #header>
        <UDashboardNavbar :title="pageTitle">
          <template #leading>
            <UDashboardSidebarCollapse />
          </template>
          <template #right>
            <UButton
              :label="desktop && agentVisible ? t('agent.hide') : t('agent.ask')"
              icon="i-lucide-sparkles"
              color="primary"
              variant="soft"
              :aria-expanded="desktop ? agentVisible : agentOpen"
              aria-controls="openwaf-agent-panel"
              @click="toggleAgent"
            />
          </template>
        </UDashboardNavbar>
      </template>
      <template #body>
        <slot />
      </template>
    </UDashboardPanel>
    <DashboardAgent v-model:open="agentOpen" v-model:visible="agentVisible" />
  </UDashboardGroup>
</template>
