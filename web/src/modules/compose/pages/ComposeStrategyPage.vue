<script setup>
import { feedback } from '../../../shared/ui/feedback/index.js'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import FormError from '../../../shared/ui/feedback/FormError.vue'
import { computed, onMounted, ref } from 'vue'
import { createComposeClient } from '../../../shared/api/compose.js'
import { createSessionClient } from '../../../shared/api/session.js'
import ResourceCard from '../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../shared/ui/resource/ResourcePageHeader.vue'
import ResourceStatGrid from '../../../shared/ui/resource/ResourceStatGrid.vue'
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'
import { formatDateTime } from '../../../shared/utils/datetime.js'

const client = createComposeClient()
const sessionClient = createSessionClient()

const me = ref(null)
const rows = ref([])
const templates = ref([])
const teams = ref([])
const games = ref([])
const loading = ref(false)
const error = ref('')

const filters = ref({ name: '', status: '' })

const visible = ref(false)
const saving = ref(false)
const editingID = ref(null)
const form = ref(defaultForm())
const fieldErrors = ref({})
const formError = ref('')

const detailVisible = ref(false)
const detailRow = ref(null)

const versionsVisible = ref(false)
const versionTarget = ref(null)
const versions = ref([])
const versionsError = ref('')

const copyVisible = ref(false)
const copyTarget = ref(null)
const copyName = ref('')
const copyError = ref('')
const copySaving = ref(false)

// 普通运营只读：与服务端 visibleStrategy 的角色分支用同一套角色词。
const canManage = computed(() => me.value?.role === 'admin' || me.value?.role === 'senior_operator')

// 列宽规则见 ContentPoolPage.vue / DiscoveryStrategiesPage.vue：每列声明 width，
// 不写 minWidth —— WebKit 的 fixed 布局不认 minWidth，会把它压成剩余空间列。
const columns = [
  { colKey: 'id', title: 'ID', width: 70 },
  { colKey: 'name', title: '策略名称', width: 180 },
  { colKey: 'game', title: '游戏', width: 120 },
  { colKey: 'content_type', title: '内容类型', width: 110 },
  { colKey: 'template', title: '模板', width: 140 },
  { colKey: 'ratio', title: '画面比例', width: 100 },
  { colKey: 'overrides', title: '可覆盖参数', width: 130 },
  { colKey: 'current_version', title: '当前版本', width: 90 },
  { colKey: 'updated_at', title: '修改时间', width: 150 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'op', title: '操作', width: 230, fixed: 'right' },
]

const tableScroll = computed(() => ({
  x: `${columns.reduce((sum, col) => sum + (col.width || 0), 0)}px`,
}))

const filteredRows = computed(() => rows.value.filter((row) => {
  if (filters.value.name && !String(row.name || '').toLowerCase().includes(filters.value.name.trim().toLowerCase())) return false
  if (filters.value.status && row.status !== filters.value.status) return false
  return true
}))

const stats = computed(() => [
  { key: 'total', label: '策略总数', value: rows.value.length, tone: 'info' },
  { key: 'active', label: '启用策略', value: rows.value.filter((row) => row.status === 'active').length, tone: 'success' },
  { key: 'templates', label: '可用模板', value: templates.value.length, tone: 'info' },
])

// 表单里的模板与参数选项全部来自目录接口，页面不写死任何模板事实。
const selectedTemplate = computed(() => templates.value.find(
  (template) => template.key === form.value.template_key && template.version === form.value.template_version,
))

const templateChoice = computed({
  get: () => (form.value.template_key && form.value.template_version ? `${form.value.template_key}@${form.value.template_version}` : ''),
  set: (choice) => {
    const found = templates.value.find((template) => `${template.key}@${template.version}` === choice)
    if (!found) {
      form.value.template_key = ''
      form.value.template_version = ''
      return
    }
    form.value.template_key = found.key
    form.value.template_version = found.version
    if (!(found.video_ratios || []).includes(form.value.video_ratio)) form.value.video_ratio = ''
    form.value.allowed_overrides = (form.value.allowed_overrides || []).filter((field) => (found.allowed_overrides || []).includes(field))
  },
})

