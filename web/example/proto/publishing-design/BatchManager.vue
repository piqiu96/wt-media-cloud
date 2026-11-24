<script setup>
import BatchDetailContent from "./BatchDetailContent.vue";
import ResourceEmptyState from "../../../src/shared/ui/resource/ResourceEmptyState.vue";
import ResourceNameLink from '../../../src/shared/ui/resource/ResourceNameLink.vue';
import ResourceId from '../../../src/shared/ui/resource/ResourceId.vue';
import {formatDateTime} from '../../../src/shared/utils/datetime.js';
import {readOperationRoute,operationURL,updateOperationURL} from "./routes.js";
import { computed, ref, nextTick, watch, onMounted, onBeforeUnmount, h } from "vue";
import { useMediaQuery } from "../../../src/shared/composables/useMediaQuery.js";
import ResourceFieldHelp from "../../../src/shared/ui/resource/ResourceFieldHelp.vue";
import "../../../src/shared/styles/resource-data-table.css";
import ResourceNotice from "../../../src/shared/ui/resource/ResourceNotice.vue";
import ResourceCard from "../../../src/shared/ui/resource/ResourceCard.vue";
import ResourceFilterBar from "../../../src/shared/ui/resource/ResourceFilterBar.vue";
import ResourceRowActions from "../../../src/shared/ui/resource/ResourceRowActions.vue";
import Overlay from "./PrototypeOverlay.vue";
import ResourceDetailPage from "../../../src/shared/ui/resource/ResourceDetailPage.vue";
import RequiredLabel from "./RequiredLabel.vue";
import BatchContentWorkspace from "./BatchContentWorkspace.vue";
import TaskActionDialog from "./TaskActionDialog.vue";
import { feedback } from "../../../src/shared/ui/feedback/index.js";
import {
  batchState,
  formalBatch,
  batchMetrics,
  batchActions,
  rowsFor,
  executionPreview,
  contentEditable,
} from "./batchLifecycle.js";
import { currentPrototypeUser } from "./prototypeSession.js";
import { batchCommand } from "./batchApi.js";
const props = defineProps({ db: Object, persist: Function, active: Boolean, preparationOnly: Boolean });
const emit = defineEmits(["configure", "recover", "copy-batch", "task", "edit-task", "preview", "detail-change", "navigate"]);
const narrow = useMediaQuery("(max-width: 900px)");
const query = ref(""),
  search = ref(""),
  stateFilter = ref(""),
  platform = ref(""),
  current = ref(null),
  detailOpen = ref(false),
  taskSearch = ref(""),
  taskFilter = ref(""),
  op = ref(""),
  opOpen = ref(false),
  partial = ref(false),
  takeover = ref(false),
  name = ref(""),
  reason = ref(""),
  error = ref(""),
  busy = ref(false),
  editOpen = ref(false),
  editRows = ref([]),
  editBaseline = ref(""),
  working = ref(false),
  taskAction = ref(null),
  version = ref(0),
  requestKey = ref("");
const labels = {
  configure: "继续配置",
  execute: "执行",
  edit: "编辑",
  detail: "详情",
  process: "去处理",
  stop: "停止发布",
  cancel: "终止",
  copy: "复制",
};
const progressHelp = "已公开发布数 / 发布目标数。";
const states = computed(() => props.preparationOnly ? ["未开始"] : ["待执行", "执行中", "待处理", "已结束", "已取消"]);
const batch = computed(() =>
  props.db.batches.find((b) => b.id === current.value),
);
const metrics = computed(() =>
  batch.value ? batchMetrics(props.db, batch.value) : {},
);
const actionsFor = row => row.preparationOnly && currentPrototypeUser().id !== row.ownerId
  ? ["detail"] : batchActions(props.db, row);
const detailActions = computed(() => batch.value
  ? actionsFor(batch.value).filter((a) => !["detail", "process", "copy", "cancel", "edit"].includes(a))
  : []);
