<script setup>
import { feedback } from '../../../shared/ui/feedback/index.js'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import { computed, h, onMounted, ref } from 'vue'
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
  materialStatusLabel,
  materialStatusTone,
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
const loading = ref(false)
const error = ref('')
const search = ref(route.query.search ? String(route.query.search) : '')
const joinedFilter = ref('')
const myUsageFilter = ref('')
const teamUsageFilter = ref('')
const teamFilter = ref('')
const categoryFilters = ref([])
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

// 每一列都必须声明 width，不许只写 minWidth —— 理由与实测数据见
// shared/testing/listPageConventions.js 里那条断言上面的说明（WebKit 的 fixed 布局下
// minWidth 不产生确定列宽，这些列会被压成「剩余空间列」）。
// 素材 ID 是第一业务列（规范 §5.1，用户裁定保留）；类目不并进「素材」副行，它有自己的一列。
// 三个使用维度各自成列：加入状态（查看者自己的关系）、我的使用（查看者是否做过成功
// 成片）、使用情况（本团队成功成片汇总）。它们不是同一个问题的不同画法，规范 §7.2
// 明令不得合并。缺值一律画占位，不兜底 —— `row.status || 'available'` 会把读不到
// 状态的素材画成「可用」。
// 使用情况是团队汇总，对全部角色成列：普通运营只少姓名，不少计数（2026-10-27 用户裁定）。
// 文件状态不进列表（同一次裁定）：它是下载链路的中间态，不是运营挑素材
// 要看的东西，需要时在详情抽屉里看。
const columns = [
  { colKey: 'id', title: '素材 ID', width: 90, sorter: true },
  { colKey: 'material', title: '素材', width: 360 },
  { colKey: 'category', title: '类目', width: 100 },
  { colKey: 'material_status', title: '素材状态', width: 100 },
  { colKey: 'joined', title: '加入状态', width: 100 },
  { colKey: 'my_usage', title: '我的使用', width: 100 },
  { colKey: 'usage', title: '使用情况', width: 200 },
  { colKey: 'published_at', title: '发布时间', width: 150, sorter: true },
  { colKey: 'created_at', title: '入库时间', width: 105, sorter: true },
  { colKey: 'created_by_name', title: '操作人', width: 110 },
  { colKey: 'op', title: '操作', width: 290, fixed: 'right' },
]

const tableScroll = computed(() => ({
  x: `${columns.reduce((sum, col) => sum + (col.width ?? 0), 0)}px`,
}))

const pagedRows = computed(() => {
  const start = (pagination.value.current - 1) * pagination.value.pageSize
  return rows.value.slice(start, start + pagination.value.pageSize)
})

// 选项是类目管理里已启用的全量类目，不是本人负责的那几个，也不依赖当前页加载过
// 哪些行：默认勾选只决定第一次看到什么，能筛到哪些类目由类目管理决定。
const categoryOptions = computed(() => categories.value.map((category) => ({ id: category.id, name: category.name })))

