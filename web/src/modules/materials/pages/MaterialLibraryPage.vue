<script setup>
import MaterialUsageDialog from '../components/MaterialUsageDialog.vue'
import ResourceFilterBar from '../../../shared/ui/resource/ResourceFilterBar.vue'
import ResourceDateRange from '../../../shared/ui/resource/ResourceDateRange.vue'
import '../../compose/production.css'
import MaterialMetricFilters from '../components/MaterialMetricFilters.vue'
import { publishedRangeParams, emptyMetricRanges, metricRangeParams, metricLabel } from '../materialFilters.js'
import { opColumnWidth } from '../../../shared/ui/resource/opColumnWidth.js'
import ResourceAdvancedToggle from '../../../shared/ui/resource/ResourceAdvancedToggle.vue'
import ResourceRowActions from '../../../shared/ui/resource/ResourceRowActions.vue'
import ResourceSelectionToolbar from '../../../shared/ui/resource/ResourceSelectionToolbar.vue'
import BusinessSelect from "../../../shared/ui/fields/BusinessSelect.vue"
import { feedback } from '../../../shared/ui/feedback/index.js'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import { computed, h, onMounted, ref, watch } from 'vue'
import { Button as TButton } from 'tdesign-vue-next'
import { useRoute, useRouter } from 'vue-router'
import { createMaterialsClient } from '../../../shared/api/materials.js'
import { createSessionClient } from '../../../shared/api/session.js'
import { createUsersClient } from '../../../apps/cloud/pages/users/usersApi.js'
import ResourceCard from '../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../shared/ui/resource/ResourcePageHeader.vue'
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'
import { formatDateTime } from '../../../shared/utils/datetime.js'
import { platformLabel } from '../../../shared/utils/platforms.js'
import { useAutoRefresh } from '../../../shared/utils/useAutoRefresh.js'
import {
  JOIN_FILTERS,
  MY_USAGE_FILTERS,
  TEAM_USAGE_FILTERS,
  categoryName,
  joinStatus,
  myUsageLabel,
  myUsageTone,
  usageOperatorCount,
  usageOperators,
  usageOutputCount,
  usageSummary,
} from '../labels.js'
import MaterialDetailDrawer from '../MaterialDetailDrawer.vue'
import MaterialCover from '../components/MaterialCover.vue'

const client = createMaterialsClient()
const sessionClient = createSessionClient()
const users = createUsersClient()
const route = useRoute()
const router = useRouter()

const rows = ref([])
const total = ref(0)
let loadSequence = 0
const loading = ref(false)
const error = ref('')
const appliedAdvancedCount = ref(0)
const advancedFilters = ref(false)
const search = ref(route.query.search ? String(route.query.search) : '')
const joinedFilter = ref('')
const myUsageFilter = ref('unused')
const teamUsageFilter = ref('')
const teamFilter = ref('')
const categoryFilters = ref([])
const publishedRange = ref([]), metricFilters = ref(emptyMetricRanges())
const sortBy = ref('published_at')
const sortOrder = ref('desc')
const categories = ref([])
const teams = ref([])
const currentUser = ref(null)
const detailVisible = ref(false)
const detail = ref(null)
const detailLoading = ref(false)
const usageVisible = ref(false)
const usageMaterial = ref(null)
const pagination = ref({ current: 1, pageSize: 20 })

const isAdmin = computed(() => currentUser.value?.role === 'admin')
// 使用情况对全部角色可见：服务端对所有角色都返回成片数与使用人数，只对普通运营
// 把姓名置空。所以这一列、筛选和详情段按同一套渲染，不需要角色开关。

// 普通运营与高级运营进入页面时，类目多选默认本人负责的类目；管理员默认不筛，
// 读的是所选团队（未选则跨团队）的库。清空筛选即可看到本团队全部类目。
function responsibleCategories() {
  if (isAdmin.value) return []
  return Array.isArray(currentUser.value?.category_ids) ? [...currentUser.value.category_ids] : []
}

// 默认勾选只取「本人负责 ∩ 已启用」：类目管理里停用的类目不进选项，选中它
// 只会得到一个永远空的筛选；字典还没读到时也不预选，避免选中不在选项里的 id。
function defaultCategoryFilters() {
  const enabled = new Set(categories.value.map((category) => category.id))
  return responsibleCategories().filter((id) => enabled.has(id))
}

