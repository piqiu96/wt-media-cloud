<script setup>
import { metricLabel } from '../materialFilters.js'
import { formatDateTime } from '../../../shared/utils/datetime.js'

defineProps({ material: { type: Object, required: true }, detail: Boolean })
const metrics = [['like_count', '点赞'], ['favorite_count', '收藏'], ['comment_count', '评论']]
</script>

<template>
  <div class="material-performance" :class="{ 'material-performance--detail': detail }">
    <div class="material-performance__metrics">
      <div v-for="[key, label] in metrics" :key="key">
        <span>{{ label }}</span>
        <strong>{{ metricLabel(material, key) }}</strong>
      </div>
    </div>
    <small>数据更新时间：{{ material.metrics_updated_at ? formatDateTime(material.metrics_updated_at) : '—' }}</small>
  </div>
</template>

<style scoped>
.material-performance { display: grid; gap: 6px; }
.material-performance__metrics { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; }
.material-performance__metrics > div { display: grid; gap: 2px; }
.material-performance__metrics span, .material-performance small { color: var(--wt-text-tertiary); font-size: 12px; line-height: 18px; }
.material-performance__metrics strong { color: var(--wt-text-primary); font-size: 13px; font-weight: 500; overflow-wrap: anywhere; }
.material-performance--detail { max-width: 480px; gap: 12px; }
.material-performance--detail strong { font-size: 20px; }
</style>
