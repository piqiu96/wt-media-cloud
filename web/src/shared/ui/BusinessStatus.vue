<!--
  BusinessStatus — 统一状态颜色组件

  严格按照决策 0007 状态的映射：
  - running / pending / queued    → Blue   (--td-brand-color)
  - success / normal              → Green  (--td-success-color)
  - warning / pending_review       → Orange (--td-warning-color)
  - failed / error                → Red    (--td-error-color)
  - discarded / cancelled / expired → Gray  (--td-text-color-disabled)
  - automated / ai_generated      → Purple (#722ed1)

  用法:
    <BusinessStatus status="running" />
    <BusinessStatus status="failed" :pill="false" />
-->
<script setup>
import { computed } from "vue"

const props = defineProps({
  /** 状态键值，映射到颜色和中文标签 */
  status: { type: String, required: true },
  /** true=胶囊标签(Tag)，false=大字点+文字 */
  pill: { type: Boolean, default: true },
  /** 自定义文字（不传则自动从 STATUS_MAP 取） */
  label: { type: String, default: "" },
})

const STATUS_MAP = {
  running:         { color: "var(--td-brand-color)",    label: "运行中" },
  starting:        { color: "var(--td-brand-color)",    label: "启动中" },
  pending:         { color: "var(--td-brand-color)",    label: "待处理" },
  queued:          { color: "var(--td-brand-color)",    label: "排队中" },
  idle:            { color: "var(--td-success-color)",  label: "空闲" },
  success:         { color: "var(--td-success-color)",  label: "成功" },
  normal:          { color: "var(--td-success-color)",  label: "正常" },
  warning:         { color: "var(--td-warning-color)",  label: "警告" },
  pending_review:  { color: "var(--td-warning-color)",  label: "待审核" },
  paused:          { color: "var(--td-warning-color)",  label: "已暂停" },
  stopping:        { color: "var(--td-warning-color)",  label: "停止中" },
  failed:          { color: "var(--td-error-color)",    label: "失败" },
  error:           { color: "var(--td-error-color)",    label: "错误" },
  unreachable:     { color: "var(--td-error-color)",    label: "不可达" },
  discarded:       { color: "var(--td-text-color-disabled)", label: "已丢弃" },
  cancelled:       { color: "var(--td-text-color-disabled)", label: "已取消" },
  expired:         { color: "var(--td-text-color-disabled)", label: "已过期" },
  stopped:         { color: "var(--td-text-color-disabled)", label: "已停止" },
  unknown:         { color: "var(--td-text-color-disabled)", label: "未知" },
  automated:       { color: "#722ed1",                 label: "自动" },
  ai_generated:    { color: "#722ed1",                 label: "AI 生成" },
}

const resolved = computed(() => {
  const key = props.status?.toLowerCase() ?? ""
  return STATUS_MAP[key] ?? { color: "var(--td-text-color-disabled)", label: key }
})
</script>

<template>
  <span v-if="pill" class="business-status-pill" :style="{ backgroundColor: resolved.color }">
    {{ label || resolved.label }}
  </span>
  <span v-else class="business-status-dot">
    <span class="dot" :style="{ backgroundColor: resolved.color }"></span>
    {{ label || resolved.label }}
  </span>
</template>

<style scoped>
.business-status-pill {
  display: inline-block;
  padding: 0 8px;
  border-radius: 4px;
  font-size: 12px;
  line-height: 22px;
  color: #fff;
  white-space: nowrap;
}

.business-status-dot {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  color: var(--td-text-color-primary);
}

.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}
</style>