// Explicit widths keep identity, performance and usage readable under horizontal scrolling.
const columns = [
  { colKey: 'id', title: '素材 ID', width: 80, sorter: true },
  { colKey: 'material', title: '素材 / 作者', width: 280 },
  { colKey: 'category', title: '类目', width: 100 },
  { colKey: 'metrics', title: '浏览 / 赞 / 收藏', width: 175 },
  { colKey: 'joined', title: '加入状态', width: 100 },
  { colKey: 'my_usage', title: '我的使用', width: 100 },
  { colKey: 'usage', title: '团队使用', width: 155 },
  { colKey: 'published_at', title: '发布时间', width: 150, sorter: true },
  { colKey: 'created_at', title: '入库时间', width: 105, sorter: true },
  { colKey: 'created_by_name', title: '操作人', width: 110 },
  { colKey: 'op', title: '操作', width: opColumnWidth(['详情','加入我的素材'], {variant:'text',primary:true}), fixed: 'right' },
]

const tableScroll = computed(() => ({
  x: `${columns.reduce((sum, col) => sum + (col.width ?? 0), 0)}px`,
}))

const pagedRows = computed(() => rows.value)

// 选项是类目管理里已启用的全量类目，不是本人负责的那几个，也不依赖当前页加载过
// 哪些行：默认勾选只决定第一次看到什么，能筛到哪些类目由类目管理决定。
const categoryOptions = computed(() => categories.value.map((category) => ({ id: category.id, name: category.name })))

// quiet 由定时刷新传入：保留页码与筛选，失败沿用上一次读数。
const appliedQuery = ref(null)
async function load({ quiet = false, keepPage = false, applyFilters = false } = {}) {
  if (!quiet && !keepPage) pagination.value.current = 1
  const sequence = ++loadSequence
  if (!quiet) loading.value = true
  if (!quiet) error.value = ''
  try {
    const draft = {
      page: pagination.value.current, page_size: pagination.value.pageSize,
      search: search.value,
      ...publishedRangeParams(publishedRange.value),
      ...metricRangeParams(metricFilters.value),
      team_id: teamFilter.value,
      category_ids: categoryFilters.value,
      joined: joinedFilter.value,
      my_usage: myUsageFilter.value,
      team_usage: teamUsageFilter.value,
      sort_by: sortBy.value,
      sort_order: sortOrder.value,
    }
    if (applyFilters || !appliedQuery.value) appliedQuery.value = JSON.parse(JSON.stringify(draft))
    const params = { ...appliedQuery.value, page: pagination.value.current, page_size: pagination.value.pageSize, sort_by: sortBy.value, sort_order: sortOrder.value }
    const data = await client.list(params)
    if (sequence !== loadSequence) return
    rows.value = data.items || []
    total.value = data.total || 0
    appliedAdvancedCount.value = ['view_count','like_count','favorite_count'].filter(key => [params[`${key}_min`],params[`${key}_max`]].some(value => value != null && value !== '')).length
  } catch (e) {
    if (!quiet && sequence === loadSequence) error.value = e?.message || '读取素材失败'
  } finally {
    if (!quiet && sequence === loadSequence) loading.value = false
  }
}

useAutoRefresh(() => load({ quiet: true }), {
  shouldSkip: () => detailVisible.value || detailLoading.value || usageVisible.value,
})

async function loadCategories() {
  try {
    // 类目只做筛选，不再是可见边界；名字字典用全量 enabled 类目，
    // 否则清空筛选后出现的其他类目只能显示 id。
    const data = await users.listCategories({ scope: 'all', enabled: true })
    categories.value = Array.isArray(data) ? data : []
  } catch {
    // 类目名只是展示：拿不到就退回 id（labels.js 的 categoryName 已经这么做了），
    // 不因此把整页标成出错 —— 素材本身是好的。
    categories.value = []
  }
}

// 角色与本人类目范围来自会话，只用于默认筛选；服务端仍然自己决定团队边界。
// 默认勾选要等类目字典一起到齐后再算（见 onMounted），这里不提前写 categoryFilters。
async function loadSession() {
  try {
    currentUser.value = await sessionClient.me()
  } catch {
    // 读不到会话时不给默认类目筛选：服务端按团队返回，少一层缩小是安全的方向。
    currentUser.value = null
  }
}

