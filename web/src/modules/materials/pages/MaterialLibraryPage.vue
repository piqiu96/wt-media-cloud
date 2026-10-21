<script setup>
import { feedback } from '../../../shared/ui/feedback/index.js'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import { computed, h, onMounted, ref } from 'vue'
import { Button as TButton } from 'tdesign-vue-next'
import { useRoute, useRouter } from 'vue-router'
import { createMaterialsClient } from '../../../shared/api/materials.js'
import { createUsersClient } from '../../../apps/cloud/pages/users/usersApi.js'
import ResourceCard from '../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../shared/ui/resource/ResourcePageHeader.vue'
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'
import { formatDateTime } from '../../../shared/utils/datetime.js'
import { createDownloadFailureMessage } from '../../transfer/downloadErrors.js'
import { VIDEO_STATUSES, gameName, materialStatusLabel, materialStatusTone, usageLines, videoStatusLabel, videoStatusTone } from '../labels.js'
import MaterialDetailDrawer from '../MaterialDetailDrawer.vue'
import MaterialCover from '../components/MaterialCover.vue'

const client = createMaterialsClient()
const users = createUsersClient()
const route = useRoute()
const router = useRouter()

const rows = ref([])
const loading = ref(false)
const error = ref('')
const search = ref(route.query.search ? String(route.query.search) : '')
const statusFilter = ref('')
const gameFilter = ref('')
const sortBy = ref('published_at')
const sortOrder = ref('desc')
const games = ref([])
const detailVisible = ref(false)
const detail = ref(null)
const detailLoading = ref(false)
const pagination = ref({ current: 1, pageSize: 20 })

// 每行的「已加入我的素材」状态。列表接口不返回它（`material` 上没有「领取人」字段，
// 关系在 `material_usages` 里），但 `/api/v1/my-materials` 已经能全量取回当前用户的
// 关系，按素材 id join 一次就有 —— 不为此新增后端字段（2026-09-30 用户裁定）。
const mineIds = ref(new Set())

// 每一列都必须声明 width，不许只写 minWidth —— 理由与实测数据见
// shared/testing/listPageConventions.js 里那条断言上面的说明（WebKit 的 fixed 布局下
// minWidth 不产生确定列宽，这些列会被压成「剩余空间列」）。
// 素材 ID 是第一业务列（规范 §5.1，用户裁定保留）；游戏不并进「素材」副行，它有自己的一列。
// 两个状态维度并排（规范 §7.2）：素材状态已有真值（materials.status），使用情况还没有。
// 缺值一律画占位，不兜底 —— `row.status || 'available'` 会把读不到状态的素材画成「可用」。
const columns = [
  { colKey: 'id', title: '素材 ID', width: 90, sorter: true },
  { colKey: 'material', title: '素材', width: 360 },
  { colKey: 'game', title: '游戏', width: 100 },
  { colKey: 'material_status', title: '素材状态', width: 100 },
  { colKey: 'video_status', title: '文件状态', width: 100 },
  { colKey: 'usage', title: '使用情况', width: 170 },
  { colKey: 'published_at', title: '发布时间', width: 150, sorter: true },
  { colKey: 'created_at', title: '入库时间', width: 105, sorter: true },
  { colKey: 'op', title: '操作', width: 290, fixed: 'right' },
]

const tableScroll = computed(() => ({
  x: `${columns.reduce((sum, col) => sum + (col.width ?? 0), 0)}px`,
}))

const pagedRows = computed(() => {
  const start = (pagination.value.current - 1) * pagination.value.pageSize
  return rows.value.slice(start, start + pagination.value.pageSize)
})

