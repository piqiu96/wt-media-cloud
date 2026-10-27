<script setup>
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
      <span>{{ item.label }}</span>
      <strong>{{ item.value }}</strong>
    </component>
  </div>
</template>

<style scoped>
.resource-stat-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 12px; margin-bottom: 16px; }
.resource-stat-card { min-height: 88px; padding: 16px; border: 1px solid var(--wt-border); border-radius: var(--wt-radius-md); background: var(--wt-bg-card); box-shadow: var(--wt-shadow-card); text-align: left; cursor: pointer; transition: border-color 0.2s ease, transform 0.2s ease; }
.resource-stat-card:not(.is-static):hover { border-color: rgba(37, 99, 235, 0.24); transform: translateY(-1px); }
.resource-stat-card.is-static { cursor: default; }
.resource-stat-card.is-active { border-color: var(--td-brand-color); box-shadow: 0 0 0 1px var(--td-brand-color) inset; }
.resource-stat-card span { display: block; color: var(--wt-text-tertiary); font-size: 13px; }
.resource-stat-card strong { display: block; margin-top: 7px; color: var(--wt-text-primary); font-size: 24px; font-weight: 600; line-height: 1; }
.resource-stat-card.is-success strong { color: var(--wt-success); }
.resource-stat-card.is-warning strong { color: var(--wt-warning); }
.resource-stat-card.is-danger strong { color: var(--wt-danger); }
</style>
