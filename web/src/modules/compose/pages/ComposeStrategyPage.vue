<script setup>
import '../production.css'
import ResourceAdvancedToggle from '../../../shared/ui/resource/ResourceAdvancedToggle.vue'
import ResourceRowActions from '../../../shared/ui/resource/ResourceRowActions.vue'
import ResourceSelectionToolbar from '../../../shared/ui/resource/ResourceSelectionToolbar.vue'
import BusinessSelect from "../../../shared/ui/fields/BusinessSelect.vue"
import { feedback } from '../../../shared/ui/feedback/index.js'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { DialogPlugin } from 'tdesign-vue-next'
import { createComposeClient } from '../../../shared/api/compose.js'
import { createSessionClient } from '../../../shared/api/session.js'
import ResourceCard from '../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../shared/ui/resource/ResourcePageHeader.vue'
import { opColumnWidth } from '../../../shared/ui/resource/opColumnWidth.js'
import { formatDateTime } from '../../../shared/utils/datetime.js'
import { enabledCategories } from '../../../shared/utils/categories.js'
import { useAutoRefresh } from '../../../shared/utils/useAutoRefresh.js'
import { settingFields, newSetting, strategySettings, validateSettings, platformText } from '../strategySettings.js'
import { TRIGGER_TYPE_OPTIONS, triggerLabel } from '../labels.js'
import { scheduleLabel } from '../scheduleForm.js'
import { defaultCondition, displayRules, saveRules } from '../strategyRules.js'
import { templateLabel } from '../strategyConfig.js'
import ComposeStrategySummary from '../components/ComposeStrategySummary.vue'
import ResourceSection from '../../../shared/ui/resource/ResourceSection.vue'
import ResourceForm from '../../../shared/ui/resource/ResourceForm.vue'
import ResourceFieldHelp from '../../../shared/ui/resource/ResourceFieldHelp.vue'
import ResourceDescriptions from '../../../shared/ui/resource/ResourceDescriptions.vue'
import ResourceOverlay from '../../../shared/ui/resource/ResourceOverlay.vue'
import ComposeReorderActions from '../components/ComposeReorderActions.vue'
import { draggedKey, reorderItems } from '../reorder.js'
import { preserveDraftScroll } from '../preserveDraftScroll.js'
import ComposeStrategySettings from '../components/ComposeStrategySettings.vue'
import ComposeStrategyExecutionPreview from '../components/ComposeStrategyExecutionPreview.vue'

const client = createComposeClient()
const route = useRoute()
const router = useRouter()
const sessionClient = createSessionClient()

const rows = ref([])
const templates = ref([])
const categories = ref([])
// 选择与筛选只列启用类目；名称解析仍读全量目录，停用类目的历史策略不显示裸 ID。
const enabledCategoryOptions = computed(() => enabledCategories(categories.value))
const teams = ref([])
const operatorOptions = ref([])
const operatorsLoading = ref(false)
const operatorError = ref('')
let operatorSequence = 0
const currentUser = ref(null)
const loading = ref(false)
const error = ref('')

// 与后端 canManageStrategies 同一条规则：启用状态下的高级运营或管理员能写，
// 普通运营只读。页面据此藏写入口；真正的授权判定在 Service 层，藏按钮只是少一次必失败请求。
const canManage = computed(() => ['senior_operator', 'admin'].includes(currentUser.value?.role))
const isAdmin = computed(() => currentUser.value?.role === 'admin')
// 高级运营的团队由服务端固定，管理员的团队来自表单。
const operatorTeamReady = computed(() => Boolean(isAdmin.value ? form.value.team_id : currentUser.value?.team_id))

const visible = ref(false)
const saving = ref(false)
const editingID = ref(null)
const form = ref(defaultForm())
const nextRunPreview = ref(null)
const nextRunPreviewError = ref('')
let previewSequence = 0
// 模板参数区由所选模板的 schema 驱动：需要哪些字段、叫什么名字、什么类型都来自
// schema（执行器 BasicCutV1() 发布），页面不写死字段清单。
const outputSettings = ref([])
const templateInfoVisible = ref(false)
const selectedTemplate = computed(() => templates.value.find((item) => item.id === form.value.compose_template_id))
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