async function loadTeams() {
  if (!isAdmin.value) return
  try {
    const data = await users.listTeams()
    teams.value = Array.isArray(data) ? data : []
  } catch {
    teams.value = []
  }
}

// 规范 §16.2：暂停与下架禁止新增领取，已领取的行仍给「去我的素材」。
// 与状态徽章放行未知值是两回事：徽章画「可用」是替服务端宣布事实，这里只是一颗按钮。
// 服务端不兜底：POST /materials/{id}/usages 是无状态校验的 upsert，拦截靠这里。
function canAdd(row) {
  return !row.status || row.status === 'available'
}

function goToMyMaterials() {
  router.push({ name: 'MyMaterial' })
}

// `/material-library?material_id=7` 是一个真实入口：内容池里「已转素材」那一列跳过来。
// 走 getMaterial 而不是在已加载的行里找 —— 深链必须能打开任何一条素材，包括这一页
// 因为分页或筛选恰好没显示的那一条。
async function openFromQuery() {
  const raw = route.query.material_id
  if (!raw || !/^\d+$/.test(String(raw))) return
  detailVisible.value = true
  detailLoading.value = true
  try {
    detail.value = await client.get(Number(raw))
  } catch (e) {
    feedback.toast.error(e?.message || '打开素材失败')
    detailVisible.value = false
  } finally {
    detailLoading.value = false
  }
}

async function openDetail(row) {
  detailVisible.value = true
  detailLoading.value = true
  try {
    detail.value = await client.get(row.id)
  } catch (e) {
    feedback.toast.error(e?.message || '打开素材失败')
  } finally {
    detailLoading.value = false
  }
}

/**
 * 加入成功后的即时反馈。素材库不提供本机下载入口（要下载请去「我的素材」），
 * 所以这里只给一个「去我的素材」的下一步，不再是「立即下载」。
 */
function notifyAdded() {
  feedback.toast.success({
    content: () => h('div', { class: 'material-added-toast' }, [
      h('span', { class: 'material-added-toast__text' }, '已加入我的素材'),
      h(TButton, {
        size: 'small', theme: 'primary', variant: 'text', onClick: goToMyMaterials,
      }, '去我的素材'),
    ]),
    duration: 5000,
  })
}

async function addToMine(row) {
  try {
    // 200（已在「我的素材」里）与 201（新建或**恢复**）的响应体完全相同，
    // http.js 两种都返回 data，所以这里不该也无法分支。恢复就是这一条命令：
    // 冻结合同里没有单独的恢复端点。
    await client.addUsage(row.id)
    // 重新读取列表，让加入状态、我的使用与操作按钮一起翻过来；不靠本地改一行
    // 假装服务端已经投影过。
    await load()
    notifyAdded()
  } catch (e) {
    feedback.toast.error(e?.message || '加入我的素材失败')
  }
}

function reset() {
  search.value = ''
  joinedFilter.value = ''
  myUsageFilter.value = 'unused'
  publishedRange.value = []
  metricFilters.value = emptyMetricRanges()
  teamUsageFilter.value = ''
  teamFilter.value = ''
  categoryFilters.value = defaultCategoryFilters()
  load({ applyFilters: true })
}

function openUsage(row) {
  usageMaterial.value = row
  usageVisible.value = true
}

function changeSort(sort) {
  sortBy.value = sort?.sortBy || 'published_at'
  sortOrder.value = sort?.descending === false ? 'asc' : 'desc'
  load()
}

