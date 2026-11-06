<script setup>
import '../mining.css'
import { DialogPlugin } from 'tdesign-vue-next'
import ContentCoverPlaceholder from '../components/ContentCoverPlaceholder.vue'
import { opColumnWidth } from '../../../shared/ui/resource/opColumnWidth.js'
import ResourceFilterBar from '../../../shared/ui/resource/ResourceFilterBar.vue'
import ResourceDateRange from '../../../shared/ui/resource/ResourceDateRange.vue'
import ResourceSelectionToolbar from '../../../shared/ui/resource/ResourceSelectionToolbar.vue'
import ResourceRowActions from '../../../shared/ui/resource/ResourceRowActions.vue'
import ResourceOverlay from '../../../shared/ui/resource/ResourceOverlay.vue'
import ResourceForm from '../../../shared/ui/resource/ResourceForm.vue'
import ResourceSection from '../../../shared/ui/resource/ResourceSection.vue'
import ResourceDescriptions from '../../../shared/ui/resource/ResourceDescriptions.vue'
import ResourceChoiceCards from '../../../shared/ui/resource/ResourceChoiceCards.vue'
import ResourceFieldHelp from '../../../shared/ui/resource/ResourceFieldHelp.vue'
import ResourceNotice from '../../../shared/ui/resource/ResourceNotice.vue'
import ResourceEmptyState from '../../../shared/ui/resource/ResourceEmptyState.vue'
import BusinessSelect from "../../../shared/ui/fields/BusinessSelect.vue"
import { feedback } from '../../../shared/ui/feedback/index.js'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { createContentPoolClient } from '../../../shared/api/contentPool.js'
import { createDiscoveryClient } from '../../../shared/api/discovery.js'
import { createSessionClient } from '../../../shared/api/session.js'
import { createUsersClient } from '../../../apps/cloud/pages/users/usersApi.js'
import ResourceCard from '../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../shared/ui/resource/ResourcePageHeader.vue'
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'
import MetricList from '../../../shared/ui/resource/MetricList.vue'
import KeywordTags from '../../../shared/ui/resource/KeywordTags.vue'
import ContentPoolSourceDrawer from '../components/ContentPoolSourceDrawer.vue'
import { formatDateTime } from '../../../shared/utils/datetime.js'
import { enabledCategories } from '../../../shared/utils/categories.js'
import { MINING_PLATFORM_OPTIONS, platformLabel } from '../../../shared/utils/platforms.js'
import { useAutoRefresh } from '../../../shared/utils/useAutoRefresh.js'

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
const contentID = ref(route.query.content_id ? String(route.query.content_id) : '')
const platform = ref('')
const sourceType = ref('')
const status = ref('')
const publishedRange = ref([])
const sortBy = ref('published_at')
const sortOrder = ref('desc')
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
const sourceVisible = ref(false)
const sourceRow = ref(null)
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
const manualCategoryId = ref('')
const teams = ref([])
const categories = ref([])
const currentUser = ref(null)
const teamFilter = ref('')
const categoryFilter = ref([])
const isAdmin = computed(() => currentUser.value?.role === 'admin')
const imageViewerVisible = ref(false)
const imageViewerImages = ref([])
const imageViewerIndex = ref(0)

const resultColumns = [
  { colKey: 'row-select', type: 'multiple', width: 48 },
  { colKey: 'title', title: '标题', width: 280 },
  { colKey: 'author_name', title: '作者', width: 140 },
  { colKey: 'like_count', title: '点赞', width: 90 },
  { colKey: 'favorite_count', title: '收藏', width: 90 },
  { colKey: 'view_count', title: '浏览', width: 90 },
  { colKey: 'comment_count', title: '评论', width: 90 },
  { colKey: 'share_count', title: '分享', width: 90 },
  { colKey: 'published_at', title: '发布时间', width: 150 },
]

const pendingCount = computed(() => rows.value.filter((row) => row.status === 'pending').length)

const pagedRows = computed(() => {
  const start = (pagination.value.current - 1) * pagination.value.pageSize
  return rows.value.slice(start, start + pagination.value.pageSize)
})

