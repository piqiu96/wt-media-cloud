<script setup>
import '../../compose/production.css'
import BusinessSelect from '../../../shared/ui/fields/BusinessSelect.vue'
import MaterialMetricFilters from '../components/MaterialMetricFilters.vue'
import { publishedRangeParams, emptyMetricRanges, metricRangeParams, metricLabel, materialVideoState } from '../materialFilters.js'
import { opColumnWidth } from '../../../shared/ui/resource/opColumnWidth.js'
import ResourceAdvancedToggle from '../../../shared/ui/resource/ResourceAdvancedToggle.vue'
import ResourceRowActions from '../../../shared/ui/resource/ResourceRowActions.vue'
import ResourceSelectionToolbar from '../../../shared/ui/resource/ResourceSelectionToolbar.vue'
import { feedback } from '../../../shared/ui/feedback/index.js'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { createMaterialsClient } from '../../../shared/api/materials.js'
import { createUsersClient } from '../../../apps/cloud/pages/users/usersApi.js'
import ResourceCard from '../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../shared/ui/resource/ResourcePageHeader.vue'
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'
import { formatDateTime } from '../../../shared/utils/datetime.js'
import { useAutoRefresh } from '../../../shared/utils/useAutoRefresh.js'
import {
  VIDEO_STATUSES,
  MY_USAGE_FILTERS, TEAM_USAGE_FILTERS, myUsageLabel, usageSummary,
  downloadActionLabel,
  categoryName,
  usageStatusTone,
  videoStatusLabel,
  videoStatusTone,
} from '../labels.js'
import { localDownloadReason } from '../../transfer/localDownloadEligibility.js'
import { createSessionClient } from '../../../shared/api/session.js'
import MaterialDetailDrawer from '../MaterialDetailDrawer.vue'
import MaterialCover from '../components/MaterialCover.vue'
import { createDownloadFailureMessage } from '../../transfer/downloadErrors.js'
import { useDownloadCentre } from '../../transfer/downloadCentre.js'
import { createFileTransferClient } from '../../../shared/api/fileTransfer.js'
import { downloadedFileName, filePresence, needsRedownload } from '../../transfer/downloadFacts.js'
import { savedFileStates } from '../../transfer/desktopBridge.js'
import { canSelectForDownload, submitMaterialDownloads } from './myMaterialsBatch.js'

const currentUser = ref(null)
const downloadReason = computed(() => localDownloadReason(currentUser.value))
const client = createMaterialsClient()
const users = createUsersClient()
const transferClient = createFileTransferClient()
const route = useRoute()
const downloadCentre = useDownloadCentre()

const rows = ref([])
const total = ref(0)
let loadSequence = 0
const loading = ref(false)
const error = ref('')
const search = ref('')
const statusFilter = ref('')
const advancedFilters = ref(false), categoryFilters = ref([]), publishedRange = ref([]), metricFilters = ref(emptyMetricRanges())
const myUsageFilter = ref('unused'), teamUsageFilter = ref(''), usageStatusFilter = ref('active')
const sortBy = ref('published_at')
const sortOrder = ref('desc')
const categories = ref([])
const detailVisible = ref(false)
const detail = ref(null)
const detailLoading = ref(false)
const pagination = ref({ current: 1, pageSize: 20 })
const selectedRowKeys = ref([])
const batchLoading = ref(false)

// Selection is keyed by the personal relation; all asset and usage facts remain separate.
const columns = [
  { colKey: 'row-select', type: 'multiple', width: 44, checkProps: ({ row }) => ({ disabled: Boolean(downloadReason.value) || !canSelectForDownload(row) || batchLoading.value }) },
  { colKey: 'id', title: '素材 ID', width: 80, sorter: true },
  { colKey: 'material', title: '素材 / 作者', width: 280 },
  { colKey: 'category', title: '类目', width: 100 },
  { colKey: 'published_at', title: '发布时间', width: 155, sorter: true },
  { colKey: 'metrics', title: '浏览 / 赞 / 收藏', width: 175 },
  { colKey: 'my_usage', title: '我的使用', width: 100 },
  { colKey: 'team_usage', title: '团队使用', width: 150 },
  { colKey: 'video_status', title: '视频状态', width: 100 },
  { colKey: 'usage_status', title: '加入状态', width: 100 },
  { colKey: 'added_at', title: '加入时间', width: 155, sorter: true },
  { colKey: 'op', title: '操作', width: opColumnWidth(['详情','重新下载','放弃使用'], {variant:'text',primary:true}), fixed: 'right' },
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
    // 下载生命周期是**这条关系**的（按用户派生），不是素材的：同一行素材对两个
    // 下载过不同次的人要显示不同状态。没下过时服务端不返回该键，这里记空串。
    download_status: usage.download_status || '',
    added_at: usage.created_at,
  }))
}

