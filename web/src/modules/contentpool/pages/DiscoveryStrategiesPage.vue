<script setup>
import '../mining.css'
import { opColumnWidth } from '../../../shared/ui/resource/opColumnWidth.js'
import ResourceFilterBar from '../../../shared/ui/resource/ResourceFilterBar.vue'
import ResourceRowActions from '../../../shared/ui/resource/ResourceRowActions.vue'
import ResourceOverlay from '../../../shared/ui/resource/ResourceOverlay.vue'
import ResourceForm from '../../../shared/ui/resource/ResourceForm.vue'
import ResourceSection from '../../../shared/ui/resource/ResourceSection.vue'
import ResourceDescriptions from '../../../shared/ui/resource/ResourceDescriptions.vue'
import ResourceFieldHelp from '../../../shared/ui/resource/ResourceFieldHelp.vue'
import ResourceNotice from '../../../shared/ui/resource/ResourceNotice.vue'
import ResourceReorderActions from '../../../shared/ui/resource/ResourceReorderActions.vue'
import BusinessSelect from "../../../shared/ui/fields/BusinessSelect.vue"
import { feedback } from '../../../shared/ui/feedback/index.js'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { DialogPlugin } from 'tdesign-vue-next'
import { createDiscoveryClient } from '../../../shared/api/discovery.js'
import { createSessionClient } from '../../../shared/api/session.js'
import { createUsersClient } from '../../../apps/cloud/pages/users/usersApi.js'
import ResourceCard from '../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../shared/ui/resource/ResourcePageHeader.vue'
import ResourceStatGrid from '../../../shared/ui/resource/ResourceStatGrid.vue'
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'
import MetricList from '../../../shared/ui/resource/MetricList.vue'
import KeywordTags from '../../../shared/ui/resource/KeywordTags.vue'
import { formatDateTime } from '../../../shared/utils/datetime.js'
import { enabledCategories } from '../../../shared/utils/categories.js'
import { MINING_PLATFORM_OPTIONS, platformLabel } from '../../../shared/utils/platforms.js'
import { useAutoRefresh } from '../../../shared/utils/useAutoRefresh.js'
import { defaultCondition, displayRules, materialRulesSummary, resolveMaterialRules, saveRules } from '../miningRules.js'
import { scheduleLabel } from '../labels.js'

const client = createDiscoveryClient()
const session = createSessionClient()
const users = createUsersClient()
const route = useRoute()
const rows = ref([])
const tasks = ref([])
const categories = ref([])
const teams = ref([])
const currentUser = ref(null)
const teamFilter = ref('')
const loading = ref(false)
const error = ref('')
const visible = ref(false)
const saving = ref(false)
const editingID = ref(null)
const form = ref(defaultForm())
const scheduleMode = ref('manual')
const scheduleTime = ref('09:00')
const detailVisible = ref(false)
const detailRow = ref(null)
const keywordVisible = ref(false)
const authorVisible = ref(false)
const keywordTarget = ref(null)
const authorTarget = ref(null)

// 筛选走列表接口（search/type/status），页面只对返回行做分页切片。
const filters = ref({ search: '', type: '', status: '' })
const pagination = ref({ current: 1, pageSize: 20 })

const isAdmin = computed(() => currentUser.value?.role === 'admin')
// 普通运营可看策略与任务，写操作和异常处置属于高级运营／管理员职责。
const canManage = computed(() => ['admin', 'senior_operator'].includes(currentUser.value?.role))

const conditionFields = [
  { label: '发布时间', value: 'published_at' },
  { label: '点赞量', value: 'like_count' },
  { label: '收藏量', value: 'favorite_count' },
]

function conditionOperators(field) {
  return field === 'published_at'
    ? [{ label: '不早于', value: 'gte' }, { label: '早于', value: 'lt' }, { label: '近 N 小时', value: 'recent_hours' }, { label: '近 N 天', value: 'recent_days' }]
    : [{ label: '等于', value: 'eq' }, { label: '大于等于', value: 'gte' }, { label: '小于等于', value: 'lte' }]
}

