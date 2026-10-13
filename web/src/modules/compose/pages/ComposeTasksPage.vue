<script setup>
import { feedback } from '../../../shared/ui/feedback/index.js'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { DialogPlugin } from 'tdesign-vue-next'
import { createComposeClient } from '../../../shared/api/compose.js'
import { createSessionClient } from '../../../shared/api/session.js'
import ResourceCard from '../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../shared/ui/resource/ResourcePageHeader.vue'
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'
import { opColumnWidth } from '../../../shared/ui/resource/opColumnWidth.js'
import { formatDateTime } from '../../../shared/utils/datetime.js'
import { createdRangeParams } from '../createdRange.js'

const client = createComposeClient()
const sessionClient = createSessionClient()
const router = useRouter()

const rows = ref([])
const strategies = ref([])
const loading = ref(false)
const error = ref('')
const filters = ref({ status: '', compose_strategy_id: '', created_range: [] })
const pagination = ref({ current: 1, pageSize: 20 })

const statusLabels = { pending: '排队中', running: '执行中', success: '已完成', failed: '失败', cancelled: '已取消' }
const statusTones = { pending: 'warning', running: 'info', success: 'success', failed: 'danger', cancelled: 'neutral' }
// 阶段是执行投影的词表；M4-C3 的 pending 还没有执行阶段，显示名钉在合同里：
// 等待源素材（源视频未就绪时任务保持 pending 并等待，PRD §5.4.4）。
const stageLabels = { pending: '等待源素材' }

// 每一列显式 width（规范 §6.2，WebKit fixed 布局下 minWidth 不产生确定列宽）。
// 操作列宽按行里最多会出现的按钮清单推导：排队中的行才有 取消。
const columns = computed(() => [
  { colKey: 'id', title: '任务 ID', width: 80 },
  { colKey: 'strategy', title: '合成策略', width: 180 },
  { colKey: 'template', title: '模板', width: 130 },
  { colKey: 'material_id', title: '素材 ID', width: 90 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'stage', title: '阶段', width: 120 },
  { colKey: 'progress', title: '进度', width: 80 },
  { colKey: 'created_at', title: '创建时间', width: 150 },
  { colKey: 'op', title: '操作', width: opColumnWidth(['详情', '取消']), fixed: 'right' },
])

// 横向滚动宽度由列宽推导，不写死。
const tableScroll = computed(() => ({
  x: `${columns.value.reduce((sum, col) => sum + (col.width || 0), 0)}px`,
}))

const pagedRows = computed(() => {
  const start = (pagination.value.current - 1) * pagination.value.pageSize
  return rows.value.slice(start, start + pagination.value.pageSize)
})

const statusOptions = Object.entries(statusLabels).map(([value, label]) => ({ value, label }))
const strategyOptions = computed(() => strategies.value.map((item) => ({ value: String(item.id), label: item.name })))

function strategyName(row) {
  const found = strategies.value.find((item) => item.id === row.compose_strategy_id)
  return found ? found.name : `#${row.compose_strategy_id}`
}

function templateText(row) {
  return `${row.template_code} v${row.template_version}`
}

function stageText(row) {
  if (row.stage) return stageLabels[row.stage] || row.stage
  return row.status === 'pending' ? stageLabels.pending : '—'
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const params = {}
    if (filters.value.status) params.status = filters.value.status
    if (filters.value.compose_strategy_id) params.compose_strategy_id = filters.value.compose_strategy_id
    Object.assign(params, createdRangeParams(filters.value.created_range))
    const data = await client.listComposeTasks(params)
    rows.value = Array.isArray(data) ? data : []
    pagination.value.current = 1
  } catch (e) {
    error.value = e?.message || '读取合成任务失败'
  } finally {
    loading.value = false
  }
}

function reset() {
  filters.value = { status: '', compose_strategy_id: '', created_range: [] }
  load()
}

async function loadStrategies() {
  try {
    const data = await client.listStrategies()
    strategies.value = Array.isArray(data) ? data : []
  } catch {
    strategies.value = []
  }
}

const detailVisible = ref(false)
const detail = ref(null)
const attempts = ref([])
const attemptsError = ref('')
// 与 ComposeStrategyPage 同一规则：页面据此藏重试入口；真正的授权判定在服务层，
// 藏按钮只是少一次必失败请求。
const currentUser = ref(null)
const canRetry = computed(() => ['senior_operator', 'admin'].includes(currentUser.value?.role))

