<script setup>
import { computed, onMounted, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { useRoute, useRouter } from 'vue-router'
import { createDiscoveryClient } from '../../../shared/api/discovery.js'
import ResourceCard from '../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../shared/ui/resource/ResourcePageHeader.vue'
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'

const client = createDiscoveryClient()
const route = useRoute()
const router = useRouter()
const rows = ref([])
const loading = ref(false)
const error = ref('')
const detail = ref(null)
const visible = ref(false)
const retrying = ref(false)
const activeTab = ref('overview')
const filters = ref({ name: '', status: '' })

const columns = [
  { colKey: 'name', title: '任务名称', minWidth: 220 },
  { colKey: 'strategy', title: '来源策略', minWidth: 160 },
  { colKey: 'trigger', title: '触发方式', width: 90 },
  { colKey: 'found', title: '发现结果', minWidth: 240 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'executed_at', title: '执行时间', width: 170 },
  { colKey: 'op', title: '操作', width: 140, fixed: 'right' },
]

const resultColumns = [
  { colKey: 'cover', title: '封面', width: 72 },
  { colKey: 'title', title: '标题', minWidth: 220 },
  { colKey: 'author_name', title: '作者', width: 120 },
  { colKey: 'like_count', title: '点赞', width: 90 },
  { colKey: 'favorite_count', title: '收藏', width: 90 },
  { colKey: 'processing_status', title: '处理状态', width: 110 },
  { colKey: 'op', title: '操作', width: 100 },
]

const filteredRows = computed(() => rows.value.filter((row) => {
  const keyword = filters.value.name.trim().toLowerCase()
  if (keyword) {
    const name = strategyName(row).toLowerCase()
    if (!name.includes(keyword)) return false
  }
  if (filters.value.status && row.status !== filters.value.status) return false
  return true
}))

onMounted(async () => {
  await load()
  const taskID = Number(route.query.task_id || 0)
  if (taskID > 0) {
    const task = rows.value.find((row) => row.id === taskID)
    if (task) await open(task)
  }
})

async function load() {
  loading.value = true
  error.value = ''
  try {
    const strategyID = Number(route.query.strategy_id || 0)
    const data = await client.listTasks(strategyID > 0 ? { strategy_id: strategyID } : undefined)
    rows.value = Array.isArray(data) ? data : []
  } catch (e) {
    error.value = e.message || '任务加载失败'
  } finally {
    loading.value = false
  }
}

function resetFilters() {
  filters.value = { name: '', status: '' }
}

async function open(row) {
  try {
    detail.value = await client.getTask(row.id)
    activeTab.value = 'overview'
    visible.value = true
  } catch (e) {
    error.value = e.message || '任务详情加载失败'
  }
}

async function retry(row) {
  retrying.value = true
  try {
    await client.retryFailed(row.id)
    await load()
    MessagePlugin.success('已重新执行，成功数据会自动跳过')
    visible.value = false
  } catch (e) {
    error.value = e.message || '重新执行失败'
  } finally {
    retrying.value = false
  }
}

function strategyName(row) {
  return row.snapshot?.strategy_name || (row.strategy_id ? `策略 #${row.strategy_id}` : '人工任务')
}

function taskName(row) {
  return `${strategyName(row)}_${compactTime(row.created_at)}`
}

function compactTime(value) {
  if (!value) return '--------'
  const d = new Date(value)
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}${pad(d.getMonth() + 1)}${pad(d.getDate())}-${pad(d.getHours())}${pad(d.getMinutes())}${pad(d.getSeconds())}`
}

function triggerLabel(row) {
  if (row.task_type === 'retry_failed_task') return '重试'
  return row.schedule_key ? '定时' : '手动'
}

function foundSummary(row) {
  const s = row.stats || {}
  return `发现 ${s.found || 0} · 新增 ${s.added || 0} · 自动素材 ${s.auto_materialized || 0} · 待审核 ${s.pending || 0} · 失败 ${s.failed || 0}`
}

function openStrategy(row) {
  if (row.strategy_id) router.push(`/discovery-strategies?strategy_id=${row.strategy_id}`)
}

function statusLabel(status) {
  return ({ pending: '待执行', running: '执行中', success: '成功', partial_success: '部分成功', failed: '失败' })[status] || status || '未知'
}

function statusTone(status) {
  return status === 'success' ? 'success' : status === 'failed' ? 'danger' : status === 'running' ? 'info' : 'warning'
}

function processingLabel(value) {
  return ({ unprocessed: '未入池', pending: '待审核', auto_materialized: '自动转素材', material_failed: '转素材失败', duplicate: '重复', failed: '失败' })[value] || value || '未知'
}

function processingTone(value) {
  return ({ pending: 'warning', auto_materialized: 'success', material_failed: 'danger', duplicate: 'neutral', failed: 'danger' })[value] || 'info'
}

function dateLabel(value) {
  return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-'
}

function timeHM(value) {
  if (!value) return ''
  const d = new Date(value)
  const pad = (n) => String(n).padStart(2, '0')
  return `${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function duration(task) {
  if (!task.started_at || !task.finished_at) return '-'
  const ms = new Date(task.finished_at) - new Date(task.started_at)
  if (ms < 0) return '-'
  const total = Math.round(ms / 1000)
  const min = Math.floor(total / 60)
  const sec = total % 60
  return min > 0 ? `${min}分${sec}秒` : `${sec}秒`
}

function timeline(task) {
  const steps = []
  const s = task.stats || {}
  steps.push({ time: timeHM(task.created_at), label: '创建任务' })
  if (task.started_at) steps.push({ time: timeHM(task.started_at), label: '开始执行策略' })
  steps.push({ time: '', label: `加载关键词并发现内容 ${s.found || 0} 条` })
  steps.push({ time: '', label: `完成去重，新增 ${s.added || 0} 条` })
  if (s.auto_materialized > 0) steps.push({ time: '', label: `执行自动转素材 ${s.auto_materialized} 条` })
  const finished = task.finished_at || (task.status !== 'pending' && task.status !== 'running' ? task.updated_at : null)
  steps.push({ time: finished ? timeHM(finished) : '', label: task.status === 'success' || task.status === 'partial_success' ? '任务完成' : `任务${statusLabel(task.status)}` })
  return steps
}

function exceptions(task) {
  const list = (task.results || [])
    .filter((item) => ['failed', 'material_failed'].includes(item.processing_status))
    .map((item) => ({
      stage: item.processing_status === 'material_failed' ? '转素材失败' : '内容抓取失败',
      reason: item.failure_reason || '平台接口异常',
      scope: item.failure_key || item.title || item.platform_content_id || '-',
      time: task.finished_at || task.updated_at,
    }))
  if (!list.length && task.error) {
    list.push({ stage: '任务失败', reason: task.error, scope: '-', time: task.finished_at || task.updated_at })
  }
  return list
}

function rowAction(item) {
  if (item.processing_status === 'auto_materialized') return { label: '查看素材', to: '/material-library' }
  if (item.processing_status === 'pending') return { label: '审核', to: `/content-pool?crawl_task_id=${detail.value?.id}` }
  if (item.processing_status === 'duplicate') return { label: '查看原内容', to: `/content-pool?crawl_task_id=${detail.value?.id}` }
  return null
}

function truncateTitle(title, max = 120) {
  const text = String(title || '')
  return text.length > max ? `${text.slice(0, max)}…` : text
}

function hasFailed(task) {
  return task.status === 'failed' || task.status === 'partial_success'
}
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true">
    <div class="wt-resource-page task-page">
      <t-alert v-if="error" theme="error" :message="error" closable style="margin-bottom:16px" @close="error=''" />
      <ResourcePageHeader title="挖掘任务" description="查看策略执行情况，发现内容结果，定位异常问题">
        <template #actions>
          <t-button class="wt-secondary-button" variant="outline" @click="load">刷新</t-button>
        </template>
      </ResourcePageHeader>

      <ResourceCard class="task-filter-card">
        <div class="filter-row">
          <t-input v-model="filters.name" clearable placeholder="搜索任务名称 / 来源策略" style="width:240px" />
          <t-select v-model="filters.status" clearable placeholder="状态" style="width:150px">
            <t-option value="success" label="成功" />
            <t-option value="partial_success" label="部分成功" />
            <t-option value="failed" label="失败" />
            <t-option value="running" label="执行中" />
            <t-option value="pending" label="待执行" />
          </t-select>
          <t-button theme="primary" @click="load">查询</t-button>
          <t-button class="wt-secondary-button" variant="outline" @click="resetFilters">重置</t-button>
        </div>
      </ResourceCard>

      <ResourceCard class="task-card">
        <div class="table-scroll-wrap">
          <t-table class="wt-resource-table" :data="filteredRows" :columns="columns" row-key="id" hover :scroll="{ x: '1200px' }" empty="暂无挖掘任务">
            <template #name="{ row }">
              <span class="task-name" :title="taskName(row)">{{ taskName(row) }}</span>
            </template>
            <template #strategy="{ row }">
              <a v-if="row.strategy_id" class="wt-primary-link" @click="openStrategy(row)">{{ strategyName(row) }}</a>
              <span v-else>{{ strategyName(row) }}</span>
            </template>
            <template #trigger="{ row }">{{ triggerLabel(row) }}</template>
            <template #found="{ row }">{{ foundSummary(row) }}</template>
            <template #status="{ row }"><ResourceStatusBadge :tone="statusTone(row.status)" :label="statusLabel(row.status)" /></template>
            <template #executed_at="{ row }">{{ dateLabel(row.created_at) }}</template>
            <template #op="{ row }">
              <t-space size="small">
                <t-button size="small" class="wt-secondary-button" variant="outline" @click="open(row)">详情</t-button>
                <t-button v-if="hasFailed(row)" size="small" theme="primary" @click="retry(row)">重试</t-button>
              </t-space>
            </template>
          </t-table>
        </div>
      </ResourceCard>

      <t-drawer v-model:visible="visible" header="挖掘任务详情" size="760px" :footer="false">
        <div v-if="detail" class="task-detail">
          <div class="detail-head">
            <h3 class="task-title">{{ taskName(detail) }}</h3>
            <ResourceStatusBadge :tone="statusTone(detail.status)" :label="statusLabel(detail.status)" />
          </div>
          <div class="detail-meta">
            <div><span>来源策略</span><strong><a v-if="detail.strategy_id" class="wt-primary-link" @click="openStrategy(detail)">{{ strategyName(detail) }}</a><span v-else>-</span></strong></div>
            <div><span>触发方式</span><strong>{{ triggerLabel(detail) }}</strong></div>
            <div><span>执行时间</span><strong>{{ dateLabel(detail.created_at) }}</strong></div>
            <div><span>耗时</span><strong>{{ duration(detail) }}</strong></div>
          </div>

          <div class="result-summary">
            <div class="summary-tile"><strong>{{ detail.stats?.found || 0 }}</strong><span>发现内容</span></div>
            <div class="summary-tile"><strong>{{ detail.stats?.added || 0 }}</strong><span>新增内容</span></div>
            <div class="summary-tile"><strong>{{ detail.stats?.auto_materialized || 0 }}</strong><span>自动素材</span></div>
            <div class="summary-tile"><strong>{{ detail.stats?.pending || 0 }}</strong><span>待审核</span></div>
            <div class="summary-tile"><strong>{{ detail.stats?.failed || 0 }}</strong><span>失败</span></div>
          </div>

          <div class="result-actions">
            <t-button v-if="hasFailed(detail)" size="small" theme="primary" :loading="retrying" @click="retry(detail)">重新执行</t-button>
          </div>

          <t-tabs v-model="activeTab">
            <t-tab-panel value="overview" label="结果概览">
              <div class="overview-summary">
                <p>共发现 <strong>{{ detail.stats?.found || 0 }}</strong> 条内容，其中 <strong>{{ detail.stats?.added || 0 }}</strong> 条为新增内容，<strong>{{ detail.stats?.auto_materialized || 0 }}</strong> 条已自动转为素材，<strong>{{ detail.stats?.pending || 0 }}</strong> 条待审核。</p>
              </div>
            </t-tab-panel>

            <t-tab-panel value="items" label="发现内容">
              <t-table class="wt-resource-table" :data="detail.results || []" :columns="resultColumns" row-key="platform_content_id" hover size="small" :scroll="{ y: '360px' }" empty="暂无发现结果">
                <template #cover="{ row }">
                  <img v-if="row.cover_url" class="result-cover" :src="row.cover_url" alt="" loading="lazy" referrerpolicy="no-referrer" />
                  <span v-else class="result-cover result-cover--empty">暂无</span>
                </template>
                <template #title="{ row }">
                  <span class="result-title" :title="row.title || row.failure_key || '未命名内容'">{{ truncateTitle(row.title || row.failure_key || '未命名内容') }}</span>
                </template>
                <template #author_name="{ row }">{{ row.author_name || '-' }}</template>
                <template #like_count="{ row }">{{ Number(row.like_count || 0).toLocaleString() }}</template>
                <template #favorite_count="{ row }">{{ Number(row.favorite_count || 0).toLocaleString() }}</template>
                <template #processing_status="{ row }"><ResourceStatusBadge :tone="processingTone(row.processing_status)" :label="processingLabel(row.processing_status)" /></template>
                <template #op="{ row }">
                  <t-button v-if="rowAction(row)" size="small" class="wt-secondary-button" variant="outline" @click="router.push(rowAction(row).to)">{{ rowAction(row).label }}</t-button>
                  <span v-else>-</span>
                </template>
              </t-table>
            </t-tab-panel>

            <t-tab-panel value="process" label="执行过程">
              <div class="timeline">
                <div v-for="(step, index) in timeline(detail)" :key="index" class="timeline-item">
                  <span class="timeline-time">{{ step.time || '—' }}</span>
                  <span class="timeline-label">{{ step.label }}</span>
                </div>
              </div>
            </t-tab-panel>

            <t-tab-panel value="errors" label="异常记录">
              <div v-if="exceptions(detail).length" class="exception-list">
                <div v-for="(item, index) in exceptions(detail)" :key="index" class="exception-item">
                  <div class="exception-head"><strong>{{ item.stage }}</strong><span>{{ dateLabel(item.time) }}</span></div>
                  <div class="exception-body"><span>失败原因</span><p>{{ item.reason }}</p></div>
                  <div class="exception-body"><span>影响范围</span><p>{{ item.scope }}</p></div>
                </div>
              </div>
              <div v-else class="exception-empty">暂无异常记录</div>
            </t-tab-panel>
          </t-tabs>
        </div>
      </t-drawer>
    </div>
  </t-loading>
</template>

<style scoped>
.task-card { padding: 18px 20px; margin-top: 16px; }
.task-filter-card { padding: 14px 20px; }
.filter-row { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.task-detail { display: flex; flex-direction: column; gap: 14px; }
.detail-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.task-title { margin: 0; color: var(--wt-text-primary); font-size: 18px; font-weight: 600; }
.task-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; display: block; }
.detail-meta { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }
.detail-meta > div { padding: 10px 12px; border: 1px solid var(--wt-border); border-radius: 8px; }
.detail-meta span { display: block; color: var(--wt-text-tertiary); font-size: 12px; }
.detail-meta strong { display: block; margin-top: 4px; color: var(--wt-text-primary); font-size: 14px; font-weight: 600; }
.result-summary { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 10px; }
.summary-tile { padding: 14px 10px; border: 1px solid var(--wt-border); border-radius: 8px; text-align: center; }
.summary-tile strong { display: block; color: var(--wt-primary); font-size: 22px; font-weight: 650; }
.summary-tile span { display: block; margin-top: 5px; color: var(--wt-text-tertiary); font-size: 12px; }
.result-actions { display: flex; justify-content: flex-end; }
.result-cover { width: 48px; height: 48px; border-radius: 6px; object-fit: cover; display: flex; align-items: center; justify-content: center; background: var(--wt-bg-page); color: var(--wt-text-tertiary); font-size: 11px; }
.result-title { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; display: block; }
.overview-summary p { margin: 0; color: var(--wt-text-secondary); font-size: 14px; line-height: 1.7; }
.overview-summary strong { color: var(--wt-primary); }
.timeline { display: flex; flex-direction: column; gap: 12px; padding: 6px 2px; }
.timeline-item { display: flex; gap: 14px; align-items: baseline; }
.timeline-time { width: 44px; color: var(--wt-text-tertiary); font-size: 12px; flex-shrink: 0; }
.timeline-label { color: var(--wt-text-primary); font-size: 14px; }
.exception-list { display: flex; flex-direction: column; gap: 10px; }
.exception-item { padding: 12px; border: 1px solid var(--wt-border); border-radius: 8px; }
.exception-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; }
.exception-head strong { color: var(--wt-danger); font-size: 14px; }
.exception-head span { color: var(--wt-text-tertiary); font-size: 12px; }
.exception-body { display: grid; grid-template-columns: 64px 1fr; gap: 8px; margin-top: 6px; }
.exception-body span { color: var(--wt-text-tertiary); font-size: 12px; }
.exception-body p { margin: 0; color: var(--wt-text-primary); font-size: 13px; }
.exception-empty { padding: 24px; text-align: center; color: var(--wt-text-tertiary); font-size: 13px; }
</style>
