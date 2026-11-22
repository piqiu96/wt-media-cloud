<script setup>
import { ErrorCircleIcon } from 'tdesign-icons-vue-next'
defineProps({ tone: { type: String, default: 'neutral' }, title: { type: String, default: '' }, details: { type: String, default: '' }, detailsLabel: { type: String, default: '查看说明' } })
</script>

<template>
  <div class="resource-notice" :class="[`is-${tone}`, { 'has-details': details }]" role="note">
    <ErrorCircleIcon v-if="tone !== 'neutral' || details" class="resource-notice__icon" />
    <div><strong v-if="title" class="resource-notice__title">{{ title }}</strong><div class="resource-notice__text"><slot /></div><details v-if="details"><summary>{{ detailsLabel }}<span class="resource-notice__chevron" aria-hidden="true">⌄</span></summary><p>{{ details }}</p></details></div>
    <div v-if="$slots.actions" class="resource-notice__actions"><slot name="actions" /></div>
  </div>
</template>

<style scoped>
.resource-notice { display: flex; align-items: flex-start; gap: 8px; margin: 12px 0; color: var(--wt-text-secondary); font-size: 12px; line-height: 20px; overflow-wrap: anywhere; }
.resource-notice:first-child { margin-top: 0; }
.resource-notice:last-child { margin-bottom: 0; }
.resource-notice.is-warning, .resource-notice.is-info, .resource-notice.is-success, .resource-notice.has-details { padding: 8px 16px; min-height: 48px; box-sizing: border-box; border: 1px solid var(--wt-border); border-radius: 6px; background: var(--wt-color-bg-subtle); color: var(--wt-text-primary); font-size: 13px; }
.resource-notice.is-warning { background: var(--wt-warning-bg); border-color: var(--wt-color-warning-border); }
.resource-notice.is-info { background: var(--wt-info-bg); border-color: var(--wt-color-primary-border); }
.resource-notice.is-success { background: var(--wt-success-bg); border-color: var(--wt-color-success-border); }
.resource-notice__title { display: block; margin-bottom: 3px; font-size: 14px; font-weight: 600; }
.resource-notice > .resource-notice__actions { flex: 0 1 auto; display: flex; align-items: center; flex-wrap: wrap; gap: 8px; }
@media (max-width: 680px) { .resource-notice:has(.resource-notice__actions) { flex-wrap: wrap; } .resource-notice > .resource-notice__actions { flex-basis: 100%; margin-left: 24px; } }
.resource-notice > div { flex: 1; min-width: 0; }
.resource-notice__icon { flex: 0 0 16px; margin-top: 2px; color: var(--wt-text-tertiary); }
.resource-notice.is-warning .resource-notice__icon { color: var(--wt-warning); }
.resource-notice details { margin-top: 4px; }
.resource-notice summary { display: flex; align-items: center; gap: 6px; cursor: pointer; width: fit-content; color: var(--wt-primary); list-style: none; border-radius: 4px; }
.resource-notice summary::-webkit-details-marker { display: none; }
.resource-notice summary:focus-visible { outline: 2px solid var(--wt-primary); outline-offset: 3px; }
.resource-notice__chevron { transition: transform .15s; }
.resource-notice details[open] .resource-notice__chevron { transform: rotate(180deg); }
.resource-notice p { margin: 10px 0 0; padding-top: 10px; border-top: 1px solid var(--wt-border); white-space: pre-wrap; }
/* Notice actions are auxiliary links, including when hosted in a form. */
.resource-notice__actions :deep(.t-button) { height:32px; min-height:32px; padding:0 8px; border:0; border-radius:4px; background:transparent; color:var(--wt-primary); font-size:13px; font-weight:400; }
.resource-notice__actions :deep(.t-button:hover:not(:disabled):not(.t-is-disabled)) { color:var(--wt-primary-hover); background:var(--wt-info-bg); }
.resource-notice__actions :deep(.t-button.t-is-disabled) { color:var(--td-text-color-disabled); background:transparent; }
.resource-notice:has(.resource-notice__title), .resource-notice.has-details { align-items:flex-start; }
</style>
