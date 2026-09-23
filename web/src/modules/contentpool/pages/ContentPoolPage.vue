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
import KeywordTags from '../../../shared/ui/resource/KeywordTags.vue'

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
const search = ref(route.query.search ? String(route.query.search) : '')
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
const manualMode = ref('id')
const manualLoading = ref(false)
const manualSearched = ref(false)
const manualResults = ref([])
const selectedResultKeys = ref([])
const manualPagination = ref({ nextOffset: 0, maxCursor: 0, hasMore: false })
const manualForm = ref({ platform: 'douyin', idList: [], keyword: '', keywords: [] })
const manualTeamId = ref('')
const manualTeamLocked = ref(false)
const manualGameId = ref('')
const teams = ref([])
const games = ref([])
const imageViewerVisible = ref(false)
const imageViewerImages = ref([])
const imageViewerIndex = ref(0)

const resultColumns = [
  { colKey: 'row-select', type: 'multiple', width: 48 },
  { colKey: 'title', title: '标题', minWidth: 280 },
  { colKey: 'author_name', title: '作者', width: 140 },
  { colKey: 'like_count', title: '点赞', width: 90 },
  { colKey: 'favorite_count', title: '收藏', width: 90 },
  { colKey: 'view_count', title: '浏览', width: 90 },
  { colKey: 'comment_count', title: '评论', width: 90 },
  { colKey: 'share_count', title: '分享', width: 90 },
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
  { colKey: 'game', title: '游戏', width: 110 },
  { colKey: 'author_name', title: '作者', width: 140 },
  { colKey: 'interaction', title: '互动', width: 170 },
  { colKey: 'source_type', title: '来源方式', width: 110 },
  { colKey: 'source', title: '来源', minWidth: 260 },
  { colKey: 'published_at', title: '发布时间', width: 170 },
  { colKey: 'created_at', title: '发现时间', width: 170 },
  { colKey: 'updated_at', title: '修改时间', width: 170 },
  { colKey: 'updated_by_name', title: '修改人', width: 120 },
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
  manualForm.value = { platform: 'douyin', idList: [], keyword: '', keywords: [] }
  manualVisible.value = true
}

function manualTitle() {
  return ({ id: 'ID/链接发现', keyword: '关键词发现' })[manualMode.value]
}

async function loadManualContext() {
  try {
    const user = await session.me()
    manualTeamLocked.value = user?.role !== 'admin'
    manualTeamId.value = user?.role === 'admin' ? '' : (user?.team_id || '')
    const data = await users.listTeams()
    teams.value = Array.isArray(data) ? data : []
    const gameData = await users.listGames()
    games.value = Array.isArray(gameData) ? gameData : []
    if (!manualGameId.value && games.value.length) manualGameId.value = (games.value.find((g) => g.id === 'other') || games.value[0]).id
  } catch {
    teams.value = []
    games.value = []
  }
}

function gameName(id) {
  const game = games.value.find((item) => item.id === id)
  return game?.name || '-'
}

function selectedTeamID() {
  return manualTeamId.value ? Number(manualTeamId.value) : undefined
}

function resetManualPagination() {
  manualPagination.value = { nextOffset: 0, maxCursor: 0, hasMore: false }
}

