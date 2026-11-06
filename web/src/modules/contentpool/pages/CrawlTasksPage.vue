<script setup>
import '../mining.css'
import { scheduleLabel } from '../labels.js'
import { materialRulesSummary } from '../miningRules.js'
import { taskContents, taskExecutionEvents, taskExceptions, taskResultSummary } from '../crawlTaskDetail.js'
import { opColumnWidth } from '../../../shared/ui/resource/opColumnWidth.js'
import ResourceFilterBar from '../../../shared/ui/resource/ResourceFilterBar.vue'
import ResourceRowActions from '../../../shared/ui/resource/ResourceRowActions.vue'
import ResourceOverlay from '../../../shared/ui/resource/ResourceOverlay.vue'
import ResourceSection from '../../../shared/ui/resource/ResourceSection.vue'
import ResourceDescriptions from '../../../shared/ui/resource/ResourceDescriptions.vue'
import ResourceNotice from '../../../shared/ui/resource/ResourceNotice.vue'
import BusinessSelect from "../../../shared/ui/fields/BusinessSelect.vue"
import { feedback } from '../../../shared/ui/feedback/index.js'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { createDiscoveryClient } from '../../../shared/api/discovery.js'
import { createSessionClient } from '../../../shared/api/session.js'
import { createUsersClient } from '../../../apps/cloud/pages/users/usersApi.js'
import ResourceCard from '../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../shared/ui/resource/ResourcePageHeader.vue'
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'
import MetricList from '../../../shared/ui/resource/MetricList.vue'
import { formatDateTime } from '../../../shared/utils/datetime.js'
import { platformLabel } from '../../../shared/utils/platforms.js'
import { useAutoRefresh } from '../../../shared/utils/useAutoRefresh.js'

const client = createDiscoveryClient()
const session = createSessionClient()
const users = createUsersClient()
const route = useRoute()
const router = useRouter()
const rows = ref([])
const teams = ref([])
const currentUser = ref(null)
const teamFilter = ref('')
const loading = ref(false)
const error = ref('')
const detail = ref(null)
const visible = ref(false)
const retrying = ref(false)
const activeTab = ref('info')
const failedCovers = ref(new Set())
const contentPagination = ref({ current: 1, pageSize: 10 })
const detailContents = computed(() => detail.value ? taskContents(detail.value) : [])
const pagedContents = computed(() => detailContents.value.slice((contentPagination.value.current - 1) * contentPagination.value.pageSize, contentPagination.value.current * contentPagination.value.pageSize))
const detailExceptions = computed(() => detail.value ? taskExceptions(detail.value) : [])
const contentTabLabel = computed(() => `${detail.value?.task_type === 'retry_failed_task' ? '处理内容' : '发现内容'}（${detailContents.value.length}）`)
const categories = ref([])
const strategyDrawerVisible = ref(false)
const strategyDrawer = ref(null)
// 筛选走列表接口（search/status），页面只对返回行做分页切片。
const filters = ref({ search: '', status: '' })
const pagination = ref({ current: 1, pageSize: 20 })

const isAdmin = computed(() => currentUser.value?.role === 'admin')
// 任务异常处置（重试）与策略管理同属高级运营／管理员职责。
const canManage = computed(() => ['admin', 'senior_operator'].includes(currentUser.value?.role))