const editorRoot = ref(null)
const ruleDropTarget = ref(null)
function updateRuleDraft(change) { preserveDraftScroll(editorRoot.value, change) }
function renumberRules() { form.value.priority_rules.forEach((rule, index) => { rule.priority = index + 1 }) }
function addRule() { updateRuleDraft(() => form.value.priority_rules.push({ priority: form.value.priority_rules.length + 1, action: 'match', logic: 'all', conditions: [defaultCondition()], _key: crypto.randomUUID() })) }
function removeRule(index) { updateRuleDraft(() => { form.value.priority_rules.splice(index, 1); renumberRules() }) }
function moveRule(index, direction) {
  const target = form.value.priority_rules[index + direction]
  if (!target) return
  updateRuleDraft(() => {
    form.value.priority_rules = reorderItems(form.value.priority_rules, form.value.priority_rules[index]._key, target._key, '_key')
    renumberRules()
  })
}
function dropRule(event, rule) {
  const source = draggedKey(event.dataTransfer, 'rules')
  if (source) updateRuleDraft(() => {
    form.value.priority_rules = reorderItems(form.value.priority_rules, source, rule._key, '_key')
    renumberRules()
  })
  ruleDropTarget.value = null
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
const detailTab = ref('overview')
const runPendingID = ref(null)
const statusPendingID = ref(null)
const runHistory = ref([])
const runHistoryError = ref('')
const runHistoryLoading = ref(false)
const runHistoryTotal = ref(0)
const runPagination = ref({ current: 1, pageSize: 10 })
const runFilters = ref({ trigger_type: '', created_range: [] })
const appliedRunFilters = ref({ trigger_type: '', created_range: [] })
let runHistorySequence = 0

function openRunBatch(run) {
  if (!run.batch_id) return
  detailVisible.value = false
  router.push({ name: 'Compose', query: { batch_id: String(run.batch_id) } })
}

async function loadRunHistory(strategyID) {
  const sequence = ++runHistorySequence
  runHistoryLoading.value = true
  runHistoryError.value = ''
  try {
    const params = { page: runPagination.value.current, page_size: runPagination.value.pageSize }
    if (appliedRunFilters.value.trigger_type) params.trigger_type = appliedRunFilters.value.trigger_type
    const [from, through] = appliedRunFilters.value.created_range || []
    if (from) params.created_from = new Date(`${from}T00:00:00`).toISOString()
    if (through) {
      const end = new Date(`${through}T00:00:00`)
      end.setDate(end.getDate() + 1)
      params.created_before = end.toISOString()
    }
    const data = await client.listStrategyRuns(strategyID, params)
    if (detailRow.value?.id === strategyID && sequence === runHistorySequence) {
      runHistory.value = data.items || []
      runHistoryTotal.value = data.total || 0
    }
  } catch (e) {
    if (detailRow.value?.id === strategyID && sequence === runHistorySequence) runHistoryError.value = e?.message || '运行记录读取失败'
  } finally {
    if (sequence === runHistorySequence) runHistoryLoading.value = false
  }
}

function applyRunFilters() {
  appliedRunFilters.value = { trigger_type: runFilters.value.trigger_type, created_range: [...(runFilters.value.created_range || [])] }
  runPagination.value.current = 1
  loadRunHistory(detailRow.value.id)
}

function resetRunFilters() {
  runFilters.value = { trigger_type: '', created_range: [] }
  applyRunFilters()
}

function onRunPageChange() { loadRunHistory(detailRow.value.id) }
function openRunTask(task) { router.push({ name: 'Compose', query: { task_id: String(task.id) } }) }

function openDetail(row) {
  detailRow.value = row
  detailTab.value = 'overview'
  runHistory.value = []
  runHistoryTotal.value = 0
  runPagination.value = { current: 1, pageSize: 10 }
  runFilters.value = { trigger_type: '', created_range: [] }
  appliedRunFilters.value = { trigger_type: '', created_range: [] }
  detailVisible.value = true
  loadRunHistory(row.id)
}

// 列表总数与当前页均由服务端按权限和筛选计算。
const advancedFiltersVisible = ref(false)
const filters = ref({ search: '', category_id: '', template_id: '', status: '' })
const pagination = ref({ current: 1, pageSize: 20 })
const total = ref(0)
let loadSequence = 0
function query() { pagination.value.current = 1; load() }
function onPageChange() { load() }
function onPageSizeChange() { pagination.value.current = 1; load() }

const columns = computed(() => [
  { colKey: 'id', title: 'ID', width: 70 },
  { colKey: 'name', title: '策略名称', width: 180 },
  { colKey: 'template', title: '合成模板', width: 115 },
  { colKey: 'category', title: '类目', width: 85 },
  ...(isAdmin.value ? [{ colKey: 'team', title: '团队', width: 100 }] : []),
  { colKey: 'operator', title: '负责人', width: 90 },
  { colKey: 'schedule', title: '执行计划', width: 180 },
  { colKey: 'status', title: '状态', width: 80 },
  { colKey: 'op', title: '操作', width: opColumnWidth(canManage.value ? ['详情', '编辑', '立即执行', '复制'] : ['详情'], { variant: 'text', primary: canManage.value, loading: canManage.value }), fixed: 'right' },
])

// 横向滚动宽度由列宽推导，不写死（写死值与列宽合计无关，是个漂移源）。
const tableScroll = computed(() => ({
  x: `${columns.value.reduce((sum, col) => sum + (col.width || 0), 0)}px`,
}))

const pagedRows = computed(() => rows.value)

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
  return settingFields(schema)
})


onMounted(() => {
  load()
  const id = String(route.query.strategy_id || '')
  if (/^\d+$/.test(id)) client.getStrategy(Number(id)).then(openDetail).catch((e) => feedback.toast.error(e?.message || '打开策略失败'))
})

// quiet 由定时刷新传入：保留页码与筛选，失败沿用上一次读数；会话只读一次。
async function load({ quiet = false } = {}) {
  const sequence = ++loadSequence
  if (!quiet) loading.value = true
  if (!quiet) error.value = ''
  try {
    // 团队列表所有角色都可读（服务端仅要求登录），编辑/详情要把 team_id 映射成团队名；
    // 管理员的创建表单另用它选目标团队。
    if (!quiet || !currentUser.value) currentUser.value = await sessionClient.me()
    const [strategyRows, templateRows, categoryRows, teamRows] = await Promise.all([
      client.listStrategies({ search: filters.value.search.trim(), category_id: filters.value.category_id, template_id: filters.value.template_id, status: filters.value.status, page: pagination.value.current, page_size: pagination.value.pageSize }),
      client.listTemplates(), client.listCategories(), client.listTeams().catch(() => []),
    ])
    if (sequence !== loadSequence) return
    rows.value = strategyRows.items || []
    total.value = strategyRows.total || 0
    templates.value = Array.isArray(templateRows) ? templateRows : []
    categories.value = Array.isArray(categoryRows) ? categoryRows : []
    teams.value = Array.isArray(teamRows) ? teamRows : []
  } catch (e) {
    if (sequence === loadSequence && !quiet) error.value = e.message || '合成策略加载失败'
  } finally {
    if (sequence === loadSequence) loading.value = false
  }
}

useAutoRefresh(() => load({ quiet: true }), {
  shouldSkip: () => visible.value || saving.value || copyVisible.value || copySaving.value || detailVisible.value || Boolean(runPendingID.value) || Boolean(statusPendingID.value) || executionVisible.value,
})

function defaultForm() {
  return {
    team_id: '',
    category_id: '',
    operator_id: '',
    name: '',
    compose_template_id: '',
    configFallback: null,
    execution_mode: 'manual',
    schedule: { mode: 'interval', interval_minutes: 30, daily_time: '09:00', timezone: 'Asia/Shanghai' },
    priority_rules: [{ priority: 1, action: 'match', logic: 'all', conditions: [defaultCondition()], _key: crypto.randomUUID() }],
    single_limit: 1,
    daily_limit: 1,
  }
}

