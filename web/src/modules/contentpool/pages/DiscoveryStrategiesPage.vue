<script setup>
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
import { GAME_STATUS_ENABLED, enabledGames } from '../../../shared/utils/games.js'
import { defaultCondition, displayRules, materialRulesSummary, resolveMaterialRules, saveRules } from '../miningRules.js'
import { scheduleLabel } from '../labels.js'

const client = createDiscoveryClient()
const session = createSessionClient()
const users = createUsersClient()
const route = useRoute()
const rows = ref([])
const tasks = ref([])
const games = ref([])
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
const columns = [
  { colKey: 'id', title: 'ID', width: 70 },
  { colKey: 'name', title: '策略名称', width: 180 },
  { colKey: 'team', title: '运营团队', width: 120 },
  { colKey: 'game', title: '游戏', width: 120 },
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
  { colKey: 'updated_by_name', title: '修改人', width: 110 },
  { colKey: 'op', title: '操作', width: 260, fixed: 'right' },
]

// 横向滚动宽度由列宽推导，不写死（写死的 '1260px' 与列宽合计无关，是个漂移源）。
const tableScroll = computed(() => ({
  x: `${columns.reduce((sum, col) => sum + (col.width || 0), 0)}px`,
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
    { key: 'today', label: '今日执行', value: todayRuns, tone: 'warning' },
    { key: 'auto', label: '自动素材', value: auto, tone: 'info' },
  ]
})

