<script setup>
import { feedback } from '../../../shared/ui/feedback/index.js'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { DialogPlugin } from 'tdesign-vue-next'
import { createComposeClient } from '../../../shared/api/compose.js'
import { createSessionClient } from '../../../shared/api/session.js'
import ResourceCard from '../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../shared/ui/resource/ResourcePageHeader.vue'
import { opColumnWidth } from '../../../shared/ui/resource/opColumnWidth.js'
import { formatDateTime } from '../../../shared/utils/datetime.js'
import { schemaFields } from '../schemaFields.js'
import { scheduleLabel } from '../scheduleForm.js'
import { defaultCondition, displayRules, saveRules } from '../strategyRules.js'
import { templateLabel } from '../strategyConfig.js'
import ComposeStrategySummary from '../components/ComposeStrategySummary.vue'

const client = createComposeClient()
const route = useRoute()
const sessionClient = createSessionClient()

const rows = ref([])
const templates = ref([])
const games = ref([])
const teams = ref([])
const currentUser = ref(null)
const loading = ref(false)
const error = ref('')

// 与后端 canManageStrategies 同一条规则：启用状态下的高级运营或管理员能写，
// 普通运营只读。页面据此藏写入口；真正的授权判定在 Service 层，藏按钮只是少一次必失败请求。
const canManage = computed(() => ['senior_operator', 'admin'].includes(currentUser.value?.role))
const isAdmin = computed(() => currentUser.value?.role === 'admin')

const visible = ref(false)
const saving = ref(false)
const editingID = ref(null)
const form = ref(defaultForm())
const nextRunPreview = ref(null)
const nextRunPreviewError = ref('')
let previewSequence = 0
// 模板参数区由所选模板的 schema 驱动：需要哪些字段、叫什么名字、什么类型都来自
// schema（执行器 BasicCutV1() 发布），页面不写死字段清单。
const templateParams = ref({})
// 校验就近提示（AdminUI 规范 §5.6）：错误挂在字段旁边，不再只靠 toast。
const paramErrors = ref({})
const nameError = ref('')
// 打开编辑器时的快照，关闭时比对判断「有未保存内容」，决定要不要二次提示。
const savedSnapshot = ref('')
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

function renumberRules() { form.value.priority_rules.forEach((rule, index) => { rule.priority = index + 1 }) }
function addRule() { form.value.priority_rules.push({ priority: form.value.priority_rules.length + 1, action: 'match', logic: 'all', conditions: [defaultCondition()] }) }
function removeRule(index) { form.value.priority_rules.splice(index, 1); renumberRules() }
function moveRule(index, direction) {
  const target = index + direction
  if (target < 0 || target >= form.value.priority_rules.length) return
  const [rule] = form.value.priority_rules.splice(index, 1)
  form.value.priority_rules.splice(target, 0, rule)
  renumberRules()
}
function changeConditionField(condition) {
  condition.operator = condition.field === 'published_at' ? 'recent_days' : 'gte'
  condition.value = condition.field === 'published_at' ? 1 : 0
}
function changeConditionOperator(condition) { condition.value = ['gte', 'lt'].includes(condition.operator) && condition.field === 'published_at' ? null : 1 }

const copyVisible = ref(false)
const copySaving = ref(false)
const copyTarget = ref(null)
const copyName = ref('')

const detailVisible = ref(false)
const detailRow = ref(null)
const runPendingID = ref(null)
const statusPendingID = ref(null)
const runHistory = ref([])
const runHistoryError = ref('')

// previously_run 是现行原因串（这条 (策略, 素材) 已有任务记录）；already_produced 与
// in_progress 是它合并前的两种，历史 run 的快照里还留着，所以照样要有词。
const skipReasons = {
  creator_disabled: '创建人已停用', template_unavailable: '模板不可用', other_game: '不属于当前游戏',
  material_unavailable: '素材不可用', already_produced: '该策略已产出成片', in_progress: '任务正在排队或执行',
  previously_run: '已有任务记录',
  filtered: '被执行条件过滤', task_create_failed: '创建任务失败',
  daily_limit: '达到每日上限', per_run_limit: '达到单次上限',
}

// 批次结果归集（CHG-20261015-095）：一次运行创建的一批任务逐素材展开；
// 统计数字直接来自服务端对库内行的 GROUP BY，页面不做二次推算。
const batchStatusLabels = { pending: '排队中', running: '处理中', success: '成功', failed: '失败', cancelled: '已取消' }
const batchDetail = ref({})

async function toggleRunBatch(run) {
  const state = batchDetail.value[run.run_key]
  if (state) {
    batchDetail.value[run.run_key] = { ...state, open: !state.open }
    return
  }
  batchDetail.value[run.run_key] = { open: true, loading: true, error: '', view: null }
  try {
    const view = await client.listStrategyRunTasks(detailRow.value.id, run.schedule_slot)
    if (detailRow.value?.id) batchDetail.value[run.run_key] = { open: true, loading: false, error: '', view }
  } catch (e) {
    batchDetail.value[run.run_key] = { open: true, loading: false, error: e?.message || '批次任务读取失败', view: null }
  }
}

function batchCountText(counts) {
  if (!counts) return ''
  return Object.entries(batchStatusLabels)
    .map(([key, label]) => `${label} ${counts[key] ?? 0}`)
    .join(' · ')
}

async function loadRunHistory(strategyID) {
  runHistoryError.value = ''
  try {
    const data = await client.listStrategyRuns(strategyID)
    if (detailRow.value?.id === strategyID) runHistory.value = Array.isArray(data) ? data : []
  } catch (e) {
    if (detailRow.value?.id === strategyID) runHistoryError.value = e?.message || '运行记录读取失败'
  }
}

function openDetail(row) {
  detailRow.value = row
  runHistory.value = []
  detailVisible.value = true
  loadRunHistory(row.id)
}

const filters = ref({ name: '', game_id: '', template_id: '', status: '' })
const pagination = ref({ current: 1, pageSize: 20 })