// 筛选项只列**已加载数据里真的出现过**的游戏：列一个点下去没有任何行的选项，
// 与「筛了但服务端没筛」是同一种误导。
const gameOptions = computed(() => {
  const used = [...new Set(rows.value.map((row) => row.game_id).filter(Boolean))]
  return used.map((id) => ({ id, name: gameName(games.value, id) }))
})

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await client.list({ search: search.value, game_id: gameFilter.value, video_status: statusFilter.value, sort_by: sortBy.value, sort_order: sortOrder.value })
    rows.value = Array.isArray(data) ? data : []
    pagination.value.current = 1
  } catch (e) {
    error.value = e?.message || '读取素材失败'
  } finally {
    loading.value = false
  }
}

async function loadGames() {
  try {
    const data = await users.listGames()
    games.value = Array.isArray(data) ? data : []
  } catch {
    // 游戏名只是展示：拿不到就退回 id（labels.js 的 gameName 已经这么做了），
    // 不因此把整页标成出错 —— 素材本身是好的。
    games.value = []
  }
}

// 拿不到关系列表只影响「去我的素材」这个分支：退回「加入我的素材」是对的，
// 那条命令是幂等的（已存在时服务端返回 200 而不是再建一条），不会因此多出关系。
async function loadMine() {
  try {
    const data = await client.listMyMaterials()
    mineIds.value = new Set((Array.isArray(data) ? data : []).map((usage) => usage.material_id))
  } catch {
    mineIds.value = new Set()
  }
}