const records = computed(() =>
  props.db.batches
    .filter(
      (b) =>
        formalBatch(props.db, b) && !!b.preparationOnly === props.preparationOnly &&
        (!b.preparationOnly || currentPrototypeUser().role !== "运营" || b.ownerId === currentPrototypeUser().id) &&
        (!search.value ||
          `${b.name} B-${b.id}`
            .toLowerCase()
            .includes(search.value.toLowerCase())) &&
        (!stateFilter.value || batchState(props.db, b) === stateFilter.value) &&
        (!platform.value || b.platform === platform.value),
    )
    .slice()
    .sort((a, b) =>
      String(b.updatedAt || b.created || "").localeCompare(
        String(a.updatedAt || a.created || ""),
      ),
    ),
);
const rows = computed(() =>
  batch.value ? rowsFor(props.db, batch.value) : [],
);
const preview = computed(() =>
  batch.value ? executionPreview(props.db, batch.value) : [],
);
const usable = computed(() => preview.value.filter((x) => !x.errors.length));
const blocked = computed(() => preview.value.filter((x) => x.errors.length));
const plan = (b) => props.db.plans.find((p) => p.id === b.planId);
const sourcePlan = (b) => props.db.plans.find((p) => p.id === b.sourcePlanId);
const account = (id) =>
  props.db.accounts.find((a) => a.id === id)?.name || "原账号不存在";
