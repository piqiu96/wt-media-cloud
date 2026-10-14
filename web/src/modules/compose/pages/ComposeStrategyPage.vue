<script setup>
import { feedback } from '../../../shared/ui/feedback/index.js'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import { computed, onMounted, ref } from 'vue'
import { DialogPlugin } from 'tdesign-vue-next'
import { createComposeClient } from '../../../shared/api/compose.js'
import { createSessionClient } from '../../../shared/api/session.js'
import ResourceCard from '../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../shared/ui/resource/ResourcePageHeader.vue'
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'
import { opColumnWidth } from '../../../shared/ui/resource/opColumnWidth.js'
import { formatDateTime } from '../../../shared/utils/datetime.js'
import { schemaFields } from '../schemaFields.js'
import { scheduleLabel } from '../scheduleForm.js'
import ComposeScheduleDialog from '../components/ComposeScheduleDialog.vue'

const client = createComposeClient()
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
// 模板参数区由所选模板的 schema 驱动：需要哪些字段、叫什么名字、什么类型都来自
// schema（执行器 BasicCutV1() 发布），页面不写死字段清单。
const templateParams = ref({})
// 校验就近提示（AdminUI 规范 §5.6）：错误挂在字段旁边，不再只靠 toast。
const paramErrors = ref({})
const nameError = ref('')
// 打开编辑器时的快照，关闭时比对判断「有未保存内容」，决定要不要二次提示。
const savedSnapshot = ref('')

const copyVisible = ref(false)
const copySaving = ref(false)
const copyTarget = ref(null)
const copyName = ref('')

const detailVisible = ref(false)
const detailRow = ref(null)
const runHistory = ref([])
const runHistoryError = ref('')
const scheduleRow = ref(null)

const skipReasons = {
  creator_disabled: '创建人已停用', template_unavailable: '模板不可用', other_game: '不属于当前游戏',
  material_unavailable: '素材不可用', already_produced: '该策略已产出成片', in_progress: '任务正在排队或执行',
  daily_limit: '达到每日上限', per_run_limit: '达到单次上限',
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
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'schedule', title: '自动执行', width: 150 },
  { colKey: 'next_run_at', title: '下次执行', width: 160 },
  { colKey: 'game', title: '游戏', width: 130 },
  { colKey: 'template', title: '模板', width: 180 },
  { colKey: 'config', title: '主要参数', width: 170 },
  { colKey: 'updated_at', title: '修改时间', width: 150 },
  { colKey: 'op', title: '操作', width: opColumnWidth(canManage.value ? ['详情', '编辑', '复制', '计划'] : ['详情']), fixed: 'right' },
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
const activeCount = computed(() => filteredRows.value.filter((row) => row.status === 'active').length)

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


onMounted(load)

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
  }
}

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

async function scheduleSaved(item) {
  scheduleRow.value = null
  if (detailRow.value?.id === item.id) detailRow.value = item
  await load()
}

function scheduleState(row) {
  if (!row.schedule || row.schedule.mode === 'manual') return '手动'
  if (!row.schedule.enabled) return '已暂停'
  return row.status === 'active' ? '已启用' : '策略停用'
}

function runNow(row) {
  const dialog = DialogPlugin.confirm({
    header: '立即创建合成任务',
    body: `将从策略创建人的“我的素材”中筛选候选素材，为每条符合条件的素材创建独立任务。此操作不改变「${row.name}」的下次计划时间。`,
    confirmBtn: { content: '立即执行', theme: 'primary' },
    onConfirm: async () => {
      try {
        const result = await client.runStrategyNow(row.id, crypto.randomUUID())
        feedback.toast.success(`已创建 ${result.created_count} 个合成任务`)
        await loadRunHistory(row.id)
        dialog.hide()
      } catch (e) {
        error.value = e?.message || '立即执行失败'
        dialog.hide()
      }
    },
  })
}