async function searchManual(reset = false) {
  if (reset) resetManualPagination()
  const form = manualForm.value
  const isDirectSearch = manualMode.value === 'id'
  const targets = isDirectSearch ? parseSearchTargets() : []
  if (isDirectSearch && !targets.length) return
  manualLoading.value = true
  try {
    if (!isDirectSearch && !(form.keywords || []).length) {
      MessagePlugin.warning('请至少添加一个关键词')
      return
    }
    const result = await discovery.search(isDirectSearch ? {
      platform: form.platform,
      query: targets.join('\n'),
    } : {
      platform: form.platform,
      keyword: (form.keywords || []).join(' '),
      limit: 50,
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

function parseSearchTargets() {
  const targets = [...new Set((manualForm.value.idList || []).map((value) => String(value).trim()).filter(Boolean))]
  if (!targets.length) {
    MessagePlugin.warning('请输入至少一个视频 ID 或链接')
    return []
  }
  const valid = targets.every((target) => (
    /^\d+$/.test(target)
    || /^https?:\/\/(www\.)?douyin\.com\/video\/\d+(?:\/.*)?$/.test(target)
    || /^https?:\/\/v\.douyin\.com\/[^\s]+$/.test(target)
  ))
  if (!valid) {
    MessagePlugin.warning('仅支持抖音视频 ID、视频链接或分享短链接')
    return []
  }
  return targets
}

async function confirmManual() {
  if (!manualResults.value.length) {
    await searchManual(true)
    return
  }
  if (!selectedTeamID()) {
    MessagePlugin.warning('请选择运营团队')
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
    const result = await discovery.importResults({ platform: manualForm.value.platform, team_id: selectedTeamID(), game_id: manualGameId.value || null, source_type: 'search', items: selected })
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

function exitReviewMode(reload = false) {
  reviewMode.value = false
  reviewQueue.value = []
  reviewIndex.value = 0
  currentReviewID.value = null
  reviewNote.value = ''
  reviewError.value = ''
  detailVisible.value = false
  if (reload) load()
}

async function advanceReview() {
  if (reviewIndex.value >= reviewQueue.value.length - 1) {
    exitReviewMode(false)
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
function sourceTypeLabel(value) { return ({ link: 'ID/链接发现', search: '关键词发现', author: '博主发现', strategy: '挖掘策略' })[value] || value || '-' }
function dateLabel(value) { return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-' }
function countLabel(value) { return Number(value || 0).toLocaleString() }
function authorLabel(row) { return row.author_name || row.author_uid || row.author_sec_uid || '-' }
function interactionLabel(row) {
  return `浏览 ${countLabel(row.view_count)} · 赞 ${countLabel(row.like_count)} · 藏 ${countLabel(row.favorite_count)}`
}

function truncateTitle(title, max = 80) {
  const text = String(title || '')
  return text.length > max ? `${text.slice(0, max)}…` : text
}

// 封面兜底：封面是未经代理的上游 CDN 绝对地址，可能被 CSP 拦截、防盗链、
// 404 或上游换域名。任何一种都不该在界面上留一个破图占位，
// 而应退化到与「无封面」一致的「暂无封面」。
const failedCovers = ref(new Set())
function markCoverFailed(url) {
  if (!url || failedCovers.value.has(url)) return
  const next = new Set(failedCovers.value)
  next.add(url)
  failedCovers.value = next
}
function coverAvailable(url) {
  return Boolean(url) && !failedCovers.value.has(url)
}

function openImageViewer(url) {
  if (!url) return
  imageViewerImages.value = [url]
  imageViewerIndex.value = 0
  imageViewerVisible.value = true
}
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true">
    <div class="wt-resource-page content-pool-page">
      <t-alert v-if="error" theme="error" :message="error" closable style="margin-bottom:16px" @close="error=''" />
      <ResourcePageHeader :title="isLibrary ? '素材库' : '内容池'" :description="isLibrary ? '查看已从内容池沉淀的素材及其来源关系' : '统一管理人工发现与自动挖掘进入系统的外部内容'">
        <template #actions>
          <t-dropdown v-if="!isLibrary" trigger="click">
            <t-button theme="primary">发现内容</t-button>
            <t-dropdown-menu>
              <t-dropdown-item @click="openManual('id')">ID/链接发现</t-dropdown-item>
              <t-dropdown-item @click="openManual('keyword')">关键词发现</t-dropdown-item>
            </t-dropdown-menu>
          </t-dropdown>
          <t-button
            v-if="!isLibrary"
            class="review-mode-button"
            :class="{ 'is-active': reviewMode }"
            :theme="reviewMode ? 'primary' : 'default'"
            :variant="reviewMode ? 'base' : 'outline'"
            :disabled="!reviewMode && !pendingCount"
            @click="reviewMode ? exitReviewMode() : enterReviewMode()"
          >审核模式</t-button>
          <t-button class="wt-secondary-button" variant="outline" @click="load">刷新</t-button>
        </template>
      </ResourcePageHeader>
      <ResourceStatGrid v-if="!isLibrary" :items="stats" @select="applyStatFilter" />
      <ResourceCard class="content-pool-card" :class="{ 'is-reviewing': reviewMode }">
        <div class="filter-bar">
          <t-space wrap>
            <label class="wt-filter-field"><span class="wt-filter-field__label">综合搜索</span><t-input v-model="search" clearable placeholder="标题、内容 ID、作者" /></label>
            <label class="wt-filter-field"><span class="wt-filter-field__label">平台</span><t-select v-model="platform" clearable placeholder="全部"><t-option value="douyin" label="抖音" /><t-option value="bilibili" label="B站" /></t-select></label>
            <label class="wt-filter-field"><span class="wt-filter-field__label">来源方式</span><t-select v-model="sourceType" clearable placeholder="全部"><t-option value="link" label="ID/链接发现" /><t-option value="search" label="关键词发现" /><t-option value="author" label="博主发现" /><t-option value="strategy" label="挖掘策略" /></t-select></label>
            <label v-if="!isLibrary" class="wt-filter-field"><span class="wt-filter-field__label">处理状态</span><t-select v-model="status" clearable placeholder="全部"><t-option value="pending" label="待处理" /><t-option value="material_created" label="已转素材" /><t-option value="ignored" label="已忽略" /></t-select></label>
            <div class="wt-filter-actions"><t-button theme="primary" @click="load">查询</t-button><t-button class="wt-secondary-button" variant="outline" @click="reset">重置</t-button><t-button v-if="!isLibrary && selectedRowKeys.length" theme="primary" :loading="batchLoading" @click="batchMaterialize">批量转素材</t-button><t-button v-if="!isLibrary && selectedRowKeys.length" class="wt-secondary-button" variant="outline" :loading="batchLoading" @click="batchSetStatus('ignored', '批量人工忽略')">批量忽略</t-button><t-button v-if="!isLibrary && selectedRowKeys.length" class="wt-secondary-button" variant="outline" :loading="batchLoading" @click="batchSetStatus('pending', '')">批量恢复</t-button></div>
          </t-space>
        </div>
        <div class="table-scroll-wrap">
          <t-table class="wt-resource-table" :data="pagedRows" :columns="columns" row-key="id" hover size="small" :scroll="{ x: '2060px' }" v-model:selected-row-keys="selectedRowKeys" empty="暂无内容">
            <template #title="{ row }">
              <div class="title-cell">
                <button v-if="coverAvailable(row.cover_url)" type="button" class="title-media is-clickable" @click.stop="openImageViewer(row.cover_url)">
                  <img :src="row.cover_url" alt="" loading="lazy" referrerpolicy="no-referrer" @error="markCoverFailed(row.cover_url)" />
                </button>
                <div v-else class="title-media"><span>暂无封面</span></div>
                <div class="title-copy">
                  <a v-if="row.source_url" class="wt-primary-link" :href="row.source_url" target="_blank" rel="noopener noreferrer" :title="row.title || '未命名内容'">{{ truncateTitle(row.title) || '未命名内容' }}</a>
                  <span v-else :title="row.title || '未命名内容'">{{ truncateTitle(row.title) || '未命名内容' }}</span>
                  <small>{{ row.platform_content_id }}</small>
                </div>
              </div>
            </template>
            <template #game="{ row }">{{ gameName(row.game_id) }}</template>
            <template #author_name="{ row }">
              <a v-if="row.author_home_url" class="wt-primary-link" :href="row.author_home_url" target="_blank" rel="noopener noreferrer">{{ authorLabel(row) }}</a>
              <span v-else>{{ authorLabel(row) }}</span>
            </template>
            <template #source_type="{ row }">{{ sourceTypeLabel(row.source_type) }}</template>
            <template #interaction="{ row }">{{ interactionLabel(row) }}</template>
            <template #source="{ row }">
              <div class="source-cell">
                <t-link v-if="row.strategy_id" class="wt-primary-link" theme="primary" @click="router.push(`/crawl-tasks?strategy_id=${row.strategy_id}`)">{{ strategyLabel(row) }}</t-link>
                <small v-if="row.crawl_task_id">{{ taskLabel(row) }}</small>
                <span v-if="!row.strategy_id && !row.crawl_task_id">-</span>
              </div>
            </template>
            <template #updated_at="{ row }">{{ dateLabel(row.updated_at) }}</template>
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
      <t-drawer v-model:visible="detailVisible" class="content-detail-drawer" :header="reviewMode ? '内容审核' : '内容详情'" size="min(72vw, 1200px)" destroy-on-close :footer="reviewMode" @close="exitReviewMode()">
        <div v-if="detail" class="detail-workspace">
          <div v-if="reviewMode" class="review-progress">
            <span>待审核 {{ reviewQueue.length }}</span>
            <strong>当前 {{ reviewIndex + 1 }} / {{ reviewQueue.length }}</strong>
          </div>
          <section class="detail-hero">
            <button v-if="coverAvailable(detail.cover_url)" type="button" class="detail-cover is-clickable" @click.stop="openImageViewer(detail.cover_url)">
              <img :src="detail.cover_url" alt="" referrerpolicy="no-referrer" @error="markCoverFailed(detail.cover_url)" />
            </button>
            <div v-else class="detail-cover"><span>暂无封面</span></div>
            <div class="detail-primary">
              <div class="detail-title-row">
                <h3>
                <a v-if="detail.source_url" class="wt-primary-link" :href="detail.source_url" target="_blank" rel="noopener noreferrer">{{ detail.title || '未命名内容' }}</a>
                <span v-else>{{ detail.title || '未命名内容' }}</span>
              </h3>
                <ResourceStatusBadge :tone="statusTone(detail.status)" :label="statusLabel(detail.status)" />
              </div>
              <div class="detail-meta-grid">
                <div>
                  <span>作者</span>
                  <strong>
                    <a v-if="detail.author_home_url" class="wt-primary-link" :href="detail.author_home_url" target="_blank" rel="noopener noreferrer">{{ authorLabel(detail) }}</a>
                    <span v-else>{{ authorLabel(detail) }}</span>
                  </strong>
                </div>
                <div><span>平台</span><strong>{{ platformLabel(detail.platform) }}</strong></div>
                <div><span>发布时间</span><strong>{{ dateLabel(detail.published_at) }}</strong></div>
                <div><span>发现时间</span><strong>{{ dateLabel(detail.created_at) }}</strong></div>
                <div><span>修改时间</span><strong>{{ dateLabel(detail.updated_at) }}</strong></div>
                <div><span>操作人</span><strong>{{ detail.created_by_name || '-' }}</strong></div>
                <div><span>审核时间</span><strong>{{ dateLabel(detail.audited_at) }}</strong></div>
                <div><span>审核人</span><strong>{{ detail.audited_by_name || '-' }}</strong></div>
              </div>
              <div class="detail-metrics">
                <div><span>点赞</span><strong>{{ countLabel(detail.like_count) }}</strong></div>
                <div><span>收藏</span><strong>{{ countLabel(detail.favorite_count) }}</strong></div>
                <div><span>浏览</span><strong>{{ countLabel(detail.view_count) }}</strong></div>
                <div><span>评论</span><strong>{{ countLabel(detail.comment_count) }}</strong></div>
                <div><span>分享</span><strong>{{ countLabel(detail.share_count) }}</strong></div>
              </div>
              <p v-if="detail.description" class="detail-description">{{ detail.description }}</p>
            </div>
          </section>
          <div class="detail-columns">
            <section class="detail-panel">
              <h4>来源追溯</h4>
              <dl>
                <div><dt>来源方式</dt><dd>{{ sourceTypeLabel(detail.source_type) }}</dd></div>
                <div><dt>来源策略</dt><dd><t-link v-if="detail.strategy_id" class="wt-primary-link" theme="primary" @click="router.push(`/crawl-tasks?strategy_id=${detail.strategy_id}`)">{{ strategyLabel(detail) }}</t-link><span v-else>-</span></dd></div>
                <div><dt>来源任务</dt><dd><t-link v-if="detail.crawl_task_id" class="wt-primary-link" theme="primary" @click="router.push(`/crawl-tasks?task_id=${detail.crawl_task_id}`)">{{ taskLabel(detail) }}</t-link><span v-else>-</span></dd></div>
                <div><dt>平台内容 ID</dt><dd>{{ detail.platform_content_id || '-' }}</dd></div>
                <div><dt>作者 sec_uid</dt><dd>{{ detail.author_sec_uid || '-' }}</dd></div>
                <div><dt>作者 uid</dt><dd>{{ detail.author_uid || '-' }}</dd></div>
              </dl>
            </section>
            <section class="detail-panel">
              <h4>处理结果</h4>
              <dl>
                <div><dt>当前状态</dt><dd><ResourceStatusBadge :tone="statusTone(detail.status)" :label="statusLabel(detail.status)" /></dd></div>
                <div><dt>关联素材</dt><dd><t-link v-if="detail.material_id" class="wt-primary-link" theme="primary" @click="viewMaterial(detail)">查看素材</t-link><span v-else>未转换</span></dd></div>
                <div><dt>审核备注</dt><dd>{{ detail.audit_note || '-' }}</dd></div>
                <div><dt>失败原因</dt><dd :class="{ 'is-danger': detail.failure_reason }">{{ detail.failure_reason || '-' }}</dd></div>
              </dl>
            </section>
          </div>
          <div class="detail-actions">
            <a v-if="detail.source_url" class="detail-primary-link" :href="detail.source_url" target="_blank" rel="noopener noreferrer">打开原作品</a>
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
              <t-button class="wt-secondary-button" variant="outline" :disabled="reviewProcessing" @click="exitReviewMode()">取消审核</t-button>
            </div>
          </div>
        </template>
      </t-drawer>
      <t-dialog v-model:visible="manualVisible" header="发现内容" width="920px" :footer="false">
        <div class="discover-dialog">
          <div class="discover-modes">
            <button type="button" class="discover-mode" :class="{ 'is-active': manualMode === 'id' }" @click="manualMode = 'id'">
              <strong>ID/链接发现</strong><span>通过视频 ID 或链接发现内容</span>
            </button>
            <button type="button" class="discover-mode" :class="{ 'is-active': manualMode === 'keyword' }" @click="manualMode = 'keyword'">
              <strong>关键词发现</strong><span>通过关键词搜索发现内容</span>
            </button>
          </div>

          <div class="discover-section">
            <div class="discover-section__title"><span class="discover-no">①</span>基础信息<small>选择平台和归属游戏，用于内容分类和数据统计</small></div>
            <t-form label-width="80px">
              <t-form-item label="运营团队"><t-select v-model="manualTeamId" :disabled="manualTeamLocked" placeholder="选择内容归属团队"><t-option v-for="team in teams" :key="team.id" :value="team.id" :label="team.name" /></t-select></t-form-item>
              <t-form-item label="所属游戏"><t-select v-model="manualGameId" clearable placeholder="选择游戏分类"><t-option v-for="game in games" :key="game.id" :value="game.id" :label="game.name" /></t-select></t-form-item>
              <t-form-item label="平台"><t-select v-model="manualForm.platform"><t-option value="douyin" label="抖音" /><t-option value="bilibili" label="B站（待接入）" disabled /></t-select></t-form-item>
            </t-form>
          </div>

          <div class="discover-section">
            <div class="discover-section__title"><span class="discover-no">②</span>{{ manualMode === 'id' ? '输入内容' : '关键词配置' }}<small>{{ manualMode === 'id' ? '支持抖音视频 ID、视频链接或分享短链接，按回车生成标签，可批量粘贴' : '添加关键词，支持批量粘贴，按回车自动生成标签' }}</small></div>
            <KeywordTags v-if="manualMode === 'id'" v-model="manualForm.idList" :max="100" placeholder="输入视频 ID 或链接，按回车添加" />
            <KeywordTags v-else v-model="manualForm.keywords" :max="100" placeholder="输入关键词，按回车添加" />
          </div>

          <t-alert v-if="manualSearched" theme="info" :message="`搜索完成，发现 ${manualResults.length} 条，已选 ${selectedResultKeys.length} 条，可加入内容池`" />
          <div v-if="manualResults.length" class="discover-results">
            <div class="discover-section__title">结果预览<small>仅所选内容会加入内容池</small></div>
            <t-table v-model:selected-row-keys="selectedResultKeys" :data="manualResults" :columns="resultColumns" row-key="platform_content_id" hover size="small" :scroll="{ y: '300px' }" empty="暂无结果">
              <template #title="{ row }">
                <div class="title-cell">
                  <button v-if="coverAvailable(row.cover_url)" type="button" class="title-media is-clickable" @click.stop="openImageViewer(row.cover_url)"><img :src="row.cover_url" alt="" loading="lazy" referrerpolicy="no-referrer" @error="markCoverFailed(row.cover_url)" /></button>
                  <div v-else class="title-media"><span>暂无封面</span></div>
                  <div class="title-copy">
                    <a v-if="row.source_url" class="wt-primary-link" :href="row.source_url" target="_blank" rel="noopener noreferrer" :title="row.title || '未命名内容'">{{ truncateTitle(row.title) || '未命名内容' }}</a>
                    <span v-else :title="row.title || '未命名内容'">{{ truncateTitle(row.title) || '未命名内容' }}</span>
                    <small>{{ row.platform_content_id }}</small>
                  </div>
                </div>
              </template>
              <template #author_name="{ row }">
                <a v-if="row.author_home_url" class="wt-primary-link" :href="row.author_home_url" target="_blank" rel="noopener noreferrer">{{ authorLabel(row) }}</a>
                <span v-else>{{ authorLabel(row) }}</span>
              </template>
              <template #like_count="{ row }">{{ countLabel(row.like_count) }}</template>
              <template #favorite_count="{ row }">{{ countLabel(row.favorite_count) }}</template>
              <template #view_count="{ row }">{{ countLabel(row.view_count) }}</template>
              <template #comment_count="{ row }">{{ countLabel(row.comment_count) }}</template>
              <template #share_count="{ row }">{{ countLabel(row.share_count) }}</template>
              <template #published_at="{ row }">{{ dateLabel(row.published_at) }}</template>
            </t-table>
            <div v-if="manualMode === 'keyword' && manualSearched" class="manual-search-actions">
              <t-button variant="outline" :loading="manualLoading" @click="searchManual(true)">重新发现</t-button>
              <t-button variant="outline" :loading="manualLoading" :disabled="!manualPagination.hasMore" @click="searchManual()">下一页</t-button>
            </div>
          </div>

          <t-alert theme="info" message="小提示：发现的内容将在结果页中进行预览和筛选，确认后可一键加入内容池。" />

          <div class="discover-footer">
            <t-button variant="outline" @click="manualVisible = false">取消</t-button>
            <t-button theme="primary" :loading="manualLoading" @click="confirmManual">{{ manualResults.length ? '加入内容池' : '开始发现' }}</t-button>
          </div>
        </div>
      </t-dialog>
      <t-image-viewer
        v-model:visible="imageViewerVisible"
        :images="imageViewerImages"
        :index="imageViewerIndex"
        image-referrerpolicy="no-referrer"
      />
    </div>
  </t-loading>
</template>

<style scoped>
.content-pool-card { padding: 18px 20px; }
.title-cell { display: flex; align-items: center; gap: 12px; min-height: 64px; }
.title-media { display: flex; align-items: center; justify-content: center; overflow: hidden; width: 88px; height: 56px; border: 1px solid var(--wt-border); border-radius: 8px; background: var(--wt-bg-page); color: var(--wt-text-tertiary); font-size: 12px; flex-shrink: 0; }
.title-media { padding: 0; cursor: default; }
.title-media img { width: 100%; height: 100%; object-fit: cover; }
.title-media.is-clickable { cursor: zoom-in; }
.title-copy { display: flex; min-width: 0; flex-direction: column; gap: 4px; }
.wt-primary-link { color: var(--wt-primary); font-weight: 600; text-decoration: none; }
.wt-primary-link:hover, .wt-primary-link:focus-visible { text-decoration: underline; }
.review-mode-button.is-active { box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.16); }
.content-pool-card.is-reviewing { border-color: rgba(37, 99, 235, 0.36); box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.10); }
/* 标题有两种渲染分支：有 source_url 时是 <a class="wt-primary-link">，否则是 <span>。
   省略号必须同时覆盖两者——只写 span 时，有外链的行会整段自由折行，
   把行高从声明的 56px 撑到 150px 以上。颜色分开写，避免覆盖链接自身的主题色。 */
.title-copy > a,
.title-copy > span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.title-copy > span { color: var(--wt-text-primary); font-weight: 600; }
.title-copy small { overflow: hidden; color: var(--wt-text-tertiary); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.content-pool-page :deep(.wt-resource-actions) { max-width: 360px; }
.content-detail-drawer :deep(.t-drawer) { max-width: 1200px; }
.detail-workspace { display: flex; flex-direction: column; gap: 18px; }
.review-progress { display: flex; align-items: center; justify-content: space-between; padding: 12px 14px; border: 1px solid rgba(37, 99, 235, 0.18); border-radius: 10px; background: var(--wt-info-bg); }
.review-progress span { color: var(--wt-text-tertiary); font-size: 13px; }
.review-progress strong { color: var(--wt-primary); font-size: 14px; }
.detail-hero { display: grid; grid-template-columns: minmax(280px, 38%) 1fr; gap: 20px; }
.detail-cover { display: flex; align-items: center; justify-content: center; overflow: hidden; width: 100%; min-height: 260px; border: 1px solid var(--wt-border); border-radius: 12px; background: var(--wt-bg-page); color: var(--wt-text-tertiary); }
.detail-cover img { width: 100%; height: 100%; object-fit: cover; }
.detail-cover.is-clickable { padding: 0; cursor: zoom-in; }
.detail-primary { min-width: 0; }
.detail-title-row { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.detail-title-row h3 { margin: 0; color: var(--wt-text-primary); font-size: 21px; font-weight: 650; line-height: 1.35; }
.source-cell { display: flex; flex-direction: column; gap: 2px; }
.source-cell small { color: var(--wt-text-tertiary); font-size: 12px; }
.detail-meta-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; margin-top: 18px; }
.detail-meta-grid div, .detail-metrics div { padding: 10px 12px; border: 1px solid var(--wt-border); border-radius: 10px; background: var(--wt-bg-card); }
.detail-meta-grid span, .detail-metrics span { display: block; color: var(--wt-text-tertiary); font-size: 12px; }
.detail-meta-grid strong, .detail-metrics strong { display: block; margin-top: 5px; color: var(--wt-text-primary); font-size: 14px; font-weight: 600; }
.detail-metrics { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; margin-top: 12px; }
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
.discover-dialog { display: flex; flex-direction: column; gap: 14px; }
.discover-modes { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
.discover-mode { display: flex; flex-direction: column; gap: 3px; padding: 12px 14px; border: 1px solid var(--wt-border); border-radius: 10px; background: var(--wt-bg-page); text-align: left; cursor: pointer; }
.discover-mode strong { color: var(--wt-text-primary); font-size: 14px; }
.discover-mode span { color: var(--wt-text-tertiary); font-size: 12px; }
.discover-mode.is-active { border-color: var(--wt-primary); background: var(--wt-info-bg); }
.discover-mode.is-active strong { color: var(--wt-primary); }
.discover-section { border: 1px solid var(--wt-border); border-radius: 10px; padding: 12px 14px; }
.discover-section__title { display: flex; align-items: baseline; gap: 8px; margin-bottom: 10px; color: var(--wt-text-primary); font-size: 14px; font-weight: 600; }
.discover-section__title small { color: var(--wt-text-tertiary); font-size: 12px; font-weight: 400; }
.discover-no { color: var(--wt-primary); font-weight: 700; }
.discover-counter { margin-top: 4px; text-align: right; color: var(--wt-text-tertiary); font-size: 12px; }
.discover-collapse { width: 100%; border: none; background: transparent; cursor: pointer; }
.discover-toggle { margin-left: auto; color: var(--wt-primary); font-size: 12px; font-weight: 400; }
.discover-advanced { margin-top: 8px; }
.discover-results { display: flex; flex-direction: column; gap: 10px; }
.discover-footer { display: flex; justify-content: flex-end; gap: 8px; padding-top: 4px; }
</style>
