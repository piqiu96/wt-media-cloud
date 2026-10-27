<script setup>
import { ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { createDiscoveryClient } from '../../../shared/api/discovery.js'
import { formatDateTime } from '../../../shared/utils/datetime.js'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
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
  <t-drawer
    :visible="visible"
    header="来源详情"
    size="min(92vw, 560px)"
    :close-btn="true"
    @update:visible="$emit('update:visible', $event)"
  >
    <div v-if="source" class="source-drawer">
      <section class="source-drawer__card">
        <h4 class="source-drawer__title">策略</h4>
        <dl class="source-drawer__fields">
          <div><dt>策略名称</dt><dd>{{ strategyName() }}</dd></div>
          <div><dt>类型</dt><dd>{{ strategyTypeLabel() }}</dd></div>
          <div><dt>关键词 / 作者</dt><dd>{{ strategyFocus() }}</dd></div>
          <div><dt>执行周期</dt><dd>{{ snapshot().schedule || '-' }}</dd></div>
        </dl>
      </section>
      <section class="source-drawer__card">
        <h4 class="source-drawer__title">任务</h4>
        <FeedbackAlert v-if="error" theme="error" :message="error" />
        <div v-if="error" class="source-drawer__retry">
          <t-button size="small" class="wt-secondary-button" variant="outline" @click="loadTask">重试</t-button>
        </div>
        <p v-else-if="loading" class="source-drawer__placeholder">任务详情加载中…</p>
        <dl v-else-if="task" class="source-drawer__fields">
          <div><dt>任务 ID</dt><dd>{{ task.id || source.crawl_task_id }}</dd></div>
          <div><dt>触发方式</dt><dd>{{ taskTriggerLabel() }}</dd></div>
          <div><dt>状态</dt><dd><ResourceStatusBadge :tone="statusTone(task.status)" :label="statusLabel(task.status)" /></dd></div>
          <div><dt>创建时间</dt><dd>{{ formatDateTime(task.created_at) }}</dd></div>
          <div><dt>完成时间</dt><dd>{{ formatDateTime(task.finished_at) }}</dd></div>
          <div><dt>结果概况</dt><dd>{{ resultSummary() }}</dd></div>
        </dl>
      </section>
    </div>
    <template #footer>
      <div v-if="source" class="source-drawer__footer">
        <t-button v-if="source.strategy_id" theme="primary" @click="goStrategy">前往策略页面</t-button>
        <t-button v-if="source.crawl_task_id" class="wt-secondary-button" variant="outline" @click="goTask">前往任务页面</t-button>
      </div>
    </template>
  </t-drawer>
</template>

<style scoped>
.source-drawer { display: flex; flex-direction: column; gap: 14px; }
.source-drawer__card { padding: 14px 16px; border: 1px solid var(--wt-border); border-radius: 8px; }
.source-drawer__title { margin: 0 0 10px; color: var(--wt-text-primary); font-size: 14px; font-weight: 600; }
.source-drawer__fields { margin: 0; }
.source-drawer__fields > div { display: flex; gap: 12px; padding: 6px 0; }
.source-drawer__fields dt { flex: none; width: 88px; color: var(--wt-text-tertiary); font-size: 12px; }
.source-drawer__fields dd { min-width: 0; margin: 0; color: var(--wt-text-primary); font-size: 13px; word-break: break-word; }
.source-drawer__placeholder { margin: 0; color: var(--wt-text-tertiary); font-size: 13px; }
.source-drawer__retry { margin-top: 8px; }
.source-drawer__footer { display: flex; justify-content: flex-end; gap: 8px; }
</style>