// 启停开关在列表行上（走查修订第 3 条）。开关受控于 row.status，确认后才真正变更；
// 取消时不动数据，下一次渲染自然回弹。
function toggleStatus(row, nextEnabled) {
  const next = nextEnabled ? 'active' : 'inactive'
  const dialog = DialogPlugin.confirm({
    header: next === 'active' ? '启用策略' : '停用策略',
    body: next === 'active'
      ? `确认启用策略「${row.name}」？启用后普通运营可以看到并使用它。`
      : `确认停用策略「${row.name}」？停用后普通运营不再看到它。`,
    confirmBtn: { content: next === 'active' ? '启用' : '停用', theme: next === 'active' ? 'primary' : 'danger' },
    onConfirm: async () => {
      try {
        await client.setStrategyStatus(row.id, next)
        await load()
        feedback.toast.success(next === 'active' ? '策略已启用' : '策略已停用')
      } catch (e) {
        error.value = e.message || '状态更新失败'
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
  saving.value = true
  try {
    if (editingID.value) {
      await client.updateStrategy(editingID.value, { name, config })
    } else {
      const template = templates.value.find((item) => item.id === Number(form.value.compose_template_id))
      const payload = {
        game_id: form.value.game_id,
        name,
        compose_template_id: template.id,
        compose_template_version: template.version,
        config,
      }
      if (isAdmin.value) payload.team_id = Number(form.value.team_id) || 0
      await client.createStrategy(payload)
    }
    visible.value = false
    await load()
    feedback.toast.success(editingID.value ? '策略已更新' : '策略已保存')
  } catch (e) {
    error.value = e.message || '策略保存失败'
  } finally {
    saving.value = false
  }
}

function gameName(id) {
  return games.value.find((game) => game.id === id)?.name || id
}

function teamName(id) {
  return teams.value.find((team) => team.id === id)?.name || `分组 #${id}`
}

function templateLabel(id) {
  const template = templates.value.find((item) => item.id === id)
  return template ? `${template.name} v${template.version}` : `模板 #${id}`
}

function validGuideURL(value) {
  try {
    const url = new URL(value)
    return url.protocol === 'https:' && !!url.hostname && !url.username && !url.password && !url.hash && (!url.port || url.port === '443')
  } catch { return false }
}

function configLabel(config) {
  if (!config) return '-'
  if (config.insert_seconds !== undefined) return `第 ${config.insert_seconds} 秒插入`
  if (config.start_seconds !== undefined && config.end_seconds !== undefined) return `${config.start_seconds}s – ${config.end_seconds}s`
  return '-'
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
            <t-option value="active" label="启用" />
            <t-option value="inactive" label="停用" />
          </t-select></label>
          <div class="wt-filter-actions">
            <t-button theme="primary" @click="load">查询</t-button>
            <t-button class="wt-secondary-button" variant="outline" @click="resetFilters">重置</t-button>
          </div>
        </div>
        <div class="strategy-list-summary" aria-live="polite">
          <span>当前结果 <strong>{{ filteredRows.length }}</strong> 条</span>
          <span>已启用 <strong>{{ activeCount }}</strong></span>
        </div>
        <div class="table-scroll-wrap">
          <t-table class="wt-resource-table" :data="pagedRows" :columns="columns" row-key="id" hover size="small" :scroll="tableScroll" empty="暂无合成策略">
            <template #id="{ row }"><span class="strategy-id">{{ row.id }}</span></template>
            <template #name="{ row }">
              <div class="strategy-name-cell">
                <span class="strategy-name-cell__main" :title="row.name">{{ row.name }}</span>
                <span class="strategy-name-cell__sub">{{ teamName(row.team_id) }}</span>
              </div>
            </template>
            <template #game="{ row }">{{ gameName(row.game_id) }}</template>
            <template #template="{ row }">{{ templateLabel(row.compose_template_id) }}</template>
            <template #config="{ row }">{{ configLabel(row.config) }}</template>
            <template #updated_at="{ row }">{{ formatDateTime(row.updated_at) }}</template>
            <template #status="{ row }">
              <t-switch v-if="canManage" :value="row.status === 'active'" :label="['启用', '停用']" @change="(value) => toggleStatus(row, value)" />
              <ResourceStatusBadge v-else :tone="row.status === 'active' ? 'success' : 'neutral'" :label="row.status === 'active' ? '启用' : '停用'" />
            </template>
            <template #schedule="{ row }"><div class="strategy-schedule-cell"><ResourceStatusBadge :tone="row.schedule?.enabled && row.status === 'active' ? 'success' : 'neutral'" :label="scheduleState(row)" /><span>{{ scheduleLabel(row.schedule) }}</span></div></template>
            <template #next_run_at="{ row }">{{ row.next_run_at ? formatDateTime(row.next_run_at) : '—' }}</template>
            <template #op="{ row }">
              <t-space size="small" class="wt-resource-actions">
                <t-button size="small" class="wt-secondary-button" variant="outline" @click="openDetail(row)">详情</t-button>
                <template v-if="canManage">
                  <t-button size="small" theme="primary" @click="openEdit(row)">编辑</t-button>
                  <t-button size="small" class="wt-secondary-button" variant="outline" @click="openCopy(row)">复制</t-button>
                  <t-button size="small" class="wt-secondary-button" variant="outline" @click="scheduleRow = row">计划</t-button>
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
                  <t-form-item label="所属团队"><t-input :value="teamName(form.team_id)" disabled /></t-form-item>
                  <t-form-item label="所属游戏"><t-input :value="gameName(form.game_id)" disabled /></t-form-item>
                </template>
                <t-form-item label="合成模板" required-mark>
                  <t-select v-if="!editingID" v-model="form.compose_template_id" clearable placeholder="选择合成模板" @change="onTemplateChange">
                    <t-option v-for="option in templateOptions" :key="option.value" :value="option.value" :label="option.label" />
                  </t-select>
                  <t-input v-else :value="templateLabel(form.compose_template_id)" disabled />
                </t-form-item>
              </t-form>
            </div>

            <div class="strategy-section">
              <div class="strategy-section__title"><span class="strategy-no">②</span>模板参数<small>需要填写哪些参数由所选合成模板决定</small></div>
              <t-form label-width="88px">
                <t-form-item v-if="!editingID && !form.compose_template_id" label=""><div class="strategy-bind-hint">先选择合成模板，需要填写哪些参数由模板决定。</div></t-form-item>
                <template v-else>
                  <t-form-item v-for="field in formSchemaFields" :key="field.key" :label="field.label" :required-mark="field.required">
                    <div class="strategy-field">
                      <t-input-number v-if="field.type === 'integer'" v-model="templateParams[field.key]" :min="field.minimum ?? 0" theme="column" style="width:160px" :status="paramErrors[field.key] ? 'error' : undefined" />
                      <t-switch v-else-if="field.type === 'boolean'" v-model="templateParams[field.key]" />
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
          </div>

          <div class="strategy-editor__aside">
            <div class="strategy-aside">
              <div class="strategy-aside__title">配置说明</div>
              <div class="strategy-aside__item"><strong>① 基础信息</strong><span>策略名称用于区分不同策略；所属团队与游戏决定普通运营的可见范围。</span></div>
              <div class="strategy-aside__item"><strong>② 模板参数</strong><span>参数由合成模板定义（如基础切段的开始/结束秒数），保存时按模板 schema 校验。</span></div>
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

      <ComposeScheduleDialog :strategy="scheduleRow" @close="scheduleRow = null" @saved="scheduleSaved" />

      <t-dialog v-model:visible="copyVisible" header="复制合成策略" width="480px" :footer="false">
        <t-form label-width="96px">
          <t-form-item label="新策略名称" required-mark><t-input v-model="copyName" :maxlength="120" placeholder="输入新策略名称" @enter="submitCopy" /></t-form-item>
          <t-form-item v-if="copyTarget" label="模板"><t-input :value="templateLabel(copyTarget.compose_template_id)" disabled /></t-form-item>
        </t-form>
        <div class="strategy-editor__footer">
          <t-button class="wt-secondary-button" variant="outline" @click="copyVisible = false">取消</t-button>
          <t-button theme="primary" :loading="copySaving" @click="submitCopy">复制</t-button>
        </div>
      </t-dialog>

      <t-drawer :close-btn="true" v-model:visible="detailVisible" header="策略详情" size="560px" :footer="false">
        <div v-if="detailRow" class="strategy-drawer">
          <div class="strategy-drawer__hero"><div><span>策略 ID {{ detailRow.id }}</span><h3>{{ detailRow.name || '-' }}</h3><p>{{ gameName(detailRow.game_id) }} · {{ teamName(detailRow.team_id) }}</p></div><ResourceStatusBadge :tone="detailRow.status === 'active' ? 'success' : 'neutral'" :label="detailRow.status === 'active' ? '启用' : '停用'" /></div>
          <h4 class="strategy-detail-section">使用范围</h4>
          <div class="strategy-drawer__field"><span>所属团队</span><strong>{{ teamName(detailRow.team_id) }}</strong></div>
          <div class="strategy-drawer__field"><span>所属游戏</span><strong>{{ gameName(detailRow.game_id) }}</strong></div>
          <h4 class="strategy-detail-section">合成配置</h4>
          <div class="strategy-drawer__field"><span>合成模板</span><strong>{{ templateLabel(detailRow.compose_template_id) }}</strong></div>
          <div class="strategy-drawer__field"><span>模板版本</span><strong>v{{ detailRow.compose_template_version }}</strong></div>
          <div class="strategy-drawer__field"><span>主要参数</span><strong>{{ configLabel(detailRow.config) }}</strong></div>
          <div v-if="detailRow.config?.guide_video_url" class="strategy-drawer__field"><span>引导视频</span><strong class="strategy-guide-url">{{ detailRow.config.guide_video_url }}</strong></div>
          <div v-if="detailRow.config?.guide_video_url" class="strategy-drawer__field"><span>引导时长</span><strong>{{ detailRow.config.guide_duration_seconds ? `${detailRow.config.guide_duration_seconds} 秒` : '完整视频' }}</strong></div>
          <div v-if="detailRow.config?.guide_video_url" class="strategy-drawer__field"><span>成片上限</span><strong>{{ detailRow.config.max_duration_seconds ? `${detailRow.config.max_duration_seconds} 秒` : '不限' }}</strong></div>
          <h4 class="strategy-detail-section">自动执行计划</h4>
          <div class="strategy-drawer__field"><span>自动执行</span><strong>{{ scheduleState(detailRow) }}</strong></div>
          <div class="strategy-drawer__field"><span>周期</span><strong>{{ scheduleLabel(detailRow.schedule) }}</strong></div>
          <div class="strategy-drawer__field"><span>时区</span><strong>{{ detailRow.schedule?.timezone || 'Asia/Shanghai' }}</strong></div>
          <div class="strategy-drawer__field"><span>素材范围</span><strong>创建人的“我的素材”（当前团队与游戏）</strong></div>
          <div class="strategy-drawer__field"><span>单次／每日上限</span><strong>{{ detailRow.schedule ? `${detailRow.schedule.per_run_limit}／${detailRow.schedule.daily_limit}` : '未设置' }}</strong></div>
          <div class="strategy-drawer__field"><span>下次执行</span><strong>{{ detailRow.next_run_at ? formatDateTime(detailRow.next_run_at) : '—' }}</strong></div>
          <div v-if="canManage && detailRow.status === 'active' && detailRow.schedule" class="strategy-drawer__actions"><t-button theme="primary" @click="runNow(detailRow)">立即执行</t-button></div>
          <h4 class="strategy-detail-section">最近运行</h4>
          <p v-if="runHistoryError" class="strategy-run-message">{{ runHistoryError }}</p>
          <p v-else-if="!runHistory.length" class="strategy-run-message">暂无运行记录</p>
          <div v-for="run in runHistory" :key="run.run_key" class="strategy-run">
            <div class="strategy-run__summary"><strong>{{ run.trigger_type === 'scheduled' ? '自动执行' : '立即执行' }} · {{ formatDateTime(run.schedule_slot) }}</strong><span>创建 {{ run.created_count }} / 候选 {{ run.candidate_count }}</span></div>
            <details v-if="run.skipped?.length"><summary>跳过 {{ run.skipped.length }} 项</summary><p v-for="(skip, index) in run.skipped" :key="`${run.run_key}-${index}`">{{ skip.material_id ? `素材 ${skip.material_id}：` : '' }}{{ skipReasons[skip.reason] || skip.reason }}</p></details>
          </div>
          <h4 class="strategy-detail-section">记录时间</h4>
          <div class="strategy-drawer__field"><span>创建时间</span><strong>{{ formatDateTime(detailRow.created_at) }}</strong></div>
          <div class="strategy-drawer__field"><span>修改时间</span><strong>{{ formatDateTime(detailRow.updated_at) }}</strong></div>
        </div>
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
.strategy-run-message { color: var(--wt-text-tertiary); font-size: 12px; }
.strategy-card { padding: 18px 20px; margin-top: 16px; }
.strategy-list-summary { display: flex; gap: 20px; align-items: center; padding: 0 0 12px; color: var(--wt-text-tertiary); font-size: 12px; }
.strategy-list-summary strong { color: var(--wt-text-primary); font-size: 14px; font-weight: 600; }
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
.strategy-detail-section { margin: 24px 0 8px; padding-bottom: 8px; border-bottom: 1px solid var(--wt-border); color: var(--wt-text-primary); font-size: 14px; font-weight: 600; }
.strategy-drawer__hero { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; padding: 4px 0 16px; }
.strategy-drawer__hero span, .strategy-drawer__hero p { margin: 0; color: var(--wt-text-tertiary); font-size: 12px; }
.strategy-drawer__hero h3 { margin: 4px 0; color: var(--wt-text-primary); font-size: 20px; }
.strategy-drawer__field { display: grid; grid-template-columns: 110px minmax(0, 1fr); gap: 12px; padding: 8px 0; }
.strategy-drawer__field span { color: var(--wt-text-tertiary); font-size: 12px; }
.strategy-drawer__field strong { color: var(--wt-text-primary); font-size: 13px; font-weight: 500; overflow-wrap: anywhere; }
.strategy-drawer__actions { display: flex; justify-content: flex-end; margin-top: 12px; }
</style>