// TDesign 的 fixed 表格布局在 WebKit 中需要明确 width，minWidth 不能保证内容列宽度。
const columns = [
  { colKey: 'row-select', type: 'multiple', width: 48, fixed: 'left' },
  { colKey: 'id', title: 'ID', width: 70, sorter: true },
  { colKey: 'title', title: '内容 / 作者', width: 280 },
  { colKey: 'platform', title: '平台', width: 80 },
  { colKey: 'category', title: '类目', width: 100 },
  { colKey: 'interaction', title: '互动', width: 130 },
  { colKey: 'source', title: '来源', width: 160 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'published_at', title: '发布时间', width: 150, sorter: true },
  { colKey: 'created_at', title: '入库时间', width: 150, sorter: true },
  { colKey: 'op', title: '操作', width: opColumnWidth(['详情', '转素材', '忽略'], { variant: 'text', primary: true }), fixed: 'right' },
]

// 横向滚动宽度与列宽共用一份定义，避免单独维护总宽度。
const tableScroll = computed(() => ({
  x: `${columns.reduce((sum, col) => sum + (col.width ?? col.minWidth ?? 0), 0)}px`,
}))

onMounted(async () => {
  // 先拿到当前用户的负责类目，首次列表才能按默认类目筛选加载。
  await loadManualContext()
  await load()
})

// quiet 是定时刷新用的：不翻页码、不闪 loading、失败保留上一次读数。用户自己改筛选
// 或翻页仍走非 quiet，读不到就显式报错。
async function load({ quiet = false } = {}) {
  if (!quiet) loading.value = true
  if (!quiet) error.value = ''
  try {
    const result = await client.list({ search: search.value.trim(), content_id: contentID.value.trim() || undefined, platform: platform.value, source_type: sourceType.value, status: isLibrary.value ? 'material_created' : status.value, team_id: teamFilter.value || undefined, category_ids: categoryFilter.value.length ? categoryFilter.value : undefined, strategy_id: strategyFilter.value || undefined, crawl_task_id: taskFilter.value || undefined, material_id: materialFilter.value || undefined, published_from: publishedRange.value?.[0] || undefined, published_to: publishedRange.value?.[1] || undefined, sort_by: sortBy.value, sort_order: sortOrder.value })
    rows.value = Array.isArray(result) ? result : []
    if (!quiet) pagination.value.current = 1
  } catch (e) {
    if (!quiet) error.value = e.message || '内容池加载失败'
  } finally {
    if (!quiet) loading.value = false
  }
}

// 定时刷新让开用户正在做的事：详情/来源/人工导入打开、审核模式、已选行或批量操作中。
useAutoRefresh(() => load({ quiet: true }), {
  shouldSkip: () => detailVisible.value || sourceVisible.value || manualVisible.value || imageViewerVisible.value || reviewMode.value || selectedRowKeys.value.length > 0 || batchLoading.value || manualLoading.value || reviewProcessing.value,
})

function reset() {
  search.value = ''; contentID.value = ''; platform.value = ''; sourceType.value = ''; status.value = ''; strategyFilter.value = ''; taskFilter.value = ''; materialFilter.value = ''; publishedRange.value = []; teamFilter.value = ''; categoryFilter.value = defaultCategoryIDs()
  load()
}

// 选择与筛选用启用类目；名称解析仍读全量目录，停用类目的历史行不会显示成裸 ID。
const enabledCategoryOptions = computed(() => enabledCategories(categories.value))

// 类目范围是“负责类目”，不是可见边界：默认只勾本人负责的类目，清空即看全团队。
// 与启用集合取交集：停用类目已不在筛选项里，勾上等于挂一个删不掉的条件。
function defaultCategoryIDs() {
  const ids = currentUser.value?.role === 'admin' ? [] : [...(currentUser.value?.category_ids || [])]
  if (!categories.value.length) return ids
  const enabledIds = new Set(enabledCategoryOptions.value.map((category) => category.id))
  return ids.filter((id) => enabledIds.has(id))
}

function changeTeamFilter() {
  // 类目目录是全局的，切团队后保留上一团队的勾选没有意义。
  categoryFilter.value = []
  load()
}

function changeSort(sort) {
  sortBy.value = sort?.sortBy || 'published_at'
  sortOrder.value = sort?.descending === false ? 'asc' : 'desc'
  load()
}

function openManual(mode) {
  manualMode.value = mode
  manualSearched.value = false
  manualResults.value = []
  selectedResultKeys.value = []
  resetManualPagination()
  manualForm.value = { platform: 'douyin', idList: [], keyword: '', keywords: [] }
  manualBaseline = manualDraft()
  manualVisible.value = true
}

