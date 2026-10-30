<script setup>
import { computed } from 'vue'
import { CheckCircleIcon, ErrorCircleIcon } from 'tdesign-icons-vue-next'
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
      <div class="operation-progress__summary" role="status" aria-live="polite">
        <div class="operation-progress__symbol" :class="isRunning ? 'is-running' : failedCount === total && total ? 'is-danger' : failedCount || unconfirmedCount || counts.pending ? 'is-warning' : 'is-success'">
          <span v-if="isRunning" class="operation-progress__spinner" />
          <ErrorCircleIcon v-else-if="failedCount || unconfirmedCount || counts.pending" />
          <CheckCircleIcon v-else />
        </div>
        <div><h3>{{ isRunning ? runningLabel : resultTitle }}</h3>
          <p v-if="isRunning">请保持当前窗口打开，处理结束后自动显示结果。</p>
          <p v-else-if="counts.pending">已完成项目保留，未处理项可重新选择，待确认项请先核实。</p>
          <p v-else-if="unconfirmedCount">请先核实实际结果，避免重复操作。</p>
          <p v-else>本次结果仅在当前窗口查看。</p>
        </div>
      </div>
      <dl class="operation-progress__counts">
        <div><dt>总数</dt><dd>{{ total }}</dd></div>
        <div v-for="status in ['success', 'failed', 'skipped', 'processing', 'pending', 'unconfirmed']" :key="status" :class="`is-${status}`"><dt>{{ statusLabel(status) }}</dt><dd>{{ counts[status] }}</dd></div>
      </dl>
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
.operation-progress__summary { display: flex; flex-direction: column; align-items: center; gap: 12px; text-align: center; margin-bottom: 24px; }
.operation-progress__summary h3 { margin: 0; color: var(--wt-text-primary); font-size: 18px; font-weight: 600; }
.operation-progress__summary p { margin: 6px 0 0; color: var(--wt-text-tertiary); font-size: 13px; line-height: 1.6; }
.operation-progress__symbol { display: flex; align-items: center; justify-content: center; flex: 0 0 44px; width: 44px; height: 44px; border-radius: 50%; font-size: 26px; }
.operation-progress__symbol.is-success { color: var(--wt-success); background: var(--wt-success-bg); }
.operation-progress__symbol.is-warning { color: var(--wt-warning); background: var(--wt-warning-bg); }
.operation-progress__symbol.is-danger { color: var(--wt-danger); background: var(--wt-danger-bg); }
.operation-progress__symbol.is-running { color: var(--wt-primary); background: var(--wt-info-bg); }
.operation-progress__spinner { width: 22px; height: 22px; border: 2px solid currentColor; border-right-color: transparent; border-radius: 50%; animation: operation-spin 1s linear infinite; }
.operation-progress__counts { display: grid; grid-template-columns: repeat(7, minmax(0, 1fr)); margin: 0 0 20px; padding: 16px 0; background: var(--wt-bg-page); border: 1px solid var(--wt-border); border-radius: var(--wt-radius-sm); }
.operation-progress__counts > div { display: flex; flex-direction: column; align-items: center; gap: 6px; padding: 0 8px; }
.operation-progress__counts > div:last-child { border: 0; }
.operation-progress__counts dt { color: var(--wt-text-tertiary); font-size: 12px; }
.operation-progress__counts dd { margin: 0; color: var(--wt-text-primary); font-size: 24px; font-weight: 600; font-variant-numeric: tabular-nums; }
.operation-progress__running { margin-bottom: 16px; }
.operation-progress__running p, .operation-progress__notice { margin: 8px 0 0; color: var(--wt-text-tertiary); font-size: 12px; line-height: 1.6; }
.operation-progress__item-label { color: var(--wt-text-primary); font-weight: 500; overflow-wrap: anywhere; }
.operation-progress__reason { color: var(--wt-text-tertiary); line-height: 1.6; overflow-wrap: anywhere; }
.operation-progress__notice { margin-top: 14px; }
@keyframes operation-spin { to { transform: rotate(360deg); } }
@media (max-width: 600px) { .operation-progress__counts { grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 16px 0; } }
@media (prefers-reduced-motion: reduce) { .operation-progress__spinner { animation: none; } }
</style>
