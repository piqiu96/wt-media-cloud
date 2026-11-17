<script setup>
import ResourceFieldHelp from './ResourceFieldHelp.vue'
defineProps({
  items: { type: Array, required: true },
  interactive: { type: Boolean, default: false },
  // 已应用的快捷筛选：点中的指标保持选中，直到清除筛选
  //（前端交互规范 §可交互 SummaryMetrics）。
  activeKey: { type: String, default: '' },
})

const emit = defineEmits(['select'])
</script>

<template>
  <div class="resource-stat-grid">
    <component :is="interactive ? 'button' : 'div'" v-for="item in items" :key="item.key" :type="interactive ? 'button' : undefined" class="resource-stat-card" :class="[`is-${item.tone || 'info'}`, { 'is-static': !interactive, 'is-active': interactive && activeKey === item.key }]" @click="interactive && emit('select', item.key)">
      <span class="resource-stat-card__label">{{ item.label }}<ResourceFieldHelp v-if="item.help" :label="item.label" :content="item.help" /></span>
      <strong>{{ item.value }}<small v-if="item.unit">{{ item.unit }}</small></strong>
      <p v-if="item.description">{{ item.description }}</p>
    </component>
  </div>
</template>

<style scoped>
.resource-stat-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 12px; margin-bottom: var(--wt-space-block, 16px); }
.resource-stat-card { box-sizing: border-box; min-height: 80px; padding: 12px 20px; border: 1px solid var(--wt-border); border-radius: var(--wt-radius-md); background: var(--wt-bg-card); box-shadow: var(--wt-shadow-card); text-align: left; cursor: pointer; transition: border-color 0.2s ease, transform 0.2s ease; }
.resource-stat-card:not(.is-static):hover { border-color: var(--wt-color-primary-border); transform: none; }
.resource-stat-card.is-static { cursor: default; }
.resource-stat-card.is-active { border-color: var(--wt-primary); background: var(--wt-info-bg); box-shadow: none; }
.resource-stat-card span { display: block; color: var(--wt-text-tertiary); font-size: 13px; }
.resource-stat-card__label { display: flex !important; align-items: center; gap: 6px; }
.resource-stat-card strong { display: block; margin-top: 6px; color: var(--wt-text-primary); font-size: 26px; font-weight: 600; line-height: 28px; font-variant-numeric: tabular-nums; }
.resource-stat-card.is-success strong { color: var(--wt-success); }
.resource-stat-card.is-warning strong { color: var(--wt-warning); }
.resource-stat-card.is-danger strong { color: var(--wt-danger); }
.resource-stat-card strong small { margin-left: 6px; color: var(--wt-text-tertiary); font-size: 12px; font-weight: 400; }
.resource-stat-card p { margin: 8px 0 0; color: var(--wt-text-tertiary); font-size: 12px; line-height: 1.6; }
</style>