// 高级运营只能在本团队、自己的游戏里建策略（服务端同样拒绝）；管理员从全量启用游戏里选。
const gameOptions = computed(() => {
  if (!me.value) return []
  if (me.value.role === 'admin') return games.value.filter((game) => game.status === 'enabled')
  const mine = new Set(me.value.game_ids || [])
  return games.value.filter((game) => mine.has(game.id) && game.status === 'enabled')
})

const overrideLabels = { video_ratio: '画面比例' }

function overrideLabel(field) {
  return overrideLabels[field] || field
}

function gameName(id) {
  return games.value.find((game) => game.id === id)?.name || id || '-'
}

function teamName(id) {
  return teams.value.find((team) => team.id === id)?.name || String(id || '-')
}

function templateLabel(row) {
  const found = templates.value.find((template) => template.key === row.template_key && template.version === row.template_version)
  return found ? `${found.name} ${found.version}` : `${row.template_key} ${row.template_version}`
}

onMounted(load)

async function load() {
  loading.value = true
  error.value = ''
  try {
    if (!me.value) me.value = await sessionClient.me()
    const [strategyRows, templateRows, teamRows, gameRows] = await Promise.all([
      client.listStrategies(),
      client.listTemplates(),
      client.listTeams(),
      client.listGames(),
    ])
    rows.value = Array.isArray(strategyRows) ? strategyRows : []
    templates.value = Array.isArray(templateRows) ? templateRows : []
    teams.value = Array.isArray(teamRows) ? teamRows : []
    games.value = Array.isArray(gameRows) ? gameRows : []
  } catch (e) {
    error.value = e.message || '策略加载失败'
  } finally {
    loading.value = false
  }
}

function resetFilters() {
  filters.value = { name: '', status: '' }
}

function defaultForm() {
  const first = templates.value[0]
  return {
    name: '',
    team_id: '',
    game_id: '',
    content_type: '',
    template_key: first?.key || '',
    template_version: first?.version || '',
    video_ratio: first?.video_ratios?.[0] || '',
    allowed_overrides: [],
    status: 'active',
    expected_version: 0,
  }
}

function openCreate() {
  editingID.value = null
  form.value = defaultForm()
  fieldErrors.value = {}
  formError.value = ''
  visible.value = true
}

function openEdit(row) {
  editingID.value = row.id
  // 期望版本在打开那一刻定格：表格行在对话框开着时可能已被别人更新，
  // 提交时再读 row 拿到的是新版本，防覆盖就失效了。
  form.value = {
    name: row.name,
    team_id: row.team_id,
    game_id: row.game_id,
    content_type: row.content_type,
    template_key: row.template_key,
    template_version: row.template_version,
    video_ratio: row.parameters?.video_ratio || '',
    allowed_overrides: Array.isArray(row.allowed_overrides) ? row.allowed_overrides.slice() : [],
    status: row.status,
    expected_version: row.current_version,
  }
  fieldErrors.value = {}
  formError.value = ''
  visible.value = true
}

function openDetail(row) {
  detailRow.value = row
  detailVisible.value = true
}

function openCopy(row) {
  copyTarget.value = row
  copyName.value = `${row.name} 副本`
  copyError.value = ''
  copyVisible.value = true
}

async function openVersions(row) {
  versionTarget.value = row
  versions.value = []
  versionsError.value = ''
  versionsVisible.value = true
  try {
    versions.value = await client.listVersions(row.id)
  } catch (e) {
    versionsError.value = e.message || '版本加载失败'
  }
}

