<script setup>
import { computed, onMounted, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { useRoute, useRouter } from 'vue-router'
import { createMaterialsClient } from '../../../shared/api/materials.js'
import { createUsersClient } from '../../../apps/cloud/pages/users/usersApi.js'
import ResourceCard from '../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../shared/ui/resource/ResourcePageHeader.vue'
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'
import { formatDateTime } from '../../../shared/utils/datetime.js'
import {
  VIDEO_STATUSES,
  downloadActionLabel,
  gameName,
  usageStatusLabel,
  usageStatusTone,
  videoStatusLabel,
  videoStatusTone,
} from '../labels.js'
import MaterialDetailDrawer from '../MaterialDetailDrawer.vue'
import MaterialCover from '../components/MaterialCover.vue'
import { createDownloadFailureMessage } from '../../transfer/downloadErrors.js'
import { useDownloadCentre } from '../../transfer/downloadCentre.js'

const client = createMaterialsClient()
const users = createUsersClient()
const route = useRoute()
const router = useRouter()
const downloadCentre = useDownloadCentre()

const rows = ref([])
const loading = ref(false)
const error = ref('')
const search = ref('')
const statusFilter = ref('')
const games = ref([])
const detailVisible = ref(false)
const detail = ref(null)
const detailLoading = ref(false)
const pagination = ref({ current: 1, pageSize: 20 })

// 行内只留识别信息；作者、链接与体积在详情抽屉里（CHG-20260930-069）。素材 ID 是
// 第一列——与素材库同一行形状，走查反馈：运营扫行时先找编号。
//
// 走查七轮：游戏不再独立成列，收进「素材」格的副行（与作者同格），腾出的位置给
// 「使用状态」——这一页第一次同时有两个状态维度要并排（规范 §7.2）。使用状态是
// 关系自己的状态，文件状态是素材的，两者不合并。
const columns = [
  { colKey: 'id', title: '素材 ID', width: 90 },
  { colKey: 'material', title: '素材', width: 380 },
  { colKey: 'video_status', title: '文件状态', width: 100 },
  { colKey: 'usage_status', title: '使用状态', width: 100 },
  { colKey: 'added_at', title: '加入时间', width: 105 },
  { colKey: 'op', title: '操作', width: 285, fixed: 'right' },
]

const tableScroll = computed(() => ({
  x: `${columns.reduce((sum, col) => sum + (col.width ?? 0), 0)}px`,
}))

/**
 * 一行 = 一条「我的素材」关系，展示的是它**嵌入的素材**。
 *
 * 服务端已经把 `material` 嵌在每条 usage 上（冻结的 `MaterialUsage` 就是这么定义的），
 * 所以这一页不需要第二个请求。展平成一个对象是为了让列声明只有一个数据源；关系自己的
 * 两个字段用不同的键名带上，不会与素材自己的 `id`／`created_at` 混淆 —— 「移出」操作
 * 删的是**关系**，拿素材 id 去删会删错东西。
 */
function toRows(usages) {
  return usages.map((usage) => ({
    ...(usage.material || {}),
    usage_id: usage.id,
    usage_status: usage.status,
    added_at: usage.created_at,
  }))
}

// 「素材」格副行：游戏 · 作者。游戏查不到名字时 gameName 退回 id（那是真实数据），
// 作者没有就不留一个空的间隔号。
function cellSubline(row) {
  const game = gameName(games.value, row.game_id)
  return row.author_name ? `${game} · ${row.author_name}` : game
}

// 走查七轮起，列表不再只返回使用中的关系，已放弃的行也在（并且可以就地恢复）。
const filteredRows = computed(() => rows.value.filter((row) => {
  if (statusFilter.value && row.video_status !== statusFilter.value) return false
  const keyword = search.value.trim()
  if (!keyword) return true
  return `${row.title || ''}${row.author_name || ''}`.includes(keyword)
}))

function reset() {
  search.value = ''
  statusFilter.value = ''
}

const pagedRows = computed(() => {
  const start = (pagination.value.current - 1) * pagination.value.pageSize
  return filteredRows.value.slice(start, start + pagination.value.pageSize)
})

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await client.listMyMaterials()
    rows.value = toRows(Array.isArray(data) ? data : [])
    pagination.value.current = 1
  } catch (e) {
    error.value = e?.message || '读取我的素材失败'
  } finally {
    loading.value = false
  }
}