const discoverModes = [
  { value: 'id', label: 'ID/链接发现', description: '输入抖音作品 ID 或链接' },
  { value: 'keyword', label: '关键词发现', description: '按关键词搜索作品' },
]
let manualBaseline = ''
function manualDraft() { return JSON.stringify([manualMode.value, manualTeamId.value, manualCategoryId.value, manualForm.value]) }
function requestCloseManual() {
  if (manualLoading.value) return
  if (manualDraft() === manualBaseline && !manualResults.value.length) { manualVisible.value = false; return }
  const dialog = DialogPlugin.confirm({ header: '放弃本次发现？', body: '输入和已选结果不会保留。', confirmBtn: { content: '放弃', theme: 'danger' }, onConfirm: () => { dialog.hide(); manualVisible.value = false } })
}

function manualTitle() {
  return ({ id: 'ID/链接发现', keyword: '关键词发现' })[manualMode.value]
}

async function loadManualContext() {
  try {
    const user = await session.me()
    currentUser.value = user
    manualTeamLocked.value = user?.role !== 'admin'
    manualTeamId.value = user?.role === 'admin' ? '' : (user?.team_id || '')
    const data = await users.listTeams()
    teams.value = Array.isArray(data) ? data : []
    // 类目名称映射与人工导入需要完整目录，非管理员也要看到本团队其他类目。
    const categoryData = await users.listCategories({ scope: 'all' })
    categories.value = Array.isArray(categoryData) ? categoryData : []
    // 默认负责类目要在目录到位后再取，否则与启用集合的交集算不出来。
    if (!categoryFilter.value.length) categoryFilter.value = defaultCategoryIDs()
    const enabled = enabledCategoryOptions.value
    if (!manualCategoryId.value && enabled.length) manualCategoryId.value = (enabled.find((g) => g.id === 'other') || enabled[0]).id
  } catch {
    teams.value = []
    categories.value = []
  }
}

function teamName(id) {
  const team = teams.value.find((item) => item.id === id)
  return team?.name || '未知团队'
}

function categoryName(id) {
  const category = categories.value.find((item) => item.id === id)
  return category?.name || '-'
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
      feedback.toast.error('请至少添加一个关键词')
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
    feedback.toast.error(e.message || '搜索失败')
  } finally {
    manualLoading.value = false
  }
}

function parseSearchTargets() {
  const targets = [...new Set((manualForm.value.idList || []).map((value) => String(value).trim()).filter(Boolean))]
  if (!targets.length) {
    feedback.toast.error('请输入至少一个视频 ID 或链接')
    return []
  }
  const valid = targets.every((target) => (
    /^\d+$/.test(target)
    || /^https?:\/\/(www\.)?douyin\.com\/video\/\d+(?:\/.*)?$/.test(target)
    || /^https?:\/\/v\.douyin\.com\/[^\s]+$/.test(target)
  ))
  if (!valid) {
    feedback.toast.error('仅支持抖音视频 ID、视频链接或分享短链接')
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
    feedback.toast.error('请选择运营团队')
    return
  }
  await confirmSelected()
}

async function confirmSelected() {
  if (!selectedResultKeys.value.length) {
    feedback.toast.error('请至少选择一条内容')
    return
  }
  manualLoading.value = true
  try {
    const selected = manualResults.value.filter((item) => selectedResultKeys.value.includes(item.platform_content_id))
    const result = await discovery.importResults({ platform: manualForm.value.platform, team_id: selectedTeamID(), category_id: manualCategoryId.value || null, source_type: 'search', items: selected })
    manualVisible.value = false
    await load()
    feedback.toast.success(`成功导入 ${result.imported || 0} 条，重复 ${result.duplicate || 0} 条`)
  } catch (e) { feedback.toast.error(e.message || '内容入池失败') } finally { manualLoading.value = false }
}

async function openDetail(row) {
  try {
    detail.value = await client.get(row.id)
    detailVisible.value = true
    return true
  } catch (e) {
    feedback.toast.error(e.message || '内容详情加载失败')
    return false
  }
}

// 来源弹窗只对策略来源开：人工来源（含只有任务 id 的历史行）没有可追溯的实体。
function openSource(row) {
  if (!row?.strategy_id) return
  sourceRow.value = row
  sourceVisible.value = true
}

