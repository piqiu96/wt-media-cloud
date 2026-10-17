<script setup>
import { feedback } from '../../../shared/ui/feedback/index.js'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { DialogPlugin } from 'tdesign-vue-next'
import { createComposeClient } from '../../../shared/api/compose.js'
import { createMaterialsClient } from '../../../shared/api/materials.js'
import { createSessionClient } from '../../../shared/api/session.js'
import ResourceCard from '../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../shared/ui/resource/ResourcePageHeader.vue'
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'
import { opColumnWidth } from '../../../shared/ui/resource/opColumnWidth.js'
import { formatDateTime } from '../../../shared/utils/datetime.js'
import { formatDurationMs } from '../../../shared/utils/units.js'
import { attemptSteps, stepMetricsText } from '../attemptSteps.js'
import { createdRangeParams } from '../createdRange.js'
import { UNCERTAIN_CODE, isRetryableTask, retryHint } from '../retryPolicy.js'
import { reviewReady, safeComposeError } from '../uncertainReview.js'
import { schemaFields } from '../schemaFields.js'
import { hasActiveTask, useTaskPolling } from '../useTaskPolling.js'
import MaterialDetailDrawer from '../../materials/MaterialDetailDrawer.vue'
import ComposeStrategySummary from '../components/ComposeStrategySummary.vue'
import ComposeOutputDetail from '../components/ComposeOutputDetail.vue'
import ComposeTaskCreateDialog from '../components/ComposeTaskCreateDialog.vue'

const client = createComposeClient()
const materialsClient = createMaterialsClient()
const sessionClient = createSessionClient()
const router = useRouter()

const rows = ref([])
const strategies = ref([])
const templates = ref([])
const games = ref([])
const teams = ref([])
const loading = ref(false)
const error = ref('')
const filters = ref({ status: '', trigger_type: '', compose_strategy_id: '', created_range: [] })
const pagination = ref({ current: 1, pageSize: 20 })
const createVisible = ref(false)

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

