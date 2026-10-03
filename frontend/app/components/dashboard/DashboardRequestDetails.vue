<script setup lang="ts">
import type { RequestLog } from '~/types/dashboard'
const request = defineModel<RequestLog | null>({
  required: true
})
const { t } = useI18n()
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
    :title="t('request.title')"
    :description="t('request.description')"
    :ui="{ footer: 'justify-end' }"
  >
    <template #body>
      <template v-if="request">
        <UBadge :label="t(request.action === 'Blocked' ? 'logs.blocked' : 'logs.allowed')" :color="getRequestActionColor(request.action)" variant="subtle" />
        <dl class="mt-5 divide-y divide-default text-sm">
          <div class="flex justify-between gap-4 py-3">
            <dt class="text-muted">
              {{ t('request.id') }}
            </dt>
            <dd class="font-mono text-xs">
              {{ request.id }}
            </dd>
          </div>
          <div class="flex justify-between gap-4 py-3">
            <dt class="text-muted">
              {{ t('request.time') }}
            </dt>
            <dd>
              {{ request.time }}
            </dd>
          </div>
          <div class="flex justify-between gap-4 py-3">
            <dt class="text-muted">
              {{ t('request.sourceIp') }}
            </dt>
            <dd class="font-mono text-xs">
              {{ request.ip }}
            </dd>
          </div>
          <div class="flex justify-between gap-4 py-3">
            <dt class="text-muted">
              {{ t('request.location') }}
            </dt>
            <dd>
              {{ request.code === '—' ? t('common.unknown') : request.code }}
            </dd>
          </div>
          <div class="flex justify-between gap-4 py-3">
            <dt class="text-muted">
              {{ t('request.request') }}
            </dt>
            <dd class="max-w-2/3 break-all text-right font-mono text-xs">
              {{ request.method }}
              {{ request.path }}
            </dd>
          </div>
          <div class="flex justify-between gap-4 py-3">
            <dt class="text-muted">
              {{ t('request.rule') }}
            </dt>
            <dd>
              {{ request.rule }}
            </dd>
          </div>
          <div class="flex justify-between gap-4 py-3">
            <dt class="text-muted">
              {{ t('request.hostname') }}
            </dt>
            <dd class="font-mono text-xs">
              {{ request.hostname || '—' }}
            </dd>
          </div>
          <div class="flex justify-between gap-4 py-3">
            <dt class="text-muted">
              {{ t('request.duration') }}
            </dt>
            <dd>
              {{ t('request.durationValue', { value: formatLatency(request.durationMs) }) }}
            </dd>
          </div>
          <div class="flex justify-between gap-4 py-3">
            <dt class="text-muted">
              {{ t('request.bodySize') }}
            </dt>
            <dd>
              {{ t('request.bodySizeValue', { inbound: formatBytes(request.requestBytes), outbound: formatBytes(request.responseBytes) }) }}
            </dd>
          </div>
          <div class="flex justify-between gap-4 py-3">
            <dt class="text-muted">
              {{ t('request.status') }}
            </dt>
            <dd>
              {{ request.status }}
            </dd>
          </div>
        </dl>
      </template>
    </template>
    <template #footer>
      <UButton :label="t('common.done')" @click="requestModalOpen = false" />
    </template>
  </UModal>
</template>