const columns = computed(() => [
  { colKey: 'id', title: 'ID', width: 70 },
  { colKey: 'name', title: '策略名称', width: 220 },
  { colKey: 'template', title: '合成模板', width: 180 },
  { colKey: 'game', title: '所属游戏', width: 130 },
  { colKey: 'team', title: '所属团队', width: 130 },
  { colKey: 'schedule', title: '执行计划', width: 150 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'op', title: '操作', width: opColumnWidth(canManage.value ? ['详情', '编辑', '执行', '复制'] : ['详情']), fixed: 'right' },
])

// 横向滚动宽度由列宽推导，不写死（写死值与列宽合计无关，是个漂移源）。
const tableScroll = computed(() => ({
  x: `${columns.value.reduce((sum, col) => sum + (col.width || 0), 0)}px`,
}))

const filteredRows = computed(() => rows.value.filter((row) => {
  if (filters.value.name && !String(row.name || '').toLowerCase().includes(filters.value.name.trim().toLowerCase())) return false
  if (filters.value.game_id && row.game_id !== filters.value.game_id) return false
  if (filters.value.template_id && row.compose_template_id !== filters.value.template_id) return false
  if (filters.value.status && row.status !== filters.value.status) return false
  return true
}))

const pagedRows = computed(() => {
  const start = (pagination.value.current - 1) * pagination.value.pageSize
  return filteredRows.value.slice(start, start + pagination.value.pageSize)
})

const templateOptions = computed(() => templates.value.map((template) => ({
  label: `${template.name} v${template.version}`,
  value: template.id,
  version: template.version,
})))

// 编辑时绑定的模板可能已停用（不在目录里）；此时从既有 config 反推字段——config
// 是写入时按 schema 校验过的，字段集必然一致。标签退化为参数名。
const formSchemaFields = computed(() => {
  let schema = null
  if (editingID.value) {
    schema = templates.value.find((item) => item.id === form.value.compose_template_id)?.schema
    if (!schema && form.value.configFallback) {
      return Object.entries(form.value.configFallback).map(([key, value]) => ({
        key, label: key, type: typeof value === 'number' ? 'integer' : typeof value, minimum: undefined, required: true,
      }))
    }
  } else {
    schema = templates.value.find((item) => item.id === form.value.compose_template_id)?.schema
  }
  return schemaFields(schema)
})


onMounted(() => {
  load()
  const id = String(route.query.strategy_id || '')
  if (/^\d+$/.test(id)) client.getStrategy(Number(id)).then(openDetail).catch((e) => feedback.toast.error(e?.message || '打开策略失败'))
})

async function load() {
  loading.value = true
  error.value = ''
  try {
    // 团队列表所有角色都可读（服务端仅要求登录），编辑/详情要把 team_id 映射成分组名；
    // 管理员的创建表单另用它选目标团队。
    currentUser.value = await sessionClient.me()
    const [strategyRows, templateRows, gameRows, teamRows] = await Promise.all([
      client.listStrategies(), client.listTemplates(), client.listGames(), client.listTeams().catch(() => []),
    ])
    rows.value = Array.isArray(strategyRows) ? strategyRows : []
    templates.value = Array.isArray(templateRows) ? templateRows : []
    games.value = Array.isArray(gameRows) ? gameRows : []
    teams.value = Array.isArray(teamRows) ? teamRows : []
    pagination.value.current = 1
  } catch (e) {
    error.value = e.message || '合成策略加载失败'
  } finally {
    loading.value = false
  }
}

function defaultForm() {
  return {
    team_id: '',
    game_id: '',
    name: '',
    compose_template_id: '',
    configFallback: null,
    execution_mode: 'manual',
    schedule: { mode: 'interval', interval_minutes: 30, daily_time: '09:00', timezone: 'Asia/Shanghai' },
    priority_rules: [{ priority: 1, action: 'match', logic: 'all', conditions: [defaultCondition()] }],
    single_limit: 1,
    daily_limit: 1,
  }
}

watch(() => [visible.value, form.value.execution_mode, form.value.schedule.mode, form.value.schedule.interval_minutes, form.value.schedule.daily_time, form.value.schedule.timezone], async () => {
  const sequence = ++previewSequence
  nextRunPreview.value = null
  nextRunPreviewError.value = ''
  if (!visible.value || form.value.execution_mode !== 'scheduled') return
  const schedule = {
    mode: form.value.schedule.mode,
    timezone: String(form.value.schedule.timezone || '').trim(),
    ...(form.value.schedule.mode === 'interval'
      ? { interval_minutes: Number(form.value.schedule.interval_minutes) }
      : { daily_time: form.value.schedule.daily_time }),
  }
  if (!schedule.timezone || (schedule.mode === 'interval' && (!Number.isInteger(schedule.interval_minutes) || schedule.interval_minutes < 1 || schedule.interval_minutes > 1440)) || (schedule.mode === 'daily' && !/^([01]\d|2[0-3]):[0-5]\d$/.test(schedule.daily_time))) return
  try {
    const result = await client.previewStrategyNextRun(schedule)
    if (sequence === previewSequence) nextRunPreview.value = result.next_run_at
  } catch (e) {
    if (sequence === previewSequence) nextRunPreviewError.value = e?.message || '无法计算下次执行时间'
  }
})

function resetFilters() {
  filters.value = { name: '', game_id: '', template_id: '', status: '' }
  pagination.value.current = 1
}

function openCreate() {
  editingID.value = null
  form.value = defaultForm()
  templateParams.value = {}
  clearParamErrors()
  nameError.value = ''
  beginSnapshot()
  visible.value = true
}

