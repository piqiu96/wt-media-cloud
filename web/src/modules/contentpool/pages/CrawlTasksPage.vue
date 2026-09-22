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

const columns = [
  { colKey: 'id', title: '任务 ID', width: 90 },
  { colKey: 'task_id', title: '执行任务', width: 150 },
  { colKey: 'strategy', title: '来源策略', minWidth: 170 },
  { colKey: 'platform', title: '平台', width: 90 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'stats', title: '执行结果', minWidth: 240 },
  { colKey: 'created_at', title: '创建时间', width: 170 },
  { colKey: 'op', title: '操作', width: 180, fixed: 'right' },
]

const resultColumns = [
  { colKey: 'title', title: '标题', minWidth: 220 },
  { colKey: 'platform_content_id', title: '内容 ID', width: 140 },
  { colKey: 'like_count', title: '点赞', width: 100 },
  { colKey: 'favorite_count', title: '收藏', width: 100 },
  { colKey: 'processing_status', title: '处理状态', width: 120 },
  { colKey: 'source_content_id', title: '内容池', width: 100 },
  { colKey: 'failure_reason', title: '失败原因', minWidth: 180 },
]

const hasFailedItems = computed(() => Boolean(detail.value?.results?.some((item) => ['failed', 'material_failed'].includes(item.processing_status))))

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

async function open(row) {
  try {
    detail.value = await client.getTask(row.id)
    visible.value = true
  } catch (e) {
    error.value = e.message || '任务详情加载失败'
  }
}

async function retryFailed() {
  if (!detail.value) return
  retrying.value = true
  try {
    detail.value = await client.retryFailed(detail.value.id)
    await load()
    MessagePlugin.success('重试任务已创建')
  } catch (e) {
    error.value = e.message || '失败项重试失败'
  } finally {
    retrying.value = false
  }
}

async function openTask(id) {
  try {
    detail.value = await client.getTask(id)
    visible.value = true
  } catch (e) {
    error.value = e.message || '任务详情加载失败'
  }
}

function strategyLabel(row) {
  return row.snapshot?.strategy_name || (row.strategy_id ? `策略 #${row.strategy_id}` : '人工任务')
}

function materialRuleLabel(snapshot) {
  if (!snapshot?.auto_material) return '关闭'
  const conditions = []
  if (Number(snapshot.like_threshold) > 0) conditions.push(`点赞≥${Number(snapshot.like_threshold).toLocaleString()}`)
  if (Number(snapshot.favorite_threshold) > 0) conditions.push(`收藏≥${Number(snapshot.favorite_threshold).toLocaleString()}`)
  return `${snapshot.material_rule || 'AND'}：${conditions.join(' / ')}`
}

function processingLabel(value) {
  return ({
    unprocessed: '未入池',
    pending: '待处理',
    auto_materialized: '自动转素材',
    material_failed: '转素材失败',
    duplicate: '重复内容',
    failed: '失败',
  })[value] || value || '未知'
}

function processingTone(value) {
  return ({
    pending: 'warning',
    auto_materialized: 'success',
    material_failed: 'danger',
    duplicate: 'neutral',
    failed: 'danger',
  })[value] || 'info'
}

function statusLabel(status) {
  return ({ pending: '待执行', running: '执行中', success: '成功', partial_success: '部分成功', failed: '失败' })[status] || status || '未知'
}

function statusTone(status) {
  return status === 'success' ? 'success' : status === 'failed' ? 'danger' : status === 'running' ? 'info' : 'warning'
}

function taskTypeLabel(value) {
  return ({ discovery_task: '策略挖掘', manual_discovery_task: '人工发现', retry_failed_task: '失败重试' })[value] || value || '未知'
}

