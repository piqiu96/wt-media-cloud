<script setup>
import { computed } from 'vue'
import ResourceStatusBadge from '../resource/ResourceStatusBadge.vue'

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
  if (failedCount.value || unconfirmedCount.value) return '处理结果'
  if (counts.value.skipped && !counts.value.success) return '没有需要处理的项目'
  return '全部完成'
})

function statusLabel(status) {
  return props.statusLabels[status] || STATUS_TEXT[status] || status
}

function badgeTone(status) {
  if (status === 'success') return 'success'
  if (status === 'failed') return 'danger'
  if (status === 'unconfirmed' || status === 'processing') return 'warning'
  return 'neutral'
}
</script>

<template>
  <t-dialog
    :visible="visible"
    :header="isRunning ? title : resultTitle"
    :close-btn="false"
    :close-on-esc-keydown="false"
    :close-on-overlay-click="false"
    :footer="!isRunning"
    width="min(92vw, 640px)"
  >
    <template v-if="isRunning">
      <p class="operation-progress__line">
        {{ runningLabel }} {{ finished }} / {{ total }}
        <template v-if="current">：{{ current.label }}</template>
      </p>
      <t-progress theme="line" :percentage="percent" :label="false" />
      <div class="operation-progress__counts">
        <span>成功 {{ counts.success }}</span>
        <span>失败 {{ counts.failed }}</span>
        <span>跳过 {{ counts.skipped }}</span>
        <span>待确认 {{ counts.unconfirmed }}</span>
        <span>未处理 {{ counts.pending }}</span>
      </div>
    </template>
    <template v-else>
      <div class="operation-progress__counts operation-progress__counts--result">
        <span>总数 {{ total }}</span>
        <span>成功 {{ counts.success }}</span>
        <span>失败 {{ counts.failed }}</span>
        <span>跳过 {{ counts.skipped }}</span>
        <span>待确认 {{ counts.unconfirmed }}</span>
        <span>未处理 {{ counts.pending }}</span>
      </div>
      <div class="operation-progress__list">
        <div v-for="item in items" :key="item.key" class="operation-progress__item">
          <div class="operation-progress__item-head">
            <span class="operation-progress__item-label">{{ item.label }}</span>
            <ResourceStatusBadge :tone="badgeTone(item.status)" :label="statusLabel(item.status)" />
          </div>
          <div v-if="item.reason" class="operation-progress__reason">{{ item.reason }}</div>
        </div>
      </div>
    </template>

    <template v-if="!isRunning" #footer>
      <t-space>
        <t-button v-if="failedCount" theme="primary" variant="outline" @click="emit('retry')">{{ retryLabel }}（{{ failedCount }}）</t-button>
        <t-button v-if="unconfirmedCount" theme="warning" variant="outline" @click="emit('verify')">{{ verifyLabel }}（{{ unconfirmedCount }}）</t-button>
        <t-button theme="primary" @click="emit('done')">完成</t-button>
      </t-space>
    </template>
  </t-dialog>
</template>

<style scoped>
.operation-progress__line { margin-bottom: 12px; color: var(--td-text-color-primary); }
.operation-progress__counts { display: flex; flex-wrap: wrap; gap: 12px; margin-top: 12px; color: var(--td-text-color-secondary); font-size: 13px; }
.operation-progress__counts--result { margin-top: 0; margin-bottom: 12px; }
.operation-progress__list { max-height: 46vh; overflow-y: auto; border-top: 1px solid var(--td-component-stroke); }
.operation-progress__item { padding: 10px 0; border-bottom: 1px solid var(--td-component-stroke); }
.operation-progress__item-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.operation-progress__item-label { font-weight: 500; }
.operation-progress__reason { margin-top: 4px; color: var(--td-text-color-secondary); font-size: 12px; line-height: 1.5; }
</style>
