<script setup>
import { feedback } from '../../../shared/ui/feedback/index.js'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import { computed, onMounted, ref } from 'vue'
import { DialogPlugin } from 'tdesign-vue-next'
import { createComposeClient } from '../../../shared/api/compose.js'
import { createSessionClient } from '../../../shared/api/session.js'
import ResourceCard from '../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../shared/ui/resource/ResourcePageHeader.vue'
import ResourceStatGrid from '../../../shared/ui/resource/ResourceStatGrid.vue'
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'
import { formatDateTime } from '../../../shared/utils/datetime.js'

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

const copyVisible = ref(false)
const copySaving = ref(false)
const copyTarget = ref(null)
const copyName = ref('')

const detailVisible = ref(false)
const detailRow = ref(null)

const filters = ref({ name: '', game_id: '', status: '' })

// 每一列都必须声明 width，不许只写 minWidth（WebKit fixed 布局下 minWidth 不产生确定列宽，
// 会被压成剩余空间列；见 ContentPoolPage.vue 里 columns 上方的完整说明）。
const columns = [
  { colKey: 'id', title: 'ID', width: 70 },
  { colKey: 'name', title: '策略名称', width: 220 },
  { colKey: 'game', title: '游戏', width: 130 },
  { colKey: 'template', title: '模板', width: 180 },
  { colKey: 'config', title: '切片区间', width: 140 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'updated_at', title: '修改时间', width: 150 },
  { colKey: 'op', title: '操作', width: 170, fixed: 'right' },
]

// 横向滚动宽度由列宽推导，不写死（写死值与列宽合计无关，是个漂移源）。
const tableScroll = computed(() => ({
  x: `${columns.reduce((sum, col) => sum + (col.width || 0), 0)}px`,
}))

const filteredRows = computed(() => rows.value.filter((row) => {
  if (filters.value.name && !String(row.name || '').toLowerCase().includes(filters.value.name.trim().toLowerCase())) return false
  if (filters.value.game_id && row.game_id !== filters.value.game_id) return false
  if (filters.value.status && row.status !== filters.value.status) return false
  return true
}))

const stats = computed(() => [
  { key: 'total', label: '策略总数', value: rows.value.length, tone: 'info' },
  { key: 'active', label: '启用中', value: rows.value.filter((row) => row.status === 'active').length, tone: 'success' },
  { key: 'inactive', label: '已停用', value: rows.value.filter((row) => row.status === 'inactive').length, tone: 'neutral' },
])

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

function schemaFields(schema) {
  const properties = schema?.properties
  if (!properties || typeof properties !== 'object') return []
  const required = Array.isArray(schema.required) ? schema.required : []
  return Object.entries(properties).map(([key, def]) => ({
    key,
    label: def.title || key,
    type: def.type || 'string',
    minimum: def.minimum,
    required: required.includes(key),
  }))
}

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
  filters.value = { name: '', game_id: '', status: '' }
}