const date = formatDateTime;
const columns = computed(() => [
  { colKey: "id", title: "批次ID", width: 115 },
  { colKey: "identity", title: "批次名称", width: 220 },
  { colKey: "platform", title: "发布平台", width: 105 },
  { colKey: "plan", title: "所属计划", width: 125 },
  { colKey: "target", title: "发布目标", width: 125 },
  { colKey: "progress", title: () => h("span", { class: "batch-progress-column-title" }, ["发布进度", h(ResourceFieldHelp, { label: "发布进度", content: progressHelp })]), width: 172 },
  { colKey: "state", title: "批次状态", width: 108 },
  { colKey: "pending", title: "待处理", width: 72 },
  { colKey: "updated", title: "最近更新", width: 132 },
  {
    colKey: "operation",
    title: "操作",
    width: 216,
    ...(narrow.value ? {} : { fixed: "right" }),
  },
]);
function open(b, process = false, updateURL = true, restore = false) {
  if (!formalBatch(props.db, b)) {
    detailOpen.value = false;
    const skeleton = b.planId || props.db.plans.some(p => p.items?.some(i => i.kind === 'batch' && i.id === b.id));
    emit(skeleton ? "configure" : "recover", b);
    return;
  }
  current.value = b.id;
  detailOpen.value = true;
  if(restore){try{const saved=JSON.parse(sessionStorage.getItem('publishing-batch-view:'+b.id)||'null');if(saved){taskSearch.value=saved.search;taskFilter.value=saved.filter;nextTick(()=>window.scrollTo(0,saved.scroll||0));}}catch{}}
  if(!restore){taskFilter.value = process ? "待处理" : "";
  taskSearch.value = "";}
  error.value = "";
  if (updateURL) {
    updateOperationURL("batches",{batchId:b.id,pending:process});
    if(!restore)window.scrollTo(0, 0);
  }
}
function returnToList() {
  const fromPlan=new URL(location.href).searchParams.get('fromPlan');
  const p=props.db.plans.find(p=>String(p.id)===fromPlan&&p.items?.some(i=>i.id===current.value));
  if(p){detailOpen.value=false;emit('navigate',operationURL('plans',{planId:p.id}));return;}
  detailOpen.value = false;
  updateOperationURL("batches");
  window.scrollTo(0, 0);
}
watch(detailOpen, (visible) => emit("detail-change", visible), { flush: "sync" });
watch(() => props.active, (active) => {
  if (!active) detailOpen.value = false;
});
async function action(b, a) {
  current.value = b.id;
  error.value = "";
  if (a === "detail" || a === "process") return open(b, a === "process");
  if (a === "configure" && actionsFor(b).includes(a)) return emit("configure", b);
  if (a === "edit") return edit(b);
  if (a === "copy") {
    if (!batchActions(props.db, b).includes("copy")) return;
    return emit("copy-batch", b);
  }
  op.value = a;
  partial.value = false;
  takeover.value = false;
  name.value = "";
  reason.value = "";
  version.value = b.version || 0;
  requestKey.value = crypto.randomUUID();
  opOpen.value = true;
}
async function command(a, input = {}) {
  // Existing prototype writes complete before the versioned command.
  if (!(await props.persist()))
    throw Error("页面尚未保存，请返回并处理页面顶部的保存提示后重试");
  const result = await batchCommand(
    batch.value.id,
    a,
    { ...input, expectedVersion: version.value },
    requestKey.value,
  );
  Object.assign(props.db, result);
}
async function submit() {
  if (busy.value) return;
  busy.value = true;
  error.value = "";
  try {
    await command(op.value, {
      partial: partial.value,
      takeover: takeover.value,
      name: name.value,
      reason: reason.value,
      taskId: pendingTask.value?.id,
    });
    opOpen.value = false;
    feedback.toast.success(
      {
        execute: "批次执行请求已接受，结构与资源映射已锁定",
        stop: "已停止未启动任务；在途任务继续回传结果",
        cancel: "批次已终止",
        copy: "已生成新草稿，请重新匹配账号与成片",
        abandon: "任务已放弃，请通过新批次补足目标",
        repair: "原资源演示可用性已恢复，请重新检查任务",
      }[op.value],
    );
  } catch (e) {
    error.value = e.message;
  } finally {
    busy.value = false;
  }
}
async function edit(b) {
  if (!contentEditable(props.db, b)) {
    feedback.toast.error("当前批次发布配置已锁定，不能编辑");
    return;
  }
  current.value = b.id;
  version.value = b.version || 0;
  editRows.value = structuredClone(
    JSON.parse(JSON.stringify(rowsFor(props.db, b))),
  ).map((t) => ({ ...t, candidateId: t.candidateId || "task-" + t.id }));
  requestKey.value = crypto.randomUUID();
  error.value = "";
  editOpen.value = true;
  await nextTick();
  editBaseline.value = JSON.stringify(editRows.value);
}
async function closeEdit() {
  if (busy.value || working.value) return;
  if (
    JSON.stringify(editRows.value) !== editBaseline.value &&
    !(await feedback.confirmWarning({
      title: "放弃未保存的内容？",
      message: "本次标题、封面等修改尚未保存，返回后将丢失。",
      confirmText: "放弃修改",
    }))
  )
    return;
  editOpen.value = false;
}
async function saveContent() {
  if (busy.value || working.value) return;
  busy.value = true;
  error.value = "";
  try {
    await command("saveContent", { rows: editRows.value });
    editOpen.value = false;
    feedback.toast.success("内容已保存，资源映射保持锁定；执行前将重新预检");
  } catch (e) {
    error.value = e.message;
  } finally {
    busy.value = false;
  }
}
const pendingTask = ref(null);
function taskOp(t, a) {
  if (a === "detail") {try{sessionStorage.setItem('publishing-batch-view:'+t.batchId,JSON.stringify({search:taskSearch.value,filter:taskFilter.value,scroll:window.scrollY}));}catch{}return emit('task',t);}
  if (a === "edit") return emit("edit-task", t);
  if (["abandon", "repair"].includes(a)) {
    pendingTask.value = t;
    op.value = a;
    version.value = batch.value.version || 0;
    reason.value = "";
    error.value = "";
    requestKey.value = crypto.randomUUID();
    opOpen.value = true;
  } else taskAction.value.open(t, a);
}
function operateTask(t, a) {
  current.value = t.batchId;
  taskOp(t, a);
}
function directURL() {
  const route=readOperationRoute(new URL(location.href));
  if(route.page!=='batches'||!route.batchId){detailOpen.value=false;return;}
  const q=new URLSearchParams(location.search),b=props.db.batches.find(x=>x.id===route.batchId);
  if (!b) {
    detailOpen.value = false;
    return;
  }
  open(b, q.get("pending") === "1", false, current.value === b.id);
  if (q.get("edit") === "content") edit(b);
}
onMounted(() => {
  window.addEventListener("popstate", directURL);
  nextTick(directURL);
});
onBeforeUnmount(() => window.removeEventListener("popstate", directURL));
defineExpose({ open, action, edit, directURL, operateTask });
</script>
<template>
  <ResourceCard v-if="active" v-show="!detailOpen" class="batch-v12-list resource-list-panel">
    <ResourceFilterBar>
      <label class="wt-filter-field wt-filter-field--wide"
        ><span class="wt-filter-field__label">综合搜索</span
        ><t-input
          v-model="query"
          aria-label="综合搜索"
          placeholder="批次名称或编号"
          clearable
          @enter="search = query"
      /></label>
      <label class="wt-filter-field"
        ><span class="wt-filter-field__label">平台</span
        ><t-select
          v-model="platform"
          clearable
          placeholder="全部"
          :options="
            ['小红书', 'B站', '百家号'].map((x) => ({ label: x, value: x }))
          "
      /></label>
      <label class="wt-filter-field"
        ><span class="wt-filter-field__label">批次状态</span
        ><t-select
          v-model="stateFilter"
          clearable
          placeholder="全部"
          :options="states.map((x) => ({ label: x, value: x }))"
      /></label>
      <div class="wt-filter-actions">
        <t-button @click="search = query">查询</t-button
        ><t-button
          variant="outline"
          theme="default"
          @click="
            query = '';
            search = '';
            stateFilter = '';
            platform = '';
          "
          >重置</t-button
        >
      </div>
    </ResourceFilterBar>
    <t-table
      class="resource-data-table"
      row-key="id"
      :data="records"
      :columns="columns"
      table-layout="fixed"
      :pagination="{
        defaultCurrent: 1,
        defaultPageSize: 10,
        pageSizeOptions: [10, 20, 50],
        total: records.length,
        showJumper: false,
      }"
      :hover="true"
    >
      <template #id="{ row }"><ResourceId :value="row.id" label="批次ID" /></template>
      <template #identity="{ row }"><ResourceNameLink class="batch-name" :title="row.name" @click="open(row)">{{row.name}}</ResourceNameLink></template>
      <template #plan="{ row }"
        ><span class="batch-cell-text" :title="plan(row)?.name || '独立批次'">{{
          plan(row)?.name || "独立批次"
        }}</span
        ><small v-if="row.sourceBatchId"
          >复制自 {{ row.sourceBatchId }}</small
        ></template
      >
      <template #target="{ row }"
        ><strong class="batch-target-value"
          >{{ batchMetrics(db, row).target }} <span>条</span></strong
        ><small class="batch-cell-text">{{
          (row.targets || [])
            .map((t) => `${t.category} ${t.count}`)
            .join(" · ") || "待配置目标"
        }}</small></template
      >
      <template #progress="{ row }">
        <span v-if="!batchMetrics(db, row).total">{{
          row.preparationOnly ? `${row.candidates?.length || 0} 条候选 · 0 条任务` : batchState(db, row) === "草稿" ? "配置中" : "未创建任务"
        }}</span>
        <template v-else
          ><div class="batch-progress-title">
            <span>{{ batchMetrics(db, row).published }} / {{ batchMetrics(db, row).target }}</span><span>{{ batchMetrics(db, row).progress }}%</span>
          </div>
          <t-progress
            color="var(--wt-color-primary)"
            :percentage="batchMetrics(db, row).progress"
            :label="false"
            :stroke-width="6"
          /></template
        ></template
      >
      <template #state="{ row }"
        ><t-tag
          :theme="
            batchState(db, row) === '待处理'
              ? 'warning'
              : batchState(db, row) === '执行中'
                ? 'primary'
                : 'default'
          "
          variant="light"
          >{{ batchState(db, row) }}</t-tag
        ><small v-if="batchState(db, row) === '已结束' && !batchMetrics(db, row).published">未发布</small><small v-else-if="row.stoppedAt">已停止发布</small></template
      >
      <template #pending="{ row }"
        ><button
          v-if="batchMetrics(db, row).pending"
          class="batch-link"
          @click="open(row, true)"
        >
          {{ batchMetrics(db, row).pending }} 项</button
        ><span v-else>—</span></template
      >
      <template #updated="{ row }"><span>{{formatDateTime(row.updatedAt || row.created)}}</span></template>
      <template #operation="{ row }"
        ><ResourceRowActions
          ><t-button
            v-for="(a, i) in actionsFor(row)"
            :key="a"
            variant="text"
            :class="{
              'is-primary': i === 0,
              'is-danger': ['cancel', 'stop'].includes(a),
            }"
            @click="action(row, a)"
            >{{ row.preparationOnly && a === "configure" ? "编辑配置" : labels[a] }}</t-button
          ></ResourceRowActions
        ></template
      >
      <template #empty
        ><ResourceEmptyState
          title="没有符合条件的批次"
          :description="preparationOnly ? '保存的批次配置将在这里显示。' : '调整搜索条件，或使用右上角“批量发布”创建批次。'"
      /></template>
    </t-table>
    <p v-if="narrow" class="batch-scroll-note">
      左右滑动表格，查看完整进度与操作。
    </p>
  </ResourceCard>
  <ResourceDetailPage
    v-if="active && detailOpen && batch"
    title="发布批次详情"
    back-label="返回发布批次列表"
    @back="returnToList"
  >
    <BatchDetailContent :db="db" :batch="batch" v-model:search="taskSearch" v-model:filter="taskFilter" @task="emit('task',$event)" @task-action="taskOp" />
    <template v-if="detailActions.length" #footer>
      <t-button
        v-for="a in [...detailActions].reverse()"
        :key="a"
        :class="{ 'is-danger': ['cancel', 'stop'].includes(a) }"
        :theme="a === detailActions[0] && !['cancel', 'stop'].includes(a) ? 'primary' : 'default'"
        :variant="['cancel', 'stop'].includes(a) ? 'text' : a === detailActions[0] ? 'base' : 'outline'"
        @click="action(batch, a)"
      >{{ labels[a] }}</t-button>
    </template>
  </ResourceDetailPage>
  <Overlay
    v-model:visible="opOpen"
    :title="
      op === 'abandon'
        ? '放弃任务'
        : op === 'repair'
          ? '恢复原资源'
          : op === 'stop'
            ? '停止发布'
            : labels[op] + '批次'
    "
    :width="640"
    :busy="busy"
  >
    <template v-if="batch"
      ><div class="batch-dialog-identity">
        <h3>{{ batch.name }}</h3>
        <small>{{ batch.id }} · {{ batch.platform }}</small>
      </div>
      <t-alert v-if="error" theme="warning" :message="error" />
      <template v-if="op === 'execute'"
        ><div class="batch-dialog-counts">
          <div>
            <strong>{{ rows.length }}</strong><small>任务总数</small>
          </div>
          <div>
            <strong>{{ usable.length }}</strong><small>可执行</small>
          </div>
          <div>
            <strong>{{ blocked.length }}</strong><small>异常</small>
          </div>
        </div>
        <p>
          系统成功接受后，批次结构与资源映射锁定。尚未领取的待执行任务仍可在发布任务页单独编辑内容。
        </p>
        <t-alert
          v-if="!usable.length"
          theme="warning"
          message="全部任务不可执行，启动已阻止。请先恢复原资源或处理异常。"
        />
        <ul v-if="blocked.length" class="batch-blocked">
          <li v-for="x in blocked" :key="x.task.id">
            T-{{ x.task.id }}：{{ x.errors.join("；") }}
          </li>
        </ul>
        <t-checkbox v-if="blocked.length && usable.length" v-model="partial"
          >我确认仅执行
          {{ usable.length }} 条可用任务，保留异常任务待处理</t-checkbox
        ><t-checkbox
          v-if="plan(batch) && !batch.sourcePlanId"
          v-model="takeover"
          >确认人工接管计划当前批次并锁定计划结构</t-checkbox
        ></template
      >
      <template v-else-if="op === 'stop'"
        ><p>只停止尚未启动的任务，不撤销已发布作品，不强行终止在途任务。</p>
        <p>
          <b>停止后不可重新启动，也不能创建新的重试执行。</b
          >在途和结果不明确的任务仍需完成核实或处置。
        </p></template
      >
      <template v-else-if="op === 'cancel'"
        ><p>
          批次尚未执行。终止后原批次不可恢复，已配置的任务一并取消；可从终态批次复制新建。
        </p>
        <label class="batch-form-field"
          >终止原因<t-textarea
            v-model="reason"
            :maxlength="500" /></label
      ></template>
      <template v-else-if="op === 'repair'"
        ><p>仅恢复原账号和原成片的演示可用性，不替换账号、成片或映射。</p>
        <p>
          原账号：{{ account(pendingTask?.accountId) }} · 原成片：V-{{
            pendingTask?.assetId
          }}
        </p>
        <t-alert
          theme="info"
          message="本地原型将模拟原账号登录有效、原文件已恢复，不调用真实账号或外部平台。原资源永久不可用时请放弃任务。"
      /></template>
      <template v-else-if="op === 'abandon'"
        ><p>
          仅用于已确认未发布且原执行已结束，或尚未启动的任务。结果不明确时必须先核实。
        </p>
        <label class="batch-form-field"
          >放弃原因<t-textarea
            v-model="reason"
            placeholder="说明原资源永久不可用等原因"
        /></label>
        <p>此任务不再执行，由新批次补足目标。</p></template
      > </template
    ><template #footer
      ><t-button
        variant="outline"
        theme="default"
        :disabled="busy"
        @click="opOpen = false"
        >返回</t-button
      ><t-button
        :theme="
          ['stop', 'cancel', 'abandon'].includes(op) ? 'danger' : 'primary'
        "
        :loading="busy"
        :disabled="
          op === 'execute' &&
          (!usable.length ||
            (blocked.length && !partial) ||
            (plan(batch) && !batch.sourcePlanId && !takeover))
        "
        @click="submit"
        >{{
          op === "execute"
            ? blocked.length
              ? "仅执行可用任务"
              : "确认执行"
            : op === "abandon"
                ? "确认放弃"
                : op === "repair"
                  ? "确认恢复原资源"
                  : "确认" + labels[op]
        }}</t-button
      ></template
    >
  </Overlay>
  <Overlay
    :visible="editOpen"
    @update:visible="
      (value) => {
        if (!value) return closeEdit();
      }
    "
    title="编辑批次内容"
    kind="drawer"
    page
    back-label="返回发布批次"
    :width="1180"
    :busy="busy || working"
    ><template v-if="batch"
      ><div class="batch-editor-header">
        <div>
          <h2>{{ batch.name }} <ResourceFieldHelp label="批次内容编辑" content="可编辑文案与封面，批次目标和资源映射已锁定。首次执行后内容只读。" /></h2>
          <small
            >{{ batch.id }} · {{ batch.platform }} · {{ rows.length }} 条任务
            · 目标 {{ metrics.target }} 条</small
          >
        </div>
        <t-tag variant="light">仅编辑内容</t-tag>
      </div>
