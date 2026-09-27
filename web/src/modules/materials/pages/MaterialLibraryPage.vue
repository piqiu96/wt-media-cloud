<script setup>
import { computed, onMounted, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { useRoute } from 'vue-router'
import { createMaterialsClient } from '../../../shared/api/materials.js'
import { createUsersClient } from '../../../apps/cloud/pages/users/usersApi.js'
import ResourceCard from '../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../shared/ui/resource/ResourcePageHeader.vue'
import ResourceStatGrid from '../../../shared/ui/resource/ResourceStatGrid.vue'
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'
import { formatDateTime } from '../../../shared/utils/datetime.js'
import { formatBytes } from '../../../shared/utils/units.js'
import { VIDEO_STATUSES, canDownload, gameName, videoStatusLabel, videoStatusTone } from '../labels.js'
import MaterialDetailDrawer from '../MaterialDetailDrawer.vue'
import { createDownloadFailureMessage } from '../../transfer/downloadErrors.js'
import { useDownloadCentre } from '../../transfer/downloadCentre.js'

const client = createMaterialsClient()
const users = createUsersClient()
const route = useRoute()
const downloadCentre = useDownloadCentre()

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

// 每一列都必须声明 width，不许只写 minWidth —— 理由与实测数据见
// shared/testing/listPageConventions.js 里那条断言上面的说明（WebKit 的 fixed 布局下
// minWidth 不产生确定列宽，这些列会被压成「剩余空间列」）。
const columns = [
  { colKey: 'title', title: '素材', width: 340 },
  { colKey: 'source', title: '来源', width: 280 },
  { colKey: 'game', title: '游戏', width: 140 },
  { colKey: 'video_status', title: '视频状态', width: 140 },
  { colKey: 'video_size_bytes', title: '体积', width: 120 },
  { colKey: 'created_at', title: '入库时间', width: 150 },
  { colKey: 'op', title: '操作', width: 280, fixed: 'right' },
]

const tableScroll = computed(() => ({
  x: `${columns.reduce((sum, col) => sum + (col.width ?? 0), 0)}px`,
}))

// 服务端只读 `search`（见 shared/api/materials.js 的白名单），所以视频状态与游戏这两个
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

async function addToMine(row) {
  try {
    // 200（已在「我的素材」里）与 201（新建或**恢复**）的响应体完全相同，
    // http.js 两种都返回 data，所以这里不该也无法分支。恢复就是这一条命令：
    // 冻结合同里没有单独的恢复端点。
    await client.addUsage(row.id)
    MessagePlugin.success('已加入我的素材')
  } catch (e) {
    MessagePlugin.error(e?.message || '加入我的素材失败')
  }
}

async function download(row) {
  try {
    await client.createDownload(row.id)
    MessagePlugin.success('已开始下载，可在下载中心查看进度')
    downloadCentre.open()
  } catch (e) {
    // 409 的两种：素材还没准备好、本机没有可用的下载节点。它们靠响应里的 error.type
    // 区分（errcode 之别无意义），由 createDownloadFailureMessage 翻成人话。
    MessagePlugin.error(createDownloadFailureMessage(e))
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
  openFromQuery()
})
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true">
    <div class="wt-resource-page material-library-page">
      <t-alert v-if="error" theme="error" :message="error" closable style="margin-bottom:16px" @close="error=''" />
      <ResourcePageHeader title="素材库" description="云端的原素材：视频准备就绪后即可下载到本机">
        <template #actions>
          <t-button class="wt-secondary-button" variant="outline" @click="load">刷新</t-button>
        </template>
      </ResourcePageHeader>
      <ResourceStatGrid :items="stats" @select="applyStatFilter" />
      <ResourceCard>
        <div class="filter-bar">
          <t-space wrap>
            <label class="wt-filter-field"><span class="wt-filter-field__label">综合搜索</span><t-input v-model="search" clearable placeholder="标题" @enter="load" /></label>
            <label class="wt-filter-field"><span class="wt-filter-field__label">视频状态</span><t-select v-model="statusFilter" clearable placeholder="全部"><t-option v-for="status in VIDEO_STATUSES" :key="status" :value="status" :label="videoStatusLabel(status)" /></t-select></label>
            <label class="wt-filter-field"><span class="wt-filter-field__label">游戏</span><t-select v-model="gameFilter" clearable placeholder="全部"><t-option v-for="game in gameOptions" :key="game.id" :value="game.id" :label="game.name" /></t-select></label>
            <div class="wt-filter-actions"><t-button theme="primary" @click="load">查询</t-button><t-button class="wt-secondary-button" variant="outline" @click="reset">重置</t-button></div>
          </t-space>
        </div>
        <div class="table-scroll-wrap">
          <t-table class="wt-resource-table" :data="pagedRows" :columns="columns" row-key="id" hover size="small" :scroll="tableScroll" empty="暂无素材">
            <template #title="{ row }">
              <div class="material-cell">
                <a v-if="row.source_url" class="wt-primary-link" :href="row.source_url" target="_blank" rel="noopener noreferrer" :title="row.title || '未命名素材'">{{ row.title || '未命名素材' }}</a>
                <span v-else :title="row.title || '未命名素材'">{{ row.title || '未命名素材' }}</span>
                <small>#{{ row.id }}<template v-if="row.author_name"> · {{ row.author_name }}</template></small>
              </div>
            </template>
            <template #source="{ row }">
              <div class="material-source">
                <span>{{ row.platform || '-' }}</span>
                <small v-if="row.published_at">发布 {{ formatDateTime(row.published_at) }}</small>
              </div>
            </template>
            <template #game="{ row }">{{ gameName(games, row.game_id) }}</template>
            <template #video_status="{ row }"><ResourceStatusBadge :tone="videoStatusTone(row.video_status)" :label="videoStatusLabel(row.video_status)" /></template>
            <template #video_size_bytes="{ row }">{{ formatBytes(row.video_size_bytes) }}</template>
            <template #created_at="{ row }">{{ formatDateTime(row.created_at) }}</template>
            <template #op="{ row }">
              <t-space class="wt-resource-actions">
                <t-button size="small" class="wt-secondary-button" variant="outline" @click="openDetail(row)">查看</t-button>
                <t-button size="small" class="wt-secondary-button" variant="outline" @click="addToMine(row)">加入我的素材</t-button>
                <!-- 只有 ready 的素材有对象键可下；其余状态点了必然是 409，所以按钮就该是灰的。 -->
                <t-button size="small" theme="primary" :disabled="!canDownload(row)" @click="download(row)">下载到本机</t-button>
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
        @add="addToMine"
        @download="download"
      />
    </div>
  </t-loading>
</template>

<style scoped>
.material-cell { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.material-cell a, .material-cell > span { overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
.material-cell small { color: var(--wt-text-tertiary); font-size: 12px; }
.material-source { display: flex; flex-direction: column; gap: 2px; }
.material-source small { color: var(--wt-text-tertiary); font-size: 12px; }
</style>
