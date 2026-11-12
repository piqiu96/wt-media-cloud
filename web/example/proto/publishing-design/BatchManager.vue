<script setup>
import { computed, ref, nextTick, onMounted, onBeforeUnmount } from "vue";
import { useMediaQuery } from "../../../src/shared/composables/useMediaQuery.js";
import ResourceCard from "../../../src/shared/ui/resource/ResourceCard.vue";
import ResourceFilterBar from "../../../src/shared/ui/resource/ResourceFilterBar.vue";
import ResourceRowActions from "../../../src/shared/ui/resource/ResourceRowActions.vue";
import ResourceEmptyState from "../../../src/shared/ui/resource/ResourceEmptyState.vue";
import Overlay from "./PrototypeOverlay.vue";
import RequiredLabel from "./RequiredLabel.vue";
import BatchContentWorkspace from "./BatchContentWorkspace.vue";
import TaskActionDialog from "./TaskActionDialog.vue";
import { feedback } from "../../../src/shared/ui/feedback/index.js";
import {
  batchState,
  batchMetrics,
  batchActions,
  rowsFor,
  executionPreview,
  contentEditable,
  executionChecks,
  batchTaskActions,
} from "./batchLifecycle.js";
import { batchCommand } from "./batchApi.js";
const props = defineProps({ db: Object, persist: Function, active: Boolean });
const emit = defineEmits(["configure", "task", "preview"]);
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
  edit: "编辑内容",
  detail: "详情",
  process: "去处理",
  stop: "停止发布",
  cancel: "取消",
  copy: "复制",
};
const states = ["草稿", "待执行", "执行中", "待处理", "已结束", "已取消"];
const batch = computed(() =>
  props.db.batches.find((b) => b.id === current.value),
);
const metrics = computed(() =>
  batch.value ? batchMetrics(props.db, batch.value) : {},
);
const records = computed(() =>
  props.db.batches
    .filter(
      (b) =>
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
const tasks = computed(() =>
  rows.value.filter(
    (t) =>
      (!taskSearch.value ||
        `${t.id} ${t.title} ${account(t.accountId)}`.includes(
          taskSearch.value,
        )) &&
      (!taskFilter.value ||
        (taskFilter.value === "待处理"
          ? [
              "执行失败",
              "结果待确认",
              "等待人工",
              "平台审核中",
              "待补链接",
              "待执行",
            ].includes(t.status)
          : t.status === taskFilter.value)),
  ),
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
const asset = (id) =>
  props.db.assets.find((a) => a.id === id)?.title || "原成片不存在";
const date = (v) =>
  v ? new Date(v).toLocaleString("zh-CN", { hour12: false }) : "暂无更新";
const dateParts = (v) =>
  v
    ? [
        new Date(v).toLocaleDateString("zh-CN"),
        new Date(v).toLocaleTimeString("zh-CN", { hour12: false }),
      ]
    : ["暂无更新", ""];
const columns = computed(() => [
  { colKey: "identity", title: "批次名称 / 编号", width: 220 },
  { colKey: "plan", title: "所属计划", width: 125 },
  { colKey: "target", title: "发布目标", width: 125 },
  { colKey: "progress", title: "执行进度", width: 172 },
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
const taskColumns = [
  { colKey: "identity", title: "任务 / 发布内容", minWidth: 220 },
  { colKey: "resource", title: "账号 / 成片", minWidth: 175 },
  { colKey: "status", title: "任务状态", width: 115 },
  { colKey: "issue", title: "异常原因", minWidth: 200 },
  { colKey: "operation", title: "操作", minWidth: 280 },
];
function open(b, process = false) {
  current.value = b.id;
  detailOpen.value = true;
  taskFilter.value = process ? "待处理" : "";
  taskSearch.value = "";
  error.value = "";
}
async function action(b, a) {
  current.value = b.id;
  error.value = "";
  if (a === "detail" || a === "process") return open(b, a === "process");
  if (a === "configure") return emit("configure", b);
  if (a === "edit") return edit(b);
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
        execute: "批次执行请求已接受，所有发布配置永久锁定",
        stop: "已停止未启动任务；在途任务继续回传结果",
        cancel: "批次已取消",
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
function taskActions(t) {
  return batchTaskActions(props.db, batch.value, t);
}
const taskLabels = {
  supplement: "补录公开结果",
  start: "执行任务",
  verify: "核实结果",
  human: "去处理",
  retry: "重试",
  cancel: "取消任务",
  abandon: "放弃任务",
  repair: "恢复原资源",
};
const history = computed(() =>
  batch.value
    ? [
        ...(batch.value.log || []),
        ...rows.value.flatMap((t) =>
          (t.log || []).map((e) => ({
            ...e,
            type: `任务 T-${t.id}`,
            text: e.text,
            actor: e.actor || "演示执行器",
          })),
        ),
      ]
        .sort((a, b) => String(b.time).localeCompare(String(a.time)))
        .map((entry, index) => ({
          ...entry,
          rowKey: `${batch.value.id}-${index}`,
        }))
    : [],
);
function directURL() {
  const raw = location.hash.slice(1),
    [section, params] = raw.split("?");
  if (section !== "batches" || !params) return;
  const q = new URLSearchParams(params),
    b = props.db.batches.find((x) => x.id === Number(q.get("batch")));
  if (!b) return;
  open(b);
  if (q.get("edit") === "content") edit(b);
}
onMounted(() => {
  window.addEventListener("hashchange", directURL);
  nextTick(directURL);
});
onBeforeUnmount(() => window.removeEventListener("hashchange", directURL));
defineExpose({ open, action, edit, directURL });
</script>
<template>
  <ResourceCard v-if="active" class="batch-v12-list">
    <div class="batch-v12-list-heading">
      <h2>
        全部批次 <span>{{ db.batches.length }}</span>
      </h2>
      <span class="batch-list-caption">按批次跟进配置、执行与处理结果</span>
    </div>
    <ResourceFilterBar>
      <label class="wt-filter-field wt-filter-field--wide"
        ><span class="wt-filter-field__label">搜索批次</span
        ><t-input
          v-model="query"
          aria-label="搜索批次"
          placeholder="批次名称或编号"
          clearable
          @enter="search = query"
      /></label>
      <label class="wt-filter-field"
        ><span class="wt-filter-field__label">平台</span
        ><t-select
          v-model="platform"
          clearable
          placeholder="全部平台"
          :options="
            ['小红书', 'B站', '百家号'].map((x) => ({ label: x, value: x }))
          "
      /></label>
      <label class="wt-filter-field"
        ><span class="wt-filter-field__label">批次状态</span
        ><t-select
          v-model="stateFilter"
          clearable
          placeholder="全部状态"
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
      row-key="id"
      :data="records"
      :columns="columns"
      table-layout="fixed"
      :pagination="{
        defaultCurrent: 1,
        defaultPageSize: 8,
        pageSizeOptions: [8, 16, 32],
        total: records.length,
        showJumper: false,
      }"
      :hover="true"
    >
      <template #identity="{ row }"
        ><button
          class="batch-link batch-name"
          :title="row.name"
          @click="open(row)"
        >
          {{ row.name }}</button
        ><small>B-{{ row.id }} · {{ row.platform }}</small></template
      >
      <template #plan="{ row }"
        ><span class="batch-cell-text" :title="plan(row)?.name || '独立批次'">{{
          plan(row)?.name || "独立批次"
        }}</span
        ><small v-if="row.sourceBatchId"
          >复制自 B-{{ row.sourceBatchId }}</small
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
          batchState(db, row) === "草稿" ? "配置中" : "未创建任务"
        }}</span>
        <template v-else
          ><div class="batch-progress-title">
            已处置 {{ batchMetrics(db, row).disposed }} /
            {{ batchMetrics(db, row).total
            }}<span>{{ batchMetrics(db, row).progress }}%</span>
          </div>
          <t-progress
            color="var(--wt-color-primary)"
            :percentage="batchMetrics(db, row).progress"
            :label="false"
            size="small"
          /><small
            >已发布 {{ batchMetrics(db, row).published }} · 达成
            {{ batchMetrics(db, row).achievement }}%</small
          ></template
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
        ><small v-if="batchState(db, row) === '已结束'">{{
          batchMetrics(db, row).result
        }}</small
        ><small v-if="row.stoppedAt">已停止发布</small></template
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
      <template #updated="{ row }"
        ><span class="batch-date">{{
          dateParts(row.updatedAt || row.created)[0]
        }}</span
        ><small class="batch-date">{{
          dateParts(row.updatedAt || row.created)[1]
        }}</small></template
      >
      <template #operation="{ row }"
        ><ResourceRowActions
          ><t-button
            v-for="(a, i) in batchActions(db, row)"
            :key="a"
            variant="text"
            :class="{
              'is-primary': i === 0,
              'is-danger': ['cancel', 'stop'].includes(a),
            }"
            @click="action(row, a)"
            >{{ labels[a] }}</t-button
          ></ResourceRowActions
        ></template
      >
      <template #empty
        ><ResourceEmptyState
          title="没有符合条件的批次"
          description="调整搜索条件，或使用右上角“批量发布”创建批次。"
      /></template>
    </t-table>
    <p v-if="narrow" class="batch-scroll-note">
      左右滑动表格，查看完整进度与操作。
    </p>
  </ResourceCard>
  <Overlay
    v-model:visible="detailOpen"
    title="批次详情"
    kind="drawer"
    :width="1180"
    workspace
  >
    <div v-if="batch" class="batch-v12-detail">
      <section aria-labelledby="batch-info">
        <div class="batch-section-title">
          <h2 id="batch-info"><span>01</span> 批次信息</h2>
          <ResourceRowActions
            ><t-button
              v-for="(a, i) in batchActions(db, batch).filter(
                (x) => !['detail', 'process'].includes(x),
              )"
              :key="a"
              variant="text"
              :class="{
                'is-primary': i === 0,
                'is-danger': ['cancel', 'stop'].includes(a),
              }"
              @click="action(batch, a)"
              >{{ labels[a] }}</t-button
            ></ResourceRowActions
          >
        </div>
        <div class="batch-overview">
          <div>
            <h3>
              {{ batch.name }}
              <t-tag variant="light">{{ batchState(db, batch) }}</t-tag>
            </h3>
            <p>
              B-{{ batch.id }} · {{ batch.platform }} ·
              {{ plan(batch)?.name || "独立批次" }}
            </p>
            <p>
              {{
                (batch.targets || [])
                  .map((t) => `${t.category} ${t.count} 条`)
                  .join(" / ")
              }}
            </p>
            <small
              >创建人 {{ batch.createdBy || "历史记录未提供" }} · 创建
              {{ date(batch.created) }} · 最近更新
              {{ date(batch.updatedAt || batch.created) }}</small
            >
            <p v-if="batchState(db, batch) === '已结束'">
              发布结果：<b>{{ metrics.result }}</b>
            </p>
            <p v-if="batch.stoppedAt">
              已停止发布。未启动任务已取消，在途任务继续回传结果；不能重新启动。
            </p>
          </div>
          <div class="batch-progress-panel">
            <div class="batch-counts">
              <div>
                <strong>{{ metrics.target }}</strong><small>发布目标</small>
              </div>
              <div>
                <strong>{{ metrics.published }}</strong><small>已发布</small>
              </div>
              <div>
                <strong>{{ metrics.pending }}</strong><small>待处理</small>
              </div>
            </div>
            <div class="batch-progress-title">
              已处置进度
              <span
                >{{ metrics.disposed }} / {{ metrics.total }} ·
                {{ metrics.progress }}%</span
              >
            </div>
            <t-progress
              :percentage="metrics.progress"
              color="var(--wt-color-primary)"
              :label="false"
              size="small"
            />
            <div class="batch-progress-title">
              发布达成率
              <span
                >{{ metrics.published }} / {{ metrics.target }} ·
                {{ metrics.achievement }}%</span
              >
            </div>
            <t-progress
              :percentage="metrics.achievement"
              :label="false"
              size="small"
            /><small>已处置仅包含已发布、已取消和已放弃。</small>
          </div>
        </div>
      </section>
      <section aria-labelledby="batch-tasks">
        <div class="batch-section-title">
          <h2 id="batch-tasks">
            <span>02</span> 批次任务 <small>{{ rows.length }} 条</small>
          </h2>
        </div>
        <t-alert
          v-if="batch.firstStartedAt"
          theme="info"
          message="首次执行后，全部发布配置永久只读。资源异常仅可恢复原资源；永久不可用请取消或放弃任务，再用新批次补足。"
        />
        <ResourceFilterBar
          ><label class="wt-filter-field wt-filter-field--wide"
            ><span class="wt-filter-field__label">搜索任务</span
            ><t-input
              v-model="taskSearch"
              placeholder="标题、账号或任务编号"
              clearable /></label
          ><label class="wt-filter-field"
            ><span class="wt-filter-field__label">任务状态</span
            ><t-select
              v-model="taskFilter"
              clearable
              placeholder="全部任务"
              :options="
                [
                  '待处理',
                  '待执行',
                  '排队中',
                  '执行中',
                  '等待人工',
                  '结果待确认',
                  '平台审核中',
                  '待补链接',
                  '执行失败',
                  '已发布',
                  '已取消',
                  '已放弃',
                ].map((x) => ({ label: x, value: x }))
              " /></label
        ></ResourceFilterBar>
        <t-table
          :key="`tasks-${current}-${tasks.length > 8}`"
          row-key="id"
          :data="tasks"
          :columns="taskColumns"
          :pagination="
            tasks.length > 8
              ? {
                  defaultCurrent: 1,
                  defaultPageSize: 8,
                  total: tasks.length,
                  showJumper: false,
                }
              : undefined
          "
        >
          <template #identity="{ row }"
            ><div class="batch-task-identity">
              <img
                v-if="db.assets.find((v) => v.id === row.assetId)?.cover"
                :src="db.assets.find((v) => v.id === row.assetId).cover"
                alt="成片封面"
              />
              <div>
                <button
                  class="batch-link batch-name"
                  :title="row.title"
                  @click="emit('task', row)"
                >
                  {{ row.title || "未配置标题" }}</button><small
                  >T-{{ row.id }} · {{ row.category }}</small
                >
              </div>
            </div></template
          >
          <template #resource="{ row }"
            >{{ account(row.accountId)
            }}<small
              >V-{{ row.assetId }} · {{ asset(row.assetId) }}</small
            ></template
          >
          <template #status="{ row }"
            ><t-tag
              variant="light"
              :theme="
                ['结果待确认', '执行失败', '等待人工'].includes(row.status)
                  ? 'warning'
                  : 'default'
              "
              >{{ row.status === "执行失败" ? "发布失败" : row.status }}</t-tag
            ></template
          >
          <template #issue="{ row }"
            ><span>{{
              ["执行失败", "结果待确认", "等待人工"].includes(row.status)
                ? row.evidence || "缺少执行依据，请在任务详情中核实原尝试"
                : row.status === "平台审核中"
                  ? "平台已接收，等待公开结果，不计入已发布"
                  : row.status === "待执行"
                    ? executionChecks(row, db).join("；") ||
                      (batch.firstStartedAt ? "本次未执行，可单条处理" : "—")
                    : "—"
            }}</span></template
          >
          <template #operation="{ row }"
            ><ResourceRowActions
              ><t-button variant="text" @click="emit('task', row)"
                >详情</t-button
              ><t-button
                v-for="a in taskActions(row)"
                :key="a"
                variant="text"
                @click="taskOp(row, a)"
                >{{ taskLabels[a] }}</t-button
              ></ResourceRowActions
            ></template
          >
          <template #empty
            ><ResourceEmptyState
              :title="rows.length ? '没有匹配的任务' : '草稿尚未创建任务'"
              :description="
                rows.length
                  ? '调整搜索或状态筛选。'
                  : '继续配置并完成预检后生成任务。'
              "
          /></template>
        </t-table>
      </section>
      <section aria-labelledby="batch-snapshot">
        <div class="batch-section-title">
          <h2 id="batch-snapshot"><span>03</span> 批次快照</h2>
          <small>{{
            batch.firstStartedAt
              ? "冻结于 " + date(batch.firstStartedAt)
              : "首次执行时冻结；当前为配置预览"
          }}</small>
        </div>
        <t-alert
          v-if="batch.legacySnapshot"
          theme="warning"
          message="历史批次没有保存原始执行快照，以下为迁移时保留的配置，不能作为原执行时完整证据。"
        />
        <div class="batch-snapshot-meta">
          <div>
            <small>发布目标</small
            >{{
              (batch.frozenSnapshot?.targets || batch.targets || [])
                .map((t) => `${t.category} ${t.count} 条`)
                .join(" / ") || "待配置"
            }}
          </div>
          <div>
            <small>资源规则</small
            >{{
              batch.firstStartedAt
                ? "原账号与原成片永久锁定"
                : "进入内容配置后资源锁定"
            }}
          </div>
          <div>
            <small>内容规则</small
            >{{
              batch.control === "已取消"
                ? "批次已取消，内容不可编辑"
                : batch.firstStartedAt
                  ? "全部发布配置永久只读"
                  : "仅首次执行前允许编辑内容"
            }}
          </div>
        </div>
        <p
          v-if="batch.frozenSnapshot?.rules?.accountOrder?.length"
          class="batch-rule-text"
        >
          账号顺序：{{
            batch.frozenSnapshot.rules.accountOrder.map(account).join(" → ")
          }}
        </p>
        <p
          v-if="
            batch.frozenSnapshot?.rules?.accountLimits &&
            Object.keys(batch.frozenSnapshot.rules.accountLimits).length
          "
          class="batch-rule-text"
        >
          账号上限：{{
            Object.entries(batch.frozenSnapshot.rules.accountLimits)
              .map(([id, limit]) => `${account(Number(id))} ${limit} 条`)
              .join(" / ")
          }}
        </p>
        <t-collapse
          ><t-collapse-panel value="mapping" header="查看账号成片映射与内容配置"
            ><div
              v-for="t in batch.frozenSnapshot?.tasks || rows"
              :key="t.id"
              class="batch-frozen-row"
            >
              <b
                >T-{{ t.id }} · {{ account(t.accountId) }} → V-{{
                  t.assetId
                }}</b
              >
              <p>{{ t.title || "未配置标题" }}</p>
              <small
                >{{ t.description || "未设置简介" }} · 标签：{{
                  t.tags || t.topics || "未设置"
                }}
                · {{ t.declaration || "未设置声明" }}</small
              ><button
                v-if="t.cover"
                class="batch-cover"
                @click="
                  emit('preview', { kind: 'cover', row: t, readOnly: true })
                "
              >
                <img :src="t.cover" alt="冻结封面" />
              </button></div></t-collapse-panel
        ></t-collapse>
      </section>
      <section aria-labelledby="batch-history">
        <div class="batch-section-title">
          <h2 id="batch-history"><span>04</span> 执行记录</h2>
          <small>按操作时间倒序</small>
        </div>
        <t-table
          :key="`history-${current}-${history.length > 8}`"
          row-key="rowKey"
          :data="history"
          :columns="[
            { colKey: 'time', title: '操作时间', width: 180 },
            { colKey: 'actor', title: '操作者', width: 130 },
            { colKey: 'type', title: '执行事件', width: 150 },
            { colKey: 'text', title: '结果与异常记录', minWidth: 250 },
          ]"
          :pagination="
            history.length > 8
              ? {
                  defaultCurrent: 1,
                  defaultPageSize: 8,
                  total: history.length,
                  showJumper: false,
                }
              : undefined
          "
          ><template #time="{ row }">{{ date(row.time) }}</template
          ><template #text="{ row }"
            >{{ row.text
            }}<small v-if="row.beforeState"
              >{{ row.beforeState }} → {{ row.afterState }} ·
              {{ row.result || "已记录" }}</small
            ></template
          ><template #empty
            ><ResourceEmptyState
              title="暂无执行记录"
              description="配置保存、执行与异常处理后会记录操作。" /></template
        ></t-table>
      </section>
    </div>
    <template #footer
      ><t-button theme="default" variant="outline" @click="detailOpen = false"
        >关闭</t-button
      ></template
    >
  </Overlay>
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
        <small>B-{{ batch.id }} · {{ batch.platform }}</small>
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
          系统成功接受后，全部任务的发布配置将永久锁定，包括本次未执行的任务。
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
          批次尚未执行。取消后原批次不可恢复，已配置的任务一并取消；可从终态批次复制为新草稿。
        </p>
        <label class="batch-form-field"
          >取消原因（选填）<t-textarea
            v-model="reason"
            :maxlength="500" /></label
      ></template>
      <template v-else-if="op === 'copy'"
        ><p>
          复制目标、类目及资源策略，生成新草稿。账号和成片将重新匹配，不复制任务、执行结果与记录。
        </p>
        <label class="batch-form-field"
          ><RequiredLabel>新批次名称</RequiredLabel>
          <t-input
            v-model="name"
            aria-label="新批次名称"
            :maxlength="80"
            placeholder="请输入新批次名称" /></label
        ><small
          >来源计划：{{
            sourcePlan(batch)?.name || plan(batch)?.name || "独立批次"
          }}。保留来源，不改变已确认计划的拆批关系。</small
        ></template
      >
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
            : op === "copy"
              ? "创建新草稿"
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
        if (!value) closeEdit();
      }
    "
    title="编辑批次内容"
    kind="drawer"
    workspace
    :width="1180"
    :busy="busy || working"
    ><template v-if="batch"
      ><div class="batch-editor-header">
        <div>
          <h2>{{ batch.name }}</h2>
          <small
            >B-{{ batch.id }} · {{ batch.platform }} · {{ rows.length }} 条任务
            · 目标 {{ metrics.target }} 条</small
          >
        </div>
        <t-tag variant="light">仅编辑内容</t-tag>
      </div>
      <t-alert
        theme="info"
        message="可编辑文案与封面；批次目标和资源映射已锁定。首次执行后全部内容永久只读。" /><t-alert
        v-if="error"
        theme="warning"
        :message="error" /><BatchContentWorkspace
        :rows="editRows"
        :db="db"
        @working="working = $event"
        @preview="emit('preview', $event)" /></template
    ><template #footer
      ><t-button
        theme="default"
        variant="outline"
        :disabled="busy || working"
        @click="closeEdit"
        >返回</t-button
      ><t-button
        :loading="busy"
        :disabled="working || !contentEditable(db, batch)"
        @click="saveContent"
        >保存内容</t-button
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
  padding: 20px 24px 0;
}
.batch-v12-list-heading h2 {
  margin: 0;
  font-size: 16px;
  font-weight: 500;
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
.batch-v12-list :deep(.t-table th) {
  height: 44px;
  padding: 10px 16px;
  white-space: nowrap;
  font-size: 13px;
  font-weight: 400;
  color: var(--wt-color-text-secondary);
  background: var(--wt-color-bg-subtle);
}
.batch-v12-list :deep(.t-table td) {
  padding: 12px 16px;
  height: 78px;
  box-sizing: border-box;
  vertical-align: middle;
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
.batch-link:focus-visible,
.batch-cover:focus-visible {
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
  font-weight: 500;
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
  font-weight: 500;
  font-variant-numeric: tabular-nums;
}
.batch-target-value > span {
  font-size: 12px;
  font-weight: 400;
}
.batch-v12-list .batch-progress-title,
.batch-v12-list .batch-progress-title + .t-progress + small {
  white-space: nowrap;
}
.batch-progress-title {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  font-size: 12px;
  line-height: 20px;
  margin: 0 0 6px;
  font-variant-numeric: tabular-nums;
}
.batch-progress-title span {
  white-space: nowrap;
  color: var(--wt-color-text-secondary);
}
.batch-v12-detail {
  display: flex;
  flex-direction: column;
  gap: 28px;
  max-width: 1120px;
  margin-inline: auto;
  color: var(--wt-color-text-primary);
  font-size: 13px;
}
.batch-v12-detail > section {
  padding: 0 0 28px;
  border-bottom: 1px solid var(--wt-color-border);
  min-width: 0;
}
.batch-v12-detail > section:last-child {
  padding-bottom: 0;
  border: 0;
}
.batch-section-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 20px;
}
.batch-section-title h2 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  line-height: 24px;
  display: flex;
  align-items: center;
  gap: 10px;
}
.batch-section-title h2 > span {
  display: grid;
  place-items: center;
  width: 24px;
  height: 24px;
  font-size: 12px;
  font-weight: 500;
  border-radius: 4px;
  color: var(--wt-color-primary);
  background: var(--wt-color-primary-light);
}
.batch-section-title h2 small {
  display: inline;
  margin: 0;
}
.batch-overview {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 340px;
  gap: 32px;
}
.batch-overview h3 {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  margin: 0 0 12px;
  font-size: 18px;
  font-weight: 500;
  line-height: 28px;
  overflow-wrap: anywhere;
}
.batch-overview p {
  line-height: 1.7;
  font-size: 13px;
  color: var(--wt-color-text-secondary);
  margin: 8px 0;
}
.batch-overview > div {
  min-width: 0;
}
.batch-progress-panel {
  padding: 16px 20px;
  background: var(--wt-color-bg-subtle);
  border: 1px solid var(--wt-color-border);
  border-radius: 6px;
}
.batch-progress-panel > .batch-progress-title:not(:first-child) {
  margin-top: 14px;
}
.batch-counts,
.batch-dialog-counts {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
  margin-bottom: 18px;
}
.batch-counts strong,
.batch-dialog-counts strong {
  font-size: 24px;
  line-height: 32px;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
}
.batch-v12-detail :deep(.t-table) {
  font-size: 13px;
  line-height: 1.7;
}
.batch-v12-detail :deep(.t-table td) {
  padding: 12px 16px;
  vertical-align: middle;
}
.batch-v12-detail :deep(.t-table th) {
  background: var(--wt-color-bg-subtle);
  font-weight: 400;
  color: var(--wt-color-text-secondary);
  white-space: nowrap;
}
.batch-task-identity {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}
.batch-task-identity > img {
  width: 38px;
  height: 50px;
  object-fit: cover;
  flex: none;
  border-radius: 4px;
  background: var(--wt-color-bg-subtle);
}
.batch-task-identity > div {
  min-width: 0;
}
.batch-snapshot-meta {
  display: grid;
  grid-template-columns: 1.2fr 1fr 1fr;
  gap: 20px;
  margin-bottom: 20px;
  font-size: 13px;
  padding: 16px 20px;
  background: var(--wt-color-bg-subtle);
  border-radius: 6px;
}
.batch-frozen-row {
  padding: 16px 0;
  border-bottom: 1px solid var(--wt-color-border);
  font-size: 13px;
  line-height: 1.7;
}
.batch-frozen-row b {
  font-weight: 500;
}
.batch-cover {
  border: 0;
  background: none;
  padding: 0;
  margin-top: 10px;
  cursor: zoom-in;
}
.batch-cover img {
  width: 140px;
  max-height: 100px;
  object-fit: contain;
  border-radius: 4px;
}
.batch-form-field {
  display: grid;
  gap: 8px;
  margin: 20px 0;
  font-size: 13px;
  font-weight: 500;
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
  font-weight: 500;
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
  font-weight: 500;
  line-height: 28px;
  margin: 0;
}
.batch-v12-detail :deep(.t-alert) {
  margin-bottom: 20px;
  font-size: 13px;
  line-height: 1.7;
}
@media (max-width: 900px) {
  .batch-overview {
    grid-template-columns: minmax(0, 1fr);
    gap: 20px;
  }
  .batch-snapshot-meta {
    grid-template-columns: minmax(0, 1fr);
    gap: 16px;
  }
  .batch-list-caption {
    display: none;
  }
  .batch-v12-detail {
    gap: 24px;
  }
  .batch-v12-detail > section {
    padding-bottom: 24px;
  }
}
@media (max-width: 540px) {
  .batch-v12-list-heading {
    padding: 16px 20px 0;
  }
  .batch-progress-panel {
    padding: 16px;
  }
  .batch-counts,
  .batch-dialog-counts {
    gap: 12px;
  }
}
</style>
