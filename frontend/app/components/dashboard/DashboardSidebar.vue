<script setup lang="ts">
import type { NavigationMenuItem } from '@nuxt/ui'
import type { DashboardSection } from '~/types/dashboard'
const props = defineProps<{
  activeSection: DashboardSection
}>()
const open = defineModel<boolean>('open', {
  default: false
})
const emit = defineEmits<{
  navigate: [section: DashboardSection, label: string]
}>()
const toast = useToast()
const sections: {
  label: string
  icon: string
  target: DashboardSection
}[] = [{
  label: 'Overview',
  icon: 'i-lucide-layout-dashboard',
  target: 'overview'
}, {
  label: 'Traffic analytics',
  icon: 'i-lucide-chart-no-axes-combined',
  target: 'traffic'
}, {
  label: 'Security events',
  icon: 'i-lucide-shield-check',
  target: 'threats'
}, {
  label: 'Request logs',
  icon: 'i-lucide-scroll-text',
  target: 'logs'
}, {
  label: 'Blocked sources',
  icon: 'i-lucide-ban',
  target: 'blocked'
}]
const navigationItems = computed<NavigationMenuItem[]>(() => sections.map(item => ({
  label: item.label,
  icon: item.icon,
  active: props.activeSection === item.target,
  onSelect: () => emit('navigate', item.target, item.label)
})))
function showHelp() {
  toast.add({
    title: 'Choose a time range, explore the charts, or filter and export request logs.',
    icon: 'i-lucide-circle-check',
    color: 'success'
  })
}
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
      <UCard v-if="!collapsed" variant="subtle" :ui="{ body: 'p-3 sm:p-3' }">
        <div class="flex items-center gap-3">
          <UAvatar text="O" size="sm" :ui="{ root: 'bg-primary/10 text-primary' }" />
          <div class="min-w-0 flex-1">
            <p class="truncate text-xs font-semibold text-highlighted">
              OpenWAF workspace
            </p>
            <p class="mt-1 text-xs text-muted">
              Demo environment
            </p>
          </div>
          <UBadge
            label="Demo"
            color="primary"
            variant="soft"
            size="sm"
          />
        </div>
      </UCard>
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
      <div class="mt-auto flex flex-col gap-3 pt-8">
        <UCard v-if="!collapsed" variant="subtle" :ui="{ body: 'p-4 sm:p-4' }">
          <UIcon name="i-lucide-shield-check" class="mb-3 size-6 text-primary" />
          <p class="text-sm font-semibold text-highlighted">
            Your edge is protected
          </p>
          <p class="mt-2 text-xs leading-relaxed text-muted">
            Threats stopped. Traffic flowing.
            <br>
            </br>
            A little peace of mind.
          </p>
          <UBadge
            label="All systems operational"
            icon="i-lucide-circle-check"
            color="success"
            variant="subtle"
            size="sm"
            class="mt-4"
          />
        </UCard>
        <UButton
          icon="i-lucide-circle-help"
          :label="collapsed ? undefined : 'Dashboard help'"
          color="neutral"
          variant="ghost"
          :square="collapsed"
          :class="collapsed ? 'mx-auto' : 'justify-start'"
          aria-label="Dashboard help"
          @click="showHelp"
        />
      </div>
    </template>
    <template #footer="{ collapsed }">
      <UUser
        name="Alex Davis"
        description="Workspace admin · Demo"
        :avatar="{ text: 'AD' }"
        :ui="collapsed ? { wrapper: 'hidden' } : {}"
        class="py-2"
        :class="collapsed ? 'mx-auto' : 'w-full'"
      />
    </template>
  </UDashboardSidebar>
</template>
