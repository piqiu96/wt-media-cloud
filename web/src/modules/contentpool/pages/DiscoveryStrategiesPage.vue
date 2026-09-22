<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'
import { createDiscoveryClient } from '../../../shared/api/discovery.js'
import ResourceCard from '../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../shared/ui/resource/ResourcePageHeader.vue'
import ResourceStatGrid from '../../../shared/ui/resource/ResourceStatGrid.vue'
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'
import KeywordTags from '../../../shared/ui/resource/KeywordTags.vue'

const client = createDiscoveryClient()
const route = useRoute()
const rows = ref([])
const tasks = ref([])
const games = ref([])
const loading = ref(false)
const error = ref('')
const visible = ref(false)
const saving = ref(false)
const editingID = ref(null)
const form = ref(defaultForm())
const scheduleMode = ref('manual')
const scheduleTime = ref('09:00')
const keywordVisible = ref(false)
const authorVisible = ref(false)
const keywordTarget = ref(null)
const authorTarget = ref(null)

const filters = ref({ name: '', type: '', status: '' })

const columns = [
  { colKey: 'id', title: 'ID', width: 70 },
  { colKey: 'name', title: '策略名称', minWidth: 180 },
  { colKey: 'game', title: '游戏', width: 120 },
  { colKey: 'type', title: '类型', width: 90 },
  { colKey: 'rule', title: '挖掘规则', width: 130 },
  { colKey: 'schedule', title: '执行周期', width: 130 },
  { colKey: 'material', title: '转素材规则', minWidth: 170 },
  { colKey: 'latest', title: '最近效果', minWidth: 200 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'op', title: '操作', width: 260, fixed: 'right' },
]

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

const filteredRows = computed(() => rows.value.filter((row) => {
  if (filters.value.name && !String(row.name || '').toLowerCase().includes(filters.value.name.trim().toLowerCase())) return false
  if (filters.value.type && row.strategy_type !== filters.value.type) return false
  if (filters.value.status && row.status !== filters.value.status) return false
  return true
}))

const stats = computed(() => {
  const today = new Date().toDateString()
  const todayRuns = tasks.value.filter((task) => new Date(task.created_at).toDateString() === today).length
  const auto = tasks.value.reduce((sum, task) => sum + (Number(task.stats?.auto_materialized) || 0), 0)
  return [
    { key: 'total', label: '策略总数', value: rows.value.length, tone: 'info' },
    { key: 'enabled', label: '启用策略', value: rows.value.filter((row) => row.status === 'enabled').length, tone: 'success' },
    { key: 'today', label: '今日执行', value: todayRuns, tone: 'warning' },
    { key: 'auto', label: '自动素材', value: auto, tone: 'info' },
  ]
})

onMounted(async () => {
  await load()
  const id = Number(route.query.strategy_id || 0)
  if (id > 0) {
    const row = rows.value.find((item) => item.id === id)
    if (row) openEdit(row)
  }
})

