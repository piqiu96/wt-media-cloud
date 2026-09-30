<script setup>
import { computed, h, onMounted, ref } from 'vue'
import { Button as TButton, MessagePlugin } from 'tdesign-vue-next'
import { useRoute, useRouter } from 'vue-router'
import { createMaterialsClient } from '../../../shared/api/materials.js'
import { createUsersClient } from '../../../apps/cloud/pages/users/usersApi.js'
import ResourceCard from '../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../shared/ui/resource/ResourcePageHeader.vue'
import ResourceStatGrid from '../../../shared/ui/resource/ResourceStatGrid.vue'
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'
import { formatDateTime } from '../../../shared/utils/datetime.js'
import { createDownloadFailureMessage } from '../../transfer/downloadErrors.js'
import { VIDEO_STATUSES, gameName, videoStatusLabel, videoStatusTone } from '../labels.js'
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
// 走查四轮：封面、标题、来源平台合成一个「素材」格（设计图：封面+标题合并，减少列数）；
// 素材 ID 仍是第一业务列（规范 §5.1，用户裁定保留）。作者、链接与体积仍在详情抽屉里。
// 游戏不并进副行：它有自己的一列，同一格里再说一遍就是重复。
const columns = [
  { colKey: 'id', title: '素材 ID', width: 90 },
  { colKey: 'material', title: '素材', width: 380 },
  { colKey: 'game', title: '游戏', width: 100 },
  { colKey: 'video_status', title: '文件状态', width: 100 },
  { colKey: 'created_at', title: '入库时间', width: 105 },
  { colKey: 'op', title: '操作', width: 290, fixed: 'right' },
]

const tableScroll = computed(() => ({
  x: `${columns.reduce((sum, col) => sum + (col.width ?? 0), 0)}px`,
}))

// 服务端只读 `search`（见 shared/api/materials.js 的白名单），所以文件状态与游戏这两个
// 筛选只能在**已加载的行**上做。这不是「假装筛了」：素材一次全取（服务端上限 200 条），
// 筛的就是全部；给服务端加一个它读不了的参数才是装样子。
const filteredRows = computed(() => rows.value.filter((row) => (
  (!statusFilter.value || row.video_status === statusFilter.value)
  && (!gameFilter.value || row.game_id === gameFilter.value)
)))

const pagedRows = computed(() => {
  const start = (pagination.value.current - 1) * pagination.value.pageSize
  return filteredRows.value.slice(start, start + pagination.value.pageSize)
})

const stats = computed(() => [
  { key: 'all', label: '全部素材', value: rows.value.length, tone: 'info' },
  ...VIDEO_STATUSES.map((status) => ({
    key: status,
    label: videoStatusLabel(status),
    value: rows.value.filter((row) => row.video_status === status).length,
    tone: videoStatusTone(status),
  })),
])

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
    const data = await client.list({ search: search.value })
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
    MessagePlugin.error(e?.message || '打开素材失败')
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
    MessagePlugin.error(e?.message || '打开素材失败')
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
  MessagePlugin.success({
    content: () => h('div', { class: 'material-added-toast' }, actions),
    duration: 5000,
  })
}

async function downloadNow(row) {
  try {
    await client.createDownload(row.id)
    MessagePlugin.success('已开始下载')
  } catch (e) {
    MessagePlugin.error(createDownloadFailureMessage(e))
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
    MessagePlugin.error(e?.message || '加入我的素材失败')
  }
}

function reset() {
  search.value = ''
  statusFilter.value = ''
  gameFilter.value = ''
  load()
}