function renumberRules() { form.value.config.material_rules.forEach((rule, index) => { rule.priority = index + 1 }) }
function addRule() { form.value.config.material_rules.push({ priority: form.value.config.material_rules.length + 1, action: 'match', logic: 'all', conditions: [defaultCondition()] }) }
function removeRule(index) { form.value.config.material_rules.splice(index, 1); renumberRules() }
function moveRule(index, direction) {
  const target = index + direction
  if (target < 0 || target >= form.value.config.material_rules.length) return
  const [rule] = form.value.config.material_rules.splice(index, 1)
  form.value.config.material_rules.splice(target, 0, rule)
  renumberRules()
}
function changeConditionField(condition) {
  condition.operator = condition.field === 'published_at' ? 'recent_days' : 'gte'
  condition.value = condition.field === 'published_at' ? 1 : 0
}
function changeConditionOperator(condition) { condition.value = ['gte', 'lt'].includes(condition.operator) && condition.field === 'published_at' ? null : 1 }

// 每一列都必须声明 width，**不许只写 minWidth**。
//
// 实测：TDesign 内层 <table> 没有宽度，表格宽度完全由列声明推导（取 col.width ?? col.minWidth），
// 而 minWidth 在 WebKit 的 fixed 布局下不产生确定列宽 —— 这类列会变成「剩余空间列」，
// 可用宽度不足时被压到远低于声明值（内容池实测：声明 340 被压成 91.5），
// 而声明 width 的列逐列精确。Chrome 认 minWidth，所以那边的模拟台看不出问题。
// 详见 ContentPoolPage.vue 里 columns 上方的完整说明。
const columns = computed(() => [
  { colKey: 'id', title: 'ID', width: 70 },
  { colKey: 'name', title: '策略名称', width: 240 },
  { colKey: 'team', title: '运营团队', width: 120 },
  { colKey: 'category', title: '类目', width: 120 },
  { colKey: 'type', title: '类型', width: 90 },
  { colKey: 'rule', title: '挖掘规则', width: 130 },
  { colKey: 'schedule', title: '执行周期', width: 130 },
  { colKey: 'material', title: '转素材规则', width: 170 },
  // 竖排后列宽只由最宽的那一项决定：实测最宽是「待审核 9,999,999」，
  // 99px（标签 36 + gap 6 + 数值 57），加 td 左右内边距 16px = 115px。
  // 取 120px 留一点余量。
  { colKey: 'latest', title: '最近效果', width: 120 },
  // 状态排在关联实体/时间之后（AdminUI 规范 §4.1：主体 → 属性 → 关联实体 → 状态/结果 → 时间）。
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'updated_at', title: '修改时间', width: 150 },
  { colKey: 'op', title: '操作', width: opColumnWidth(canManage.value ? ['详情', '执行', '任务', '编辑', '更多'] : ['详情', '任务'], { variant: 'text', primary: true }), fixed: 'right' },
])

// 横向滚动宽度由列宽推导，不写死（写死的 '1260px' 与列宽合计无关，是个漂移源）。
const tableScroll = computed(() => ({
  x: `${columns.value.reduce((sum, col) => sum + (col.width || 0), 0)}px`,
}))

const latestTasks = computed(() => {
  const grouped = new Map()
  for (const task of tasks.value) {
    if (!task.strategy_id || grouped.has(task.strategy_id)) continue
    grouped.set(task.strategy_id, task)
  }
  return grouped
})

const runningSet = computed(() => new Set(
  tasks.value.filter((task) => task.strategy_id && ['pending', 'running'].includes(task.status)).map((task) => task.strategy_id)
))

const pagedRows = computed(() => {
  const start = (pagination.value.current - 1) * pagination.value.pageSize
  return rows.value.slice(start, start + pagination.value.pageSize)
})

watch(filters, () => { pagination.value.current = 1 }, { deep: true })

// 只保留列表查询拿不到的动作统计：总数与启用数等于分页总数和状态筛选结果，不再重复展示（§3.3）。
const stats = computed(() => {
  const today = new Date().toDateString()
  const todayRuns = tasks.value.filter((task) => new Date(task.created_at).toDateString() === today).length
  const auto = tasks.value.reduce((sum, task) => sum + (Number(task.stats?.auto_materialized) || 0), 0)
  return [
    { key: 'today', label: '今日执行', value: todayRuns, tone: 'info' },
    { key: 'auto', label: '自动素材', value: auto, tone: 'info' },
  ]
})

