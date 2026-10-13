<script setup>
import { feedback } from '../../../shared/ui/feedback/index.js'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import { computed, onMounted, ref } from 'vue'
import { DialogPlugin } from 'tdesign-vue-next'
import { useRouter } from 'vue-router'
import { createComposeClient } from '../../../shared/api/compose.js'
import ResourceCard from '../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../shared/ui/resource/ResourcePageHeader.vue'
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'
import { opColumnWidth } from '../../../shared/ui/resource/opColumnWidth.js'
import { formatDateTime } from '../../../shared/utils/datetime.js'
import { formatBytes } from '../../../shared/utils/units.js'
import { usageStatusLabel } from '../../materials/labels.js'
import { useDownloadCentre } from '../../transfer/downloadCentre.js'
import { createDownloadFailureMessage } from '../../transfer/downloadErrors.js'

// 发布状态由服务端筛选；下载、预览和归档不改变发布状态。
const client = createComposeClient()
const router = useRouter()
const downloadCentre = useDownloadCentre()
const downloadingID = ref(null)

const rows = ref([])
const strategies = ref([])
const loading = ref(false)
const error = ref('')
const filters = ref({ publish_status: '' })
const pagination = ref({ current: 1, pageSize: 20 })

const statusLabels = { pending_publish: '待发布', published: '已发布', archived: '已归档' }
const statusTones = { pending_publish: 'warning', published: 'success', archived: 'neutral' }
// 任务状态是来源追踪里的投影，词表与合成任务页一致。
const taskStatusLabels = { pending: '排队中', running: '执行中', success: '已完成', failed: '失败', cancelled: '已取消' }