onMounted(async () => {
  await loadContext()
  await load()
  const id = Number(route.query.strategy_id || 0)
  if (id > 0) {
    const row = rows.value.find((item) => item.id === id)
    if (row) openEdit(row)
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
    game_id: '',
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

async function load() {
  loading.value = true
  error.value = ''
  try {
    const query = { search: String(filters.value.search || '').trim(), type: filters.value.type, status: filters.value.status, teamId: teamFilter.value || undefined }
    const [strategyRows, taskRows, gameRows] = await Promise.all([client.listStrategies(query), client.listTasks({ team_id: teamFilter.value || undefined }), client.listGames({ scope: 'all' })])
    rows.value = Array.isArray(strategyRows) ? strategyRows : []
    pagination.value.current = 1
    tasks.value = Array.isArray(taskRows) ? taskRows : []
    games.value = Array.isArray(gameRows) ? gameRows : []
  } catch (e) {
    error.value = e.message || '策略加载失败'
  } finally {
    loading.value = false
  }
}

function resetFilters() {
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
    game_id: row.game_id || '',
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
    if (!String(form.value.game_id || '').trim()) {
      feedback.toast.error('请选择所属游戏')
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

function gameName(id) {
  const game = games.value.find((item) => item.id === id)
  return game?.name || '-'
}

// 只列启用游戏。编辑一条挂在停用游戏上的历史策略时，把当前游戏补成不可选选项：
// 不补，原值显示不出来，保存一次就把游戏悄悄换掉了。
const formGameOptions = computed(() => {
  const options = enabledGames(games.value).map((game) => ({ id: game.id, label: game.name, disabled: false }))
  const current = games.value.find((game) => game.id === form.value.game_id)
  if (current && current.status !== GAME_STATUS_ENABLED) {
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
    <div class="wt-resource-page strategy-page">
      <FeedbackAlert v-if="error" theme="error" :message="error" closable style="margin-bottom:16px" @close="error=''" />
      <ResourcePageHeader title="挖掘策略" description="配置内容发现规则，让系统自动帮你找到优质内容">
        <template #actions>
          <t-button v-if="canManage" theme="primary" @click="openCreate">新增策略</t-button>
          <t-button class="wt-secondary-button" variant="outline" @click="load">刷新</t-button>
        </template>
      </ResourcePageHeader>

      <ResourceStatGrid :items="stats" />

      <ResourceCard class="strategy-card">
        <div class="filter-bar">
          <label class="wt-filter-field wt-filter-field--wide"><span class="wt-filter-field__label">策略名称</span><t-input v-model="filters.search" clearable placeholder="搜索策略名称或 ID" @enter="load" /></label>
          <label v-if="isAdmin" class="wt-filter-field"><span class="wt-filter-field__label">运营团队</span><t-select v-model="teamFilter" clearable placeholder="全部" @change="changeTeamFilter"><t-option v-for="team in teams" :key="team.id" :value="team.id" :label="team.name" /></t-select></label>
          <label class="wt-filter-field"><span class="wt-filter-field__label">策略类型</span><t-select v-model="filters.type" clearable placeholder="全部">
            <t-option value="keyword" label="关键词" />
            <t-option value="author" label="作者" />
          </t-select></label>
          <label class="wt-filter-field"><span class="wt-filter-field__label">状态</span><t-select v-model="filters.status" clearable placeholder="全部">
            <t-option value="enabled" label="启用" />
            <t-option value="disabled" label="停用" />
          </t-select></label>
          <div class="wt-filter-actions"><t-button theme="primary" @click="load">查询</t-button><t-button class="wt-secondary-button" variant="outline" @click="resetFilters">重置</t-button></div>
        </div>
        <div class="table-scroll-wrap">
          <t-table class="wt-resource-table" :data="pagedRows" :columns="columns" row-key="id" hover :scroll="tableScroll" empty="暂无挖掘策略">
            <template #team="{ row }">{{ teamName(row.team_id) }}</template>
            <template #game="{ row }">{{ gameName(row.game_id) }}</template>
            <template #type="{ row }">{{ typeLabel(row.strategy_type) }}</template>
            <template #rule="{ row }">
              <a class="strategy-rule-link" @click="row.strategy_type === 'author' ? openAuthors(row) : openKeywords(row)">{{ ruleLabel(row) }}</a>
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
              <t-space size="small">
                <t-button size="small" class="wt-secondary-button" variant="outline" @click="openDetail(row)">详情</t-button>
                <t-button v-if="canManage" size="small" theme="primary" :disabled="row.status !== 'enabled' || isRunning(row) || row.strategy_type === 'author'" @click="run(row)">执行</t-button>
                <t-button size="small" class="wt-secondary-button" variant="outline" @click="$router.push(`/crawl-tasks?strategy_id=${row.id}`)">任务</t-button>
                <t-button v-if="canManage" size="small" class="wt-secondary-button" variant="outline" :disabled="isRunning(row)" @click="openEdit(row)">编辑</t-button>
                <t-dropdown v-if="canManage" trigger="click">
                  <t-button size="small" class="wt-secondary-button" variant="outline">更多</t-button>
                  <t-dropdown-menu>
                    <t-dropdown-item @click="openCopy(row)">复制</t-dropdown-item>
                    <t-dropdown-item :disabled="isRunning(row)" @click="remove(row)">删除</t-dropdown-item>
                  </t-dropdown-menu>
                </t-dropdown>
              </t-space>
            </template>
          </t-table>
        </div>
        <div class="pagination-bar"><t-pagination v-model:current="pagination.current" v-model:pageSize="pagination.pageSize" :total="rows.length" :page-size-options="[10, 20, 50]" /></div>
      </ResourceCard>

      <t-dialog v-model:visible="visible" :header="editingID ? '编辑挖掘策略' : '新增挖掘策略'" width="880px" :footer="false">
        <div class="strategy-editor">
          <div class="strategy-editor__form">
            <div class="strategy-section">
              <div class="strategy-section__title"><span class="strategy-no">①</span>基础信息<small>配置策略基本属性和内容归属，用于管理和数据统计</small></div>
              <t-form label-width="88px">
                <t-form-item label="策略名称" required-mark><t-input v-model="form.name" placeholder="例如：王者荣耀热点" /></t-form-item>
                <t-form-item v-if="isAdmin && !editingID" label="目标团队" required-mark><t-select v-model="form.team_id" placeholder="选择运营团队"><t-option v-for="team in teams" :key="team.id" :value="team.id" :label="team.name" /></t-select></t-form-item>
                <t-form-item label="所属游戏" required-mark><t-select v-model="form.game_id" clearable placeholder="选择游戏分类"><t-option v-for="game in formGameOptions" :key="game.id" :value="game.id" :label="game.label" :disabled="game.disabled" /></t-select></t-form-item>
                <t-form-item label="平台" required-mark><t-select v-model="form.platform"><t-option value="douyin" label="抖音" /><t-option value="bilibili" label="B站（待接入）" disabled /></t-select></t-form-item>
                <t-form-item label="策略类型"><t-radio-group v-model="form.strategy_type"><t-radio value="keyword">关键词</t-radio><t-radio value="author" disabled>作者（维护中）</t-radio></t-radio-group></t-form-item>
              </t-form>
            </div>

            <div class="strategy-section">
              <div class="strategy-section__title"><span class="strategy-no">②</span>挖掘对象<small>配置具体的挖掘关键词或作者</small></div>
              <t-form label-width="88px">
                <t-form-item v-if="form.strategy_type === 'keyword'" :label="`关键词 · ${(form.config.keywords || []).length}个`"><KeywordTags v-model="form.config.keywords" placeholder="输入关键词后按回车添加" /></t-form-item>
                <t-form-item v-else :label="`作者 · ${(form.config.authors || []).length}个`"><KeywordTags v-model="form.config.authors" placeholder="输入作者昵称或 ID 后按回车添加" /></t-form-item>
              </t-form>
            </div>

            <div class="strategy-section">
              <div class="strategy-section__title"><span class="strategy-no">③</span>执行计划<small>设置策略的执行方式和时间</small></div>
              <t-form label-width="88px">
                <t-form-item label="执行方式" required-mark><t-radio-group v-model="scheduleMode"><t-radio value="manual">手动执行</t-radio><t-radio value="scheduled">定时执行</t-radio></t-radio-group></t-form-item>
                <t-form-item v-if="scheduleMode === 'scheduled'" label="执行周期" required-mark><t-space><span class="strategy-schedule-prefix">每天</span><t-input v-model="scheduleTime" style="width:120px" placeholder="09:00" /></t-space></t-form-item>
              </t-form>
            </div>

            <div class="strategy-section">
              <div class="strategy-section__title"><span class="strategy-no">④</span>转素材规则<small>按优先级从上往下匹配，第一条命中的规则决定是否自动转素材；不配置时只采集</small></div>
              <div v-for="(rule, index) in form.config.material_rules" :key="rule.priority" class="strategy-rule">
                <div class="strategy-rule__header">
                  <strong>优先级 {{ rule.priority }}</strong>
                  <t-space size="small">
                    <t-button size="small" variant="text" :disabled="index === 0" @click="moveRule(index, -1)">上移</t-button>
                    <t-button size="small" variant="text" :disabled="index === form.config.material_rules.length - 1" @click="moveRule(index, 1)">下移</t-button>
                    <t-button size="small" theme="danger" variant="text" @click="removeRule(index)">删除</t-button>
                  </t-space>
                </div>
                <t-form label-width="88px">
                  <t-form-item label="规则类型" required-mark>
                    <t-radio-group v-model="rule.action"><t-radio value="match">命中</t-radio><t-radio value="filter">过滤</t-radio></t-radio-group>
                  </t-form-item>
                  <t-form-item label="判断方式" required-mark>
                    <t-radio-group v-model="rule.logic"><t-radio value="all">全部满足</t-radio><t-radio value="any">任一满足</t-radio></t-radio-group>
                  </t-form-item>
                  <t-form-item label="条件" required-mark>
                    <div class="strategy-rule__conditions">
                      <div v-for="(condition, conditionIndex) in rule.conditions" :key="conditionIndex" class="strategy-rule__condition">
                        <t-select v-model="condition.field" @change="changeConditionField(condition)"><t-option v-for="field in conditionFields" :key="field.value" :value="field.value" :label="field.label" /></t-select>
                        <!-- 运算符选项必须走 :options：<t-option> 子节点是插槽内容，编辑器换 form 对象后下拉会停在旧对象上。 -->
                        <t-select v-model="condition.operator" :options="conditionOperators(condition.field)" @change="changeConditionOperator(condition)" />
                        <t-date-picker v-if="condition.field === 'published_at' && ['gte', 'lt'].includes(condition.operator)" v-model="condition.value" enable-time-picker value-type="time-stamp" format="YYYY-MM-DD HH:mm:ss" clearable />
                        <t-input-number v-else v-model="condition.value" :min="condition.operator === 'recent_hours' || condition.operator === 'recent_days' ? 1 : 0" :max="condition.operator === 'recent_hours' ? 8760 : condition.operator === 'recent_days' ? 365 : undefined" theme="column" />
                        <t-button size="small" theme="danger" variant="text" :disabled="rule.conditions.length === 1" @click="rule.conditions.splice(conditionIndex, 1)">删除</t-button>
                      </div>
                      <t-button size="small" variant="text" @click="rule.conditions.push(defaultCondition())">添加条件</t-button>
                    </div>
                  </t-form-item>
                </t-form>
              </div>
              <t-button class="strategy-rule__add" variant="outline" @click="addRule">添加优先级</t-button>
            </div>

            <div class="strategy-section">
              <div class="strategy-section__title"><span class="strategy-no">⑤</span>策略状态<small>关闭后将不会执行该策略</small></div>
              <t-form label-width="88px"><t-form-item label="状态"><t-switch v-model="form.status" :custom-value="['enabled', 'disabled']" /></t-form-item></t-form>
            </div>
          </div>

          <div class="strategy-editor__aside">
            <div class="strategy-aside">
              <div class="strategy-aside__title">配置说明</div>
              <div class="strategy-aside__item"><strong>① 基础信息</strong><span>策略名称用于区分不同策略，游戏仅用于内容分类和数据统计。</span></div>
              <div class="strategy-aside__item"><strong>② 挖掘对象</strong><span>可配置关键词或作者，系统将根据配置对象发现相关内容。</span></div>
              <div class="strategy-aside__item"><strong>③ 执行计划</strong><span>支持手动执行和定时执行，定时执行按设定时间自动运行。</span></div>
              <div class="strategy-aside__item"><strong>④ 转素材规则</strong><span>规则按优先级从上往下匹配，命中「过滤」不转素材，命中「命中」自动转素材；无规则时只采集。</span></div>
              <div class="strategy-aside__tip">小提示：创建策略后，建议先手动执行一次，验证规则是否符合预期。</div>
            </div>
          </div>
        </div>
        <div class="strategy-editor__footer">
          <t-button class="wt-secondary-button" variant="outline" @click="visible = false">取消</t-button>
          <t-button theme="primary" :loading="saving" @click="save">保存</t-button>
        </div>
      </t-dialog>

      <t-dialog v-model:visible="keywordVisible" header="关键词配置" width="520px" :footer="false">
        <div v-if="keywordTarget" class="config-dialog">
          <div class="config-meta"><span>策略名称</span><strong>{{ keywordTarget.name }}</strong></div>
          <div class="config-meta"><span>关键词数量</span><strong>{{ keywordTarget.keywords.length }}</strong></div>
          <div class="config-list">
            <div class="config-list__title">关键词列表</div>
            <div v-for="(keyword, index) in keywordTarget.keywords" :key="index" class="config-list__item">{{ index + 1 }}. {{ keyword }}</div>
            <div v-if="!keywordTarget.keywords.length" class="config-list__empty">暂无关键词</div>
          </div>
        </div>
      </t-dialog>

      <t-dialog v-model:visible="authorVisible" header="作者配置" width="560px" :footer="false">
        <div v-if="authorTarget" class="config-dialog">
          <div class="config-meta"><span>策略名称</span><strong>{{ authorTarget.name }}</strong></div>
          <div class="config-meta"><span>作者数量</span><strong>{{ authorTarget.authors.length }}</strong></div>
          <div class="config-list">
            <div class="config-list__title">作者列表</div>
            <div v-for="(author, index) in authorTarget.authors" :key="index" class="author-item">
              <span class="author-avatar">{{ (author.name || '?').slice(0, 1) }}</span>
              <div class="author-info"><strong>{{ author.name }}</strong><small>平台ID：{{ author.platform_id }}</small></div>
              <div class="author-side"><span>粉丝量</span><strong>{{ author.fans }}</strong></div>
            </div>
          </div>
        </div>
      </t-dialog>

      <t-drawer :close-btn="true" v-model:visible="detailVisible" header="策略详情" size="min(92vw, 560px)" :footer="false">
        <div v-if="detailRow" class="strategy-drawer">
          <div class="strategy-drawer__field"><span>策略名称</span><strong>{{ detailRow.name || '-' }}</strong></div>
          <div class="strategy-drawer__field"><span>游戏</span><strong>{{ gameName(detailRow.game_id) }}</strong></div>
          <div class="strategy-drawer__field"><span>平台</span><strong>{{ detailRow.platform || '-' }}</strong></div>
          <div class="strategy-drawer__field"><span>类型</span><strong>{{ typeLabel(detailRow.strategy_type) }}</strong></div>
          <div class="strategy-drawer__field"><span>关键词</span><strong>{{ (detailRow.config?.keywords || []).join('、') || detailRow.config?.author || '-' }}</strong></div>
          <div class="strategy-drawer__field"><span>执行周期</span><strong>{{ scheduleLabel(detailRow.schedule) }}</strong></div>
          <div class="strategy-drawer__field"><span>转素材规则</span><strong>{{ materialLabel(detailRow) }}</strong></div>
          <div class="strategy-drawer__field"><span>状态</span><strong>{{ detailRow.status === 'enabled' ? '启用' : '停用' }}</strong></div>
          <div class="strategy-drawer__field"><span>创建时间</span><strong>{{ formatDateTime(detailRow.created_at) }}</strong></div>
          <div class="strategy-drawer__field"><span>修改时间</span><strong>{{ formatDateTime(detailRow.updated_at) }}</strong></div>
          <div class="strategy-drawer__field"><span>创建人</span><strong>{{ detailRow.created_by_name || '-' }}</strong></div>
          <div class="strategy-drawer__field"><span>修改人</span><strong>{{ detailRow.updated_by_name || '-' }}</strong></div>
        </div>
      </t-drawer>
    </div>
  </t-loading>
</template>

<style scoped>
.strategy-card { padding: 18px 20px; margin-top: 16px; }
.strategy-rule-link { color: #7c3aed; cursor: pointer; font-weight: 500; }
.strategy-rule-link:hover { text-decoration: underline; }
.material-summary { display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
.strategy-rule { margin: 12px 0; padding: 12px; border: 1px solid var(--wt-border); border-radius: 8px; }
.strategy-rule__header { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-bottom: 12px; }
.strategy-rule__conditions { display: flex; flex-direction: column; gap: 8px; width: 100%; }
.strategy-rule__condition { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; align-items: center; }
.strategy-rule__condition > :nth-child(-n + 3) { min-width: 0; width: 100%; }
.strategy-rule__condition > :last-child { justify-self: end; }
.strategy-rule__add { width: 100%; margin-bottom: 16px; }
.form-section { margin: 6px 0 4px; padding-left: 96px; color: var(--wt-text-secondary); font-size: 13px; font-weight: 600; }
.config-dialog { display: flex; flex-direction: column; gap: 12px; }
.config-meta { display: flex; justify-content: space-between; padding: 10px 12px; border: 1px solid var(--wt-border); border-radius: 8px; }
.config-meta span { color: var(--wt-text-tertiary); font-size: 13px; }
.config-meta strong { color: var(--wt-text-primary); font-size: 14px; }
.config-list { padding: 12px; border: 1px solid var(--wt-border); border-radius: 8px; }
.config-list__title { margin-bottom: 8px; color: var(--wt-text-secondary); font-size: 13px; font-weight: 600; }
.config-list__item { padding: 6px 0; border-bottom: 1px dashed var(--wt-border); color: var(--wt-text-primary); font-size: 14px; }
.config-list__item:last-child { border-bottom: none; }
.config-list__empty { color: var(--wt-text-tertiary); font-size: 13px; }
.author-item { display: flex; align-items: center; gap: 10px; padding: 8px 0; border-bottom: 1px dashed var(--wt-border); }
.author-item:last-child { border-bottom: none; }
.author-avatar { display: flex; align-items: center; justify-content: center; width: 34px; height: 34px; border-radius: 50%; background: var(--wt-info-bg); color: var(--wt-primary); font-weight: 600; }
.author-info { flex: 1; display: flex; flex-direction: column; gap: 2px; }
.author-info strong { color: var(--wt-text-primary); font-size: 14px; }
.author-info small { color: var(--wt-text-tertiary); font-size: 12px; }
.author-side { display: flex; flex-direction: column; align-items: flex-end; gap: 2px; }
.author-side span { color: var(--wt-text-tertiary); font-size: 12px; }
.author-side strong { color: var(--wt-text-primary); font-size: 14px; }
.strategy-editor { display: grid; grid-template-columns: 1fr 260px; gap: 18px; }
.strategy-editor__form { display: flex; flex-direction: column; gap: 14px; min-width: 0; }
.strategy-section { border: 1px solid var(--wt-border); border-radius: 10px; padding: 12px 14px; }
.strategy-section__title { display: flex; align-items: baseline; gap: 8px; margin-bottom: 10px; color: var(--wt-text-primary); font-size: 14px; font-weight: 600; }
.strategy-section__title small { color: var(--wt-text-tertiary); font-size: 12px; font-weight: 400; }
.strategy-no { color: var(--wt-primary); font-weight: 700; }
.strategy-schedule-prefix { color: var(--wt-text-secondary); font-size: 14px; }
.strategy-editor__aside { min-width: 0; }
.strategy-aside { position: sticky; top: 12px; display: flex; flex-direction: column; gap: 12px; padding: 14px; border: 1px solid var(--wt-border); border-radius: 10px; background: var(--wt-bg-page); }
.strategy-aside__title { color: var(--wt-text-primary); font-size: 14px; font-weight: 600; }
.strategy-aside__item { display: flex; flex-direction: column; gap: 3px; }
.strategy-aside__item strong { color: var(--wt-text-primary); font-size: 13px; }
.strategy-aside__item span { color: var(--wt-text-tertiary); font-size: 12px; line-height: 1.55; }
.strategy-aside__tip { margin-top: 4px; padding: 10px; border-radius: 8px; background: var(--wt-info-bg); color: var(--wt-primary); font-size: 12px; line-height: 1.55; }
.strategy-editor__footer { display: flex; justify-content: flex-end; gap: 8px; margin-top: 16px; }
.strategy-drawer { display: flex; flex-direction: column; gap: 10px; }
.strategy-drawer__field { display: flex; justify-content: space-between; gap: 12px; padding: 10px 12px; border: 1px solid var(--wt-border); border-radius: 8px; }
.strategy-drawer__field span { color: var(--wt-text-tertiary); font-size: 12px; flex-shrink: 0; }
.strategy-drawer__field strong { color: var(--wt-text-primary); font-size: 14px; font-weight: 600; text-align: right; word-break: break-all; }
</style>