function openEdit(row) {
  editingID.value = row.id
  form.value = {
    team_id: row.team_id,
    game_id: row.game_id,
    name: row.name,
    compose_template_id: row.compose_template_id,
    configFallback: templates.value.some((item) => item.id === row.compose_template_id) ? null : row.config,
    execution_mode: row.execution_mode || 'manual',
    schedule: { mode: row.schedule?.mode || 'interval', interval_minutes: row.schedule?.interval_minutes || 30, daily_time: row.schedule?.daily_time || '09:00', timezone: row.schedule?.timezone || 'Asia/Shanghai' },
    priority_rules: displayRules(row.priority_rules || []),
    single_limit: row.single_limit || 1,
    daily_limit: row.daily_limit || 1,
  }
  templateParams.value = Object.assign({}, row.config || {})
  clearParamErrors()
  nameError.value = ''
  beginSnapshot()
  visible.value = true
}

function beginSnapshot() {
  savedSnapshot.value = editorState()
}

function editorState() {
  return JSON.stringify([editingID.value, form.value, templateParams.value])
}

function editorDirty() {
  return editorState() !== savedSnapshot.value
}

// 选了模板（或换模板）后按该模板 schema 重置参数区：整数字段给 schema 下限（无下限给 0），
// 字符串给空串，布尔给 false。
function onTemplateChange() {
  const params = {}
  for (const field of formSchemaFields.value) {
    if (field.type === 'integer') params[field.key] = field.minimum ?? 0
    else if (field.type === 'boolean') params[field.key] = false
    else params[field.key] = ''
  }
  templateParams.value = params
}

function openCopy(row) {
  copyTarget.value = row
  copyName.value = `${row.name} 副本`
  copyVisible.value = true
}

async function submitCopy() {
  const name = String(copyName.value || '').trim()
  if (!name) {
    feedback.toast.error('请输入新策略名称')
    return
  }
  copySaving.value = true
  try {
    await client.copyStrategy(copyTarget.value.id, name)
    copyVisible.value = false
    await load()
    feedback.toast.success('策略已复制')
  } catch (e) {
    error.value = e.message || '策略复制失败'
  } finally {
    copySaving.value = false
  }
}

function scheduleState(row) {
  if (row.execution_mode !== 'scheduled') return '手动执行'
  return scheduleLabel(row.schedule)
}

function runNow(row) {
  if (runPendingID.value !== null || row.status !== 'enabled') return
  const dialog = DialogPlugin.confirm({
    header: '立即创建合成任务',
    body: `将从策略创建人的“我的素材”中筛选候选素材，为每条符合条件的素材创建独立任务。此操作不改变「${row.name}」的下次计划时间。`,
    confirmBtn: { content: '立即执行', theme: 'primary' },
    onConfirm: async () => {
      if (runPendingID.value !== null) return
      runPendingID.value = row.id
      try {
        const result = await client.runStrategyNow(row.id, crypto.randomUUID())
        feedback.toast.success(`执行已提交，创建 ${result.created_count} 个合成任务`)
        await loadRunHistory(row.id)
        dialog.hide()
      } catch (e) {
        error.value = e?.message || '立即执行失败'
        feedback.toast.error(error.value)
        dialog.hide()
      } finally {
        runPendingID.value = null
      }
    },
  })
}

// 启停开关在列表行上（走查修订第 3 条）。开关受控于 row.status，确认后才真正变更；
// 取消时不动数据，下一次渲染自然回弹。
function toggleStatus(row, nextEnabled) {
  if (statusPendingID.value !== null) return
  const next = nextEnabled ? 'enabled' : 'disabled'
  const dialog = DialogPlugin.confirm({
    header: next === 'enabled' ? '启用策略' : '停用策略',
    body: next === 'enabled'
      ? `即将启用「${row.name}」。启用后可人工执行${row.execution_mode === 'scheduled' ? '，也会按计划创建任务' : ''}。`
      : `停用「${row.name}」后不再创建新的人工或定时任务；已有任务和历史成片不受影响。`,
    confirmBtn: { content: next === 'enabled' ? '启用' : '停用', theme: next === 'enabled' ? 'primary' : 'danger' },
    onConfirm: async () => {
      if (statusPendingID.value !== null) return
      statusPendingID.value = row.id
      try {
        const updated = await client.setStrategyStatus(row.id, next)
        rows.value = rows.value.map((item) => item.id === row.id ? updated : item)
        if (detailRow.value?.id === row.id) detailRow.value = updated
        feedback.toast.success(next === 'enabled' ? '策略已启用' : '策略已停用')
      } catch (e) {
        error.value = e.message || '状态更新失败'
        feedback.toast.error(error.value)
      } finally {
        statusPendingID.value = null
      }
      dialog.hide()
    },
  })
}

function clearParamErrors() {
  paramErrors.value = {}
}

// 关闭编辑器：内容没动直接关；动过就按规范二次提示（X 和「取消」同一条路径）。
function requestCloseEditor() {
  if (!editorDirty()) {
    visible.value = false
    return
  }
  const dialog = DialogPlugin.confirm({
    header: '放弃未保存的修改？',
    body: '编辑内容尚未保存，关闭后将丢失。',
    confirmBtn: { content: '不保存', theme: 'danger' },
    onConfirm: () => {
      dialog.hide()
      visible.value = false
    },
  })
}

