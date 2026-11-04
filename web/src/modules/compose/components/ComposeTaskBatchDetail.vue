<script setup>
import { computed, ref, watch } from 'vue'
import { DialogPlugin } from 'tdesign-vue-next'
import { createComposeClient } from '../../../shared/api/compose.js'
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
useAutoRefresh(() => load(true), { shouldSkip: () => !props.visible || busy.value })
</script>
<template>
  <ResourceOverlay kind="drawer" :visible="visible" :title="batch?.name || '任务批次详情'" :width="1040" :busy="busy" :footer="!!batch && (actionCount('cancel') > 0 || actionCount('retry') > 0)" @close="close">
    <FeedbackAlert v-if="error" theme="error" :message="error" />
    <t-loading :loading="loading" :show-overlay="true">
      <div v-if="batch" class="batch-detail">
        <section><h4>批次信息</h4><div class="batch-detail__fields"><div><span>批次 ID</span><strong>{{ batch.id }}</strong></div><div><span>合成策略</span><strong>{{ batch.strategy_name || `策略 ${batch.compose_strategy_id}` }}</strong></div><div><span>负责人</span><strong>{{ batch.operator_name || '未记录' }}</strong></div><div><span>触发来源</span><strong>{{ triggerLabel(batch.trigger_type) }}</strong></div><div><span>创建时间</span><strong>{{ formatDateTime(batch.created_at) }}</strong></div></div></section>
        <section><h4>全批进度</h4><div class="batch-detail__counts"><span v-for="(label, key) in TASK_STATUS_LABELS" :key="key">{{ label }} <strong>{{ batch.counts?.[key] ?? 0 }}</strong></span></div><t-progress :percentage="progress.percent" /><p>已结束 {{ progress.finished }} / {{ progress.total }}，成功 {{ batch.counts?.success ?? 0 }} 条</p></section>
        <FeedbackAlert v-if="result" :theme="result.failed || result.skipped ? 'warning' : 'success'" :message="bulkResultMessage(result, result.action)" />
        <div v-if="result?.items?.some((item) => item.status !== 'accepted')" class="batch-detail__results"><p v-for="item in result.items.filter((item) => item.status !== 'accepted')" :key="item.task_id">任务 {{ item.task_id }} · {{ item.status === 'skipped' ? '跳过' : '失败' }} · {{ item.reason }}</p></div>
        <div class="batch-detail__toolbar"><label class="wt-filter-field"><span class="wt-filter-field__label">任务状态</span><t-select v-model="status" :options="statusOptions" clearable placeholder="全部" @change="filter" /></label><span v-if="selectedKeys.length">针对已选 {{ selectedKeys.length }} 条任务 <t-link theme="primary" @click="selectedKeys = []">清空选择</t-link></span><span v-else>操作范围：整个批次</span></div>
        <div class="table-scroll-wrap"><t-table class="wt-resource-table" :data="rows" :columns="columns" row-key="id" v-model:selected-row-keys="selectedKeys" reserve-selected-row-on-paginate :scroll="tableScroll" size="small" hover empty="暂无符合条件的任务"><template #status="{row}">{{ TASK_STATUS_LABELS[row.status] || row.status }}<template v-if="row.cancel_requested && row.status === 'running'"> · 已请求停止</template></template><template #created_at="{row}">{{ formatDateTime(row.created_at) }}</template><template #op="{row}"><t-button size="small" variant="outline" class="wt-secondary-button" @click="emit('task', row)">详情</t-button></template></t-table></div>
        <div class="pagination-bar"><t-pagination v-model:current="pagination.current" v-model:pageSize="pagination.pageSize" :total="total" :page-size-options="[10,20,50]" @current-change="load()" @page-size-change="filter" /></div>
      </div>
    </t-loading>
    <template #footer><t-button v-if="actionCount('retry') && canRetry" variant="outline" class="wt-secondary-button" :loading="busy" @click="command('retry')">重试（{{ actionCount('retry') }}）</t-button><t-button v-if="actionCount('cancel')" variant="outline" class="wt-secondary-button wt-danger-button" :loading="busy" @click="command('cancel')">取消（{{ actionCount('cancel') }}）</t-button></template>
  </ResourceOverlay>
</template>
<style scoped>
.batch-detail { display: grid; gap: 16px; }
.batch-detail section { padding: 16px; border: 1px solid var(--wt-border); border-radius: var(--wt-radius-md); }
.batch-detail h4 { margin: 0 0 14px; color: var(--wt-text-primary); }
.batch-detail__fields { display: grid; grid-template-columns: repeat(2,minmax(0,1fr)); gap: 12px 20px; }
.batch-detail__fields > div { display: grid; grid-template-columns: 90px 1fr; gap: 8px; }
.batch-detail__fields span,.batch-detail p { color: var(--wt-text-secondary); font-size: 13px; }
.batch-detail__fields strong { font-weight: 500; overflow-wrap: anywhere; }
.batch-detail__counts,.batch-detail__toolbar { display: flex; align-items: center; gap: 16px; flex-wrap: wrap; margin-bottom: 12px; }
.batch-detail__toolbar > .wt-filter-field { flex: 0 0 220px; width: 220px; min-width: 0; max-width: 100%; }
.batch-detail__counts strong { color: var(--wt-text-primary); }
.batch-detail__results { max-height: 160px; overflow: auto; }
@media(max-width:600px) { .batch-detail__fields { grid-template-columns:1fr; } }
</style>