// 每一列都必须声明 width，**不许只写 minWidth**。
//
// 实测（用户实机截图按表头左边界间距反推）：本页三列只有 minWidth 的
// 「任务名称」220→76、「任务来源」160→76.5、「发现结果」240→76.5，
// 而所有声明 width 的列逐列精确（ID 69.5 / 类目 110 / 触发方式 90 / 状态 100 /
// 执行时间 170 / 修改时间 170 / 修改人 110）。任务名称被压成「m3a…」就是这么来的。
// 机理与「Chrome 看不出问题」的原因见 ContentPoolPage.vue 里 columns 上方的完整说明。
const columns = computed(() => [
  { colKey: 'id', title: 'ID', width: 70 },
  { colKey: 'name', title: '任务名称', width: 260 },
  { colKey: 'strategy', title: '任务来源', width: 160 },
  { colKey: 'team', title: '运营团队', width: 120 },
  { colKey: 'category', title: '类目', width: 110 },
  { colKey: 'trigger', title: '触发方式', width: 90 },
  // 竖排后列宽只由最宽的那一项决定：实测最宽是「自动素材 9,999,999」，
  // 111px（标签 48 + gap 6 + 数值 57），加 td 左右内边距 16px = 127px。
  // 取 130px 留一点余量。计数不可能到千万级，这个上界已经很宽松。
  { colKey: 'found', title: '发现结果', width: 130 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'executed_at', title: '执行时间', width: 150 },
  { colKey: 'op', title: '操作', width: opColumnWidth(canManage.value ? ['详情', '重试'] : ['详情'], { variant: 'text', primary: true }), fixed: 'right' },
])

// 横向滚动宽度由列宽推导，不写死（写死的 '1200px' 与列宽合计无关，是个漂移源）。
const tableScroll = computed(() => ({
  x: `${columns.value.reduce((sum, col) => sum + (col.width || 0), 0)}px`,
}))

const resultColumns = [
  { colKey: 'content', title: '内容信息', width: 360 },
  { colKey: 'metrics', title: '互动数据', width: 150 },
  { colKey: 'processing_status', title: '处理结果', width: 220 },
  { colKey: 'op', title: '操作', width: opColumnWidth(['查看素材']), fixed: 'right' },
]
const resultScroll = computed(() => ({ x: `${resultColumns.reduce((sum, col) => sum + col.width, 0)}px` }))

const pagedRows = computed(() => {
  const start = (pagination.value.current - 1) * pagination.value.pageSize
  return rows.value.slice(start, start + pagination.value.pageSize)
})

watch(filters, () => { pagination.value.current = 1 }, { deep: true })

onMounted(async () => {
  await loadContext()
  await load()
  const taskID = Number(route.query.task_id || 0)
  if (taskID > 0) {
    const task = rows.value.find((row) => row.id === taskID)
    if (task) await open(task)
  }
})

async function loadContext() {
  try {
    currentUser.value = await session.me()
    if (isAdmin.value) {
      const data = await users.listTeams()
      teams.value = Array.isArray(data) ? data : []
    }
  } catch {
    currentUser.value = null
  }
}

// quiet 由定时刷新传入：保留页码与筛选，失败沿用上一次读数。
async function load({ quiet = false } = {}) {
  if (!quiet) loading.value = true
  if (!quiet) error.value = ''
  try {
    const strategyID = Number(route.query.strategy_id || 0)
    // 从策略详情跳进来时锁定来源策略，其余筛选交给后端。
    const params = { strategy_id: strategyID > 0 ? strategyID : undefined, search: filters.value.search.trim(), status: filters.value.status, team_id: teamFilter.value || undefined }
    const data = await client.listTasks(params)
    rows.value = Array.isArray(data) ? data : []
    if (!quiet) pagination.value.current = 1
    const categoryRows = await client.listCategories({ scope: 'all' })
    categories.value = Array.isArray(categoryRows) ? categoryRows : []
  } catch (e) {
    if (!quiet) error.value = e.message || '任务加载失败'
  } finally {
    if (!quiet) loading.value = false
  }
}

useAutoRefresh(() => load({ quiet: true }), {
  shouldSkip: () => visible.value || strategyDrawerVisible.value || retrying.value,
})

function resetFilters() {
  teamFilter.value = ''
  filters.value = { search: '', status: '' }
  load()
}

async function open(row) {
  try {
    detail.value = await client.getTask(row.id)
    activeTab.value = 'info'
    contentPagination.value.current = 1
    visible.value = true
  } catch (e) {
    error.value = e.message || '任务详情加载失败'
  }
}