async function save() {
  nameError.value = ''
  clearParamErrors()
  const name = String(form.value.name || '').trim()
  if (!name) {
    nameError.value = '请输入策略名称'
    return
  }
  if (!editingID.value && !form.value.compose_template_id) {
    feedback.toast.error('请选择合成模板')
    return
  }
  const fields = formSchemaFields.value
  if (!fields.length) {
    feedback.toast.error('模板参数未就绪，请重新选择合成模板')
    return
  }
  // 按模板 schema 的字段集组装 config：整数字段强制取整并套 schema 下限，
  // 必填字段缺值直接拦在前端；schema 外的值不会出现，因为表单就是按 schema 渲染的。
  const config = {}
  let invalid = ''
  for (const field of fields) {
    const raw = templateParams.value[field.key]
    if (field.type === 'integer') {
      // t-input-number 清空后给 null，Number(null) 是 0，会静默变成合法下限值——先拦。
      if (raw === null || raw === undefined || raw === '') {
        paramErrors.value[field.key] = `请填写${field.label}`
        invalid = invalid || field.label
        continue
      }
      const value = Number(raw)
      if (!Number.isInteger(value) || (field.minimum !== undefined && value < field.minimum)) {
        paramErrors.value[field.key] = `必须是不小于 ${field.minimum ?? 0} 的整数`
        invalid = invalid || field.label
        continue
      }
      config[field.key] = value
    } else if (field.type === 'boolean') {
      config[field.key] = Boolean(raw)
    } else {
      const value = String(raw ?? '').trim()
      if (field.required && !value) {
        paramErrors.value[field.key] = `请输入${field.label}`
        invalid = invalid || field.label
        continue
      }
      if (field.key === 'guide_video_url' && !validGuideURL(value)) {
        paramErrors.value[field.key] = '请输入可公开下载的 HTTPS 视频地址'
        invalid = invalid || field.label
        continue
      }
      config[field.key] = value
    }
  }
  if (invalid) return
  const singleLimit = Number(form.value.single_limit)
  const dailyLimit = Number(form.value.daily_limit)
  if (!Number.isInteger(singleLimit) || singleLimit < 1 || singleLimit > 100 || !Number.isInteger(dailyLimit) || dailyLimit < singleLimit || dailyLimit > 1000) {
    feedback.toast.error('单次上限需为 1～100，每日上限需不小于单次上限且不超过 1000')
    return
  }
  if (!form.value.priority_rules.some((rule) => rule.action === 'match')) {
    feedback.toast.error('请至少添加一个命中优先级')
    return
  }
  for (const rule of form.value.priority_rules) {
    for (const condition of rule.conditions) {
      if (condition.value === null || condition.value === undefined || condition.value === '' || !Number.isFinite(Number(condition.value)) || Number(condition.value) < 0) {
        feedback.toast.error(`请填写优先级 ${rule.priority} 的有效条件值`)
        return
      }
    }
  }
  const execution = {
    execution_mode: form.value.execution_mode,
    schedule: form.value.execution_mode === 'scheduled' ? {
      mode: form.value.schedule.mode,
      timezone: String(form.value.schedule.timezone || '').trim(),
      ...(form.value.schedule.mode === 'interval' ? { interval_minutes: Number(form.value.schedule.interval_minutes) } : { daily_time: form.value.schedule.daily_time }),
    } : null,
    priority_rules: saveRules(form.value.priority_rules),
    single_limit: singleLimit,
    daily_limit: dailyLimit,
  }
  saving.value = true
  try {
    if (editingID.value) {
      await client.updateStrategy(editingID.value, { name, config, ...execution })
    } else {
      const template = templates.value.find((item) => item.id === Number(form.value.compose_template_id))
      const payload = {
        game_id: form.value.game_id,
        name,
        compose_template_id: template.id,
        compose_template_version: template.version,
        config,
        ...execution,
      }
      if (isAdmin.value) payload.team_id = Number(form.value.team_id) || 0
      await client.createStrategy(payload)
    }
    visible.value = false
    await load()
    feedback.toast.success(editingID.value ? '策略已更新' : '策略创建成功')
  } catch (e) {
    error.value = e.message || '策略保存失败'
    feedback.toast.error(error.value)
  } finally {
    saving.value = false
  }
}

function gameName(id) {
  return games.value.find((game) => game.id === id)?.name || id
}

function teamName(id) {
  return teams.value.find((team) => team.id === id)?.name || `分组 ${id}`
}

function validGuideURL(value) {
  try {
    const url = new URL(value)
    return url.protocol === 'https:' && !!url.hostname && !url.username && !url.password && !url.hash && (!url.port || url.port === '443')
  } catch { return false }
}

function guideFileName(value) {
  try { return decodeURIComponent(new URL(value).pathname.split('/').pop()) || '引导视频' } catch { return '引导视频' }
}

function conditionText(condition) {
  const field = conditionFields.find((item) => item.value === condition.field)?.label || condition.field
  const operators = conditionOperators(condition.field)
  const operator = operators.find((item) => item.value === condition.operator)?.label || condition.operator
  if (condition.operator === 'recent_hours') return `${field}近 ${condition.value} 小时`
  if (condition.operator === 'recent_days') return `${field}近 ${condition.value} 天`
  const value = condition.field === 'published_at' ? formatDateTime(condition.value) : condition.value
  return `${field}${operator}${value}`
}

</script>

