<script setup>
import { computed, onMounted, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { useRoute, useRouter } from 'vue-router'
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
const router = useRouter()
const isLibrary = computed(() => route.path === '/material-library')
const rows = ref([])
const loading = ref(false)
const error = ref('')
const search = ref('')
const platform = ref('')
const sourceType = ref('')
const status = ref('')
const strategyFilter = ref(route.query.strategy_id ? String(route.query.strategy_id) : '')
const taskFilter = ref(route.query.crawl_task_id ? String(route.query.crawl_task_id) : '')
const materialFilter = ref(route.query.material_id ? String(route.query.material_id) : '')
const reviewMode = ref(false)
const reviewQueue = ref([])
const reviewIndex = ref(0)
const currentReviewID = ref(null)
const reviewNote = ref('')
const reviewError = ref('')
const reviewProcessing = ref(false)
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

const pendingCount = computed(() => rows.value.filter((row) => row.status === 'pending').length)

const pagedRows = computed(() => {
  const start = (pagination.value.current - 1) * pagination.value.pageSize
  return rows.value.slice(start, start + pagination.value.pageSize)
})

const stats = computed(() => [
  { key: 'all', label: '全部内容', value: rows.value.length, tone: 'info' },
  { key: 'pending', label: '待处理', value: pendingCount.value, tone: 'warning' },
  { key: 'material_created', label: '已转素材', value: rows.value.filter((row) => row.status === 'material_created').length, tone: 'success' },
  { key: 'ignored', label: '已忽略', value: rows.value.filter((row) => row.status === 'ignored').length, tone: 'neutral' },
])

const columns = [
  { colKey: 'row-select', type: 'multiple', width: 48 },
  { colKey: 'id', title: 'ID', width: 80 },
  { colKey: 'title', title: '内容', minWidth: 340 },
  { colKey: 'platform', title: '平台', width: 110 },
  { colKey: 'author_name', title: '作者', width: 140 },
  { colKey: 'like_count', title: '点赞', width: 100 },
  { colKey: 'favorite_count', title: '收藏', width: 100 },
  { colKey: 'source_type', title: '来源方式', width: 110 },
  { colKey: 'strategy_id', title: '来源策略', minWidth: 180 },
  { colKey: 'crawl_task_id', title: '来源任务', minWidth: 240 },
  { colKey: 'published_at', title: '发布时间', width: 170 },
  { colKey: 'created_at', title: '发现时间', width: 170 },
  { colKey: 'status', title: '状态', width: 120 },
  { colKey: 'op', title: '操作', width: 260, fixed: 'right' },
]

onMounted(() => {
  load()
  loadManualContext()
})

async function load() {
  loading.value = true
  error.value = ''
  try {
    const result = await client.list({ search: search.value.trim(), platform: platform.value, source_type: sourceType.value, status: isLibrary.value ? 'material_created' : status.value, strategy_id: strategyFilter.value || undefined, crawl_task_id: taskFilter.value || undefined, material_id: materialFilter.value || undefined })
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
  search.value = ''; platform.value = ''; sourceType.value = ''; status.value = ''; strategyFilter.value = ''; taskFilter.value = ''; materialFilter.value = ''
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
    return true
  } catch (e) {
    error.value = e.message || '内容详情加载失败'
    return false
  }
}

async function materialize(row, reload = true) {
  try {
    await client.materialize(row.id)
    if (reload) await load()
    MessagePlugin.success('已转为素材，来源关系已保留')
    return true
  } catch (e) {
    error.value = e.message || '转素材失败'
    return false
  }
}

async function updateStatus(row, nextStatus, auditNote = '', reload = true) {
  try {
    await client.setStatus(row.id, nextStatus, nextStatus === 'ignored' ? '人工忽略' : '', auditNote)
    if (reload) await load()
    MessagePlugin.success(nextStatus === 'ignored' ? '内容已忽略' : '内容已恢复待处理')
    return true
  } catch (e) {
    error.value = e.message || '状态更新失败'
    return false
  }
}

function viewMaterial(row) {
  if (!row.material_id) return
  materialFilter.value = String(row.material_id)
  status.value = ''
  router.push({ path: '/material-library', query: { material_id: String(row.material_id) } })
  load()
}

async function batchMaterialize() {
  if (!selectedRowKeys.value.length) return
  batchLoading.value = true
  try {
    const result = await client.batchMaterialize(selectedRowKeys.value.map(Number))
    selectedRowKeys.value = []
    await load()
    MessagePlugin.success(`转素材成功 ${result.succeeded || 0} 条，失败 ${result.failed || 0} 条`)
  } catch (e) {
    error.value = e.message || '批量转素材失败'
  } finally {
    batchLoading.value = false
  }
}

async function batchSetStatus(nextStatus, reason) {
  if (!selectedRowKeys.value.length) return
  batchLoading.value = true
  try {
    const result = await client.batchSetStatus(selectedRowKeys.value.map(Number), nextStatus, reason, reviewNote.value)
    selectedRowKeys.value = []
    await load()
    MessagePlugin.success(`处理成功 ${result.succeeded || 0} 条，失败 ${result.failed || 0} 条`)
  } catch (e) {
    error.value = e.message || '批量处理失败'
  } finally {
    batchLoading.value = false
  }
}

async function enterReviewMode() {
  status.value = 'pending'
  await load()
  const pending = rows.value.filter((row) => row.status === 'pending')
  if (!pending.length) {
    reviewMode.value = false
    MessagePlugin.info('当前没有待处理内容')
    return
  }
  reviewQueue.value = pending
  reviewIndex.value = 0
  currentReviewID.value = pending[0].id
  reviewMode.value = true
  reviewNote.value = ''
  reviewError.value = ''
  await openDetail(pending[0])
}

async function reviewAction(action) {
  if (reviewProcessing.value) return
  if (action === 'skip') {
    advanceReview()
    return
  }
  const row = rows.value.find((item) => item.id === currentReviewID.value)
  if (!row) return
  reviewProcessing.value = true
  reviewError.value = ''
  try {
    if (action === 'materialize') {
      const material = await client.materialize(row.id)
      row.status = 'material_created'
      row.material_id = material.id
    } else if (action === 'ignore') {
      const updated = await client.setStatus(row.id, 'ignored', '人工忽略', reviewNote.value)
      Object.assign(row, updated)
    } else {
      return
    }
    try {
      detail.value = await client.get(row.id)
    } catch {
      // The action succeeded; a detail refresh failure must not keep the item pending.
    }
    advanceReview()
  } catch (e) {
    reviewError.value = e.message || '审核操作失败，当前内容保持可处理状态'
  } finally {
    reviewProcessing.value = false
  }
}

async function advanceReview() {
  if (reviewIndex.value >= reviewQueue.value.length - 1) {
    reviewMode.value = false
    reviewQueue.value = []
    reviewIndex.value = 0
    currentReviewID.value = null
    reviewError.value = ''
    detailVisible.value = false
    await load()
    MessagePlugin.success('待处理内容已审核完成')
    return
  }
  reviewIndex.value += 1
  const next = reviewQueue.value[reviewIndex.value]
  currentReviewID.value = next.id
  reviewNote.value = ''
  reviewError.value = ''
  await openDetail(next)
}

function strategyLabel(row) { return row.strategy_name || (row.strategy_id ? `策略 #${row.strategy_id}` : '-') }
function taskLabel(row) { return row.crawl_task_name || (row.crawl_task_id ? `任务 #${row.crawl_task_id}` : '-') }
function platformLabel(value) { return ({ douyin: '抖音', bilibili: 'B站' })[value] || value || '-' }
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
          <t-button v-if="!isLibrary" class="wt-secondary-button" variant="outline" :disabled="!pendingCount" @click="enterReviewMode">进入审核模式</t-button>
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
            <div class="wt-filter-actions"><t-button theme="primary" @click="load">查询</t-button><t-button class="wt-secondary-button" variant="outline" @click="reset">重置</t-button><t-button v-if="!isLibrary && selectedRowKeys.length" theme="primary" :loading="batchLoading" @click="batchMaterialize">批量转素材</t-button><t-button v-if="!isLibrary && selectedRowKeys.length" class="wt-secondary-button" variant="outline" :loading="batchLoading" @click="batchSetStatus('ignored', '批量人工忽略')">批量忽略</t-button><t-button v-if="!isLibrary && selectedRowKeys.length" class="wt-secondary-button" variant="outline" :loading="batchLoading" @click="batchSetStatus('pending', '')">批量恢复</t-button></div>
          </t-space>
        </div>
        <div class="table-scroll-wrap">
          <t-table class="wt-resource-table" :data="pagedRows" :columns="columns" row-key="id" hover size="small" :scroll="{ x: '1760px' }" v-model:selected-row-keys="selectedRowKeys" empty="暂无内容">
            <template #title="{ row }">
              <div class="title-cell">
                <div class="title-media">
                  <img v-if="row.cover_url" :src="row.cover_url" alt="" loading="lazy" referrerpolicy="no-referrer" />
                  <span v-else>暂无封面</span>
                </div>
                <div class="title-copy"><span>{{ row.title || '未命名内容' }}</span><small>{{ row.platform_content_id }}</small></div>
              </div>
            </template>
            <template #source_type="{ row }">{{ sourceTypeLabel(row.source_type) }}</template>
            <template #like_count="{ row }">{{ Number(row.like_count || 0).toLocaleString() }}</template>
            <template #favorite_count="{ row }">{{ Number(row.favorite_count || 0).toLocaleString() }}</template>
            <template #strategy_id="{ row }"><t-link v-if="row.strategy_id" @click="router.push(`/crawl-tasks?strategy_id=${row.strategy_id}`)">{{ strategyLabel(row) }}</t-link><span v-else>-</span></template>
            <template #crawl_task_id="{ row }"><t-link v-if="row.crawl_task_id" @click="router.push(`/crawl-tasks?task_id=${row.crawl_task_id}`)">{{ taskLabel(row) }}</t-link><span v-else>-</span></template>
            <template #published_at="{ row }">{{ dateLabel(row.published_at) }}</template>
            <template #created_at="{ row }">{{ dateLabel(row.created_at) }}</template>
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
      <t-drawer v-model:visible="detailVisible" class="content-detail-drawer" :header="reviewMode ? '内容审核' : '内容详情'" size="min(72vw, 1200px)" destroy-on-close :footer="reviewMode">
        <div v-if="detail" class="detail-workspace">
          <div v-if="reviewMode" class="review-progress">
            <span>待审核 {{ reviewQueue.length }}</span>
            <strong>当前 {{ reviewIndex + 1 }} / {{ reviewQueue.length }}</strong>
          </div>
          <section class="detail-hero">
            <div class="detail-cover">
              <img v-if="detail.cover_url" :src="detail.cover_url" alt="" referrerpolicy="no-referrer" />
              <span v-else>暂无封面</span>
            </div>
            <div class="detail-primary">
              <div class="detail-title-row">
                <h3>{{ detail.title || '未命名内容' }}</h3>
                <ResourceStatusBadge :tone="statusTone(detail.status)" :label="statusLabel(detail.status)" />
              </div>
              <div class="detail-meta-grid">
                <div><span>作者</span><strong>{{ detail.author_name || '-' }}</strong></div>
                <div><span>平台</span><strong>{{ platformLabel(detail.platform) }}</strong></div>
                <div><span>发布时间</span><strong>{{ dateLabel(detail.published_at) }}</strong></div>
                <div><span>发现时间</span><strong>{{ dateLabel(detail.created_at) }}</strong></div>
              </div>
              <div class="detail-metrics">
                <div><span>点赞</span><strong>{{ Number(detail.like_count || 0).toLocaleString() }}</strong></div>
                <div><span>收藏</span><strong>{{ Number(detail.favorite_count || 0).toLocaleString() }}</strong></div>
              </div>
              <p v-if="detail.description" class="detail-description">{{ detail.description }}</p>
            </div>
          </section>
          <div class="detail-columns">
            <section class="detail-panel">
              <h4>来源追溯</h4>
              <dl>
                <div><dt>来源方式</dt><dd>{{ sourceTypeLabel(detail.source_type) }}</dd></div>
                <div><dt>来源策略</dt><dd><t-link v-if="detail.strategy_id" @click="router.push(`/crawl-tasks?strategy_id=${detail.strategy_id}`)">{{ strategyLabel(detail) }}</t-link><span v-else>-</span></dd></div>
                <div><dt>来源任务</dt><dd><t-link v-if="detail.crawl_task_id" @click="router.push(`/crawl-tasks?task_id=${detail.crawl_task_id}`)">{{ taskLabel(detail) }}</t-link><span v-else>-</span></dd></div>
                <div><dt>平台内容 ID</dt><dd>{{ detail.platform_content_id || '-' }}</dd></div>
              </dl>
            </section>
            <section class="detail-panel">
              <h4>处理结果</h4>
              <dl>
                <div><dt>当前状态</dt><dd><ResourceStatusBadge :tone="statusTone(detail.status)" :label="statusLabel(detail.status)" /></dd></div>
                <div><dt>关联素材</dt><dd><t-link v-if="detail.material_id" @click="viewMaterial(detail)">查看素材</t-link><span v-else>未转换</span></dd></div>
                <div><dt>审核备注</dt><dd>{{ detail.audit_note || '-' }}</dd></div>
                <div><dt>失败原因</dt><dd :class="{ 'is-danger': detail.failure_reason }">{{ detail.failure_reason || '-' }}</dd></div>
              </dl>
            </section>
          </div>
          <div class="detail-actions">
            <a v-if="detail.source_url" class="detail-primary-link" :href="detail.source_url" target="_blank" rel="noreferrer">打开原作品</a>
            <t-button v-if="detail.material_id" class="wt-secondary-button" variant="outline" @click="viewMaterial(detail)">查看素材</t-button>
            <template v-if="!reviewMode && !isLibrary && detail.status === 'pending'">
              <t-button theme="primary" @click="materialize(detail)">转素材</t-button>
              <t-button class="wt-secondary-button" variant="outline" @click="updateStatus(detail, 'ignored')">忽略</t-button>
            </template>
            <t-button v-if="!reviewMode && !isLibrary && detail.status === 'ignored'" class="wt-secondary-button" variant="outline" @click="updateStatus(detail, 'pending')">恢复待处理</t-button>
          </div>
        </div>
        <template #footer>
          <div v-if="reviewMode" class="review-footer">
            <t-alert v-if="reviewError" theme="error" :message="reviewError" />
            <t-textarea v-model="reviewNote" :rows="3" placeholder="审核备注（可选）" />
            <div class="review-actions">
              <t-button theme="primary" :loading="reviewProcessing" @click="reviewAction('materialize')">转素材并下一条</t-button>
              <t-button class="wt-secondary-button" variant="outline" :loading="reviewProcessing" @click="reviewAction('ignore')">忽略并下一条</t-button>
              <t-button class="wt-secondary-button" variant="outline" :disabled="reviewProcessing" @click="reviewAction('skip')">跳过</t-button>
            </div>
          </div>
        </template>
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
          <template #title="{ row }">
              <div class="title-cell">
                <div class="title-media">
                  <img v-if="row.cover_url" :src="row.cover_url" alt="" loading="lazy" referrerpolicy="no-referrer" />
                  <span v-else>暂无封面</span>
                </div>
                <div class="title-copy"><span>{{ row.title || '未命名内容' }}</span><small>{{ row.platform_content_id }}</small></div>
              </div>
            </template>
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
.title-cell { display: flex; align-items: center; gap: 12px; min-height: 64px; }
.title-media { display: flex; align-items: center; justify-content: center; overflow: hidden; width: 88px; height: 56px; border: 1px solid var(--wt-border); border-radius: 8px; background: var(--wt-bg-page); color: var(--wt-text-tertiary); font-size: 12px; flex-shrink: 0; }
.title-media img { width: 100%; height: 100%; object-fit: cover; }
.title-copy { display: flex; min-width: 0; flex-direction: column; gap: 4px; }
.title-copy span { overflow: hidden; color: var(--wt-text-primary); font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }
.title-copy small { overflow: hidden; color: var(--wt-text-tertiary); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.content-pool-page :deep(.wt-resource-actions) { max-width: 360px; }
.content-detail-drawer :deep(.t-drawer) { max-width: 1200px; }
.detail-workspace { display: flex; flex-direction: column; gap: 18px; }
.review-progress { display: flex; align-items: center; justify-content: space-between; padding: 12px 14px; border: 1px solid rgba(37, 99, 235, 0.18); border-radius: 10px; background: var(--wt-info-bg); }
.review-progress span { color: var(--wt-text-tertiary); font-size: 13px; }
.review-progress strong { color: var(--wt-primary); font-size: 14px; }
.detail-hero { display: grid; grid-template-columns: minmax(280px, 38%) 1fr; gap: 20px; }
.detail-cover { display: flex; align-items: center; justify-content: center; overflow: hidden; min-height: 260px; border: 1px solid var(--wt-border); border-radius: 12px; background: var(--wt-bg-page); color: var(--wt-text-tertiary); }
.detail-cover img { width: 100%; height: 100%; object-fit: cover; }
.detail-primary { min-width: 0; }
.detail-title-row { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.detail-title-row h3 { margin: 0; color: var(--wt-text-primary); font-size: 21px; font-weight: 650; line-height: 1.35; }
.detail-meta-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; margin-top: 18px; }
.detail-meta-grid div, .detail-metrics div { padding: 10px 12px; border: 1px solid var(--wt-border); border-radius: 10px; background: var(--wt-bg-card); }
.detail-meta-grid span, .detail-metrics span { display: block; color: var(--wt-text-tertiary); font-size: 12px; }
.detail-meta-grid strong, .detail-metrics strong { display: block; margin-top: 5px; color: var(--wt-text-primary); font-size: 14px; font-weight: 600; }
.detail-metrics { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; margin-top: 12px; }
.detail-description { margin: 14px 0 0; color: var(--wt-text-secondary); font-size: 14px; line-height: 1.65; white-space: pre-line; }
.detail-columns { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
.detail-panel { padding: 14px 16px; border: 1px solid var(--wt-border); border-radius: 12px; background: var(--wt-bg-card); }
.detail-panel h4 { margin: 0 0 12px; color: var(--wt-text-primary); font-size: 15px; font-weight: 650; }
.detail-panel dl { display: flex; flex-direction: column; gap: 10px; margin: 0; }
.detail-panel dl > div { display: grid; grid-template-columns: 96px 1fr; gap: 10px; align-items: baseline; }
.detail-panel dt { color: var(--wt-text-tertiary); font-size: 13px; }
.detail-panel dd { overflow-wrap: anywhere; margin: 0; color: var(--wt-text-primary); font-size: 14px; line-height: 1.5; }
.detail-panel dd.is-danger { color: var(--wt-danger); }
.detail-actions { display: flex; align-items: center; justify-content: flex-end; gap: 10px; padding-top: 4px; border-top: 1px solid var(--wt-border); }
.detail-primary-link { display: inline-flex; align-items: center; justify-content: center; min-height: 32px; padding: 0 16px; border-radius: 6px; background: var(--wt-primary); color: #fff; font-size: 13px; font-weight: 600; text-decoration: none; }
.review-footer { display: flex; flex-direction: column; gap: 10px; }
.review-actions { display: flex; justify-content: flex-end; gap: 8px; }
.manual-search-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 12px; }
@media (max-width: 1180px) {
  .detail-hero, .detail-columns { grid-template-columns: 1fr; }
  .detail-cover { min-height: 220px; }
}
@media (max-width: 760px) {
  .detail-title-row, .detail-actions { align-items: stretch; flex-direction: column; }
  .detail-primary-link { width: 100%; }
  .review-actions { flex-wrap: wrap; }
}
</style>
