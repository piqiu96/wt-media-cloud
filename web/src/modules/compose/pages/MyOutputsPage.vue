<script setup>
import { feedback } from '../../../shared/ui/feedback/index.js'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import { computed, onMounted, ref } from 'vue'
import { DialogPlugin } from 'tdesign-vue-next'
import { useRoute, useRouter } from 'vue-router'
import { createComposeClient } from '../../../shared/api/compose.js'
import ResourceCard from '../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../shared/ui/resource/ResourcePageHeader.vue'
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'
import { opColumnWidth } from '../../../shared/ui/resource/opColumnWidth.js'
import { formatDateTime } from '../../../shared/utils/datetime.js'
import { createdRangeParams } from '../createdRange.js'
import { formatBytes, formatDurationMs } from '../../../shared/utils/units.js'
import ComposeOutputDetail from '../components/ComposeOutputDetail.vue'
import { useDownloadCentre } from '../../transfer/downloadCentre.js'
import { createDownloadFailureMessage } from '../../transfer/downloadErrors.js'

// 发布状态由服务端筛选；下载、预览和归档不改变发布状态。
const client = createComposeClient()
const router = useRouter()
const route = useRoute()
const downloadCentre = useDownloadCentre()
const downloadingID = ref(null)

const rows = ref([])
const strategies = ref([])
const loading = ref(false)
const error = ref('')
const filters = ref({ publish_status: '', source: '', created_range: [] })
const pagination = ref({ current: 1, pageSize: 20 })

const statusLabels = { pending_publish: '待发布', published: '已发布', archived: '已归档' }
const statusTones = { pending_publish: 'warning', published: 'success', archived: 'neutral' }

const columns = computed(() => [
  { colKey: 'id', title: '成片 ID', width: 90 },
  { colKey: 'publish_status', title: '状态', width: 100 },
  { colKey: 'material_id', title: '来源素材', width: 130 },
  { colKey: 'strategy', title: '生产策略', width: 180 },
  { colKey: 'duration', title: '时长', width: 80 },
  { colKey: 'size', title: '大小', width: 90 },
  { colKey: 'created_at', title: '生成时间', width: 150 },
  { colKey: 'op', title: '操作', width: opColumnWidth(['下载', '详情', '去发布', '归档'], { loading: true }), fixed: 'right' },
])

// 横向滚动宽度由列宽推导，不写死。
const tableScroll = computed(() => ({
  x: `${columns.value.reduce((sum, col) => sum + (col.width || 0), 0)}px`,
}))

const pagedRows = computed(() => {
  const start = (pagination.value.current - 1) * pagination.value.pageSize
  return rows.value.slice(start, start + pagination.value.pageSize)
})

const statusOptions = Object.entries(statusLabels).map(([value, label]) => ({ value, label }))
const visibleCount = computed(() => rows.value.length)
const pendingCount = computed(() => rows.value.filter((row) => row.publish_status === 'pending_publish').length)
const archivedCount = computed(() => rows.value.filter((row) => row.publish_status === 'archived').length)

function strategyName(row) {
  const found = strategies.value.find((item) => item.id === row.compose_strategy_id)
  return found ? found.name : `策略 ${row.compose_strategy_id}`
}

function templateText(row) {
  return `${row.template_code} v${row.template_version}`
}

// media 是执行时量到的文件事实（时长等）；解析不动就当没有，不替服务端编数。
function durationText(row) { return formatDurationMs(row.media?.duration_ms) }

async function load() {
  loading.value = true
  error.value = ''
  try {
    const params = {}
    if (filters.value.publish_status) params.publish_status = filters.value.publish_status
    if (filters.value.source.trim()) params.source = filters.value.source.trim()
    Object.assign(params, createdRangeParams(filters.value.created_range))
    const data = await client.listComposeOutputs(params)
    rows.value = Array.isArray(data) ? data : []
    pagination.value.current = 1
  } catch (e) {
    error.value = e?.message || '读取成片列表失败'
  } finally {
    loading.value = false
  }
}

function reset() {
  filters.value = { publish_status: '', source: '', created_range: [] }
  load()
}

async function loadStrategies() {
  try {
    const data = await client.listStrategies()
    strategies.value = Array.isArray(data) ? data : []
  } catch {
    strategies.value = []
  }
}

const detailVisible = ref(false)
const detail = ref(null)
const detailTab = ref('basic')
const previewUrl = ref('')
const previewLoading = ref(false)

