<script setup>
import { computed, ref, watch, onBeforeUnmount, nextTick } from "vue";
import {
  useRoute,
  useRouter,
  onBeforeRouteLeave,
  onBeforeRouteUpdate,
} from "vue-router";
import ResourceDetailPage from "../../../shared/ui/resource/ResourceDetailPage.vue";
import ResourceEditorPage from "../../../shared/ui/resource/ResourceEditorPage.vue";
import ResourceCard from "../../../shared/ui/resource/ResourceCard.vue";
import ResourceSection from "../../../shared/ui/resource/ResourceSection.vue";
import ResourceDescriptions from "../../../shared/ui/resource/ResourceDescriptions.vue";
import ResourceOverlay from "../../../shared/ui/resource/ResourceOverlay.vue";
import ResourceStatGrid from "../../../shared/ui/resource/ResourceStatGrid.vue";
import ResourceStatusBadge from "../../../shared/ui/resource/ResourceStatusBadge.vue";
import ResourceNotice from "../../../shared/ui/resource/ResourceNotice.vue";
import ResourceRowActions from "../../../shared/ui/resource/ResourceRowActions.vue";
import ResourceProgress from "../../../shared/ui/resource/ResourceProgress.vue";
import FeedbackAlert from "../../../shared/ui/feedback/FeedbackAlert.vue";
import { feedback } from "../../../shared/ui/feedback/index.js";
import { createPublicationPlanClient } from "../../../shared/api/publicationPlan.js";
import { createPublicationBatchClient } from "../../../shared/api/publication.js";
import { createPublicationDraftClient } from "../../../shared/api/publicationDraft.js";
import { createSessionClient } from "../../../shared/api/session.js";
import { createComposeClient } from "../../../shared/api/compose.js";
import { cloudResourceURL } from "../../../shared/api/http.js";
import { canRunLocalAgent } from "../../../utils.js";
import { choosePublicationDraft } from "../draftRecovery.js";
import PlanList from "../components/PlanList.vue";
import PlanWorkbench from "../components/PlanWorkbench.vue";
import BatchReadonlyBody from "../components/BatchReadonlyBody.vue";
import BatchExecutionControls from "../components/BatchExecutionControls.vue";
import {
  emptyPlan,
  clone,
  planProblem,
  planStates,
  planLabels,
  planRecordLabels,
  planProgressLabels,
  planJournal,
  sendPlanCommand,
  readPlanDraftChoice,
} from "../planPreparation.js";
import {
  batchPhases,
  buildBatchStart,
  batchTaskLocation,
} from "../batchExecution.js";
import { formatPublicationTime } from "../dateTime.js";
import "../plan.css";
const route = useRoute(),
  router = useRouter(),
  api = createPublicationPlanClient(),
  batchApi = createPublicationBatchClient();
const actor = ref(null),
  categories = ref([]),
  list = ref({ items: [], total: 0 }),
  filters = ref({}),
  detail = ref(null),
  form = ref(null),
  loading = ref(false),
  busy = ref(false),
  error = ref(""),
  draftError = ref(""),
  records = ref({ items: [], total: 0 });
const drawerBounds = ref({}),
  preview = ref(null),
  overlayHost = ref(null),
  operation = ref(""),
  reason = ref(""),
  startPreview = ref([]),
  ack = ref(false),
  recommendation = ref(null),
  pending = ref(null),
  draft = ref(null),
  baseline = ref(""),
  pendingCreate = ref(null),
  recordPage = ref(1);
let preparedDraft = null;
let requestedAction = null;
let recordSequence = 0;
let drawerLocked = false;
let drawerScroll = 0,
  drawerOverflow = "",
  seq = 0,
  disposed = false,
  timer,
  stream,
  journal,
  savedScroll = 0,
  localActor,
  allowLeave = false;
const mode = computed(() =>
  ["detail", "create", "edit", "copy"].includes(route.query.view)
    ? route.query.view
    : "list",
);
const page = computed(() => Math.max(1, Number(route.query.page) || 1)),
  size = computed(() =>
    [10, 20, 50].includes(Number(route.query.page_size))
      ? Number(route.query.page_size)
      : 10,
  );
const plan = computed(() => detail.value?.plan),
  canPrepare = computed(
    () => canRunLocalAgent(actor.value) && !!actor.value?.team_id,
  ),
  dirty = computed(
    () =>
      !["list", "detail"].includes(mode.value) &&
      form.value &&
      JSON.stringify(form.value) !== baseline.value,
  );
const count = computed(
  () => plan.value?.goals.reduce((n, g) => n + g.target_count, 0) || 0,
);
const rate = computed(() =>
  count.value
    ? Math.round((plan.value.published_count / count.value) * 10000) / 100
    : 0,
);
const batchFacts = computed(() =>
  mode.value === "edit"
    ? Object.fromEntries(
        (detail.value?.batches || []).map((r) => [
          r.batch_id,
          {
            configured: r.configured,
            fixed:
              !!r.batch.execution?.first_started_at ||
              r.batch.execution_status !== "not_started",
          },
        ]),
      )
    : {},
);
const pendingBatchCount = (b) =>
  ["awaiting_human", "uncertain", "reviewing", "failed"].reduce(
    (n, k) => n + Number(b.execution?.task_counts?.[k] || 0),
    0,
  );