function validateForm() {
  const errors = {}
  const name = String(form.value.name || '').trim()
  if (!name) errors.name = '请输入策略名称'
  else if (name.length > 120) errors.name = '策略名称不能超过 120 字'
  if (me.value?.role === 'admin' && !form.value.team_id) errors.team = '请选择所属团队'
  if (!form.value.game_id) errors.game = '请选择所属游戏'
  const contentType = String(form.value.content_type || '').trim()
  if (!contentType) errors.contentType = '请输入内容类型'
  else if (contentType.length > 64) errors.contentType = '内容类型不能超过 64 字'
  if (!selectedTemplate.value) errors.template = '请选择合成模板'
  if (!form.value.video_ratio) errors.ratio = '请选择画面比例'
  return errors
}

function buildPayload() {
  // 逐字段显式构造：服务端 DisallowUnknownFields，散开 form 会把期望版本等
  // 非合同字段一起发出去，换来具名的 15201。
  return {
    team_id: me.value.role === 'admin' ? form.value.team_id : me.value.team_id,
    game_id: form.value.game_id,
    name: String(form.value.name || '').trim(),
    content_type: String(form.value.content_type || '').trim(),
    template_key: form.value.template_key,
    template_version: form.value.template_version,
    parameters: {
      video_ratio: form.value.video_ratio,
      // 目录 1.0 没有字幕/音乐/片尾的资源合同，服务端只接受 false。
      subtitle: false, bgm: false, outro: false,
    },
    allowed_overrides: (form.value.allowed_overrides || []).slice(),
    status: form.value.status,
  }
}

async function save() {
  fieldErrors.value = validateForm()
  if (Object.keys(fieldErrors.value).length) return
  saving.value = true
  formError.value = ''
  try {
    const payload = buildPayload()
    if (editingID.value) await client.updateStrategy(editingID.value, form.value.expected_version, payload)
    else await client.createStrategy(payload)
    visible.value = false
    await load()
    feedback.toast.success(editingID.value ? '策略已更新' : '策略已保存')
  } catch (e) {
    if (e.errcode === 15204) {
      formError.value = e.message || '策略已被更新，请关闭后重新打开编辑'
      await load()
    } else if (e.errcode === 15201) {
      formError.value = e.message || '合成策略参数无效，请检查各字段'
    } else {
      formError.value = e.message || '策略保存失败'
    }
  } finally {
    saving.value = false
  }
}