function applyStatFilter(key) {
  statusFilter.value = key === 'all' ? '' : key
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
      <t-alert v-if="error" theme="error" :message="error" closable style="margin-bottom:16px" @close="error=''" />
      <ResourcePageHeader title="素材库" description="云端的原素材：加入我的素材后即可下载到本机">
        <template #actions>
          <t-button class="wt-secondary-button" variant="outline" @click="load">刷新</t-button>
        </template>
      </ResourcePageHeader>
      <ResourceStatGrid :items="stats" @select="applyStatFilter" />
      <ResourceCard class="material-filter-card">
        <div class="filter-row">
          <label class="filter-field"><span>综合搜索</span><t-input v-model="search" clearable placeholder="标题 / 素材 ID / 作者" style="width:220px" @enter="load" /></label>
          <label class="filter-field"><span>文件状态</span><t-select v-model="statusFilter" clearable placeholder="文件状态" style="width:140px"><t-option v-for="status in VIDEO_STATUSES" :key="status" :value="status" :label="videoStatusLabel(status)" /></t-select></label>
          <label class="filter-field"><span>游戏</span><t-select v-model="gameFilter" clearable placeholder="游戏" style="width:140px"><t-option v-for="game in gameOptions" :key="game.id" :value="game.id" :label="game.name" /></t-select></label>
          <t-button theme="primary" @click="load">查询</t-button>
          <t-button class="wt-secondary-button" variant="outline" @click="reset">重置</t-button>
        </div>
      </ResourceCard>

      <ResourceCard class="material-table-card">
        <div class="table-scroll-wrap">
          <t-table class="wt-resource-table" :data="pagedRows" :columns="columns" row-key="id" hover size="small" :scroll="tableScroll" empty="暂无素材">
            <template #id="{ row }">{{ row.id }}</template>
            <!-- 走查四轮：三样识别信息合成一格——封面认内容、标题认是哪条、来源平台认从哪来。
                 标题蓝色可点，跳来源平台落地页（source_url），与内容池页同一形状；
                 没有落地页的素材退回普通文本，链接形状留给真的能点的东西。 -->
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
            <template #video_status="{ row }"><ResourceStatusBadge :tone="videoStatusTone(row.video_status)" :label="videoStatusLabel(row.video_status)" /></template>
            <template #created_at="{ row }">{{ formatDateTime(row.created_at) }}</template>
            <template #op="{ row }">
              <!-- 走查三轮（交互对齐 §2.3/§5.5）：详情 | 单一主操作，素材库的主操作是
                   领取（加入我的素材），取文件归「我的素材」，行里不给第二条路。
                   走查四轮：已加入的行把主操作换成「去我的素材」——对一个已经在
                   「我的素材」里的素材再给一颗「加入我的素材」，是一次必然空转的点击。 -->
              <t-space class="wt-resource-actions">
                <t-button size="small" class="wt-secondary-button" variant="outline" @click="openDetail(row)">详情</t-button>
                <t-button v-if="!isMine(row)" size="small" theme="primary" @click="addToMine(row)">加入我的素材</t-button>
                <t-button v-else size="small" theme="primary" @click="goToMyMaterials()">去我的素材</t-button>
              </t-space>
            </template>
          </t-table>
        </div>
        <div class="pagination-bar"><t-pagination v-model:current="pagination.current" v-model:pageSize="pagination.pageSize" :total="filteredRows.length" :page-size-options="[10, 20, 50]" /></div>
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
.material-filter-card { padding: 14px 20px; }
.material-table-card { padding: 18px 20px; margin-top: 16px; }
.filter-row { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.filter-field { display: flex; align-items: center; gap: 8px; color: var(--wt-text-secondary); font-size: 13px; font-weight: 500; white-space: nowrap; }
.material-cell { display: flex; align-items: center; gap: 10px; min-width: 0; }
.material-cell__cover { width: 56px; height: 36px; flex: none; }
.material-cell__text { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.material-cell__source { color: var(--wt-text-tertiary); font-size: 12px; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
/* 标题两种分支（外链 a / 普通 span）共用省略号；颜色只写在 span 分支上，
   免得覆盖 .wt-primary-link 的主题色——与内容池页同一条教训。 */
.material-title { display: block; overflow: hidden; font-weight: 600; white-space: nowrap; text-overflow: ellipsis; }
span.material-title { color: var(--wt-text-primary); }
.wt-primary-link { color: var(--wt-primary); font-weight: 600; text-decoration: none; }
.wt-primary-link:hover, .wt-primary-link:focus-visible { text-decoration: underline; }
/* 反馈里的两颗动作按钮与文字同排，别让消息撑成两行。 */
.material-added-toast { display: flex; align-items: center; gap: 12px; }
.material-added-toast__text { color: var(--wt-text-primary); }
@media (max-width: 680px) {
  .material-filter-card, .material-table-card { padding: 16px; }
}
</style>
