<script setup>
import { computed, onMounted, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { useRoute } from 'vue-router'
import { createContentPoolClient } from '../../../shared/api/contentPool.js'
import ResourceCard from '../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../shared/ui/resource/ResourcePageHeader.vue'
import ResourceStatGrid from '../../../shared/ui/resource/ResourceStatGrid.vue'
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'

const client = createContentPoolClient()
const route = useRoute()
const isLibrary = computed(() => route.path === '/material-library')
const rows = ref([])
const loading = ref(false)
const error = ref('')
const search = ref('')
const platform = ref('')
const sourceType = ref('')
const status = ref('')
const detailVisible = ref(false)
const detail = ref(null)
const pagination = ref({ current: 1, pageSize: 20 })
const selectedRowKeys = ref([])
const batchLoading = ref(false)

const pagedRows = computed(() => {
  const start = (pagination.value.current - 1) * pagination.value.pageSize
  return rows.value.slice(start, start + pagination.value.pageSize)
})

const stats = computed(() => [
  { key: 'all', label: '全部内容', value: rows.value.length, tone: 'info' },
  { key: 'pending', label: '待处理', value: rows.value.filter((row) => row.status === 'pending').length, tone: 'warning' },
  { key: 'material_created', label: '已转素材', value: rows.value.filter((row) => row.status === 'material_created').length, tone: 'success' },
  { key: 'ignored', label: '已忽略', value: rows.value.filter((row) => row.status === 'ignored').length, tone: 'neutral' },
])

const columns = [
  { colKey: 'row-select', type: 'multiple', width: 48 },
  { colKey: 'id', title: 'ID', width: 80 },
  { colKey: 'title', title: '标题', minWidth: 260 },
  { colKey: 'platform', title: '平台', width: 110 },
  { colKey: 'author_name', title: '作者', width: 150 },
  { colKey: 'source_type', title: '来源方式', width: 120 },
  { colKey: 'published_at', title: '发布时间', width: 180 },
  { colKey: 'status', title: '状态', width: 120 },
  { colKey: 'op', title: '操作', width: 250, fixed: 'right' },
]

onMounted(load)

async function load() {
  loading.value = true
  error.value = ''
  try {
    const result = await client.list({ search: search.value.trim(), platform: platform.value, source_type: sourceType.value, status: isLibrary.value ? 'material_created' : status.value })
    rows.value = Array.isArray(result) ? result : []
    pagination.value.current = 1
  } catch (e) {
    error.value = e.message || '内容池加载失败'
  } finally {
    loading.value = false
  }
}

function applyStatFilter(key) {
  status.value = key === 'all' ? '' : key
  load()
}

function reset() {
  search.value = ''; platform.value = ''; sourceType.value = ''; status.value = ''
  load()
}

async function openDetail(row) {
  try {
    detail.value = await client.get(row.id)
    detailVisible.value = true
  } catch (e) { error.value = e.message || '内容详情加载失败' }
}

async function materialize(row) {
  try {
    await client.materialize(row.id)
    await load()
    MessagePlugin.success('已转为素材，来源关系已保留')
  } catch (e) { error.value = e.message || '转素材失败' }
}

async function updateStatus(row, nextStatus) {
  try {
    await client.setStatus(row.id, nextStatus, nextStatus === 'ignored' ? '人工忽略' : '')
    await load()
    MessagePlugin.success(nextStatus === 'ignored' ? '内容已忽略' : '内容已恢复待处理')
  } catch (e) { error.value = e.message || '状态更新失败' }
}

async function batchIgnore() {
  if (!selectedRowKeys.value.length) return
  batchLoading.value = true
  try {
    await client.batchSetStatus(selectedRowKeys.value.map(Number), 'ignored', '批量人工忽略')
    selectedRowKeys.value = []
    await load()
    MessagePlugin.success('已批量忽略所选内容')
  } catch (e) { error.value = e.message || '批量处理失败' } finally { batchLoading.value = false }
}

function statusLabel(value) { return ({ pending: '待处理', material_created: '已转素材', ignored: '已忽略' })[value] || value || '未知' }
function statusTone(value) { return ({ pending: 'warning', material_created: 'success', ignored: 'neutral' })[value] || 'info' }
function sourceTypeLabel(value) { return ({ link: '分享链接', search: '关键词搜索', author: '博主搜索', strategy: '挖掘策略' })[value] || value || '-' }
function dateLabel(value) { return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-' }
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true">
    <div class="wt-resource-page content-pool-page">
      <t-alert v-if="error" theme="error" :message="error" closable style="margin-bottom:16px" @close="error=''" />
      <ResourcePageHeader :title="isLibrary ? '素材库' : '内容池'" :description="isLibrary ? '查看已从内容池沉淀的素材及其来源关系' : '统一管理人工发现与自动挖掘进入系统的外部内容'">
        <template #actions>
          <t-button v-if="!isLibrary" theme="primary" @click="MessagePlugin.info('链接导入将在 M3-B 接入抖音解析适配器')">导入链接</t-button>
          <t-button class="wt-secondary-button" variant="outline" @click="load">刷新</t-button>
        </template>
      </ResourcePageHeader>
      <ResourceStatGrid v-if="!isLibrary" :items="stats" @select="applyStatFilter" />
      <ResourceCard class="content-pool-card">
        <div class="filter-bar">
          <t-space wrap>
            <label class="wt-filter-field"><span class="wt-filter-field__label">综合搜索</span><t-input v-model="search" clearable placeholder="标题、内容 ID、作者" /></label>
            <label class="wt-filter-field"><span class="wt-filter-field__label">平台</span><t-select v-model="platform" clearable placeholder="全部"><t-option value="douyin" label="抖音" /><t-option value="bilibili" label="B站" /></t-select></label>
            <label class="wt-filter-field"><span class="wt-filter-field__label">来源方式</span><t-select v-model="sourceType" clearable placeholder="全部"><t-option value="link" label="分享链接" /><t-option value="search" label="关键词搜索" /><t-option value="author" label="博主搜索" /><t-option value="strategy" label="挖掘策略" /></t-select></label>
            <label v-if="!isLibrary" class="wt-filter-field"><span class="wt-filter-field__label">处理状态</span><t-select v-model="status" clearable placeholder="全部"><t-option value="pending" label="待处理" /><t-option value="material_created" label="已转素材" /><t-option value="ignored" label="已忽略" /></t-select></label>
            <div class="wt-filter-actions"><t-button theme="primary" @click="load">查询</t-button><t-button class="wt-secondary-button" variant="outline" @click="reset">重置</t-button><t-button v-if="!isLibrary && selectedRowKeys.length" class="wt-secondary-button" variant="outline" :loading="batchLoading" @click="batchIgnore">批量忽略</t-button></div>
          </t-space>
        </div>
        <div class="table-scroll-wrap">
          <t-table class="wt-resource-table" :data="pagedRows" :columns="columns" row-key="id" hover size="small" :scroll="{ x: '1100px' }" v-model:selected-row-keys="selectedRowKeys" empty="暂无内容">
            <template #title="{ row }"><div class="title-cell"><span>{{ row.title || '未命名内容' }}</span><small>{{ row.platform_content_id }}</small></div></template>
            <template #source_type="{ row }">{{ sourceTypeLabel(row.source_type) }}</template>
            <template #published_at="{ row }">{{ dateLabel(row.published_at) }}</template>
            <template #status="{ row }"><ResourceStatusBadge :tone="statusTone(row.status)" :label="statusLabel(row.status)" /></template>
            <template #op="{ row }">
              <t-space class="wt-resource-actions">
                <t-button size="small" class="wt-secondary-button" variant="outline" @click="openDetail(row)">查看</t-button>
                <t-button v-if="!isLibrary && row.status === 'pending'" size="small" theme="primary" @click="materialize(row)">转素材</t-button>
                <t-button v-if="!isLibrary && row.status !== 'ignored' && row.status !== 'material_created'" size="small" class="wt-secondary-button" variant="outline" @click="updateStatus(row, 'ignored')">忽略</t-button>
                <t-button v-if="!isLibrary && row.status === 'ignored'" size="small" class="wt-secondary-button" variant="outline" @click="updateStatus(row, 'pending')">恢复</t-button>
              </t-space>
            </template>
          </t-table>
        </div>
        <div class="pagination-bar"><t-pagination v-model:current="pagination.current" v-model:pageSize="pagination.pageSize" :total="rows.length" :page-size-options="[10, 20, 50]" /></div>
      </ResourceCard>
      <t-drawer v-model:visible="detailVisible" header="内容详情" size="520px" destroy-on-close :footer="false">
        <t-descriptions v-if="detail" bordered :column="1">
          <t-descriptions-item label="标题">{{ detail.title || '-' }}</t-descriptions-item>
          <t-descriptions-item label="平台 / 内容 ID">{{ detail.platform }} / {{ detail.platform_content_id }}</t-descriptions-item>
          <t-descriptions-item label="作者">{{ detail.author_name || '-' }}</t-descriptions-item>
          <t-descriptions-item label="来源方式">{{ sourceTypeLabel(detail.source_type) }}</t-descriptions-item>
          <t-descriptions-item label="来源链接"><a v-if="detail.source_url" :href="detail.source_url" target="_blank" rel="noreferrer">{{ detail.source_url }}</a><span v-else>-</span></t-descriptions-item>
          <t-descriptions-item label="状态"><ResourceStatusBadge :tone="statusTone(detail.status)" :label="statusLabel(detail.status)" /></t-descriptions-item>
        </t-descriptions>
      </t-drawer>
    </div>
  </t-loading>
</template>

<style scoped>
.content-pool-card { padding: 18px 20px; }
.title-cell { display: flex; flex-direction: column; gap: 3px; }
.title-cell span { color: var(--wt-text-primary); font-weight: 500; }
.title-cell small { color: var(--wt-text-tertiary); font-size: 12px; }
.content-pool-page :deep(.wt-resource-actions) { max-width: 360px; }
</style>