<template>
  <t-loading :loading="loading" :show-overlay="true">
    <div class="wt-resource-page strategy-page">
      <FeedbackAlert v-if="error" theme="error" :message="error" closable style="margin-bottom:16px" @close="error=''" />
      <ResourcePageHeader title="合成策略" :description="canManage ? '维护合成模板的业务参数，普通运营据此创建合成任务' : '选择可用策略，在创建合成任务时使用'">
        <template #actions>
          <t-button v-if="canManage" theme="primary" @click="openCreate">新增策略</t-button>
          <t-button class="wt-secondary-button" variant="outline" @click="load">刷新</t-button>
        </template>
      </ResourcePageHeader>

      <!-- 筛选区与表格同卡（与内容池一致）：filter-bar 自带分隔线，下方接表格与分页。 -->
      <ResourceCard class="strategy-card">
        <div class="filter-bar">
          <label class="wt-filter-field wt-filter-field--wide"><span class="wt-filter-field__label">策略名称</span><t-input v-model="filters.name" clearable placeholder="搜索策略名称" @enter="load" /></label>
          <label class="wt-filter-field"><span class="wt-filter-field__label">游戏</span><t-select v-model="filters.game_id" clearable placeholder="全部">
            <t-option v-for="game in games" :key="game.id" :value="game.id" :label="game.name" />
          </t-select></label>
          <label class="wt-filter-field"><span class="wt-filter-field__label">合成模板</span><t-select v-model="filters.template_id" clearable placeholder="全部">
            <t-option v-for="template in templates" :key="template.id" :value="template.id" :label="`${template.name} v${template.version}`" />
          </t-select></label>
          <label class="wt-filter-field"><span class="wt-filter-field__label">状态</span><t-select v-model="filters.status" clearable placeholder="全部">
            <t-option value="enabled" label="启用" />
            <t-option value="disabled" label="停用" />
          </t-select></label>
          <div class="wt-filter-actions">
            <t-button theme="primary" @click="load">查询</t-button>
            <t-button class="wt-secondary-button" variant="outline" @click="resetFilters">重置</t-button>
          </div>
        </div>
        <div class="table-scroll-wrap">
          <t-table class="wt-resource-table" :data="pagedRows" :columns="columns" row-key="id" hover size="small" :scroll="tableScroll" empty="暂无合成策略">
            <template #id="{ row }"><span class="strategy-id">{{ row.id }}</span></template>
            <template #name="{ row }">
              <div class="strategy-name-cell">
                <span class="strategy-name-cell__main" :title="row.name">{{ row.name }}</span>
              </div>
            </template>
            <template #game="{ row }">{{ gameName(row.game_id) }}</template>
            <template #team="{ row }">{{ teamName(row.team_id) }}</template>
            <template #template="{ row }">{{ templateLabel(templates, row.compose_template_id) }}</template>
            <template #status="{ row }">
              <t-switch v-if="canManage" :value="row.status === 'enabled'" :label="['启用', '停用']" :loading="statusPendingID === row.id" :disabled="statusPendingID !== null" @change="(value) => toggleStatus(row, value)" />
              <t-switch v-else :value="row.status === 'enabled'" :label="['启用', '停用']" disabled title="当前角色只能查看策略状态" />
            </template>
            <template #schedule="{ row }">{{ scheduleState(row) }}</template>
            <template #op="{ row }">
              <t-space size="small" class="wt-resource-actions">
                <t-button size="small" class="wt-secondary-button" variant="outline" @click="openDetail(row)">详情</t-button>
                <template v-if="canManage">
                  <t-button size="small" theme="primary" @click="openEdit(row)">编辑</t-button>
                  <t-button size="small" class="wt-secondary-button" variant="outline" :loading="runPendingID === row.id" :disabled="row.status !== 'enabled' || runPendingID !== null" :title="row.status !== 'enabled' ? '策略已停用，不能执行' : ''" @click="runNow(row)">执行</t-button>
                  <t-button size="small" class="wt-secondary-button" variant="outline" @click="openCopy(row)">复制</t-button>
                </template>
              </t-space>
            </template>
          </t-table>
        </div>
        <div class="pagination-bar"><t-pagination v-model:current="pagination.current" v-model:pageSize="pagination.pageSize" :total="filteredRows.length" :page-size-options="[10, 20, 50]" /></div>
      </ResourceCard>

      <t-dialog :visible="visible" :header="editingID ? '编辑合成策略' : '新增合成策略'" width="880px" :footer="false" @close="requestCloseEditor">
        <div class="strategy-editor">
          <div class="strategy-editor__form">
            <div class="strategy-section">
              <div class="strategy-section__title"><span class="strategy-no">①</span>基础信息<small>配置策略基本属性和内容归属，用于管理和数据统计</small></div>
              <t-form label-width="88px">
                <t-form-item label="策略名称" required-mark>
                  <div class="strategy-field">
                    <t-input v-model="form.name" :maxlength="120" placeholder="例如：王者荣耀高光切片" :status="nameError ? 'error' : undefined" />
                    <p v-if="nameError" class="strategy-field-error">{{ nameError }}</p>
                  </div>
                </t-form-item>
                <template v-if="!editingID">
                  <t-form-item v-if="isAdmin" label="所属团队" required-mark>
                    <t-select v-model="form.team_id" clearable placeholder="选择团队">
                      <t-option v-for="team in teams" :key="team.id" :value="team.id" :label="team.name" />
                    </t-select>
                  </t-form-item>
                  <t-form-item label="所属游戏" required-mark>
                    <t-select v-model="form.game_id" clearable placeholder="选择游戏">
                      <t-option v-for="game in games" :key="game.id" :value="game.id" :label="game.name" />
                    </t-select>
                  </t-form-item>
                </template>
                <!-- 编辑时归属不可改（服务端 Update 不动 team/game），但要展示出来让人知道策略挂在哪。 -->
                <template v-else>
                  <t-form-item label="所属团队"><strong class="strategy-readonly">{{ teamName(form.team_id) }}</strong></t-form-item>
                  <t-form-item label="所属游戏"><strong class="strategy-readonly">{{ gameName(form.game_id) }}</strong></t-form-item>
                </template>
                <t-form-item label="合成模板" required-mark>
                  <t-select v-if="!editingID" v-model="form.compose_template_id" clearable placeholder="选择合成模板" @change="onTemplateChange">
                    <t-option v-for="option in templateOptions" :key="option.value" :value="option.value" :label="option.label" />
                  </t-select>
                  <strong v-else class="strategy-readonly">{{ templateLabel(templates, form.compose_template_id) }}</strong>
                </t-form-item>
              </t-form>
            </div>

            <div class="strategy-section">
              <div class="strategy-section__title"><span class="strategy-no">②</span>合成配置<small>需要填写哪些参数由所选合成模板决定</small></div>
              <t-form label-width="88px">
                <t-form-item v-if="!editingID && !form.compose_template_id" label=""><div class="strategy-bind-hint">先选择合成模板，需要填写哪些参数由模板决定。</div></t-form-item>
                <template v-else>
                  <t-form-item v-for="field in formSchemaFields" :key="field.key" :label="field.label" :required-mark="field.required">
                    <div class="strategy-field">
                      <t-input-number v-if="field.type === 'integer'" v-model="templateParams[field.key]" :min="field.minimum ?? 0" theme="column" style="width:160px" :status="paramErrors[field.key] ? 'error' : undefined" />
                      <t-switch v-else-if="field.type === 'boolean'" v-model="templateParams[field.key]" />
                      <div v-else-if="field.key === 'guide_video_url'" class="strategy-media">
                        <div v-if="validGuideURL(templateParams[field.key])" class="strategy-media__card">
                          <strong>{{ guideFileName(templateParams[field.key]) }}</strong>
                          <span>视频资源</span>
                          <a :href="templateParams[field.key]" target="_blank" rel="noopener noreferrer">预览</a>
                          <t-button size="small" variant="text" @click="templateParams[field.key] = ''">清除</t-button>
                        </div>
                        <t-input v-model="templateParams[field.key]" type="url" :placeholder="templateParams[field.key] ? '替换视频地址' : '输入 HTTPS 视频地址'" :status="paramErrors[field.key] ? 'error' : undefined" />
                      </div>
                      <t-input v-else v-model="templateParams[field.key]" :type="field.format === 'uri' ? 'url' : 'text'" :placeholder="field.format === 'uri' ? 'https://.../guide.mp4' : `请输入${field.label}`" :status="paramErrors[field.key] ? 'error' : undefined" />
                      <p v-if="paramErrors[field.key]" class="strategy-field-error">{{ paramErrors[field.key] }}</p>
                    </div>
                  </t-form-item>
                  <t-form-item v-if="!editingID" label="">
                    <div class="strategy-bind-hint">模板版本创建后固定；模板停用后不能新建绑定，已保存的策略不受影响。</div>
                  </t-form-item>
                </template>
              </t-form>
            </div>
            <div class="strategy-section">
              <div class="strategy-section__title"><span class="strategy-no">③</span>执行设置<small>选择策略默认执行方式</small></div>
              <t-form label-width="112px">
                <t-form-item label="执行方式" required-mark>
                  <t-radio-group v-model="form.execution_mode">
                    <t-radio value="manual">手动执行</t-radio>
                    <t-radio value="scheduled">定时执行</t-radio>
                  </t-radio-group>
                </t-form-item>
                <template v-if="form.execution_mode === 'scheduled'">
                  <t-form-item label="执行周期" required-mark>
                    <t-select v-model="form.schedule.mode"><t-option value="interval" label="每 N 分钟" /><t-option value="daily" label="每天固定时间" /></t-select>
                  </t-form-item>
                  <t-form-item v-if="form.schedule.mode === 'interval'" label="执行间隔" required-mark><t-input-number v-model="form.schedule.interval_minutes" :min="1" :max="1440" suffix="分钟" /></t-form-item>
                  <t-form-item v-else label="每日时间" required-mark><t-input v-model="form.schedule.daily_time" placeholder="09:00" /></t-form-item>
                  <t-form-item label="时区" required-mark><t-input v-model="form.schedule.timezone" placeholder="Asia/Shanghai" /></t-form-item>
                  <t-form-item label="下次执行预览"><span class="strategy-readonly">{{ nextRunPreview ? formatDateTime(nextRunPreview) : nextRunPreviewError || '填写有效计划后由服务端计算' }}</span></t-form-item>
                </template>
              </t-form>
            </div>
            <div class="strategy-section">
              <div class="strategy-section__title"><span class="strategy-no">④</span>执行条件<small>根据条件从当前业务范围内选择可生产素材，用于创建合成任务。</small></div>
              <p class="strategy-bind-hint">系统先检查业务范围、素材可用性和同策略素材防重；下列条件只作用于通过系统检查的素材。</p>
              <div v-for="(rule, index) in form.priority_rules" :key="rule.priority" class="strategy-rule">
                <div class="strategy-rule__header">
                  <strong>优先级 {{ rule.priority }}</strong>
                  <t-space size="small">
                    <t-button size="small" variant="text" :disabled="index === 0" @click="moveRule(index, -1)">上移</t-button>
                    <t-button size="small" variant="text" :disabled="index === form.priority_rules.length - 1" @click="moveRule(index, 1)">下移</t-button>
                    <t-button size="small" theme="danger" variant="text" @click="removeRule(index)">删除</t-button>
                  </t-space>
                </div>
                <t-form label-width="112px">
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
                        <!-- 运算符选项必须走 :options：<t-option> 子节点是插槽内容，下拉自己把它缓存成
                             一个 computed，编辑器整体换 form 对象后它追的还是旧对象，第一个条件的运算符
                             列表会一直停在「发布时间」那一套（收藏量只能选到「早于」这类非法运算符）。 -->
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
              <t-form label-width="112px">
                <t-form-item label="单次上限" required-mark><t-input-number v-model="form.single_limit" :min="1" :max="100" suffix="条" /></t-form-item>
                <t-form-item label="每日上限" required-mark><t-input-number v-model="form.daily_limit" :min="1" :max="1000" suffix="条" /></t-form-item>
              </t-form>
            </div>
          </div>

          <div class="strategy-editor__aside">
            <div class="strategy-aside">
              <div class="strategy-aside__title">配置说明</div>
              <div class="strategy-aside__item"><strong>① 基础信息</strong><span>策略名称用于区分不同策略；所属团队与游戏决定普通运营的可见范围。</span></div>
              <div class="strategy-aside__item"><strong>② 合成配置</strong><span>参数由合成模板定义（如基础切段的开始/结束秒数），保存时按模板 schema 校验。</span></div>
              <div class="strategy-aside__tip">小提示：模板版本创建后固定，模板停用不影响已保存的策略。</div>
            </div>
          </div>
        </div>
        <!-- 取消在左、保存在右（AdminUI 规范：底部动作固定次序，主按钮突出主要动作）。 -->
        <div class="strategy-editor__footer">
          <t-button class="wt-secondary-button" variant="outline" @click="requestCloseEditor">取消</t-button>
          <t-button theme="primary" :loading="saving" @click="save">保存</t-button>
        </div>
      </t-dialog>

      <t-dialog v-model:visible="copyVisible" header="复制合成策略" width="480px" :footer="false">
        <t-form label-width="96px">
          <t-form-item label="新策略名称" required-mark><t-input v-model="copyName" :maxlength="120" placeholder="输入新策略名称" @enter="submitCopy" /></t-form-item>
          <t-form-item v-if="copyTarget" label="模板"><t-input :value="templateLabel(templates, copyTarget.compose_template_id)" disabled /></t-form-item>
        </t-form>
        <div class="strategy-editor__footer">
          <t-button class="wt-secondary-button" variant="outline" @click="copyVisible = false">取消</t-button>
          <t-button theme="primary" :loading="copySaving" @click="submitCopy">复制</t-button>
        </div>
      </t-dialog>

      <t-drawer :close-btn="true" v-model:visible="detailVisible" :header="detailRow?.name || '策略详情'" size="min(62vw, 880px)" :footer="canManage">
        <div v-if="detailRow" class="strategy-drawer">
          <div class="strategy-drawer__field"><span>策略 ID</span><strong>{{ detailRow.id }}</strong></div>
          <ComposeStrategySummary
            :strategy="detailRow"
            :team="teamName(detailRow.team_id)"
            :game="gameName(detailRow.game_id)"
            :templates="templates"
            :guide-url="detailRow.config?.guide_video_url || ''"
          />
          <h4 class="strategy-detail-section">执行设置</h4>
          <div class="strategy-drawer__field"><span>执行方式</span><strong>{{ detailRow.execution_mode === 'scheduled' ? '定时执行' : '手动执行' }}</strong></div>
          <template v-if="detailRow.execution_mode === 'scheduled'">
            <div class="strategy-drawer__field"><span>周期</span><strong>{{ scheduleLabel(detailRow.schedule) }}</strong></div>
            <div class="strategy-drawer__field"><span>时区</span><strong>{{ detailRow.schedule?.timezone || '—' }}</strong></div>
            <div class="strategy-drawer__field"><span>下次执行</span><strong>{{ detailRow.next_run_at ? formatDateTime(detailRow.next_run_at) : '—' }}</strong></div>
          </template>
          <h4 class="strategy-detail-section">执行条件</h4>
          <div class="strategy-drawer__field"><span>候选范围</span><strong>创建人的“我的素材”（当前团队与游戏）</strong></div>
          <div class="strategy-drawer__field"><span>单次／每日上限</span><strong>{{ detailRow.single_limit }}／{{ detailRow.daily_limit }}</strong></div>
          <div v-for="group in detailRow.priority_rules || []" :key="group.priority" class="strategy-drawer__field"><span>优先级 {{ group.priority }}</span><strong>{{ group.action === 'filter' ? '过滤' : '命中' }} · {{ group.logic === 'all' ? '全部满足' : '任一满足' }}<br />{{ (group.conditions || []).map(conditionText).join(group.logic === 'all' ? ' 且 ' : ' 或 ') }}</strong></div>
          <h4 class="strategy-detail-section">最近运行</h4>
          <p v-if="runHistoryError" class="strategy-run-message">{{ runHistoryError }}</p>
          <p v-else-if="!runHistory.length" class="strategy-run-message">暂无运行记录</p>
          <div v-for="run in runHistory" :key="run.run_key" class="strategy-run">
            <div class="strategy-run__summary"><strong>{{ run.trigger_type === 'scheduled' ? '自动执行' : '立即执行' }} · {{ formatDateTime(run.schedule_slot) }}</strong><span>创建 {{ run.created_count }} / 候选 {{ run.candidate_count }}</span></div>
            <details v-if="run.skipped?.length"><summary>跳过 {{ run.skipped.length }} 项</summary><p v-for="(skip, index) in run.skipped" :key="`${run.run_key}-${index}`">{{ skip.material_id ? `素材 ${skip.material_id}：` : '' }}{{ skipReasons[skip.reason] || skip.reason }}</p></details>
            <div v-if="run.created_count > 0" class="strategy-run__batch">
              <button type="button" class="strategy-run__batch-toggle" @click="toggleRunBatch(run)">{{ batchDetail[run.run_key]?.open ? '收起批次任务' : '查看批次任务' }}</button>
              <div v-if="batchDetail[run.run_key]?.open" class="strategy-run__batch-body">
                <p v-if="batchDetail[run.run_key].loading" class="strategy-run-message">批次任务读取中…</p>
                <p v-else-if="batchDetail[run.run_key].error" class="strategy-run-message">{{ batchDetail[run.run_key].error }}</p>
                <template v-else-if="batchDetail[run.run_key].view">
                  <p class="strategy-run__counts">{{ batchCountText(batchDetail[run.run_key].view.counts) }}</p>
                  <div v-for="task in batchDetail[run.run_key].view.tasks" :key="task.id" class="strategy-run__task">
                    <strong>任务 {{ task.id }} · 素材 {{ task.material_id }}</strong>
                    <span>{{ batchStatusLabels[task.status] || task.status }}<template v-if="task.composite_output_id"> · 成片 {{ task.composite_output_id }}</template></span>
                    <em v-if="task.error_message || task.error_code">{{ task.error_code ? `${task.error_code}: ` : '' }}{{ task.error_message || '—' }}</em>
                  </div>
                </template>
              </div>
            </div>
          </div>
          <h4 class="strategy-detail-section">记录时间</h4>
          <div class="strategy-drawer__field"><span>创建时间</span><strong>{{ formatDateTime(detailRow.created_at) }}</strong></div>
          <div class="strategy-drawer__field"><span>修改时间</span><strong>{{ formatDateTime(detailRow.updated_at) }}</strong></div>
        </div>
        <template #footer>
          <div v-if="detailRow && canManage" class="strategy-editor__footer">
            <t-button class="wt-secondary-button" variant="outline" @click="openEdit(detailRow)">编辑</t-button>
            <t-button class="wt-secondary-button" variant="outline" :disabled="detailRow.status !== 'enabled'" @click="runNow(detailRow)">执行</t-button>
            <t-button class="wt-secondary-button" variant="outline" @click="openCopy(detailRow)">复制</t-button>
          </div>
        </template>
      </t-drawer>
    </div>
  </t-loading>