async function openDetail(row) {
  detailVisible.value = true
  detailTab.value = 'basic'
  detail.value = null
  previewUrl.value = ''
  try {
    // 详情按 id 重取：行是列表加载那一刻的副本，追踪信息要库里现在的那份。
    // 预览地址跟着这一次打开现取——签名短时会过期，不缓存也不复用上一次的。
    // 地址取失败不挡详情：预览区给一句错误，来源追踪照常看。
    const [data, link] = await Promise.all([
      client.getComposeOutput(row.id),
      client.getComposeOutputVideoUrl(row.id).catch(() => null),
    ])
    detail.value = data
    previewUrl.value = link?.url || ''
    if (!previewUrl.value) feedback.toast.error('预览地址获取失败，可在预览区重试')
  } catch (e) {
    feedback.toast.error(e?.message || '打开成片失败')
    detailVisible.value = false
  }
}

async function retryPreview() {
  if (!detail.value || previewLoading.value) return
  previewLoading.value = true
  try {
    const link = await client.getComposeOutputVideoUrl(detail.value.output.id)
    previewUrl.value = link?.url || ''
    if (!previewUrl.value) feedback.toast.error('预览地址获取失败，请稍后重试')
  } catch (e) {
    feedback.toast.error(e?.message || '预览地址获取失败，请稍后重试')
  } finally {
    previewLoading.value = false
  }
}

function archive(row) {
  const dialog = DialogPlugin.confirm({
    header: '归档成片',
    body: `确认归档成片 ${row.id}？归档后不再出现在待发布列表，且不可恢复。`,
    confirmBtn: { content: '归档', theme: 'danger' },
    onConfirm: async () => {
      dialog.destroy()
      try {
        await client.archiveComposeOutput(row.id)
        feedback.toast.success('已归档')
      } catch (e) {
        feedback.toast.error(e?.message || '归档失败')
      }
      // 成败都重读：409（已发布/已归档）也只有重读才看得出它现在是什么状态。
      await load()
    },
  })
}

// 「去发布」是衔接动作：把成片信息带进发布管理，不在这里创建发布记录、不翻状态。
function publish(row) {
  const dialog = DialogPlugin.confirm({
    header: '去发布',
    body: `成片 ${row.id}（素材 ${row.material_id}）将带入发布流程。发布管理上线前不会记录任何发布结果。`,
    confirmBtn: { content: '进入发布管理', theme: 'primary' },
    onConfirm: () => {
      dialog.destroy()
      router.push({ name: 'Publish', query: { output_id: String(row.id) } })
    },
  })
}

async function download(row) {
  if (downloadingID.value !== null) return
  downloadingID.value = row.id
  try {
    await client.createOutputDownload(row.id)
    feedback.toast.info('下载任务已提交，可在下载中心查看进度')
    downloadCentre.open()
  } catch (e) {
    feedback.toast.error(createDownloadFailureMessage(e))
  } finally {
    downloadingID.value = null
  }
}

