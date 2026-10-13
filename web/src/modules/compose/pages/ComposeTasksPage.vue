<script setup>
import { feedback } from '../../../shared/ui/feedback/index.js'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import { computed, onMounted, ref } from 'vue'
import { DialogPlugin } from 'tdesign-vue-next'
import { createComposeClient } from '../../../shared/api/compose.js'
import ResourceCard from '../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../shared/ui/resource/ResourcePageHeader.vue'
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'
import { opColumnWidth } from '../../../shared/ui/resource/opColumnWidth.js'
import { formatDateTime } from '../../../shared/utils/datetime.js'

// CHG-20261013-082：合成任务页。列表/详情/取消，筛选（状态、策略）走服务端。
// M4-C3 只有 pending/cancelled 两个状态真实流转，running/success/failed 由
// M4-C5 接管——状态词表一次给全，行里出现哪种画哪种。
const client = createComposeClient()

const rows = ref([])
const strategies = ref([])
const loading = ref(false)
const error = ref('')
const filters = ref({ status: '', compose_strategy_id: '' })
const pagination = ref({ current: 1, pageSize: 20 })

const statusLabels = { pending: '排队中', running: '执行中', success: '已完成', failed: '失败', cancelled: '已取消' }
const statusTones = { pending: 'warning', running: 'info', success: 'success', failed: 'danger', cancelled: 'neutral' }
// 阶段是执行投影的词表；M4-C3 的 pending 还没有执行阶段，显示名钉在合同里：
// 等待源素材（源视频未就绪时任务保持 pending 并等待，PRD §5.4.4）。
const stageLabels = { pending: '等待源素材' }