async function retry(row) {
  retrying.value = true
  try {
    await client.retryFailed(row.id)
    await load()
    feedback.toast.success('已重新执行，成功数据会自动跳过')
    visible.value = false
  } catch (e) {
    error.value = e.message || '重新执行失败'
  } finally {
    retrying.value = false
  }
}

function sourceType(row) {
  if (row.task_type === 'discovery_task') return '策略'
  if (row.task_type === 'retry_failed_task') return '重试'
  return row.snapshot?.operation === 'url' ? 'ID发现' : '关键词发现'
}

function sourceName(row) {
  if (row.task_type === 'discovery_task') return row.snapshot?.strategy_name || '策略'
  if (row.task_type === 'retry_failed_task') return row.snapshot?.strategy_name || '重试'
  return row.snapshot?.operation === 'url' ? 'ID发现' : '关键词发现'
}

function strategyName(row) {
  return sourceName(row)
}

function isStrategyTask(row) {
  return row.task_type === 'discovery_task' && Boolean(row.strategy_id)
}

function taskName(row) {
  return `${sourceName(row)}_${compactTime(row.created_at)}`
}

function categoryName(id) {
  const category = categories.value.find((item) => item.id === id)
  return category?.name || id || '-'
}

function teamName(id) {
  if (currentUser.value?.team_id != null && String(currentUser.value.team_id) === String(id)) return currentUser.value.team_name || '-'
  const team = teams.value.find((item) => String(item.id) === String(id))
  return team?.name || '-'
}

function changeTeamFilter() { load() }

