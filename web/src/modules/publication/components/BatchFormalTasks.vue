<script setup>
import { onBeforeUnmount, ref, watch } from "vue";
import ResourceFilterBar from "../../../shared/ui/resource/ResourceFilterBar.vue";
import ResourceId from "../../../shared/ui/resource/ResourceId.vue";
import ResourceStatusBadge from "../../../shared/ui/resource/ResourceStatusBadge.vue";
import FeedbackAlert from "../../../shared/ui/feedback/FeedbackAlert.vue";
import { createPublicationBatchClient } from "../../../shared/api/publication.js";
import { publicationStatuses } from "../status.js";
import { executionPhase } from "../executionPresentation.js";
import { publicationTitle } from '../platformContent.js';

const props = defineProps({
  batch: { type: Object, required: true },
  accountOptions: { type: Array, default: () => [] },
  accountName: Function,
  outputName: Function,
  taskLink: Function,
  scope: { type: Object, required: true },
  active: Boolean,
});
const emit = defineEmits(["scope", "refresh-summary"]);
const api = createPublicationBatchClient();
const state = ref({ items: [], total: 0 });
const fields = ref({});
const loading = ref(false);
const error = ref("");
let sequence = 0;
let taskRead = null;
let disposed = false;
const columns = [
  { colKey: "id", title: "任务 ID", width: 135 },
  { colKey: "content", title: "发布文案", ellipsis: true, width: 204 },
  { colKey: "account", title: "发布账号", width: 148 },
  { colKey: "output", title: "成片", width: 153 },
  { colKey: "status", title: "任务状态", width: 125 },
  { colKey: "group", title: "执行组", width: 92 },
  { colKey: "execution", title: "阶段与阻断", width: 215 },
  { colKey: "operation", title: "操作", width: 112, fixed: "right" },
];
const statuses = Object.entries(publicationStatuses).map(([value, label]) => ({ value, label }));

async function load() {
  const run = ++sequence;
  if (!props.active) {
    loading.value = false;
    return;
  }
  loading.value = true;
  error.value = "";
  const { id } = props.batch;
  const query = props.scope;
  const key = JSON.stringify([id, query]);
  if (taskRead?.key !== key) taskRead = { key, promise: api.tasks(id, query) };
  const request = taskRead;
  try {
    const page = await request.promise;
    if (run === sequence) state.value = page;
  } catch (failure) {
    if (run === sequence) error.value = failure.message || "读取正式任务失败";
  } finally {
    if (taskRead === request) taskRead = null;
    if (run === sequence) loading.value = false;
  }
}
async function refresh() {
  // A summary read can finish while an older task response is still pending.
  while (!disposed && props.active && taskRead) await taskRead.promise.catch(() => {});
  if (!disposed && props.active) await load();
}
defineExpose({ refresh });
watch(() => JSON.stringify(props.scope), () => { fields.value = { ...props.scope }; }, { immediate: true });
watch([() => props.batch.id, () => JSON.stringify(props.scope), () => props.active], load, { immediate: true });
onBeforeUnmount(() => {
  disposed = true;
  sequence++;
});

function apply(page = 1, pageSize = props.scope.page_size) {
  const { search, status, media_account_id } = fields.value;
  const next = { search, status, media_account_id, page, page_size: pageSize };
  if (JSON.stringify(next) === JSON.stringify(props.scope)) load();
  else emit("scope", next);
}
function query(resetPage = false) {
  const filtersChanged = ["search", "status", "media_account_id"].some(key =>
    (fields.value[key] || "") !== (props.scope[key] || ""));
  apply(resetPage || filtersChanged ? 1 : props.scope.page);
  emit("refresh-summary");
}
function reset() {
  fields.value = {};
  query(true);
}
function taskAction(task) {
  const actions = task.allowed_actions || [];
  if (actions.includes("resume")) return "去处理";
  if (actions.includes("resolve")) return task.status === "reviewing" ? "补录结果" : "核实结果";
  if (actions.includes("retry")) return "处理失败";
  return "查看任务";
}
</script>

<template>
  <div class="batch-tab">
    <ResourceFilterBar>
      <label class="wt-filter-field wt-filter-field--wide">
        <span class="wt-filter-field__label">搜索任务</span>
        <t-input v-model="fields.search" aria-label="搜索任务" :maxlength="100" clearable @enter="query()" />
      </label>
      <label class="wt-filter-field">
        <span class="wt-filter-field__label">发布账号</span>
        <t-select v-model="fields.media_account_id" aria-label="发布账号" :options="accountOptions" placeholder="全部" clearable />
      </label>
      <label class="wt-filter-field">
        <span class="wt-filter-field__label">任务状态</span>
        <t-select v-model="fields.status" aria-label="任务状态" :options="statuses" placeholder="全部" clearable />
      </label>
      <div class="wt-filter-actions">
        <t-button theme="primary" :disabled="loading" @click="query()">查询</t-button>
        <t-button theme="default" variant="outline" :disabled="loading" @click="reset">重置</t-button>
      </div>
    </ResourceFilterBar>
    <FeedbackAlert v-if="error" theme="error" :message="error"><t-button theme="default" variant="text" @click="load">重新读取</t-button></FeedbackAlert>
    <div class="table-scroll-wrap resource-data-table">
      <t-table class="wt-resource-table" :columns="columns" :data="state.items || []" row-key="id"
        :scroll="{ x: '1184px' }" :loading="loading" :empty="error ? '任务读取失败' : '没有符合条件的正式任务'">
        <template #id="{ row }"><ResourceId :value="row.id" label="任务 ID" /></template>
        <template #content="{ row }">{{ publicationTitle(batch.platform,row.content) || "—" }}</template>
        <template #account="{ row }"><span class="batch-task-resource-name" :title="row.resources?.account_name || accountName(row)">{{ row.resources?.account_name || accountName(row) }}</span></template>
        <template #output="{ row }"><span class="batch-task-resource-name" :title="row.resources?.output_name || outputName(row)">{{ row.resources?.output_name || outputName(row) }}</span></template>
        <template #status="{ row }"><ResourceStatusBadge :label="publicationStatuses[row.status] || '状态未知'"
          :tone="row.status === 'published' ? 'success' : ['failed','awaiting_human','uncertain'].includes(row.status) ? 'warning' : 'neutral'" /></template>
        <template #group="{ row }">{{ row.execution_group_no ? '第 ' + row.execution_group_no + ' 组' : '—' }}</template>
        <template #execution="{ row }">
          <span v-if="row.execution_stage">{{ executionPhase(row.execution_stage) }}</span>
          <small v-if="row.execution_block?.message" class="batch-muted batch-task-block">{{ row.execution_block.message }}</small>
          <span v-if="!row.execution_stage && !row.execution_block?.message">—</span>
        </template>
        <template #operation="{ row }"><RouterLink class="resource-name-link" :to="taskLink(row.id)">{{ taskAction(row) }}</RouterLink></template>
      </t-table>
    </div>
    <div class="pagination-bar">
      <t-pagination :current="scope.page" :page-size="scope.page_size" :total="state.total" :page-size-options="[10,20,50]"
        :show-jumper="false" @current-change="apply($event)" @page-size-change="apply(1, $event)" />
    </div>
  </div>
</template>

<style scoped>
/* BatchDetailContent prototype: retain full resource names, bound their row height. */
.batch-task-resource-name { overflow-wrap:anywhere; max-width:100%; font-weight:400; display:-webkit-box; -webkit-line-clamp:2; -webkit-box-orient:vertical; overflow:hidden; text-align:left; }
</style>
