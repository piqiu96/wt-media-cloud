<script setup>
import { Progress } from 'tdesign-vue-next'
import ResourceFieldHelp from './ResourceFieldHelp.vue'
defineProps({
  label: { type: String, required: true },
  percentage: { type: Number, default: 0 },
  valueText: { type: String, default: '' },
  description: { type: String, default: '' },
  help: { type: String, default: '' },
})
</script>

<template>
  <div class="resource-progress" role="group" :aria-label="label">
    <div class="resource-progress__heading"><span class="resource-progress__label">{{ label }}<ResourceFieldHelp v-if="help" :label="label" :content="help" /></span><span class="resource-progress__value">{{ valueText }}<b>{{ percentage }}%</b></span></div>
    <Progress :percentage="Math.max(0, Math.min(100, percentage))" :label="false" :stroke-width="6" color="var(--wt-primary)" track-color="var(--wt-color-bg-subtle)" />
    <p v-if="description">{{ description }}</p>
  </div>
</template>

<style scoped>
.resource-progress { min-width: 0; }
.resource-progress__heading { display: flex; justify-content: space-between; align-items: baseline; flex-wrap: wrap; gap: 4px 16px; margin-bottom: 10px; font-size: 13px; }
.resource-progress__label { display: inline-flex; align-items: center; gap: 4px; }
.resource-progress__value { display: inline-flex; align-items: baseline; gap: 12px; color: var(--wt-text-tertiary); font-variant-numeric: tabular-nums; }
.resource-progress__value b { color: var(--wt-text-primary); font-size: 14px; font-weight: 500; }
.resource-progress p { margin: 8px 0 0; color: var(--wt-text-tertiary); font-size: 12px; line-height: 1.6; }
.resource-progress :deep(.t-progress__bar), .resource-progress :deep(.t-progress__inner) { border-radius: 3px; }
</style>
