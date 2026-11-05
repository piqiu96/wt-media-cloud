<script setup>
import { computed } from 'vue'
import { batchProgress, TASK_STATUS_LABELS } from '../batchActions.js'
const props = defineProps({ counts: { type: Object, default: () => ({}) }, compact: Boolean })
const progress = computed(() => batchProgress(props.counts))
</script>
<template>
  <div class="batch-progress" :class="{ 'batch-progress--compact': compact }">
    <div class="batch-progress__heading"><span>已结束 <strong>{{ progress.finished }} / {{ progress.total }}</strong></span><span>{{ progress.percent }}%</span></div>
    <t-progress :percentage="progress.percent" :label="false" :status="progress.finished === progress.total && counts.failed ? 'warning' : 'default'" />
    <div v-if="!compact" class="batch-progress__counts"><div v-for="(label,key) in TASK_STATUS_LABELS" :key="key"><span>{{ label }}</span><strong>{{ counts[key] || 0 }}</strong></div></div>
    <span v-else class="batch-progress__summary">成功 {{ counts.success || 0 }} · 失败 {{ counts.failed || 0 }}</span>
  </div>
</template>
<style scoped>
.batch-progress { min-width:0; }
.batch-progress__heading { display:flex;justify-content:space-between;gap:16px;align-items:baseline;margin-bottom:12px;color:var(--wt-text-tertiary);font-size:13px; }
.batch-progress__heading strong {color:var(--wt-text-primary);font-size:20px;font-weight:600;}
.batch-progress__counts {display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:16px;margin-top:20px;}
.batch-progress__counts > div {display:grid;gap:6px;}
.batch-progress__counts span {font-size:12px;color:var(--wt-text-tertiary);}
.batch-progress__counts strong {font-size:20px;font-weight:600;color:var(--wt-text-primary);}
.batch-progress--compact .batch-progress__heading {margin-bottom:5px;font-size:12px;}
.batch-progress--compact .batch-progress__heading strong {font-size:13px;font-weight:500;}
.batch-progress__summary {display:block;margin-top:5px;font-size:12px;color:var(--wt-text-tertiary);}
@media(max-width:520px){.batch-progress__counts{gap:8px;}.batch-progress__counts strong{font-size:18px;}}
</style>