async function saveCopy() {
  if (!String(copyName.value || '').trim()) {
    copyError.value = '请输入副本名称'
    return
  }
  copySaving.value = true
  try {
    await client.copyStrategy(copyTarget.value.id, copyName.value.trim())
    copyVisible.value = false
    await load()
    feedback.toast.success('已创建独立副本')
  } catch (e) {
    copyError.value = e.message || '复制失败'
  } finally {
    copySaving.value = false
  }
}
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true">
    <div class="wt-resource-page compose-strategy-page">
      <FeedbackAlert v-if="error" theme="error" :message="error" closable style="margin-bottom:16px" @close="error=''" />
      <ResourcePageHeader title="合成策略" description="维护本业务的合成策略版本；普通运营可查看有权使用的启用策略">
        <template #actions>
          <t-button v-if="canManage" theme="primary" @click="openCreate">新增策略</t-button>
          <t-button class="wt-secondary-button" variant="outline" @click="load">刷新</t-button>
        </template>
      </ResourcePageHeader>

      <ResourceStatGrid :items="stats" />

      <ResourceCard class="compose-filter-card">
        <div class="filter-row">
          <label class="filter-field"><span>策略名称</span><t-input v-model="filters.name" clearable placeholder="搜索策略名称" style="width:220px" /></label>
          <label class="filter-field"><span>状态</span><t-select v-model="filters.status" clearable placeholder="全部" style="width:140px">
            <t-option value="active" label="启用" />
            <t-option value="inactive" label="停用" />
          </t-select></label>
          <t-button theme="primary" @click="load">查询</t-button>
          <t-button class="wt-secondary-button" variant="outline" @click="resetFilters">重置</t-button>
        </div>
      </ResourceCard>

      <ResourceCard class="compose-card">
        <div class="table-scroll-wrap">
          <t-table class="wt-resource-table" :data="filteredRows" :columns="columns" row-key="id" hover :scroll="tableScroll" empty="暂无合成策略">
            <template #game="{ row }">{{ gameName(row.game_id) }}</template>
            <template #template="{ row }">{{ templateLabel(row) }}</template>
            <template #ratio="{ row }">{{ row.parameters?.video_ratio || '-' }}</template>
            <template #overrides="{ row }">
              <span v-if="(row.allowed_overrides || []).length">{{ row.allowed_overrides.map(overrideLabel).join('、') }}</span>
              <span v-else>-</span>
            </template>
            <template #updated_at="{ row }">{{ formatDateTime(row.updated_at) }}</template>
            <template #status="{ row }"><ResourceStatusBadge :tone="row.status === 'active' ? 'success' : 'neutral'" :label="row.status === 'active' ? '启用' : '停用'" /></template>
            <template #op="{ row }">
              <t-space size="small">
                <t-button size="small" class="wt-secondary-button" variant="outline" @click="openDetail(row)">详情</t-button>
                <t-button size="small" class="wt-secondary-button" variant="outline" @click="openVersions(row)">版本</t-button>
                <template v-if="canManage">
                  <t-button size="small" class="wt-secondary-button" variant="outline" @click="openEdit(row)">编辑</t-button>
                  <t-button size="small" class="wt-secondary-button" variant="outline" @click="openCopy(row)">复制</t-button>
                </template>
              </t-space>
            </template>
          </t-table>
        </div>
      </ResourceCard>

      <t-dialog v-model:visible="visible" :header="editingID ? '编辑合成策略' : '新增合成策略'" width="720px" :footer="false">
        <div class="compose-editor">
          <t-form label-width="96px">
            <div class="compose-section">
              <div class="compose-section__title">基础信息</div>
              <t-form-item label="策略名称" required-mark>
                <t-input v-model="form.name" placeholder="例如：火影每日高光" :status="fieldErrors.name ? 'error' : undefined" />
                <FormError id="strategy-name-error" :message="fieldErrors.name" />
              </t-form-item>
              <t-form-item v-if="me?.role === 'admin'" label="所属团队" required-mark>
                <t-select v-model="form.team_id" placeholder="选择团队">
                  <t-option v-for="team in teams" :key="team.id" :value="team.id" :label="team.name" />
                </t-select>
                <FormError id="strategy-team-error" :message="fieldErrors.team" />
              </t-form-item>
              <t-form-item v-else label="所属团队">
                <span class="compose-fixed-value">{{ me?.team_name || '-' }}</span>
              </t-form-item>
              <t-form-item label="所属游戏" required-mark>
                <t-select v-model="form.game_id" placeholder="选择游戏">
                  <t-option v-for="game in gameOptions" :key="game.id" :value="game.id" :label="game.name" />
                </t-select>
                <FormError id="strategy-game-error" :message="fieldErrors.game" />
              </t-form-item>
              <t-form-item label="内容类型" required-mark>
                <t-input v-model="form.content_type" placeholder="例如：短视频" :status="fieldErrors.contentType ? 'error' : undefined" />
                <FormError id="strategy-content-type-error" :message="fieldErrors.contentType" />
              </t-form-item>
            </div>

            <div class="compose-section">
              <div class="compose-section__title">模板与参数</div>
              <t-form-item label="合成模板" required-mark>
                <t-select v-model="templateChoice" placeholder="选择模板">
                  <t-option v-for="template in templates" :key="`${template.key}@${template.version}`" :value="`${template.key}@${template.version}`" :label="`${template.name}（${template.key} ${template.version}）`" />
                </t-select>
                <FormError id="strategy-template-error" :message="fieldErrors.template" />
              </t-form-item>
              <t-form-item label="画面比例" required-mark>
                <t-radio-group v-model="form.video_ratio">
                  <t-radio v-for="ratio in selectedTemplate?.video_ratios || []" :key="ratio" :value="ratio">{{ ratio }}</t-radio>
                </t-radio-group>
                <FormError id="strategy-ratio-error" :message="fieldErrors.ratio" />
              </t-form-item>
              <t-form-item label="可覆盖参数">
                <t-checkbox-group v-model="form.allowed_overrides">
                  <t-checkbox v-for="field in selectedTemplate?.allowed_overrides || []" :key="field" :value="field">{{ overrideLabel(field) }}</t-checkbox>
                </t-checkbox-group>
                <div class="compose-field-hint">勾选后，普通运营提交合成任务时可修改该参数</div>
              </t-form-item>
              <t-form-item label="字幕"><t-switch :model-value="false" :disabled="true" /> <!-- 字幕：当前模板版本未提供资源，不可开启 --></t-form-item>
              <t-form-item label="背景音乐"><t-switch :model-value="false" :disabled="true" /> <!-- 背景音乐：当前模板版本未提供资源，不可开启 --></t-form-item>
              <t-form-item label="片尾"><t-switch :model-value="false" :disabled="true" /> <!-- 片尾：当前模板版本未提供资源，不可开启 --></t-form-item>
            </div>

            <div class="compose-section">
              <div class="compose-section__title">状态</div>
              <t-form-item label="状态"><t-switch v-model="form.status" :custom-value="['active', 'inactive']" /></t-form-item>
              <div v-if="editingID" class="compose-field-hint">当前版本 v{{ form.expected_version }}，保存后生成新版本；历史版本不可变</div>
            </div>
          </t-form>

          <FeedbackAlert v-if="formError" theme="error" :message="formError" style="margin-top:12px" />
          <div class="compose-editor__footer">
            <t-button variant="outline" @click="visible = false">取消</t-button>
            <t-button theme="primary" :loading="saving" @click="save">保存</t-button>
          </div>
        </div>
      </t-dialog>

      <t-dialog v-model:visible="copyVisible" header="复制策略" width="480px" :footer="false">
        <div class="compose-copy">
          <p class="compose-field-hint">以「{{ copyTarget?.name }}」的当前配置创建一条独立策略（版本回到 1），原策略不受影响。</p>
          <t-form label-width="80px">
            <t-form-item label="副本名称" required-mark>
              <t-input v-model="copyName" placeholder="输入新策略名称" />
            </t-form-item>
          </t-form>
          <FeedbackAlert v-if="copyError" theme="error" :message="copyError" style="margin:8px 0" />
          <div class="compose-editor__footer">
            <t-button variant="outline" @click="copyVisible = false">取消</t-button>
            <t-button theme="primary" :loading="copySaving" @click="saveCopy">复制</t-button>
          </div>
        </div>
      </t-dialog>

      <t-dialog v-model:visible="versionsVisible" :header="`版本历史 · ${versionTarget?.name || ''}`" width="640px" :footer="false">
        <div class="compose-versions">
          <FeedbackAlert v-if="versionsError" theme="error" :message="versionsError" />
          <p v-if="!versionsError && !versions.length" class="compose-field-hint">暂无版本记录</p>
          <div v-for="item in versions" :key="item.version" class="compose-version">
            <div class="compose-version__head">
              <strong>v{{ item.version }}</strong>
              <span v-if="versionTarget && item.version === versionTarget.current_version" class="compose-version__current">当前</span>
              <span class="compose-version__time">{{ formatDateTime(item.created_at) }}</span>
            </div>
            <div class="compose-version__body">
              <span>名称：{{ item.snapshot?.name || '-' }}</span>
              <span>模板：{{ item.snapshot ? `${item.snapshot.template_key} ${item.snapshot.template_version}` : '-' }}</span>
              <span>画面比例：{{ item.snapshot?.parameters?.video_ratio || '-' }}</span>
              <span>可覆盖参数：{{ (item.snapshot?.allowed_overrides || []).map(overrideLabel).join('、') || '无' }}</span>
              <span>状态：{{ item.snapshot?.status === 'active' ? '启用' : '停用' }}</span>
            </div>
          </div>
        </div>
      </t-dialog>

      <t-drawer :close-btn="true" v-model:visible="detailVisible" header="策略详情" size="480px" :footer="false">
        <div v-if="detailRow" class="compose-drawer">
          <div class="compose-drawer__field"><span>策略名称</span><strong>{{ detailRow.name || '-' }}</strong></div>
          <div class="compose-drawer__field"><span>所属团队</span><strong>{{ teamName(detailRow.team_id) }}</strong></div>
          <div class="compose-drawer__field"><span>所属游戏</span><strong>{{ gameName(detailRow.game_id) }}</strong></div>
          <div class="compose-drawer__field"><span>内容类型</span><strong>{{ detailRow.content_type || '-' }}</strong></div>
          <div class="compose-drawer__field"><span>合成模板</span><strong>{{ templateLabel(detailRow) }}</strong></div>
          <div class="compose-drawer__field"><span>画面比例</span><strong>{{ detailRow.parameters?.video_ratio || '-' }}</strong></div>
          <div class="compose-drawer__field"><span>可覆盖参数</span><strong>{{ (detailRow.allowed_overrides || []).map(overrideLabel).join('、') || '无' }}</strong></div>
          <div class="compose-drawer__field"><span>当前版本</span><strong>v{{ detailRow.current_version }}</strong></div>
          <div class="compose-drawer__field"><span>状态</span><strong>{{ detailRow.status === 'active' ? '启用' : '停用' }}</strong></div>
          <div class="compose-drawer__field"><span>创建时间</span><strong>{{ formatDateTime(detailRow.created_at) }}</strong></div>
          <div class="compose-drawer__field"><span>修改时间</span><strong>{{ formatDateTime(detailRow.updated_at) }}</strong></div>
        </div>
      </t-drawer>
    </div>
  </t-loading>