// 负责人候选只由团队＋类目决定（服务端同一条有效性规则）。编辑既有策略时团队和类目
// 不可改，直接用行上的值；新建时管理员先选团队、所有人先选类目。
async function loadOperatorOptions() {
  const sequence = ++operatorSequence
  operatorOptions.value = []
  const teamID = isAdmin.value ? Number(form.value.team_id) || 0 : Number(currentUser.value?.team_id) || 0
  const categoryID = form.value.category_id
  operatorError.value = ''
  if (!teamID || !categoryID) {
    operatorOptions.value = []
    operatorsLoading.value = false
    return
  }
  operatorsLoading.value = true
  try {
    const options = await client.listComposeOperators({ team_id: teamID, category_id: categoryID })
    if (sequence !== operatorSequence) return
    operatorOptions.value = Array.isArray(options) ? options : []
    // 原来选的人已不在候选里（停用／改团队／不再负责该类目）就不再声称已指派。
    if (form.value.operator_id && !operatorOptions.value.some((option) => option.id === Number(form.value.operator_id))) form.value.operator_id = ''
  } catch (e) {
    if (sequence !== operatorSequence) return
    operatorOptions.value = []
    operatorError.value = e?.message || '负责人读取失败'
  } finally {
    if (sequence === operatorSequence) operatorsLoading.value = false
  }
}

watch(() => [visible.value, form.value.team_id, form.value.category_id], () => {
  if (visible.value) loadOperatorOptions()
})

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
  filters.value = { search: '', category_id: '', template_id: '', status: '' }
  pagination.value.current = 1
  load()
}

function openCreate() {
  editingID.value = null
  form.value = defaultForm()
  operatorOptions.value = []
  operatorError.value = ''
  outputSettings.value = []
  clearParamErrors()
  nameError.value = ''
  beginSnapshot()
  visible.value = true
}

function openEdit(row) {
  editingID.value = row.id
  form.value = {
    team_id: row.team_id,
    category_id: row.category_id,
    operator_id: row.operator_id || '',
    name: row.name,
    compose_template_id: row.compose_template_id,
    configFallback: templates.value.some((item) => item.id === row.compose_template_id) ? null : row.config,
    execution_mode: row.execution_mode || 'manual',
    schedule: { mode: row.schedule?.mode || 'interval', interval_minutes: row.schedule?.interval_minutes || 30, daily_time: row.schedule?.daily_time || '09:00', timezone: row.schedule?.timezone || 'Asia/Shanghai' },
    priority_rules: displayRules(row.priority_rules || []).map((rule) => ({ ...rule, _key: crypto.randomUUID() })),
    single_limit: row.single_limit || 1,
    daily_limit: row.daily_limit || 1,
  }
  outputSettings.value = strategySettings(row)
  clearParamErrors()
  nameError.value = ''
  beginSnapshot()
  visible.value = true
}

function beginSnapshot() {
  savedSnapshot.value = editorState()
}

function editorState() {
  return JSON.stringify([editingID.value, form.value, outputSettings.value])
}

function editorDirty() {
  return editorState() !== savedSnapshot.value
}