// 重试分类（change.md 决定 2，服务端 error_code IN 同一张表）：storage_error/task_timeout
// 环境类可直接重试；execution_failed 属人工判断，按钮照给，是否重试由运营看错误信息定；
// input_missing/input_invalid/template_unavailable 不可重试，页面不给入口只引导新建任务。
const retryableCodes = new Set(['storage_error', 'task_timeout', 'execution_failed'])
const newTaskCodes = new Set(['input_missing', 'input_invalid', 'template_unavailable'])

// 尝试行的词表（M4-G）：source 区分自动重试与人工重试，outcome 是该次尝试自己的结局，
// 与任务行的 status 不是一回事。
const sourceLabels = { auto: '自动', manual: '人工' }
const outcomeLabels = { pending: '等待执行', running: '执行中', success: '已成功', failed: '失败', cancelled: '已取消' }
const outcomeTones = { pending: 'warning', running: 'info', success: 'success', failed: 'danger', cancelled: 'neutral' }

async function openDetail(row) {
  detailVisible.value = true
  detail.value = null
  attempts.value = []
  attemptsError.value = ''
  try {
    // 按 id 重取：详情里的快照必须是库里存的那份，不是列表加载那一刻的副本。
    // 执行记录一起取：详情要按「任务状态 → 快照 → 执行记录 → 成片」组织，逐次记录
    // 不能拿任务摘要冒充。
    detail.value = await client.getComposeTask(row.id)
    attempts.value = []
    attemptsError.value = ''
    attempts.value = await client.listComposeTaskAttempts(row.id)
  } catch (e) {
    // 任务读不到才关弹窗；执行记录读不到走 attemptsError 就地提示，不掀桌子。
    if (!detail.value) {
      feedback.toast.error(e?.message || '打开任务失败')
      detailVisible.value = false
      return
    }
    attemptsError.value = e?.message || '执行记录读取失败'
  }
}

// 重试成败都重读详情与记录：409（状态已变、不可重试）也只有重读才看得出任务现在
// 真实的样子，不能让对话框停在重试前的旧快照上。
async function reloadDetail(id) {
  try {
    detail.value = await client.getComposeTask(id)
    attemptsError.value = ''
    attempts.value = await client.listComposeTaskAttempts(id)
  } catch (e) {
    attemptsError.value = e?.message || '执行记录读取失败'
  }
}

function retry(item) {
  const dialog = DialogPlugin.confirm({
    header: '重试失败任务',
    body: `确认重试任务 #${item.id}？任务将回到排队中，按创建时的策略快照重新执行。`,
    confirmBtn: { content: '重试', theme: 'primary' },
    onConfirm: async () => {
      dialog.destroy()
      try {
        await client.retryComposeTask(item.id)
        feedback.toast.success('已重新排队')
      } catch (e) {
        feedback.toast.error(e?.message || '重试失败')
      }
      await load()
      await reloadDetail(item.id)
    },
  })
}

function goOutput() {
  router.push({ name: 'FinishedMedia' })
}

function cancel(row) {
  const dialog = DialogPlugin.confirm({
    header: '取消合成任务',
    body: `确认取消任务 #${row.id}？排队中的任务取消后不再执行。`,
    confirmBtn: { content: '取消任务', theme: 'danger' },
    onConfirm: async () => {
      dialog.destroy()
      try {
        await client.cancelComposeTask(row.id)
        feedback.toast.success('已取消')
      } catch (e) {
        feedback.toast.error(e?.message || '取消失败')
      }
      // 成败都重读：409（已被领取）也只有重读才看得出任务现在是什么状态。
      await load()
    },
  })
}