</template>

<style scoped>
.compose-filter-card { padding: 14px 20px; }
.filter-row { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.filter-field { display: flex; align-items: center; gap: 8px; color: var(--wt-text-secondary); font-size: 13px; font-weight: 500; white-space: nowrap; }
.compose-card { padding: 18px 20px; margin-top: 16px; }
.compose-editor { display: flex; flex-direction: column; }
.compose-section { border: 1px solid var(--wt-border); border-radius: 10px; padding: 12px 14px; margin-bottom: 14px; }
.compose-section__title { margin-bottom: 10px; color: var(--wt-text-primary); font-size: 14px; font-weight: 600; }
.compose-fixed-value { color: var(--wt-text-primary); font-size: 14px; }
.compose-field-hint { color: var(--wt-text-tertiary); font-size: 12px; line-height: 1.55; }
.compose-editor__footer { display: flex; justify-content: flex-end; gap: 8px; margin-top: 16px; }
.compose-copy { display: flex; flex-direction: column; }
.compose-versions { display: flex; flex-direction: column; gap: 12px; }
.compose-version { border: 1px solid var(--wt-border); border-radius: 8px; padding: 10px 12px; }
.compose-version__head { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; }
.compose-version__head strong { color: var(--wt-text-primary); font-size: 14px; }
.compose-version__current { padding: 1px 8px; border-radius: 10px; background: var(--wt-info-bg); color: var(--wt-primary); font-size: 12px; }
.compose-version__time { margin-left: auto; color: var(--wt-text-tertiary); font-size: 12px; }
.compose-version__body { display: flex; flex-direction: column; gap: 4px; color: var(--wt-text-secondary); font-size: 13px; }
.compose-drawer { display: flex; flex-direction: column; gap: 10px; }
.compose-drawer__field { display: flex; justify-content: space-between; gap: 12px; padding: 10px 12px; border: 1px solid var(--wt-border); border-radius: 8px; }
.compose-drawer__field span { color: var(--wt-text-tertiary); font-size: 12px; flex-shrink: 0; }
.compose-drawer__field strong { color: var(--wt-text-primary); font-size: 14px; font-weight: 600; text-align: right; word-break: break-all; }
</style>
