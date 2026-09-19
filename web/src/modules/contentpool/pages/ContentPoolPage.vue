<script setup>
import { computed, onMounted, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { useRoute } from 'vue-router'
import { createContentPoolClient } from '../../../shared/api/contentPool.js'
import { createDiscoveryClient } from '../../../shared/api/discovery.js'
import { createSessionClient } from '../../../shared/api/session.js'
import { createUsersClient } from '../../../apps/cloud/pages/users/usersApi.js'
import ResourceCard from '../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../shared/ui/resource/ResourcePageHeader.vue'
import ResourceStatGrid from '../../../shared/ui/resource/ResourceStatGrid.vue'
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'

const client = createContentPoolClient()
const discovery = createDiscoveryClient()
const session = createSessionClient()
const users = createUsersClient()
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
const manualVisible = ref(false)
const manualMode = ref('url')
const manualLoading = ref(false)
const manualSearched = ref(false)
const manualResults = ref([])
const selectedResultKeys = ref([])
const manualPagination = ref({ nextOffset: 0, maxCursor: 0, hasMore: false })
const manualForm = ref({ platform: 'douyin', url: '', keyword: '' })
const manualTeamId = ref('')
const teams = ref([])

const resultColumns = [
  { colKey: 'row-select', type: 'multiple', width: 48 },
  { colKey: 'title', title: '标题', minWidth: 280 },
  { colKey: 'author_name', title: '作者', width: 150 },
  { colKey: 'published_at', title: '发布时间', width: 170 },
]

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

onMounted(() => {
  load()
  loadManualContext()
})

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

function openManual(mode) {
  manualMode.value = mode
  manualSearched.value = false
  manualResults.value = []
  selectedResultKeys.value = []
  resetManualPagination()
  manualForm.value = { platform: 'douyin', url: '', keyword: '' }
  manualVisible.value = true
}

function manualTitle() {
  return ({ url: '分享链接导入', keyword: '关键词搜索' })[manualMode.value]
}

async function loadManualContext() {
  try {
    const user = await session.me()
    if (user?.role === 'admin') {
      const data = await users.listTeams()
      teams.value = Array.isArray(data) ? data : []
    } else {
      manualTeamId.value = user?.team_id || ''
    }
  } catch {
    teams.value = []
  }
}

function selectedTeamID() {
  return manualTeamId.value ? Number(manualTeamId.value) : undefined
}

function resetManualPagination() {
  manualPagination.value = { nextOffset: 0, maxCursor: 0, hasMore: false }
}

async function searchManual(reset = false) {
  if (reset) resetManualPagination()
  manualLoading.value = true
  try {
    const form = manualForm.value
    const result = await discovery.search({
      platform: form.platform,
      keyword: form.keyword.trim(),
      limit: 20,
      offset: manualPagination.value.nextOffset || 0,
    })
    manualResults.value = Array.isArray(result?.items) ? result.items : []
    selectedResultKeys.value = []
    manualSearched.value = true
    manualPagination.value = {
      nextOffset: Number(result?.next_offset || 0),
      maxCursor: Number(result?.max_cursor || 0),
      hasMore: Boolean(result?.has_more),
    }
  } catch (e) {
    error.value = e.message || '搜索失败'
  } finally {
    manualLoading.value = false
  }
}

async function confirmManual() {
  if (manualMode.value === 'url') {
    manualLoading.value = true
    try {
      const form = manualForm.value
      const urls = form.url.split(/\r?\n|,/).map((value) => value.trim()).filter(Boolean)
      await discovery.importUrl({ platform: form.platform, team_id: selectedTeamID(), ...(urls.length > 1 ? { urls } : { url: urls[0] || '' }) })
      manualVisible.value = false
      MessagePlugin.success('链接解析任务已创建，完成后内容会自动进入内容池')
    } catch (e) {
      error.value = e.message || '链接解析任务创建失败'
    } finally {
      manualLoading.value = false
    }
    return
  }
  if (!selectedTeamID()) {
      MessagePlugin.warning('请选择运营团队')
      return
    }
    if (!manualResults.value.length) {
    await searchManual(true)
    return
  }
  await confirmSelected()
}

async function confirmSelected() {
  if (!selectedResultKeys.value.length) {
    MessagePlugin.warning('请至少选择一条内容')
    return
  }
  manualLoading.value = true
  try {
    const selected = manualResults.value.filter((item) => selectedResultKeys.value.includes(item.platform_content_id))
    const result = await discovery.importResults({ platform: manualForm.value.platform, team_id: selectedTeamID(), source_type: 'search', items: selected })
    manualVisible.value = false
    await load()
    MessagePlugin.success(`成功导入 ${result.imported || 0} 条，重复 ${result.duplicate || 0} 条`)
  } catch (e) { error.value = e.message || '内容入池失败' } finally { manualLoading.value = false }
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
          <t-button v-if="!isLibrary" theme="primary" @click="openManual('url')">导入链接</t-button>
          <t-button v-if="!isLibrary" class="wt-secondary-button" variant="outline" @click="openManual('keyword')">关键词搜索</t-button>
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
      <t-dialog v-model:visible="manualVisible" :header="manualTitle()" width="760px" :confirm-btn="{ loading: manualLoading, theme: 'primary', content: manualResults.length && manualMode !== 'url' ? '加入内容池' : '开始执行' }" @confirm="confirmManual">
        <t-form label-width="88px">
          <t-form-item label="运营团队"><t-select v-model="manualTeamId" placeholder="选择内容归属团队"><t-option v-for="team in teams" :key="team.id" :value="team.id" :label="team.name" /></t-select></t-form-item>
          <t-form-item label="平台"><t-select v-model="manualForm.platform"><t-option value="douyin" label="抖音" /><t-option value="bilibili" label="B站（待接入）" disabled /></t-select></t-form-item>
          <t-form-item v-if="manualMode === 'url'" label="内容链接"><t-textarea v-model="manualForm.url" :rows="4" placeholder="粘贴视频链接，支持单条或批量（每行一条）" /></t-form-item>
          <t-form-item v-else label="关键词"><t-input v-model="manualForm.keyword" placeholder="例如：王者荣耀 新英雄" /></t-form-item>
        </t-form>
        <t-alert v-if="manualSearched && manualMode !== 'url'" theme="info" :message="`搜索完成，发现 ${manualResults.length} 条，请选择后加入内容池`" style="margin: 12px 0" />
        <t-table v-if="manualMode !== 'url' && manualResults.length" v-model:selected-row-keys="selectedResultKeys" :data="manualResults" :columns="resultColumns" row-key="platform_content_id" hover size="small" :scroll="{ y: '300px' }" empty="暂无结果">
          <template #title="{ row }"><div class="title-cell"><span>{{ row.title || '未命名内容' }}</span><small>{{ row.platform_content_id }}</small></div></template>
          <template #published_at="{ row }">{{ dateLabel(row.published_at) }}</template>
        </t-table>
        <div v-if="manualMode !== 'url' && manualSearched" class="manual-search-actions">
          <t-button variant="outline" :loading="manualLoading" @click="searchManual(true)">重新搜索</t-button>
          <t-button variant="outline" :loading="manualLoading" :disabled="!manualPagination.hasMore" @click="searchManual()">下一页</t-button>
        </div>
      </t-dialog>
    </div>
  </t-loading>
</template>

<style scoped>
.content-pool-card { padding: 18px 20px; }
.title-cell { display: flex; flex-direction: column; gap: 3px; }
.title-cell span { color: var(--wt-text-primary); font-weight: 500; }
.title-cell small { color: var(--wt-text-tertiary); font-size: 12px; }
.content-pool-page :deep(.wt-resource-actions) { max-width: 360px; }
.manual-search-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 12px; }
</style>