function dateLabel(value) {
  return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-'
}
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true">
    <div class="wt-resource-page task-page">
      <t-alert v-if="error" theme="error" :message="error" closable style="margin-bottom:16px" @close="error=''" />
      <ResourcePageHeader title="挖掘任务" description="查看策略快照、发现结果、自动转素材和失败重试记录">
        <template #actions>
          <t-button class="wt-secondary-button" variant="outline" @click="load">刷新</t-button>
        </template>
      </ResourcePageHeader>
      <ResourceCard class="task-card">
        <div class="table-scroll-wrap">
          <t-table class="wt-resource-table" :data="rows" :columns="columns" row-key="id" hover :scroll="{ x: '1300px' }" empty="暂无挖掘任务">
            <template #strategy="{ row }">{{ strategyLabel(row) }}</template>
            <template #status="{ row }"><ResourceStatusBadge :tone="statusTone(row.status)" :label="statusLabel(row.status)" /></template>
            <template #stats="{ row }">新增 {{ row.stats?.added || 0 }} · 重复 {{ row.stats?.duplicate || 0 }} · 自动 {{ row.stats?.auto_materialized || 0 }} · 待处理 {{ row.stats?.pending || 0 }} · 失败 {{ row.stats?.failed || 0 }}</template>
            <template #created_at="{ row }">{{ dateLabel(row.created_at) }}</template>
            <template #op="{ row }">
              <t-space>
                <t-button size="small" class="wt-secondary-button" variant="outline" @click="open(row)">查看详情</t-button>
                <t-button size="small" class="wt-secondary-button" variant="outline" @click="router.push(`/content-pool?crawl_task_id=${row.id}`)">内容结果</t-button>
              </t-space>
            </template>
          </t-table>
        </div>
      </ResourceCard>
      <t-drawer v-model:visible="visible" header="挖掘任务详情" size="760px" :footer="false">
        <div v-if="detail" class="task-detail">
          <t-descriptions bordered :column="1">
            <t-descriptions-item label="任务">{{ detail.id }} / {{ detail.task_id || '-' }}</t-descriptions-item>
            <t-descriptions-item label="策略快照">{{ strategyLabel(detail) }}</t-descriptions-item>
            <t-descriptions-item label="平台 / 类型">{{ detail.platform }} / {{ taskTypeLabel(detail.task_type) }}</t-descriptions-item>
            <t-descriptions-item v-if="detail.parent_task_id" label="关联原任务">
              <t-link @click="openTask(detail.parent_task_id)">#{{ detail.parent_task_id }}</t-link>
            </t-descriptions-item>
            <t-descriptions-item label="执行周期">{{ detail.snapshot?.schedule || '-' }}</t-descriptions-item>
            <t-descriptions-item label="自动转素材">{{ materialRuleLabel(detail.snapshot) }}</t-descriptions-item>
            <t-descriptions-item label="状态"><ResourceStatusBadge :tone="statusTone(detail.status)" :label="statusLabel(detail.status)" /></t-descriptions-item>
            <t-descriptions-item label="统计">扫描 {{ detail.stats?.scanned || 0 }}，发现 {{ detail.stats?.found || 0 }}，新增 {{ detail.stats?.added || 0 }}，重复 {{ detail.stats?.duplicate || 0 }}，自动转素材 {{ detail.stats?.auto_materialized || 0 }}，待处理 {{ detail.stats?.pending || 0 }}，失败 {{ detail.stats?.failed || 0 }}</t-descriptions-item>
            <t-descriptions-item label="错误">{{ detail.error || '-' }}</t-descriptions-item>
          </t-descriptions>
          <div class="result-actions">
            <t-button v-if="hasFailedItems" size="small" theme="primary" :loading="retrying" @click="retryFailed">重试失败项</t-button>
          </div>
          <t-table class="wt-resource-table" :data="detail.results || []" :columns="resultColumns" row-key="platform_content_id" hover size="small" :scroll="{ y: '360px' }" empty="暂无发现结果">
            <template #title="{ row }">{{ row.title || row.failure_key || '未命名内容' }}</template>
            <template #like_count="{ row }">{{ Number(row.like_count || 0).toLocaleString() }}</template>
            <template #favorite_count="{ row }">{{ Number(row.favorite_count || 0).toLocaleString() }}</template>
            <template #processing_status="{ row }"><ResourceStatusBadge :tone="processingTone(row.processing_status)" :label="processingLabel(row.processing_status)" /></template>
            <template #source_content_id="{ row }">
              <t-link v-if="row.source_content_id" @click="router.push(`/content-pool?crawl_task_id=${detail.id}`)">#{{ row.source_content_id }}</t-link>
              <span v-else>-</span>
            </template>
          </t-table>
        </div>
      </t-drawer>
    </div>
  </t-loading>
</template>

<style scoped>
.task-card { padding: 18px 20px; }
.task-detail { display: flex; flex-direction: column; gap: 14px; }
.result-actions { display: flex; justify-content: flex-end; }
</style>
