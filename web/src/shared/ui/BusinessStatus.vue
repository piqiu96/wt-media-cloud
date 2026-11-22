<script setup>
import { computed } from 'vue';
import ResourceStatusBadge from './resource/ResourceStatusBadge.vue';

const props = defineProps({
  status: { type: String, required: true },
  pill: { type: Boolean, default: true },
  label: { type: String, default: '' },
});

// Semantic colors belong here; callers provide the contract state and label.
const STATUS_MAP = {
  pending: { tone: 'neutral', label: '待处理' },
  queued: { tone: 'neutral', label: '排队中' },
  running: { tone: 'info', label: '运行中' },
  starting: { tone: 'info', label: '启动中' },
  idle: { tone: 'success', label: '空闲' },
  success: { tone: 'success', label: '成功' },
  normal: { tone: 'success', label: '正常' },
  published: { tone: 'success', label: '已发布' },
  enabled: { tone: 'success', label: '启用' },
  warning: { tone: 'warning', label: '警告' },
  pending_review: { tone: 'warning', label: '待审核' },
  reviewing: { tone: 'warning', label: '平台审核中' },
  awaiting_human: { tone: 'warning', label: '等待人工' },
  uncertain: { tone: 'warning', label: '结果待确认' },
  partial_success: { tone: 'warning', label: '部分成功' },
  interrupted: { tone: 'warning', label: '已中断' },
  paused: { tone: 'warning', label: '已暂停' },
  stopping: { tone: 'warning', label: '停止中' },
  failed: { tone: 'danger', label: '失败' },
  error: { tone: 'danger', label: '错误' },
  unreachable: { tone: 'danger', label: '不可达' },
  discarded: { tone: 'neutral', label: '已丢弃' },
  cancelled: { tone: 'neutral', label: '已取消' },
  abandoned: { tone: 'neutral', label: '已放弃' },
  expired: { tone: 'neutral', label: '已过期' },
  stopped: { tone: 'neutral', label: '已停止' },
  disabled: { tone: 'neutral', label: '停用' },
  archived: { tone: 'neutral', label: '归档' },
  unknown: { tone: 'neutral', label: '未知' },
  automated: { tone: 'automation', label: '自动' },
  ai_generated: { tone: 'automation', label: 'AI 生成' },
};
const COLORS = {
  neutral: 'var(--wt-text-tertiary)', info: 'var(--wt-primary)',
  success: 'var(--wt-color-success-text)', warning: 'var(--wt-color-warning-text)',
  danger: 'var(--wt-color-danger-text)', automation: '#722ed1',
};
const resolved = computed(() => {
  const key = props.status?.toLowerCase() ?? '';
  return STATUS_MAP[key] ?? { tone: 'neutral', label: key };
});
</script>

<template>
  <ResourceStatusBadge v-if="pill" :tone="resolved.tone" :label="label || resolved.label" />
  <span v-else class="business-status-dot">
    <span class="dot" :style="{ backgroundColor: COLORS[resolved.tone] }"></span>
    {{ label || resolved.label }}
  </span>
</template>

<style scoped>
/* Preserve the existing non-business automation marker. */
.is-automation { color: #722ed1; background: #f9f0ff; }
.business-status-dot { display: inline-flex; align-items: center; gap: 6px; font-size: 14px; color: var(--wt-text-primary); }
.dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
</style>