// 列序固定：识别 → 来源 → 结果 → 状态 → 时间 → 操作（AdminUI 规范 §4.1）。
// 执行阶段是过程信息，只在详情里；列表放的是决策信息。
const columns = computed(() => [
  { colKey: 'id', title: '任务 ID', width: 90 },
  { colKey: 'name', title: '任务名称', width: 240 },
  { colKey: 'trigger_type', title: '触发方式', width: 100 },
  { colKey: 'material_id', title: '关联素材', width: 140 },
  { colKey: 'strategy', title: '合成策略', width: 180 },
  { colKey: 'output', title: '生成成片', width: 120 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'created_at', title: '创建时间', width: 150 },
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
  return found ? found.name : `策略 ${row.compose_strategy_id}`
}

/** 当前阶段只说阶段，不带进度百分比；失败时也不拿错误码的文字冒充阶段。 */
function stageText(row) {
  if (row.status === 'success') return '已完成'
  // 阶段名里带「中」的说法（视频合成中）不能直接接「失败」，去掉尾字再拼。
  if (row.status === 'failed') return row.stage ? `${(stageLabels[row.stage] || '执行').replace(/中$/, '')}失败` : '执行失败'
  if (row.status === 'cancelled') return '已取消'
  if (row.status === 'pending') return row.stage === 'waiting_input' ? stageLabels.waiting_input : '等待执行'
  return stageLabels[row.stage] || '处理中'
}

// 任务名称是创建那一刻形成的事实：素材标题摘要 · 策略名称。历史行没有它，
// 退回「合成任务 252」——不显示内码，也不为了好看编一个名字。
function taskName(row) { return row.display_name?.trim() || `合成任务 ${row.id}` }
function triggerText(row) { return row.trigger_type === 'scheduled' ? '策略自动' : '人工创建' }
function outputID(row) { return row.status === 'success' ? row.composite_output_id : null }
function canRetryTask(row) { return canRetry.value && isRetryableTask(row) }
function canCancelTask(row) { return row.status === 'pending' || (row.status === 'running' && !row.cancel_requested) }
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

/**
 * 读列表。
 *
 * `quiet` 是轮询那一遍：不掀整页 Loading、不把用户正在看的页码拨回第一页、读失败也不
 * 换掉已经拿到的那一屏（一次网络抖动不该把列表清成错误页）。
 */
async function load({ quiet = false } = {}) {
  if (!quiet) loading.value = true
  if (!quiet) error.value = ''
  try {
    const params = {}
    if (filters.value.status) params.status = filters.value.status
    if (filters.value.trigger_type) params.trigger_type = filters.value.trigger_type
    if (filters.value.compose_strategy_id) params.compose_strategy_id = filters.value.compose_strategy_id
    Object.assign(params, createdRangeParams(filters.value.created_range))
    const data = await client.listComposeTasks(params)
    rows.value = Array.isArray(data) ? data : []
    if (!quiet) pagination.value.current = 1
  } catch (e) {
    if (!quiet) error.value = e?.message || '读取合成任务失败'
  } finally {
    if (!quiet) loading.value = false
    syncListPolling()
  }
}

function reset() {
  filters.value = { status: '', trigger_type: '', compose_strategy_id: '', created_range: [] }
  load()
}

// 新建走创建抽屉（它自己取「使用中的素材」当候选：入口只剩这一处了）。创建后回到第一屏
// 重读 —— 新的排队中任务会被列表轮询接管，不需要这一页自己再起一个 timer。
async function onCreated(result) {
  const created = result.created || []
  const failed = result.failed || []
  if (failed.length > 0) {
    feedback.toast.error(`已创建 ${created.length} 个任务，${failed.length} 个失败：${failed.map((item) => item.message).join('；')}`)
  } else {
    feedback.toast.success(`已创建 ${created.length} 个合成任务`)
  }
  await load()
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

// 关联抽屉要用的两张表：游戏名与分组名。读不到只影响那两行的说法，不拦页面。
async function loadLookups() {
  const [gameRows, teamRows] = await Promise.all([client.listGames().catch(() => []), client.listTeams().catch(() => [])])
  games.value = Array.isArray(gameRows) ? gameRows : []
  teams.value = Array.isArray(teamRows) ? teamRows : []
}
function gameLabel(id) { return games.value.find((game) => game.id === id)?.name || String(id ?? '—') }
function teamLabel(id) { return teams.value.find((team) => team.id === id)?.name || (id === undefined || id === null ? '—' : `分组 ${id}`) }

// —— 状态自动刷新 ——
// 列表这一份：有排队中/处理中的任务就每 5 秒重读一次，跑完最后一单就自己停。
const listPolling = useTaskPolling(() => load({ quiet: true }))
function syncListPolling() {
  if (hasActiveTask(rows.value)) listPolling.start()
  else listPolling.stop()
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

// 详情这一份：抽屉开着且任务还在跑时才轮询；关抽屉、到终态、离开页面都停。
const detailPolling = useTaskPolling(() => (detail.value ? reloadDetail(detail.value.id) : Promise.resolve()))
function syncDetailPolling() {
  if (detailVisible.value && (detail.value?.status === 'pending' || detail.value?.status === 'running')) detailPolling.start()
  else detailPolling.stop()
}

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

// 「哪些任务能重试」与「为什么没有按钮」都在 retryPolicy 一处：页面据此决定给不给入口，
// 详情据此说明理由，两句话不会各说各话。

// 尝试行的词表（M4-G）：source 区分自动重试与人工重试，outcome 是该次尝试自己的结局，
// 与任务行的 status 不是一回事。
const outcomeLabels = { pending: '等待执行', running: '处理中', success: '成功', failed: '失败', cancelled: '已取消' }
const outcomeTones = { pending: 'warning', running: 'info', success: 'success', failed: 'danger', cancelled: 'neutral' }

// 抽屉级业务动作只有重试与取消，都随任务状态与角色出现；两件都不成立就不渲染页脚。
const detailFooterOps = computed(() => {
  if (!detail.value) return []
  const ops = []
  if (canRetryTask(detail.value)) ops.push('retry')
  if (canCancelTask(detail.value)) ops.push('cancel')
  return ops
})

async function openDetail(row) {
  closeRelated()
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
  } finally {
    syncDetailPolling()
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
  } finally {
    syncDetailPolling()
  }
}

function closeDetail() {
  detailVisible.value = false
  detail.value = null
  syncDetailPolling()
}

// —— 关联实体：一律在当前抽屉上方开一层只读抽屉，不改路由（交互规范 §9.4）——
const materialDrawer = ref({ visible: false, loading: false, row: null })
const strategyDrawer = ref({ visible: false, loading: false, row: null })
const outputDrawer = ref({ visible: false, loading: false, previewLoading: false, detail: null, previewUrl: '', tab: 'basic' })

async function openMaterial(id) {
  materialDrawer.value = { visible: true, loading: true, row: null }
  try {
    materialDrawer.value.row = await materialsClient.get(id)
  } catch (e) {
    feedback.toast.error(e?.message || '打开素材失败')
    materialDrawer.value.visible = false
  } finally {
    materialDrawer.value.loading = false
  }
}

async function openStrategy(id) {
  strategyDrawer.value = { visible: true, loading: true, row: null }
  try {
    strategyDrawer.value.row = await client.getStrategy(id)
  } catch (e) {
    feedback.toast.error(e?.message || '打开策略失败')
    strategyDrawer.value.visible = false
  } finally {
    strategyDrawer.value.loading = false
  }
}

async function openOutput(id) {
  outputDrawer.value = { visible: true, loading: true, previewLoading: false, detail: null, previewUrl: '', tab: 'basic' }
  try {
    // 预览地址是短时签名，每次打开现取；取不到不挡详情，预览区自己给重试。
    const [data, link] = await Promise.all([
      client.getComposeOutput(id),
      client.getComposeOutputVideoUrl(id).catch(() => null),
    ])
    outputDrawer.value.detail = data
    outputDrawer.value.previewUrl = link?.url || ''
  } catch (e) {
    feedback.toast.error(e?.message || '打开成片失败')
    outputDrawer.value.visible = false
  } finally {
    outputDrawer.value.loading = false
  }
}

async function retryOutputPreview() {
  const id = outputDrawer.value.detail?.output?.id
  if (!id || outputDrawer.value.previewLoading) return
  outputDrawer.value.previewLoading = true
  try {
    const link = await client.getComposeOutputVideoUrl(id)
    outputDrawer.value.previewUrl = link?.url || ''
    if (!outputDrawer.value.previewUrl) feedback.toast.error('预览地址获取失败，请稍后重试')
  } catch (e) {
    feedback.toast.error(e?.message || '预览地址获取失败，请稍后重试')
  } finally {
    outputDrawer.value.previewLoading = false
  }
}

/** 关联抽屉都叠在任务详情之上；任务详情一关就一起收，不留孤儿层。 */
function closeRelated() {
  materialDrawer.value.visible = false
  strategyDrawer.value.visible = false
  outputDrawer.value.visible = false
}

// 「前往…页面」是关联抽屉里唯一的路由入口：用户自己点才离开当前页。
function goMaterialPage(id) { router.push({ name: 'MaterialLibrary', query: { material_id: String(id) } }) }
function goStrategyPage(id) { router.push({ name: 'ComposeStrategy', query: { strategy_id: String(id) } }) }
function goOutputPage(id) { router.push({ name: 'FinishedMedia', query: { output_id: String(id) } }) }

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
    header: '重试任务',
    body: `确认重试任务 ${item.id}？任务将回到排队中，按创建时的策略快照重新执行；自动容错重试机会重新计算。`,
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
    body: row.status === 'running' ? `确认取消任务 ${row.id}？系统将请求 Worker 停止执行，停止完成后任务才会变为已取消。` : `确认取消任务 ${row.id}？排队中的任务取消后不再执行。`,
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
      await reloadDetail(row.id)
    },
  })
}

