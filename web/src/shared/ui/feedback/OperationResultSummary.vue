<script setup>
import { CheckCircleIcon, ErrorCircleIcon, InfoCircleIcon } from 'tdesign-icons-vue-next'

defineProps({
  title: { type: String, required: true },
  description: { type: String, default: '' },
  tone: { type: String, default: 'success' },
  running: Boolean,
  counts: { type: Array, default: () => [] },
})
</script>

<template>
  <div class="operation-result-summary" role="status" aria-live="polite">
    <div class="operation-result-summary__symbol" :class="`is-${running ? 'running' : tone}`">
      <span v-if="running" class="operation-result-summary__spinner" />
      <CheckCircleIcon v-else-if="tone === 'success'" />
      <ErrorCircleIcon v-else-if="tone === 'danger'" />
      <InfoCircleIcon v-else />
    </div>
    <h3>{{ title }}</h3>
    <p v-if="description">{{ description }}</p>
    <div v-if="$slots.default" class="operation-result-summary__note"><slot /></div>
  </div>
  <dl v-if="counts.length" class="operation-result-counts">
    <div v-for="item in counts" :key="item.key" :class="`is-${item.tone || 'neutral'}`"><dt>{{ item.label }}</dt><dd>{{ item.value }}</dd></div>
  </dl>
</template>

<style scoped>
.operation-result-summary { display: flex; flex-direction: column; align-items: center; gap: 12px; text-align: center; margin-bottom: 24px; min-width: 0; overflow-wrap: anywhere; }
.operation-result-summary h3 { margin: 0; color: var(--wt-text-primary); font-size: 18px; font-weight: 600; line-height: 1.6; }
.operation-result-summary p, .operation-result-summary__note { margin: 0; color: var(--wt-text-tertiary); font-size: 13px; line-height: 1.6; }
.operation-result-summary__symbol { display: flex; align-items: center; justify-content: center; flex: 0 0 44px; width: 44px; height: 44px; border-radius: 50%; font-size: 26px; }
.is-success { --result-color: var(--wt-success); --result-bg: var(--wt-success-bg); }
.is-warning { --result-color: var(--wt-warning); --result-bg: var(--wt-warning-bg); }
.is-danger { --result-color: var(--wt-danger); --result-bg: var(--wt-danger-bg); }
.is-info, .is-running { --result-color: var(--wt-primary); --result-bg: var(--wt-info-bg); }
.operation-result-summary__symbol { color: var(--result-color); background: var(--result-bg); }
.operation-result-summary__spinner { width: 22px; height: 22px; border: 2px solid currentColor; border-right-color: transparent; border-radius: 50%; animation: operation-spin 1s linear infinite; }
.operation-result-counts { display: grid; grid-template-columns: repeat(7, minmax(0, 1fr)); margin: 0 0 20px; padding: 16px 0; background: var(--wt-bg-page); border: 1px solid var(--wt-border); border-radius: var(--wt-radius-sm); }
.operation-result-counts > div { display: flex; flex-direction: column; align-items: center; gap: 6px; padding: 0 8px; min-width: 0; }
.operation-result-counts dt { color: var(--wt-text-tertiary); font-size: 12px; }
.operation-result-counts dd { margin: 0; color: var(--result-color, var(--wt-text-primary)); font-size: 24px; font-weight: 600; font-variant-numeric: tabular-nums; }
@keyframes operation-spin { to { transform: rotate(360deg); } }
@media (max-width: 600px) { .operation-result-counts { grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 16px 0; } }
@media (prefers-reduced-motion: reduce) { .operation-result-summary__spinner { animation: none; } }
</style>
