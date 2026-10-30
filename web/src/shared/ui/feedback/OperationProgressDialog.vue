<script setup>
import { computed } from 'vue'
import OperationResultSummary from './OperationResultSummary.vue'
import ResourceStatusBadge from '../resource/ResourceStatusBadge.vue'
import ResourceOverlay from '../resource/ResourceOverlay.vue'

// 现场处理弹窗：同步、批量开关窗、批量设置代理、导入和检测共用。
// 处理中不给关闭入口；结束后只在原地提供失败重试与待确认核实，不建任务中心。
const props = defineProps({
  visible: { type: Boolean, default: false },
  title: { type: String, default: '处理进度' },
  phase: { type: String, default: 'running' },
  items: { type: Array, default: () => [] },
  runningLabel: { type: String, default: '正在处理' },
  statusLabels: { type: Object, default: () => ({}) },
  retryLabel: { type: String, default: '重试失败项' },
  verifyLabel: { type: String, default: '核实待确认项' },
})

const emit = defineEmits(['retry', 'verify', 'done'])

const STATUS_TEXT = {
  pending: '未处理',
  processing: '处理中',
  success: '成功',
  failed: '失败',
  skipped: '跳过',
  unconfirmed: '待确认',
}

const counts = computed(() => {
  const result = { pending: 0, processing: 0, success: 0, failed: 0, skipped: 0, unconfirmed: 0 }
  for (const item of props.items) {
    if (result[item.status] !== undefined) result[item.status] += 1
  }
  return result
})
const total = computed(() => props.items.length)
const finished = computed(() => counts.value.success + counts.value.failed + counts.value.skipped + counts.value.unconfirmed)
const percent = computed(() => (total.value ? Math.round((finished.value / total.value) * 100) : 0))
const current = computed(() => props.items.find((item) => item.status === 'processing') || null)
const failedCount = computed(() => counts.value.failed)
const unconfirmedCount = computed(() => counts.value.unconfirmed)
const isRunning = computed(() => props.phase === 'running')

const resultTitle = computed(() => {
  if (counts.value.pending) return '处理已中断'
  if (unconfirmedCount.value) return '部分结果待确认'
  if (failedCount.value === total.value && total.value) return '本次处理失败'
  if (failedCount.value) return '部分项目未完成'
  if (counts.value.skipped && !counts.value.success) return '没有需要处理的项目'
  return '全部完成'
})

function statusLabel(status) {
  return props.statusLabels[status] || STATUS_TEXT[status] || status
}

function badgeTone(status) {
  if (status === 'success') return 'success'
  if (status === 'failed') return 'danger'
  if (status === 'processing') return 'info'
  if (status === 'unconfirmed') return 'warning'
  return 'neutral'
}
</script>

<template>
  <ResourceOverlay :visible="visible" :title="title" :busy="isRunning" :esc-closable="false" :footer="!isRunning" :width="960" @close="emit('done')">
    <div class="operation-progress" :aria-busy="isRunning">
      <OperationResultSummary :title="isRunning ? runningLabel : resultTitle" :running="isRunning"
        :tone="failedCount === total && total ? 'danger' : failedCount || unconfirmedCount || counts.pending ? 'warning' : 'success'"
        :counts="[{ key: 'total', label: '总数', value: total }, ...['success', 'failed', 'skipped', 'processing', 'pending', 'unconfirmed'].map(status => ({ key: status, label: statusLabel(status), value: counts[status], tone: badgeTone(status) }))]">
        <span v-if="isRunning">请保持当前窗口打开，处理结束后自动显示结果。</span>
        <span v-else-if="counts.pending">已完成项目保留，未处理项可重新选择，待确认项请先核实。</span>
        <span v-else-if="unconfirmedCount">请先核实实际结果，避免重复操作。</span>
        <span v-else>本次结果仅在当前窗口查看。</span>
      </OperationResultSummary>
      <div v-if="isRunning" class="operation-progress__running">
        <t-progress theme="line" :percentage="percent" :label="false" />
        <p>已处理 {{ finished }} / {{ total }} 项<span v-if="current"> · {{ current.label }}</span></p>
      </div>
      <t-table
        class="operation-progress__table"
        :data="items"
        row-key="key"
        size="small"
        :max-height="360"
        :columns="[{ colKey: 'index', title: '序号', width: 60 }, { colKey: 'label', title: '操作对象', width: 240 }, { colKey: 'status', title: '结果', width: 100 }, { colKey: 'reason', title: '原因 / 后续处理', width: 300 }]"
        :scroll="{ x: '700px' }"
        empty="暂无处理项目"
      >
        <template #index="{ rowIndex }">{{ rowIndex + 1 }}</template>
        <template #label="{ row }"><span class="operation-progress__item-label">{{ row.label }}</span></template>
        <template #status="{ row }"><ResourceStatusBadge :tone="badgeTone(row.status)" :label="statusLabel(row.status)" /></template>
        <template #reason="{ row }"><span class="operation-progress__reason">{{ row.reason || (row.status === 'processing' ? '正在处理' : row.status === 'pending' ? '尚未开始' : '—') }}</span></template>
      </t-table>
      <p v-if="isRunning" class="operation-progress__notice">处理中不可关闭；强制离开不会撤销已生效的项目。</p>
    </div>
    <template v-if="!isRunning" #footer>
      <t-space>
        <t-button v-if="failedCount" theme="default" variant="outline" @click="emit('retry')">{{ retryLabel }}（{{ failedCount }}）</t-button>
        <t-button v-if="unconfirmedCount" theme="default" variant="outline" @click="emit('verify')">{{ verifyLabel }}（{{ unconfirmedCount }}）</t-button>
        <t-button theme="primary" @click="emit('done')">完成</t-button>
      </t-space>
    </template>
  </ResourceOverlay>
</template>

<style scoped>
.operation-progress { color: var(--wt-text-secondary); }
.operation-progress__running { margin-bottom: 16px; }
.operation-progress__running p, .operation-progress__notice { margin: 8px 0 0; color: var(--wt-text-tertiary); font-size: 12px; line-height: 1.6; }
.operation-progress__item-label { color: var(--wt-text-primary); font-weight: 500; overflow-wrap: anywhere; }
.operation-progress__reason { color: var(--wt-text-tertiary); line-height: 1.6; overflow-wrap: anywhere; }
.operation-progress__notice { margin-top: 14px; }
</style>