function countLabel(value) { return value === null || value === undefined ? '—' : Number(value).toLocaleString() }

function fileStatus(row) {
  return materialVideoState(row, localPresence(row))
}

// Category has its own column; the material subline shows the author.
function cellSubline(row) {
  return row.author_name || '—'
}

// 「本机那一份文件还在不在」，按任务表里的文件名索引（同 MaterialDetailDrawer 与下载中心）。
// 空表 ＝ 没查过：浏览器读不到本机、Desktop 还没扫完 —— 那时一个字都不说，绝不把
// 「我读不到」写成「文件不在」。
const tasks = ref([])
const presence = ref({})
// 这张 presence 表是为哪个文件名集合量出来的。集合没变就不问第二遍。
let measuredKey = null

/**
 * 只有**已下载**的行才需要这一问：名字从下载任务表里挑（`downloadedFileName` 只认本机、
 * `user_download`、成功过、报了文件名的那一条），在不在由 Desktop 在已知保存位置里量。
 * 没有已下载的行时连任务表都不读 —— 这一页平时不发多余请求。
 */
async function loadPresence() {
  const downloaded = rows.value.filter((row) => row.download_status === 'downloaded')
  if (downloaded.length === 0) {
    measuredKey = null
    tasks.value = []
    presence.value = {}
    return
  }
  try {
    const data = await transferClient.listTasks({ limit: 200 })
    tasks.value = Array.isArray(data) ? data : []
  } catch {
    tasks.value = []
  }
  const names = downloaded
    .map((row) => downloadedFileName(tasks.value, row.id))
    .filter((name) => typeof name === 'string' && name.trim())
  const key = names.slice().sort().join('\n')
  if (key === measuredKey) return
  measuredKey = key
  if (!key) {
    presence.value = {}
    return
  }
  try {
    presence.value = await savedFileStates(names)
  } catch {
    // 问不到就退回「没查过」：界面上与不查完全一样（那颗「重新下载」不出现），不打扰运营。
    presence.value = {}
  }
}

function localPresence(row) {
  const name = downloadedFileName(tasks.value, row.id)
  return filePresence(name ? { file_name: name } : null, presence.value)
}

// 走查七轮起，列表不再只返回使用中的关系，已放弃的行也在（并且可以就地恢复）。
function reset() {
  myUsageFilter.value = 'unused'
  teamUsageFilter.value = ''
  usageStatusFilter.value = 'active'
  categoryFilters.value = (currentUser.value?.category_ids || []).filter(id => categories.value.some(row => row.id === id))
  publishedRange.value = []
  metricFilters.value = emptyMetricRanges()
  search.value = ''
  statusFilter.value = ''
  load()
}

watch([search, statusFilter], () => {
  pagination.value.current = 1
  selectedRowKeys.value = []
})

const pagedRows = computed(() => rows.value)

// quiet 由定时刷新传入：保留页码与筛选，失败沿用上一次读数。
async function load({ quiet = false, keepPage = false } = {}) {
  if (!quiet && !keepPage) pagination.value.current = 1
  const sequence = ++loadSequence
  if (!quiet) loading.value = true
  if (!quiet) error.value = ''
  try {
    const data = await client.listMyMaterials({ category_ids: categoryFilters.value.join(','), ...publishedRangeParams(publishedRange.value), ...metricRangeParams(metricFilters.value), my_usage: myUsageFilter.value, team_usage: teamUsageFilter.value, usage_status: usageStatusFilter.value, page: pagination.value.current, page_size: pagination.value.pageSize, search: search.value.trim(), video_status: statusFilter.value, sort_by: sortBy.value, sort_order: sortOrder.value })
    if (sequence !== loadSequence) return
    rows.value = toRows(data.items || [])
    total.value = data.total || 0
    selectedRowKeys.value = selectedRowKeys.value.filter((key) => rows.value.some((row) => String(row.usage_id) === String(key) && canSelectForDownload(row)))
  } catch (e) {
    if (!quiet && sequence === loadSequence) error.value = e?.message || '读取我的素材失败'
  } finally {
    if (!quiet && sequence === loadSequence) loading.value = false
  }
  await loadPresence()
}