<t-alert
        v-if="error"
        theme="warning"
        :message="error" /><BatchContentWorkspace
        :rows="editRows"
        :db="db"
        @working="working = $event"
        @preview="emit('preview', $event)" /></template
    ><template #footer
      ><t-button
        :loading="busy"
        :disabled="working || !contentEditable(db, batch)"
        @click="saveContent"
        >保存</t-button
      ></template
    ></Overlay
  >
  <TaskActionDialog ref="taskAction" :db="db" :persist="persist" />
</template>
<style scoped>
.batch-v12-list-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 20px 20px 0;
}
.batch-v12-list-heading h2 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  line-height: 24px;
}
.batch-v12-list-heading h2 span {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 24px;
  height: 22px;
  padding: 0 6px;
  margin-left: 8px;
  border-radius: 4px;
  background: var(--wt-color-bg-subtle);
  font-size: 12px;
  font-weight: 400;
  color: var(--wt-color-text-secondary);
}
.batch-list-caption,
.batch-scroll-note {
  font-size: 12px;
  line-height: 20px;
  color: var(--wt-color-text-secondary);
}
.batch-scroll-note {
  margin: 8px 20px 0;
}
.batch-v12-list :deep(.t-table) {
  font-size: 13px;
  line-height: 1.6;
}
.batch-v12-list :deep(.t-table__content table) {
  min-width: 1170px;
}