const endedLabel = (p) =>
  ({
    achieved: "目标达成",
    unmet: "保留目标缺口",
    cancelled: "已终止",
    terminated: "已终止",
  })[p.end_reason] || "—";
const planStats = computed(() => [
  { key: "target", label: "计划目标", value: count.value, unit: "条" },
  {
    key: "published",
    label: "发布成功",
    value: plan.value?.published_count || 0,
    unit: "条",
    tone: "success",
  },
  {
    key: "gap",
    label: "目标缺口",
    value: plan.value?.goal_results.reduce((n, g) => n + g.shortfall, 0) || 0,
    unit: "条",
    tone: "warning",
  },
  {
    key: "batches",
    label: "批次进度",
    value:
      (plan.value?.processed_batch_count || 0) +
      " / " +
      (plan.value?.batch_count || 0),
    unit: "批",
  },
]);
const categoryName = (id) =>
  categories.value.find((c) => String(c.id) === id)?.name || "原类目暂不可用";
const progressLabel = (p) =>
  p.progress === "preparing" && p.allowed_actions?.includes("start")
    ? "全部待执行批次已就绪"
    : planProgressLabels[p.progress] || "等待状态更新";
const permitted = (a) => plan.value?.allowed_actions?.includes(a);
const listQuery = () =>
  Object.fromEntries(
    [
      "search",
      "status",
      "platform",
      "category_id",
      "page",
      "page_size",
    ].flatMap((k) =>
      typeof route.query[k] === "string" ? [[k, route.query[k]]] : [],
    ),
  );