// 每一列显式 width（规范 §6.2，WebKit fixed 布局下 minWidth 不产生确定列宽）。
// 操作列宽按行里最多会出现的按钮清单推导：排队中的行才有 取消。
const columns = computed(() => [
  { colKey: 'id', title: '任务 ID', width: 80 },
  { colKey: 'strategy', title: '合成策略', width: 180 },
  { colKey: 'template', title: '模板', width: 130 },
  { colKey: 'material_id', title: '素材 ID', width: 90 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'stage', title: '阶段', width: 120 },
  { colKey: 'progress', title: '进度', width: 80 },
  { colKey: 'created_at', title: '创建时间', width: 150 },
  { colKey: 'op', title: '操作', width: opColumnWidth(['详情', '取消']), fixed: 'right' },
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
const strategyOptions = computed(() => strategies.value.map((item) => ({ value: String(item.id), label: item.name })))

function strategyName(row) {
  const found = strategies.value.find((item) => item.id === row.compose_strategy_id)
  return found ? found.name : `#${row.compose_strategy_id}`
}

function templateText(row) {
  return `${row.template_code} v${row.template_version}`
}

function stageText(row) {
  if (row.stage) return stageLabels[row.stage] || row.stage
  return row.status === 'pending' ? stageLabels.pending : '—'
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const params = {}
    if (filters.value.status) params.status = filters.value.status
    if (filters.value.compose_strategy_id) params.compose_strategy_id = filters.value.compose_strategy_id
    const data = await client.listComposeTasks(params)
    rows.value = Array.isArray(data) ? data : []
    pagination.value.current = 1
  } catch (e) {
    error.value = e?.message || '读取合成任务失败'
  } finally {
    loading.value = false
  }
}

function reset() {
  filters.value = { status: '', compose_strategy_id: '' }
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

async function openDetail(row) {
  detailVisible.value = true
  try {
    // 按 id 重取：详情里的快照必须是库里存的那份，不是列表加载那一刻的副本。
    detail.value = await client.getComposeTask(row.id)
  } catch (e) {
    feedback.toast.error(e?.message || '打开任务失败')
    detailVisible.value = false
  }
}

function cancel(row) {
  const dialog = DialogPlugin.confirm({
    header: '取消合成任务',
    body: `确认取消任务 #${row.id}？排队中的任务取消后不再执行。`,
    confirmBtn: { content: '取消任务', theme: 'danger' },
    onConfirm: async () => {
      dialog.destroy()
      try {
        await client.cancelComposeTask(row.id)
        feedback.toast.success('已取消')
      } catch (e) {
        feedback.toast.error(e?.message || '取消失败')
      }
      // 成败都重读：409（已被领取）也只有重读才看得出任务现在是什么状态。
      await load()
    },
  })
}

onMounted(() => {
  load()
  loadStrategies()
})
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true">
    <div class="wt-resource-page compose-tasks-page">
      <FeedbackAlert v-if="error" theme="error" :message="error" closable style="margin-bottom:16px" @close="error=''" />
      <ResourcePageHeader title="合成任务" description="从我的素材创建的合成任务，在这里查看进度并取消排队中的任务">
        <template #actions>
          <t-button class="wt-secondary-button" variant="outline" @click="load">刷新</t-button>
        </template>
      </ResourcePageHeader>
      <ResourceCard class="compose-tasks-card">
        <div class="filter-bar">
          <label class="wt-filter-field"><span class="wt-filter-field__label">状态</span><t-select v-model="filters.status" :options="statusOptions" clearable placeholder="全部" /></label>
          <label class="wt-filter-field wt-filter-field--wide"><span class="wt-filter-field__label">合成策略</span><t-select v-model="filters.compose_strategy_id" :options="strategyOptions" clearable placeholder="全部" /></label>
          <div class="wt-filter-actions">
            <t-button theme="primary" @click="load">查询</t-button>
            <t-button class="wt-secondary-button" variant="outline" @click="reset">重置</t-button>
          </div>
        </div>
        <div class="table-scroll-wrap">
          <t-table class="wt-resource-table" :data="pagedRows" :columns="columns" row-key="id" hover size="small" :scroll="tableScroll" empty="还没有合成任务">
            <template #strategy="{ row }">{{ strategyName(row) }}</template>
            <template #template="{ row }">{{ templateText(row) }}</template>
            <template #status="{ row }"><ResourceStatusBadge :tone="statusTones[row.status] || 'default'" :label="statusLabels[row.status] || row.status" /></template>
            <template #stage="{ row }">{{ stageText(row) }}</template>
            <template #progress="{ row }">{{ row.progress }}%</template>
            <template #created_at="{ row }">{{ formatDateTime(row.created_at) }}</template>
            <template #op="{ row }">
              <t-space class="wt-resource-actions">
                <t-button size="small" class="wt-secondary-button" variant="outline" @click="openDetail(row)">详情</t-button>
                <t-button v-if="row.status === 'pending'" size="small" class="wt-secondary-button wt-danger-button" variant="outline" @click="cancel(row)">取消</t-button>
              </t-space>
            </template>
          </t-table>
        </div>
        <div class="pagination-bar"><t-pagination v-model:current="pagination.current" v-model:pageSize="pagination.pageSize" :total="rows.length" :page-size-options="[10, 20, 50]" /></div>
      </ResourceCard>

      <t-dialog v-model:visible="detailVisible" header="任务详情" width="640px" :footer="false">
        <t-loading :loading="!detail" :show-overlay="true">
          <div v-if="detail" class="compose-detail">
            <div class="compose-detail__grid">
              <div><span class="compose-detail__label">任务 ID</span><span>{{ detail.id }}</span></div>
              <div><span class="compose-detail__label">状态</span><ResourceStatusBadge :tone="statusTones[detail.status] || 'neutral'" :label="statusLabels[detail.status] || detail.status" /></div>
              <div><span class="compose-detail__label">合成策略</span><span>#{{ detail.compose_strategy_id }}</span></div>
              <div><span class="compose-detail__label">模板</span><span>{{ templateText(detail) }}</span></div>
              <div><span class="compose-detail__label">素材 ID</span><span>{{ detail.material_id }}</span></div>
              <div><span class="compose-detail__label">阶段</span><span>{{ stageText(detail) }}</span></div>
              <div><span class="compose-detail__label">创建时间</span><span>{{ formatDateTime(detail.created_at) }}</span></div>
              <div v-if="detail.finished_at"><span class="compose-detail__label">结束时间</span><span>{{ formatDateTime(detail.finished_at) }}</span></div>
            </div>
            <template v-if="detail.error_code || detail.error_message">
              <h4 class="compose-detail__title">失败信息</h4>
              <t-alert theme="error" :message="detail.error_message || detail.error_code" />
            </template>
            <h4 class="compose-detail__title">参数快照（创建当时，不可变）</h4>
            <pre class="compose-detail__snapshot">{{ JSON.stringify(detail.config_snapshot, null, 2) }}</pre>
          </div>
        </t-loading>
      </t-dialog>
    </div>
  </t-loading>
</template>

<style scoped>
.compose-tasks-card { padding: 14px 20px 18px; }
.compose-detail__grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px 24px; font-size: 13px; }
.compose-detail__grid > div { display: flex; gap: 8px; align-items: center; }
.compose-detail__label { color: var(--wt-text-secondary); min-width: 64px; }
.compose-detail__title { margin: 16px 0 8px; font-size: 13px; font-weight: 600; color: var(--wt-text-primary); }
.compose-detail__snapshot { margin: 0; padding: 12px; background: var(--wt-bg-secondary, #f5f6f8); border-radius: 6px; font-size: 12px; overflow: auto; max-height: 220px; }
</style>