onMounted(() => {
  load()
  loadStrategies()
  const id = String(route.query.output_id || '')
  if (/^\d+$/.test(id)) openDetail({ id: Number(id) })
})
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true">
    <div class="wt-resource-page my-outputs-page">
      <FeedbackAlert v-if="error" theme="error" :message="error" closable style="margin-bottom:16px" @close="error=''" />
      <ResourcePageHeader title="我的成片" description="合成完成的成片，在这里下载、预览、追溯来源并归档或去发布">
        <template #actions>
          <t-button class="wt-secondary-button" variant="outline" @click="load">刷新</t-button>
        </template>
      </ResourcePageHeader>
      <ResourceCard class="my-outputs-card">
        <div class="filter-bar">
          <label class="wt-filter-field"><span class="wt-filter-field__label">状态</span><t-select v-model="filters.publish_status" :options="statusOptions" clearable placeholder="全部" /></label>
          <label class="wt-filter-field wt-filter-field--wide"><span class="wt-filter-field__label">来源素材</span><t-input v-model="filters.source" :maxlength="100" clearable placeholder="素材 ID 或名称" /></label>
          <label class="wt-filter-field compose-date-filter"><span class="wt-filter-field__label">生成时间</span><t-date-range-picker v-model="filters.created_range" value-type="YYYY-MM-DD" clearable /></label>
          <div class="wt-filter-actions">
            <t-button theme="primary" @click="load">查询</t-button>
            <t-button class="wt-secondary-button" variant="outline" @click="reset">重置</t-button>
          </div>
        </div>
        <div class="output-list-summary" aria-live="polite">
          <span>当前结果 <strong>{{ visibleCount }}</strong> 条</span>
          <span>待发布 <strong>{{ pendingCount }}</strong></span>
          <span>已归档 <strong>{{ archivedCount }}</strong></span>
        </div>
        <div class="table-scroll-wrap">
          <t-table class="wt-resource-table" :data="pagedRows" :columns="columns" row-key="id" hover size="small" :scroll="tableScroll" empty="还没有成片">
            <template #id="{ row }"><span class="output-id">{{ row.id }}</span></template>
            <template #material_id="{ row }"><span class="output-source-ref">素材 {{ row.material_id }}</span></template>
            <template #strategy="{ row }">
              <div class="output-related-cell">
                <span class="output-related-cell__main" :title="strategyName(row)">{{ strategyName(row) }}</span>
                <span class="output-related-cell__sub">{{ templateText(row) }}</span>
              </div>
            </template>
            <template #duration="{ row }">{{ durationText(row) }}</template>
            <template #size="{ row }">{{ formatBytes(row.file_size_bytes) }}</template>
            <template #publish_status="{ row }"><ResourceStatusBadge :tone="statusTones[row.publish_status] || 'default'" :label="statusLabels[row.publish_status] || row.publish_status" /></template>
            <template #created_at="{ row }">{{ formatDateTime(row.created_at) }}</template>
            <template #op="{ row }">
              <t-space class="wt-resource-actions">
                <t-button size="small" theme="primary" :loading="downloadingID === row.id" @click="download(row)">下载</t-button>
                <t-button size="small" class="wt-secondary-button" variant="outline" @click="openDetail(row)">详情</t-button>
                <template v-if="row.publish_status === 'pending_publish'">
                  <t-button size="small" class="wt-secondary-button" variant="outline" @click="publish(row)">去发布</t-button>
                  <t-button size="small" class="wt-secondary-button wt-danger-button" variant="outline" @click="archive(row)">归档</t-button>
                </template>
              </t-space>
            </template>
          </t-table>
        </div>
        <div class="pagination-bar"><t-pagination v-model:current="pagination.current" v-model:pageSize="pagination.pageSize" :total="rows.length" :page-size-options="[10, 20, 50]" /></div>
      </ResourceCard>

      <t-drawer :close-btn="true" v-model:visible="detailVisible" header="成片详情" size="680px">
        <template #footer>
          <t-space v-if="detail" class="wt-resource-actions">
            <t-button theme="primary" :loading="downloadingID === detail.output.id" @click="download(detail.output)">下载</t-button>
            <template v-if="detail.output.publish_status === 'pending_publish'">
              <t-button class="wt-secondary-button" variant="outline" @click="publish(detail.output)">去发布</t-button>
              <t-button class="wt-secondary-button wt-danger-button" variant="outline" @click="archive(detail.output)">归档</t-button>
            </template>
          </t-space>
        </template>
        <t-loading :loading="!detail" :show-overlay="true">
          <div v-if="detail" class="output-detail">
            <div class="output-detail__hero">
              <div><h3>成片 {{ detail.output.id }}</h3><p>生成于 {{ formatDateTime(detail.output.created_at) }}</p></div>
              <ResourceStatusBadge :tone="statusTones[detail.output.publish_status] || 'neutral'" :label="statusLabels[detail.output.publish_status] || detail.output.publish_status" />
            </div>
            <ComposeOutputDetail v-model:tab="detailTab" :detail="detail" :preview-url="previewUrl" :preview-loading="previewLoading" @retry-preview="retryPreview" />
          </div>
        </t-loading>
      </t-drawer>
    </div>
  </t-loading>
</template>

<style scoped>
.my-outputs-card { padding: 14px 20px 18px; }
.filter-bar > .compose-date-filter { flex-basis: 300px; }
.compose-date-filter :deep(.t-date-range-picker) { min-width: 0; width: 100%; }
.output-list-summary { display: flex; gap: 20px; align-items: center; padding: 0 0 12px; color: var(--wt-text-tertiary); font-size: 12px; }
.output-list-summary strong { color: var(--wt-text-primary); font-size: 14px; font-weight: 600; }
.output-id { color: var(--wt-text-primary); font-weight: 600; font-variant-numeric: tabular-nums; }
.output-source-ref { color: var(--wt-text-secondary); font-weight: 500; }
.output-related-cell { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.output-related-cell__main { overflow: hidden; color: var(--wt-text-primary); font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }
.output-related-cell__sub { color: var(--wt-text-tertiary); font-size: 12px; }
.output-detail__hero { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; padding: 4px 0 20px; }
.output-detail__hero h3 { margin: 4px 0; color: var(--wt-text-primary); font-size: 20px; }
.output-detail__hero p { margin: 0; color: var(--wt-text-tertiary); font-size: 12px; }
</style>