function humanError(e) {
  return (
    e?.details?.issues
      ?.map((i) =>
        i.related_batch_id
          ? (i.message || "成片重复") +
            `（批次${i.batch_id}候选${i.row_key}与批次${i.related_batch_id}候选${i.related_row_key}）`
          : i.message ||
            `批次${i.batch_id}候选${i.row_key}与批次${i.related_batch_id}候选${i.related_row_key}成片重复`,
      )
      .join("；") ||
    e.message ||
    "操作失败，请保留当前输入"
  );
}
async function identity() {
  const u = await createSessionClient().me();
  if (localActor && String(u.id) !== localActor)
    throw Error("登录身份已变化，请重新打开计划");
  actor.value = u;
  localActor = String(u.id);
  return u;
}
function setBaseline() {
  baseline.value = JSON.stringify(form.value);
}
async function open(view, id) {
  if (view === "create" && mode.value !== "create") {
    try {
      await identity();
      const client = createPublicationDraftClient({
        userId: localActor,
        draftType: "plan",
      });
      const local = await readPlanDraftChoice(client, (savedAt) =>
        choosePublicationDraft(savedAt, { subject: "发布计划" }),
      );
      if (!local) return;
      preparedDraft = { userId: localActor, client, local };
    } catch (e) {
      error.value = humanError(e);
      return;
    }
  }
  savedScroll =
    document.querySelector(".content-area")?.scrollTop || window.scrollY;
  return router.push({
    name: "PublishPlans",
    query: { ...listQuery(), view, ...(id ? { id } : {}) },
  });
}
async function actionFromList(row, action) {
  requestedAction = { id: row.id, action };
  return open("detail", row.id);
}
async function back() {
  return router.push({ name: "PublishPlans", query: listQuery() });
}
async function reload(background = false) {
  const run = ++seq;
  if (!background) loading.value = true;
  error.value = "";
  try {
    await identity();
    if (!categories.value.length) {
      const c = await createComposeClient().listCategories({ enabled: true });
      if (run !== seq) return;
      categories.value = Array.isArray(c) ? c : c.items || [];
    }
    if (mode.value === "list") {
      const r = await api.list({
        ...listQuery(),
        page: page.value,
        page_size: size.value,
      });
      if (run === seq) {
        list.value = r;
        filters.value = listQuery();
      }
    } else if (mode.value === "detail") {
      const d = await api.get(String(route.query.id));
      if (run !== seq) return;
      detail.value = d;
      journal = planJournal(
        new URL(cloudResourceURL("/api/v1")).origin,
        window.__TAURI_INTERNALS__ ? "desktop" : "web",
        localActor,
        d.plan.id,
      );
      pending.value = journal.read();
      await loadRecords(recordPage.value);
      if (!background && requestedAction?.id === d.plan.id) {
        const action = requestedAction.action;
        requestedAction = null;
        await prepare(action);
      }
    } else if (!background) {
      form.value = null;
      draft.value = null;
      pendingCreate.value = null;
      recommendation.value = null;
      if (!canPrepare.value) throw Error("当前角色只能查看计划");
      if (mode.value === "edit") {
        const d = await api.get(String(route.query.id));
        if (run !== seq) return;
        if (!d.plan.allowed_actions.includes("edit"))
          throw Error("计划已启动或状态变化，不能编辑");
        detail.value = d;
        form.value = emptyPlan({
          ...d.plan,
          batches: d.batches.map((r) => ({
            batch_id: r.batch_id,
            row_key: r.row_key,
            name: r.batch.name,
            platform: r.batch.platform,
            group_order: r.group_order,
            batch_order: r.batch_order,
            targets: r.batch.targets,
          })),
        });
      } else {
        const prepared =
          mode.value === "create" && preparedDraft?.userId === localActor
            ? preparedDraft
            : null;
        preparedDraft = null;
        draft.value =
          prepared?.client ||
          createPublicationDraftClient({
            userId: localActor,
            draftType: "plan",
          });
        let local =
          prepared?.local || (await draft.value.get({ reload: true }));
        if (run !== seq) return;
        if (mode.value === "create" && local.payload && !prepared) {
          local = await readPlanDraftChoice(draft.value, (savedAt) =>
            choosePublicationDraft(savedAt, { subject: "发布计划" }),
          );
          if (run !== seq) return;
          if (!local) {
            allowLeave = true;
            await router.replace({ name: "PublishPlans", query: listQuery() });
            return;
          }
        }
        if (mode.value === "copy") {
          if (local.payload) {
            const replace = await feedback.confirmWarning({
              title: "已有未完成的计划输入",
              message:
                "复制将替换本机原新建输入。原输入仍需保留时，请取消并先恢复处理。",
              confirmText: "替换为复制",
            });
            if (run !== seq) return;
            if (!replace) throw Error("已保留原续编，复制已取消");
            await draft.value.clear();
          }
          const d = await api.get(String(route.query.id));
          const c = await api.copy(d.plan.id, d.plan.revision);
          if (run !== seq) return;
          form.value = emptyPlan(c);
        } else if (local.payload) {
          form.value = emptyPlan(local.payload.form);
          pendingCreate.value = local.payload.pending || null;
        } else form.value = emptyPlan();
      }
      setBaseline();
    }
  } catch (e) {
    if (run === seq) error.value = humanError(e);
  } finally {
    if (run === seq) {
      loading.value = false;
      if (!background) {
        await nextTick();
        const host = document.querySelector(".content-area");
        const restore = Number(route.query.scroll);
        if (host)
          host.scrollTop =
            Number.isFinite(restore) && restore >= 0
              ? restore
              : mode.value === "list"
                ? savedScroll
                : 0;
      }
    }
  }
}
async function loadRecords(p = 1) {
  if (!plan.value) return;
  const id = plan.value.id,
    run = ++recordSequence;
  recordPage.value = p;
  try {
    const r = await api.records(id, { page: p, page_size: 10 });
    if (run === recordSequence && plan.value?.id === id) records.value = r;
  } catch (e) {
    if (run === recordSequence) error.value = humanError(e);
  }
}
async function search(p = 1, s = size.value) {
  const q = Object.fromEntries(
    Object.entries(filters.value).filter(([, v]) => v),
  );
  q.page = String(p);
  q.page_size = String(s);
  return router.replace({ name: "PublishPlans", query: q });
}
async function saveDraft() {
  try {
    await identity();
    await draft.value.save({
      form: clone(form.value),
      pending: pendingCreate.value,
    });
    draftError.value = "";
    feedback.toast.success("已保存本机新建续编");
  } catch (e) {
    draftError.value = humanError(e);
  }
}
async function save() {
  if (busy.value) return;
  error.value = "";
  const issue = planProblem(form.value);
  if (issue) {
    error.value = issue;
    return;
  }
  const firstCreate = mode.value !== "edit" && !pendingCreate.value;
  let createPostSent = false;
  busy.value = true;
  try {
    await identity();
    if (mode.value === "edit") {
      const r = await api.update(plan.value.id, {
        ...clone(form.value),
        expected_revision: plan.value.revision,
      });
      setBaseline();
      allowLeave = true;
      await open("detail", r.plan.id);
      return;
    }
    if (!pendingCreate.value) {
      pendingCreate.value = {
        ...clone(form.value),
        request_key: crypto.randomUUID(),
      };
      await draft.value.save({
        form: clone(form.value),
        pending: pendingCreate.value,
      });
    }
    // A lost response is found by creator request key before a same-command replay.
    let r;
    try {
      const found = await api.list({
        request_key: pendingCreate.value.request_key,
        page: 1,
        page_size: 10,
      });
      if (found.items.length === 1) r = await api.get(found.items[0].id);
      if (found.items.length > 1) throw Error("原请求找回结果不唯一");
    } catch (e) {
      throw e;
    }
    if (!r) {
      createPostSent = true;
      r = await api.create(pendingCreate.value);
    }
    if (!r?.plan?.id) throw Error("创建结果不完整，请保留原请求");
    try {
      await draft.value.clear();
    } catch (clearError) {
      feedback.toast.info(
        "计划已创建，本机输入已有更新或清理失败，已保留续编供下次核对",
      );
    }
    pendingCreate.value = null;
    setBaseline();
    allowLeave = true;
    await open("detail", r.plan.id);
  } catch (e) {
    // Only a first command that was never sent, or an explicit rejection of that
    // first POST, may return to editable input. Previously unknown commands stay.
    if (
      firstCreate &&
      (!createPostSent || [400, 401, 403, 404, 409, 422].includes(e.status))
    ) {
      pendingCreate.value = null;
      if (draft.value)
        try {
          await draft.value.save({ form: clone(form.value), pending: null });
        } catch (localError) {
          draftError.value = humanError(localError);
        }
    }
    error.value =
      humanError(e) +
      (pendingCreate.value ? "；原请求已保留，请核对创建结果后继续" : "");
  } finally {
    busy.value = false;
  }
}
async function validate() {
  busy.value = true;
  try {
    await identity();
    const r = await api.validate({
      ...clone(form.value),
      ...(mode.value === "edit"
        ? { plan_id: plan.value.id, expected_revision: plan.value.revision }
        : {}),
    });
    error.value = r.valid ? "" : r.issues.map((i) => i.message).join("；");
    if (r.valid) feedback.toast.success("目标与批次安排检查通过");
  } catch (e) {
    error.value = humanError(e);
  } finally {
    busy.value = false;
  }
}
async function recommend(instruction) {
  if (busy.value) return;
  busy.value = true;
  error.value = "";
  recommendation.value = null;
  stream = new AbortController();
  const basis = JSON.stringify(form.value);
  try {
    await identity();
    const result = await api.recommend(
      {
        goals: clone(form.value.goals),
        batches: clone(form.value.batches.map(({ batch_id, ...r }) => r)),
        instruction,
      },
      { signal: stream.signal },
    );
    if (disposed || JSON.stringify(form.value) !== basis) return;
    const events = Array.isArray(result) ? result : result?.events;
    const item = events?.find(
      (e) => e.event === "item_result" || e.type === "item_result",
    );
    recommendation.value =
      item?.data?.result || item?.data || result?.result || result;
    if (!Array.isArray(recommendation.value?.batches))
      throw Error("未收到完整拆批建议");
    recommendation.value = { ...recommendation.value, basis };
  } catch (e) {
    if (!disposed) error.value = humanError(e);
  } finally {
    busy.value = false;
  }
}
async function adopt() {
  if (recommendation.value?.basis !== JSON.stringify(form.value)) {
    error.value = "输入已变化，请重新生成建议";
    return;
  }
  if (Object.values(batchFacts.value).some((f) => f.fixed)) {
    error.value = "存在已终止固定批次，不能整体替换拆批安排";
    return;
  }
  if (form.value.batches.length) {
    const basis = JSON.stringify(form.value);
    const accepted = await feedback.confirmWarning({
      title: "替换批次安排？",
      message:
        "采用建议会替换当前批次规划。已有候选资源配置需要重新检查；目标数量保持不变。",
      confirmText: "采用并替换",
    });
    if (!accepted) return;
    if (basis !== JSON.stringify(form.value)) {
      error.value = "输入已变化，请重新生成建议";
      return;
    }
  }
  form.value.batches = clone(recommendation.value.batches);
  recommendation.value = null;
}
function batchRoute(row, view = "detail") {
  const q = {
    view,
    id: row.batch_id,
    from_plan: plan.value.id,
    plan_scroll: String(
      document.querySelector(".content-area")?.scrollTop || 0,
    ),
  };
  for (const [k, v] of Object.entries(listQuery())) q["plan_" + k] = v;
  return { name: "PublishBatches", query: q };
}
async function closePreview() {
  preview.value = null;
  if (!drawerLocked) return;
  drawerLocked = false;
  const area = document.querySelector(".content-area");
  if (area) {
    area.style.overflow = drawerOverflow;
    await nextTick();
    area.scrollTop = drawerScroll;
  }
}
function taskLink(batch, id) {
  const context = {
    from_plan: plan.value.id,
    plan_scroll: String(
      document.querySelector(".content-area")?.scrollTop || 0,
    ),
  };
  for (const [k, v] of Object.entries(listQuery())) context["plan_" + k] = v;
  return batchTaskLocation(batch.id, id, context, {});
}
async function showBatch(row) {
  const area = document.querySelector(".content-area");
  if (area) {
    const r = area.getBoundingClientRect();
    drawerScroll = area.scrollTop;
    drawerOverflow = area.style.overflow;
    drawerBounds.value = {
      position: "fixed",
      left: r.left + "px",
      top: r.top + "px",
      width: r.width + "px",
      height: r.height + "px",
    };
    drawerLocked = true;
    area.style.overflow = "hidden";
  }
  busy.value = true;
  try {
    preview.value = await batchApi.get(row.batch_id);
  } catch (e) {
    await closePreview();
    error.value = humanError(e);
  } finally {
    busy.value = false;
  }
}
async function prepare(action) {
  if (busy.value || pending.value) return;
  busy.value = true;
  reason.value = "";
  error.value = "";
  ack.value = false;
  operation.value = action;
  startPreview.value = [];
  try {
    await identity();
    const d = await api.get(plan.value.id);
    detail.value = d;
    if (!d.plan.allowed_actions.includes(action))
      throw Error("计划状态已变化，请重新核对");
    if (action === "start") {
      for (const row of d.batches) {
        if (row.batch.execution_status === "cancelled") continue;
        const b = await batchApi.get(row.batch_id);
        const pre = await batchApi.executionPreview(b.id, {
          expected_revision: b.revision,
          row_keys: b.config.candidates.map((r) => r.row_key),
        });
        startPreview.value.push({ b, pre });
      }
    }
  } catch (e) {
    operation.value = "";
    error.value = humanError(e);
  } finally {
    busy.value = false;
  }
}
async function command(recover = false) {
  if (busy.value) return;
  busy.value = true;
  error.value = "";
  try {
    await identity();
    const p = plan.value;
    let op = pending.value;
    if (!recover) {
      let input = { expected_revision: p.revision };
      if (operation.value === "start") {
        if (!ack.value) throw Error("请确认全部批次范围与目标缺口");
        const confirmations = startPreview.value.map(({ b, pre }) => ({
          batch_id: b.id,
          ...buildBatchStart(b, pre, "unused", true),
        }));
        for (const c of confirmations) delete c.request_key;
        input = {
          ...input,
          request_key: crypto.randomUUID(),
          first_batch_id: confirmations[0]?.batch_id,
          batch_confirmations: confirmations,
        };
      } else if (operation.value === "confirm-next") {
        const b = p.handoff_basis;
        input = {
          ...input,
          request_key: crypto.randomUUID(),
          current_batch_id: b.current_batch_id,
          next_batch_id: b.next_batch_id,
          expected_basis_fingerprint: b.fingerprint,
          decision: p.progress === "waiting_skip" ? "skip" : "continue",
          reason: reason.value.trim(),
        };
      } else if (operation.value === "takeover") {
        const row = detail.value.batches.find(
          (r) => r.batch_id === p.current_batch_id,
        );
        if (!row) throw Error("当前位置已变化");
        input = {
          ...input,
          current_batch_id: p.current_batch_id,
          expected_batch_revision: row.batch.revision,
          reason: reason.value.trim(),
        };
      } else if (operation.value === "finish")
        input = {
          ...input,
          expected_shortfalls: p.goal_results.map((g) => ({
            platform: g.platform,
            category_id: g.category_id,
            shortfall: g.shortfall,
          })),
          end_reason: p.goal_results.some((g) => g.shortfall > 0)
            ? "unmet"
            : "achieved",
          end_note: reason.value.trim(),
        };
      else if (["pause", "cancel"].includes(operation.value))
        input.reason = reason.value.trim();
      op = { action: operation.value, input };
      if (["start", "confirm-next"].includes(op.action)) {
        await journal.save(op);
        pending.value = journal.read();
      }
    }
    if (["start", "confirm-next"].includes(op.action)) {
      await sendPlanCommand(api, p.id, op, { recover });
      await journal.clear(op);
      pending.value = journal.read();
    } else await api[op.action](p.id, op.input);
    operation.value = "";
    await reload(true);
    feedback.toast.success("计划已更新");
  } catch (e) {
    error.value = humanError(e);
    if (
      !e.outcomeUnknown &&
      pending.value &&
      (e.planRejection ||
        (!recover && [400, 401, 403, 404, 422].includes(e.status)))
    ) {
      await journal.clear(pending.value);
      pending.value = journal.read();
      if (!pending.value && e.planRejection) {
        const message = humanError(e);
        operation.value = "";
        ack.value = false;
        startPreview.value = [];
        await reload(true);
        error.value = message;
      }
    }
  } finally {
    busy.value = false;
  }
}
watch(
  () => route.fullPath,
  () => {
    recordSequence++;
    recordPage.value = 1;
    allowLeave = false;
    operation.value = "";
    preview.value = null;
    reload();
  },
  { immediate: true },
);
async function guardNavigation() {
  if (allowLeave) return true;
  if (busy.value || pendingCreate.value) return false;
  if (dirty.value && !allowLeave)
    return await feedback.confirmWarning({
      title: "离开当前配置？",
      message: "尚未保存的输入将留在当前页面，确认离开？",
      confirmText: "确认离开",
    });
  return true;
}
onBeforeRouteLeave(guardNavigation);
onBeforeRouteUpdate(guardNavigation);
watch(
  form,
  () => {
    if (draft.value && !draft.value.persistent && !busy.value && form.value)
      draft.value.remember({
        form: clone(form.value),
        pending: pendingCreate.value,
      });
  },
  { deep: true },
);
timer = setInterval(() => {
  if (
    mode.value === "detail" &&
    !busy.value &&
    !operation.value &&
    !preview.value &&
    !pending.value
  )
    reload(true);
}, 5000);
onBeforeUnmount(() => {
  disposed = true;
  seq++;
  clearInterval(timer);
  stream?.abort();
  const area = document.querySelector(".content-area");
  if (area && drawerLocked) {
    area.style.overflow = drawerOverflow;
    area.scrollTop = drawerScroll;
  }
});
</script>
<template>
  <div ref="overlayHost" class="plan-page">
    <FeedbackAlert v-if="error" theme="error" :message="error" />
    <t-button
      v-if="error && !form && !plan && mode !== 'list'"
      variant="outline"
      @click="back"
      >返回发布计划列表</t-button
    >
    <p v-if="loading" role="status">正在读取发布计划</p>
    <PlanList
      v-if="mode === 'list'"
      :rows="list.items"
      :filters="filters"
      :categories="categories"
      :page="page"
      :page-size="size"
      :total="list.total"
      :loading="loading"
      :can-create="canPrepare"
      :progress-label="progressLabel"
      @open="open"
      @action="actionFromList"
      @query="search"
      @reset="
        filters = {};
        search(1);
      "
    />
    <ResourceEditorPage
      v-else-if="form && !loading && mode !== 'detail'"
      :title="mode === 'edit' ? '编辑发布计划' : '新建发布计划'"
      :busy="busy"
      @back="back"
      ><FeedbackAlert
        v-if="draftError"
        theme="error"
        :message="draftError"
      /><PlanWorkbench
        :form="form"
        :categories="categories"
        :batch-facts="batchFacts"
        :disabled="busy || !!pendingCreate"
        @recommend="recommend"
      /><ResourceCard v-if="recommendation"
        ><ResourceSection title="拆批建议"
          ><div v-for="b in recommendation.batches" :key="b.row_key">
            {{ b.name }} ·
            {{
              b.targets
                .map((t) => categoryName(t.category_id) + " " + t.target_count)
                .join("，")
            }}
          </div>
          <t-button variant="outline" @click="adopt"
            >采用建议</t-button
          ></ResourceSection
        ></ResourceCard
      ><template #footer
        ><span class="resource-footer-status plan-muted">{{
          form.goals.length && form.batches.length && !planProblem(form)
            ? "目标与批次分配已匹配"
            : "完成目标与批次分配后创建计划"
        }}</span
        ><t-button
          v-if="draft?.persistent"
          variant="outline"
          :disabled="busy || !!pendingCreate"
          @click="saveDraft"
          >保存本机续编</t-button
        ><t-button
          variant="outline"
          :disabled="busy || !!pendingCreate"
          @click="validate"
          >检查安排</t-button
        ><t-button :loading="busy" @click="save">{{
          pendingCreate
            ? "核对原创建请求"
            : mode === "edit"
              ? "保存修改"
              : "创建计划"
        }}</t-button></template
      ></ResourceEditorPage
    >
    <ResourceDetailPage
      v-else-if="plan && mode === 'detail' && !loading"
      title="发布计划详情"
      back-label="返回发布计划列表"
      @back="back"
    >
      <ResourceNotice
        :tone="
          [
            'waiting_tasks',
            'waiting_verification',
            'waiting_resources',
            'waiting_continue',
            'waiting_skip',
            'manual_paused',
          ].includes(plan.progress)
            ? 'warning'
            : 'info'
        "
        :title="progressLabel(plan)"
      >
        {{
          plan.end_note ||
          (plan.started_at
            ? "计划按批次顺序推进；需要处理时可使用当前批次的原操作入口。"
            : "完成所有批次配置后，统一检查并执行计划。")
        }}
        <template v-if="plan.current_batch_id" #actions
          ><t-button
            theme="primary"
            variant="text"
            @click="
              router.push(
                batchRoute(
                  detail.batches.find(
                    (r) => r.batch_id === plan.current_batch_id,
                  ),
                ),
              )
            "
            >去当前批次处理</t-button
          ></template
        >
      </ResourceNotice>
      <ResourceCard
        ><ResourceSection title="计划信息"
          ><ResourceDescriptions
            ><t-descriptions-item
              v-for="item in [
                { label: '计划名称', value: plan.name },
                { label: '计划编号', value: plan.id },
                { label: '计划状态', value: planStates[plan.status] },
                ...(plan.status === 'ended'
                  ? [{ label: '结束结果', value: endedLabel(plan) }]
                  : []),
                { label: '负责人 ID', value: plan.owner_id },
                {
                  label: '执行周期',
                  value: plan.date_start + ' ~ ' + plan.date_end,
                },
                { label: '发布平台', value: '百家号' },
                {
                  label: '发布类目',
                  value: [
                    ...new Set(
                      plan.goals.map((g) => categoryName(g.category_id)),
                    ),
                  ].join('、'),
                },
                { label: '规划批次', value: plan.batch_count + ' 个原始批次' },
                {
                  label: '创建时间',
                  value: formatPublicationTime(plan.created_at),
                },
                {
                  label: '最近更新',
                  value: formatPublicationTime(plan.updated_at),
                },
                { label: '备注', value: plan.note || '—' },
              ]"
              :key="item.label"
              :label="item.label"
              ><ResourceStatusBadge
                v-if="item.label === '计划状态'"
                :label="item.value"
                :tone="
                  plan.status === 'ended'
                    ? 'success'
                    : plan.status === 'not_started'
                      ? 'neutral'
                      : 'info'
                "
              /><template v-else>{{
                item.value
              }}</template></t-descriptions-item
            ></ResourceDescriptions
          ></ResourceSection
        ></ResourceCard
      >
      <ResourceCard
        ><ResourceSection
          title="目标完成情况"
          description="仅最终确认发布成功的任务计入达成量"
          ><ResourceStatGrid :items="planStats" />
          <ResourceProgress
            label="总体达成率"
            :value-text="plan.published_count + ' / ' + count + ' 条 · '"
            :percentage="rate"
          />
          <div class="plan-detail-subtitle">平台 × 类目完成矩阵</div>
          <div
            class="plan-detail-table resource-data-table resource-embedded-table is-summary"
            tabindex="0"
            role="region"
            aria-label="计划目标与批次表，可横向滚动"
          >
            <table>
              <thead>
                <tr>
                  <th>平台</th>
                  <th>类目</th>
                  <th>目标数</th>
                  <th>发布成功</th>
                  <th class="resource-table-number">目标缺口</th>
                  <th class="resource-table-number">完成率</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="g in plan.goal_results"
                  :key="g.platform + g.category_id"
                >
                  <td>百家号</td>
                  <td>{{ categoryName(g.category_id) }}</td>
                  <td>{{ g.target_count }}</td>
                  <td>{{ g.published_count }}</td>
                  <td class="resource-table-number">{{ g.shortfall }}</td>
                  <td class="resource-table-number">
                    {{
                      g.target_count
                        ? Math.round(
                            (g.published_count / g.target_count) * 10000,
                          ) / 100
                        : 0
                    }}%
                  </td>
                </tr>
                <tr class="plan-detail-total">
                  <td colspan="2">合计</td>
                  <td class="resource-table-number">{{ count }}</td>
                  <td class="resource-table-number">
                    {{ plan.published_count }}
                  </td>
                  <td class="resource-table-number">
                    {{ plan.goal_results.reduce((n, g) => n + g.shortfall, 0) }}
                  </td>
                  <td class="resource-table-number">{{ rate }}%</td>
                </tr>
              </tbody>
            </table>
          </div>
          <p class="plan-muted">
            只有核实公开并关联作品的任务计入成功；平台审核中不计入。
          </p></ResourceSection
        ></ResourceCard
      >
      <ResourceCard
        ><ResourceSection title="批次推进"
          ><div
            class="plan-detail-table resource-data-table resource-embedded-table is-summary"
            tabindex="0"
            role="region"
            aria-label="计划目标与批次表，可横向滚动"
          >
            <table>
              <thead>
                <tr>
                  <th>顺序</th>
                  <th>批次名称</th>
                  <th>平台／类目</th>
                  <th>目标数</th>
                  <th>资源配置</th>
                  <th>执行状态</th>
                  <th class="resource-table-number">结果</th>
                  <th>当前待办</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(r, i) in detail.batches" :key="r.batch_id">
                  <td>{{ i + 1 }}</td>
                  <td>
                    {{ r.batch.name
                    }}<small
                      >{{ r.batch_id
                      }}{{
                        plan.current_batch_id === r.batch_id
                          ? " · 当前批次"
                          : ""
                      }}</small
                    >
                  </td>
                  <td>
                    百家号<small>{{
                      r.batch.targets
                        .map(
                          (t) =>
                            categoryName(t.category_id) + " " + t.target_count,
                        )
                        .join("，")
                    }}</small>
                  </td>
                  <td>
                    {{
                      r.batch.targets.reduce((n, t) => n + t.target_count, 0)
                    }}
                  </td>
                  <td>
                    {{
                      plan.started_at
                        ? "已锁定"
                        : r.configured
                          ? "已配置"
                          : "未配置"
                    }}
                  </td>
                  <td>
                    {{
                      r.batch.execution_status === "not_started"
                        ? "未开始"
                        : batchPhases[r.batch.execution?.phase] ||
                          {
                            running: "执行中",
                            paused: "已暂停",
                            stopping: "停止中",
                            ended: "已结束",
                            cancelled: "已取消",
                          }[r.batch.execution_status] ||
                          "等待状态更新"
                    }}
                  </td>
                  <td class="resource-table-number">
                    {{
                      r.batch.execution?.first_started_at
                        ? (r.batch.execution?.published_count || 0) +
                          " / " +
                          r.batch.targets.reduce(
                            (n, t) => n + t.target_count,
                            0,
                          )
                        : "—"
                    }}
                  </td>
                  <td>
                    <span
                      :class="{
                        'plan-detail-pending': pendingBatchCount(r.batch),
                      }"
                      >{{
                        pendingBatchCount(r.batch)
                          ? pendingBatchCount(r.batch) + " 条待处理"
                          : "—"
                      }}</span
                    >
                  </td>
                  <td>
                    <div class="plan-inline">
                      <t-button variant="text" @click="showBatch(r)"
                        >详情</t-button
                      ><t-button
                        v-if="r.allowed_actions.includes('edit')"
                        variant="text"
                        @click="router.push(batchRoute(r, 'edit'))"
                        >编辑配置</t-button
                      ><BatchExecutionControls
                        :batch="r.batch"
                        :actor="actor"
                        :category-name="categoryName"
                        :task-link="(id) => taskLink(r.batch, id)"
                        @updated="reload(true)"
                        @busy="busy = $event"
                      /><t-button
                        v-if="
                          r.batch.execution?.task_counts?.awaiting_human ||
                          r.batch.execution?.task_counts?.uncertain ||
                          r.batch.execution?.task_counts?.reviewing ||
                          r.batch.execution?.task_counts?.failed
                        "
                        variant="text"
                        @click="router.push(batchRoute(r))"
                        >去处理</t-button
                      >
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div></ResourceSection
        ></ResourceCard
      >
      <ResourceCard
        ><ResourceSection title="执行记录"
          ><ol v-if="records.items.length" class="plan-detail-history">
            <li v-for="r in records.items" :key="r.id">
              <p>{{ planRecordLabels[r.event] || "计划业务记录" }}</p>
              <span
                >{{ formatPublicationTime(r.created_at)
                }}<template v-if="r.actor_id">
                  · 操作人 ID：{{ r.actor_id }}</template
                ></span
              >
            </li>
          </ol>
          <ResourceNotice v-else>暂无执行记录</ResourceNotice>
          <t-pagination
            :current="recordPage"
            :page-size="10"
            :total="records.total"
            :show-page-size="false"
            @current-change="loadRecords" /></ResourceSection
      ></ResourceCard>
      <template #footer
        ><t-button v-if="pending" :loading="busy" @click="command(true)"
          >核对原计划请求</t-button
        ><template v-else
          ><t-button
            v-for="a in Object.keys(planLabels).filter(permitted)"
            :key="a"
            :theme="
              a === 'cancel'
                ? 'danger'
                : ['start', 'confirm-next', 'finish'].includes(a)
                  ? 'primary'
                  : 'default'
            "
            :variant="a === 'cancel' ? 'text' : 'base'"
            :disabled="busy"
            @click="prepare(a)"
            >{{
              a === "confirm-next" && plan.progress === "waiting_skip"
                ? "确认跳过当前批次"
                : planLabels[a]
            }}</t-button
          ></template
        ></template
      >
    </ResourceDetailPage>
    <div class="plan-drawer-host" :style="drawerBounds"></div>
    <ResourceOverlay
      :visible="!!preview"
      kind="drawer"
      full-width
      attach=".plan-drawer-host"
      show-in-attached-element
      :footer="false"
      title="发布批次详情"
      @update:visible="closePreview"
      ><div v-if="preview" class="plan-stack">
        <BatchReadonlyBody
          :key="preview.id"
          :batch="preview"
          :category-name="categoryName"
          :task-link="(id) => taskLink(preview, id)"
        /></div
    ></ResourceOverlay>
    <ResourceOverlay
      :visible="!!operation"
      :title="planLabels[operation] || '计划操作'"
      :busy="busy"
      @update:visible="operation = ''"
      :footer="false"
      ><p v-if="operation === 'start'">
        确认后冻结计划目标、顺序及全部批次配置，只为首批建任务；后续按10%条件推进。
      </p>
      <div v-if="operation === 'start'" class="plan-table">
        <table>
          <thead>
            <tr>
              <th>批次</th>
              <th>目标</th>
              <th>本次候选</th>
              <th>缺口</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="s in startPreview" :key="s.b.id">
              <td>{{ s.b.name }}</td>
              <td>{{ s.b.targets.reduce((n, t) => n + t.target_count, 0) }}</td>
              <td>{{ s.pre.rows?.length }}</td>
              <td>
                {{ s.pre.shortfalls?.reduce((n, t) => n + t.shortfall, 0) }}
              </td>
            </tr>
          </tbody>
        </table>
        <t-checkbox v-model="ack">确认全部批次范围及目标缺口</t-checkbox>
      </div>
      <p v-if="operation === 'takeover'">
        接管当前已有批次，暂停计划后续推进，不重新建单或上传。完成后可恢复计划。
      </p>
      <p v-if="operation === 'pause'">
        暂停后续批次，当前已启动执行继续跟进真实结果。
      </p>
      <p v-if="operation === 'cancel'">
        终止全部所属批次的新派发，在途和未知结果保留安全收尾入口。终止后不可恢复。
      </p>
      <p v-if="operation === 'finish'">
        按真实公开结果结束；未达成保留原目标和缺口。
      </p>
      <label
        v-if="
          ['pause', 'cancel', 'confirm-next', 'finish', 'takeover'].includes(
            operation,
          )
        "
        >说明<t-textarea
          v-model="reason"
          :maxlength="200"
          aria-label="操作说明"
      /></label>
      <div class="plan-inline">
        <t-button
          theme="default"
          variant="outline"
          :disabled="busy"
          @click="operation = ''"
          >返回</t-button
        ><t-button v-if="pending" :loading="busy" @click="command(true)"
          >核对原计划请求</t-button
        ><t-button
          v-else
          :loading="busy"
          :disabled="
            (operation === 'start' && !ack) ||
            (['pause', 'cancel', 'confirm-next', 'takeover'].includes(
              operation,
            ) &&
              !reason.trim()) ||
            (operation === 'finish' &&
              plan.goal_results.some((g) => g.shortfall > 0) &&
              !reason.trim())
          "
          @click="command()"
          >{{
            operation === "confirm-next"
              ? plan.progress === "waiting_skip"
                ? "确认跳过当前批次"
                : "确认进入下一批"
              : "确认" + planLabels[operation]
          }}</t-button
        >
      </div></ResourceOverlay
    >
  </div>
</template>