useAutoRefresh(() => load({ quiet: true }), {
  shouldSkip: () => detailVisible.value || detailLoading.value || selectedRowKeys.value.length > 0 || batchLoading.value,
})

function changeSort(sort) {
  sortBy.value = sort?.sortBy || 'published_at'
  sortOrder.value = sort?.descending === false ? 'asc' : 'desc'
  load()
}

async function loadCategories() {
  try {
    const data = await users.listCategories()
    categories.value = Array.isArray(data) ? data : []
  } catch {
    categories.value = []
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
    feedback.toast.error(e?.message || '打开素材失败')
  } finally {
    detailLoading.value = false
  }
}

async function giveUp(row) {
  try {
    // 真 204：http.js 对 204 直接返回 null，所以这里没有 body 可读，也不该去读。
    await client.removeUsage(row.usage_id)
    feedback.toast.success('已放弃使用')
    await load()
  } catch (e) {
    feedback.toast.error(e?.message || '放弃使用失败')
  }
}

// 已放弃的关系就地恢复：行留在原地，状态从「已放弃」翻回「使用中」。
async function restore(row) {
  try {
    await client.restoreUsage(row.usage_id)
    feedback.toast.success('已恢复使用')
    await load()
  } catch (e) {
    feedback.toast.error(e?.message || '恢复使用失败')
  }
}

async function download(row) {
  if (downloadReason.value) return
  try {
    await client.createDownload(row.id)
    feedback.toast.success('已开始下载，可在下载中心查看进度')
    // 重读一次：这一行的「文件状态」和按钮都按使用记录算，不重读就停在上一帧，看起来像没反应。
    await load()
    downloadCentre.open()
  } catch (e) {
    feedback.toast.error(createDownloadFailureMessage(e))
  }
}

async function batchDownload() {
  if (downloadReason.value) return
  if (!selectedRowKeys.value.length || batchLoading.value) return
  batchLoading.value = true
  try {
    const result = await submitMaterialDownloads(rows.value, selectedRowKeys.value, (materialId) => client.createDownload(materialId))
    selectedRowKeys.value = []
    if (result.submitted) {
      feedback.toast.success(`已提交 ${result.submitted} 个下载任务，可在下载中心查看进度`)
      downloadCentre.open()
    }
    if (result.failures.length || result.skipped) {
      const reason = result.failures[0] ? `；首个失败：${createDownloadFailureMessage(result.failures[0].error)}` : ''
      feedback.toast.error(`批量下载共 ${result.total} 项，提交 ${result.submitted} 项，失败 ${result.failures.length} 项，跳过 ${result.skipped} 项${reason}`)
    }
    await load()
  } finally {
    batchLoading.value = false
  }
}

