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
  { colKey: 'name', title: '策略名称', width: 190 },
  { colKey: 'game', title: '游戏', width: 120 },
  { colKey: 'template', title: '模板', width: 170 },
  { colKey: 'content_type', title: '内容类型', width: 120 },
  { colKey: 'config', title: '切片区间', width: 130 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'updated_at', title: '修改时间', width: 150 },
  { colKey: 'op', title: '操作', width: 190, fixed: 'right' },
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

onMounted(load)

async function load() {
  loading.value = true
  error.value = ''
  try {
    // 先拿到当前用户再决定要不要取团队列表：只有管理员的创建表单用得上它。
    currentUser.value = await sessionClient.me()
    const requests = [client.listStrategies(), client.listTemplates(), client.listGames()]
    if (isAdmin.value) requests.push(client.listTeams())
    const [strategyRows, templateRows, gameRows, teamRows] = await Promise.all(requests)
    rows.value = Array.isArray(strategyRows) ? strategyRows : []
    templates.value = Array.isArray(templateRows) ? templateRows : []
    games.value = Array.isArray(gameRows) ? gameRows : []
    if (Array.isArray(teamRows)) teams.value = teamRows
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
    content_type: '',
    compose_template_id: '',
    start_seconds: 0,
    end_seconds: 0,
  }
}

function resetFilters() {
  filters.value = { name: '', game_id: '', status: '' }
}

function openCreate() {
  editingID.value = null
  form.value = defaultForm()
  visible.value = true
}

function openEdit(row) {
  editingID.value = row.id
  form.value = {
    team_id: row.team_id,
    game_id: row.game_id,
    name: row.name,
    content_type: row.content_type,
    compose_template_id: row.compose_template_id,
    start_seconds: row.config?.start_seconds ?? 0,
    end_seconds: row.config?.end_seconds ?? 0,
  }
  visible.value = true
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

function toggleStatus(row) {
  const next = row.status === 'active' ? 'inactive' : 'active'
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
  const contentType = String(form.value.content_type || '').trim()
  if (!contentType) {
    feedback.toast.error('请输入内容类型')
    return
  }
  const start = Number(form.value.start_seconds)
  const end = Number(form.value.end_seconds)
  if (!Number.isInteger(start) || start < 0) {
    feedback.toast.error('开始秒数必须是不小于 0 的整数')
    return
  }
  if (!Number.isInteger(end) || end <= start) {
    feedback.toast.error('结束秒数必须是大于开始秒数的整数')
    return
  }
  saving.value = true
  try {
    if (editingID.value) {
      await client.updateStrategy(editingID.value, { name, content_type: contentType, config: { start_seconds: start, end_seconds: end } })
    } else {
      const template = templates.value.find((item) => item.id === Number(form.value.compose_template_id))
      if (!template) {
        feedback.toast.error('请选择合成模板')
        return
      }
      const payload = {
        game_id: form.value.game_id,
        name,
        content_type: contentType,
        compose_template_id: template.id,
        compose_template_version: template.version,
        config: { start_seconds: start, end_seconds: end },
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

function templateLabel(id) {
  const template = templates.value.find((item) => item.id === id)
  return template ? `${template.name} v${template.version}` : `模板 #${id}`
}

function cutLabel(config) {
  if (!config || config.start_seconds === undefined || config.end_seconds === undefined) return '-'
  return `${config.start_seconds}s – ${config.end_seconds}s`
}

function statusLabel(status) {
  return status === 'active' ? '启用' : '停用'
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
            <template #status="{ row }"><ResourceStatusBadge :tone="row.status === 'active' ? 'success' : 'neutral'" :label="statusLabel(row.status)" /></template>
            <template #op="{ row }">
              <t-space size="small">
                <t-button size="small" class="wt-secondary-button" variant="outline" @click="detailRow = row; detailVisible = true">详情</t-button>
                <template v-if="canManage">
                  <t-button size="small" theme="primary" @click="openEdit(row)">编辑</t-button>
                  <t-dropdown trigger="click">
                    <t-button size="small" class="wt-secondary-button" variant="outline">更多</t-button>
                    <t-dropdown-menu>
                      <t-dropdown-item @click="openCopy(row)">复制</t-dropdown-item>
                      <t-dropdown-item @click="toggleStatus(row)">{{ row.status === 'active' ? '停用' : '启用' }}</t-dropdown-item>
                    </t-dropdown-menu>
                  </t-dropdown>
                </template>
              </t-space>
            </template>
          </t-table>
        </div>
      </ResourceCard>

      <t-dialog v-model:visible="visible" :header="editingID ? '编辑合成策略' : '新增合成策略'" width="640px" :footer="false">
        <t-form label-width="96px">
          <t-form-item label="策略名称" required-mark><t-input v-model="form.name" :maxlength="120" placeholder="例如：王者荣耀高光切片" /></t-form-item>
          <t-form-item label="内容类型" required-mark><t-input v-model="form.content_type" :maxlength="64" placeholder="例如：高光切片" /></t-form-item>
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
            <t-form-item label="合成模板" required-mark>
              <t-select v-model="form.compose_template_id" clearable placeholder="选择合成模板">
                <t-option v-for="option in templateOptions" :key="option.value" :value="option.value" :label="option.label" />
              </t-select>
            </t-form-item>
          </template>
          <t-form-item v-else label="合成模板"><t-input :value="templateLabel(form.compose_template_id)" disabled /></t-form-item>
          <t-form-item label="开始秒数" required-mark>
            <t-input-number v-model="form.start_seconds" :min="0" theme="column" style="width:160px" />
          </t-form-item>
          <t-form-item label="结束秒数" required-mark>
            <t-input-number v-model="form.end_seconds" :min="1" theme="column" style="width:160px" />
          </t-form-item>
          <t-form-item v-if="!editingID" label="">
            <div class="strategy-bind-hint">模板版本创建后固定；模板停用后不能新建绑定，已保存的策略不受影响。</div>
          </t-form-item>
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
          <div class="strategy-drawer__field"><span>所属游戏</span><strong>{{ gameName(detailRow.game_id) }}</strong></div>
          <div class="strategy-drawer__field"><span>合成模板</span><strong>{{ templateLabel(detailRow.compose_template_id) }}</strong></div>
          <div class="strategy-drawer__field"><span>模板版本</span><strong>v{{ detailRow.compose_template_version }}</strong></div>
          <div class="strategy-drawer__field"><span>内容类型</span><strong>{{ detailRow.content_type || '-' }}</strong></div>
          <div class="strategy-drawer__field"><span>切片区间</span><strong>{{ cutLabel(detailRow.config) }}</strong></div>
          <div class="strategy-drawer__field"><span>状态</span><strong>{{ statusLabel(detailRow.status) }}</strong></div>
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
.strategy-bind-hint { color: var(--wt-text-tertiary); font-size: 12px; line-height: 1.55; }
.strategy-editor__footer { display: flex; justify-content: flex-end; gap: 8px; margin-top: 16px; }
.strategy-drawer { display: flex; flex-direction: column; gap: 10px; }
.strategy-drawer__field { display: flex; justify-content: space-between; gap: 12px; padding: 10px 12px; border: 1px solid var(--wt-border); border-radius: 8px; }
.strategy-drawer__field span { color: var(--wt-text-tertiary); font-size: 12px; flex-shrink: 0; }
.strategy-drawer__field strong { color: var(--wt-text-primary); font-size: 14px; font-weight: 600; text-align: right; word-break: break-all; }
</style>
