<script setup lang="ts">
import { Markdown } from '@comark/vue'
import security from '@comark/vue/plugins/security'

defineProps<{ text: string, streaming: boolean }>()
const options = { registerDefaultPlugins: false }
const plugins = [security()]
</script>

<template>
  <div class="agent-markdown min-w-0 text-sm leading-relaxed">
    <Suspense>
      <Markdown :value="text" :streaming="streaming" :caret="streaming" :options="options" :plugins="plugins" />
      <template #fallback>
        <p class="whitespace-pre-wrap break-words">{{ text }}</p>
      </template>
    </Suspense>
  </div>
</template>

<style scoped>
.agent-markdown :deep(p),
.agent-markdown :deep(ul),
.agent-markdown :deep(ol),
.agent-markdown :deep(blockquote),
.agent-markdown :deep(pre),
.agent-markdown :deep(table) {
  margin-block: 0.75rem;
}
.agent-markdown :deep(:first-child) { margin-top: 0; }
.agent-markdown :deep(:last-child) { margin-bottom: 0; }
.agent-markdown :deep(p), .agent-markdown :deep(li) { overflow-wrap: anywhere; }
.agent-markdown :deep(h1), .agent-markdown :deep(h2), .agent-markdown :deep(h3),
.agent-markdown :deep(h4), .agent-markdown :deep(h5), .agent-markdown :deep(h6) {
  margin-block: 1rem 0.5rem;
  font-weight: 600;
  color: var(--ui-text-highlighted);
}
.agent-markdown :deep(h1) { font-size: 1.125rem; }
.agent-markdown :deep(ul) { list-style: disc; padding-left: 1.25rem; }
.agent-markdown :deep(ol) { list-style: decimal; padding-left: 1.25rem; }
.agent-markdown :deep(li + li) { margin-top: 0.25rem; }
.agent-markdown :deep(a) { color: var(--ui-primary); text-decoration: underline; text-underline-offset: 3px; }
.agent-markdown :deep(blockquote) {
  border-left: 2px solid var(--ui-border-accented);
  padding-left: 0.75rem;
  color: var(--ui-text-muted);
}
.agent-markdown :deep(code) { font-size: 0.8em; background: var(--ui-bg-muted); padding: 0.125rem 0.25rem; border-radius: var(--ui-radius); }
.agent-markdown :deep(pre) { overflow-x: auto; padding: 0.75rem; background: var(--ui-bg-muted); border-radius: var(--ui-radius); }
.agent-markdown :deep(pre code) { padding: 0; background: transparent; white-space: pre; }
.agent-markdown :deep(table) { display: block; max-width: 100%; overflow-x: auto; border-collapse: collapse; }
.agent-markdown :deep(th), .agent-markdown :deep(td) { padding: 0.375rem 0.5rem; border: 1px solid var(--ui-border); text-align: left; }
.agent-markdown :deep(th) { font-weight: 600; background: var(--ui-bg-muted); }
.agent-markdown :deep(hr) { margin-block: 1rem; border-color: var(--ui-border); }
</style>