onMounted(() => {
  load()
  loadStrategies()
  loadTemplates()
  loadLookups()
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
          <t-button theme="primary" @click="createVisible = true">新建合成任务</t-button>
          <t-button class="wt-secondary-button" variant="outline" @click="load()">刷新</t-button>
        </template>
      </ResourcePageHeader>
      <ResourceCard class="compose-tasks-card">
        <div class="filter-bar">
          <!-- 顺序按运营找任务时的顺序：先认策略，再认是谁触发的，然后才轮到状态与时间。 -->
          <label class="wt-filter-field wt-filter-field--wide"><span class="wt-filter-field__label">合成策略</span><t-select v-model="filters.compose_strategy_id" :options="strategyOptions" clearable placeholder="全部" /></label>
          <label class="wt-filter-field"><span class="wt-filter-field__label">触发方式</span><t-select v-model="filters.trigger_type" clearable placeholder="全部"><t-option value="manual" label="人工创建" /><t-option value="scheduled" label="策略自动" /></t-select></label>
          <label class="wt-filter-field"><span class="wt-filter-field__label">状态</span><t-select v-model="filters.status" :options="statusOptions" clearable placeholder="全部" /></label>
          <label class="wt-filter-field compose-date-filter"><span class="wt-filter-field__label">创建时间</span><t-date-range-picker v-model="filters.created_range" value-type="YYYY-MM-DD" clearable /></label>
          <div class="wt-filter-actions">
            <t-button theme="primary" @click="load()">查询</t-button>
            <t-button class="wt-secondary-button" variant="outline" @click="reset">重置</t-button>
          </div>
        </div>
        <div class="table-scroll-wrap">
          <t-table class="wt-resource-table" :data="pagedRows" :columns="columns" row-key="id" hover size="small" :scroll="tableScroll" empty="还没有合成任务">
            <template #id="{ row }"><span class="compose-task-id">{{ row.id }}</span></template>
            <template #name="{ row }"><span class="compose-task-name" :title="taskName(row)">{{ taskName(row) }}</span></template>
            <template #trigger_type="{ row }">{{ triggerText(row) }}</template>
            <template #material_id="{ row }"><t-link theme="primary" @click="openMaterial(row.material_id)">素材 {{ row.material_id }}</t-link></template>
            <template #strategy="{ row }"><t-link class="compose-strategy-link" theme="primary" :title="strategyName(row)" @click="openStrategy(row.compose_strategy_id)">{{ strategyName(row) }}</t-link></template>
            <template #output="{ row }"><t-link v-if="outputID(row)" theme="primary" @click="openOutput(outputID(row))">成片 {{ outputID(row) }}</t-link><span v-else>—</span></template>
            <template #status="{ row }"><ResourceStatusBadge :tone="statusTones[row.status] || 'default'" :label="statusLabels[row.status] || row.status" /></template>
            <template #created_at="{ row }">{{ formatDateTime(row.created_at) }}</template>
            <template #op="{ row }">
              <t-space class="wt-resource-actions">
                <t-button size="small" class="wt-secondary-button" variant="outline" @click="openDetail(row)">详情</t-button>
                <t-button v-if="canCancelTask(row)" size="small" class="wt-secondary-button wt-danger-button" variant="outline" @click="cancel(row)">取消</t-button>
                <t-button v-if="canRetryTask(row)" size="small" class="wt-secondary-button" variant="outline" @click="retry(row)">重试</t-button>
              </t-space>
            </template>
          </t-table>
        </div>
        <p v-if="rows.length === 200" class="compose-list-limit">仅展示当前条件下最新的 200 条任务，请缩小时间范围继续查找。</p>
        <div class="pagination-bar"><t-pagination v-model:current="pagination.current" v-model:pageSize="pagination.pageSize" :total="rows.length" :page-size-options="[10, 20, 50]" /></div>
      </ResourceCard>

      <t-drawer v-model:visible="detailVisible" header="任务详情" size="min(62vw, 880px)" :close-btn="true" :footer="detailFooterOps.length > 0" @close="closeDetail">
        <t-loading :loading="!detail" :show-overlay="true">
          <div v-if="detail" class="compose-detail">
            <t-tabs v-model="detailTab" class="compose-detail__tabs">
              <t-tab-panel value="overview" label="概览">
                <div class="compose-detail__section">
                  <h4>任务信息</h4>
                  <div class="compose-detail__grid">
                    <div><span class="compose-detail__label">任务名称</span><span>{{ taskName(detail) }}</span></div>
                    <div><span class="compose-detail__label">任务 ID</span><span>{{ detail.id }}</span></div>
                    <div><span class="compose-detail__label">触发方式</span><span>{{ triggerText(detail) }}</span></div>
                    <div><span class="compose-detail__label">创建人 ID</span><span>{{ detail.created_by }}</span></div>
                    <div><span class="compose-detail__label">创建时间</span><span>{{ formatDateTime(detail.created_at) }}</span></div>
                    <div><span class="compose-detail__label">当前状态</span><ResourceStatusBadge :tone="statusTones[detail.status] || 'neutral'" :label="statusLabels[detail.status] || detail.status" /></div>
                    <!-- 失败原因就写在任务信息里：找「为什么失败」的人先看的就是这一段，不必
                         再翻到执行记录去对错误码。列表页不加列（详情才需要这个上下文）。 -->
                    <div v-if="detail.status === 'failed'"><span class="compose-detail__label">失败原因</span><span class="compose-detail__failure-reason">{{ safeComposeError(detail.error_code) }}</span></div>
                  </div>
                  <!-- 重试按钮为什么不在，就写在这里：这一格给的是「不是所有失败都能重试」的
                       那个理由（错误类型 / 账号权限），两列网格里放不下，也不属于任务信息。 -->
                  <p v-if="retryHint(detail, canRetry)" class="compose-detail__hint">{{ retryHint(detail, canRetry) }}</p>
                </div>
                <div class="compose-detail__section">
                  <h4>关联信息</h4>
                  <div class="compose-detail__grid">
                    <div><span class="compose-detail__label">关联素材</span><t-link theme="primary" @click="openMaterial(detail.material_id)">素材 {{ detail.material_id }}</t-link></div>
                    <div><span class="compose-detail__label">合成策略</span><t-link theme="primary" @click="openStrategy(detail.compose_strategy_id)">{{ strategyName(detail) }}</t-link></div>
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
                  <!-- 成片只在「生成结果」里出现一次，生成时间自己占一格：这两样是并排的两条
                       事实，拼成一句话反而让人在一串文字里找时间。 -->
                  <div v-if="outputID(detail)" class="compose-detail__grid">
                    <div><span class="compose-detail__label">生成成片</span><t-link theme="primary" @click="openOutput(outputID(detail))">成片 {{ outputID(detail) }}</t-link></div>
                    <div><span class="compose-detail__label">生成时间</span><span>{{ outputSummary ? formatDateTime(outputSummary.created_at) : '—' }}</span></div>
                  </div>
                  <template v-else-if="detail.status === 'failed'">
                    <p class="compose-detail__empty">未生成成片</p>
                    <p class="compose-detail__hint">任务执行失败，因此没有生成成片。</p>
                  </template>
                  <p v-else-if="detail.status === 'cancelled'" class="compose-detail__empty">未生成成片</p>
                  <p v-else class="compose-detail__empty">成片尚未生成</p>
                </div>
              </t-tab-panel>
              <t-tab-panel value="attempts" label="执行记录">
                <div v-if="detail.error_code || detail.error_message" class="compose-detail__section">
                  <h4>失败原因</h4>
                  <FeedbackAlert theme="error" :message="detail.error_code === 'execution_uncertain' ? '上次执行结果不明确，等待管理员核对。系统尚未确认旧进程已经停止。' : safeComposeError(detail.error_code)" />
                  <!-- 与概览里那一句同源（retryPolicy），同一件事不并存两种说法。 -->
                  <p v-if="retryHint(detail, canRetry)" class="compose-detail__hint">{{ retryHint(detail, canRetry) }}</p>
                  <p v-else-if="detail.status === 'failed' && detail.error_code === 'execution_failed'" class="compose-detail__hint">请根据失败原因判断是否重试。</p>
                </div>
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
                      <!-- 过程：云端每走完一步追加一条，谁在哪一步倒下、每步花了多久都在这。
                           没有步骤的用户是更早的执行记录（那一列当时还不存在），如实说没有。 -->
                      <ul v-if="attemptSteps(attempt).length > 0" class="compose-detail__steps">
                        <li v-for="step in attemptSteps(attempt)" :key="step.step">
                          <span class="compose-detail__step-name">{{ step.label || '未记录' }}</span>
                          <span>{{ formatDateTime(step.finished_at) }}</span>
                          <span>用时 {{ formatDurationMs(step.duration_ms) }}</span>
                          <span v-if="stepMetricsText(step)" class="compose-detail__step-metrics">{{ stepMetricsText(step) }}</span>
                          <span v-if="step.outcome === 'failed'" class="compose-detail__step-error">失败：{{ step.error_label || '原因未记录' }}</span>
                        </li>
                      </ul>
                      <p v-else class="compose-detail__attempt-meta">该次执行没有步骤记录。</p>
                      <p v-if="attempt.outcome === 'success'" class="compose-detail__attempt-meta">本次执行成功，输出校验完成<span v-if="outputID(detail)">，生成成片 {{ outputID(detail) }}</span>。</p>
                      <template v-if="attempt.outcome === 'failed'">
                        <p class="compose-detail__attempt-meta">失败阶段：{{ stageLabels[attempt.stage] || '阶段未记录' }}</p>
                        <p class="compose-detail__attempt-err">失败原因：{{ attempt.error_code === 'execution_uncertain' ? '执行结果不明确，需管理员人工核对' : safeComposeError(attempt.error_code) }}</p>
                        <p class="compose-detail__attempt-meta">可重试：{{ attempt.error_code === UNCERTAIN_CODE ? '不可直接重试' : '可由高级运营提交重试' }}</p>
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
              <t-tab-panel value="snapshot" label="配置快照">
                <div class="compose-detail__section">
                  <h4>创建时的生效配置</h4>
                  <p class="compose-detail__hint">任务按创建时的配置执行，后续策略修改不会影响该任务。</p>
                  <div class="compose-detail__grid compose-detail__snapshot-fields">
                    <div><span class="compose-detail__label">创建时任务名称</span><span>{{ taskName(detail) }}</span></div>
                    <div><span class="compose-detail__label">合成策略 ID</span><span>{{ detail.compose_strategy_id }}</span></div>
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

        <!-- 业务动作只在页脚：状态与角色都不成立时不渲染页脚，关闭仍走右上角的 ×。 -->
        <template #footer>
          <div class="compose-detail__footer">
            <t-button v-if="detailFooterOps.includes('cancel')" class="wt-secondary-button wt-danger-button" variant="outline" @click="cancel(detail)">取消任务</t-button>
            <t-button v-if="detailFooterOps.includes('retry')" theme="primary" @click="retry(detail)">重试</t-button>
          </div>
        </template>
      </t-drawer>

      <!-- 关联实体：叠在任务详情之上的一层只读抽屉，返回时原筛选、原分页、原 Tab 都还在。 -->
      <MaterialDetailDrawer
        :visible="materialDrawer.visible"
        :material="materialDrawer.row"
        :loading="materialDrawer.loading"
        :games="games"
        mode="readonly"
        @update:visible="materialDrawer.visible = $event"
        @go-page="goMaterialPage(detail?.material_id)"
      />

      <t-drawer :visible="strategyDrawer.visible" header="策略详情" size="min(62vw, 880px)" :close-btn="true" @update:visible="strategyDrawer.visible = $event">
        <t-loading :loading="strategyDrawer.loading" :show-overlay="true">
          <ComposeStrategySummary
            v-if="strategyDrawer.row"
            :strategy="strategyDrawer.row"
            :team="teamLabel(strategyDrawer.row.team_id)"
            :game="gameLabel(strategyDrawer.row.game_id)"
            :templates="templates"
          />
        </t-loading>
        <template #footer>
          <div class="compose-detail__footer">
            <t-button class="wt-secondary-button" variant="outline" @click="goStrategyPage(strategyDrawer.row?.id)">前往策略页面</t-button>
          </div>
        </template>
      </t-drawer>

      <t-drawer :visible="outputDrawer.visible" header="成片详情" size="min(62vw, 880px)" :close-btn="true" @update:visible="outputDrawer.visible = $event">
        <t-loading :loading="outputDrawer.loading" :show-overlay="true">
          <ComposeOutputDetail
            v-model:tab="outputDrawer.tab"
            :detail="outputDrawer.detail"
            :preview-url="outputDrawer.previewUrl"
            :preview-loading="outputDrawer.previewLoading"
            :tabs="['basic', 'source']"
            @retry-preview="retryOutputPreview"
          />
        </t-loading>
        <template #footer>
          <div class="compose-detail__footer">
            <t-button class="wt-secondary-button" variant="outline" @click="goOutputPage(outputDrawer.detail?.output?.id)">前往成片页面</t-button>
          </div>
        </template>
      </t-drawer>

      <ComposeTaskCreateDialog v-model:visible="createVisible" @created="onCreated" />
    </div>
  </t-loading>
</template>

<style scoped>
.compose-tasks-card { padding: 14px 20px 18px; }
.filter-bar > .compose-date-filter { flex-basis: 300px; }
.compose-date-filter :deep(.t-date-range-picker) { min-width: 0; width: 100%; }
.compose-task-id { color: var(--wt-text-primary); font-weight: 600; font-variant-numeric: tabular-nums; }
.compose-task-name { display: block; overflow: hidden; color: var(--wt-text-primary); font-weight: 500; text-overflow: ellipsis; white-space: nowrap; }
/* 策略名长起来会把行撑成两行；与成片页同一处理：一行截断，全名挂在 title 上。 */
.compose-strategy-link { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.compose-list-limit { margin: 8px 0 0; color: var(--wt-text-secondary); font-size: 12px; }
.compose-detail { min-height: 320px; }
.compose-detail__tabs { margin-top: 4px; }
.compose-detail__section { padding: 16px 2px; }
.compose-detail__section h4 { margin: 0 0 14px; color: var(--wt-text-primary); font-size: 14px; font-weight: 600; }
.compose-detail__grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px 20px; font-size: 13px; }
.compose-detail__grid > div { display: flex; flex-direction: column; gap: 4px; min-width: 0; color: var(--wt-text-primary); }
.compose-detail__label { color: var(--wt-text-tertiary); font-size: 12px; }
.compose-detail__snapshot-fields { margin-top: 16px; }
/* 失败态一律用 --wt-danger：语义表 tokens.css 里的 --wt-status-error 从未被任何入口引入，运行时取不到值；
   这支红与紧邻「当前状态」的失败徽标（statusTones.failed=danger）同色。 */
.compose-detail__failure-reason { color: var(--wt-danger); font-weight: 500; }
.compose-detail__hint { margin: 0; color: var(--wt-text-secondary); font-size: 12px; }
.compose-detail__empty { margin: 0 0 4px; color: var(--wt-text-secondary); font-size: 13px; }
.compose-detail__attempt { padding: 14px; border: 1px solid var(--wt-border); border-radius: var(--wt-radius-sm); margin-bottom: 10px; }
.compose-detail__attempt-head { display: flex; justify-content: space-between; gap: 12px; align-items: center; }
.compose-detail__attempt-head strong { color: var(--wt-text-primary); font-size: 13px; }
.compose-detail__attempt-meta { margin-top: 8px; color: var(--wt-text-secondary); font-size: 12px; }
.compose-detail__attempt-err { margin: 8px 0 0; color: var(--wt-danger); font-size: 12px; }
.compose-detail__steps { margin: 10px 0 0; padding: 0; list-style: none; }
.compose-detail__steps li { display: flex; flex-wrap: wrap; align-items: baseline; gap: 10px; padding: 4px 0; border-top: 1px dashed var(--wt-border); color: var(--wt-text-secondary); font-size: 12px; }
.compose-detail__step-name { min-width: 108px; color: var(--wt-text-primary); font-weight: 500; }
.compose-detail__step-metrics { color: var(--wt-text-tertiary); }
.compose-detail__step-error { color: var(--wt-danger); }
.compose-detail__review { display: flex; flex-direction: column; align-items: flex-start; gap: 12px; }
.compose-detail__review :deep(.t-textarea) { width: 100%; }
.compose-detail__review-evidence { margin: 8px 0 0; color: var(--wt-text-primary); font-size: 13px; white-space: pre-wrap; overflow-wrap: anywhere; }
.compose-detail__footer { display: flex; justify-content: flex-end; align-items: center; gap: 8px; }
@media (max-width: 760px) { .compose-detail__grid { grid-template-columns: 1fr; } }
</style>
