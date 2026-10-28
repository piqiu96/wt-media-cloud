<script setup>
import { ErrorCircleIcon } from 'tdesign-icons-vue-next'
defineProps({ tone: { type: String, default: 'neutral' }, details: { type: String, default: '' }, detailsLabel: { type: String, default: '查看说明' } })
</script>

<template>
  <div class="resource-notice" :class="`is-${tone}`" role="note">
    <ErrorCircleIcon v-if="tone === 'warning'" class="resource-notice__icon" />
    <div><div class="resource-notice__text"><slot /></div><details v-if="details"><summary>{{ detailsLabel }}</summary><p>{{ details }}</p></details></div>
  </div>
</template>

<style scoped>
.resource-notice { display: flex; gap: 8px; margin: 12px 0; color: var(--wt-text-tertiary); font-size: 12px; line-height: 1.7; overflow-wrap: anywhere; }
.resource-notice:first-child { margin-top: 0; }
.resource-notice:last-child { margin-bottom: 0; }
.resource-notice.is-warning { color: var(--wt-text-secondary); border-left: 2px solid var(--wt-warning); padding-left: 10px; }
.resource-notice__icon { flex: 0 0 16px; margin-top: 2px; color: var(--wt-warning); }
.resource-notice details { margin-top: 2px; }
.resource-notice summary { cursor: pointer; width: fit-content; color: var(--wt-text-secondary); }
.resource-notice p { margin: 6px 0 0; }
</style>
