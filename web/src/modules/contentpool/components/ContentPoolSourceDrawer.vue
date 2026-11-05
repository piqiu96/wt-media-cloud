<script setup>
import { ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { createDiscoveryClient } from '../../../shared/api/discovery.js'
import { formatDateTime } from '../../../shared/utils/datetime.js'
import { scheduleLabel } from '../labels.js'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import ResourceOverlay from '../../../shared/ui/resource/ResourceOverlay.vue'
import ResourceDescriptions from '../../../shared/ui/resource/ResourceDescriptions.vue'
import ResourceSection from '../../../shared/ui/resource/ResourceSection.vue'
import ResourceNotice from '../../../shared/ui/resource/ResourceNotice.vue'
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'

// 内容池「来源」列的关联详情：只读、无路由（前端交互规范 §9.4），关闭后回到原列表，
// 筛选与分页都还在。列表只显示策略名，策略与任务的关键信息在这里合并呈现；
// 人工来源没有可打开的实体，列表直接显示固定文案，不进这个抽屉。
//
// 任务自己取：来源行只带 id 与名称，快照（关键词、执行周期、统计）只有 getTask 才有。
// 取不到就保留策略名称、就地显示错误与重试，不把未知字段画成 0。
//
// 触发方式沿用挖掘任务页的「重试 / 定时 / 手动」：那是挖掘任务的触发口径，
// 与合成任务的「策略自动 / 人工创建」不是同一套词。
const props = defineProps({
  visible: { type: Boolean, default: false },
  source: { type: Object, default: null },
})
const emit = defineEmits(['update:visible'])

const client = createDiscoveryClient()
const router = useRouter()
const task = ref(null)
const loading = ref(false)
const error = ref('')
// 连续打开两条来源时，先发的请求可能后回；只认最后一次。
let requestSeq = 0

async function loadTask() {
  const taskID = Number(props.source?.crawl_task_id || 0)
  if (!taskID) {
    task.value = null
    error.value = ''
    loading.value = false
    return
  }
  const seq = requestSeq + 1
  requestSeq = seq
  loading.value = true
  error.value = ''
  try {
    const data = await client.getTask(taskID)
    if (seq !== requestSeq) return
    task.value = data || null
  } catch (e) {
    if (seq !== requestSeq) return
    task.value = null
    error.value = e?.message || '读取任务详情失败'
  } finally {
    if (seq === requestSeq) loading.value = false
  }
}

watch(
  () => [props.visible, props.source?.crawl_task_id],
  ([visible]) => {
    if (visible) loadTask()
  },
  { immediate: true },
)

function strategyName() {
  const source = props.source || {}
  return source.strategy_name || (source.strategy_id ? `策略 ${source.strategy_id}` : '-')
}

function snapshot() {
  return task.value?.snapshot || {}
}

function strategyTypeLabel() {
  if (snapshot().strategy_type === 'author') return '作者'
  if (snapshot().strategy_type === 'keyword') return '关键词'
  return '-'
}

function strategyFocus() {
  const snap = snapshot()
  const keywords = Array.isArray(snap.keywords) ? snap.keywords.filter(Boolean) : []
  return keywords.join('、') || snap.author || '-'
}

function taskTriggerLabel() {
  if (!task.value) return '-'
  if (task.value.task_type === 'retry_failed_task') return '重试'
  return task.value.schedule_key ? '定时' : '手动'
}

function statusLabel(status) {
  return ({ pending: '待执行', running: '执行中', success: '成功', partial_success: '部分成功', failed: '失败' })[status] || status || '未知'
}

function statusTone(status) {
  return status === 'success' ? 'success' : status === 'failed' ? 'danger' : status === 'running' ? 'info' : 'warning'
}

function resultSummary() {
  const stats = task.value?.stats || {}
  return `发现 ${stats.found || 0} · 新增 ${stats.added || 0} · 待审核 ${stats.pending || 0}`
}

function goStrategy() {
  if (!props.source?.strategy_id) return
  emit('update:visible', false)
  router.push({ path: '/discovery-strategies', query: { strategy_id: props.source.strategy_id } })
}

function goTask() {
  if (!props.source?.crawl_task_id) return
  emit('update:visible', false)
  router.push({ path: '/crawl-tasks', query: { task_id: props.source.crawl_task_id } })
}
</script>

<template>
  <ResourceOverlay :visible="visible" title="来源详情" kind="drawer" :width="800" @update:visible="$emit('update:visible', $event)">
    <div v-if="source" class="source-drawer">
      <ResourceSection title="来源策略">
        <ResourceDescriptions>
          <t-descriptions-item label="策略名称" full>{{ strategyName() }}</t-descriptions-item>
          <t-descriptions-item label="策略 ID">{{ source.strategy_id || '—' }}</t-descriptions-item>
        </ResourceDescriptions>
      </ResourceSection>
      <FeedbackAlert v-if="error" theme="error" :message="error" />
      <t-button v-if="error" size="small" variant="outline" @click="loadTask">重试</t-button>
      <p v-else-if="loading">任务详情加载中…</p>
      <template v-else-if="task">
        <ResourceSection title="执行时配置">
          <ResourceNotice>以下为本次任务使用的配置。</ResourceNotice>
          <ResourceDescriptions>
            <t-descriptions-item label="类型">{{ strategyTypeLabel() }}</t-descriptions-item>
            <t-descriptions-item label="执行周期">{{ scheduleLabel(snapshot().schedule) }}</t-descriptions-item>
            <t-descriptions-item label="关键词 / 作者" full>{{ strategyFocus() }}</t-descriptions-item>
          </ResourceDescriptions>
        </ResourceSection>
        <ResourceSection title="来源任务">
          <ResourceDescriptions>
            <t-descriptions-item label="任务 ID">{{ task.id || source.crawl_task_id }}</t-descriptions-item>
            <t-descriptions-item label="状态"><ResourceStatusBadge :tone="statusTone(task.status)" :label="statusLabel(task.status)" /></t-descriptions-item>
            <t-descriptions-item label="触发方式">{{ taskTriggerLabel() }}</t-descriptions-item>
            <t-descriptions-item label="结果概况">{{ resultSummary() }}</t-descriptions-item>
            <t-descriptions-item label="创建时间">{{ formatDateTime(task.created_at) }}</t-descriptions-item>
            <t-descriptions-item label="完成时间">{{ formatDateTime(task.finished_at) }}</t-descriptions-item>
          </ResourceDescriptions>
        </ResourceSection>
      </template>
    </div>
    <template #footer>
      <t-button v-if="source?.crawl_task_id" variant="outline" @click="goTask">前往任务页面</t-button>
      <t-button v-if="source?.strategy_id" theme="primary" @click="goStrategy">前往策略页面</t-button>
    </template>
  </ResourceOverlay>
</template>
