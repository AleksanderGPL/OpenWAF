<script setup lang="ts">
import type { NavigationMenuItem } from '@nuxt/ui'
import type { DashboardSection } from '~/types/dashboard'
const props = defineProps<{
  activeSection: DashboardSection | 'settings'
}>()
const open = defineModel<boolean>('open', {
  default: false
})
const emit = defineEmits<{
  navigate: [section: DashboardSection, label: string]
}>()
const auth = useAuthStore()
const { user } = storeToRefs(auth)
const displayName = computed(() => user.value?.name || user.value?.username || 'Account')
const initials = computed(() => displayName.value.replace(/[^a-zA-Z0-9]/g, '').slice(0, 2).toUpperCase() || 'OW')

async function onSignOut() {
  await auth.signOut()
  await navigateTo('/auth')
}
const sections: {
  label: string
  icon: string
  target: DashboardSection
}[] = [{
  label: 'Overview',
  icon: 'i-lucide-layout-dashboard',
  target: 'overview'
}]
const navigationItems = computed<NavigationMenuItem[]>(() => [...sections.map(item => ({
  label: item.label,
  icon: item.icon,
  active: props.activeSection === item.target,
  onSelect: () => emit('navigate', item.target, item.label)
})), {
  label: 'Settings',
  icon: 'i-lucide-settings',
  to: '/dash/settings',
  active: props.activeSection === 'settings',
  onSelect: () => { open.value = false }
}])
</script>

<template>
  <UDashboardSidebar
    id="main"
    v-model:open="open"
    collapsible
    resizable
    :default-size="260"
    :min-size="220"
    :max-size="340"
    :collapsed-size="72"
    :ui="{ root: 'bg-default', header: 'border-b border-default', footer: 'border-t border-default' }"
  >
    <template #header="{ collapsed }">
      <UButton
        icon="i-lucide-shield-check"
        color="primary"
        variant="soft"
        :label="collapsed ? undefined : 'OpenWAF'"
        :square="collapsed"
        class="font-bold"
        :class="collapsed ? 'mx-auto' : 'w-full justify-start'"
        aria-label="OpenWAF overview"
        @click="emit('navigate', 'overview', 'Overview')"
      />
    </template>
    <template #default="{ collapsed }">
      <p v-if="!collapsed" class="px-2 pt-2 text-xs font-medium tracking-wider text-dimmed">
        WORKSPACE
      </p>
      <UNavigationMenu
        :items="navigationItems"
        orientation="vertical"
        :collapsed="collapsed"
        tooltip
        popover
        class="w-full"
      />
    </template>
    <template #footer="{ collapsed }">
      <div class="flex items-center gap-1 py-2" :class="collapsed ? 'flex-col' : 'w-full'">
        <UUser
          :name="displayName"
          :description="collapsed ? undefined : user?.username"
          :avatar="{ text: initials }"
          :ui="collapsed ? { wrapper: 'hidden' } : {}"
          class="min-w-0 flex-1"
        />
        <UButton
          icon="i-lucide-log-out"
          color="neutral"
          variant="ghost"
          square
          aria-label="Sign out"
          @click="onSignOut"
        />
      </div>
    </template>
  </UDashboardSidebar>
</template>