</template>

<style scoped>
.strategy-run { padding: 10px 0; border-bottom: 1px solid var(--wt-border); color: var(--wt-text-secondary); font-size: 12px; }
.strategy-run__summary { display: flex; justify-content: space-between; gap: 10px; }
.strategy-run__summary strong { color: var(--wt-text-primary); font-weight: 600; }
.strategy-run details { margin-top: 6px; }
.strategy-run details p { margin: 4px 0 0 14px; }
.strategy-run__batch { margin-top: 6px; }
.strategy-run__batch-toggle { padding: 0; border: 0; background: none; color: var(--wt-primary); font-size: 12px; cursor: pointer; }
.strategy-run__batch-body { margin-top: 6px; padding-left: 14px; }
.strategy-run__counts { margin: 0 0 6px; color: var(--wt-text-tertiary); }
.strategy-run__task { display: flex; flex-direction: column; gap: 1px; padding: 4px 0; border-top: 1px dashed var(--wt-border); }
.strategy-run__task strong { color: var(--wt-text-primary); font-weight: 600; }
.strategy-run__task span { color: var(--wt-text-secondary); }
.strategy-run__task em { font-style: normal; color: var(--td-error-color, #d54941); font-size: 12px; word-break: break-all; }
.strategy-run-message { color: var(--wt-text-tertiary); font-size: 12px; }
.strategy-card { padding: 18px 20px; margin-top: 16px; }
.strategy-id { color: var(--wt-text-primary); font-weight: 600; font-variant-numeric: tabular-nums; }
.strategy-name-cell { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.strategy-name-cell__main { overflow: hidden; color: var(--wt-text-primary); font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }
.strategy-name-cell__sub { color: var(--wt-text-tertiary); font-size: 12px; }
.strategy-schedule-cell { display: flex; flex-direction: column; align-items: flex-start; gap: 3px; color: var(--wt-text-tertiary); font-size: 12px; }
.strategy-editor { display: grid; grid-template-columns: 1fr 260px; gap: 18px; }
.strategy-editor__form { display: flex; flex-direction: column; gap: 14px; min-width: 0; }
.strategy-section { border: 1px solid var(--wt-border); border-radius: 10px; padding: 12px 14px; }
.strategy-section__title { display: flex; align-items: baseline; gap: 8px; margin-bottom: 10px; color: var(--wt-text-primary); font-size: 14px; font-weight: 600; }
.strategy-section__title small { color: var(--wt-text-tertiary); font-size: 12px; font-weight: 400; }
.strategy-no { color: var(--wt-primary); font-weight: 700; }
.strategy-editor__aside { min-width: 0; }
.strategy-aside { position: sticky; top: 12px; display: flex; flex-direction: column; gap: 12px; padding: 14px; border: 1px solid var(--wt-border); border-radius: 10px; background: var(--wt-bg-page); }
.strategy-aside__title { color: var(--wt-text-primary); font-size: 14px; font-weight: 600; }
.strategy-aside__item { display: flex; flex-direction: column; gap: 3px; }
.strategy-aside__item strong { color: var(--wt-text-primary); font-size: 13px; }
.strategy-aside__item span { color: var(--wt-text-tertiary); font-size: 12px; line-height: 1.55; }
.strategy-aside__tip { margin-top: 4px; padding: 10px; border-radius: 8px; background: var(--wt-info-bg); color: var(--wt-primary); font-size: 12px; line-height: 1.55; }
.strategy-editor__footer { display: flex; justify-content: flex-end; gap: 8px; margin-top: 16px; }
.strategy-field { display: flex; flex-direction: column; gap: 4px; width: 100%; }
.strategy-field-error { margin: 0; color: var(--td-error-color, #d54941); font-size: 12px; line-height: 1.4; }
.strategy-bind-hint { color: var(--wt-text-tertiary); font-size: 12px; line-height: 1.55; }
.strategy-rule { margin: 12px 0; padding: 12px; border: 1px solid var(--wt-border); border-radius: 8px; }
.strategy-rule__header { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-bottom: 12px; }
.strategy-rule__conditions { display: flex; flex-direction: column; gap: 8px; width: 100%; }
.strategy-rule__condition { display: grid; grid-template-columns: minmax(110px, 1fr) minmax(110px, 1fr) minmax(150px, 1.5fr) auto; gap: 8px; align-items: center; }
.strategy-rule__add { width: 100%; margin-bottom: 16px; }
.strategy-detail-section { margin: 24px 0 8px; padding-bottom: 8px; border-bottom: 1px solid var(--wt-border); color: var(--wt-text-primary); font-size: 14px; font-weight: 600; }
.strategy-drawer__hero { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; padding: 4px 0 16px; }
.strategy-drawer__hero span, .strategy-drawer__hero p { margin: 0; color: var(--wt-text-tertiary); font-size: 12px; }
.strategy-drawer__hero h3 { margin: 4px 0; color: var(--wt-text-primary); font-size: 20px; }
.strategy-drawer__field { display: grid; grid-template-columns: 110px minmax(0, 1fr); gap: 12px; padding: 8px 0; }
.strategy-drawer__field span { color: var(--wt-text-tertiary); font-size: 12px; }
.strategy-drawer__field strong { color: var(--wt-text-primary); font-size: 13px; font-weight: 500; overflow-wrap: anywhere; }
.strategy-drawer__actions { display: flex; justify-content: flex-end; margin-top: 12px; }
.strategy-readonly { display: block; padding: 7px 0; color: var(--wt-text-primary); font-size: 13px; font-weight: 500; }
.strategy-media { display: flex; flex-direction: column; gap: 8px; width: 100%; }
.strategy-media__card { display: flex; align-items: center; gap: 12px; padding: 8px 12px; border: 1px solid var(--wt-border); border-radius: var(--wt-radius-md); }
.strategy-media__card strong { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--wt-text-primary); }
.strategy-media__card span { color: var(--wt-text-tertiary); }
.strategy-media__card a { color: var(--wt-primary); }
</style>