small {
  display: block;
  color: var(--wt-color-text-secondary);
  font-size: 12px;
  font-weight: 400;
  line-height: 20px;
  margin-top: 4px;
}
.batch-link {
  padding: 0;
  border: 0;
  background: none;
  text-align: left;
  color: var(--wt-color-primary);
  font: inherit;
  cursor: pointer;
  line-height: 1.6;
}
.batch-link:hover {
  text-decoration: underline;
}
.batch-link:focus-visible {
  outline: 2px solid var(--wt-color-primary);
  outline-offset: 3px;
  border-radius: 4px;
}
.batch-name,
.batch-cell-text {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  overflow-wrap: anywhere;
}
.batch-name {
  font-weight: 400;
}
.batch-date {
  display: block;
  white-space: nowrap;
  font-size: 12px;
  line-height: 20px;
}
.batch-date + small {
  margin-top: 0;
}
.batch-target-value {
  font-size: 16px;
  font-weight: 400;
  font-variant-numeric: tabular-nums;
}
.batch-target-value > span {
  font-size: 12px;
  font-weight: 400;
}
.batch-v12-list .batch-progress-title {
  white-space: nowrap;
}
.batch-progress-title {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  font-size: 13px;
  line-height: 22px;
  margin: 0 0 6px;
  font-variant-numeric: tabular-nums;
}
.batch-progress-title span {
  white-space: nowrap;
  color: var(--wt-color-text-primary);
}
.batch-progress-title span:last-child { color: var(--wt-color-text-secondary); }
.batch-progress-column-title { display: inline-flex; align-items: center; gap: 4px; }
.batch-metric-label { display: inline-flex; align-items: center; gap: 4px; }
.batch-progress-grid { display: grid; grid-template-columns: minmax(0, 1fr); gap: 20px; }
.batch-dialog-counts { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 16px; margin-bottom: 18px; }
.batch-dialog-counts strong { font-size: 24px; line-height: 32px; font-weight: 400; font-variant-numeric: tabular-nums; }
@media (max-width: 680px) { .batch-progress-grid { grid-template-columns: 1fr; gap: 16px; } }
.batch-form-field {
  display: grid;
  gap: 8px;
  margin: 20px 0;
  font-size: 13px;
  font-weight: 400;
}
.batch-blocked {
  max-height: 200px;
  overflow: auto;
  border: 1px solid var(--wt-color-border);
  border-radius: 4px;
  background: var(--wt-color-bg-subtle);
  padding: 12px 12px 12px 30px;
  font-size: 13px;
  line-height: 1.8;
}
.batch-dialog-identity {
  padding-bottom: 16px;
  border-bottom: 1px solid var(--wt-color-border);
  margin-bottom: 20px;
}
.batch-dialog-identity h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 400;
  line-height: 26px;
}
.batch-dialog-counts {
  padding: 16px 20px;
  background: var(--wt-color-bg-subtle);
  border: 1px solid var(--wt-color-border);
  border-radius: 6px;
}
.batch-editor-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 20px;
}
.batch-editor-header h2 {
  font-size: 18px;
  font-weight: 400;
  line-height: 28px;
  margin: 0;
}
@media (max-width: 900px) {
  .batch-list-caption {
    display: none;
  }
}
@media (max-width: 540px) {
  .batch-v12-list-heading {
    padding: 16px 20px 0;
  }
  .batch-dialog-counts {
    gap: 12px;
  }
}
.batch-editor-header h2 {display:flex;align-items:center;gap:8px;margin:0;font-size:16px;line-height:24px;font-weight:500;}
</style>