onMounted(() => {
  Promise.all([createSessionClient().me().then(user => { currentUser.value = user }).catch(() => {}), loadCategories()]).then(() => { categoryFilters.value = (currentUser.value?.category_ids || []).filter(id => categories.value.some(row => row.id === id)); load() })
  // `/my-material?material_id=7` 与素材库同源（详情抽屉里的操作会带上它）。
  const raw = route.query.material_id
  if (raw && /^\d+$/.test(String(raw))) {
    detailVisible.value = true
    detailLoading.value = true
    client.get(Number(raw))
      .then((data) => { detail.value = data })
      .catch((e) => { feedback.toast.error(e?.message || '打开素材失败'); detailVisible.value = false })
      .finally(() => { detailLoading.value = false })
  }
})
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true">
    <div class="wt-resource-page resource-visual-approved production-page my-materials-page">
      <FeedbackAlert v-if="error" theme="error" :message="error" closable style="margin-bottom:16px" @close="error=''" />
      <ResourcePageHeader title="我的素材" description="管理已加入的素材，查看使用情况并下载。" />
      <ResourceCard class="material-table-card">
        <div class="filter-bar">
          <label class="wt-filter-field wt-filter-field--wide"><span class="wt-filter-field__label">综合搜索</span><t-input v-model="search" clearable placeholder="名称 / 作者 / ID" @enter="load" /></label>
          <label class="wt-filter-field"><span class="wt-filter-field__label">类目</span><BusinessSelect kind="category" filter v-model="categoryFilters" multiple clearable><t-option v-for="category in categories" :key="category.id" :value="category.id" :label="category.name" /></BusinessSelect></label>
          <label class="wt-filter-field"><span class="wt-filter-field__label">我的使用</span><t-select v-model="myUsageFilter" :options="MY_USAGE_FILTERS" clearable placeholder="全部" /></label>
          <div class="wt-filter-actions"><t-button theme="primary" @click="load">查询</t-button><t-button class="wt-secondary-button" variant="outline" @click="reset">重置</t-button></div>
          <ResourceAdvancedToggle v-model="advancedFilters" />
          <div v-if="advancedFilters" class="wt-advanced-filters">
            <label class="wt-filter-field compose-date-filter"><span class="wt-filter-field__label">发布时间</span><t-date-range-picker v-model="publishedRange" value-type="YYYY-MM-DD" clearable /></label>
            <label class="wt-filter-field"><span class="wt-filter-field__label">团队使用</span><t-select v-model="teamUsageFilter" :options="TEAM_USAGE_FILTERS" clearable placeholder="全部" /></label>
            <label class="wt-filter-field"><span class="wt-filter-field__label">加入状态</span><t-select v-model="usageStatusFilter" :options="[{value:'active',label:'已加入'},{value:'removed',label:'已移出'}]" clearable placeholder="全部" /></label>
            <label class="wt-filter-field"><span class="wt-filter-field__label">视频准备</span><t-select v-model="statusFilter" clearable placeholder="全部"><t-option v-for="status in VIDEO_STATUSES" :key="status" :value="status" :label="videoStatusLabel(status)" /></t-select></label>
            <MaterialMetricFilters v-model="metricFilters" />
          </div>
        </div>
        <ResourceSelectionToolbar :count="selectedRowKeys.length" unit="条素材" @clear="selectedRowKeys=[]"><t-button variant="outline" :loading="batchLoading" :disabled="Boolean(downloadReason)" :title="downloadReason" @click="batchDownload">下载（{{ selectedRowKeys.length }}）</t-button></ResourceSelectionToolbar>
        <p v-if="downloadReason" class="wt-download-reason">{{ downloadReason }}</p>
        <div class="table-scroll-wrap">
          <t-table class="wt-resource-table" :data="pagedRows" :columns="columns" row-key="usage_id" hover size="small" :scroll="tableScroll" :sort="{ sortBy, descending: sortOrder === 'desc' }" @sort-change="changeSort" v-model:selected-row-keys="selectedRowKeys" empty="我的素材还是空的">
            <template #id="{ row }">{{ row.id }}</template>
            <!-- 封面、标题与「类目 · 作者」合成一格（走查七轮起副行是这两样；素材库那一页
                 的副行仍是来源平台）。标题就是去来源平台的入口，蓝色可点；没有落地页的
                 素材退回普通文本。 -->
            <template #material="{ row }">
              <div class="material-cell">
                <MaterialCover class="material-cell__cover" :url="row.cover_url" />
                <div class="material-cell__text">
                  <t-link theme="primary" class="material-title" :title="row.title || '未命名素材'" @click="openDetail(row)">{{ row.title || '未命名素材' }}</t-link>
                  <span class="material-cell__source">{{ cellSubline(row) }}</span>
                </div>
              </div>
            </template>
            <template #category="{row}">{{ categoryName(categories,row.category_id) }}</template>
            <template #my_usage="{row}">{{ myUsageLabel(row.my_usage) }}</template>
            <template #team_usage="{row}">{{ usageSummary(row) }}</template>
            <template #metrics="{row}"><div class="material-metrics"><span>{{ metricLabel(row,'view_count') }} 浏览</span><span>{{ metricLabel(row,'like_count') }} 赞 · {{ metricLabel(row,'favorite_count') }} 收藏</span></div></template>
            <template #like_count="{ row }">{{ countLabel(row.like_count) }}</template>
            <template #favorite_count="{ row }">{{ countLabel(row.favorite_count) }}</template>
            <template #video_status="{ row }"><ResourceStatusBadge :tone="fileStatus(row).tone" :label="fileStatus(row).label" /></template>
            <!-- 有值渲染值，没值画 —；两条分支不能合成一条带默认值的（缺值兜底成「使用中」
                 就是替服务端宣布一条它没说过关系）。 -->
            <template #usage_status="{ row }">
              <ResourceStatusBadge v-if="row.usage_status" :tone="usageStatusTone(row.usage_status)" :label="row.usage_status === 'active' ? '已加入' : '已移出'" />
              <template v-else>—</template>
            </template>
            <template #added_at="{ row }">{{ formatDateTime(row.added_at) }}</template>
            <template #published_at="{ row }">{{ formatDateTime(row.published_at) }}</template>
            <!-- CHG-069 任务 23：按钮由「使用状态 × 下载状态」矩阵驱动（任务状态与业务状态
                 严格隔离），每行最多 3 颗全部平铺（规范 §5.6）。已放弃只给恢复；使用中按
                 下载状态给一步主操作——下载中不给（文件已经在准备了，再点还是同一条命令）。
                 已下载是终态：本机那份文件还在就无事可做，**实测不在了**才给一次「重新下载」
                 （合成的入口在合成任务页，这一页不再有合成按钮）。 -->
            <template #op="{ row }">
              <ResourceRowActions>
                <t-button size="small" class="wt-secondary-button" variant="text" @click="openDetail(row)">详情</t-button>
                <template v-if="row.usage_status === 'removed'">
                  <t-button size="small" theme="primary" variant="text" class="is-primary" @click="restore(row)">恢复使用</t-button>
                </template>
                <template v-else>
                  <t-button v-if="needsRedownload(row.download_status, localPresence(row))" size="small" theme="primary" variant="text" class="is-primary" :disabled="Boolean(downloadReason)" :title="downloadReason" @click="download(row)">重新下载</t-button>
                  <t-button v-else-if="row.download_status !== 'downloading' && row.download_status !== 'downloaded'" size="small" theme="primary" variant="text" class="is-primary" :disabled="Boolean(downloadReason)" :title="downloadReason" @click="download(row)">{{ downloadActionLabel(row.download_status) }}</t-button>
                  <t-button size="small" class="wt-secondary-button wt-danger-button" variant="text" @click="giveUp(row)">放弃使用</t-button>
                </template>
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
        mode="mine"
        @download="download"
        @redownload="download"
        @cancel="load"
        @give-up="giveUp"
        @restore="restore"
      />
    </div>
  </t-loading>
</template>

<style scoped>
.wt-download-reason { margin: 12px 16px; font-size: 13px; color: var(--wt-text-secondary); }
.material-table-card { padding: 18px 20px; margin-top: 16px; }
/* 标题两种分支（外链 a / 普通 span）共用省略号；颜色只写在 span 分支上，
   免得覆盖 .wt-primary-link 的主题色——与内容池页同一条教训。 */
.material-metrics { display:grid;gap:4px; }
.material-metrics span+span { font-size:12px;color:var(--wt-text-tertiary); }
.material-cell { display: flex; align-items: center; gap: 10px; min-width: 0; }
.material-cell__cover { width: 42px; height: 52px; flex: none; }
.material-cell__text { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.material-cell__source { color: var(--wt-text-tertiary); font-size: 12px; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
.material-title { display: block; overflow: hidden; font-weight: 600; white-space: nowrap; text-overflow: ellipsis; }
span.material-title { color: var(--wt-text-primary); }
.wt-primary-link { color: var(--wt-primary); font-weight: 600; text-decoration: none; }
.wt-primary-link:hover, .wt-primary-link:focus-visible { text-decoration: underline; }
@media (max-width: 680px) {
  .material-table-card { padding: 16px; }
}
</style>