onMounted(() => {
  load()
  loadStrategies()
  // 只为藏/显重试按钮；读失败不拦页面，角色判定由服务端兜底。
  sessionClient.me().then((me) => { currentUser.value = me }).catch(() => {})
})
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true">
    <div class="wt-resource-page compose-tasks-page">
      <FeedbackAlert v-if="error" theme="error" :message="error" closable style="margin-bottom:16px" @close="error=''" />
      <ResourcePageHeader title="合成任务" description="从我的素材创建的合成任务，在这里查看进度并取消排队中的任务">
        <template #actions>
          <t-button class="wt-secondary-button" variant="outline" @click="load">刷新</t-button>
        </template>
      </ResourcePageHeader>
      <ResourceCard class="compose-tasks-card">
        <div class="filter-bar">
          <label class="wt-filter-field"><span class="wt-filter-field__label">状态</span><t-select v-model="filters.status" :options="statusOptions" clearable placeholder="全部" /></label>
          <label class="wt-filter-field wt-filter-field--wide"><span class="wt-filter-field__label">合成策略</span><t-select v-model="filters.compose_strategy_id" :options="strategyOptions" clearable placeholder="全部" /></label>
          <label class="wt-filter-field compose-date-filter"><span class="wt-filter-field__label">创建时间</span><t-date-range-picker v-model="filters.created_range" value-type="YYYY-MM-DD" clearable /></label>
          <div class="wt-filter-actions">
            <t-button theme="primary" @click="load">查询</t-button>
            <t-button class="wt-secondary-button" variant="outline" @click="reset">重置</t-button>
          </div>
        </div>
        <div class="table-scroll-wrap">
          <t-table class="wt-resource-table" :data="pagedRows" :columns="columns" row-key="id" hover size="small" :scroll="tableScroll" empty="还没有合成任务">
            <template #strategy="{ row }">{{ strategyName(row) }}</template>
            <template #template="{ row }">{{ templateText(row) }}</template>
            <template #status="{ row }"><ResourceStatusBadge :tone="statusTones[row.status] || 'default'" :label="statusLabels[row.status] || row.status" /></template>
            <template #stage="{ row }">{{ stageText(row) }}</template>
            <template #progress="{ row }">{{ row.progress }}%</template>
            <template #created_at="{ row }">{{ formatDateTime(row.created_at) }}</template>
            <template #op="{ row }">
              <t-space class="wt-resource-actions">
                <t-button size="small" class="wt-secondary-button" variant="outline" @click="openDetail(row)">详情</t-button>
                <t-button v-if="row.status === 'pending'" size="small" class="wt-secondary-button wt-danger-button" variant="outline" @click="cancel(row)">取消</t-button>
              </t-space>
            </template>
          </t-table>
        </div>
        <p v-if="rows.length === 200" class="compose-list-limit">仅展示当前条件下最新的 200 条任务，请缩小时间范围继续查找。</p>
        <div class="pagination-bar"><t-pagination v-model:current="pagination.current" v-model:pageSize="pagination.pageSize" :total="rows.length" :page-size-options="[10, 20, 50]" /></div>
      </ResourceCard>

      <t-dialog v-model:visible="detailVisible" header="任务详情" width="640px" :footer="false">
        <t-loading :loading="!detail" :show-overlay="true">
          <div v-if="detail" class="compose-detail">
            <div class="compose-detail__grid">
              <div><span class="compose-detail__label">任务 ID</span><span>{{ detail.id }}</span></div>
              <div><span class="compose-detail__label">状态</span><ResourceStatusBadge :tone="statusTones[detail.status] || 'neutral'" :label="statusLabels[detail.status] || detail.status" /></div>
              <div><span class="compose-detail__label">阶段</span><span>{{ stageText(detail) }}</span></div>
              <div><span class="compose-detail__label">进度</span><span>{{ detail.progress }}%</span></div>
              <div><span class="compose-detail__label">创建时间</span><span>{{ formatDateTime(detail.created_at) }}</span></div>
              <div><span class="compose-detail__label">更新时间</span><span>{{ formatDateTime(detail.updated_at) }}</span></div>
              <div v-if="detail.started_at"><span class="compose-detail__label">开始时间</span><span>{{ formatDateTime(detail.started_at) }}</span></div>
              <div v-if="detail.finished_at"><span class="compose-detail__label">结束时间</span><span>{{ formatDateTime(detail.finished_at) }}</span></div>
            </div>
            <template v-if="detail.error_code || detail.error_message">
              <h4 class="compose-detail__title">失败信息</h4>
              <t-alert theme="error" :message="detail.error_message || detail.error_code" />
              <div v-if="detail.status === 'failed'" class="compose-detail__retry">
                <template v-if="canRetry && retryableCodes.has(detail.error_code)">
                  <t-button size="small" theme="primary" @click="retry(detail)">重试此任务</t-button>
                  <span v-if="detail.error_code === 'execution_failed'" class="compose-detail__hint">该失败属执行错误，请根据上方错误信息判断是否重试。</span>
                </template>
                <span v-else-if="newTaskCodes.has(detail.error_code)" class="compose-detail__hint">该失败不能重试，请修正后新建任务。</span>
              </div>
            </template>
            <h4 class="compose-detail__title">素材与策略快照（创建当时，不可变）</h4>
            <div class="compose-detail__grid">
              <div><span class="compose-detail__label">合成策略</span><span>#{{ detail.compose_strategy_id }}</span></div>
              <div><span class="compose-detail__label">模板</span><span>{{ templateText(detail) }}</span></div>
              <div><span class="compose-detail__label">素材 ID</span><span>{{ detail.material_id }}</span></div>
              <div><span class="compose-detail__label">执行次数</span><span>{{ detail.attempt_count }}</span></div>
            </div>
            <pre class="compose-detail__snapshot">{{ JSON.stringify(detail.config_snapshot, null, 2) }}</pre>
            <h4 class="compose-detail__title">执行记录</h4>
            <t-alert v-if="attemptsError" theme="warning" :message="attemptsError" />
            <template v-else>
              <p v-if="!attempts.length" class="compose-detail__empty">还没有执行记录。</p>
              <div v-for="attempt in attempts" :key="attempt.attempt" class="compose-detail__attempt">
                <span class="compose-detail__attempt-no">第 {{ attempt.attempt }} 次</span>
                <span class="compose-detail__attempt-source">{{ sourceLabels[attempt.source] || attempt.source }}发起</span>
                <ResourceStatusBadge :tone="outcomeTones[attempt.outcome] || 'neutral'" :label="outcomeLabels[attempt.outcome] || attempt.outcome" />
                <span class="compose-detail__attempt-time">{{ formatDateTime(attempt.started_at) }} ~ {{ formatDateTime(attempt.finished_at) }}</span>
                <span v-if="attempt.stage" class="compose-detail__attempt-stage">{{ attempt.stage }}</span>
                <span v-if="attempt.error_code || attempt.error_message" class="compose-detail__attempt-err">{{ attempt.error_message || attempt.error_code }}</span>
              </div>
            </template>
            <h4 class="compose-detail__title">成片结果</h4>
            <div v-if="detail.status === 'success' && detail.composite_output_id" class="compose-detail__output">
              <span>成片 #{{ detail.composite_output_id }}</span>
              <t-link theme="primary" @click="goOutput">前往我的成片查看</t-link>
            </div>
            <p v-else class="compose-detail__empty">任务完成后成片会显示在这里。</p>
          </div>
        </t-loading>
      </t-dialog>
    </div>
  </t-loading>
