<script setup lang="ts">
import type { DashboardSection } from '~/types/dashboard'
const sidebarOpen = ref(false)
const activeSection = ref<DashboardSection>('overview')
const activeNav = ref('Overview')
async function navigate(section: DashboardSection, label: string) {
  activeSection.value = section
  activeNav.value = label
  sidebarOpen.value = false
  await nextTick()
  document.getElementById(section)?.scrollIntoView({
    behavior: 'smooth',
    block: 'start'
  })
}
</script>

<template>
  <UDashboardGroup unit="px" storage="local" storage-key="openwaf-dashboard">
    <DashboardSidebar v-model:open="sidebarOpen" :active-section="activeSection" @navigate="navigate" />
    <UDashboardPanel id="overview-panel" :ui="{ body: 'bg-muted/40' }">
      <template #header>
        <UDashboardNavbar :title="activeNav">
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
