<script setup lang="ts">
import type { DashboardSection } from '~/types/dashboard'

const auth = useAuthStore()
const { user } = storeToRefs(auth)
const route = useRoute()

const sidebarOpen = ref(false)
const activeSection = ref<DashboardSection>('overview')
const activeNav = ref('Overview')
const isSettings = computed(() => route.path === '/dash/settings')
const pageTitle = computed(() => isSettings.value ? 'Settings' : activeNav.value)
async function navigate(section: DashboardSection, label: string) {
  activeSection.value = section
  activeNav.value = label
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
    <DashboardSidebar v-model:open="sidebarOpen" :active-section="isSettings ? 'settings' : activeSection" @navigate="navigate" />
    <UDashboardPanel id="overview-panel" :ui="{ body: 'bg-muted/40' }">
      <template #header>
        <UDashboardNavbar :title="pageTitle">
          <template #leading>
            <UDashboardSidebarCollapse />
          </template>
        </UDashboardNavbar>
      </template>
      <template #body>
        <slot />
      </template>
    </UDashboardPanel>
  </UDashboardGroup>
</template>