async function loadGames() {
  try {
    const data = await users.listGames()
    games.value = Array.isArray(data) ? data : []
  } catch {
    games.value = []
  }
}

async function openDetail(row) {
  detailVisible.value = true
  detailLoading.value = true
  try {
    // 行里嵌的素材已经是全量了，但仍按 id 重取一次：详情要和素材库显示同一份东西，
    // 而列表里的嵌入体是「这一页加载那一刻」的副本。
    //
    // 关系那一维（使用状态、关系自己的时间）随行带过去：单条素材接口返回的是素材，
    // 它不知道「我」和这条素材是什么关系，而详情的首段与页脚都按它分支。
    detail.value = {
      ...(await client.get(row.id)),
      usage_status: row.usage_status,
      added_at: row.added_at,
    }
  } catch (e) {
    MessagePlugin.error(e?.message || '打开素材失败')
  } finally {
    detailLoading.value = false
  }
}

async function giveUp(row) {
  try {
    // 真 204：http.js 对 204 直接返回 null，所以这里没有 body 可读，也不该去读。
    await client.removeUsage(row.usage_id)
    MessagePlugin.success('已放弃使用')
    await load()
  } catch (e) {
    MessagePlugin.error(e?.message || '放弃使用失败')
  }
}

// 已放弃的关系就地恢复：行留在原地，状态从「已放弃」翻回「使用中」。
async function restore(row) {
  try {
    await client.restoreUsage(row.usage_id)
    MessagePlugin.success('已恢复使用')
    await load()
  } catch (e) {
    MessagePlugin.error(e?.message || '恢复使用失败')
  }
}

// 合成还没有后端端点（M4-C2 未启动），`/compose` 今天指向 ComingSoon。这里做的是
// **跳转**：点了会看见那一页的真实状态。发一个「已加入合成」的提示才是伪造——运营会
// 回合成页里找这条素材，而那里什么都没有。
function goToCompose() {
  router.push({ name: 'Compose' })
}

async function download(row) {
  try {
    await client.createDownload(row.id)
    MessagePlugin.success('已开始下载，可在下载中心查看进度')
    downloadCentre.open()
  } catch (e) {
    MessagePlugin.error(createDownloadFailureMessage(e))
  }
}

