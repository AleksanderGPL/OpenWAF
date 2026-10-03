<script setup lang="ts">
import type { RequestLog } from '~/types/dashboard'
const request = defineModel<RequestLog | null>({
  required: true
})
const requestModalOpen = computed({
  get: () => request.value !== null,
  set: (open: boolean) => {
    if (!open) request.value = null
  }
})
</script>

<template>
  <UModal
    v-model:open="requestModalOpen"
    title="Request details"
    description="Recorded by the proxy."
    :ui="{ footer: 'justify-end' }"
  >
    <template #body>
      <template v-if="request">
        <UBadge :label="request.action" :color="getRequestActionColor(request.action)" variant="subtle" />
        <dl class="mt-5 divide-y divide-default text-sm">
          <div class="flex justify-between gap-4 py-3">
            <dt class="text-muted">
              Request ID
            </dt>
            <dd class="font-mono text-xs">
              {{ request.id }}
            </dd>
          </div>
          <div class="flex justify-between gap-4 py-3">
            <dt class="text-muted">
              Time
            </dt>
            <dd>
              {{ request.time }}
            </dd>
          </div>
          <div class="flex justify-between gap-4 py-3">
            <dt class="text-muted">
              Source IP
            </dt>
            <dd class="font-mono text-xs">
              {{ request.ip }}
            </dd>
          </div>
          <div class="flex justify-between gap-4 py-3">
            <dt class="text-muted">
              Location
            </dt>
            <dd>
              {{ request.code === '—' ? 'Unknown' : request.code }}
            </dd>
          </div>
          <div class="flex justify-between gap-4 py-3">
            <dt class="text-muted">
              Request
            </dt>
            <dd class="max-w-2/3 break-all text-right font-mono text-xs">
              {{ request.method }}
              {{ request.path }}
            </dd>
          </div>
          <div class="flex justify-between gap-4 py-3">
            <dt class="text-muted">
              Matched rule
            </dt>
            <dd>
              {{ request.rule }}
            </dd>
          </div>
          <div class="flex justify-between gap-4 py-3">
            <dt class="text-muted">
              Hostname
            </dt>
            <dd class="font-mono text-xs">
              {{ request.hostname || '—' }}
            </dd>
          </div>
          <div class="flex justify-between gap-4 py-3">
            <dt class="text-muted">
              Duration
            </dt>
            <dd>
              {{ formatLatency(request.durationMs) }} ms
            </dd>
          </div>
          <div class="flex justify-between gap-4 py-3">
            <dt class="text-muted">
              Body size
            </dt>
            <dd>
              {{ formatBytes(request.requestBytes) }} in · {{ formatBytes(request.responseBytes) }} out
            </dd>
          </div>
          <div class="flex justify-between gap-4 py-3">
            <dt class="text-muted">
              Response status
            </dt>
            <dd>
              {{ request.status }}
            </dd>
          </div>
        </dl>
      </template>
    </template>
    <template #footer>
      <UButton label="Done" @click="requestModalOpen = false" />
    </template>
  </UModal>
</template>
