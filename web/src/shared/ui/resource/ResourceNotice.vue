<script setup>
import { ErrorCircleIcon } from 'tdesign-icons-vue-next'
defineProps({ tone: { type: String, default: 'neutral' }, details: { type: String, default: '' }, detailsLabel: { type: String, default: '查看说明' } })
</script>

<template>
  <div class="resource-notice" :class="[`is-${tone}`, { 'has-details': details }]" role="note">
    <ErrorCircleIcon v-if="tone === 'warning' || details" class="resource-notice__icon" />
    <div><div class="resource-notice__text"><slot /></div><details v-if="details"><summary>{{ detailsLabel }}<span class="resource-notice__chevron" aria-hidden="true">⌄</span></summary><p>{{ details }}</p></details></div>
  </div>
</template>

<style scoped>
.resource-notice { display: flex; gap: 8px; margin: 12px 0; color: var(--wt-text-tertiary); font-size: 12px; line-height: 1.7; overflow-wrap: anywhere; }
.resource-notice:first-child { margin-top: 0; }
.resource-notice:last-child { margin-bottom: 0; }
.resource-notice.is-warning, .resource-notice.has-details { padding: 12px 16px; border: 1px solid var(--wt-border); border-radius: 6px; background: var(--wt-color-bg-subtle); color: var(--wt-text-primary); font-size: 13px; }
.resource-notice.is-warning { background: var(--wt-warning-bg); border-color: var(--wt-color-warning-border); }
.resource-notice > div { flex: 1; min-width: 0; }
.resource-notice__icon { flex: 0 0 16px; margin-top: 3px; color: var(--wt-text-tertiary); }
.resource-notice.is-warning .resource-notice__icon { color: var(--wt-warning); }
.resource-notice details { margin-top: 4px; }
.resource-notice summary { display: flex; align-items: center; gap: 6px; cursor: pointer; width: fit-content; color: var(--wt-primary); list-style: none; border-radius: 4px; }
.resource-notice summary::-webkit-details-marker { display: none; }
.resource-notice summary:focus-visible { outline: 2px solid var(--wt-primary); outline-offset: 3px; }
.resource-notice__chevron { transition: transform .15s; }
.resource-notice details[open] .resource-notice__chevron { transform: rotate(180deg); }
.resource-notice p { margin: 10px 0 0; padding-top: 10px; border-top: 1px solid var(--wt-border); white-space: pre-wrap; }
</style>