// 用户 2026-10-27：管理员转素材后必须知道素材进了哪个团队（目标团队恒为来源内容的
// 团队，管理员不选团队）。单条用单按钮告知把去向说清楚；运营 / 高级运营的素材恒属于
// 本团队，保留原来的 toast。审核模式是连续操作，用非阻塞 toast 带出去向，不打断
// 「转素材并下一条」。
function announceMaterialized(material, { modal = true } = {}) {
  if (!isAdmin.value) {
    feedback.toast.success('已转为素材，来源关系已保留')
    return
  }
  const message = `素材 ${material?.id ?? '-'}「${material?.title || '未命名素材'}」已转入团队「${teamName(material?.team_id)}」的素材库`
  if (modal) feedback.notice({ title: '已转入素材库', message })
  else feedback.toast.success(message)
}

async function materialize(row, reload = true) {
  try {
    const material = await client.materialize(row.id)
    if (reload) await load()
    announceMaterialized(material)
    return true
  } catch (e) {
    feedback.toast.error(e.message || '转素材失败')
    return false
  }
}

async function updateStatus(row, nextStatus, auditNote = '', reload = true) {
  try {
    await client.setStatus(row.id, nextStatus, nextStatus === 'ignored' ? '人工忽略' : '', auditNote)
    if (reload) await load()
    feedback.toast.success(nextStatus === 'ignored' ? '内容已忽略' : '内容已恢复待处理')
    return true
  } catch (e) {
    feedback.toast.error(e.message || '状态更新失败')
    return false
  }
}

function viewMaterial(row) {
  if (!row.material_id) return
  router.push({ path: '/material-library', query: { material_id: String(row.material_id) } })
}

async function batchMaterialize() {
  if (!selectedRowKeys.value.length) return
  batchLoading.value = true
  try {
    const result = await client.batchMaterialize(selectedRowKeys.value.map(Number))
    selectedRowKeys.value = []
    await load()
    const succeeded = (result.items || []).filter((item) => item.success && item.material)
    if (isAdmin.value && succeeded.length) {
      // 管理员跨团队批量转换：按目标团队聚合，逐条列出素材进了哪个团队。
      const counts = new Map()
      for (const item of succeeded) {
        const teamID = item.material.team_id
        counts.set(teamID, (counts.get(teamID) || 0) + 1)
      }
      const lines = [...counts.entries()].map(([teamID, count]) => `团队「${teamName(teamID)}」：${count} 条`)
      feedback.notice({
        title: '已转入素材库',
        message: `转素材成功 ${result.succeeded || 0} 条，失败 ${result.failed || 0} 条。${lines.join('；')}`,
      })
    } else {
      feedback.toast.success(`转素材成功 ${result.succeeded || 0} 条，失败 ${result.failed || 0} 条`)
    }
  } catch (e) {
    feedback.toast.error(e.message || '批量转素材失败')
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
    feedback.toast.success(`处理成功 ${result.succeeded || 0} 条，失败 ${result.failed || 0} 条`)
  } catch (e) {
    feedback.toast.error(e.message || '批量处理失败')
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
    feedback.toast.info('当前没有待处理内容')
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
      // 审核模式是连续操作：管理员用 toast 带出目标团队，不弹窗打断下一条。
      if (isAdmin.value) announceMaterialized(material, { modal: false })
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
    feedback.toast.success('待处理内容已审核完成')
    return
  }
  reviewIndex.value += 1
  const next = reviewQueue.value[reviewIndex.value]
  currentReviewID.value = next.id
  reviewNote.value = ''
  reviewError.value = ''
  await openDetail(next)
}

