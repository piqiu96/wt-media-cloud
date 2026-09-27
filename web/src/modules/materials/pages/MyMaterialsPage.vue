<script setup>
import { computed, onMounted, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { useRoute } from 'vue-router'
import { createMaterialsClient } from '../../../shared/api/materials.js'
import { createUsersClient } from '../../../apps/cloud/pages/users/usersApi.js'
import ResourceCard from '../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../shared/ui/resource/ResourcePageHeader.vue'
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'
import { formatDateTime } from '../../../shared/utils/datetime.js'
import { formatBytes } from '../../../shared/utils/units.js'
import { VIDEO_STATUSES, downloadHint, gameName, videoStatusLabel, videoStatusTone } from '../labels.js'
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
const search = ref('')
const statusFilter = ref('')
const games = ref([])
const detailVisible = ref(false)
const detail = ref(null)
const detailLoading = ref(false)
const pagination = ref({ current: 1, pageSize: 20 })

const columns = [
  { colKey: 'title', title: '素材', width: 340 },
  { colKey: 'source', title: '来源', width: 280 },
  { colKey: 'game', title: '游戏', width: 140 },
  { colKey: 'video_status', title: '视频状态', width: 140 },
  { colKey: 'video_size_bytes', title: '体积', width: 120 },
  { colKey: 'added_at', title: '加入时间', width: 150 },
  { colKey: 'op', title: '操作', width: 280, fixed: 'right' },
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
    added_at: usage.created_at,
  }))
}

// 列表接口只返回 active 的关系（repository 的 `WHERE user_id = ? AND status = 'active'`），
// 所以这里看不到已移出的行，也就没有「撤销移出」按钮。恢复的方式是在素材库里再点一次
// 「加入我的素材」 —— 冻结合同里没有恢复端点，那条命令本身就是幂等的创建或恢复。
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
    detail.value = await client.get(row.id)
  } catch (e) {
    MessagePlugin.error(e?.message || '打开素材失败')
  } finally {
    detailLoading.value = false
  }
}

async function remove(row) {
  try {
    // 真 204：http.js 对 204 直接返回 null，所以这里没有 body 可读，也不该去读。
    await client.removeUsage(row.usage_id)
    MessagePlugin.success('已移出我的素材')
    await load()
  } catch (e) {
    MessagePlugin.error(e?.message || '移出失败')
  }
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

async function addBack(row) {
  try {
    await client.addUsage(row.id)
    MessagePlugin.success('已加入我的素材')
    await load()
  } catch (e) {
    MessagePlugin.error(e?.message || '加入我的素材失败')
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
      <ResourcePageHeader title="我的素材" description="你收藏的素材，可随时下载到本机">
        <template #actions>
          <t-button class="wt-secondary-button" variant="outline" @click="load">刷新</t-button>
        </template>
      </ResourcePageHeader>
      <ResourceCard>
        <div class="filter-bar">
          <t-space wrap>
            <label class="wt-filter-field"><span class="wt-filter-field__label">综合搜索</span><t-input v-model="search" clearable placeholder="标题、作者" /></label>
            <label class="wt-filter-field"><span class="wt-filter-field__label">视频状态</span><t-select v-model="statusFilter" clearable placeholder="全部"><t-option v-for="status in VIDEO_STATUSES" :key="status" :value="status" :label="videoStatusLabel(status)" /></t-select></label>
            <div class="wt-filter-actions"><t-button class="wt-secondary-button" variant="outline" @click="reset">重置</t-button></div>
          </t-space>
        </div>
        <div class="table-scroll-wrap">
          <t-table class="wt-resource-table" :data="pagedRows" :columns="columns" row-key="usage_id" hover size="small" :scroll="tableScroll" empty="我的素材还是空的">
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
            <template #added_at="{ row }">{{ formatDateTime(row.added_at) }}</template>
            <template #op="{ row }">
              <t-space class="wt-resource-actions">
                <t-button size="small" class="wt-secondary-button" variant="outline" @click="openDetail(row)">查看</t-button>
                <div class="wt-row-action">
                  <t-button size="small" theme="primary" @click="download(row)">下载到本机</t-button>
                  <small v-if="downloadHint(row)" class="wt-row-hint">{{ downloadHint(row) }}</small>
                </div>
                <t-button size="small" class="wt-secondary-button wt-danger-button" variant="outline" @click="remove(row)">移出</t-button>
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
        @add="addBack"
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
