<script setup>
import { computed, onMounted, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { createDiscoveryClient } from '../../../shared/api/discovery.js'
import ResourceCard from '../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../shared/ui/resource/ResourcePageHeader.vue'
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'

const client = createDiscoveryClient()
const rows = ref([])
const tasks = ref([])
const loading = ref(false)
const error = ref('')
const visible = ref(false)
const saving = ref(false)
const editingID = ref(null)
const keywordText = ref('')
const form = ref(defaultForm())

const columns = [
  { colKey: 'name', title: '策略名称', minWidth: 180 },
  { colKey: 'strategy_type', title: '策略类型', width: 110 },
  { colKey: 'platform', title: '平台', width: 90 },
  { colKey: 'schedule', title: '执行周期', width: 130 },
  { colKey: 'material_rule', title: '自动转素材', width: 150 },
  { colKey: 'latest', title: '最近执行', minWidth: 190 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'op', title: '操作', width: 300, fixed: 'right' },
]

const latestTasks = computed(() => {
  const grouped = new Map()
  for (const task of tasks.value) {
    if (!task.strategy_id) continue
    if (!grouped.has(task.strategy_id)) grouped.set(task.strategy_id, task)
  }
  return grouped
})

onMounted(load)

function defaultForm() {
  return {
    name: '',
    strategy_type: 'keyword',
    platform: 'douyin',
    config: {
      keywords: [],
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
    const [strategyRows, taskRows] = await Promise.all([client.listStrategies(), client.listTasks()])
    rows.value = Array.isArray(strategyRows) ? strategyRows : []
    tasks.value = Array.isArray(taskRows) ? taskRows : []
  } catch (e) {
    error.value = e.message || '策略加载失败'
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingID.value = null
  form.value = defaultForm()
  keywordText.value = ''
  visible.value = true
}

function openEdit(row) {
  editingID.value = row.id
  const config = row.config || {}
  form.value = {
    name: row.name,
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
  keywordText.value = (config.keywords || []).join('\n')
  visible.value = true
}

async function save() {
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
    if (form.value.strategy_type === 'keyword') {
      config.keywords = keywordText.value.split(/[,\n]/).map((item) => item.trim()).filter(Boolean)
    }
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

function typeLabel(type) {
  return type === 'author' ? '博主策略' : '关键词策略'
}

function materialLabel(row) {
  const config = row.config || {}
  if (!config.auto_material) return '关闭'
  const conditions = []
  if (Number(config.like_threshold) > 0) conditions.push(`点赞≥${Number(config.like_threshold).toLocaleString()}`)
  if (Number(config.favorite_threshold) > 0) conditions.push(`收藏≥${Number(config.favorite_threshold).toLocaleString()}`)
  return `${config.material_rule || 'AND'}：${conditions.join(' / ')}`
}

function latestSummary(strategyID) {
  const task = latestTasks.value.get(strategyID)
  if (!task) return '尚未执行'
  const stats = task.stats || {}
  return `${statusLabel(task.status)} · 新增${stats.added || 0} · 自动${stats.auto_materialized || 0} · 待处理${stats.pending || 0}`
}

function statusLabel(status) {
  return ({ pending: '待执行', running: '执行中', success: '成功', failed: '失败' })[status] || status || '未知'
}

function statusTone(status) {
  return ({ pending: 'warning', running: 'info', success: 'success', failed: 'danger' })[status] || 'neutral'
}
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true">
    <div class="wt-resource-page strategy-page">
      <t-alert v-if="error" theme="error" :message="error" closable style="margin-bottom:16px" @close="error=''" />
      <ResourcePageHeader title="挖掘策略" description="定义关键词自动发现规则与自动转素材阈值，执行由 Cloud Scheduler 与 Crawler 负责">
        <template #actions>
          <t-button theme="primary" @click="openCreate">新增策略</t-button>
          <t-button class="wt-secondary-button" variant="outline" @click="load">刷新</t-button>
        </template>
      </ResourcePageHeader>
      <ResourceCard class="strategy-card">
        <div class="table-scroll-wrap">
          <t-table class="wt-resource-table" :data="rows" :columns="columns" row-key="id" hover :scroll="{ x: '1300px' }" empty="暂无挖掘策略">
            <template #strategy_type="{ row }">{{ typeLabel(row.strategy_type) }}</template>
            <template #material_rule="{ row }">{{ materialLabel(row) }}</template>
            <template #latest="{ row }">{{ latestSummary(row.id) }}</template>
            <template #status="{ row }"><ResourceStatusBadge :tone="row.status === 'enabled' ? 'success' : 'neutral'" :label="row.status === 'enabled' ? '启用' : '停用'" /></template>
            <template #op="{ row }">
              <t-space class="wt-resource-actions">
                <t-button size="small" theme="primary" :disabled="row.status !== 'enabled' || row.strategy_type === 'author'" @click="run(row)">立即执行</t-button>
                <t-button size="small" class="wt-secondary-button" variant="outline" @click="openEdit(row)">编辑</t-button>
                <t-button size="small" class="wt-secondary-button" variant="outline" @click="toggle(row)">{{ row.status === 'enabled' ? '停用' : '启用' }}</t-button>
                <t-button size="small" class="wt-secondary-button" variant="outline" @click="$router.push(`/crawl-tasks?strategy_id=${row.id}`)">执行记录</t-button>
                <t-button size="small" class="wt-secondary-button" variant="outline" @click="$router.push(`/content-pool?strategy_id=${row.id}`)">内容结果</t-button>
              </t-space>
            </template>
          </t-table>
        </div>
      </ResourceCard>
      <t-dialog v-model:visible="visible" :header="editingID ? '编辑挖掘策略' : '新增挖掘策略'" width="620px" :confirm-btn="{ loading: saving, theme: 'primary', content: '保存' }" @confirm="save">
        <t-form label-width="104px">
          <t-form-item label="策略名称"><t-input v-model="form.name" placeholder="例如：王者荣耀热点" /></t-form-item>
          <t-form-item label="策略类型">
            <t-radio-group v-model="form.strategy_type">
              <t-radio value="keyword">关键词</t-radio>
              <t-radio value="author" disabled>博主（维护中）</t-radio>
            </t-radio-group>
          </t-form-item>
          <t-form-item label="平台">
            <t-select v-model="form.platform">
              <t-option value="douyin" label="抖音" />
              <t-option value="bilibili" label="B站（待接入）" disabled />
            </t-select>
          </t-form-item>
          <t-form-item v-if="form.strategy_type === 'keyword'" label="关键词"><t-textarea v-model="keywordText" placeholder="每行一个关键词" /></t-form-item>
          <t-form-item v-else label="博主账号"><t-input v-model="form.config.author" disabled placeholder="接口维护中，暂不可配置" /></t-form-item>
          <t-form-item label="执行周期">
            <t-select v-model="form.schedule">
              <t-option value="manual" label="手动执行" />
              <t-option value="daily 09:00" label="每天 09:00" />
              <t-option value="interval:60" label="每 60 分钟" />
            </t-select>
          </t-form-item>
          <t-form-item label="状态"><t-switch v-model="form.status" :custom-value="['enabled', 'disabled']" /></t-form-item>
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
    </div>
  </t-loading>
</template>

<style scoped>.strategy-card { padding: 18px 20px; }</style>
