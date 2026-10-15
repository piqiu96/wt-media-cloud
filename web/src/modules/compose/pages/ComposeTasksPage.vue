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
import { reviewReady, safeComposeError } from '../uncertainReview.js'
import { schemaFields } from '../schemaFields.js'

const client = createComposeClient()
const sessionClient = createSessionClient()
const router = useRouter()

const rows = ref([])
const strategies = ref([])
const templates = ref([])
const loading = ref(false)
const error = ref('')
const filters = ref({ status: '', trigger_type: '', compose_strategy_id: '', created_range: [] })
const pagination = ref({ current: 1, pageSize: 20 })

const statusLabels = { pending: '排队中', running: '处理中', success: '成功', failed: '失败', cancelled: '已取消' }
const statusTones = { pending: 'warning', running: 'info', success: 'success', failed: 'danger', cancelled: 'neutral' }
const stageLabels = {
  pending: '等待源素材',
  validating_input: '校验素材',
  waiting_input: '等待素材准备',
  cutting: '视频合成中',
  uploading: '上传成片',
  committing: '保存成片',
}

const columns = computed(() => [
  { colKey: 'task', title: '任务', width: 265 },
  { colKey: 'trigger_type', title: '触发方式', width: 100 },
  { colKey: 'material_id', title: '关联素材', width: 145 },
  { colKey: 'strategy', title: '合成策略', width: 190 },
  { colKey: 'output', title: '生成成片', width: 120 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'stage', title: '执行阶段', width: 170 },
  { colKey: 'op', title: '操作', width: opColumnWidth(['详情', '取消', '重试']), fixed: 'right' },
])

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

function stageText(row) {
  if (row.status === 'success') return '已完成'
  if (row.status === 'failed') return row.stage ? `${stageLabels[row.stage] || '执行'}失败` : safeComposeError(row.error_code)
  if (row.status === 'cancelled') return '已取消'
  if (row.status === 'pending') return row.stage === 'waiting_input' ? stageLabels.waiting_input : '等待执行'
  const stage = stageLabels[row.stage] || '处理中'
  return row.status === 'running' && Number.isFinite(row.progress) && row.progress > 0 ? `${stage} · ${row.progress}%` : stage
}