function defaultForm() {
  return {
    name: '',
    game_id: '',
    strategy_type: 'keyword',
    platform: 'douyin',
    config: {
      keywords: [],
      authors: [],
      author: '',
      auto_material: false,
      material_rule: 'AND',
      like_threshold: 0,
      favorite_threshold: 0,
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
    const [strategyRows, taskRows, gameRows] = await Promise.all([client.listStrategies(), client.listTasks(), client.listGames()])
    rows.value = Array.isArray(strategyRows) ? strategyRows : []
    tasks.value = Array.isArray(taskRows) ? taskRows : []
    games.value = Array.isArray(gameRows) ? gameRows : []
  } catch (e) {
    error.value = e.message || '策略加载失败'
  } finally {
    loading.value = false
  }
}

function resetFilters() {
  filters.value = { name: '', type: '', status: '' }
}

function openCreate() {
  editingID.value = null
  form.value = defaultForm()
  scheduleMode.value = 'manual'
  scheduleTime.value = '09:00'
  visible.value = true
}

function fillForm(row) {
  const config = row.config || {}
  form.value = {
    name: row.name,
    game_id: row.game_id || '',
    strategy_type: row.strategy_type,
    platform: row.platform,
    config: {
      ...config,
      auto_material: Boolean(config.auto_material),
      material_rule: config.material_rule || 'AND',
      like_threshold: Number(config.like_threshold || 0),
      favorite_threshold: Number(config.favorite_threshold || 0),
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

function openCopy(row) {
  editingID.value = null
  fillForm(row)
  form.value.name = `${row.name} 副本`
  visible.value = true
}

async function save() {
  if (!String(form.value.name || '').trim()) {
    MessagePlugin.warning('请输入策略名称')
    return
  }
  if (form.value.config.auto_material && Number(form.value.config.like_threshold) <= 0 && Number(form.value.config.favorite_threshold) <= 0) {
    MessagePlugin.warning('开启自动转素材后，至少设置一个正数阈值')
    return
  }
  saving.value = true
  try {
    const config = {
      ...form.value.config,
      auto_material: Boolean(form.value.config.auto_material),
      material_rule: form.value.config.material_rule || 'AND',
      like_threshold: Number(form.value.config.like_threshold || 0),
      favorite_threshold: Number(form.value.config.favorite_threshold || 0),
    }
    if (!String(form.value.game_id || '').trim()) {
      MessagePlugin.warning('请选择所属游戏')
      return
    }
    if (!String(form.value.platform || '').trim()) {
      MessagePlugin.warning('请选择平台')
      return
    }
    form.value.schedule = scheduleMode.value === 'manual' ? 'manual' : `daily ${scheduleTime.value}`
    config.keywords = Array.isArray(config.keywords) ? config.keywords.map((item) => String(item).trim()).filter(Boolean) : []
    config.authors = Array.isArray(config.authors) ? config.authors.map((item) => String(item).trim()).filter(Boolean) : []
    const payload = { ...form.value, config }
    if (editingID.value) await client.updateStrategy(editingID.value, payload)
    else await client.createStrategy(payload)
    visible.value = false
    await load()
    MessagePlugin.success(editingID.value ? '策略已更新' : '策略已保存')
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
    MessagePlugin.success('已创建挖掘任务')
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
        MessagePlugin.success('策略已删除')
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

function formatThreshold(value) {
  const n = Number(value || 0)
  if (n <= 0) return ''
  if (n >= 10000) {
    const w = n / 10000
    return `${Number.isInteger(w) ? w : w.toFixed(1).replace(/\.0$/, '')}万`
  }
  if (n >= 1000) {
    const k = n / 1000
    return `${Number.isInteger(k) ? k : k.toFixed(1).replace(/\.0$/, '')}千`
  }
  return String(n)
}

function materialLabel(row) {
  const config = row.config || {}
  if (!config.auto_material) return '关闭'
  const like = Number(config.like_threshold || 0)
  const favorite = Number(config.favorite_threshold || 0)
  const parts = []
  if (like > 0) parts.push(`赞≥${formatThreshold(like)}`)
  if (favorite > 0) parts.push(`藏≥${formatThreshold(favorite)}`)
  if (!parts.length) return '关闭'
  const joiner = config.material_rule === 'OR' ? ' 或 ' : ' 且 '
  return parts.join(joiner)
}

function scheduleLabel(schedule) {
  if (!schedule || schedule === 'manual') return '手动执行'
  if (String(schedule).startsWith('daily ')) return `每天 ${String(schedule).slice(6)}`
  if (String(schedule).startsWith('interval:')) return `每 ${String(schedule).slice(9)} 分钟`
  return schedule
}

function latestSummary(row) {
  const task = latestTasks.value.get(row.id)
  if (!task) return '-'
  const s = task.stats || {}
  return `发现 ${s.found || 0} · 新增 ${s.added || 0} · 自动 ${s.auto_materialized || 0} · 待审核 ${s.pending || 0}`
}
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true">
    <div class="wt-resource-page strategy-page">
      <t-alert v-if="error" theme="error" :message="error" closable style="margin-bottom:16px" @close="error=''" />
      <ResourcePageHeader title="挖掘策略" description="配置内容发现规则，让系统自动帮你找到优质内容">
        <template #actions>
          <t-button theme="primary" @click="openCreate">新增策略</t-button>
          <t-button class="wt-secondary-button" variant="outline" @click="load">刷新</t-button>
        </template>
      </ResourcePageHeader>

      <ResourceStatGrid :items="stats" />

      <ResourceCard class="strategy-filter-card">
        <div class="filter-row">
          <t-input v-model="filters.name" clearable placeholder="搜索策略名称" style="width:220px" @enter="load" />
          <t-select v-model="filters.type" clearable placeholder="策略类型" style="width:140px">
            <t-option value="keyword" label="关键词" />
            <t-option value="author" label="作者" />
          </t-select>
          <t-select v-model="filters.status" clearable placeholder="状态" style="width:140px">
            <t-option value="enabled" label="启用" />
            <t-option value="disabled" label="停用" />
          </t-select>
          <t-button theme="primary" @click="load">查询</t-button>
          <t-button class="wt-secondary-button" variant="outline" @click="resetFilters">重置</t-button>
        </div>
      </ResourceCard>

      <ResourceCard class="strategy-card">
        <div class="table-scroll-wrap">
          <t-table class="wt-resource-table" :data="filteredRows" :columns="columns" row-key="id" hover :scroll="{ x: '1260px' }" empty="暂无挖掘策略">
            <template #game="{ row }">{{ gameName(row.game_id) }}</template>
            <template #type="{ row }">{{ typeLabel(row.strategy_type) }}</template>
            <template #rule="{ row }">
              <a class="strategy-rule-link" @click="row.strategy_type === 'author' ? openAuthors(row) : openKeywords(row)">{{ ruleLabel(row) }}</a>
            </template>
            <template #schedule="{ row }">{{ scheduleLabel(row.schedule) }}</template>
            <template #material="{ row }">{{ materialLabel(row) }}</template>
            <template #latest="{ row }">{{ latestSummary(row) }}</template>
            <template #status="{ row }"><ResourceStatusBadge :tone="row.status === 'enabled' ? 'success' : 'neutral'" :label="row.status === 'enabled' ? '启用' : '停用'" /></template>
            <template #op="{ row }">
              <t-space size="small">
                <t-button size="small" theme="primary" :disabled="row.status !== 'enabled' || isRunning(row) || row.strategy_type === 'author'" @click="run(row)">执行</t-button>
                <t-button size="small" class="wt-secondary-button" variant="outline" @click="$router.push(`/crawl-tasks?strategy_id=${row.id}`)">记录</t-button>
                <t-button size="small" class="wt-secondary-button" variant="outline" :disabled="isRunning(row)" @click="openEdit(row)">编辑</t-button>
                <t-button size="small" class="wt-secondary-button" variant="outline" @click="openCopy(row)">复制</t-button>
                <t-button size="small" class="wt-secondary-button" variant="outline" theme="danger" :disabled="isRunning(row)" @click="remove(row)">删除</t-button>
              </t-space>
            </template>
          </t-table>
        </div>
      </ResourceCard>

      <t-dialog v-model:visible="visible" :header="editingID ? '编辑挖掘策略' : '新增挖掘策略'" width="640px" :confirm-btn="{ loading: saving, theme: 'primary', content: '保存' }" @confirm="save">
        <t-form label-width="96px">
          <div class="form-section">基础信息</div>
          <t-form-item label="策略名称" required-mark><t-input v-model="form.name" placeholder="例如：王者荣耀热点" /></t-form-item>
          <t-form-item label="所属游戏" required-mark><t-select v-model="form.game_id" clearable placeholder="选择游戏分类"><t-option v-for="game in games" :key="game.id" :value="game.id" :label="game.name" /></t-select></t-form-item>
          <t-form-item label="平台" required-mark>
            <t-select v-model="form.platform">
              <t-option value="douyin" label="抖音" />
              <t-option value="bilibili" label="B站（待接入）" disabled />
            </t-select>
          </t-form-item>
          <t-form-item label="类型">
            <t-radio-group v-model="form.strategy_type">
              <t-radio value="keyword">关键词</t-radio>
              <t-radio value="author" disabled>作者（维护中）</t-radio>
            </t-radio-group>
          </t-form-item>

          <div class="form-section">挖掘对象</div>
          <t-form-item v-if="form.strategy_type === 'keyword'" :label="`关键词 · 已配置 ${(form.config.keywords || []).length} 个`">
            <KeywordTags v-model="form.config.keywords" placeholder="输入关键词后按回车添加，支持批量粘贴" />
          </t-form-item>
          <t-form-item v-else :label="`作者 · 已配置 ${(form.config.authors || []).length} 个`">
            <KeywordTags v-model="form.config.authors" placeholder="输入作者昵称或 ID 后按回车添加" />
          </t-form-item>

          <div class="form-section">执行计划</div>
          <t-form-item label="执行周期">
            <t-select v-model="form.schedule">
              <t-option value="manual" label="手动执行" />
              <t-option value="daily 09:00" label="每天 09:00" />
              <t-option value="interval:60" label="每 60 分钟" />
            </t-select>
          </t-form-item>
          <t-form-item label="状态"><t-switch v-model="form.status" :custom-value="['enabled', 'disabled']" /></t-form-item>

          <div class="form-section">转素材规则</div>
          <t-form-item label="自动转素材"><t-switch v-model="form.config.auto_material" /></t-form-item>
          <t-form-item v-if="form.config.auto_material" label="阈值规则">
            <t-radio-group v-model="form.config.material_rule">
              <t-radio value="AND">全部满足</t-radio>
              <t-radio value="OR">任一满足</t-radio>
            </t-radio-group>
          </t-form-item>
          <t-form-item v-if="form.config.auto_material" label="点赞阈值"><t-input-number v-model="form.config.like_threshold" :min="0" :step="1000" theme="column" placeholder="0 表示不参与" /></t-form-item>
          <t-form-item v-if="form.config.auto_material" label="收藏阈值"><t-input-number v-model="form.config.favorite_threshold" :min="0" :step="100" theme="column" placeholder="0 表示不参与" /></t-form-item>
        </t-form>
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
    </div>
  </t-loading>
</template>

<style scoped>
.strategy-card { padding: 18px 20px; margin-top: 16px; }
.strategy-filter-card { padding: 14px 20px; }
.filter-row { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.strategy-rule-link { color: #7c3aed; cursor: pointer; font-weight: 500; }
.strategy-rule-link:hover { text-decoration: underline; }
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
</style>