// 任务名称里内嵌的时间戳，不带秒：20260923-143045 → 20260923-1430。
// 秒对「这条任务是什么时候跑的」没有信息量，却让任务名称长出一截。
function compactTime(value) {
  if (!value) return '--------'
  const d = new Date(value)
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}${pad(d.getMonth() + 1)}${pad(d.getDate())}-${pad(d.getHours())}${pad(d.getMinutes())}`
}

function triggerLabel(row) {
  if (row.task_type === 'retry_failed_task') return '重试'
  return row.schedule_key ? '定时' : '手动'
}

// 竖排一行一项，返回数组而不是拼好的字符串（渲染交给 MetricList）。
// 五项拼成一行时宽度不够会在任意位置折断，数字和标签被拆散。
function foundStats(row) {
  const s = row.stats || {}
  return [
    { label: '发现', value: s.found || 0 },
    { label: '新增', value: s.added || 0 },
    { label: '自动素材', value: s.auto_materialized || 0 },
    { label: '待审核', value: s.pending || 0 },
    { label: '失败', value: s.failed || 0 },
  ]
}

function openStrategy(row) {
  strategyDrawer.value = row.snapshot || null
  strategyDrawerVisible.value = true
}

function goStrategy() {
  const id = strategyDrawer.value?.strategy_id
  if (id) router.push(`/discovery-strategies?strategy_id=${id}`)
}

function statusLabel(status) {
  return ({ pending: '待执行', running: '执行中', success: '成功', partial_success: '部分成功', failed: '失败' })[status] || status || '未知'
}

function statusTone(status) {
  return status === 'success' ? 'success' : status === 'failed' ? 'danger' : status === 'running' ? 'info' : 'warning'
}

function processingLabel(value) {
  return ({ unprocessed: '未入池', pending: '待审核', auto_materialized: '已转素材', material_failed: '转素材失败', duplicate: '已在内容池', failed: '失败' })[value] || value || '未知'
}

function processingTone(value) {
  return ({ pending: 'warning', auto_materialized: 'success', material_failed: 'danger', duplicate: 'neutral', failed: 'danger' })[value] || 'info'
}

function rowAction(item) {
  const id = encodeURIComponent(item.platform_content_id || '')
  if (item.processing_status === 'auto_materialized') {
    return { label: '查看素材', to: item.material_id ? `/material-library?material_id=${item.material_id}` : '/material-library' }
  }
  if (item.processing_status === 'pending') {
    return { label: '去审核', to: `/content-pool?crawl_task_id=${detail.value?.id}${id ? `&search=${id}` : ''}` }
  }
  if (item.processing_status === 'duplicate') {
    return { label: '查看内容', to: id ? `/content-pool?search=${id}` : `/content-pool?crawl_task_id=${detail.value?.id}` }
  }
  return null
}

function truncateTitle(title, max = 80) {
  const text = String(title || '')
  return text.length > max ? `${text.slice(0, max)}…` : text
}

function hasFailed(task) {
  return task.status === 'failed' || task.status === 'partial_success'
}
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true">
    <div class="wt-resource-page resource-visual-approved task-page">
      <FeedbackAlert v-if="error" theme="error" :message="error" closable style="margin-bottom:16px" @close="error=''" />
      <ResourcePageHeader title="挖掘任务" description="查看发现结果和执行记录，处理失败任务。" />

      <ResourceCard class="task-card">
        <ResourceFilterBar>
          <label class="wt-filter-field wt-filter-field--wide"><span class="wt-filter-field__label">综合搜索</span><t-input v-model="filters.search" clearable placeholder="搜索任务 ID / 任务名称 / 来源策略" @enter="load" /></label>
          <label v-if="isAdmin" class="wt-filter-field"><span class="wt-filter-field__label">运营团队</span><BusinessSelect kind="team" filter v-model="teamFilter" clearable @change="changeTeamFilter"><t-option v-for="team in teams" :key="team.id" :value="team.id" :label="team.name" /></BusinessSelect></label>
          <label class="wt-filter-field"><span class="wt-filter-field__label">状态</span><t-select v-model="filters.status" clearable placeholder="全部">
            <t-option value="success" label="成功" />
            <t-option value="partial_success" label="部分成功" />
            <t-option value="failed" label="失败" />
            <t-option value="running" label="执行中" />
            <t-option value="pending" label="待执行" />
          </t-select></label>
          <div class="wt-filter-actions"><t-button theme="primary" @click="load">查询</t-button><t-button class="wt-secondary-button" variant="outline" @click="resetFilters">重置</t-button></div>
        </ResourceFilterBar>
        <div class="table-scroll-wrap">
          <t-table class="wt-resource-table" :data="pagedRows" :columns="columns" row-key="id" hover :scroll="tableScroll" empty="暂无挖掘任务">
            <template #name="{ row }">
              <span class="task-name" :title="taskName(row)">{{ taskName(row) }}</span>
            </template>
            <template #strategy="{ row }">
              <t-link v-if="isStrategyTask(row)" theme="primary" @click="openStrategy(row)">{{ sourceName(row) }}</t-link>
              <span v-else>{{ sourceName(row) }}</span>
            </template>
            <template #team="{ row }">{{ teamName(row.team_id) }}</template>
            <template #category="{ row }">{{ categoryName(row.snapshot?.category_id) }}</template>
            <template #trigger="{ row }">{{ triggerLabel(row) }}</template>
            <template #found="{ row }"><MetricList :items="foundStats(row)" /></template>
            <template #status="{ row }"><ResourceStatusBadge :tone="statusTone(row.status)" :label="statusLabel(row.status)" /></template>
            <template #executed_at="{ row }">{{ formatDateTime(row.created_at) }}</template>
            <template #op="{ row }"><ResourceRowActions><t-button size="small" variant="text" @click="open(row)">详情</t-button><t-button v-if="canManage && hasFailed(row)" size="small" variant="text" class="is-primary" :loading="retrying" @click="retry(row)">重试</t-button></ResourceRowActions></template>
          </t-table>
        </div>
        <div class="pagination-bar"><t-pagination v-model:current="pagination.current" v-model:pageSize="pagination.pageSize" :total="rows.length" :page-size-options="[10, 20, 50]" /></div>
      </ResourceCard>

      <ResourceOverlay v-model:visible="visible" title="挖掘任务详情" kind="drawer" :width="960" :footer="false">
        <div v-if="detail" class="mining-detail">
          <t-tabs v-model="activeTab" class="mining-task-tabs task-detail-tabs">
            <t-tab-panel value="info" label="任务信息" :destroy-on-hide="true">
              <ResourceSection title="基本信息"><ResourceDescriptions>
                <t-descriptions-item label="任务名称" full>{{ taskName(detail) }}</t-descriptions-item>
                <t-descriptions-item label="任务 ID">{{ detail.id }}</t-descriptions-item><t-descriptions-item label="状态"><ResourceStatusBadge :tone="statusTone(detail.status)" :label="statusLabel(detail.status)" /></t-descriptions-item>
                <t-descriptions-item label="类目">{{ categoryName(detail.snapshot?.category_id) }}</t-descriptions-item><t-descriptions-item label="运营团队">{{ teamName(detail.team_id) }}</t-descriptions-item>
                <t-descriptions-item label="任务来源"><t-link v-if="isStrategyTask(detail)" theme="primary" @click="openStrategy(detail)">{{ sourceName(detail) }}</t-link><span v-else>{{ sourceName(detail) }}</span></t-descriptions-item><t-descriptions-item label="触发方式">{{ triggerLabel(detail) }}</t-descriptions-item>
                <t-descriptions-item label="发起人" full>{{ triggerLabel(detail) === '定时' ? '系统' : detail.created_by_name || '—' }}</t-descriptions-item>
              </ResourceDescriptions></ResourceSection>
              <ResourceSection title="执行结果">
                <p class="task-result-summary">{{ taskResultSummary(detail) }}</p>
                <ResourceNotice v-if="detailExceptions.length" tone="warning">
                  {{ detailExceptions[0].reason }}

                </ResourceNotice>
                <details :key="detail.id" class="task-process">
                  <summary><span class="task-process-open">查看过程</span><span class="task-process-close">收起过程</span></summary>
                  <div class="task-process-body">
                    <t-timeline>
                      <t-timeline-item v-for="(event, index) in taskExecutionEvents(detail)" :key="index"><div>{{ event.label }}</div><time class="task-event-time">{{ formatDateTime(event.time) }}</time></t-timeline-item>
                    </t-timeline>
                    <div v-if="detailExceptions.length" class="task-exceptions">
                      <div v-for="(item, index) in detailExceptions" :key="index" class="task-exception">
                        <strong>{{ item.stage }}</strong>
                        <ResourceDescriptions><t-descriptions-item label="处理对象" full>{{ item.scope }}</t-descriptions-item><t-descriptions-item label="失败原因" full>{{ item.reason }}</t-descriptions-item></ResourceDescriptions>
                      </div>
                    </div>
                  </div>
                </details>
              </ResourceSection>
            </t-tab-panel>

            <t-tab-panel value="items" :label="contentTabLabel" :destroy-on-hide="true">
              <t-table class="wt-resource-table" :data="pagedContents" :columns="resultColumns" row-key="platform_content_id" hover :scroll="resultScroll" empty="暂无内容">
                <template #content="{ row }">
                  <div class="task-content-cell">
                    <img v-if="row.cover_url && !failedCovers.has(row.cover_url)" @error="failedCovers.add(row.cover_url)" class="result-cover" :src="row.cover_url" alt="" loading="lazy" referrerpolicy="no-referrer" />
                    <div class="task-content-text">
                      <span class="result-title" :title="row.title || row.platform_content_id">{{ truncateTitle(row.title || row.platform_content_id) }}</span>
                      <span class="task-content-author">{{ row.author_name || '—' }}</span>
                    </div>
                  </div>
                </template>
                <template #metrics="{ row }"><MetricList :items="[{ label: '点赞', value: Number(row.like_count || 0).toLocaleString() }, { label: '收藏', value: Number(row.favorite_count || 0).toLocaleString() }]" /></template>
                <template #processing_status="{ row }">
                  <ResourceStatusBadge :tone="processingTone(row.processing_status)" :label="processingLabel(row.processing_status)" />
                  <span v-if="row.failure_reason" class="task-content-error">{{ row.failure_reason }}</span>
                </template>
                <template #op="{ row }">
                  <ResourceRowActions><t-button v-if="rowAction(row)" size="small" variant="text" @click="router.push(rowAction(row).to)">{{ rowAction(row).label }}</t-button><span v-else>—</span></ResourceRowActions>
                </template>
              </t-table>
              <div v-if="detailContents.length > 10" class="pagination-bar"><t-pagination v-model:current="contentPagination.current" v-model:pageSize="contentPagination.pageSize" :total="detailContents.length" :page-size-options="[10, 20, 50]" /></div>
            </t-tab-panel>

          </t-tabs>
        </div>
      </ResourceOverlay>
      <ResourceOverlay v-model:visible="strategyDrawerVisible" title="执行时策略配置" kind="drawer" :width="800">
        <template v-if="strategyDrawer">
          <ResourceNotice>本次任务创建时保存的配置。</ResourceNotice>
          <ResourceSection title="策略信息"><ResourceDescriptions>
            <t-descriptions-item label="策略名称" full>{{ strategyDrawer.strategy_name || '—' }}</t-descriptions-item>
            <t-descriptions-item label="类目">{{ categoryName(strategyDrawer.category_id) }}</t-descriptions-item><t-descriptions-item label="平台">{{ platformLabel(strategyDrawer.platform) }}</t-descriptions-item>
            <t-descriptions-item label="类型">{{ strategyDrawer.strategy_type === 'author' ? '作者' : '关键词' }}</t-descriptions-item><t-descriptions-item label="执行周期">{{ scheduleLabel(strategyDrawer.schedule) }}</t-descriptions-item>
            <t-descriptions-item label="关键词 / 作者" full>{{ (strategyDrawer.keywords || []).join('、') || strategyDrawer.author || '—' }}</t-descriptions-item><t-descriptions-item label="转素材规则" full>{{ materialRulesSummary(strategyDrawer) }}</t-descriptions-item>
          </ResourceDescriptions></ResourceSection>
        </template>
        <template #footer><t-button v-if="strategyDrawer?.strategy_id" theme="primary" @click="goStrategy">查看策略</t-button></template>
      </ResourceOverlay>
    </div>
  </t-loading>
</template>

<style scoped>
.task-name, .result-title { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; display:block; }
.result-cover { width:40px; height:48px; flex:0 0 40px; object-fit:cover; border-radius:4px; }
.task-detail-tabs { margin-top:0; }
.task-result-summary { margin:0; line-height:24px; color:var(--wt-text-primary); }
.task-process { margin-top:12px; }
.task-process > summary { width:fit-content; color:var(--wt-primary); cursor:pointer; line-height:24px; border-radius:4px; }
.task-process > summary:focus-visible { outline:2px solid var(--wt-primary); outline-offset:3px; }
.task-process-close, .task-process[open] .task-process-open { display:none; }
.task-process[open] .task-process-close { display:inline; }
.task-process-body { padding-top:20px; }
.task-exceptions { padding-top:16px; border-top:1px solid var(--wt-border); }
.task-content-cell { display:flex; align-items:center; gap:12px; }
.task-content-text { min-width:0; }
.task-content-author { display:block; margin-top:4px; font-size:12px; color:var(--wt-text-tertiary); }
.task-event-time { font-size:12px; color:var(--wt-text-tertiary); }
.task-content-error { display:block; margin-top:6px; font-size:12px; line-height:20px; color:var(--wt-danger); overflow-wrap:anywhere; }
.task-exception + .task-exception { margin-top:16px; padding-top:16px; border-top:1px solid var(--wt-border); }
.task-exception > strong { color:var(--wt-danger); font-size:13px; font-weight:500; }
</style>