function taskName(row) { return row.display_name?.trim() || `合成任务 #${row.id}` }
function triggerText(row) { return row.trigger_type === 'scheduled' ? '策略自动' : '人工创建' }
function outputID(row) { return row.status === 'success' ? row.composite_output_id : null }
function canRetryTask(row) { return canRetry.value && row.status === 'failed' && retryableCodes.has(row.error_code) }
function goMaterial(row) { router.push({ name: 'MaterialLibrary', query: { material_id: String(row.material_id) } }) }
function goStrategy(row) { router.push({ name: 'ComposeStrategy', query: { strategy_id: String(row.compose_strategy_id) } }) }
function goOutput(row) { if (outputID(row)) router.push({ name: 'FinishedMedia', query: { output_id: String(outputID(row)) } }) }
function snapshotFields(row) {
  const template = templates.value.find((item) => item.code === row.template_code && item.version === row.template_version)
  return schemaFields(template?.schema).filter((field) => Object.hasOwn(row.config_snapshot || {}, field.key) && field.key !== 'guide_video_url')
}
function snapshotValue(row, field) {
  const value = row.config_snapshot?.[field.key]
  if (typeof value === 'boolean') return value ? '开启' : '关闭'
  if (typeof value === 'number' && field.key.endsWith('_seconds')) return `${value} 秒`
  return typeof value === 'string' && !value.includes('://') ? value : '—'
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const params = {}
    if (filters.value.status) params.status = filters.value.status
    if (filters.value.trigger_type) params.trigger_type = filters.value.trigger_type
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
  filters.value = { status: '', trigger_type: '', compose_strategy_id: '', created_range: [] }
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

async function loadTemplates() {
  try { templates.value = await client.listTemplates() || [] } catch { templates.value = [] }
}

const detailVisible = ref(false)
const detail = ref(null)
const outputSummary = ref(null)
const detailTab = ref('overview')
const attempts = ref([])
const attemptsError = ref('')
// 与 ComposeStrategyPage 同一规则：页面据此藏重试入口；真正的授权判定在服务层，
// 藏按钮只是少一次必失败请求。
const currentUser = ref(null)
const canRetry = computed(() => ['senior_operator', 'admin'].includes(currentUser.value?.role))
const canReviewUncertain = computed(() => currentUser.value?.role === 'admin')
const reviews = ref([])
const reviewsError = ref('')
const reviewBusy = ref(false)
const reviewInput = ref({ oldExecutionStopped: false, noOutputCommitted: false, evidence: '' })
const reviewValid = computed(() => reviewReady(reviewInput.value))

async function loadReviews(id) {
  reviews.value = []
  reviewsError.value = ''
  if (!canReviewUncertain.value) return
  try {
    const data = await client.listComposeUncertainReviews(id)
    reviews.value = Array.isArray(data) ? data : []
  } catch (e) {
    reviewsError.value = e?.message || '核对记录读取失败'
  }
}

// 重试分类（change.md 决定 2，服务端 error_code IN 同一张表）：storage_error/task_timeout
// 环境类可直接重试；execution_failed 属人工判断，按钮照给，是否重试由运营看错误信息定；
// input_missing/input_invalid/template_unavailable 不可重试，页面不给入口只引导新建任务。
const retryableCodes = new Set(['storage_error', 'task_timeout', 'execution_failed'])
const newTaskCodes = new Set(['input_missing', 'input_invalid', 'template_unavailable'])

// 尝试行的词表（M4-G）：source 区分自动重试与人工重试，outcome 是该次尝试自己的结局，
// 与任务行的 status 不是一回事。
const outcomeLabels = { pending: '等待执行', running: '处理中', success: '成功', failed: '失败', cancelled: '已取消' }
const outcomeTones = { pending: 'warning', running: 'info', success: 'success', failed: 'danger', cancelled: 'neutral' }

async function openDetail(row) {
  detailVisible.value = true
  detailTab.value = 'overview'
  detail.value = null
  outputSummary.value = null
  attempts.value = []
  attemptsError.value = ''
  reviews.value = []
  reviewsError.value = ''
  reviewInput.value = { oldExecutionStopped: false, noOutputCommitted: false, evidence: '' }
  try {
    // 按 id 重取：详情里的快照必须是库里存的那份，不是列表加载那一刻的副本。
    // 执行记录一起取：详情要按「任务状态 → 快照 → 执行记录 → 成片」组织，逐次记录
    // 不能拿任务摘要冒充。
    detail.value = await client.getComposeTask(row.id)
    if (outputID(detail.value)) {
      client.getComposeOutput(outputID(detail.value)).then((data) => { if (detail.value?.id === row.id) outputSummary.value = data?.output || null }).catch(() => {})
    }
    attempts.value = []
    attemptsError.value = ''
    attempts.value = await client.listComposeTaskAttempts(row.id)
    await loadReviews(row.id)
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
    outputSummary.value = null
    if (outputID(detail.value)) {
      client.getComposeOutput(outputID(detail.value)).then((data) => { if (detail.value?.id === id) outputSummary.value = data?.output || null }).catch(() => {})
    }
    attemptsError.value = ''
    attempts.value = await client.listComposeTaskAttempts(id)
    await loadReviews(id)
  } catch (e) {
    attemptsError.value = e?.message || '执行记录读取失败'
  }
}

async function resolveUncertain() {
  if (!detail.value || !canReviewUncertain.value || !reviewValid.value || reviewBusy.value) return
  const id = detail.value.id
  reviewBusy.value = true
  try {
    await client.resolveComposeUncertain(id, {
      old_execution_stopped: reviewInput.value.oldExecutionStopped,
      no_output_committed: reviewInput.value.noOutputCommitted,
      evidence: reviewInput.value.evidence.trim(),
    })
    feedback.toast.success('核对依据已记录，任务已重新排队')
    reviewInput.value = { oldExecutionStopped: false, noOutputCommitted: false, evidence: '' }
  } catch (e) {
    feedback.toast.error(e?.message || '核对处置失败，请刷新任务状态')
  } finally {
    await load()
    await reloadDetail(id)
    reviewBusy.value = false
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
        feedback.toast.success('重试任务已提交')
      } catch (e) {
        feedback.toast.error(e?.message || '重试失败')
      }
      await load()
      await reloadDetail(item.id)
    },
  })
}