// quiet 由定时刷新传入：保留页码与筛选，失败沿用上一次读数。
async function load({ quiet = false } = {}) {
  if (!quiet) loading.value = true
  if (!quiet) error.value = ''
  try {
    const data = await client.list({
      search: search.value,
      team_id: teamFilter.value,
      category_ids: categoryFilters.value,
      joined: joinedFilter.value,
      my_usage: myUsageFilter.value,
      team_usage: teamUsageFilter.value,
      sort_by: sortBy.value,
      sort_order: sortOrder.value,
    })
    rows.value = Array.isArray(data) ? data : []
    if (!quiet) pagination.value.current = 1
  } catch (e) {
    if (!quiet) error.value = e?.message || '读取素材失败'
  } finally {
    if (!quiet) loading.value = false
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
  myUsageFilter.value = ''
  teamUsageFilter.value = ''
  teamFilter.value = ''
  categoryFilters.value = defaultCategoryFilters()
  load()
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
    <div class="wt-resource-page material-library-page">
      <FeedbackAlert v-if="error" theme="error" :message="error" closable style="margin-bottom:16px" @close="error=''" />
      <ResourcePageHeader title="素材库" description="团队公共素材池：加入「我的素材」后可下载到本机并用于合成" />
      <ResourceCard class="material-table-card">
        <div class="filter-bar">
          <label class="wt-filter-field wt-filter-field--wide"><span class="wt-filter-field__label">综合搜索</span><t-input v-model="search" clearable placeholder="标题 / 素材 ID / 作者" @enter="load" /></label>
          <label v-if="isAdmin" class="wt-filter-field"><span class="wt-filter-field__label">团队</span><t-select v-model="teamFilter" clearable placeholder="全部"><t-option v-for="team in teams" :key="team.id" :value="team.id" :label="team.name" /></t-select></label>
          <label class="wt-filter-field"><span class="wt-filter-field__label">类目</span><t-select v-model="categoryFilters" multiple clearable placeholder="全部"><t-option v-for="category in categoryOptions" :key="category.id" :value="category.id" :label="category.name" /></t-select></label>
          <label class="wt-filter-field"><span class="wt-filter-field__label">加入状态</span><t-select v-model="joinedFilter" :options="JOIN_FILTERS" clearable placeholder="全部" /></label>
          <label class="wt-filter-field"><span class="wt-filter-field__label">我的使用</span><t-select v-model="myUsageFilter" :options="MY_USAGE_FILTERS" clearable placeholder="全部" /></label>
          <label class="wt-filter-field"><span class="wt-filter-field__label">使用情况</span><t-select v-model="teamUsageFilter" :options="TEAM_USAGE_FILTERS" clearable placeholder="全部" /></label>
          <div class="wt-filter-actions"><t-button theme="primary" @click="load">查询</t-button><t-button class="wt-secondary-button" variant="outline" @click="reset">重置</t-button></div>
        </div>
        <div class="table-scroll-wrap">
          <t-table class="wt-resource-table" :data="pagedRows" :columns="columns" row-key="id" hover size="small" :scroll="tableScroll" :sort="{ sortBy, descending: sortOrder === 'desc' }" @sort-change="changeSort" empty="暂无素材">
            <template #id="{ row }">{{ row.id }}</template>
            <!-- 封面 + 标题 + 来源平台合成一格；标题跳来源平台落地页，没有则退回普通文本。 -->
            <template #material="{ row }">
              <div class="material-cell">
                <MaterialCover class="material-cell__cover" :url="row.cover_url" />
                <div class="material-cell__text">
                  <a v-if="row.source_url" class="wt-primary-link material-title" :href="row.source_url" target="_blank" rel="noopener noreferrer" :title="row.title || '未命名素材'">{{ row.title || '未命名素材' }}</a>
                  <span v-else class="material-title" :title="row.title || '未命名素材'">{{ row.title || '未命名素材' }}</span>
                  <span class="material-cell__source">{{ platformLabel(row.platform) }}</span>
                </div>
              </div>
            </template>
            <template #category="{ row }">{{ categoryName(categories, row.category_id) }}</template>
            <!-- 有值渲染值，没值画 —；两条分支不能合成一条带默认值的。 -->
            <template #material_status="{ row }">
              <ResourceStatusBadge v-if="row.status" :tone="materialStatusTone(row.status)" :label="materialStatusLabel(row.status)" />
              <template v-else>—</template>
            </template>
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
              <t-space class="wt-resource-actions">
                <t-button size="small" class="wt-secondary-button" variant="outline" @click="openDetail(row)">详情</t-button>
                <t-button v-if="canAdd(row) && row.usage_status !== 'active'" size="small" theme="primary" @click="addToMine(row)">{{ row.usage_status === 'removed' ? '重新加入' : '加入我的素材' }}</t-button>
                <t-button v-else-if="row.usage_status === 'active'" size="small" theme="primary" @click="goToMyMaterials()">去我的素材</t-button>
                <!-- 暂停/下架且未加入：没有可做的主操作，画 — 而不是留一颗会被拒的按钮。 -->
                <template v-else>—</template>
              </t-space>
            </template>
          </t-table>
        </div>
        <div class="pagination-bar"><t-pagination v-model:current="pagination.current" v-model:pageSize="pagination.pageSize" :total="rows.length" :page-size-options="[10, 20, 50]" /></div>
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

      <t-dialog v-model:visible="usageVisible" header="使用情况" width="420px" :footer="false">
        <div v-if="usageMaterial" class="usage-detail">
          <p class="usage-detail__total">成功成片 {{ usageOutputCount(usageMaterial) }}</p>
          <!-- 普通运营的读模型只有计数没有姓名（服务端置空 operators），这一行就是
               他们能看到的「被引用情况」；缺字段不渲染，不把未知画成 0 人。 -->
          <p v-if="usageOperatorCount(usageMaterial) > 0" class="usage-detail__total">使用人数 {{ usageOperatorCount(usageMaterial) }}</p>
          <ul v-if="usageOperators(usageMaterial).length" class="usage-detail__list">
            <li v-for="operator in usageOperators(usageMaterial)" :key="operator.id" class="usage-detail__row">
              <span class="usage-detail__name">{{ operator.name }}</span>
              <span class="usage-detail__count">成片 {{ operator.output_count }}</span>
            </li>
          </ul>
        </div>
      </t-dialog>
    </div>
  </t-loading>
</template>

<style scoped>
.material-table-card { padding: 18px 20px; margin-top: 16px; }
.material-cell { display: flex; align-items: center; gap: 10px; min-width: 0; }
.material-cell__cover { width: 56px; height: 36px; flex: none; }
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
  .material-table-card { padding: 16px; }
}
</style>
