<script setup lang="ts">
import type { DashboardSection } from '~/types/dashboard'
import { useMediaQuery } from '@vueuse/core'

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
  '/dash/settings': 'settings',
  '/dash/services': 'services'
} as const
const routedPage = computed(() => routedPages[route.path as keyof typeof routedPages] ?? null)
const pageTitle = computed(() => {
  if (routedPage.value === 'settings') return t('nav.settings')
  if (routedPage.value === 'services') return t('nav.services')
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
    <UDashboardPanel id="overview-panel" :ui="{ body: 'bg-muted/40' }">
      <template #header>
        <UDashboardNavbar :title="pageTitle">
          <template #leading>
            <UDashboardSidebarCollapse />
          </template>
          <template #right>
            <ULocaleSelect v-model="selectedLocale" :locales="localeOptions" class="w-36" :aria-label="t('settings.language')" />
            <UButton
              :label="desktop && agentVisible ? 'Hide agent' : 'Ask agent'"
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