function cancel(row) {
  const dialog = DialogPlugin.confirm({
    header: '取消合成任务',
    body: row.status === 'running' ? `确认取消任务 #${row.id}？系统将请求 Worker 停止执行，停止完成后任务才会变为已取消。` : `确认取消任务 #${row.id}？排队中的任务取消后不再执行。`,
    confirmBtn: { content: '取消任务', theme: 'danger' },
    onConfirm: async () => {
      dialog.destroy()
      try {
        const result = await client.cancelComposeTask(row.id)
        feedback.toast.success(result.status === 'cancelled' ? '已取消' : '取消请求已提交')
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
  loadTemplates()
  // 只为藏/显重试按钮；读失败不拦页面，角色判定由服务端兜底。
  sessionClient.me().then((me) => {
    currentUser.value = me
    if (detail.value) loadReviews(detail.value.id)
  }).catch(() => {})
})
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true">
    <div class="wt-resource-page compose-tasks-page">
      <FeedbackAlert v-if="error" theme="error" :message="error" closable style="margin-bottom:16px" @close="error=''" />
      <ResourcePageHeader title="合成任务" description="从我的素材创建的合成任务，在这里查看任务状态和生产结果">
        <template #actions>
          <t-button class="wt-secondary-button" variant="outline" @click="load">刷新</t-button>
        </template>
      </ResourcePageHeader>
      <ResourceCard class="compose-tasks-card">
        <div class="filter-bar">
          <label class="wt-filter-field"><span class="wt-filter-field__label">状态</span><t-select v-model="filters.status" :options="statusOptions" clearable placeholder="全部" /></label>
          <label class="wt-filter-field"><span class="wt-filter-field__label">触发方式</span><t-select v-model="filters.trigger_type" clearable placeholder="全部"><t-option value="manual" label="人工创建" /><t-option value="scheduled" label="策略自动" /></t-select></label>
          <label class="wt-filter-field wt-filter-field--wide"><span class="wt-filter-field__label">合成策略</span><t-select v-model="filters.compose_strategy_id" :options="strategyOptions" clearable placeholder="全部" /></label>
          <label class="wt-filter-field compose-date-filter"><span class="wt-filter-field__label">创建时间</span><t-date-range-picker v-model="filters.created_range" value-type="YYYY-MM-DD" clearable /></label>
          <div class="wt-filter-actions">
            <t-button theme="primary" @click="load">查询</t-button>
            <t-button class="wt-secondary-button" variant="outline" @click="reset">重置</t-button>
          </div>
        </div>
        <div class="table-scroll-wrap">
          <t-table class="wt-resource-table" :data="pagedRows" :columns="columns" row-key="id" hover size="small" :scroll="tableScroll" empty="还没有合成任务">
            <template #task="{ row }">
              <div class="compose-related-cell">
                <strong class="compose-related-cell__main" :title="taskName(row)">{{ taskName(row) }}</strong>
                <span class="compose-related-cell__sub">#{{ row.id }} · {{ formatDateTime(row.created_at) }}</span>
              </div>
            </template>
            <template #trigger_type="{ row }">{{ triggerText(row) }}</template>
            <template #material_id="{ row }"><t-link theme="primary" @click="goMaterial(row)">素材 #{{ row.material_id }}</t-link></template>
            <template #strategy="{ row }">
              <div class="compose-related-cell">
                <t-link theme="primary" :title="strategyName(row)" @click="goStrategy(row)">{{ strategyName(row) }}</t-link>
              </div>
            </template>
            <template #output="{ row }"><t-link v-if="outputID(row)" theme="primary" @click="goOutput(row)">成片 #{{ outputID(row) }}</t-link><span v-else>—</span></template>
            <template #status="{ row }"><ResourceStatusBadge :tone="statusTones[row.status] || 'default'" :label="statusLabels[row.status] || row.status" /></template>
            <template #stage="{ row }">{{ stageText(row) }}</template>
            <template #op="{ row }">
              <t-space class="wt-resource-actions">
                <t-button size="small" class="wt-secondary-button" variant="outline" @click="openDetail(row)">详情</t-button>
                <t-button v-if="row.status === 'pending' || (row.status === 'running' && !row.cancel_requested)" size="small" class="wt-secondary-button wt-danger-button" variant="outline" @click="cancel(row)">取消</t-button>
                <t-button v-if="canRetryTask(row)" size="small" class="wt-secondary-button" variant="outline" @click="retry(row)">重试</t-button>
              </t-space>
            </template>
          </t-table>
        </div>
        <p v-if="rows.length === 200" class="compose-list-limit">仅展示当前条件下最新的 200 条任务，请缩小时间范围继续查找。</p>
        <div class="pagination-bar"><span>共 {{ rows.length }} 条</span><t-pagination v-model:current="pagination.current" v-model:pageSize="pagination.pageSize" :total="rows.length" :page-size-options="[10, 20, 50]" /></div>
      </ResourceCard>

      <t-drawer v-model:visible="detailVisible" header="任务详情" size="720px" :close-btn="true" :footer="false">
        <t-loading :loading="!detail" :show-overlay="true">
          <div v-if="detail" class="compose-detail">
            <div class="compose-detail__hero">
              <div class="compose-detail__identity">
                <div><strong>{{ taskName(detail) }}</strong><span>#{{ detail.id }} · {{ triggerText(detail) }} · 创建于 {{ formatDateTime(detail.created_at) }}</span></div>
                <ResourceStatusBadge :tone="statusTones[detail.status] || 'neutral'" :label="statusLabels[detail.status] || detail.status" />
              </div>
              <div v-if="detail.status === 'running' || detail.status === 'failed'" class="compose-detail__progress">当前阶段：{{ stageText(detail) }}</div>
            </div>
            <t-tabs v-model="detailTab" class="compose-detail__tabs">
              <t-tab-panel value="overview" label="概览">
                <div class="compose-detail__section">
                  <h4>任务信息</h4>
                  <div class="compose-detail__grid">
                    <div><span class="compose-detail__label">任务名称</span><span>{{ taskName(detail) }}</span></div>
                    <div><span class="compose-detail__label">任务 ID</span><span>#{{ detail.id }}</span></div>
                    <div><span class="compose-detail__label">触发方式</span><span>{{ triggerText(detail) }}</span></div>
                    <div><span class="compose-detail__label">创建人 ID</span><span>#{{ detail.created_by }}</span></div>
                    <div><span class="compose-detail__label">创建时间</span><span>{{ formatDateTime(detail.created_at) }}</span></div>
                    <div><span class="compose-detail__label">当前状态</span><ResourceStatusBadge :tone="statusTones[detail.status] || 'neutral'" :label="statusLabels[detail.status] || detail.status" /></div>
                  </div>
                </div>
                <div class="compose-detail__section">
                  <h4>关联信息</h4>
                  <div class="compose-detail__grid">
                    <div><span class="compose-detail__label">关联素材</span><t-link theme="primary" @click="goMaterial(detail)">素材 #{{ detail.material_id }}</t-link></div>
                    <div><span class="compose-detail__label">合成策略</span><t-link theme="primary" @click="goStrategy(detail)">{{ strategyName(detail) }}</t-link></div>
                    <div><span class="compose-detail__label">生成成片</span><t-link v-if="outputID(detail)" theme="primary" @click="goOutput(detail)">成片 #{{ outputID(detail) }}</t-link><span v-else>—</span></div>
                  </div>
                </div>
                <div class="compose-detail__section">
                  <h4>执行信息</h4>
                  <div class="compose-detail__grid">
                    <div><span class="compose-detail__label">当前阶段</span><span>{{ stageText(detail) }}</span></div>
                    <div><span class="compose-detail__label">执行次数</span><span>{{ detail.attempt_count }}</span></div>
                    <div><span class="compose-detail__label">开始时间</span><span>{{ formatDateTime(detail.started_at) }}</span></div>
                    <div><span class="compose-detail__label">结束时间</span><span>{{ formatDateTime(detail.finished_at) }}</span></div>
                    <div><span class="compose-detail__label">更新时间</span><span>{{ formatDateTime(detail.updated_at) }}</span></div>
                  </div>
                </div>
                <div class="compose-detail__section">
                  <h4>生成结果</h4>
                  <div v-if="outputID(detail)" class="compose-detail__output">
                    <t-link theme="primary" @click="goOutput(detail)">成片 #{{ outputID(detail) }} · 查看成片详情</t-link>
                    <span v-if="outputSummary">生成于 {{ formatDateTime(outputSummary.created_at) }}</span>
                  </div>
                  <p v-else class="compose-detail__empty">{{ detail.status === 'failed' ? `任务在「${stageText(detail)}」阶段失败，未生成成片。` : '任务尚未生成成片。' }}</p>
                </div>
                <div v-if="detail.error_code || detail.error_message" class="compose-detail__section">
                  <h4>失败原因</h4>
                  <FeedbackAlert theme="error" :message="detail.error_code === 'execution_uncertain' ? '上次执行结果不明确，等待管理员核对。系统尚未确认旧进程已经停止。' : safeComposeError(detail.error_code)" />
                  <div v-if="detail.status === 'failed'" class="compose-detail__retry">
                    <template v-if="canRetry && retryableCodes.has(detail.error_code)">
                      <t-button theme="primary" @click="retry(detail)">重试</t-button>
                      <span v-if="detail.error_code === 'execution_failed'" class="compose-detail__hint">请根据失败原因判断是否重新执行。</span>
                    </template>
                    <span v-else-if="newTaskCodes.has(detail.error_code)" class="compose-detail__hint">该失败不能重试，请修正后新建任务。</span>
                  </div>
                </div>
                <div v-if="detail.status === 'failed' && detail.error_code === 'execution_uncertain'" class="compose-detail__section compose-detail__review">
                  <h4>待管理员核对</h4>
                  <p class="compose-detail__hint">租约过期不代表进程已停止。请在原 Worker 节点和成片记录中人工核查；无法确认时保持当前失败状态。</p>
                  <template v-if="canReviewUncertain">
                    <t-checkbox v-model="reviewInput.oldExecutionStopped">我已核实旧 FFmpeg 已停止</t-checkbox>
                    <t-checkbox v-model="reviewInput.noOutputCommitted">我已核实没有已提交成片</t-checkbox>
                    <t-textarea v-model="reviewInput.evidence" :maxlength="2000" placeholder="记录核对依据（至少 20 字，不要填写凭据或签名链接）" />
                    <t-button theme="primary" :disabled="!reviewValid || reviewBusy" :loading="reviewBusy" @click="resolveUncertain">记录核对并人工重试</t-button>
                    <p class="compose-detail__hint">此操作记录管理员声明并重新排队；系统无法自动证明旧进程已停止。</p>
                  </template>
                  <p v-else class="compose-detail__hint">请联系管理员核对，当前任务不会自动重试。</p>
                </div>
              </t-tab-panel>
              <t-tab-panel value="attempts" label="执行记录">
                <div class="compose-detail__section">
                  <FeedbackAlert v-if="attemptsError" theme="error" :message="attemptsError" />
                  <template v-else>
                    <p v-if="!attempts.length" class="compose-detail__empty">还没有执行记录。</p>
                    <div v-for="attempt in attempts" :key="attempt.attempt" class="compose-detail__attempt">
                      <div class="compose-detail__attempt-head">
                        <strong>第 {{ attempt.attempt }} 次执行</strong>
                        <ResourceStatusBadge :tone="outcomeTones[attempt.outcome] || 'neutral'" :label="outcomeLabels[attempt.outcome] || '状态未知'" />
                      </div>
                      <div class="compose-detail__attempt-meta">开始：{{ formatDateTime(attempt.started_at) }} · 结束：{{ formatDateTime(attempt.finished_at) }}</div>
                      <p v-if="attempt.outcome === 'success'" class="compose-detail__attempt-meta">本次执行成功，输出校验完成<span v-if="outputID(detail)">，生成成片 #{{ outputID(detail) }}</span>。</p>
                      <template v-if="attempt.outcome === 'failed'">
                        <p class="compose-detail__attempt-meta">失败阶段：{{ stageLabels[attempt.stage] || '阶段未记录' }}</p>
                        <p class="compose-detail__attempt-err">失败原因：{{ attempt.error_code === 'execution_uncertain' ? '执行结果不明确，需管理员人工核对' : safeComposeError(attempt.error_code) }}</p>
                        <p class="compose-detail__attempt-meta">可重试：{{ retryableCodes.has(attempt.error_code) ? '可由高级运营提交重试' : '不可直接重试' }}</p>
                      </template>
                    </div>
                    <template v-if="canReviewUncertain">
                      <FeedbackAlert v-if="reviewsError" theme="error" :message="reviewsError" />
                      <div v-for="review in reviews" :key="review.id" class="compose-detail__attempt">
                        <strong>第 {{ review.attempt }} 次结果不明确 · 管理员核对</strong>
                        <p class="compose-detail__attempt-meta">管理员 {{ review.actor_user_id }} · {{ formatDateTime(review.created_at) }}</p>
                        <p class="compose-detail__review-evidence">{{ review.evidence }}</p>
                      </div>
                    </template>
                  </template>
                </div>
              </t-tab-panel>
              <t-tab-panel value="snapshot" label="配置快照">
                <div class="compose-detail__section">
                  <h4>创建时的生效配置</h4>
                  <p class="compose-detail__hint">任务按创建时的配置执行，后续策略修改不会影响该任务。</p>
                  <div class="compose-detail__grid compose-detail__snapshot-fields">
                    <div><span class="compose-detail__label">创建时任务名称</span><span>{{ taskName(detail) }}</span></div>
                    <div><span class="compose-detail__label">合成策略 ID</span><span>#{{ detail.compose_strategy_id }}</span></div>
                    <div><span class="compose-detail__label">模板</span><span>{{ detail.template_code }}</span></div>
                    <div><span class="compose-detail__label">模板版本</span><span>v{{ detail.template_version }}</span></div>
                    <div v-for="field in snapshotFields(detail)" :key="field.key"><span class="compose-detail__label">{{ field.label }}</span><span>{{ snapshotValue(detail, field) }}</span></div>
                  </div>
                  <p v-if="!snapshotFields(detail).length" class="compose-detail__hint">当前没有可安全展示的模板参数定义。</p>
                </div>
              </t-tab-panel>
            </t-tabs>
          </div>
        </t-loading>
      </t-drawer>
    </div>
  </t-loading>
</template>

<style scoped>
.compose-tasks-card { padding: 14px 20px 18px; }
.filter-bar > .compose-date-filter { flex-basis: 300px; }
.compose-date-filter :deep(.t-date-range-picker) { min-width: 0; width: 100%; }
.compose-related-cell { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.compose-related-cell__main { overflow: hidden; color: var(--wt-text-primary); font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }
.compose-related-cell__sub { color: var(--wt-text-tertiary); font-size: 12px; }
.compose-list-limit { margin: 8px 0 0; color: var(--wt-text-secondary); font-size: 12px; }
.compose-detail { min-height: 320px; }
.compose-detail__hero { padding: 16px; border: 1px solid var(--wt-border); border-radius: var(--wt-radius-md); background: var(--wt-bg-page); }
.compose-detail__identity { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; }
.compose-detail__identity > div { display: flex; flex-direction: column; gap: 3px; }
.compose-detail__identity strong { color: var(--wt-text-primary); font-size: 18px; }
.compose-detail__identity span { color: var(--wt-text-tertiary); font-size: 12px; }
.compose-detail__progress { display: flex; justify-content: space-between; margin: 16px 0 6px; color: var(--wt-text-secondary); font-size: 13px; }
.compose-detail__progress strong { color: var(--wt-text-primary); font-variant-numeric: tabular-nums; }
.compose-detail__tabs { margin-top: 12px; }
.compose-detail__section { padding: 16px 2px; }
.compose-detail__section h4 { margin: 0 0 14px; color: var(--wt-text-primary); font-size: 14px; font-weight: 600; }
.compose-detail__grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px 20px; font-size: 13px; }
.compose-detail__grid > div { display: flex; flex-direction: column; gap: 4px; min-width: 0; color: var(--wt-text-primary); }
.compose-detail__label { color: var(--wt-text-tertiary); font-size: 12px; }
.compose-detail__snapshot-fields { margin-top: 16px; }
.compose-detail__output { display: flex; flex-direction: column; align-items: flex-start; gap: 4px; font-size: 13px; }
.compose-detail__output span { color: var(--wt-text-tertiary); }
.compose-detail__retry { display: flex; gap: 12px; align-items: center; margin-top: 12px; }
.compose-detail__hint { margin: 0; color: var(--wt-text-secondary); font-size: 12px; }
.compose-detail__empty { margin: 0; color: var(--wt-text-secondary); font-size: 13px; }
.compose-detail__attempt { padding: 14px; border: 1px solid var(--wt-border); border-radius: var(--wt-radius-sm); margin-bottom: 10px; }
.compose-detail__attempt-head { display: flex; justify-content: space-between; gap: 12px; align-items: center; }
.compose-detail__attempt-head strong { color: var(--wt-text-primary); font-size: 13px; }
.compose-detail__attempt-meta { margin-top: 8px; color: var(--wt-text-secondary); font-size: 12px; }
.compose-detail__attempt-err { margin: 8px 0 0; color: var(--wt-status-error); font-size: 12px; }
.compose-detail__review { display: flex; flex-direction: column; align-items: flex-start; gap: 12px; }
.compose-detail__review :deep(.t-textarea) { width: 100%; }
.compose-detail__review-evidence { margin: 8px 0 0; color: var(--wt-text-primary); font-size: 13px; white-space: pre-wrap; overflow-wrap: anywhere; }
@media (max-width: 760px) { .compose-detail__grid { grid-template-columns: 1fr; } }
</style>