onMounted(async () => {
  await loadContext()
  await load()
  const id = Number(route.query.strategy_id || 0)
  if (id > 0) {
    const row = rows.value.find((item) => item.id === id)
    if (row) openDetail(row)
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

function defaultForm() {
  return {
    name: '',
    team_id: '',
    category_id: '',
    strategy_type: 'keyword',
    platform: 'douyin',
    config: {
      keywords: [],
      authors: [],
      author: '',
      // 空规则组 = 只采集、不自动转素材；规则组形状与合成策略 priority_rules 一致。
      material_rules: [],
    },
    schedule: 'manual',
    timezone: 'Asia/Shanghai',
    status: 'disabled',
  }
}

// quiet 由定时刷新传入：保留当前页码与筛选，失败沿用上一次读数。
async function load({ quiet = false } = {}) {
  if (!quiet) loading.value = true
  if (!quiet) error.value = ''
  try {
    const query = { search: String(filters.value.search || '').trim(), type: filters.value.type, status: filters.value.status, teamId: teamFilter.value || undefined }
    const [strategyRows, taskRows, categoryRows] = await Promise.all([client.listStrategies(query), client.listTasks({ team_id: teamFilter.value || undefined }), client.listCategories({ scope: 'all' })])
    rows.value = Array.isArray(strategyRows) ? strategyRows : []
    if (!quiet) pagination.value.current = 1
    tasks.value = Array.isArray(taskRows) ? taskRows : []
    categories.value = Array.isArray(categoryRows) ? categoryRows : []
  } catch (e) {
    if (!quiet) error.value = e.message || '策略加载失败'
  } finally {
    if (!quiet) loading.value = false
  }
}

useAutoRefresh(() => load({ quiet: true }), {
  shouldSkip: () => visible.value || detailVisible.value || keywordVisible.value || authorVisible.value || saving.value,
})

function resetFilters() {
  teamFilter.value = ''
  filters.value = { search: '', type: '', status: '' }
  load()
}

function openCreate() {
  editingID.value = null
  form.value = defaultForm()
  scheduleMode.value = 'manual'
  scheduleTime.value = '09:00'
  visible.value = true
}

// 历史配置把转素材设置摊在 config 顶层；编辑器只认 material_rules，
// 打开时翻译一次，保存后旧键自然消失（后端 NormalizeMaterialConfig 也会清理）。
const LEGACY_MATERIAL_KEYS = ['auto_material', 'material_rule', 'like_threshold', 'favorite_threshold']

function withoutLegacyMaterialKeys(config) {
  const next = { ...config }
  for (const key of LEGACY_MATERIAL_KEYS) delete next[key]
  return next
}

function fillForm(row) {
  const config = row.config || {}
  form.value = {
    name: row.name,
    team_id: row.team_id || '',
    category_id: row.category_id || '',
    strategy_type: row.strategy_type,
    platform: row.platform,
    config: {
      ...withoutLegacyMaterialKeys(config),
      material_rules: displayRules(resolveMaterialRules(config)),
    },
    schedule: row.schedule || 'manual',
    timezone: row.timezone || 'Asia/Shanghai',
    status: row.status || 'disabled',
  }
  if (!Array.isArray(form.value.config.keywords)) form.value.config.keywords = []
  if (!Array.isArray(form.value.config.authors)) form.value.config.authors = []
  const schedule = form.value.schedule || 'manual'
  if (schedule === 'manual') {
    scheduleMode.value = 'manual'
  } else if (String(schedule).startsWith('daily ')) {
    scheduleMode.value = 'scheduled'
    scheduleTime.value = String(schedule).slice(6)
  } else {
    scheduleMode.value = 'scheduled'
    scheduleTime.value = '09:00'
  }
}

function openEdit(row) {
  editingID.value = row.id
  fillForm(row)
  visible.value = true
}

function openDetail(row) {
  detailRow.value = row
  detailVisible.value = true
}

function openCopy(row) {
  editingID.value = null
  fillForm(row)
  form.value.name = `${row.name} 副本`
  visible.value = true
}

let editorBaseline = ''
function editorDraft() { return JSON.stringify([form.value, scheduleMode.value, scheduleTime.value]) }
watch(visible, opened => { if (opened) editorBaseline = editorDraft() })
function requestCloseEditor() {
  if (saving.value) return
  if (editorDraft() === editorBaseline) { visible.value = false; return }
  const dialog = DialogPlugin.confirm({ header: '放弃未保存的修改？', body: '关闭后，修改不会保留。', confirmBtn: { content: '不保存', theme: 'danger' }, onConfirm: () => { dialog.hide(); visible.value = false } })
}
function runDisabledReason(row) {
  if (row.strategy_type === 'author') return '作者挖掘暂不可用'
  if (isRunning(row)) return '已有任务执行中'
  if (row.status !== 'enabled') return '请先启用策略'
  return ''
}

// 空规则组合法（只采集）；非空时必须至少有一个命中组，否则内容永远不会转素材。
function materialRulesError(rules) {
  if (!rules.length) return ''
  if (!rules.some((rule) => rule.action === 'match')) return '至少需要一个「命中」规则组，否则内容不会自动转素材'
  for (const rule of rules) {
    if (!Array.isArray(rule.conditions) || !rule.conditions.length) return `优先级 ${rule.priority} 至少需要一个条件`
    for (const condition of rule.conditions) {
      if (condition.field === 'published_at' && ['gte', 'lt'].includes(condition.operator)) {
        if (!condition.value || !Number.isFinite(Number(condition.value))) return `优先级 ${rule.priority} 的发布时间未填写完整`
        continue
      }
      const value = Number(condition.value)
      if (!Number.isInteger(value) || value < 0) return `优先级 ${rule.priority} 的条件值必须是非负整数`
      if (condition.field === 'published_at' && value < 1) return `优先级 ${rule.priority} 的时间范围至少为 1`
      if (condition.field === 'published_at' && condition.operator === 'recent_hours' && value > 8760) return `优先级 ${rule.priority} 的小时数不能超过 8760`
      if (condition.field === 'published_at' && condition.operator === 'recent_days' && value > 365) return `优先级 ${rule.priority} 的天数不能超过 365`
    }
  }
  return ''
}

async function save() {
  if (!String(form.value.name || '').trim()) {
    feedback.toast.error('请输入策略名称')
    return
  }
  if (!editingID.value && isAdmin.value && !form.value.team_id) {
    feedback.toast.error('请选择目标团队')
    return
  }
  const materialRules = Array.isArray(form.value.config.material_rules) ? form.value.config.material_rules : []
  const rulesError = materialRulesError(materialRules)
  if (rulesError) {
    feedback.toast.error(rulesError)
    return
  }
  saving.value = true
  try {
    const config = {
      ...withoutLegacyMaterialKeys(form.value.config),
      material_rules: saveRules(materialRules),
    }
    if (!String(form.value.category_id || '').trim()) {
      feedback.toast.error('请选择所属类目')
      return
    }
    if (!String(form.value.platform || '').trim()) {
      feedback.toast.error('请选择平台')
      return
    }
    form.value.schedule = scheduleMode.value === 'manual' ? 'manual' : `daily ${scheduleTime.value}`
    config.keywords = Array.isArray(config.keywords) ? config.keywords.map((item) => String(item).trim()).filter(Boolean) : []
    config.authors = Array.isArray(config.authors) ? config.authors.map((item) => String(item).trim()).filter(Boolean) : []
    const payload = { ...form.value, config }
    // team_id 只用于管理员新建时指定目标团队；空串会被后端按整数解码失败，更新也不改团队。
    if (!payload.team_id || editingID.value) delete payload.team_id
    if (editingID.value) await client.updateStrategy(editingID.value, payload)
    else await client.createStrategy(payload)
    visible.value = false
    await load()
    feedback.toast.success(editingID.value ? '策略已更新' : '策略已保存')
  } catch (e) {
    error.value = e.message || '策略保存失败'
  } finally {
    saving.value = false
  }
}

async function toggle(row) {
  try {
    await client.setStrategyStatus(row.id, row.status === 'enabled' ? 'disabled' : 'enabled')
    await load()
  } catch (e) {
    error.value = e.message || '状态更新失败'
  }
}

async function run(row) {
  try {
    await client.runStrategy(row.id)
    feedback.toast.success('已创建挖掘任务')
    await load()
  } catch (e) {
    error.value = e.message || '任务创建失败'
  }
}

function remove(row) {
  const dialog = DialogPlugin.confirm({
    header: '删除策略',
    body: `确认删除策略「${row.name}」？删除后不可恢复。`,
    confirmBtn: { content: '删除', theme: 'danger' },
    onConfirm: async () => {
      try {
        await client.deleteStrategy(row.id)
        await load()
        feedback.toast.success('策略已删除')
      } catch (e) {
        error.value = e.message || '策略删除失败'
      }
      dialog.hide()
    },
  })
}

function isRunning(row) {
  return runningSet.value.has(row.id)
}

function categoryName(id) {
  const category = categories.value.find((item) => item.id === id)
  return category?.name || '-'
}

// 只列启用类目。编辑一条挂在停用类目上的历史策略时，把当前类目补成不可选选项：
// 不补，原值显示不出来，保存一次就把类目悄悄换掉了。
const formCategoryOptions = computed(() => {
  const options = enabledCategories(categories.value).map((category) => ({ id: category.id, label: category.name, disabled: false }))
  const current = categories.value.find((category) => category.id === form.value.category_id)
  if (current && !current.enabled) {
    options.unshift({ id: current.id, label: `${current.name}（已停用）`, disabled: true })
  }
  return options
})

function teamName(id) {
  if (currentUser.value?.team_id != null && String(currentUser.value.team_id) === String(id)) return currentUser.value.team_name || '-'
  const team = teams.value.find((item) => String(item.id) === String(id))
  return team?.name || '-'
}

function changeTeamFilter() { load() }

function typeLabel(type) {
  return type === 'author' ? '作者' : '关键词'
}

function keywordList(config) {
  const keywords = Array.isArray(config?.keywords) ? config.keywords : []
  if (keywords.length) return keywords
  return config?.keyword ? [config.keyword] : []
}

function keywordCount(config) {
  return keywordList(config).length
}

function authorCount(config) {
  return config?.author || config?.author_id ? 1 : 0
}

function ruleLabel(row) {
  const config = row.config || {}
  return row.strategy_type === 'author' ? `作者 ${authorCount(config)}个` : `关键词 ${keywordCount(config)}个`
}

function openKeywords(row) {
  keywordTarget.value = { name: row.name, keywords: keywordList(row.config || {}) }
  keywordVisible.value = true
}

function openAuthors(row) {
  const config = row.config || {}
  authorTarget.value = {
    name: row.name,
    authors: [{ name: config.author || config.author_id || '-', platform_id: config.author_id || '-', fans: '-' }],
  }
  authorVisible.value = true
}

function materialLabel(row) {
  return materialRulesSummary(row.config || {})
}

// 竖排一行一项，返回数组而不是拼好的字符串（渲染交给 MetricList）。
// 没有任务时返回空数组，模板据此显示占位符。
function latestStats(row) {
  const task = latestTasks.value.get(row.id)
  if (!task) return []
  const s = task.stats || {}
  return [
    { label: '发现', value: s.found || 0 },
    { label: '新增', value: s.added || 0 },
    { label: '自动', value: s.auto_materialized || 0 },
    { label: '待审核', value: s.pending || 0 },
  ]
}
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true">
    <div class="wt-resource-page resource-visual-approved strategy-page">
      <FeedbackAlert v-if="error" theme="error" :message="error" closable style="margin-bottom:16px" @close="error=''" />
      <ResourcePageHeader title="挖掘策略" description="设置挖掘对象、执行时间和转素材规则。">
        <template #actions>
          <t-button v-if="canManage" theme="primary" @click="openCreate">新增策略</t-button>
        </template>
      </ResourcePageHeader>

      <ResourceStatGrid :items="stats" />

      <ResourceCard class="strategy-card">
        <ResourceFilterBar>
          <label class="wt-filter-field wt-filter-field--wide"><span class="wt-filter-field__label">综合搜索</span><t-input v-model="filters.search" clearable placeholder="搜索策略 ID / 策略名称" @enter="load" /></label>
          <label v-if="isAdmin" class="wt-filter-field"><span class="wt-filter-field__label">运营团队</span><BusinessSelect kind="team" filter v-model="teamFilter" clearable @change="changeTeamFilter"><t-option v-for="team in teams" :key="team.id" :value="team.id" :label="team.name" /></BusinessSelect></label>
          <label class="wt-filter-field"><span class="wt-filter-field__label">策略类型</span><t-select v-model="filters.type" clearable placeholder="全部">
            <t-option value="keyword" label="关键词" />
            <t-option value="author" label="作者" />
          </t-select></label>
          <label class="wt-filter-field"><span class="wt-filter-field__label">状态</span><t-select v-model="filters.status" clearable placeholder="全部">
            <t-option value="enabled" label="启用" />
            <t-option value="disabled" label="停用" />
          </t-select></label>
          <div class="wt-filter-actions"><t-button theme="primary" @click="load">查询</t-button><t-button class="wt-secondary-button" variant="outline" @click="resetFilters">重置</t-button></div>
        </ResourceFilterBar>
        <div class="table-scroll-wrap">
          <t-table class="wt-resource-table" :data="pagedRows" :columns="columns" row-key="id" hover :scroll="tableScroll" empty="暂无挖掘策略">
            <template #name="{ row }"><span class="mining-ellipsis" :title="row.name">{{ row.name }}</span></template>
            <template #team="{ row }">{{ teamName(row.team_id) }}</template>
            <template #category="{ row }">{{ categoryName(row.category_id) }}</template>
            <template #type="{ row }">{{ typeLabel(row.strategy_type) }}</template>
            <template #rule="{ row }">
              <t-link theme="primary" class="strategy-rule-link" @click="row.strategy_type === 'author' ? openAuthors(row) : openKeywords(row)">{{ ruleLabel(row) }}</t-link>
            </template>
            <template #schedule="{ row }">{{ scheduleLabel(row.schedule) }}</template>
            <template #material="{ row }"><span class="material-summary" :title="materialLabel(row)">{{ materialLabel(row) }}</span></template>
            <template #latest="{ row }">
              <MetricList v-if="latestStats(row).length" :items="latestStats(row)" />
              <span v-else>-</span>
            </template>
            <template #updated_at="{ row }">{{ formatDateTime(row.updated_at) }}</template>
            <template #status="{ row }"><ResourceStatusBadge :tone="row.status === 'enabled' ? 'success' : 'neutral'" :label="row.status === 'enabled' ? '启用' : '停用'" /></template>
            <template #op="{ row }">
              <ResourceRowActions>
                <t-button size="small" variant="text" @click="openDetail(row)">详情</t-button>
                <t-tooltip v-if="canManage" :content="runDisabledReason(row)" :disabled="!runDisabledReason(row)"><span><t-button size="small" variant="text" class="is-primary" :disabled="Boolean(runDisabledReason(row))" @click="run(row)">执行</t-button></span></t-tooltip>
                <t-button size="small" variant="text" @click="$router.push(`/crawl-tasks?strategy_id=${row.id}`)">任务</t-button>
                <t-tooltip v-if="canManage" content="已有任务执行中" :disabled="!isRunning(row)"><span><t-button size="small" variant="text" :disabled="isRunning(row)" @click="openEdit(row)">编辑</t-button></span></t-tooltip>
                <t-dropdown v-if="canManage" trigger="click">
                  <t-button size="small" variant="text">更多</t-button>
                  <t-dropdown-menu><t-dropdown-item @click="openCopy(row)">复制</t-dropdown-item><t-dropdown-item :disabled="isRunning(row)" @click="remove(row)">删除</t-dropdown-item></t-dropdown-menu>
                </t-dropdown>
              </ResourceRowActions>
            </template>
          </t-table>
        </div>
        <div class="pagination-bar"><t-pagination v-model:current="pagination.current" v-model:pageSize="pagination.pageSize" :total="rows.length" :page-size-options="[10, 20, 50]" /></div>
      </ResourceCard>

      <ResourceOverlay :visible="visible" @update:visible="value => { if (!value) requestCloseEditor() }" :title="editingID ? '编辑挖掘策略' : '新增挖掘策略'" kind="drawer" :width="960" :busy="saving">
        <ResourceForm>
          <ResourceSection title="基本信息" variant="form">
            <div class="resource-form-grid">
              <t-form-item label="策略名称" required-mark class="resource-form-full"><t-input v-model="form.name" placeholder="输入策略名称" /></t-form-item>
              <t-form-item v-if="isAdmin && !editingID" label="运营团队" required-mark><BusinessSelect kind="team" v-model="form.team_id"><t-option v-for="team in teams" :key="team.id" :value="team.id" :label="team.name" /></BusinessSelect></t-form-item>
              <t-form-item v-else label="运营团队"><t-input :value="teamName(form.team_id || currentUser?.team_id)" disabled /></t-form-item>
              <t-form-item label="所属类目" required-mark><BusinessSelect kind="category" v-model="form.category_id" clearable><t-option v-for="category in formCategoryOptions" :key="category.id" :value="category.id" :label="category.label" :disabled="category.disabled" /></BusinessSelect></t-form-item>
              <t-form-item label="平台" required-mark><t-select v-model="form.platform"><t-option v-for="option in MINING_PLATFORM_OPTIONS" :key="option.value" :value="option.value" :label="option.label" /></t-select></t-form-item>
              <t-form-item label="策略类型"><t-radio-group v-model="form.strategy_type"><t-radio value="keyword">关键词</t-radio><t-radio value="author" disabled>作者（维护中）</t-radio></t-radio-group></t-form-item>
            </div>
          </ResourceSection>
          <ResourceSection title="挖掘对象" variant="form">
            <KeywordTags v-if="form.strategy_type === 'keyword'" v-model="form.config.keywords" placeholder="输入关键词后按回车添加" />
            <KeywordTags v-else v-model="form.config.authors" subject="作者" placeholder="输入作者昵称或 ID 后按回车添加" />
          </ResourceSection>
          <ResourceSection title="执行计划" variant="form">
            <div class="resource-form-grid">
              <t-form-item required-mark><template #label>执行方式<ResourceFieldHelp content="手动执行由列表发起；定时执行按配置时间自动运行。" /></template><t-radio-group v-model="scheduleMode"><t-radio value="manual">手动执行</t-radio><t-radio value="scheduled">定时执行</t-radio></t-radio-group></t-form-item>
              <t-form-item v-if="scheduleMode === 'scheduled'" label="执行周期" required-mark><div class="mining-time-field"><span>每天</span><t-time-picker v-model="scheduleTime" format="HH:mm" value-type="HH:mm" clearable /></div></t-form-item>
              <t-form-item label="策略状态"><t-switch v-model="form.status" :custom-value="['enabled', 'disabled']" :label="['启用', '停用']" /></t-form-item>
            </div>
          </ResourceSection>
          <ResourceSection title="转素材规则" variant="form">
            <template #title-extra><ResourceFieldHelp content="按优先级依次匹配，首条命中决定是否转素材。没有规则时，仅采集内容。" /></template>
            <ResourceNotice v-if="!form.config.material_rules.length">尚未配置规则，发现的内容需人工审核。</ResourceNotice>
            <div v-for="(rule, index) in form.config.material_rules" :key="rule.priority" class="mining-rule">
              <ResourceReorderActions :item-key="String(rule.priority)" scope="mining-material-rules" :index="index" :count="form.config.material_rules.length" :draggable="false" @move="moveRule(index, $event)" @remove="removeRule(index)">优先级 {{ rule.priority }}</ResourceReorderActions>
              <div class="resource-form-grid">
                <t-form-item label="规则类型" required-mark><t-radio-group v-model="rule.action"><t-radio value="match">命中</t-radio><t-radio value="filter">过滤</t-radio></t-radio-group></t-form-item>
                <t-form-item label="判断方式" required-mark><t-radio-group v-model="rule.logic"><t-radio value="all">全部满足</t-radio><t-radio value="any">任一满足</t-radio></t-radio-group></t-form-item>
              </div>
              <div class="mining-rule-conditions">
                <div v-for="(condition, conditionIndex) in rule.conditions" :key="conditionIndex" class="mining-rule-condition">
                  <label><span>指标</span><t-select v-model="condition.field" @change="changeConditionField(condition)"><t-option v-for="field in conditionFields" :key="field.value" :value="field.value" :label="field.label" /></t-select></label>
                  <label><span>比较</span><t-select v-model="condition.operator" :options="conditionOperators(condition.field)" @change="changeConditionOperator(condition)" /></label>
                  <label><span>数值</span><t-date-picker v-if="condition.field === 'published_at' && ['gte', 'lt'].includes(condition.operator)" v-model="condition.value" enable-time-picker value-type="time-stamp" format="YYYY-MM-DD HH:mm:ss" clearable /><t-input-number v-else v-model="condition.value" :min="condition.operator === 'recent_hours' || condition.operator === 'recent_days' ? 1 : 0" :max="condition.operator === 'recent_hours' ? 8760 : condition.operator === 'recent_days' ? 365 : undefined" theme="column" /></label>
                  <t-button size="small" theme="danger" variant="text" :disabled="rule.conditions.length === 1" @click="rule.conditions.splice(conditionIndex, 1)">移除</t-button>
                </div>
                <t-button size="small" variant="text" style="justify-self:start" @click="rule.conditions.push(defaultCondition())">添加条件</t-button>
              </div>
            </div>
            <t-button variant="outline" @click="addRule">添加优先级</t-button>
          </ResourceSection>
        </ResourceForm>
        <template #footer><t-button variant="outline" :disabled="saving" @click="requestCloseEditor">取消</t-button><t-button theme="primary" :loading="saving" @click="save">保存</t-button></template>
      </ResourceOverlay>

      <ResourceOverlay v-model:visible="keywordVisible" title="关键词配置" :width="640" :footer="false">
        <template v-if="keywordTarget"><ResourceDescriptions><t-descriptions-item label="策略名称" full>{{ keywordTarget.name }}</t-descriptions-item><t-descriptions-item label="关键词数量">{{ keywordTarget.keywords.length }}</t-descriptions-item></ResourceDescriptions><div class="mining-tags"><t-tag v-for="keyword in keywordTarget.keywords" :key="keyword" theme="primary" variant="light">{{ keyword }}</t-tag><ResourceNotice v-if="!keywordTarget.keywords.length">暂无关键词</ResourceNotice></div></template>
      </ResourceOverlay>
      <ResourceOverlay v-model:visible="authorVisible" title="作者配置" :width="640" :footer="false">
        <template v-if="authorTarget"><ResourceDescriptions><t-descriptions-item label="策略名称" full>{{ authorTarget.name }}</t-descriptions-item><t-descriptions-item label="作者数量">{{ authorTarget.authors.length }}</t-descriptions-item></ResourceDescriptions><ResourceDescriptions v-for="(author, index) in authorTarget.authors" :key="index"><t-descriptions-item label="作者">{{ author.name }}</t-descriptions-item><t-descriptions-item label="平台 ID">{{ author.platform_id }}</t-descriptions-item><t-descriptions-item label="粉丝量">{{ author.fans }}</t-descriptions-item></ResourceDescriptions></template>
      </ResourceOverlay>
      <ResourceOverlay v-model:visible="detailVisible" title="策略详情" kind="drawer" :width="800" :footer="false">
        <div v-if="detailRow" class="mining-detail">
          <ResourceSection title="基本信息"><ResourceDescriptions>
            <t-descriptions-item label="策略名称" full>{{ detailRow.name }}</t-descriptions-item>
            <t-descriptions-item label="策略 ID">{{ detailRow.id }}</t-descriptions-item><t-descriptions-item label="状态"><ResourceStatusBadge :tone="detailRow.status === 'enabled' ? 'success' : 'neutral'" :label="detailRow.status === 'enabled' ? '启用' : '停用'" /></t-descriptions-item>
            <t-descriptions-item label="所属类目">{{ categoryName(detailRow.category_id) }}</t-descriptions-item><t-descriptions-item label="平台">{{ platformLabel(detailRow.platform) }}</t-descriptions-item><t-descriptions-item label="运营团队">{{ teamName(detailRow.team_id) }}</t-descriptions-item>
          </ResourceDescriptions></ResourceSection>
          <ResourceSection title="挖掘配置"><ResourceDescriptions><t-descriptions-item label="类型">{{ typeLabel(detailRow.strategy_type) }}</t-descriptions-item><t-descriptions-item :label="detailRow.strategy_type === 'author' ? '作者' : '关键词'" full>{{ detailRow.strategy_type === 'author' ? detailRow.config?.author || '—' : keywordList(detailRow.config).join('、') || '—' }}</t-descriptions-item></ResourceDescriptions></ResourceSection>
          <ResourceSection title="执行配置"><ResourceDescriptions><t-descriptions-item label="执行周期">{{ scheduleLabel(detailRow.schedule) }}</t-descriptions-item></ResourceDescriptions></ResourceSection>
          <ResourceSection title="转素材规则"><ResourceNotice v-if="!resolveMaterialRules(detailRow.config).length">不自动转素材</ResourceNotice><ol v-else class="mining-readonly-rules"><li v-for="rule in [...resolveMaterialRules(detailRow.config)].sort((a,b) => a.priority-b.priority)" :key="rule.priority"><strong>优先级 {{ rule.priority }}</strong><p>{{ materialRulesSummary({ material_rules: [rule] }) }}</p></li></ol></ResourceSection>
          <ResourceSection title="操作记录"><ResourceDescriptions><t-descriptions-item label="创建时间">{{ formatDateTime(detailRow.created_at) }}</t-descriptions-item><t-descriptions-item label="创建人">{{ detailRow.created_by_name || '—' }}</t-descriptions-item><t-descriptions-item label="修改时间">{{ formatDateTime(detailRow.updated_at) }}</t-descriptions-item><t-descriptions-item label="修改人">{{ detailRow.updated_by_name || '—' }}</t-descriptions-item></ResourceDescriptions></ResourceSection>
        </div>
      </ResourceOverlay>
    </div>
  </t-loading>
</template>

<style scoped>
.material-summary { display:block; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.mining-time-field { display:flex; gap:12px; align-items:center; }
.mining-time-field :deep(.t-time-picker) { flex:1; min-width:0; }
.mining-readonly-rules { list-style:none; margin:0; padding:0; }
.mining-readonly-rules li + li { margin-top:16px; padding-top:16px; border-top:1px solid var(--wt-border); }
.mining-readonly-rules strong { font-size:13px; font-weight:500; }
.mining-readonly-rules p { margin:8px 0 0; font-size:14px; line-height:1.7; }
</style>
