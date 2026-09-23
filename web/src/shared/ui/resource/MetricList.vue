<script setup>
// 表格单元格里的多项计数，竖排一行一项。
//
// 之前是把几项用「·」串成一行，宽度不够时会在任意位置折断成两行，
// 数字和标签被拆散，读起来是一团。竖排后每项各占一行，标签弱化、数字加粗，
// 行高随项数增长但每项都完整可读。
defineProps({
  items: { type: Array, default: () => [] },
})
</script>

<template>
  <div class="metric-list">
    <div v-for="item in items" :key="item.label" class="metric-list__item">
      <span class="metric-list__label">{{ item.label }}</span>
      <strong class="metric-list__value">{{ item.value }}</strong>
    </div>
  </div>
</template>

<style scoped>
.metric-list { display: flex; flex-direction: column; gap: 2px; }
.metric-list__item { display: flex; align-items: baseline; gap: 6px; font-size: 12px; line-height: 16px; white-space: nowrap; }
.metric-list__label { color: var(--wt-text-tertiary); }
/* 等宽数字：同一列里各位对齐，扫读时不会跳。 */
.metric-list__value { color: var(--wt-text-primary); font-weight: 600; font-variant-numeric: tabular-nums; }
</style>