function openCreate() {
  editingID.value = null
  form.value = defaultForm()
  templateParams.value = {}
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
  visible.value = true
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

async function save() {
  const name = String(form.value.name || '').trim()
  if (!name) {
    feedback.toast.error('请输入策略名称')
    return
  }
  const fields = formSchemaFields.value
  if (!editingID.value && !form.value.compose_template_id) {
    feedback.toast.error('请选择合成模板')
    return
  }
  if (!fields.length) {
    feedback.toast.error('模板参数未就绪，请重新选择合成模板')
    return
  }
  // 按模板 schema 的字段集组装 config：整数字段强制取整并套 schema 下限，
  // 必填字段缺值直接拦在前端；schema 外的值不会出现，因为表单就是按 schema 渲染的。
  const config = {}
  for (const field of fields) {
    const raw = templateParams.value[field.key]
    if (field.type === 'integer') {
      // t-input-number 清空后给 null，Number(null) 是 0，会静默变成合法下限值——先拦。
      if (raw === null || raw === undefined || raw === '') {
        feedback.toast.error(`请填写${field.label}`)
        return
      }
      const value = Number(raw)
      if (!Number.isInteger(value) || (field.minimum !== undefined && value < field.minimum)) {
        feedback.toast.error(`${field.label}必须是不小于 ${field.minimum ?? 0} 的整数`)
        return
      }
      config[field.key] = value
    } else if (field.type === 'boolean') {
      config[field.key] = Boolean(raw)
    } else {
      const value = String(raw ?? '').trim()
      if (field.required && !value) {
        feedback.toast.error(`请输入${field.label}`)
        return
      }
      config[field.key] = value
    }
  }
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

function cutLabel(config) {
  if (!config || config.start_seconds === undefined || config.end_seconds === undefined) return '-'
  return `${config.start_seconds}s – ${config.end_seconds}s`
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

      <ResourceStatGrid :items="stats" />

      <ResourceCard class="strategy-filter-card">
        <div class="filter-row">
          <label class="filter-field"><span>策略名称</span><t-input v-model="filters.name" clearable placeholder="搜索策略名称" style="width:200px" @enter="load" /></label>
          <label class="filter-field"><span>游戏</span><t-select v-model="filters.game_id" clearable placeholder="全部" style="width:150px">
            <t-option v-for="game in games" :key="game.id" :value="game.id" :label="game.name" />
          </t-select></label>
          <label class="filter-field"><span>状态</span><t-select v-model="filters.status" clearable placeholder="全部" style="width:120px">
            <t-option value="active" label="启用" />
            <t-option value="inactive" label="停用" />
          </t-select></label>
          <t-button theme="primary" @click="load">查询</t-button>
          <t-button class="wt-secondary-button" variant="outline" @click="resetFilters">重置</t-button>
        </div>
      </ResourceCard>

      <ResourceCard class="strategy-card">
        <div class="table-scroll-wrap">
          <t-table class="wt-resource-table" :data="filteredRows" :columns="columns" row-key="id" hover :scroll="tableScroll" empty="暂无合成策略">
            <template #game="{ row }">{{ gameName(row.game_id) }}</template>
            <template #template="{ row }">{{ templateLabel(row.compose_template_id) }}</template>
            <template #config="{ row }">{{ cutLabel(row.config) }}</template>
            <template #updated_at="{ row }">{{ formatDateTime(row.updated_at) }}</template>
            <template #status="{ row }">
              <t-switch v-if="canManage" :value="row.status === 'active'" :label="['启用', '停用']" @change="(value) => toggleStatus(row, value)" />
              <ResourceStatusBadge v-else :tone="row.status === 'active' ? 'success' : 'neutral'" :label="row.status === 'active' ? '启用' : '停用'" />
            </template>
            <template #op="{ row }">
              <t-space size="small">
                <t-button size="small" class="wt-secondary-button" variant="outline" @click="detailRow = row; detailVisible = true">详情</t-button>
                <template v-if="canManage">
                  <t-button size="small" theme="primary" @click="openEdit(row)">编辑</t-button>
                  <t-button size="small" class="wt-secondary-button" variant="outline" @click="openCopy(row)">复制</t-button>
                </template>
              </t-space>
            </template>
          </t-table>
        </div>
      </ResourceCard>

      <t-dialog v-model:visible="visible" :header="editingID ? '编辑合成策略' : '新增合成策略'" width="640px" :footer="false">
        <t-form label-width="96px">
          <div class="strategy-form-section">基础信息</div>
          <t-form-item label="策略名称" required-mark><t-input v-model="form.name" :maxlength="120" placeholder="例如：王者荣耀高光切片" /></t-form-item>
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

          <div class="strategy-form-section">模板参数</div>
          <div v-if="!editingID && !form.compose_template_id" class="strategy-bind-hint strategy-form-hint">先选择合成模板，需要填写哪些参数由模板决定。</div>
          <template v-else>
            <t-form-item v-for="field in formSchemaFields" :key="field.key" :label="field.label" :required-mark="field.required">
              <t-input-number v-if="field.type === 'integer'" v-model="templateParams[field.key]" :min="field.minimum ?? 0" theme="column" style="width:160px" />
              <t-switch v-else-if="field.type === 'boolean'" v-model="templateParams[field.key]" />
              <t-input v-else v-model="templateParams[field.key]" :placeholder="`请输入${field.label}`" />
            </t-form-item>
            <t-form-item v-if="!editingID" label="">
              <div class="strategy-bind-hint">模板版本创建后固定；模板停用后不能新建绑定，已保存的策略不受影响。</div>
            </t-form-item>
          </template>
        </t-form>
        <div class="strategy-editor__footer">
          <t-button variant="outline" @click="visible = false">取消</t-button>
          <t-button theme="primary" :loading="saving" @click="save">保存</t-button>
        </div>
      </t-dialog>

      <t-dialog v-model:visible="copyVisible" header="复制合成策略" width="480px" :footer="false">
        <t-form label-width="96px">
          <t-form-item label="新策略名称" required-mark><t-input v-model="copyName" :maxlength="120" placeholder="输入新策略名称" @enter="submitCopy" /></t-form-item>
          <t-form-item v-if="copyTarget" label="模板"><t-input :value="templateLabel(copyTarget.compose_template_id)" disabled /></t-form-item>
        </t-form>
        <div class="strategy-editor__footer">
          <t-button variant="outline" @click="copyVisible = false">取消</t-button>
          <t-button theme="primary" :loading="copySaving" @click="submitCopy">复制</t-button>
        </div>
      </t-dialog>

      <t-drawer :close-btn="true" v-model:visible="detailVisible" header="策略详情" size="460px" :footer="false">
        <div v-if="detailRow" class="strategy-drawer">
          <div class="strategy-drawer__field"><span>策略名称</span><strong>{{ detailRow.name || '-' }}</strong></div>
          <div class="strategy-drawer__field"><span>所属团队</span><strong>{{ teamName(detailRow.team_id) }}</strong></div>
          <div class="strategy-drawer__field"><span>所属游戏</span><strong>{{ gameName(detailRow.game_id) }}</strong></div>
          <div class="strategy-drawer__field"><span>合成模板</span><strong>{{ templateLabel(detailRow.compose_template_id) }}</strong></div>
          <div class="strategy-drawer__field"><span>模板版本</span><strong>v{{ detailRow.compose_template_version }}</strong></div>
          <div class="strategy-drawer__field"><span>切片区间</span><strong>{{ cutLabel(detailRow.config) }}</strong></div>
          <div class="strategy-drawer__field"><span>状态</span><strong>{{ detailRow.status === 'active' ? '启用' : '停用' }}</strong></div>
          <div class="strategy-drawer__field"><span>创建时间</span><strong>{{ formatDateTime(detailRow.created_at) }}</strong></div>
          <div class="strategy-drawer__field"><span>修改时间</span><strong>{{ formatDateTime(detailRow.updated_at) }}</strong></div>
        </div>
      </t-drawer>
    </div>
  </t-loading>
</template>

<style scoped>
.strategy-card { padding: 18px 20px; margin-top: 16px; }
.strategy-filter-card { padding: 14px 20px; }
.filter-row { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.filter-field { display: flex; align-items: center; gap: 8px; color: var(--wt-text-secondary); font-size: 13px; font-weight: 500; white-space: nowrap; }
.strategy-form-section { margin: 4px 0 14px; padding-left: 8px; border-left: 3px solid var(--wt-brand, var(--wt-border)); color: var(--wt-text-primary); font-size: 14px; font-weight: 600; line-height: 1.4; }
.strategy-form-hint { padding-left: 8px; }
.strategy-bind-hint { color: var(--wt-text-tertiary); font-size: 12px; line-height: 1.55; }
.strategy-editor__footer { display: flex; justify-content: flex-end; gap: 8px; margin-top: 16px; }
.strategy-drawer { display: flex; flex-direction: column; gap: 10px; }
.strategy-drawer__field { display: flex; justify-content: space-between; gap: 12px; padding: 10px 12px; border: 1px solid var(--wt-border); border-radius: 8px; }
.strategy-drawer__field span { color: var(--wt-text-tertiary); font-size: 12px; flex-shrink: 0; }
.strategy-drawer__field strong { color: var(--wt-text-primary); font-size: 14px; font-weight: 600; text-align: right; word-break: break-all; }
</style>
