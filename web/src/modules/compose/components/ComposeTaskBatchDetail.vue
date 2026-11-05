<script setup>
import { computed, ref, watch } from 'vue'
import { DialogPlugin } from 'tdesign-vue-next'
import { createComposeClient } from '../../../shared/api/compose.js'
import ComposeBatchProgress from './ComposeBatchProgress.vue'
import ResourceSection from '../../../shared/ui/resource/ResourceSection.vue'
import ResourceDescriptions from '../../../shared/ui/resource/ResourceDescriptions.vue'
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'
import ResourceRowActions from '../../../shared/ui/resource/ResourceRowActions.vue'
import ResourceSelectionToolbar from '../../../shared/ui/resource/ResourceSelectionToolbar.vue'
import ResourceOverlay from '../../../shared/ui/resource/ResourceOverlay.vue'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import { formatDateTime } from '../../../shared/utils/datetime.js'
import { triggerLabel } from '../labels.js'
import { useAutoRefresh } from '../../../shared/utils/useAutoRefresh.js'
import { TASK_STATUS_LABELS, batchProgress, taskActionEligible, bulkResultMessage, selectionSkipSummary } from '../batchActions.js'
const props = defineProps({ visible: Boolean, batchId: Number, canRetry: Boolean })
const emit = defineEmits(['update:visible', 'task', 'changed'])
const client = createComposeClient()
const batch = ref(null), rows = ref([]), total = ref(0), loading = ref(false), error = ref(''), busy = ref(false)
const status = ref(''), pagination = ref({ current: 1, pageSize: 10 }), selectedKeys = ref([]), knownTasks = ref({}), result = ref(null)
let sequence = 0
const progress = computed(() => batchProgress(batch.value?.counts))
const statusOptions = Object.entries(TASK_STATUS_LABELS).map(([value, label]) => ({ value, label }))
const selection = computed(() => selectedKeys.value.map((id) => knownTasks.value[id]).filter(Boolean))
const actionCount = (action) => action === 'retry' && !props.canRetry ? 0 : selection.value.length ? selection.value.filter((row) => taskActionEligible(row, action, props.canRetry)).length : Number(batch.value?.actions?.[`${action}_count`] || 0)
const columns = [ { colKey: 'row-select', type: 'multiple', width: 44 }, { colKey: 'id', title: 'ID', width: 80 }, { colKey: 'display_name', title: '任务名称', width: 230, ellipsis: true }, { colKey: 'setting_name', title: '成片设置', width: 140 }, { colKey: 'status', title: '状态', width: 110 }, { colKey: 'created_at', title: '创建时间', width: 160 }, { colKey: 'op', title: '操作', width: 80, fixed: 'right' } ]
const tableScroll = computed(() => ({x:`${columns.reduce((sum,column) => sum + (column.width || 0),0)}px`}))
async function load(quiet = false) {
  const request = ++sequence
  if (!props.batchId || !props.visible) return
  if (!quiet) loading.value = true
  error.value = ''
  try {
    const [view, tasks] = await Promise.all([client.getComposeTaskBatch(props.batchId), client.listComposeTasks({ batch_id: props.batchId, status: status.value || undefined, page: pagination.value.current, page_size: pagination.value.pageSize })])
    if (request !== sequence) return
    batch.value = { ...view.batch, actions: view.actions }
    rows.value = tasks.items || []
    total.value = tasks.total || 0
    for (const row of rows.value) knownTasks.value[row.id] = row
  } catch (e) { if (request === sequence) error.value = e?.message || '批次读取失败' }
  finally { if (request === sequence) loading.value = false }
}
watch(() => [props.visible, props.batchId], () => {
  if (!props.visible) { ++sequence; return }
  batch.value = null; selectedKeys.value = []; knownTasks.value = {}; result.value = null; status.value = ''; pagination.value.current = 1
  load()
})
function filter() { pagination.value.current = 1; load() }
function close() { if (!busy.value) emit('update:visible', false) }
function command(action) {
  const count = actionCount(action)
  if (!count || busy.value) return
  const selected = [...selectedKeys.value]
  const scope = selected.length ? `所选 ${selected.length} 条任务` : `整个批次的 ${progress.value.total} 条任务`
  const skipped = selected.length ? selectionSkipSummary(selection.value, action, props.canRetry) : batch.value.actions?.[`${action}_skipped`] || []
  const skipText = skipped.map((item) => `${item.reason} ${item.count} 条`).join('；')
  const dialog = DialogPlugin.confirm({ header: action === 'retry' ? '重试任务' : '取消任务', body: `${scope}中，当前可${action === 'retry' ? '重试' : '取消'} ${count} 条。${skipText ? `跳过：${skipText}。` : ''}提交时逐项重新核验。${action === 'cancel' ? '运行任务先受理停止请求，确认退出后才会变为已取消。' : '使用原任务和原配置，不创建新批次。'}`, confirmBtn: { content: action === 'retry' ? '重试' : '取消任务', theme: action === 'retry' ? 'primary' : 'danger' }, onConfirm: async () => {
    if (busy.value) return
    busy.value = true
    try {
      const payload = selected.length ? { task_ids: selected } : { batch_id: props.batchId }
      result.value = await (action === 'retry' ? client.batchRetryComposeTasks(payload) : client.batchCancelComposeTasks(payload))
      result.value.action = action
      selectedKeys.value = []
      await load(true)
      emit('changed')
      dialog.hide()
    } catch (e) { error.value = e?.message || '操作失败'; dialog.hide() }
    finally { busy.value = false }
  } })
}
useAutoRefresh(() => load(true), { shouldSkip: () => !props.visible || busy.value || selectedKeys.value.length > 0 })
</script>
<template>
  <ResourceOverlay kind="drawer" :visible="visible" :title="batch?.name || '任务批次详情'" :width="1040" :busy="busy" :footer="!!batch && (actionCount('cancel') > 0 || actionCount('retry') > 0)" @close="close">
    <FeedbackAlert v-if="error" theme="error" :message="error" />
    <t-loading :loading="loading" :show-overlay="true">
      <div v-if="batch" class="batch-detail wt-resource-page resource-visual-approved production-page">
        <ResourceSection title="批次信息"><ResourceDescriptions><t-descriptions-item label="批次 ID">{{ batch.id }}</t-descriptions-item><t-descriptions-item label="合成策略">{{ batch.strategy_name || `策略 ${batch.compose_strategy_id}` }}</t-descriptions-item><t-descriptions-item label="负责人">{{ batch.operator_name || '—' }}</t-descriptions-item><t-descriptions-item label="触发方式">{{ triggerLabel(batch.trigger_type) }}</t-descriptions-item><t-descriptions-item label="创建时间">{{ formatDateTime(batch.created_at) }}</t-descriptions-item></ResourceDescriptions></ResourceSection>
        <ResourceSection title="全批进度"><ComposeBatchProgress :counts="batch.counts" /></ResourceSection>
        <FeedbackAlert v-if="result" :theme="result.failed || result.skipped ? 'warning' : 'success'" :message="bulkResultMessage(result, result.action)" />
        <div v-if="result?.items?.some((item) => item.status !== 'accepted')" class="batch-detail__results"><p v-for="item in result.items.filter((item) => item.status !== 'accepted')" :key="item.task_id">任务 {{ item.task_id }} · {{ item.status === 'skipped' ? '跳过' : '失败' }} · {{ item.reason }}</p></div>
        <div class="batch-detail__toolbar"><label class="wt-filter-field"><span class="wt-filter-field__label">任务状态</span><t-select v-model="status" :options="statusOptions" clearable placeholder="全部" @change="filter" /></label><span v-if="!selectedKeys.length" class="batch-detail__scope">操作范围：整个批次</span></div>
        <ResourceSelectionToolbar :count="selectedKeys.length" unit="条任务" @clear="selectedKeys = []" />
        <div class="table-scroll-wrap"><t-table class="wt-resource-table" :data="rows" :columns="columns" row-key="id" v-model:selected-row-keys="selectedKeys" reserve-selected-row-on-paginate :scroll="tableScroll" size="small" hover empty="暂无符合条件的任务"><template #status="{row}"><ResourceStatusBadge :label="TASK_STATUS_LABELS[row.status] || row.status" :tone="({pending:'warning',running:'info',success:'success',failed:'danger',cancelled:'neutral'})[row.status]" /><template v-if="row.cancel_requested && row.status === 'running'"> · 已请求停止</template></template><template #created_at="{row}">{{ formatDateTime(row.created_at) }}</template><template #op="{row}"><ResourceRowActions><t-button size="small" variant="text" @click="emit('task', row)">详情</t-button></ResourceRowActions></template></t-table></div>
        <div class="pagination-bar"><t-pagination :show-jumper="false" v-model:current="pagination.current" v-model:pageSize="pagination.pageSize" :total="total" :page-size-options="[10,20,50]" @current-change="load()" @page-size-change="filter" /></div>
      </div>
    </t-loading>
    <template #footer><t-button v-if="actionCount('retry') && canRetry" variant="outline" class="wt-secondary-button" :loading="busy" @click="command('retry')">重试（{{ actionCount('retry') }}）</t-button><t-button v-if="actionCount('cancel')" variant="outline" class="wt-secondary-button wt-danger-button" :loading="busy" @click="command('cancel')">取消（{{ actionCount('cancel') }}）</t-button></template>
  </ResourceOverlay>
</template>
<style scoped>
.batch-detail {display:grid;gap:28px;min-width:0;}
.batch-detail > * {min-width:0;}
.batch-detail .resource-section + .resource-section {margin-top:0;}
.batch-detail__toolbar {display:flex;align-items:end;flex-wrap:wrap;gap:16px 24px;}
.batch-detail__toolbar > .wt-filter-field {flex:0 1 220px;max-width:100%;}
.batch-detail__scope {font-size:12px;color:var(--wt-text-tertiary);}
.batch-detail__results {max-height:160px;overflow:auto;font-size:13px;color:var(--wt-text-secondary);}
</style>