// 选了模板（或换模板）后按该模板 schema 重置参数区：整数字段给 schema 下限（无下限给 0），
// 字符串给空串，布尔给 false。
function onTemplateChange() {
  outputSettings.value = form.value.compose_template_id ? [newSetting(formSchemaFields.value)] : []
  clearParamErrors()
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

const executionVisible = ref(false)
const executionLoading = ref(false)
const executionView = ref(null)
const executionError = ref('')
const executionStrategy = ref(null)
const executionRequestKey = ref('')
const canExecutePreview = computed(() => Boolean(executionView.value?.preview_token && executionView.value?.selected_count > 0 && !executionView.value?.active_batch && !executionLoading.value && !runPendingID.value))
let executionSequence = 0
async function runNow(row) {
  if (runPendingID.value !== null || row.status !== 'enabled') return
  executionStrategy.value = row
  executionRequestKey.value = crypto.randomUUID()
  executionView.value = null
  executionVisible.value = true
  await refreshExecutionPreview()
}
async function refreshExecutionPreview() {
  const sequence = ++executionSequence
  executionLoading.value = true
  executionError.value = ''
  executionView.value = null
  try {
    const view = await client.previewStrategyExecution(executionStrategy.value.id)
    if (sequence === executionSequence) executionView.value = view
  } catch (e) {
    if (sequence === executionSequence) executionError.value = e?.message || '执行预览读取失败'
  } finally { if (sequence === executionSequence) executionLoading.value = false }
}
async function confirmExecution() {
  if (!canExecutePreview.value) return
  const row = executionStrategy.value
  runPendingID.value = row.id
  executionError.value = ''
  try {
    const result = await client.runStrategyNow(row.id, executionRequestKey.value, executionView.value.preview_token)
    executionVisible.value = false
    feedback.toast.success(`执行已提交，创建 ${result.created_count} 个合成任务`)
    if (detailRow.value?.id === row.id) await loadRunHistory(row.id)
    await load({ quiet: true })
  } catch (e) {
    const changed = ['compose_preview_changed', 'compose_execution_preview_changed', 'compose_preview_expired'].includes(e?.code)
    if (changed || e?.code === 'compose_already_running') {
      await refreshExecutionPreview()
      executionError.value = changed ? '素材或配置已变化，请检查最新预览后重新确认' : ''
      executionRequestKey.value = crypto.randomUUID()
    } else executionError.value = e?.message || '立即执行失败'
  } finally { runPendingID.value = null }
}
function openExecutionProgress(batch) {
  executionVisible.value = false
  router.push({ name: 'Compose', query: { batch_id: String(batch.id) } })
}

// 启停开关在列表行上（走查修订第 3 条）。开关受控于 row.status，确认后才真正变更；
// 取消时不动数据，下一次渲染自然回弹。
function toggleStatus(row, nextEnabled) {
  if (statusPendingID.value !== null) return
  const next = nextEnabled ? 'enabled' : 'disabled'
  const dialog = DialogPlugin.confirm({
    header: next === 'enabled' ? '启用策略' : '停用策略',
    body: next === 'enabled'
      ? `即将启用「${row.name}」。启用后可立即执行${row.execution_mode === 'scheduled' ? '，也会按计划创建任务' : ''}。`
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
  if (!editingID.value && isAdmin.value && !form.value.team_id) { feedback.toast.error('请选择运营团队'); return }
  if (!editingID.value && !form.value.category_id) { feedback.toast.error('请选择业务分类'); return }
  if (!editingID.value && !form.value.compose_template_id) {
    feedback.toast.error('请选择合成模板')
    return
  }
  if (!form.value.operator_id) {
    operatorError.value = '请选择负责人'
    return
  }
  const fields = formSchemaFields.value
  if (!fields.length) {
    feedback.toast.error('模板参数未就绪，请重新选择合成模板')
    return
  }
  paramErrors.value = validateSettings(outputSettings.value, fields)
  if (Object.keys(paramErrors.value).length) return
  const settings = outputSettings.value.map((group) => ({ ...group, name: group.name.trim(), params: Object.fromEntries(fields.map((field) => [field.key, typeof group.params[field.key] === 'string' ? group.params[field.key].trim() : group.params[field.key]])) }))
  const config = settings[0].params
  const singleLimit = Number(form.value.single_limit)
  const dailyLimit = Number(form.value.daily_limit)
  if (!Number.isInteger(singleLimit) || singleLimit < 1 || singleLimit > 100 || (form.value.execution_mode === 'scheduled' && (!Number.isInteger(dailyLimit) || dailyLimit < singleLimit || dailyLimit > 1000))) {
    feedback.toast.error(form.value.execution_mode === 'scheduled' ? '每次上限需为 1～100，每日上限需不小于每次上限且不超过 1000' : '每次上限需为 1～100')
    return
  }
  if (!form.value.priority_rules.some((rule) => rule.action === 'match')) {
    feedback.toast.error('请至少添加一个选入优先级')
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
    priority_rules: saveRules(form.value.priority_rules.map(({ _key, ...rule }) => rule)),
    single_limit: singleLimit,
    ...(form.value.execution_mode === 'scheduled' ? { daily_limit: dailyLimit } : {}),
    output_settings: settings,
  }
  saving.value = true
  try {
    if (editingID.value) {
      await client.updateStrategy(editingID.value, { name, config, operator_id: Number(form.value.operator_id), ...execution })
    } else {
      const template = templates.value.find((item) => item.id === Number(form.value.compose_template_id))
      const payload = {
        category_id: form.value.category_id,
        name,
        operator_id: Number(form.value.operator_id),
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

function categoryName(id) {
  return categories.value.find((category) => category.id === id)?.name || id
}

function teamName(id) {
  return teams.value.find((team) => team.id === id)?.name || `团队 ${id}`
}

function conditionParts(condition) {
  const field = conditionFields.find((item) => item.value === condition.field)?.label || condition.field
  const operator = conditionOperators(condition.field).find((item) => item.value === condition.operator)?.label || condition.operator
  const value = condition.field === 'published_at' && !['recent_hours', 'recent_days'].includes(condition.operator)
    ? formatDateTime(condition.value)
    : ['recent_hours', 'recent_days'].includes(condition.operator)
      ? `${condition.value} ${condition.operator === 'recent_hours' ? '小时' : '天'}`
      : condition.value
  return { field, operator, value }
}

function orderedRules(row) { return [...(row.priority_rules || [])].sort((a, b) => a.priority - b.priority) }

const runColumns = [
  { colKey: 'created_at', title: '执行时间', width: 155 },
  { colKey: 'trigger_type', title: '触发方式', width: 90 },
  { colKey: 'candidate_count', title: '候选数', width: 70 },
  { colKey: 'skipped_count', title: '跳过数', width: 70 },
  { colKey: 'created_count', title: '创建任务数', width: 95 },
  { colKey: 'failed_count', title: '失败数', width: 65 },
  { colKey: 'op', title: '操作', width: 120 },
]
const runTableScroll = computed(() => ({ x: `${runColumns.reduce((sum, column) => sum + column.width, 0)}px` }))

</script>

<template>
  <t-loading :loading="loading" :show-overlay="true">
    <div class="wt-resource-page resource-visual-approved production-page strategy-page">
      <FeedbackAlert v-if="error" theme="error" :message="error" closable style="margin-bottom:16px" @close="error=''" />
      <ResourcePageHeader title="合成策略" description="设置制作方法、选材条件和执行时间。">
        <template #actions>
          <t-button v-if="canManage" theme="primary" @click="openCreate">新增策略</t-button>
        </template>
      </ResourcePageHeader>

      <!-- 筛选区与表格同卡（与内容池一致）：filter-bar 自带分隔线，下方接表格与分页。 -->
      <ResourceCard class="strategy-card">
        <div class="filter-bar">
          <label class="wt-filter-field wt-filter-field--wide"><span class="wt-filter-field__label">综合搜索</span><t-input v-model="filters.search" clearable placeholder="策略名称 / ID" @enter="query" /></label>
          <label class="wt-filter-field"><span class="wt-filter-field__label">所属类目</span><BusinessSelect kind="category" filter v-model="filters.category_id" clearable>
            <t-option v-for="category in enabledCategoryOptions" :key="category.id" :value="category.id" :label="category.name" />
          </BusinessSelect></label>

          <label class="wt-filter-field"><span class="wt-filter-field__label">状态</span><t-select v-model="filters.status" clearable placeholder="全部">
            <t-option value="enabled" label="启用" />
            <t-option value="disabled" label="停用" />
          </t-select></label>
          <div class="wt-filter-actions">
            <t-button theme="primary" @click="query">查询</t-button>
            <t-button class="wt-secondary-button" variant="outline" @click="resetFilters">重置</t-button>
            </div>
          <ResourceAdvancedToggle v-model="advancedFiltersVisible" />
          <div v-if="advancedFiltersVisible" class="wt-advanced-filters">
          <label class="wt-filter-field"><span class="wt-filter-field__label">合成模板</span><t-select v-model="filters.template_id" clearable placeholder="全部">
            <t-option v-for="template in templates" :key="template.id" :value="template.id" :label="`${template.name} v${template.version}`" />
          </t-select></label>
          </div>
        </div>
        <div class="table-scroll-wrap">
          <t-table class="wt-resource-table" :data="pagedRows" :columns="columns" row-key="id" hover size="small" :scroll="tableScroll" empty="暂无合成策略">
            <template #id="{ row }"><span class="strategy-id">{{ row.id }}</span></template>
            <template #name="{ row }">
              <div class="strategy-name-cell">
                <span class="strategy-name-cell__main" :title="row.name">{{ row.name }}</span><span class="strategy-name-cell__sub">{{ strategySettings(row).map(group => `${group.name} × ${group.copies}`).join('、') }}</span>
              </div>
            </template>
            <template #category="{ row }">{{ categoryName(row.category_id) }}</template>
            <template #team="{ row }">{{ teamName(row.team_id) }}</template>
            <!-- 历史策略没有负责人来源：显示“未记录”，不拿创建人冒充。 -->
            <template #operator="{ row }">{{ row.operator_name || '未记录' }}</template>
            <template #template="{ row }">{{ templateLabel(templates, row.compose_template_id) }}</template>
            <template #status="{ row }">
              <t-switch v-if="canManage" :value="row.status === 'enabled'" :label="['启用', '停用']" :loading="statusPendingID === row.id" :disabled="statusPendingID !== null" @change="(value) => toggleStatus(row, value)" />
              <t-switch v-else :value="row.status === 'enabled'" :label="['启用', '停用']" disabled title="当前角色只能查看策略状态" />
            </template>
            <template #schedule="{ row }"><div class="strategy-schedule-cell"><span class="strategy-schedule-cell__mode">{{ scheduleState(row) }}</span><span>每次 {{ row.single_limit }}<template v-if="row.execution_mode === 'scheduled'"> / 每日 {{ row.daily_limit }}</template> 条素材</span><span>{{ (row.priority_rules || []).filter(rule => rule.action === 'filter').length }} 条排除 · {{ (row.priority_rules || []).filter(rule => rule.action === 'match').length }} 条选入</span></div></template>
            <template #op="{ row }">
              <ResourceRowActions>
                <t-button size="small" class="wt-secondary-button" variant="text" @click="openDetail(row)">详情</t-button>
                <template v-if="canManage">
                  <t-button size="small" theme="primary" variant="text" @click="openEdit(row)">编辑</t-button>
                  <t-button size="small" class="is-primary" variant="text" :loading="runPendingID === row.id" :disabled="row.status !== 'enabled' || runPendingID !== null" :title="row.status !== 'enabled' ? '策略已停用，不能执行' : ''" @click="runNow(row)">立即执行</t-button>
                  <t-button size="small" class="wt-secondary-button" variant="text" @click="openCopy(row)">复制</t-button>
                </template>
              </ResourceRowActions>
            </template>
          </t-table>
        </div>
        <div class="pagination-bar"><t-pagination :show-jumper="false" v-model:current="pagination.current" v-model:pageSize="pagination.pageSize" :total="total" :page-size-options="[10, 20, 50]" @current-change="onPageChange" @page-size-change="onPageSizeChange" /></div>
      </ResourceCard>

      <ResourceOverlay kind="drawer" :visible="visible" :title="editingID ? '编辑合成策略' : '新增合成策略'" :width="960" :busy="saving" @close="requestCloseEditor">
        <div ref="editorRoot" class="strategy-editor">
          <ResourceSection title="基本信息" variant="form">
            <ResourceForm class="resource-form-grid">
              <t-form-item label="策略名称" required-mark><div class="strategy-field"><t-input v-model="form.name" :maxlength="120" placeholder="输入策略名称" :status="nameError ? 'error' : undefined" /><p v-if="nameError" class="strategy-field-error">{{ nameError }}</p></div></t-form-item>
              <t-form-item label="所属类目" required-mark>
                <BusinessSelect v-if="!editingID" kind="category" v-model="form.category_id" clearable @change="form.operator_id = ''"><t-option v-for="category in enabledCategoryOptions" :key="category.id" :value="category.id" :label="category.name" /></BusinessSelect>
                <span v-else class="resource-form-value">{{ categoryName(form.category_id) }}</span>
              </t-form-item>
              <t-form-item required-mark>
                <template #label><span class="strategy-template-label">合成模板<ResourceFieldHelp content="模板决定制作方式。保存后如需更换模板，请复制策略。" /><t-link v-if="form.compose_template_id" theme="primary" @click="templateInfoVisible = true">模板说明 ↗</t-link></span></template>
                <t-select v-if="!editingID" v-model="form.compose_template_id" clearable placeholder="选择合成模板" @change="onTemplateChange"><t-option v-for="option in templateOptions" :key="option.value" :value="option.value" :label="option.label" /></t-select>
                <span v-else class="resource-form-value">{{ templateLabel(templates, form.compose_template_id) }}</span>
              </t-form-item>
              <t-form-item label="负责人" required-mark><div class="strategy-field"><BusinessSelect kind="member" v-model="form.operator_id" clearable :loading="operatorsLoading" :disabled="operatorsLoading || !form.category_id || !operatorTeamReady" :placeholder="!form.category_id ? '先选择类目' : (!operatorTeamReady ? '先选择团队' : '选择负责人')" :status="operatorError ? 'error' : undefined" @change="operatorError = ''"><t-option v-for="option in operatorOptions" :key="option.id" :value="option.id" :label="option.display_name" /></BusinessSelect><p v-if="operatorError" class="strategy-field-error">{{ operatorError }}</p><p v-else-if="form.category_id && operatorTeamReady && !operatorsLoading && !operatorOptions.length" class="strategy-bind-hint">该团队与类目下没有可指派的运营</p></div></t-form-item>
              <t-form-item v-if="isAdmin" label="运营团队" :required-mark="!editingID"><BusinessSelect v-if="!editingID" kind="team" v-model="form.team_id" clearable @change="form.operator_id = ''"><t-option v-for="team in teams" :key="team.id" :value="team.id" :label="team.name" /></BusinessSelect><span v-else class="resource-form-value">{{ teamName(form.team_id) }}</span></t-form-item>
            </ResourceForm>
          </ResourceSection>
          <ResourceSection title="默认成片设置" variant="form">
            <p v-if="!form.compose_template_id" class="strategy-bind-hint">请先选择合成模板</p>
            <template v-else><p class="strategy-section-note">各组独立保存参数，新增带入模板默认值。</p><ComposeStrategySettings v-model="outputSettings" :fields="formSchemaFields" :errors="paramErrors" /></template>
          </ResourceSection>
          <ResourceSection title="执行设置" variant="form">
            <ResourceForm class="resource-form-grid">
              <t-form-item required-mark><template #label>执行方式<ResourceFieldHelp :content="form.execution_mode === 'scheduled' ? '按计划自动执行；也可点击立即执行。保存不会立即开始。' : '点击立即执行，确认素材和预计成片数后开始。'" /></template><t-select v-model="form.execution_mode" :options="[{value:'manual',label:'手动执行'},{value:'scheduled',label:'定时执行'}]" /></t-form-item>
              <t-form-item label="每次素材上限" required-mark><t-input-number v-model="form.single_limit" :min="1" :max="100" suffix="条" theme="column" /></t-form-item>
              <template v-if="form.execution_mode === 'scheduled'">
                <t-form-item label="计划类型" required-mark><t-select v-model="form.schedule.mode" :options="[{value:'daily',label:'每天固定时间'},{value:'interval',label:'固定间隔'}]" /></t-form-item>
                <t-form-item v-if="form.schedule.mode === 'interval'" label="间隔分钟" required-mark><t-input-number v-model="form.schedule.interval_minutes" :min="1" :max="1440" suffix="分钟" theme="column" /></t-form-item>
                <t-form-item v-else label="每日执行时间" required-mark><t-time-picker v-model="form.schedule.daily_time" format="HH:mm" /></t-form-item>
                <t-form-item label="每日素材上限" required-mark><t-input-number v-model="form.daily_limit" :min="1" :max="1000" suffix="条" theme="column" /></t-form-item>
                <t-form-item required-mark><template #label>时区<ResourceFieldHelp content="使用 IANA 时区名称，例如 Asia/Shanghai、UTC。" /></template><t-input v-model="form.schedule.timezone" placeholder="Asia/Shanghai" /></t-form-item>
                <t-form-item label="下次执行"><span class="resource-form-value">{{ nextRunPreview ? formatDateTime(nextRunPreview) : nextRunPreviewError || '填写计划后计算' }}</span></t-form-item>
              </template>
            </ResourceForm>
          </ResourceSection>
          <ResourceSection title="选材条件" variant="form">
            <template #title-extra><ResourceFieldHelp content="先排除，再按优先级选入。缺少必要指标或已在本策略创建过任务的素材将跳过。" /></template>
            <template #default>
              <div v-for="(rule, index) in form.priority_rules" :key="rule._key" class="strategy-rule" :class="{ 'is-drop-target': ruleDropTarget === rule._key }" @dragover.prevent="ruleDropTarget = rule._key" @dragleave.self="ruleDropTarget = null" @drop.prevent="dropRule($event, rule)">
                <ComposeReorderActions :item-key="rule._key" scope="rules" :index="index" :count="form.priority_rules.length" @move="moveRule(index, $event)" @remove="removeRule(index)" @dragend="ruleDropTarget = null">{{ rule.action === 'filter' ? '排除规则' : '选入优先级' }} {{ rule.priority }}</ComposeReorderActions>
                <ResourceForm class="resource-form-grid">
                  <t-form-item label="命中后" required-mark><t-select v-model="rule.action" :options="[{value:'match',label:'选入'},{value:'filter',label:'排除'}]" /></t-form-item>
                  <t-form-item label="条件关系" required-mark><t-select v-model="rule.logic" :options="[{value:'all',label:'全部满足'},{value:'any',label:'任一满足'}]" /></t-form-item>
                  <div class="strategy-rule__conditions resource-form-full">
                    <div v-for="(condition, conditionIndex) in rule.conditions" :key="conditionIndex" class="strategy-rule__condition">
                      <t-form-item label="指标"><t-select v-model="condition.field" :options="conditionFields" @change="changeConditionField(condition)" /></t-form-item>
                      <t-form-item label="比较"><t-select v-model="condition.operator" :options="conditionOperators(condition.field)" @change="changeConditionOperator(condition)" /></t-form-item>
                      <t-form-item label="数值"><t-date-picker v-if="condition.field === 'published_at' && ['gte', 'lt'].includes(condition.operator)" v-model="condition.value" enable-time-picker value-type="time-stamp" format="YYYY-MM-DD HH:mm:ss" clearable /><t-input-number v-else v-model="condition.value" :min="['recent_hours','recent_days'].includes(condition.operator) ? 1 : 0" :max="condition.operator === 'recent_hours' ? 8760 : condition.operator === 'recent_days' ? 365 : undefined" theme="column" /></t-form-item>
                      <t-button size="small" variant="text" :disabled="rule.conditions.length === 1" @click="updateRuleDraft(() => rule.conditions.splice(conditionIndex, 1))">移除条件</t-button>
                    </div>
                    <t-button size="small" theme="primary" variant="text" @click="updateRuleDraft(() => rule.conditions.push(defaultCondition()))">添加条件</t-button>
                  </div>
                </ResourceForm>
              </div>
              <t-button variant="outline" @click="addRule">添加优先级规则</t-button>
            </template>
          </ResourceSection>
        </div>
        <template #footer><t-button class="wt-secondary-button" variant="outline" :disabled="saving" @click="requestCloseEditor">取消</t-button><t-button theme="primary" :loading="saving" @click="save">保存</t-button></template>
      </ResourceOverlay>

      <ResourceOverlay v-model:visible="templateInfoVisible" title="模板说明" :width="720" :footer="false">
        <strong>{{ templateLabel(templates, form.compose_template_id) }}</strong>
        <p>{{ selectedTemplate?.description || selectedTemplate?.schema?.description || '此版本暂未提供用途说明' }}</p>
        <div v-for="field in formSchemaFields" :key="field.key" class="strategy-overview__rule">
          <strong>{{ field.label }}</strong><p>{{ field.description || '暂无参数说明' }}</p>
          <span>{{ field.required ? '必填' : '选填' }} · {{ field.editable === true ? '任务内可调整' : '任务内固定' }}<template v-if="field.minimum !== undefined"> · 最小 {{ field.minimum }}</template><template v-if="field.maximum !== undefined"> · 最大 {{ field.maximum }}</template><template v-if="field.default !== undefined"> · 默认 {{ field.default }}</template></span>
        </div>
      </ResourceOverlay>
      <ResourceOverlay :visible="executionVisible" title="立即执行预览" :width="800" :busy="!!runPendingID" @close="!runPendingID && (executionVisible = false)">
        <FeedbackAlert v-if="executionError" theme="error" :message="executionError" />
        <t-loading :loading="executionLoading" :show-overlay="true"><div style="min-height:100px"><ComposeStrategyExecutionPreview v-if="executionView" :view="executionView" @progress="openExecutionProgress" /></div></t-loading>
        <template #footer>
          <t-button class="wt-secondary-button" variant="outline" :disabled="!!runPendingID" @click="executionVisible = false">关闭</t-button>
          <t-button v-if="executionError" variant="outline" :disabled="!!runPendingID || executionLoading" @click="refreshExecutionPreview">重新预览</t-button>
          <t-button v-if="executionView?.selected_count && !executionView?.active_batch" theme="primary" :disabled="!canExecutePreview" :loading="!!runPendingID" @click="confirmExecution">确认执行</t-button>
        </template>
      </ResourceOverlay>

      <ResourceOverlay v-model:visible="copyVisible" title="复制合成策略" :width="560" :busy="copySaving">
        <ResourceForm>
          <t-form-item label="新策略名称" required-mark><t-input v-model="copyName" :maxlength="120" placeholder="输入新策略名称" @enter="submitCopy" /></t-form-item>
          <t-form-item v-if="copyTarget" label="模板"><t-input :value="templateLabel(templates, copyTarget.compose_template_id)" disabled /></t-form-item>
        </ResourceForm>
        <template #footer>
          <t-button class="wt-secondary-button" variant="outline" @click="copyVisible = false">取消</t-button>
          <t-button theme="primary" :loading="copySaving" @click="submitCopy">复制</t-button>
        </template>
      </ResourceOverlay>

      <ResourceOverlay  v-model:visible="detailVisible" :title="detailRow?.name || '策略详情'" kind="drawer" :width="880" :footer="canManage">
        <t-tabs v-if="detailRow" v-model="detailTab" class="strategy-detail-tabs">
          <t-tab-panel value="overview" label="策略信息">
            <div class="strategy-overview">
              <ComposeStrategySummary
                overview
                :show-config="false"
                :strategy="detailRow"
                :team="teamName(detailRow.team_id)"
                :category="categoryName(detailRow.category_id)"
                :templates="templates"

              />
              <section class="strategy-overview__section">
                <h4>默认成片设置</h4>
                <div v-for="setting in strategySettings(detailRow)" :key="setting.key" class="strategy-overview__rule">
                  <div class="strategy-overview__rule-head"><strong>{{ setting.name }}</strong><span>{{ platformText(setting.platforms) }}</span><span>每条 {{ setting.copies }} 份</span></div>
                  <div v-for="(value, key) in setting.params" :key="key" class="strategy-overview__field"><span>{{ settingFields(templates.find((item) => item.id === detailRow.compose_template_id)?.schema).find((field) => field.key === key)?.label || key }}</span><strong>{{ value }}</strong></div>
                </div>
              </section>
              <section class="strategy-overview__section">
                <h4>执行设置</h4>
                <div class="strategy-overview__grid">
                  <div class="strategy-overview__field"><span>执行方式</span><strong>{{ detailRow.execution_mode === 'scheduled' ? '定时执行' : '手动执行' }}</strong></div>
                  <template v-if="detailRow.execution_mode === 'scheduled'">
                    <div class="strategy-overview__field"><span>执行计划</span><strong>{{ scheduleLabel(detailRow.schedule) }}</strong></div>
                    <div class="strategy-overview__field"><span>时区</span><strong>{{ detailRow.schedule?.timezone || '—' }}</strong></div>
                    <div class="strategy-overview__field"><span>下次执行</span><strong>{{ detailRow.next_run_at ? formatDateTime(detailRow.next_run_at) : '—' }}</strong></div>
                  </template>
                </div>
              </section>
              <section class="strategy-overview__section">
                <h4>执行条件</h4>
                <div class="strategy-overview__grid">
                  <div class="strategy-overview__field strategy-overview__field--wide"><span>候选范围</span><strong>当前策略业务范围内可生产素材</strong></div>
                  <div class="strategy-overview__field"><span>单次上限</span><strong>{{ detailRow.single_limit }} 条</strong></div>
                  <div v-if="detailRow.execution_mode === 'scheduled'" class="strategy-overview__field"><span>每日上限</span><strong>{{ detailRow.daily_limit }} 条</strong></div>
                </div>
                <div v-for="group in orderedRules(detailRow)" :key="group.priority" class="strategy-overview__rule">
                  <div class="strategy-overview__rule-head"><strong>优先级 {{ group.priority }}</strong><span>{{ group.action === 'filter' ? '排除' : '选入' }}</span><span>{{ group.logic === 'all' ? '全部满足' : '任一满足' }}</span></div>
                  <div v-for="(condition, index) in group.conditions || []" :key="index" class="strategy-overview__condition">
                    <span>{{ conditionParts(condition).field }}</span><span>{{ conditionParts(condition).operator }}</span><strong>{{ conditionParts(condition).value }}</strong>
                  </div>
                </div>
              </section>
            </div>
          </t-tab-panel>
          <t-tab-panel value="runs" label="执行记录">
            <div class="strategy-history">
              <div class="strategy-history__filters">
                <label class="wt-filter-field"><span class="wt-filter-field__label">触发方式</span><t-select v-model="runFilters.trigger_type" :options="TRIGGER_TYPE_OPTIONS" clearable placeholder="全部" /></label>
                <label class="wt-filter-field"><span class="wt-filter-field__label">执行时间</span><t-date-range-picker v-model="runFilters.created_range" value-type="YYYY-MM-DD" clearable /></label>
                <div class="strategy-history__actions"><t-button theme="primary" @click="applyRunFilters">查询</t-button><t-button class="wt-secondary-button" variant="outline" @click="resetRunFilters">重置</t-button></div>
              </div>
              <p v-if="runHistoryError" class="strategy-run-message">{{ runHistoryError }} <t-link theme="primary" @click="loadRunHistory(detailRow.id)">重试</t-link></p>
              <t-loading v-else :loading="runHistoryLoading" :show-overlay="true">
                <div class="table-scroll-wrap">
                  <t-table class="wt-resource-table" :data="runHistory" :columns="runColumns" row-key="run_key" hover size="small" :scroll="runTableScroll" empty="暂无运行记录">
                    <template #created_at="{ row }">{{ formatDateTime(row.created_at) }}</template>
                    <template #trigger_type="{ row }">{{ triggerLabel(row.trigger_type) }}</template>
                    <template #skipped_count="{ row }">{{ row.skipped?.length ?? 0 }}</template>
                    <template #op="{ row }"><t-link v-if="row.batch_id" theme="primary" @click="openRunBatch(row)">详情</t-link><span v-else>—</span></template>
                  </t-table>
                </div>
                <div class="pagination-bar"><t-pagination :show-jumper="false" v-model:current="runPagination.current" v-model:pageSize="runPagination.pageSize" :total="runHistoryTotal" :page-size-options="[10, 20, 50]" @current-change="onRunPageChange" @page-size-change="onRunPageChange" /></div>
              </t-loading>
            </div>
          </t-tab-panel>
        </t-tabs>
        <template #footer>
          <div v-if="detailRow && canManage" class="strategy-detail__footer">
            <t-button class="wt-secondary-button" variant="outline" @click="openCopy(detailRow)">复制</t-button>
            <t-button class="wt-secondary-button" variant="outline" @click="openEdit(detailRow)">编辑</t-button>
            <span :title="detailRow.status !== 'enabled' ? '策略已停用，请先在策略列表启用后执行。' : ''"><t-button theme="primary" :disabled="detailRow.status !== 'enabled' || runPendingID !== null" :loading="runPendingID === detailRow.id" @click="runNow(detailRow)">立即执行</t-button></span>
          </div>
        </template>
      </ResourceOverlay>
    </div>
  </t-loading>
</template>

<style scoped>
.strategy-name-cell, .strategy-schedule-cell { display: flex; flex-direction: column; gap: 5px; min-width: 0; }
.strategy-name-cell__main { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-weight: 600; }
.strategy-name-cell__sub, .strategy-schedule-cell { color: var(--wt-text-tertiary); font-size: 12px; }
.strategy-schedule-cell__mode { color: var(--wt-text-primary); font-size: 13px; }
.strategy-editor { min-width: 0; }
.strategy-editor .resource-section + .resource-section { margin-top: 28px; }
.strategy-template-label { display: inline-flex; align-items: center; gap: 4px; width: calc(100% - 16px); vertical-align: middle; }
.strategy-template-label .t-link { margin-left: auto; font-size: 12px; }
.strategy-section-note { margin: 0 0 20px; padding: 12px 14px; border-left: 3px solid var(--wt-primary); background: var(--wt-bg-page); color: var(--wt-text-tertiary); font-size: 12px; }
.strategy-field { display: grid; gap: 4px; width: 100%; }
.strategy-field-error { margin: 0; color: var(--wt-danger); font-size: 12px; }
.strategy-bind-hint { color: var(--wt-text-tertiary); font-size: 12px; }
.strategy-rule { overflow-anchor: none; margin: 16px 0; padding: 20px; border: 1px solid var(--wt-border); border-radius: var(--wt-radius-md); }
.strategy-rule.is-drop-target { border-color: var(--wt-primary); }
.strategy-rule-help { display: flex; align-items: center; gap: 4px; color: var(--wt-text-tertiary); font-size: 12px; }
.strategy-rule__conditions { display: grid; gap: 16px; min-width: 0; }
.strategy-rule__conditions > .t-button { justify-self: start; }
.strategy-rule__condition { display: grid; grid-template-columns: minmax(0,1fr) minmax(0,1fr) minmax(0,1.2fr) auto; gap: 16px; align-items: end; }
.strategy-rule__condition > * { min-width: 0; }
.strategy-editor__footer, .strategy-detail__footer { display:flex; justify-content:flex-end; gap:8px; margin-top:16px; }
.strategy-overview { display:grid;gap:24px;padding-top:20px; }
.strategy-overview__section h4 { margin:0 0 16px;padding-left:10px;border-left:3px solid var(--wt-primary);font-size:14px; }
.strategy-overview__grid { display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:16px 24px; }
.strategy-overview__field { display:grid;grid-template-columns:112px minmax(0,1fr);gap:16px;font-size:14px;line-height:24px; }
.strategy-overview__field span {color:var(--wt-text-tertiary);}
.strategy-overview__field strong {font-weight:400;overflow-wrap:anywhere;}
.strategy-overview__field--wide {grid-column:1/-1;}
.strategy-overview__rule {margin-top:12px;padding:16px;border:1px solid var(--wt-border);border-radius:var(--wt-radius-sm);}
.strategy-overview__rule-head {display:flex;flex-wrap:wrap;gap:8px 16px;margin-bottom:12px;font-size:13px;}
.strategy-overview__rule-head strong {margin-right:auto;}
.strategy-overview__condition {display:flex;gap:16px;padding:6px 0;font-size:13px;}
.strategy-history {padding-top:20px;}
.strategy-history__filters {display:flex;flex-wrap:wrap;gap:20px 24px;align-items:flex-end;margin-bottom:20px;}
.strategy-history__filters > .wt-filter-field {flex:0 1 180px;flex-direction:column;align-items:stretch;gap:8px;}
.strategy-history__filters > .wt-filter-field:has(.t-date-range-picker) {flex-basis:300px;}
.strategy-history__actions {display:flex;gap:8px;}
.strategy-run-message {font-size:13px;color:var(--wt-text-tertiary);}
@media(max-width:680px){.strategy-rule__condition{grid-template-columns:1fr 1fr;}.strategy-overview__grid{grid-template-columns:1fr;}.strategy-rule{padding:16px;}}
</style>