onMounted(async () => {
  // 默认勾选 = 本人负责 ∩ 类目管理启用；两者都到齐后先算出筛选，再取列表。
  await Promise.all([loadSession(), loadCategories()])
  categoryFilters.value = defaultCategoryFilters()
  load()
  loadTeams()
  openFromQuery()
})
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true">
    <div class="wt-resource-page resource-visual-approved production-page material-library-page">
      <FeedbackAlert v-if="error" theme="error" :message="error" closable style="margin-bottom:16px" @close="error=''" />
      <ResourcePageHeader title="素材库" description="筛选团队素材，加入我的素材后用于制作。" />
      <ResourceCard class="material-table-card">
        <ResourceFilterBar>
          <label class="wt-filter-field wt-filter-field--wide"><span class="wt-filter-field__label">综合搜索</span><t-input v-model="search" clearable placeholder="标题 / 素材 ID / 作者" @enter="load({ applyFilters: true })" /></label>
          <label class="wt-filter-field"><span class="wt-filter-field__label">我的使用</span><t-select v-model="myUsageFilter" :options="MY_USAGE_FILTERS" clearable placeholder="全部" /></label>
          <label v-if="isAdmin" class="wt-filter-field"><span class="wt-filter-field__label">运营团队</span><BusinessSelect kind="team" filter v-model="teamFilter" clearable><t-option v-for="team in teams" :key="team.id" :value="team.id" :label="team.name" /></BusinessSelect></label>
          <label class="wt-filter-field"><span class="wt-filter-field__label">类目</span><BusinessSelect kind="category" filter v-model="categoryFilters" multiple clearable><t-option v-for="category in categoryOptions" :key="category.id" :value="category.id" :label="category.name" /></BusinessSelect></label>
          <label class="wt-filter-field compose-date-filter"><span class="wt-filter-field__label">发布时间</span><ResourceDateRange v-model="publishedRange" value-type="YYYY-MM-DD" clearable /></label>
<label class="wt-filter-field"><span class="wt-filter-field__label">团队使用</span><t-select v-model="teamUsageFilter" :options="TEAM_USAGE_FILTERS" clearable placeholder="全部" /></label>
<label class="wt-filter-field"><span class="wt-filter-field__label">加入状态</span><t-select v-model="joinedFilter" :options="JOIN_FILTERS" clearable placeholder="全部" /></label>
<div class="wt-filter-actions"><t-button theme="primary" @click="load({ applyFilters: true })">查询</t-button><t-button class="wt-secondary-button" variant="outline" @click="reset">重置</t-button></div>
          <ResourceAdvancedToggle v-model="advancedFilters" :count="appliedAdvancedCount" controls="material-advanced-filters" />
          <div v-if="advancedFilters" id="material-advanced-filters" class="wt-advanced-filters">



          <MaterialMetricFilters v-model="metricFilters" />
          </div>
        </ResourceFilterBar>
        <div class="table-scroll-wrap">
          <t-table class="wt-resource-table" :data="pagedRows" :columns="columns" row-key="id" hover size="small" :scroll="tableScroll" :sort="{ sortBy, descending: sortOrder === 'desc' }" @sort-change="changeSort" empty="暂无素材">
            <template #id="{ row }">{{ row.id }}</template>
            <!-- 标题进入素材详情，原作品入口统一收在基本信息。 -->
            <template #material="{ row }">
              <div class="material-cell">
                <MaterialCover class="material-cell__cover" :url="row.cover_url" />
                <div class="material-cell__text">
                  <t-link theme="primary" class="material-title" :title="row.title || '未命名素材'" @click="openDetail(row)">{{ row.title || '未命名素材' }}</t-link>
                  <span class="material-cell__source">{{ row.author_name || '—' }} · {{ platformLabel(row.platform) }}</span>
                </div>
              </div>
            </template>
            <template #category="{ row }">{{ categoryName(categories, row.category_id) }}</template>
            <template #metrics="{row}"><div class="material-metrics"><span>{{ metricLabel(row,'view_count') }} 浏览</span><span>{{ metricLabel(row,'like_count') }} 赞 · {{ metricLabel(row,'favorite_count') }} 收藏</span></div></template>
            <!-- 加入状态读服务端投影：active=已加入，removed=已移出，无关系=未加入。 -->
            <template #joined="{ row }"><ResourceStatusBadge :tone="joinStatus(row).tone" :label="joinStatus(row).label" /></template>
            <template #my_usage="{ row }"><ResourceStatusBadge :tone="myUsageTone(row.my_usage)" :label="myUsageLabel(row.my_usage)" /></template>
            <!-- 一行摘要，整格可点开明细；没有成功成片就只画 —，不弹一个空弹窗。 -->
            <template #usage="{ row }">
              <button v-if="usageOutputCount(row)" type="button" class="usage-cell" @click="openUsage(row)">{{ usageSummary(row) }}</button>
              <span v-else class="usage-cell usage-cell--empty">—</span>
            </template>
            <template #created_at="{ row }">{{ formatDateTime(row.created_at) }}</template>
            <!-- 操作人 = 素材创建人；目录里已没有此人时留空，画 —，不伪造姓名。 -->
            <template #created_by_name="{ row }">{{ row.created_by_name || '—' }}</template>
            <template #published_at="{ row }">{{ formatDateTime(row.published_at) }}</template>
            <template #op="{ row }">
              <!-- 详情 | 单一主操作；已加入的行换成「去我的素材」，不再给必然空转的「加入」。
                   已移出的行给「重新加入」：服务端的同一条 upsert 会把它恢复成 active。 -->
              <ResourceRowActions>
                <t-button size="small" class="wt-secondary-button" variant="text" @click="openDetail(row)">详情</t-button>
                <t-button v-if="canAdd(row) && row.usage_status !== 'active'" size="small" theme="primary" variant="text" class="is-primary" @click="addToMine(row)">{{ row.usage_status === 'removed' ? '重新加入' : '加入我的素材' }}</t-button>
                <t-button v-else-if="row.usage_status === 'active'" size="small" theme="primary" variant="text" class="is-primary" @click="goToMyMaterials()">去我的素材</t-button>
                <!-- 暂停/下架且未加入：没有可做的主操作，画 — 而不是留一颗会被拒的按钮。 -->
                <template v-else>—</template>
              </ResourceRowActions>
            </template>
          </t-table>
        </div>
        <div class="pagination-bar"><t-pagination :show-jumper="false" v-model:current="pagination.current" v-model:pageSize="pagination.pageSize" :total="total" :page-size-options="[10, 20, 50]" @current-change="load({ keepPage: true })" @page-size-change="load({ keepPage: true })" /></div>
      </ResourceCard>

      <MaterialDetailDrawer
        v-model:visible="detailVisible"
        :material="detail"
        :loading="detailLoading"
        :categories="categories"
        :mine="detail?.usage_status === 'active'"
        mode="library"
        @add="addToMine"
        @go-mine="goToMyMaterials"
      />

      <MaterialUsageDialog v-model:visible="usageVisible" :material="usageMaterial" />
    </div>
  </t-loading>