function isMine(row) {
  return Boolean(row?.id) && mineIds.value.has(row.id)
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
 * 加入成功后的即时反馈（2026-09-30 走查四轮，用户裁定）。
 *
 * 这是素材库上下文里**唯一**出现「下载」的地方：行与详情抽屉都不提供下载入口，
 * 但刚做完「领取」这个动作时，下一步几乎必然是「取文件」，让人先跳到「我的素材」
 * 再点一次是把一次跳转换一个说法而已。所以两颗按钮：一步下载（仅文件已就绪时出现，
 * 未就绪时它必然换来一个 409）与去「我的素材」。
 */
function notifyAdded(row) {
  const actions = [
    h('span', { class: 'material-added-toast__text' }, '已加入我的素材'),
  ]
  if (row.video_status === 'ready') {
    actions.push(h(TButton, {
      size: 'small', theme: 'primary', variant: 'text', onClick: () => downloadNow(row),
    }, '立即下载'))
  }
  actions.push(h(TButton, {
    size: 'small', theme: 'primary', variant: 'text', onClick: goToMyMaterials,
  }, '去我的素材'))
  feedback.toast.success({
    content: () => h('div', { class: 'material-added-toast' }, actions),
    duration: 5000,
  })
}

async function downloadNow(row) {
  try {
    await client.createDownload(row.id)
    feedback.toast.success('已开始下载')
  } catch (e) {
    feedback.toast.error(createDownloadFailureMessage(e))
  }
}

async function addToMine(row) {
  try {
    // 200（已在「我的素材」里）与 201（新建或**恢复**）的响应体完全相同，
    // http.js 两种都返回 data，所以这里不该也无法分支。恢复就是这一条命令：
    // 冻结合同里没有单独的恢复端点。
    await client.addUsage(row.id)
    // 立刻翻成「去我的素材」，不让运营再刷新一次才看见结果。返回 200 时它本来
    // 就已经在这个集合里，重复 add 无害。
    mineIds.value.add(row.id)
    notifyAdded(row)
  } catch (e) {
    feedback.toast.error(e?.message || '加入我的素材失败')
  }
}

function reset() {
  search.value = ''
  statusFilter.value = ''
  gameFilter.value = ''
  load()
}

function changeSort(sort) {
  sortBy.value = sort?.sortBy || 'published_at'
  sortOrder.value = sort?.descending === false ? 'asc' : 'desc'
  load()
}

onMounted(() => {
  load()
  loadGames()
  loadMine()
  openFromQuery()
})
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true">
    <div class="wt-resource-page material-library-page">
      <FeedbackAlert v-if="error" theme="error" :message="error" closable style="margin-bottom:16px" @close="error=''" />
      <ResourcePageHeader title="素材库" description="云端的原素材：加入我的素材后即可下载到本机">
        <template #actions>
          <t-button class="wt-secondary-button" variant="outline" @click="load">刷新</t-button>
        </template>
      </ResourcePageHeader>
      <ResourceCard class="material-table-card">
        <div class="filter-bar">
          <label class="wt-filter-field wt-filter-field--wide"><span class="wt-filter-field__label">综合搜索</span><t-input v-model="search" clearable placeholder="标题 / 素材 ID / 作者" @enter="load" /></label>
          <label class="wt-filter-field"><span class="wt-filter-field__label">文件状态</span><t-select v-model="statusFilter" clearable placeholder="全部"><t-option v-for="status in VIDEO_STATUSES" :key="status" :value="status" :label="videoStatusLabel(status)" /></t-select></label>
          <label class="wt-filter-field"><span class="wt-filter-field__label">游戏</span><t-select v-model="gameFilter" clearable placeholder="全部"><t-option v-for="game in gameOptions" :key="game.id" :value="game.id" :label="game.name" /></t-select></label>
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
                  <span class="material-cell__source">{{ row.platform || '-' }}</span>
                </div>
              </div>
            </template>
            <template #game="{ row }">{{ gameName(games, row.game_id) }}</template>
            <!-- 有值渲染值，没值画 —；两条分支不能合成一条带默认值的。 -->
            <template #material_status="{ row }">
              <ResourceStatusBadge v-if="row.status" :tone="materialStatusTone(row.status)" :label="materialStatusLabel(row.status)" />
              <template v-else>—</template>
            </template>
            <template #video_status="{ row }"><ResourceStatusBadge :tone="videoStatusTone(row.video_status)" :label="videoStatusLabel(row.video_status)" /></template>
            <template #usage="{ row }">
              <div class="usage-cell">
                <span class="usage-cell__counts">{{ usageLines(row).counts }}</span>
                <span class="usage-cell__recent">{{ usageLines(row).recent }}</span>
              </div>
            </template>
            <template #created_at="{ row }">{{ formatDateTime(row.created_at) }}</template>
            <template #published_at="{ row }">{{ formatDateTime(row.published_at) }}</template>
            <template #op="{ row }">
              <!-- 详情 | 单一主操作；已加入的行换成「去我的素材」，不再给必然空转的「加入」。 -->
              <t-space class="wt-resource-actions">
                <t-button size="small" class="wt-secondary-button" variant="outline" @click="openDetail(row)">详情</t-button>
                <t-button v-if="canAdd(row) && !isMine(row)" size="small" theme="primary" @click="addToMine(row)">加入我的素材</t-button>
                <t-button v-else-if="isMine(row)" size="small" theme="primary" @click="goToMyMaterials()">去我的素材</t-button>
                <!-- 暂停/下架且未领取：没有可做的主操作，画 — 而不是留一颗会被拒的按钮。 -->
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
        :games="games"
        :mine="isMine(detail)"
        mode="library"
        @add="addToMine"
        @go-mine="goToMyMaterials"
      />
    </div>
  </t-loading>
</template>

<style scoped>
.material-table-card { padding: 18px 20px; margin-top: 16px; }
.material-cell { display: flex; align-items: center; gap: 10px; min-width: 0; }
.material-cell__cover { width: 56px; height: 36px; flex: none; }
.material-cell__text { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.material-cell__source { color: var(--wt-text-tertiary); font-size: 12px; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
/* 两行都不折行：行高跟着数据变，一张表里每行不一样高，扫描成本立刻上去。 */
.usage-cell { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.usage-cell__counts { color: var(--wt-text-primary); font-size: 13px; white-space: nowrap; }
.usage-cell__recent { color: var(--wt-text-tertiary); font-size: 12px; white-space: nowrap; }
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