// 每一列显式 width（规范 §6.2，WebKit fixed 布局下 minWidth 不产生确定列宽）。
// 操作列宽按待发布行的按钮清单推导（常驻 下载/详情，待发布多 去发布/归档；
// 下载 loading 时多一个 spinner），非待发布行只是留白更多。
const columns = computed(() => [
  { colKey: 'id', title: '成片 ID', width: 90 },
  { colKey: 'material_id', title: '素材 ID', width: 90 },
  { colKey: 'strategy', title: '策略', width: 180 },
  { colKey: 'template', title: '模板', width: 140 },
  { colKey: 'size', title: '大小', width: 100 },
  { colKey: 'publish_status', title: '状态', width: 100 },
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

function strategyName(row) {
  const found = strategies.value.find((item) => item.id === row.compose_strategy_id)
  return found ? found.name : `#${row.compose_strategy_id}`
}

function templateText(row) {
  return `${row.template_code} v${row.template_version}`
}

// media 是执行时量到的文件事实（时长等）；解析不动就当没有，不替服务端编数。
function durationText(row) {
  const ms = row.media?.duration_ms
  if (!Number.isFinite(ms) || ms <= 0) return '—'
  return `${(ms / 1000).toFixed(1)} 秒`
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const params = {}
    if (filters.value.publish_status) params.publish_status = filters.value.publish_status
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
  filters.value = { publish_status: '' }
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
const previewUrl = ref('')

async function openDetail(row) {
  detailVisible.value = true
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
    if (!link) feedback.toast.warning('预览地址获取失败，可稍后重开详情')
  } catch (e) {
    feedback.toast.error(e?.message || '打开成片失败')
    detailVisible.value = false
  }
}

function archive(row) {
  const dialog = DialogPlugin.confirm({
    header: '归档成片',
    body: `确认归档成片 #${row.id}？归档后不再出现在待发布列表，且不可恢复。`,
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
    body: `成片 #${row.id}（素材 #${row.material_id}）将带入发布流程。发布管理上线前不会记录任何发布结果。`,
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
          <div class="wt-filter-actions">
            <t-button theme="primary" @click="load">查询</t-button>
            <t-button class="wt-secondary-button" variant="outline" @click="reset">重置</t-button>
          </div>
        </div>
        <div class="table-scroll-wrap">
          <t-table class="wt-resource-table" :data="pagedRows" :columns="columns" row-key="id" hover size="small" :scroll="tableScroll" empty="还没有成片">
            <template #strategy="{ row }">{{ strategyName(row) }}</template>
            <template #template="{ row }">{{ templateText(row) }}</template>
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

      <t-drawer :close-btn="true" v-model:visible="detailVisible" header="成片详情" size="560px">
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
            <h4 class="output-detail__title">预览</h4>
            <video v-if="previewUrl" :src="previewUrl" controls preload="metadata" class="output-detail__video">当前环境不支持视频预览</video>
            <p v-else class="output-detail__hint">预览地址获取失败，可关闭后重开详情</p>

            <h4 class="output-detail__title">文件信息</h4>
            <div class="output-detail__grid">
              <div><span class="output-detail__label">成片 ID</span><span>{{ detail.output.id }}</span></div>
              <div><span class="output-detail__label">状态</span><ResourceStatusBadge :tone="statusTones[detail.output.publish_status] || 'neutral'" :label="statusLabels[detail.output.publish_status] || detail.output.publish_status" /></div>
              <div><span class="output-detail__label">大小</span><span>{{ formatBytes(detail.output.file_size_bytes) }}</span></div>
              <div><span class="output-detail__label">时长</span><span>{{ durationText(detail.output) }}</span></div>
              <div><span class="output-detail__label">生成时间</span><span>{{ formatDateTime(detail.output.created_at) }}</span></div>
            </div>
            <div class="output-detail__hash"><span class="output-detail__label">SHA-256</span><span>{{ detail.output.sha256 || '—' }}</span></div>

            <h4 class="output-detail__title">来源追踪</h4>
            <div class="output-detail__grid">
              <div><span class="output-detail__label">素材</span><span>#{{ detail.trace.material.id }} {{ detail.trace.material.title }}</span></div>
              <div><span class="output-detail__label">素材使用</span><span>#{{ detail.trace.material_usage.id }} {{ usageStatusLabel(detail.trace.material_usage.status) || detail.trace.material_usage.status || '—' }}</span></div>
              <div><span class="output-detail__label">策略</span><span>#{{ detail.trace.strategy.id }} {{ detail.trace.strategy.name || '—' }}</span></div>
              <div><span class="output-detail__label">模板</span><span>{{ detail.trace.strategy.template_code }} v{{ detail.trace.strategy.template_version }}</span></div>
              <div><span class="output-detail__label">任务</span><span>#{{ detail.trace.compose_task.id }} {{ taskStatusLabels[detail.trace.compose_task.status] || detail.trace.compose_task.status || '—' }}</span></div>
              <div><span class="output-detail__label">任务创建时间</span><span>{{ formatDateTime(detail.trace.compose_task.created_at) }}</span></div>
            </div>
            <h4 class="output-detail__title">任务参数快照（创建当时，不可变）</h4>
            <pre class="output-detail__snapshot">{{ JSON.stringify(detail.trace.compose_task.config_snapshot, null, 2) }}</pre>
            <h4 class="output-detail__title">策略当前配置</h4>
            <pre class="output-detail__snapshot">{{ JSON.stringify(detail.trace.strategy.config_snapshot, null, 2) }}</pre>
          </div>
        </t-loading>
      </t-drawer>
    </div>
  </t-loading>
</template>

<style scoped>
.my-outputs-card { padding: 14px 20px 18px; }
.output-detail__title { margin: 16px 0 8px; font-size: 13px; font-weight: 600; color: var(--wt-text-primary); }
.output-detail__title:first-child { margin-top: 0; }
.output-detail__video { width: 100%; border-radius: 6px; background: #000; max-height: 280px; }
.output-detail__hint { margin: 0; color: var(--wt-text-secondary); font-size: 13px; }
.output-detail__grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px 24px; font-size: 13px; }
.output-detail__grid > div { display: flex; gap: 8px; align-items: center; }
.output-detail__label { color: var(--wt-text-secondary); min-width: 72px; flex: none; }
.output-detail__hash { display: flex; gap: 8px; align-items: baseline; margin-top: 10px; font-size: 12px; }
.output-detail__hash span:last-child { word-break: break-all; color: var(--wt-text-primary); }
.output-detail__snapshot { margin: 0; padding: 12px; background: var(--wt-bg-secondary, #f5f6f8); border-radius: 6px; font-size: 12px; overflow: auto; max-height: 220px; }
</style>