function strategyLabel(row) { return row.strategy_name || (row.strategy_id ? `策略 ${row.strategy_id}` : '-') }
function taskLabel(row) { return row.crawl_task_name || (row.crawl_task_id ? `任务 ${row.crawl_task_id}` : '-') }
function statusLabel(value) { return ({ pending: '待处理', material_created: '已转素材', ignored: '已忽略' })[value] || value || '未知' }
function statusTone(value) { return ({ pending: 'warning', material_created: 'success', ignored: 'neutral' })[value] || 'info' }
function sourceTypeLabel(value) { return ({ link: 'ID/链接发现', search: '关键词发现', author: '博主发现', strategy: '挖掘策略' })[value] || value || '-' }
function countLabel(value) { return Number(value || 0).toLocaleString() }
function authorLabel(row) { return row.author_name || row.author_uid || row.author_sec_uid || '-' }
// 竖排一行一项，返回数组而不是拼好的字符串（渲染交给 MetricList）。
// 拼成「浏览 N · 赞 N · 藏 N」时，宽度不够会在任意位置折断，
// 数字和标签被拆散，读起来是一团。
function interactionLabel(row) {
  return [
    { label: '浏览', value: countLabel(row.view_count) },
    { label: '赞', value: countLabel(row.like_count) },
    { label: '藏', value: countLabel(row.favorite_count) },
  ]
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
    <div class="wt-resource-page resource-visual-approved content-pool-page">
      <FeedbackAlert v-if="error" theme="error" :message="error" closable style="margin-bottom:16px" @close="error=''" />
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
        </template>
      </ResourcePageHeader>
      <ResourceCard class="content-pool-card" :class="{ 'is-reviewing': reviewMode }">
        <ResourceFilterBar>
            <label class="wt-filter-field wt-filter-field--wide"><span class="wt-filter-field__label">综合搜索</span><t-input v-model="search" clearable placeholder="标题、内容 ID、作者" /></label>
            <label class="wt-filter-field"><span class="wt-filter-field__label">类目</span><BusinessSelect kind="category" filter v-model="categoryFilter" multiple clearable><t-option v-for="category in enabledCategoryOptions" :key="category.id" :value="category.id" :label="category.name" /></BusinessSelect></label>
            <label class="wt-filter-field"><span class="wt-filter-field__label">平台</span><t-select v-model="platform" clearable placeholder="全部"><t-option v-for="option in MINING_PLATFORM_OPTIONS" :key="option.value" :value="option.value" :label="option.label" /></t-select></label>
            <label v-if="!isLibrary" class="wt-filter-field"><span class="wt-filter-field__label">处理状态</span><t-select v-model="status" clearable placeholder="全部"><t-option value="pending" label="待处理" /><t-option value="material_created" label="已转素材" /><t-option value="ignored" label="已忽略" /></t-select></label>
            <label class="wt-filter-field"><span class="wt-filter-field__label">发布时间</span><ResourceDateRange v-model="publishedRange" /></label>
            <label class="wt-filter-field"><span class="wt-filter-field__label">来源方式</span><t-select v-model="sourceType" clearable placeholder="全部"><t-option value="link" label="ID/链接发现" /><t-option value="search" label="关键词发现" /><t-option value="author" label="博主发现" /><t-option value="strategy" label="挖掘策略" /></t-select></label>
            <label v-if="isAdmin" class="wt-filter-field"><span class="wt-filter-field__label">运营团队</span><BusinessSelect kind="team" filter v-model="teamFilter" clearable @change="changeTeamFilter"><t-option v-for="team in teams" :key="team.id" :value="team.id" :label="team.name" /></BusinessSelect></label>
            <label class="wt-filter-field"><span class="wt-filter-field__label">内容 ID</span><t-input v-model="contentID" clearable placeholder="精确匹配" /></label>
            <div class="wt-filter-actions"><t-button theme="primary" @click="load">查询</t-button><t-button class="wt-secondary-button" variant="outline" @click="reset">重置</t-button></div>
        </ResourceFilterBar>
        <ResourceSelectionToolbar v-if="!isLibrary" :count="selectedRowKeys.length" unit="条内容" @clear="selectedRowKeys = []">
          <t-button variant="outline" :loading="batchLoading" @click="batchMaterialize">批量转素材</t-button>
          <t-button variant="outline" :loading="batchLoading" @click="batchSetStatus('ignored', '批量人工忽略')">批量忽略</t-button>
          <t-button variant="outline" :loading="batchLoading" @click="batchSetStatus('pending', '')">批量恢复</t-button>
        </ResourceSelectionToolbar>
        <div class="table-scroll-wrap">
          <t-table class="wt-resource-table" :data="pagedRows" :columns="columns" row-key="id" hover size="small" :scroll="tableScroll" :sort="{ sortBy, descending: sortOrder === 'desc' }" @sort-change="changeSort" v-model:selected-row-keys="selectedRowKeys" empty="暂无内容">
            <template #title="{ row }">
              <div class="title-cell">
                <button v-if="coverAvailable(row.cover_url)" type="button" class="title-media is-clickable" @click.stop="openImageViewer(row.cover_url)">
                  <img :src="row.cover_url" alt="" loading="lazy" referrerpolicy="no-referrer" @error="markCoverFailed(row.cover_url)" />
                </button>
                <ContentCoverPlaceholder v-else />
                <div class="title-copy">
                  <a v-if="row.source_url" class="wt-primary-link" :href="row.source_url" target="_blank" rel="noopener noreferrer" :title="row.title || '未命名内容'">{{ truncateTitle(row.title) || '未命名内容' }}</a>
                  <span v-else :title="row.title || '未命名内容'">{{ truncateTitle(row.title) || '未命名内容' }}</span>
                  <small><a v-if="row.author_home_url" class="wt-primary-link" :href="row.author_home_url" target="_blank" rel="noopener noreferrer">{{ authorLabel(row) }}</a><span v-else>{{ authorLabel(row) }}</span></small>
                </div>
              </div>
            </template>
            <template #platform="{ row }">{{ platformLabel(row.platform) }}</template>
            <template #category="{ row }">{{ categoryName(row.category_id) }}</template>
            <template #interaction="{ row }"><MetricList :items="interactionLabel(row)" /></template>
            <template #source="{ row }">
              <t-link v-if="row.strategy_id" class="wt-primary-link" theme="primary" @click="openSource(row)">{{ strategyLabel(row) }}</t-link>
              <span v-else>人工</span>
              <small class="mining-secondary">{{ sourceTypeLabel(row.source_type) }}</small>
            </template>
            <template #published_at="{ row }">{{ formatDateTime(row.published_at) }}</template>
            <template #created_at="{ row }">{{ formatDateTime(row.created_at) }}</template>
            <template #status="{ row }"><ResourceStatusBadge :tone="statusTone(row.status)" :label="statusLabel(row.status)" /></template>
            <template #op="{ row }">
              <ResourceRowActions>
                <t-button size="small" variant="text" @click="openDetail(row)">详情</t-button>
                <t-button v-if="!isLibrary && row.status === 'pending'" size="small" variant="text" class="is-primary" @click="materialize(row)">转素材</t-button>
                <t-button v-if="!isLibrary && row.status === 'pending'" size="small" variant="text" @click="updateStatus(row, 'ignored')">忽略</t-button>
                <t-button v-if="!isLibrary && row.status === 'ignored'" size="small" variant="text" @click="updateStatus(row, 'pending')">恢复</t-button>
              </ResourceRowActions>
            </template>
          </t-table>
        </div>
        <div class="pagination-bar"><t-pagination v-model:current="pagination.current" v-model:pageSize="pagination.pageSize" :total="rows.length" :page-size-options="[10, 20, 50]" /></div>
      </ResourceCard>
      <ResourceOverlay :visible="detailVisible" @update:visible="value => { if (!value) exitReviewMode() }" :title="reviewMode ? '内容审核' : '内容详情'" kind="drawer" :width="960" :busy="reviewProcessing" :footer="reviewMode">
        <div v-if="detail" class="mining-detail">
          <ResourceNotice v-if="reviewMode">当前 {{ reviewIndex + 1 }} / {{ reviewQueue.length }} · 按顺序审核待处理内容</ResourceNotice>
          <ResourceSection title="基本信息">
            <ResourceDescriptions>
              <t-descriptions-item label="内容标题" full><div class="mining-content-heading"><button v-if="coverAvailable(detail.cover_url)" type="button" class="mining-cover-button" aria-label="查看封面" @click.stop="openImageViewer(detail.cover_url)"><img :src="detail.cover_url" alt="" referrerpolicy="no-referrer" @error="markCoverFailed(detail.cover_url)" /></button><span>{{ detail.title || '未命名内容' }}</span></div></t-descriptions-item>
              <t-descriptions-item label="内容 ID">{{ detail.id }}</t-descriptions-item>
              <t-descriptions-item label="状态"><ResourceStatusBadge :tone="statusTone(detail.status)" :label="statusLabel(detail.status)" /></t-descriptions-item>
              <t-descriptions-item label="类目">{{ categoryName(detail.category_id) }}</t-descriptions-item>
              <t-descriptions-item label="平台">{{ platformLabel(detail.platform) }}</t-descriptions-item>
              <t-descriptions-item label="作者"><a v-if="detail.author_home_url" class="wt-primary-link" :href="detail.author_home_url" target="_blank" rel="noopener noreferrer">{{ authorLabel(detail) }}</a><span v-else>{{ authorLabel(detail) }}</span></t-descriptions-item>
              <t-descriptions-item label="发布时间">{{ formatDateTime(detail.published_at) }}</t-descriptions-item>
              <t-descriptions-item v-if="detail.description && detail.description !== detail.title" label="内容描述" full>{{ detail.description }}</t-descriptions-item>
            </ResourceDescriptions>
          </ResourceSection>
          <ResourceSection title="内容表现">
            <div class="mining-metrics"><div v-for="metric in [{ label: '点赞', value: detail.like_count }, { label: '收藏', value: detail.favorite_count }, { label: '浏览', value: detail.view_count }, { label: '评论', value: detail.comment_count }, { label: '分享', value: detail.share_count }]" :key="metric.label"><span>{{ metric.label }}</span><strong>{{ countLabel(metric.value) }}</strong></div></div>
          </ResourceSection>
          <ResourceSection title="来源追溯">
            <ResourceDescriptions>
              <t-descriptions-item label="来源方式">{{ sourceTypeLabel(detail.source_type) }}</t-descriptions-item>
              <t-descriptions-item label="来源策略"><t-link v-if="detail.strategy_id" theme="primary" @click="openSource(detail)">{{ strategyLabel(detail) }}</t-link><span v-else>—</span></t-descriptions-item>
              <t-descriptions-item label="来源任务"><t-link v-if="detail.crawl_task_id" theme="primary" @click="router.push(`/crawl-tasks?task_id=${detail.crawl_task_id}`)">{{ taskLabel(detail) }}</t-link><span v-else>—</span></t-descriptions-item>
              <t-descriptions-item label="平台作品 ID">{{ detail.platform_content_id || '—' }}</t-descriptions-item>
              <t-descriptions-item label="平台作品链接" full><a v-if="detail.source_url" class="wt-primary-link" :href="detail.source_url" target="_blank" rel="noopener noreferrer">{{ detail.source_url }}</a><span v-else>—</span></t-descriptions-item>
              <t-descriptions-item label="作者 sec_uid" full>{{ detail.author_sec_uid || '—' }}</t-descriptions-item>
              <t-descriptions-item label="作者 uid">{{ detail.author_uid || '—' }}</t-descriptions-item>
            </ResourceDescriptions>
          </ResourceSection>
          <ResourceSection title="处理结果">
            <ResourceDescriptions>
              <t-descriptions-item label="关联素材"><t-link v-if="detail.material_id" theme="primary" @click="viewMaterial(detail)">素材详情</t-link><span v-else>未转换</span></t-descriptions-item>
              <t-descriptions-item label="审核备注">{{ detail.audit_note || '—' }}</t-descriptions-item>
              <t-descriptions-item v-if="detail.failure_reason" label="失败原因" full>{{ detail.failure_reason }}</t-descriptions-item>
            </ResourceDescriptions>
          </ResourceSection>
          <ResourceSection title="操作记录">
            <ResourceDescriptions>
              <t-descriptions-item label="入库时间">{{ formatDateTime(detail.created_at) }}</t-descriptions-item>
              <t-descriptions-item label="操作人">{{ detail.created_by_name || '—' }}</t-descriptions-item>
              <t-descriptions-item label="修改时间">{{ formatDateTime(detail.updated_at) }}</t-descriptions-item>
              <t-descriptions-item label="修改人">{{ detail.updated_by_name || '—' }}</t-descriptions-item>
              <t-descriptions-item label="审核时间">{{ formatDateTime(detail.audited_at) }}</t-descriptions-item>
              <t-descriptions-item label="审核人">{{ detail.audited_by_name || '—' }}</t-descriptions-item>
            </ResourceDescriptions>
          </ResourceSection>
          <ResourceSection v-if="reviewMode" title="本次审核">
            <FeedbackAlert v-if="reviewError" theme="error" :message="reviewError" />
            <ResourceForm><t-form-item label="审核备注"><t-textarea v-model="reviewNote" :rows="3" placeholder="填写审核备注（选填）" /></t-form-item></ResourceForm>
          </ResourceSection>
        </div>
        <template v-if="reviewMode" #footer>
          <t-button variant="outline" :disabled="reviewProcessing" @click="exitReviewMode()">取消审核</t-button>
          <t-button variant="outline" :disabled="reviewProcessing" @click="reviewAction('skip')">跳过</t-button>
          <t-button variant="outline" :loading="reviewProcessing" @click="reviewAction('ignore')">忽略并下一条</t-button>
          <t-button theme="primary" :loading="reviewProcessing" @click="reviewAction('materialize')">转素材并下一条</t-button>
        </template>
      </ResourceOverlay>
      <ResourceOverlay :visible="manualVisible" @update:visible="value => { if (!value) requestCloseManual() }" title="发现内容" :width="960" :busy="manualLoading">
        <div class="mining-detail discover-dialog">
          <ResourceChoiceCards class="mining-discovery-modes" v-model="manualMode" :options="discoverModes" :disabled="manualLoading" />
          <ResourceSection title="基本信息" variant="form">
            <ResourceForm><div class="resource-form-grid">
              <t-form-item label="运营团队"><BusinessSelect kind="team" v-model="manualTeamId" :disabled="manualTeamLocked"><t-option v-for="team in teams" :key="team.id" :value="team.id" :label="team.name" /></BusinessSelect></t-form-item>
              <t-form-item label="类目"><BusinessSelect kind="category" v-model="manualCategoryId" clearable><t-option v-for="category in enabledCategoryOptions" :key="category.id" :value="category.id" :label="category.name" /></BusinessSelect></t-form-item>
              <t-form-item label="平台"><t-select v-model="manualForm.platform"><t-option v-for="option in MINING_PLATFORM_OPTIONS" :key="option.value" :value="option.value" :label="option.label" /></t-select></t-form-item>
            </div></ResourceForm>
          </ResourceSection>
          <ResourceSection :title="manualMode === 'id' ? '输入内容' : '关键词'" variant="form">
            <template #title-extra><ResourceFieldHelp :content="manualMode === 'id' ? '支持抖音视频 ID、作品链接或分享短链接，可批量粘贴。' : '多个关键词共同用于本次搜索，可批量粘贴。'" /></template>
            <KeywordTags v-if="manualMode === 'id'" v-model="manualForm.idList" subject="ID或链接" :max="100" placeholder="输入视频 ID 或链接，按回车添加" />
            <KeywordTags v-else v-model="manualForm.keywords" :max="100" placeholder="输入关键词，按回车添加" />
          </ResourceSection>
          <ResourceNotice v-if="manualSearched">发现 {{ manualResults.length }} 条 · 已选 {{ selectedResultKeys.length }} 条，所选内容将加入内容池。</ResourceNotice>
          <ResourceEmptyState v-if="manualSearched && !manualResults.length" title="未发现内容" description="请调整输入后重新发现。" />
          <div v-if="manualResults.length" class="discover-results">
            <div class="discover-section__title">结果预览<small>仅所选内容会加入内容池</small></div>
            <t-table v-model:selected-row-keys="selectedResultKeys" :data="manualResults" :columns="resultColumns" row-key="platform_content_id" hover size="small" :scroll="{ y: '300px' }" empty="暂无结果">
              <template #title="{ row }">
                <div class="title-cell">
                  <button v-if="coverAvailable(row.cover_url)" type="button" class="title-media is-clickable" @click.stop="openImageViewer(row.cover_url)"><img :src="row.cover_url" alt="" loading="lazy" referrerpolicy="no-referrer" @error="markCoverFailed(row.cover_url)" /></button>
                  <ContentCoverPlaceholder v-else />
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
              <template #published_at="{ row }">{{ formatDateTime(row.published_at) }}</template>
            </t-table>
            <div v-if="manualMode === 'keyword' && manualSearched" class="manual-search-actions">
              <t-button variant="outline" :loading="manualLoading" @click="searchManual(true)">重新发现</t-button>
              <t-button variant="outline" :loading="manualLoading" :disabled="!manualPagination.hasMore" @click="searchManual()">下一页</t-button>
            </div>
          </div>

        </div>
        <template #footer>
          <t-button variant="outline" :disabled="manualLoading" @click="requestCloseManual">取消</t-button>
          <t-button theme="primary" :loading="manualLoading" @click="confirmManual">{{ manualResults.length ? '加入内容池' : '开始发现' }}</t-button>
        </template>
      </ResourceOverlay>
      <ContentPoolSourceDrawer v-model:visible="sourceVisible" :source="sourceRow" />
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
.title-cell { display:flex; align-items:center; gap:12px; min-width:0; }
.title-media { width:40px; height:48px; flex:none; border:0; padding:0; border-radius:4px; overflow:hidden; background:var(--wt-bg-page); }
.title-media img { width:100%; height:100%; object-fit:cover; }
.title-media.is-clickable { cursor:zoom-in; }
.title-copy { min-width:0; flex:1; }
.title-copy > a,
.title-copy > span { display:block; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.title-copy small { display:block; margin-top:5px; color:var(--wt-text-tertiary); font-size:12px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.discover-results { min-width:0; overflow:auto; }
.discover-section__title { margin-bottom:12px; font-weight:500; }
.discover-section__title small { margin-left:12px; font-weight:400; color:var(--wt-text-tertiary); }
.manual-search-actions { display:flex; justify-content:flex-end; gap:8px; margin-top:12px; }
</style>