</template>

<style scoped>
.compose-tasks-card { padding: 14px 20px 18px; }
.filter-bar > .compose-date-filter { flex-basis: 300px; }
.compose-date-filter :deep(.t-date-range-picker) { min-width: 0; width: 100%; }
.compose-list-limit { margin: 8px 0 0; color: var(--wt-text-secondary); font-size: 12px; }
.compose-detail__grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px 24px; font-size: 13px; }
.compose-detail__grid > div { display: flex; gap: 8px; align-items: center; }
.compose-detail__label { color: var(--wt-text-secondary); min-width: 64px; }
.compose-detail__title { margin: 16px 0 8px; font-size: 13px; font-weight: 600; color: var(--wt-text-primary); }
.compose-detail__snapshot { margin: 0; padding: 12px; background: var(--wt-bg-secondary, #f5f6f8); border-radius: 6px; font-size: 12px; overflow: auto; max-height: 220px; }
.compose-detail__retry { display: flex; gap: 12px; align-items: center; margin-top: 10px; }
.compose-detail__hint { color: var(--wt-text-secondary); font-size: 12px; }
.compose-detail__empty { margin: 0; color: var(--wt-text-secondary); font-size: 12px; }
.compose-detail__attempt { display: flex; gap: 10px; align-items: center; flex-wrap: wrap; padding: 6px 0; border-bottom: 1px solid var(--wt-border-light, #eceef1); font-size: 12px; }
.compose-detail__attempt-no { font-weight: 600; color: var(--wt-text-primary); }
.compose-detail__attempt-source { color: var(--wt-text-secondary); }
.compose-detail__attempt-stage { color: var(--wt-text-secondary); }
.compose-detail__attempt-err { color: var(--wt-text-danger, #d54941); }
.compose-detail__output { display: flex; gap: 12px; align-items: center; font-size: 13px; }
</style>
