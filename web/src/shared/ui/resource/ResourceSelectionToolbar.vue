<script setup>
defineProps({ count: { type: Number, required: true }, unit: { type: String, default: '项' }, hint: { type: String, default: '' } })
const emit = defineEmits(['clear'])
</script>

<template>
  <div v-if="count" class="resource-selection-toolbar" role="region" aria-label="批量操作">
    <div class="resource-selection-toolbar__main">
      <span class="resource-selection-toolbar__count">已选择 <strong>{{ count }}</strong> {{ unit }}</span>
      <div class="resource-selection-toolbar__actions"><slot /></div>
      <t-button theme="default" variant="text" class="resource-selection-toolbar__clear" @click="emit('clear')">取消选择</t-button>
    </div>
    <p v-if="hint" class="resource-selection-toolbar__hint">{{ hint }}</p>
  </div>
</template>

<style scoped>
.resource-selection-toolbar { margin-bottom: 16px; padding: 12px 16px; border: 1px solid var(--wt-color-primary-border); border-radius: var(--wt-radius-sm); background: var(--wt-info-bg); }
.resource-selection-toolbar__main { display: flex; align-items: center; flex-wrap: wrap; gap: 8px 16px; }
.resource-selection-toolbar__count { color: var(--wt-text-secondary); font-size: 13px; white-space: nowrap; }
.resource-selection-toolbar__count strong { color: var(--wt-primary); font-variant-numeric: tabular-nums; }
.resource-selection-toolbar__actions { display: flex; flex-wrap: wrap; gap: 8px; }
.resource-selection-toolbar__clear { margin-left: auto; }
.resource-selection-toolbar__hint { margin: 8px 0 0; color: var(--wt-warning); font-size: 12px; line-height: 1.6; }
</style>
