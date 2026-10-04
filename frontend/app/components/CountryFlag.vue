<script setup lang="ts">
const props = defineProps<{
  code: string | null
  label?: string
}>()
const iso = computed(() => {
  const code = props.code?.trim().toLowerCase() ?? ''
  return /^[a-z]{2}$/.test(code) ? code : ''
})
</script>

<template>
  <span
    v-if="iso"
    class="fi shrink-0 rounded-[2px] shadow-[0_0_0_1px_rgba(255,255,255,0.16)]"
    :class="`fi-${iso}`"
    role="img"
    :aria-label="label || code || undefined"
  />
  <span v-else class="inline-block h-3.5 w-5 shrink-0 rounded-[2px] bg-elevated" aria-hidden="true" />
</template>

<style scoped>
.fi {
  width: 1.25rem;
  height: 0.875rem;
  background-size: cover;
  line-height: 0.875rem;
  font-size: 0.875rem;
}
.fi::before {
  content: none;
}
</style>