</template>

<style scoped>
.material-table-card { margin-top: 16px; }
.material-metrics { display:grid;gap:4px; }
.material-metrics span+span { font-size:12px;color:var(--wt-text-tertiary); }
.material-cell { display: flex; align-items: center; gap: 10px; min-width: 0; }
.material-cell__cover { width: 42px; height: 52px; flex: none; }
.material-cell__text { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.material-cell__source { color: var(--wt-text-tertiary); font-size: 12px; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
/* 一行不折行：行高跟着数据变，一张表里每行不一样高，扫描成本立刻上去。 */
.usage-cell { display: block; max-width: 100%; padding: 0; border: 0; background: none; color: var(--wt-primary); font: inherit; font-size: 13px; text-align: left; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; cursor: pointer; }
.usage-cell--empty { color: var(--wt-text-secondary); cursor: default; }
.usage-cell:not(.usage-cell--empty):hover, .usage-cell:not(.usage-cell--empty):focus-visible { text-decoration: underline; }
/* 使用人可能很多：弹窗列表自己滚动，不把弹窗撑出屏幕。 */
.usage-detail__total { margin: 0 0 12px; color: var(--wt-text-primary); font-size: 13px; }
.usage-detail__list { max-height: 280px; margin: 0; padding: 0; overflow-y: auto; list-style: none; }
.usage-detail__row { display: flex; justify-content: space-between; gap: 12px; padding: 8px 0; border-top: 1px dashed var(--wt-border); }
.usage-detail__row:first-child { border-top: 0; }
.usage-detail__name { min-width: 0; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
.usage-detail__count { flex: none; color: var(--wt-text-tertiary); font-size: 12px; }
/* 颜色只写在 span 分支上，免得覆盖 .wt-primary-link 的主题色。 */
.material-title { display: block; overflow: hidden; font-weight: 600; white-space: nowrap; text-overflow: ellipsis; }
span.material-title { color: var(--wt-text-primary); }
.wt-primary-link { color: var(--wt-primary); font-weight: 600; text-decoration: none; }
.wt-primary-link:hover, .wt-primary-link:focus-visible { text-decoration: underline; }
.material-added-toast { display: flex; align-items: center; gap: 12px; }
.material-added-toast__text { color: var(--wt-text-primary); }
@media (max-width: 680px) {
}
</style>