onMounted(() => {
  load()
  loadGames()
  // `/my-material?material_id=7` 与素材库同源（详情抽屉里的操作会带上它）。
  const raw = route.query.material_id
  if (raw && /^\d+$/.test(String(raw))) {
    detailVisible.value = true
    detailLoading.value = true
    client.get(Number(raw))
      .then((data) => { detail.value = data })
      .catch((e) => { MessagePlugin.error(e?.message || '打开素材失败'); detailVisible.value = false })
      .finally(() => { detailLoading.value = false })
  }
})
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true">
    <div class="wt-resource-page my-materials-page">
      <t-alert v-if="error" theme="error" :message="error" closable style="margin-bottom:16px" @close="error=''" />
      <ResourcePageHeader title="我的素材" description="已加入的素材，在这里下载、补充文件并进入后续生产">
        <template #actions>
          <t-button class="wt-secondary-button" variant="outline" @click="load">刷新</t-button>
        </template>
      </ResourcePageHeader>
      <ResourceCard class="material-filter-card">
        <div class="filter-row">
          <label class="filter-field"><span>综合搜索</span><t-input v-model="search" clearable placeholder="标题、作者" style="width:220px" /></label>
          <label class="filter-field"><span>文件状态</span><t-select v-model="statusFilter" clearable placeholder="全部" style="width:140px"><t-option v-for="status in VIDEO_STATUSES" :key="status" :value="status" :label="videoStatusLabel(status)" /></t-select></label>
          <t-button theme="primary" @click="load">查询</t-button>
          <t-button class="wt-secondary-button" variant="outline" @click="reset">重置</t-button>
        </div>
      </ResourceCard>

      <ResourceCard class="material-table-card">
        <div class="table-scroll-wrap">
          <t-table class="wt-resource-table" :data="pagedRows" :columns="columns" row-key="usage_id" hover size="small" :scroll="tableScroll" empty="我的素材还是空的">
            <template #id="{ row }">{{ row.id }}</template>
            <!-- 封面、标题与「游戏 · 作者」合成一格（走查七轮起副行是这两样；素材库那一页
                 的副行仍是来源平台）。标题就是去来源平台的入口，蓝色可点；没有落地页的
                 素材退回普通文本。 -->
            <template #material="{ row }">
              <div class="material-cell">
                <MaterialCover class="material-cell__cover" :url="row.cover_url" />
                <div class="material-cell__text">
                  <a v-if="row.source_url" class="wt-primary-link material-title" :href="row.source_url" target="_blank" rel="noopener noreferrer" :title="row.title || '未命名素材'">{{ row.title || '未命名素材' }}</a>
                  <span v-else class="material-title" :title="row.title || '未命名素材'">{{ row.title || '未命名素材' }}</span>
                  <span class="material-cell__source">{{ cellSubline(row) }}</span>
                </div>
              </div>
            </template>
            <template #video_status="{ row }"><ResourceStatusBadge :tone="videoStatusTone(row.video_status)" :label="videoStatusLabel(row.video_status)" /></template>
            <!-- 有值渲染值，没值画 —；两条分支不能合成一条带默认值的（缺值兜底成「使用中」
                 就是替服务端宣布一条它没说过关系）。 -->
            <template #usage_status="{ row }">
              <ResourceStatusBadge v-if="row.usage_status" :tone="usageStatusTone(row.usage_status)" :label="usageStatusLabel(row.usage_status)" />
              <template v-else>—</template>
            </template>
            <template #added_at="{ row }">{{ formatDateTime(row.added_at) }}</template>
            <!-- 走查七轮：按「使用状态 × 文件状态」分支，每行只突出一个下一步。
                 规范 §5.6 与用户同轮裁定（超过 5 个才出现「更多」）：本行最多 3 颗，
                 全部平铺。下载中不给主操作——文件已经在准备了，再点一次还是同一条命令。 -->
            <template #op="{ row }">
              <t-space class="wt-resource-actions">
                <t-button size="small" class="wt-secondary-button" variant="outline" @click="openDetail(row)">详情</t-button>
                <template v-if="row.usage_status === 'removed'">
                  <t-button size="small" theme="primary" @click="restore(row)">恢复使用</t-button>
                </template>
                <template v-else>
                  <t-button v-if="row.video_status === 'ready'" size="small" theme="primary" @click="goToCompose()">加入合成</t-button>
                  <t-button v-else-if="row.video_status !== 'downloading'" size="small" theme="primary" @click="download(row)">{{ downloadActionLabel(row.video_status) }}</t-button>
                  <t-button size="small" class="wt-secondary-button wt-danger-button" variant="outline" @click="giveUp(row)">放弃使用</t-button>
                </template>
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
        mode="mine"
        @download="download"
        @redownload="download"
        @give-up="giveUp"
        @restore="restore"
        @compose="goToCompose"
      />
    </div>
  </t-loading>
</template>

<style scoped>
.material-filter-card { padding: 14px 20px; }
.material-table-card { padding: 18px 20px; margin-top: 16px; }
.filter-row { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.filter-field { display: flex; align-items: center; gap: 8px; color: var(--wt-text-secondary); font-size: 13px; font-weight: 500; white-space: nowrap; }
/* 标题两种分支（外链 a / 普通 span）共用省略号；颜色只写在 span 分支上，
   免得覆盖 .wt-primary-link 的主题色——与内容池页同一条教训。 */
.material-cell { display: flex; align-items: center; gap: 10px; min-width: 0; }
.material-cell__cover { width: 56px; height: 36px; flex: none; }
.material-cell__text { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.material-cell__source { color: var(--wt-text-tertiary); font-size: 12px; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
.material-title { display: block; overflow: hidden; font-weight: 600; white-space: nowrap; text-overflow: ellipsis; }
span.material-title { color: var(--wt-text-primary); }
.wt-primary-link { color: var(--wt-primary); font-weight: 600; text-decoration: none; }
.wt-primary-link:hover, .wt-primary-link:focus-visible { text-decoration: underline; }
@media (max-width: 680px) {
  .material-filter-card, .material-table-card { padding: 16px; }
}
</style>
