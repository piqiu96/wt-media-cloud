<script setup>
import {
  ref,
  reactive,
  computed,
  watch,
  onMounted,
  onBeforeUnmount,
  nextTick,
} from "vue";
import { feedback } from "../../../src/shared/ui/feedback/index.js";
import ResourceFilterBar from "../../../src/shared/ui/resource/ResourceFilterBar.vue";
import ResourceAdvancedToggle from "../../../src/shared/ui/resource/ResourceAdvancedToggle.vue";
import ResourceSelectionToolbar from "../../../src/shared/ui/resource/ResourceSelectionToolbar.vue";
import ResourceRowActions from "../../../src/shared/ui/resource/ResourceRowActions.vue";
import ResourceEmptyState from "../../../src/shared/ui/resource/ResourceEmptyState.vue";
import ResourceDescriptions from "../../../src/shared/ui/resource/ResourceDescriptions.vue";
import ResourceNotice from "../../../src/shared/ui/resource/ResourceNotice.vue";
import MediaPreview from "../../../src/shared/ui/media/MediaPreview.vue";
import { imageReferences, templateReferences, deleteImage } from "./library.js";
import BatchWorkbench from "./BatchWorkbench.vue";
import { addScenario } from "./scenarios.js";
import PlanWizard from "./PlanWizard.vue";
import PlanWorkbench from "./PlanWorkbench.vue";
import { defaultDocument, renderDocument } from "./covers.js";
import {
  pausePlan,
  pauseBatch,
  queueTask,
  takeBatchControl,
  cancelBatchSafely,
  cancellationImpact,
  cancelPlan,
  claimTask,
  setResult,
  decision,
  targetTotal,
} from "./relationships.js";
import { coverSpecs, ensureCoverResults } from "./covers.js";
import { loadPrototype, savePrototype } from "./prototypeStorage.js";
import {
  batchTasks,
  batchSummary,
  planItems,
  planTasks as collectPlanTasks,
  itemTasks,
  itemRecord,
  associatedPlan,
  canLink,
  linkItem,
  moveItem,
  dispatchBatch,
  dispatchPlan,
  advance,
  syncPlan,
  migrateRelationships,
} from "./relationships.js";
import PublishComposer from "./PublishComposer.vue";
import { useMediaQuery } from "../../../src/shared/composables/useMediaQuery.js";
import ResourceOverlay from "./PrototypeOverlay.vue";
import ResourcePageHeader from "../../../src/shared/ui/resource/ResourcePageHeader.vue";
import ResourceCard from "../../../src/shared/ui/resource/ResourceCard.vue";
import PrototypeEditor from "./PrototypeEditor.vue";
import {
  platforms,
  backgroundPicture,
  states,
  categories,
  seed,
  validate,
  stats,
  nextBatch,
  canRetry,
  declarationOptions,
  resourceStatus,
} from "./model.js";
const compact = useMediaQuery("(max-width:700px)");
const clone = (x) => JSON.parse(JSON.stringify(x));
const db = reactive(seed()),
  storageError = ref(""),
  hydrated = ref(false),
  savingSnapshot = ref(false);
const simulateSaveFailure = ref(false);
let saveTimer;
let saveQueue = Promise.resolve();
async function persistNow() {
  const snapshot = clone(db);
  savingSnapshot.value = true;
  saveQueue = saveQueue
    .catch(() => {})
    .then(() => {
      if (simulateSaveFailure.value) throw Error("演示：本地存储写入失败");
      return savePrototype(snapshot);
    });
  try {
    await saveQueue;
    storageError.value = "";
    return true;
  } catch (e) {
    storageError.value = `演示配置未能保存：${e.message || "请重试"}。当前内容仍在本页，请不要刷新。`;
    return false;
  } finally {
    savingSnapshot.value = false;
  }
}
onMounted(async () => {
  try {
    const saved = await loadPrototype();
    if (saved) Object.assign(db, saved);
    migrateRelationships(db);
    if (!db.images.some((i) => i.pureBackground))
      db.images.push({
        id: id(),
        name: "通用纯净底图（演示）",
        category: "通用",
        uses: ["底图"],
        enabled: true,
        src: backgroundPicture(),
        pureBackground: true,
        format: "SVG",
        width: 1200,
        height: 900,
        source: "内置图形（演示）",
        version: 1,
        updatedAt: new Date().toISOString(),
      });
    if (!db.templates.length) {
      for (const [width, height] of [
        [1200, 900],
        [1600, 900],
        [1080, 1440],
      ]) {
        const doc = defaultDocument(
          width,
          height,
          db.images.find((i) => i.enabled)?.src || "",
        );
        db.templates.push({
          id: id(),
          name: `通用动态短标题 ${width}×${height}`,
          document: doc,
          ratio: `${width}×${height}`,
          version: 1,
          enabled: true,
          preview: doc.image,
        });
      }
    }
  } catch (e) {
    storageError.value = `演示记录未能恢复：${e.message}。当前显示初始示例。`;
  }
  hydrated.value = true;
});
watch(
  db,
  () => {
    if (hydrated.value) {
      clearTimeout(saveTimer);
      saveTimer = setTimeout(persistNow, 200);
    }
  },
  { deep: true, flush: "post" },
);
const page = ref(
    ["tasks", "batches", "plans", "images", "templates"].includes(
      location.hash.slice(1),
    )
      ? location.hash.slice(1)
      : "tasks",
  ),
  pageNo = ref(1),
  size = ref(8);
function navigate(p) {
  page.value = p;
  location.hash = p;
  pageNo.value = 1;
  chosen.value = [];
  query.value = "";
  applied.value = "";
  filterStatus.value = "";
  filterPlatform.value = "";
  filterCategory.value = "";
  Object.assign(appliedFilters, { platform: "", category: "", status: "" });
  batchFilter.value = null;
  planFilter.value = null;
}
const hash = () => {
  const p = location.hash.slice(1);
  if (
    ["tasks", "batches", "plans", "images", "templates"].includes(p) &&
    p !== page.value
  )
    navigate(p);
};
window.addEventListener("hashchange", hash);
onBeforeUnmount(() => window.removeEventListener("hashchange", hash));
const names = {
  tasks: "发布任务",
  batches: "发布批次",
  plans: "发布计划",
  images: "图片库",
  templates: "封面模板库",
};
const query = ref(""),
  applied = ref(""),
  filterStatus = ref(""),
  filterPlatform = ref(""),
  filterCategory = ref(""),
  advanced = ref(false),
  chosen = ref([]),
  batchFilter = ref(null),
  planFilter = ref(null);
const libraryFilter = reactive({ use: "", ratio: "", source: "" });
const referenceRows = ref(null);
const appliedFilters = reactive({ platform: "", category: "", status: "" });
function applyQuery() {
  applied.value = query.value;
  Object.assign(appliedFilters, {
    platform: filterPlatform.value,
    category: filterCategory.value,
    status: filterStatus.value,
  });
  pageNo.value = 1;
  chosen.value = [];
}
function resetQuery() {
  query.value = "";
  Object.assign(libraryFilter, { use: "", ratio: "", source: "" });
  filterPlatform.value = "";
  filterCategory.value = "";
  filterStatus.value = "";
  batchFilter.value = null;
  planFilter.value = null;
  applyQuery();
}
const records = computed(() => {
  let rows = db[page.value] || [];
  if (page.value === "tasks") {
    if (batchFilter.value)
      rows = rows.filter((t) => t.batchId === batchFilter.value);
    if (planFilter.value)
      rows = rows.filter((t) =>
        planTasks(
          db.plans.find((p) => p.id === planFilter.value) || { items: [] },
        ).some((r) => r.id === t.id),
      );
  }
  return rows.filter(
    (r) =>
      (
        r.name ||
        r.title ||
        db.assets.find((a) => a.id === r.assetId)?.title ||
        ""
      ).includes(applied.value) &&
      (!["images", "templates"].includes(page.value) ||
        ((!libraryFilter.use || r.uses?.includes(libraryFilter.use)) &&
          (!libraryFilter.ratio ||
            Math.abs(
              (r.document?.width || r.width || 1200) /
                (r.document?.height || r.height || 900) -
                Number(libraryFilter.ratio),
            ) < 0.01) &&
          (!libraryFilter.source ||
            (r.source || "种子资源").includes(libraryFilter.source)))) &&
      (!appliedFilters.platform || r.platform === appliedFilters.platform) &&
      (!appliedFilters.category ||
        r.category === appliedFilters.category ||
        r.categories?.includes(appliedFilters.category)) &&
      (!appliedFilters.status ||
        r.status === appliedFilters.status ||
        r.control === appliedFilters.status ||
        String(r.enabled) === appliedFilters.status),
  );
});
const paged = computed(() =>
  records.value.slice(
    (pageNo.value - 1) * size.value,
    pageNo.value * size.value,
  ),
);
watch([records, size], () => {
  pageNo.value = Math.min(
    pageNo.value,
    Math.max(1, Math.ceil(records.value.length / size.value)),
  );
  chosen.value = chosen.value.filter((id) =>
    records.value.some((r) => r.id === id),
  );
});
const asset = (id) => db.assets.find((a) => a.id === id),
  account = (id) => db.accounts.find((a) => a.id === id),
  id = () =>
    Math.max(
      1000,
      ...db.tasks.map((t) => t.id),
      ...db.batches.map((b) => b.id),
      ...db.plans.map((p) => p.id),
      ...db.images.map((i) => i.id),
      ...db.templates.map((t) => t.id),
    ) + 1;
const toast = (text) => feedback.toast.success(text),
  warning = (text) => feedback.toast.error(text);
const stateTheme = (s) =>
  s === "已发布"
    ? "success"
    : ["执行失败", "结果待确认", "等待人工"].includes(s)
      ? "warning"
      : s === "执行中"
        ? "primary"
        : "default";
const detail = ref(null),
  detailOpen = ref(false),
  evidence = ref(""),
  bulkResult = ref(null);
function view(row) {
  if (row?.items) planView.value = "总览";
  detail.value = row;
  detailOpen.value = true;
  evidence.value = row.evidence || "";
}
function related(batchId, planId) {
  navigate("tasks");
  batchFilter.value = batchId;
  planFilter.value = planId;
  detailOpen.value = false;
}
function demoTransitions(t) {
  return t.status === "排队中"
    ? ["执行中"]
    : t.status === "执行中"
      ? ["等待人工", "结果待确认", "平台审核中", "已发布", "执行失败"]
      : t.status === "等待人工"
        ? ["执行中", "结果待确认"]
        : ["结果待确认", "平台审核中"].includes(t.status)
          ? ["已发布", "执行失败"]
          : [];
}
function taskResult(t, status) {
  try {
    if (status === "执行中" && t.status === "等待人工") {
      db.accounts.find((a) => a.id === t.accountId).loginValid = true;
      t.status = "排队中";
    }
    if (status === "执行中") {
      if (!claimTask(db, t))
        throw Error("领取受阻：检查账号串行、批次暂停、定时时间或并发上限");
      return;
    }
    if (
      status === "已取消" &&
      (!["待执行", "排队中"].includes(t.status) || t.claimed)
    )
      throw Error("在途或未知任务不可直接取消");
    if (["已取消", "已放弃"].includes(t.status))
      throw Error("已终结任务不可恢复");
    if (
      ["等待人工", "结果待确认", "平台审核中", "已发布", "执行失败"].includes(
        status,
      ) &&
      !["执行中", "等待人工", "结果待确认", "平台审核中"].includes(t.status)
    )
      throw Error("请先合法领取任务再更新演示结果");
    if (
      ["结果待确认", "平台审核中"].includes(t.status) &&
      ["已发布", "执行失败"].includes(status) &&
      !t.verifiedAt
    )
      throw Error("先保存演示核验依据，再确认结果");
    setResult(t, status, {
      evidence:
        status === "执行失败"
          ? "本地演示明确拒绝，确认原尝试结束且未发布"
          : t.evidence || "",
    });
  } catch (e) {
    warning(e.message);
  }
}
async function takeControl(item) {
  const b = db.batches.find((b) => b.id === item.id),
    p = associatedPlan(db, item);
  if (!p) return true;
  if (
    !(await feedback.confirmWarning({
      title: "人工接管计划批次？",
      message: `将取得计划“${p.name}”当前批次调度权并永久锁定结构，暂停后续自动调度；不能跳过前序批次。`,
      confirmText: "确认接管并锁定",
    }))
  )
    return false;
  return true;
}
async function start(t, silent = false) {
  const errors = validate(t, db, t.id);
  if (errors.length) {
    warning(errors.join("；"));
    return false;
  }
  if (t.batchId && !canRetry(t)) {
    if (!(await takeControl({ kind: "batch", id: t.batchId }))) return false;
    try {
      takeBatchControl(
        db,
        db.batches.find((b) => b.id === t.batchId),
        true,
      );
    } catch (e) {
      warning(e.message);
      return false;
    }
  }
  try {
    const ok = queueTask(db, t, { retry: canRetry(t) });
    if (ok) t.manualDispatch = true;
    if (ok && !silent) toast("已进入本地演示队列");
    return ok;
  } catch (e) {
    warning(e.message);
    return false;
  }
}
async function release(t) {
  if (
    !["已取消", "执行失败"].includes(t.status) ||
    t.claimed ||
    (t.status === "执行失败" && !canRetry(t))
  )
    return;
  if (
    !(await feedback.confirmWarning({
      title: "放弃并释放成片？",
      message:
        "会释放该成片在当前平台的占用，原任务不可恢复。其他平台任务不受影响。",
      confirmText: "放弃并释放",
      danger: true,
    }))
  )
    return;
  setResult(t, "已放弃");
}
async function bulk(action) {
  const rows = chosen.value
      .map((id) => db.tasks.find((t) => t.id === id))
      .filter(Boolean),
    done = [],
    skip = [];
  for (const t of rows) {
    if (action === "启动" && t.status === "待执行") {
      if (await start(t, true)) done.push(t.id);
      else skip.push({ id: t.id, reason: "执行前内容检查未通过" });
    } else if (action === "重试" && canRetry(t)) {
      if (await start(t, true)) done.push(t.id);
      else skip.push({ id: t.id, reason: "执行前内容检查未通过" });
    } else if (action === "取消" && ["待执行", "排队中"].includes(t.status)) {
      taskResult(t, "已取消");
      done.push(t.id);
    } else skip.push({ id: t.id, reason: `${t.status}不支持${action}` });
  }
  bulkResult.value = { action, done, skip };
  chosen.value = [];
}
async function batchAction(b) {
  if (b.control === "执行中") {
    pauseBatch(b);
    return;
  }
  if (b.phase !== "内容预检通过") {
    composerRef.value?.openBatch(b);
    return;
  }
  const p = associatedPlan(db, { kind: "batch", id: b.id });
  if (p && !(await takeControl({ kind: "batch", id: b.id }))) return;
  try {
    dispatchBatch(db, b, { takeover: !!p });
    toast("批次已启动本地演示队列");
  } catch (e) {
    warning(e.message);
  }
}
async function cancelBatch(b) {
  const i = cancellationImpact(db, b);
  if (
    !(await feedback.confirmWarning({
      title: "取消批次并安全收口？",
      message: `停止新派发；未领取 ${i.pending} 条将取消且保留成片占用；在途 ${i.inFlight} 条安全收尾；未知 ${i.unknown} 条继续核实。确认时重新检查领取状态。`,
      confirmText: "确认取消批次",
    }))
  )
    return;
  cancelBatchSafely(db, b);
  toast("已停止派发，未领取项已安全取消");
}
async function cancelCurrentPlan(p) {
  const rows = planTasks(p);
  if (
    !(await feedback.confirmWarning({
      title: "终止计划后续调度？",
      message: `涉及 ${p.items.length - p.position} 个未处理批次，已有 ${rows.length} 条任务保持原状态，${rows.filter((t) => ["执行中", "等待人工", "结果待确认"].includes(t.status)).length} 条在途继续安全收尾。不撤销作品、不释放占用。`,
      confirmText: "取消计划",
    }))
  )
    return;
  cancelPlan(p);
  toast("计划后续调度已终止");
}
function configureBatch(b) {
  detailOpen.value = false;
  if (b.phase !== "内容预检通过") composerRef.value?.openBatch(b);
  else view(b);
}
watch(
  () => db.tasks.map((t) => `${t.id}:${t.status}`).join("|"),
  () => {
    if (!hydrated.value) return;
    for (const b of db.batches)
      if (
        b.control === "执行中" &&
        batchTasks(db, b).length &&
        !batchTasks(db, b).some((t) =>
          ["待执行", "排队中", "执行中", "等待人工", "结果待确认"].includes(
            t.status,
          ),
        )
      )
        b.control = "执行已结束";
    for (const p of db.plans)
      try {
        syncPlan(db, p);
      } catch (error) {
        if (p.control !== "等待配置") p.control = "已暂停";
        warning(error.message);
      }
  },
  { flush: "post" },
);
const listColumns = computed(() => {
  const first = [
    ...(["tasks", "images"].includes(page.value)
      ? [{ colKey: "row-select", type: "multiple", width: 44 }]
      : []),
    { colKey: "id", title: "ID", width: 85 },
    {
      colKey: "content",
      title: page.value === "tasks" ? "发布内容" : "名称",
      minWidth: 240,
      ellipsis: true,
    },
  ];
  const middle =
    page.value === "tasks"
      ? [
          { colKey: "scope", title: "平台 / 类目", width: 140 },
          { colKey: "account", title: "发布账号", width: 150 },
          { colKey: "association", title: "批次 / 计划", minWidth: 185 },
          { colKey: "status", title: "执行状态 / 结果", width: 150 },
        ]
      : page.value === "batches"
        ? [
            { colKey: "scope", title: "平台 / 类目", minWidth: 175 },
            { colKey: "resourceCount", title: "成片 / 任务", width: 110 },
            { colKey: "progress", title: "已公开 / 待执行 / 在途", width: 180 },
            { colKey: "problems", title: "失败 / 待处理", width: 130 },
            { colKey: "control", title: "调度状态", width: 125 },
          ]
        : page.value === "plans"
          ? [
              { colKey: "target", title: "目标 / 已发布", width: 150 },
              { colKey: "gap", title: "目标差额", width: 110 },
              { colKey: "control", title: "调度状态", width: 120 },
            ]
          : page.value === "images"
            ? [
                { colKey: "category", title: "类目", width: 130 },
                { colKey: "uses", title: "用途", minWidth: 160 },
                { colKey: "enabled", title: "状态", width: 95 },
                {
                  colKey: "metadata",
                  title: "尺寸 / 格式 / 来源 / 更新时间",
                  minWidth: 215,
                },
                { colKey: "references", title: "引用 / 版本", width: 110 },
              ]
            : [
                { colKey: "ratio", title: "画布规格", width: 140 },
                { colKey: "layers", title: "图层", width: 100 },
                { colKey: "references", title: "引用 / 版本", width: 110 },
                { colKey: "enabled", title: "状态", width: 95 },
              ];
  return [
    ...first,
    ...middle,
    {
      colKey: "operation",
      title: "操作",
      width:
        page.value === "tasks" ? 250 : page.value === "templates" ? 285 : 215,
      fixed: compact.value ? undefined : "right",
    },
  ];
});
const composerRef = ref(null),
  planWizardRef = ref(null);
const rowEdit = ref(null),
  rowEditOpen = ref(false),
  editingTaskId = ref(null),
  rowError = ref("");
const rowOriginal = ref("");
const platformTopics = computed({
  get: () =>
    rowEdit.value?.platform === "B站"
      ? rowEdit.value?.tags
      : rowEdit.value?.topics,
  set: (v) => {
    if (rowEdit.value)
      rowEdit.value[rowEdit.value.platform === "B站" ? "tags" : "topics"] = v;
  },
});
function newPublish(mode = "快速发布", planId = null) {
  composerRef.value?.open(mode, planId);
}
async function onCreated({ batch, tasks }) {
  const saved = await persistNow();
  navigate(batch ? "batches" : "tasks");
  view(batch || tasks[0]);
  if (saved) toast(`已保存 ${tasks.length} 条待执行任务，尚未启动`);
}
const mediaPreviewOpen = ref(false),
  previewImages = ref([]),
  previewRow = ref(null),
  previewTitle = ref("");
async function openPreview(payload) {
  previewRow.value = payload.row || null;
  previewTitle.value = payload.row
    ? "最终发布内容预览"
    : payload.title || "图片预览";
  const r = payload.row;
  if (r) {
    try {
      await ensureCoverResults(r);
    } catch (e) {
      warning(e.message);
    }
  }
  previewImages.value = r
    ? coverSpecs(r.platform).length
      ? coverSpecs(r.platform).map((spec) => ({
          src: r.coverVariants?.[spec.key] || r.cover,
          label: spec.label,
          note: "当前实际生成结果，请检查裁切与文字可读性。",
        }))
      : [
          {
            src: r.cover,
            label: "原始封面",
            note: "百家号横竖封面规格待核验，当前仅预览已有图片。",
          },
        ]
    : [{ src: payload.src, label: payload.title || "图片" }];
  mediaPreviewOpen.value = true;
}
async function closeRow() {
  if (
    JSON.stringify(rowEdit.value) !== rowOriginal.value &&
    !(await feedback.confirmWarning({
      title: "放弃本条修改？",
      message: "尚未保存的修改将丢失，其他任务配置不受影响。",
      confirmText: "放弃修改",
    }))
  )
    return;
  rowEditOpen.value = false;
}
async function editRow(row, task = false) {
  if (task && !(row.status === "待执行" || canRetry(row))) {
    warning("当前状态不允许修改已提交快照");
    return;
  }
  rowEdit.value = clone(row);
  try {
    await ensureCoverResults(rowEdit.value);
  } catch (e) {
    warning(e.message);
  }
  rowOriginal.value = JSON.stringify(rowEdit.value);
  editingTaskId.value = task ? row.id : null;
  rowError.value = "";
  rowEditOpen.value = true;
}
async function saveRow() {
  rowError.value = "";
  try {
    await ensureCoverResults(rowEdit.value);
  } catch (e) {
    rowError.value = e.message;
    return;
  }
  rowEdit.value.contentApproved = true;
  rowEdit.value.coverApproved = true;
  const errors = validate(rowEdit.value, db, editingTaskId.value);
  if (errors.length) {
    rowError.value = errors.join("；");
    return;
  }
  if (editingTaskId.value) {
    Object.assign(
      db.tasks.find((t) => t.id === editingTaskId.value),
      rowEdit.value,
      {
        version: (rowEdit.value.version || 1) + 1,
        snapshot: clone(rowEdit.value),
      },
    );
  } else {
    composerRef.value?.updateRow(rowEdit.value);
  }
  await nextTick();
  if (!(await persistNow())) {
    rowError.value = "修改尚未保存，请重试；当前输入已保留";
    return;
  }
  rowEditOpen.value = false;
}
const imagePicker = ref(false),
  imageSearch = ref(""),
  imageCategory = ref(""),
  coverTarget = ref("row"),
  selectedImage = ref(null);
const imageDraft = reactive({ search: "", category: "" });
const imageContextCategory = computed(() =>
  coverTarget.value === "row" || editorPurpose.value === "cover"
    ? rowEdit.value?.category
    : "",
);
const availableImages = computed(() =>
  db.images.filter(
    (i) =>
      i.enabled &&
      i.uses.includes(coverTarget.value === "editor" ? "底图" : "现成封面") &&
      i.name.includes(imageSearch.value) &&
      (!imageCategory.value ||
        (imageCategory.value === "适用"
          ? [imageContextCategory.value, "通用"].includes(i.category)
          : i.category === imageCategory.value)),
  ),
);
function openImages(target) {
  coverTarget.value = target;
  imagePicker.value = true;
  selectedImage.value = null;
  imageSearch.value = "";
  imageCategory.value = imageContextCategory.value ? "适用" : "";
  imageDraft.search = "";
  imageDraft.category = imageCategory.value;
}
function useImage() {
  const im = db.images.find((i) => i.id === selectedImage.value);
  if (!im) return;
  if (coverTarget.value === "editor") {
    imageEditorRef.value?.setBackground(im.src);
    editorDoc.value.imageId = im.id;
  } else {
    rowEdit.value.imageId = im.id;
    rowEdit.value.imageVersion = im.version || 1;
    rowEdit.value.cover = im.src;
    rowEdit.value.coverSource = im.src;
    rowEdit.value.coverVariants = {};
    ensureCoverResults(rowEdit.value).catch((e) => warning(e.message));
  }
  imagePicker.value = false;
}
function randomCover() {
  const rows = db.images.filter(
    (i) => i.enabled && i.uses.includes("现成封面"),
  );
  let pool = rows.filter((i) => i.category === rowEdit.value.category);
  if (!pool.length) pool = rows.filter((i) => i.category === "通用");
  if (!pool.length) {
    warning("没有匹配类目或通用的可用封面");
    return;
  }
  rowEdit.value.cover = pool[Math.floor(Math.random() * pool.length)].src;
  rowEdit.value.coverSource = rowEdit.value.cover;
  rowEdit.value.coverVariants = {};
  ensureCoverResults(rowEdit.value).catch((e) => warning(e.message));
}
async function removeImage(im) {
  if (imageReferences(db, im).length) {
    warning("图片已被引用，只能停用，历史快照保留");
    return;
  }
  if (
    await feedback.confirmWarning({
      title: "删除图片？",
      message: "图片将不再用于新任务，已创建任务的封面快照保留。",
      confirmText: "删除图片",
      danger: true,
    })
  )
    deleteImage(db, im);
}
const imageOriginal = ref("");
let imageReadToken = 0;
const imageReading = ref(false);
const imageForm = reactive({
    open: false,
    id: null,
    name: "",
    category: "通用",
    uses: ["现成封面"],
    enabled: true,
    src: "",
  }),
  imageError = ref("");
async function setDefaultTemplate(row) {
  if (!row.enabled) {
    warning("请先启用模板");
    return;
  }
  db.templates
    .filter((t) => t.ratio === row.ratio)
    .forEach((t) => (t.isDefault = t.id === row.id));
  if (await persistNow()) toast("已设置为该画布规格的默认模板");
}
function imageEdit(i) {
  Object.assign(
    imageForm,
    i
      ? clone(i)
      : {
          id: null,
          name: "",
          category: "通用",
          uses: ["现成封面"],
          enabled: true,
          src: "",
        },
  );
  imageForm.open = true;
  imageOriginal.value = JSON.stringify(imageForm);
  imageError.value = "";
}
async function closeImage() {
  if (
    JSON.stringify(imageForm) !== imageOriginal.value &&
    !(await feedback.confirmWarning({
      title: "放弃图片修改？",
      message: "当前未保存的名称、用途和图片将丢失。",
      confirmText: "放弃修改",
    }))
  )
    return;
  imageReadToken++;
  imageReading.value = false;
  imageForm.open = false;
}
function readImage(e) {
  const f = e.target.files?.[0];
  if (!f) return;
  if (
    !["image/png", "image/jpeg", "image/webp"].includes(f.type) ||
    f.size > 3 * 1024 * 1024
  ) {
    imageError.value =
      "请选择不超过 3MB 的 PNG、JPEG 或 WebP（本地演示存储限制）";
    return;
  }
  const token = ++imageReadToken;
  imageReading.value = true;
  const reader = new FileReader();
  reader.onerror = () => {
    if (token !== imageReadToken) return;
    imageReading.value = false;
    imageError.value = "读取失败，请重新选择图片";
  };
  reader.onload = () => {
    if (token !== imageReadToken) return;
    imageForm.src = reader.result;
    imageForm.name ||= f.name;
    imageForm.format = f.type.split("/")[1].toUpperCase();
    imageForm.size = f.size;
    imageForm.source = "人工上传（本地演示）";
    const im = new Image();
    im.onload = () => {
      if (token !== imageReadToken) return;
      imageReading.value = false;
      imageError.value = "";
      imageForm.width = im.naturalWidth;
      imageForm.height = im.naturalHeight;
    };
    im.onerror = () => {
      if (token !== imageReadToken) return;
      imageReading.value = false;
      imageForm.src = "";
      imageError.value = "图片损坏，请重新选择";
    };
    im.src = reader.result;
  };
  reader.readAsDataURL(f);
}
async function saveImage() {
  if (imageReading.value) {
    imageError.value = "正在读取图片尺寸，请稍后保存";
    return;
  }
  if (!imageForm.name.trim() || !imageForm.src || !imageForm.uses.length) {
    imageError.value = "请填写名称、选择图片和至少一种用途";
    return;
  }
  const row = clone(imageForm);
  delete row.open;
  row.id ||= id();
  row.version = (db.images.find((i) => i.id === row.id)?.version || 0) + 1;
  row.updatedAt = new Date().toISOString();
  const at = db.images.findIndex((i) => i.id === row.id);
  if (at < 0) db.images.push(row);
  else db.images[at] = row;
  imageForm.id = row.id;
  if (!(await persistNow())) {
    imageError.value = "图片尚未保存，请重试；当前输入已保留";
    return;
  }
  imageForm.open = false;
}
const imageEditorRef = ref(null);
const editorOpen = ref(false),
  editorDoc = ref(null),
  editorName = ref(""),
  editorPlatforms = ref([]),
  editorCategories = ref([]),
  editorId = ref(null),
  editorPurpose = ref("template"),
  editorError = ref(""),
  editorPreview = ref(""),
  editorCoverSpec = ref(null),
  editorOriginal = ref(""),
  editorSaving = ref(false);
function editorReady(doc) {
  editorOriginal.value = JSON.stringify({
    name: editorName.value,
    tags: [editorPlatforms.value, editorCategories.value],
    document: doc,
  });
}
function openEditor(template = null, purpose = "template", spec = null) {
  editorCoverSpec.value = spec ||
    coverSpecs(rowEdit.value?.platform)[0] || {
      key: "source",
      width: 1200,
      height: 900,
      label: "原始封面",
    };
  editorPurpose.value = purpose;
  editorId.value = template?.id || null;
  editorDoc.value = template?.document
    ? clone(template.document)
    : purpose === "cover"
      ? {
          width: 1200,
          height: 900,
          background: "#172f4c",
          image: rowEdit.value.coverSource || rowEdit.value.cover,
          imageFit: "meet",
          layers: [],
        }
      : defaultDocument(
          1200,
          900,
          (
            db.images.find((i) => i.enabled && i.pureBackground) ||
            db.images.find((i) => i.enabled && i.uses.includes("底图"))
          )?.src || "",
        );
  if (purpose === "cover" && template?.document)
    editorDoc.value.layers
      .filter((l) => l.kind === "text" && l.id === "title")
      .forEach((l) => (l.text = rowEdit.value.shortTitle || ""));
  if (purpose === "cover") {
    const { width, height } = editorCoverSpec.value;
    const doc = editorDoc.value;
    const sx = width / doc.width,
      sy = height / doc.height;
    doc.layers.forEach((l) => {
      l.x *= sx;
      l.w *= sx;
      l.y *= sy;
      l.h *= sy;
      if (l.kind === "text") {
        l.fontSize *= Math.min(sx, sy);
        l.minFont *= Math.min(sx, sy);
      }
    });
    doc.width = width;
    doc.height = height;
  }
  editorName.value = template?.name || "新封面模板";
  editorPlatforms.value = clone(template?.tags?.platforms || []);
  editorCategories.value = clone(template?.tags?.categories || []);
  editorPreview.value = template?.preview || "";
  editorError.value = "";
  editorOpen.value = true;
}
async function cancelEditor() {
  if (editorSaving.value) return;
  if (
    JSON.stringify({
      name: editorName.value,
      tags: [editorPlatforms.value, editorCategories.value],
      document: editorDoc.value,
    }) !== editorOriginal.value &&
    !(await feedback.confirmWarning({
      title: "放弃封面修改？",
      message: "尚未保存的设计将丢失，已应用的封面和其他任务不受影响。",
      confirmText: "放弃修改",
    }))
  )
    return;
  editorOpen.value = false;
}
async function saveEditor() {
  if (!editorName.value.trim() || !editorDoc.value) {
    editorError.value = "请输入模板名称并准备画布";
    return;
  }
  editorSaving.value = true;
  let rendered;
  try {
    rendered = await renderDocument(editorDoc.value);
  } catch (e) {
    editorError.value = e.message;
    return;
  } finally {
    editorSaving.value = false;
  }
  if (!rendered) {
    editorError.value = "封面未能生成，请检查溢出、字体及底图";
    return;
  }
  editorPreview.value = rendered;
  if (editorPurpose.value === "cover") {
    const template = db.templates.find((t) => t.id === editorId.value);
    if (template) {
      rowEdit.value.templateId = template.id;
      rowEdit.value.templateVersion = template.version;
      rowEdit.value.templateDocuments = {};
    }
    rowEdit.value.coverSource = editorDoc.value.image;
    rowEdit.value.imageId = editorDoc.value.imageId || null;
    rowEdit.value.imageVersion =
      db.images.find((i) => i.id === rowEdit.value.imageId)?.version || null;
    rowEdit.value.coverVariants ||= {};
    rowEdit.value.coverVariants[editorCoverSpec.value.key] =
      editorPreview.value;
    if (
      !coverSpecs(rowEdit.value.platform).length ||
      editorCoverSpec.value.key === coverSpecs(rowEdit.value.platform)[0]?.key
    )
      rowEdit.value.cover = editorPreview.value;
    rowEdit.value.templateDocument = clone(editorDoc.value);
    rowEdit.value.templateDocuments ||= {};
    rowEdit.value.templateDocuments[editorCoverSpec.value.key] = clone(
      editorDoc.value,
    );
    rowEdit.value.coverMode = "规则生成封面";
    editorOpen.value = false;
    return;
  }
  const item = {
    isDefault:
      db.templates.find((t) => t.id === editorId.value)?.isDefault || false,
    id: editorId.value || id(),
    imageId: editorDoc.value.imageId || null,
    updatedAt: new Date().toISOString(),
    name: editorName.value,
    tags: {
      platforms: clone(editorPlatforms.value),
      categories: clone(editorCategories.value),
    },
    version:
      (db.templates.find((t) => t.id === editorId.value)?.version || 0) + 1,
    document: clone(editorDoc.value),
    preview: editorPreview.value,
    ratio: `${editorDoc.value.width}×${editorDoc.value.height}`,
    enabled: db.templates.find((t) => t.id === editorId.value)?.enabled ?? true,
  };
  const index = db.templates.findIndex((t) => t.id === item.id);
  if (index < 0) db.templates.push(item);
  else db.templates[index] = item;
  editorId.value = item.id;
  editorSaving.value = true;
  const saved = await persistNow();
  editorSaving.value = false;
  if (!saved) {
    editorError.value = "模板尚未保存，请重试；当前设计已保留";
    return;
  }
  editorOpen.value = false;
  toast("模板已保存，可重新打开编辑");
}
const planView = ref("总览");
function editPlan(p) {
  if (p?.lockedAt || ["已取消", "执行已结束"].includes(p?.control)) {
    warning("计划结构已锁定");
    return;
  }
  planWizardRef.value?.open(p);
}
const planTasks = (p) => collectPlanTasks(db, p),
  planTarget = (p) => p.targets.reduce((sum, t) => sum + Number(t.count), 0);
function currentDecision(p) {
  return decision(db, p);
}
function returnToPlan(p) {
  view(p);
}
const planStats = computed(() =>
  detail.value?.items
    ? stats(planTasks(detail.value), planTarget(detail.value))
    : null,
);
const planGroups = computed(() => {
  const p = detail.value;
  if (!p?.items) return [];
  const groups = new Map();
  for (const target of p.targets) {
    const key =
      planView.value === "按平台"
        ? target.platform
        : planView.value === "按类目"
          ? target.category
          : target.platform + " · " + target.category;
    if (!groups.has(key))
      groups.set(key, {
        name: key,
        platform: planView.value === "按类目" ? null : target.platform,
        category: planView.value === "按平台" ? null : target.category,
        target: 0,
      });
    groups.get(key).target += Number(target.count);
  }
  return [...groups.values()].map((g) => ({
    ...g,
    ...stats(
      planTasks(p).filter(
        (t) =>
          (!g.platform || t.platform === g.platform) &&
          (!g.category || t.category === g.category),
      ),
      g.target,
    ),
  }));
});
function startPlan(p) {
  try {
    dispatchPlan(db, p);
  } catch (error) {
    warning(error.message);
  }
}
async function advancePlan(p) {
  const decision = currentDecision(p);
  if (decision === "等待核实") {
    warning("请先处理当前项目中的待执行、在途、人工和未知结果");
    return;
  }
  if (
    decision !== "自动继续" &&
    !(await feedback.confirmWarning({
      title:
        decision === "等待跳过确认" ? "跳过当前项目调度？" : "继续下一项目？",
      message:
        decision === "等待跳过确认"
          ? "不会取消任务或释放占用。"
          : "当前批次失败率超过 10%，失败任务保留在原批次。",
      confirmText: "确认并继续",
    }))
  )
    return;
  try {
    advance(db, p, true);
  } catch (error) {
    warning(error.message);
  }
}
function linkEvidence() {
  if (!/^https:\/\//.test(evidence.value)) {
    warning("请输入完整的 HTTPS 作品链接作为演示核验依据");
    return;
  }
  detail.value.evidence = evidence.value;
  detail.value.verifiedAt = new Date().toISOString();
  toast("已保存核验依据；请用下方演示控制明确核验结果");
}
function cloneTask(t) {
  composerRef.value?.open("快速发布", null, t);
  detailOpen.value = false;
}
</script>

<template>
  <div
    v-if="hydrated"
    class="publish-shell wt-resource-page resource-visual-approved"
  >
    <aside class="publish-nav">
      <a href="/example/" class="brand">WT <span>MEDIA</span></a>
      <p>内容运营</p>
      <button
        v-for="p in ['tasks', 'batches', 'plans', 'images', 'templates']"
        :key="p"
        :class="{ active: page === p }"
        @click="navigate(p)"
      >
        {{ names[p] }}
      </button>
      <div class="nav-bottom">
        <a href="/example/proto/content-operations/">内容生产工作台 ↗</a
        ><a href="/__docs/workspace/docs/design/publishing.md">发布设计说明 ↗</a
        ><small>交互原型 · 仅本地演示<br />不会上传或发布到真实平台</small>
      </div>
    </aside>
    <main class="publish-main">
      <div class="breadcrumbs">
        内容运营 <span>/</span> {{ names[page] }}
        <t-tag size="small" variant="light">原型预览</t-tag>
      </div>
      <ResourcePageHeader
        :title="names[page]"
        :description="
          {
            tasks: '检查发布内容，跟进执行与平台结果。',
            batches: '集中配置和管理一组发布任务。',
            plans: '盘点资源、设定目标，按顺序组织发布。',
            images: '管理现成封面和可用于生成封面的底图。',
            templates: '管理画布、文字区域与可复用的封面设计。',
          }[page]
        "
        ><template #actions
          ><t-button v-if="page === 'tasks'" @click="newPublish('快速发布')"
            >快速发布</t-button
          ><t-button v-if="page === 'batches'" @click="newPublish('批量发布')"
            >批量发布</t-button
          ><t-button v-if="page === 'plans'" @click="editPlan()"
            >新建发布计划</t-button
          ><t-button v-if="page === 'images'" @click="imageEdit()"
            >上传图片</t-button
          ><t-button v-if="page === 'templates'" @click="openEditor()"
            >新建模板</t-button
          ></template
        ></ResourcePageHeader
      >
      <ResourceNotice
        v-if="composerRef?.hasDraft && ((page === 'tasks' && composerRef?.draftMode === '快速发布') || (page === 'batches' && composerRef?.draftMode !== '快速发布'))"
        tone="info"
      >
        有未完成的{{ page === 'tasks' ? '快速发布' : '批量发布' }}配置。
        <t-button variant="text" @click="composerRef.resume()">继续配置</t-button>
      </ResourceNotice>
      <t-alert
        v-if="db.legacyPublishDraft"
        theme="info"
        message="旧多平台未完成配置已保留为兼容记录；请按单平台重新创建批次，避免混合提交。"
      />
      <t-alert v-if="storageError" theme="warning" :message="storageError"
        ><template #operation
          ><t-button
            variant="text"
            :loading="savingSnapshot"
            @click="persistNow"
            >重试保存</t-button
          ></template
        ></t-alert
      >
      <details class="prototype-scenarios">
        <summary>可复现演示场景（新增虚构数据，保留已有记录）</summary>
        <label
          ><input
            type="checkbox"
            v-model="simulateSaveFailure"
          />模拟本地保存失败（取消后重试）</label
        >
        <div class="inline-bar">
          <t-button
            v-for="kind in ['30/28', '10%', '20%', '未知']"
            :key="kind"
            variant="outline"
            @click="view(addScenario(db, kind))"
            >载入 {{ kind }} 场景</t-button
          >
        </div>
        <p>
          仅为回归验收生成独立的示例计划、账号与成片；AI
          和发布结果均是演示数据。
        </p>
      </details>
      <ResourceCard>
        <ResourceFilterBar>
          <label class="wt-filter-field wt-filter-field--wide"
            ><span class="wt-filter-field__label">名称</span
            ><t-input
              v-model="query"
              placeholder="输入名称查询"
              clearable
              @enter="applyQuery"
          /></label>
          <label
            v-if="['tasks', 'batches'].includes(page)"
            class="wt-filter-field"
            ><span class="wt-filter-field__label">平台</span
            ><t-select
              :popup-props="{
                hideEmptyPopup: false,
                attach: 'body',
                zIndex: 5500,
              }"
              v-model="filterPlatform"
              :options="[
                { label: '全部平台', value: '' },
                ...platforms.map((p) => ({ label: p, value: p })),
              ]"
          /></label>
          <label v-if="page === 'tasks'" class="wt-filter-field"
            ><span class="wt-filter-field__label">执行状态</span
            ><t-select
              :popup-props="{
                hideEmptyPopup: false,
                attach: 'body',
                zIndex: 5500,
              }"
              v-model="filterStatus"
              :options="[
                { label: '全部状态', value: '' },
                ...states.map((s) => ({ label: s, value: s })),
              ]"
          /></label>
          <label
            v-if="['images', 'templates'].includes(page)"
            class="wt-filter-field"
            ><span class="wt-filter-field__label">启停</span
            ><t-select
              v-model="filterStatus"
              :options="[
                { label: '全部状态', value: '' },
                { label: '启用', value: 'true' },
                { label: '停用', value: 'false' },
              ]"
          /></label>
          <label v-if="page === 'images'"
            >用途<select v-model="libraryFilter.use">
              <option value="">全部用途</option>
              <option>现成封面</option>
              <option>底图</option>
            </select></label
          >
          <label v-if="['images', 'templates'].includes(page)"
            >比例<select v-model="libraryFilter.ratio">
              <option value="">全部比例</option>
              <option :value="4 / 3">4:3</option>
              <option :value="16 / 9">16:9</option>
              <option :value="3 / 4">3:4</option>
              <option value="1">1:1</option>
            </select></label
          >
          <label v-if="page === 'images'"
            >来源<select v-model="libraryFilter.source">
              <option value="">全部来源</option>
              <option>种子资源</option>
              <option>人工上传</option>
            </select></label
          >
          <div class="wt-filter-actions">
            <t-button @click="applyQuery">查询</t-button
            ><t-button theme="default" variant="outline" @click="resetQuery"
              >重置</t-button
            >
          </div>
          <ResourceAdvancedToggle
            v-model="advanced"
            :count="appliedFilters.category ? 1 : 0"
            controls="publishing-advanced"
          />
          <div
            v-if="advanced"
            id="publishing-advanced"
            class="wt-advanced-filters"
          >
            <label class="wt-filter-field"
              ><span class="wt-filter-field__label">内容类目</span
              ><t-select
                :popup-props="{
                  hideEmptyPopup: false,
                  attach: 'body',
                  zIndex: 5500,
                }"
                v-model="filterCategory"
                :options="[
                  { label: '全部类目', value: '' },
                  ...categories.map((c) => ({ label: c, value: c })),
                ]"
            /></label>
          </div>
        </ResourceFilterBar>
        <ResourceNotice v-if="batchFilter || planFilter"
          >当前范围：{{
            batchFilter ? "批次 " + batchFilter : "计划 " + planFilter
          }}
          <t-button
            variant="text"
            @click="
              batchFilter = null;
              planFilter = null;
            "
            >清除范围</t-button
          ></ResourceNotice
        >
        <ResourceSelectionToolbar
          :count="page === 'tasks' ? chosen.length : 0"
          unit="条任务"
          @clear="chosen = []"
          ><t-button variant="text" @click="bulk('启动')">启动待执行项</t-button
          ><t-button variant="text" @click="bulk('重试')"
            >重试确认失败项</t-button
          ><t-button variant="text" @click="bulk('取消')"
            >取消未执行项</t-button
          ></ResourceSelectionToolbar
        >
        <ResourceSelectionToolbar
          v-if="page === 'images'"
          :count="chosen.length"
          unit="张图片"
          @clear="chosen = []"
          ><t-button
            variant="text"
            @click="
              db.images
                .filter((i) => chosen.includes(i.id))
                .forEach((i) => (i.uses = ['现成封面', '底图']))
            "
            >批量标记双用途</t-button
          ><t-button
            variant="text"
            @click="
              db.images
                .filter((i) => chosen.includes(i.id))
                .forEach((i) => (i.enabled = true))
            "
            >批量启用</t-button
          ><t-button
            variant="text"
            @click="
              db.images
                .filter((i) => chosen.includes(i.id))
                .forEach((i) => (i.enabled = false))
            "
            >批量停用</t-button
          ></ResourceSelectionToolbar
        >
        <t-table
          class="wt-resource-table"
          row-key="id"
          :data="paged"
          :columns="listColumns"
          :selected-row-keys="chosen"
          @select-change="(keys) => (chosen = keys)"
          :pagination="{
            current: pageNo,
            pageSize: size,
            total: records.length,
            showJumper: false,
            pageSizeOptions: [8, 20, 50],
          }"
          @page-change="
            (p) => {
              pageNo = p.current;
              size = p.pageSize;
            }
          "
        >
          <template #content="{ row }"
            ><div class="title-cell">
              <button
                v-if="row.cover || row.src || row.preview"
                class="media-thumb"
                @click="
                  openPreview(
                    row.assetId
                      ? { row }
                      : { src: row.src || row.preview, title: row.name },
                  )
                "
              >
                <img
                  :src="row.cover || row.src || row.preview"
                  alt="查看大图"
                />
              </button>
              <div>
                <t-tooltip
                  :content="row.name || row.title || asset(row.assetId)?.title"
                  ><strong>{{
                    row.name || row.title || asset(row.assetId)?.title
                  }}</strong></t-tooltip
                ><small v-if="page === 'tasks'"
                  >成片 {{ row.assetId }} ·
                  {{ asset(row.assetId)?.duration }}</small
                >
              </div>
            </div></template
          >
          <template #scope="{ row }"
            >{{ row.platform
            }}<small class="cell-secondary">{{
              row.category || row.categories?.join("、")
            }}</small></template
          >
          <template #account="{ row }"
            >{{ account(row.accountId)?.name
            }}<small class="cell-secondary"
              >窗口 {{ account(row.accountId)?.window }}</small
            ></template
          >
          <template #association="{ row }"
            ><t-button
              v-if="row.batchId"
              variant="text"
              @click="view(db.batches.find((b) => b.id === row.batchId))"
              >{{
                db.batches.find((b) => b.id === row.batchId)?.name
              }}</t-button
            ><span v-else>独立任务</span
            ><small class="cell-secondary">{{
              associatedPlan(db, {
                kind: row.batchId ? "batch" : "task",
                id: row.batchId || row.id,
              })?.name || "未关联计划"
            }}</small></template
          >
          <template #status="{ row }"
            ><t-tag :theme="stateTheme(row.status)" variant="light">{{
              row.status
            }}</t-tag
            ><small class="cell-secondary">{{ row.result }}</small></template
          >
          <template #resourceCount="{ row }"
            >{{ batchSummary(db, row).videos }} /
            {{ batchSummary(db, row).total }}</template
          >
          <template #progress="{ row }"
            >{{ batchSummary(db, row).published }} /
            {{ batchSummary(db, row).waiting }} /
            {{ batchSummary(db, row).running }}</template
          >
          <template #problems="{ row }"
            >{{ batchSummary(db, row).failed }} /
            {{ batchSummary(db, row).unresolved }}</template
          >
          <template #target="{ row }"
            >{{ planTarget(row) }} /
            {{ stats(planTasks(row)).published }}</template
          >
          <template #gap="{ row }">{{
            stats(planTasks(row), planTarget(row)).gap
          }}</template>
          <template #metadata="{ row }"
            >{{ row.width || 1200 }} × {{ row.height || 900 }} ·
            {{ row.format || "SVG 演示图"
            }}<small class="cell-secondary"
              >{{ row.source || "种子资源" }} ·
              {{ row.updatedAt || "2026-11-10" }}</small
            ></template
          >
          <template #references="{ row }"
            ><t-button
              variant="text"
              @click="
                referenceRows =
                  page === 'images'
                    ? imageReferences(db, row)
                    : templateReferences(db, row).map((t) => ({
                        kind: '正式任务',
                        id: t.id,
                        name: t.title,
                      }))
              "
              >{{
                page === "images"
                  ? imageReferences(db, row).length
                  : templateReferences(db, row).length
              }}
              次 / v{{ row.version || 1 }}</t-button
            ></template
          >
          <template #uses="{ row }">{{ row.uses.join("、") }}</template>
          <template #enabled="{ row }"
            ><t-tag
              :theme="row.enabled ? 'success' : 'default'"
              variant="light"
              >{{ row.enabled ? "启用" : "停用" }}</t-tag
            ></template
          >
          <template #layers="{ row }"
            >{{ row.document.layers.length }} 个图层<small
              class="cell-secondary"
              >动态槽
              {{
                row.document.layers.filter((l) => l.slot || l.id === "title")
                  .length
              }}
              ·
              {{
                [
                  ...(row.tags?.platforms || []),
                  ...(row.tags?.categories || []),
                ].join(" / ") || "跨平台通用"
              }}</small
            ></template
          >
          <template #operation="{ row }">
            <ResourceRowActions>
              <template v-if="page === 'tasks'"
                ><t-button variant="text" @click="view(row)">详情</t-button
                ><t-button
                  v-if="row.status === '待执行' || canRetry(row)"
                  variant="text"
                  @click="start(row)"
                  >{{ canRetry(row) ? "重试" : "启动" }}</t-button
                ><t-button
                  v-if="row.status === '待执行' || canRetry(row)"
                  variant="text"
                  @click="editRow(row, true)"
                  >编辑</t-button
                ><t-button
                  v-if="['待执行', '排队中'].includes(row.status)"
                  variant="text"
                  @click="taskResult(row, '已取消')"
                  >取消</t-button
                ><t-button
                  v-if="['已取消', '执行失败'].includes(row.status)"
                  theme="danger"
                  variant="text"
                  @click="release(row)"
                  >放弃释放</t-button
                ></template
              ><template v-else-if="['batches', 'plans'].includes(page)"
                ><t-button variant="text" @click="view(row)">详情</t-button
                ><t-button
                  v-if="
                    page === 'batches' &&
                    !['已取消', '执行已结束'].includes(row.control)
                  "
                  variant="text"
                  @click="batchAction(row)"
                  >{{
                    row.phase !== "内容预检通过"
                      ? "配置资源 / 内容"
                      : row.control === "执行中"
                        ? "暂停"
                        : "启动 / 恢复"
                  }}</t-button
                ><t-button
                  v-else-if="
                    page === 'plans' &&
                    !row.lockedAt &&
                    !['已取消', '执行已结束'].includes(row.control)
                  "
                  variant="text"
                  @click="editPlan(row)"
                  >编辑目标</t-button
                ></template
              ><template v-else-if="page === 'images'"
                ><t-button variant="text" @click="imageEdit(row)">编辑</t-button
                ><t-button variant="text" @click="row.enabled = !row.enabled">{{
                  row.enabled ? "停用" : "启用"
                }}</t-button
                ><t-button
                  variant="text"
                  theme="danger"
                  :disabled="!!imageReferences(db, row).length"
                  @click="removeImage(row)"
                  >删除</t-button
                ></template
              ><template v-else
                ><t-button variant="text" @click="openEditor(row)"
                  >编辑</t-button
                ><t-button
                  variant="text"
                  @click="
                    db.templates.push({
                      ...clone(row),
                      id: id(),
                      isDefault: false,
                      name: row.name + ' 副本',
                    })
                  "
                  >复制</t-button
                ><t-button
                  variant="text"
                  :disabled="row.isDefault"
                  @click="setDefaultTemplate(row)"
                  >{{ row.isDefault ? "默认模板" : "设为默认" }}</t-button
                ><t-button variant="text" @click="row.enabled = !row.enabled">{{
                  row.enabled ? "停用" : "启用"
                }}</t-button></template
              >
            </ResourceRowActions></template
          >
          <template #empty
            ><ResourceEmptyState
              :title="
                page === 'templates' ? '还没有封面模板' : '没有符合条件的记录'
              "
              :description="
                page === 'templates'
                  ? '设计一次版式，在发布时替换标题和底图即可复用。'
                  : '调整筛选条件，或新增一条记录。'
              "
              ><t-button v-if="page === 'templates'" @click="openEditor()"
                >创建第一个模板</t-button
              ></ResourceEmptyState
            ></template
          >
        </t-table>
      </ResourceCard>
    </main>
    <PlanWizard
      ref="planWizardRef"
      :db="db"
      :persist="persistNow"
      @saved="view"
    />
    <PublishComposer
      ref="composerRef"
      :db="db"
      @edit="editRow"
      @preview="openPreview"
      @created="onCreated"
    />
    <MediaPreview
      v-model:visible="mediaPreviewOpen"
      :title="previewTitle"
      :images="previewImages"
    >
      <template v-if="previewRow" #details>
        <div class="preview-publishing-details">
          <t-tag variant="light">{{ previewRow.platform }}</t-tag>
          <h3>
            {{
              previewRow.platform === "百家号"
                ? previewRow.description
                : previewRow.title
            }}
          </h3>
          <p class="preview-description">
            {{
              previewRow.platform === "百家号"
                ? ""
                : previewRow.description || "未填写正文 / 简介"
            }}
          </p>
          <dl>
            <dt>发布账号</dt>
            <dd>
              {{ account(previewRow.accountId)?.name }} · 窗口
              {{ account(previewRow.accountId)?.window }}
            </dd>
            <dt>成片</dt>
            <dd>
              {{ previewRow.assetId }} ·
              {{ asset(previewRow.assetId)?.duration }} ·
              {{ previewRow.category }}
            </dd>
            <dt>创作声明</dt>
            <dd>{{ previewRow.declaration || "待确认" }}</dd>
            <template v-if="previewRow.platform === 'B站'"
              ><dt>分区 / 标签</dt>
              <dd>
                {{ previewRow.partition }} / {{ previewRow.tags }}
              </dd></template
            ><template v-if="previewRow.platform !== '百家号'"
              ><dt>可见范围</dt>
              <dd>{{ previewRow.visibility }}</dd></template
            ><template v-if="previewRow.platform !== 'B站' && previewRow.tags"
              ><dt>话题</dt>
              <dd>{{ previewRow.tags }}</dd></template
            >
            <dt>提交时间</dt>
            <dd>{{ previewRow.schedule || "启动后尽快提交" }}</dd>
          </dl>
          <ResourceNotice v-if="previewRow.platform === '百家号'"
            >{{ previewRow.mount ? "挂载待核验" : "不挂载" }} ·
            {{ previewRow.watermark ? "水印设置待核验" : "不添加水印" }} ·
            地点和活动未选。横竖封面的具体规格仍待核验。</ResourceNotice
          >
          <t-alert
            v-if="validate(previewRow, db, previewRow.id).length"
            theme="warning"
            :message="validate(previewRow, db, previewRow.id).join('；')"
          />
          <t-button
            variant="outline"
            @click="
              mediaPreviewOpen = false;
              editRow(previewRow, !!previewRow.id);
            "
            >返回编辑本条</t-button
          >
        </div>
      </template>
    </MediaPreview>
    <ResourceOverlay
      :visible="rowEditOpen"
      @update:visible="closeRow"
      title="编辑发布内容"
      kind="drawer"
      :width="800"
    >
      <template v-if="rowEdit"
        ><t-alert v-if="rowError" theme="warning" :message="rowError" />
        <p>
          {{ rowEdit.platform }} · {{ rowEdit.category }} · 成片
          {{ rowEdit.assetId }}
        </p>
        <div class="form-grid">
          <label
            >发布账号<t-select
              :popup-props="{
                hideEmptyPopup: false,
                attach: 'body',
                zIndex: 5500,
              }"
              v-model="rowEdit.accountId"
              :disabled="!!editingTaskId"
              :options="
                db.accounts
                  .filter(
                    (a) =>
                      a.enabled &&
                      a.platform === rowEdit.platform &&
                      a.categories.includes(rowEdit.category),
                  )
                  .map((a) => ({ label: a.name, value: a.id }))
              " /></label
          ><label v-if="rowEdit.platform !== '百家号'"
            >标题 *<t-input
              v-model="rowEdit.title"
              :maxlength="rowEdit.platform === '小红书' ? 20 : 80"
              show-limit-number /></label
          ><label class="full"
            >{{
              rowEdit.platform === "百家号"
                ? "作品描述 *"
                : "正文 / 简介（选填）"
            }}<t-textarea
              v-model="rowEdit.description"
              :maxlength="
                rowEdit.platform === '百家号'
                  ? 50
                  : rowEdit.platform === '小红书'
                    ? 1000
                    : 2000
              "
              :autosize="{ minRows: 3, maxRows: 6 }"
          /></label>
        </div>
        <p v-if="rowEdit.platform === '百家号'" class="muted">
          百家号声明当前仅确认“无需声明”，其他内容类型需要先核验平台选项。
        </p>
        <label class="block-field"
          >独立封面短标题<t-input
            v-model="rowEdit.shortTitle"
            aria-label="独立封面短标题"
        /></label>
        <h3>最终封面 · 保存本条即确认当前实际输出</h3>
        <div class="cover-previews">
          <figure
            v-for="spec in coverSpecs(rowEdit.platform).length
              ? coverSpecs(rowEdit.platform)
              : [{ key: 'source', label: '原始封面 · 规格待核验' }]"
            :key="spec.key"
          >
            <button
              class="media-thumb"
              @click="
                openPreview({
                  src: rowEdit.coverVariants?.[spec.key] || rowEdit.cover,
                  title: spec.label,
                })
              "
            >
              <img
                :src="rowEdit.coverVariants?.[spec.key] || rowEdit.cover"
                :alt="spec.label"
              />
            </button>
            <figcaption>{{ spec.label }}</figcaption>
            <t-button
              v-if="spec.key !== 'source'"
              variant="text"
              @click="
                openEditor(
                  {
                    document: rowEdit.templateDocuments?.[spec.key] ||
                      rowEdit.templateDocument || {
                        width: spec.width,
                        height: spec.height,
                        background: '#172f4c',
                        image: rowEdit.coverSource || rowEdit.cover,
                        imageFit: 'meet',
                        layers: [],
                      },
                    name: '本条封面',
                  },
                  'cover',
                  spec,
                )
              "
              >调整此封面</t-button
            >
          </figure>
        </div>
        <div class="inline-bar">
          <t-button variant="outline" @click="openImages('row')"
            >从图片库选择</t-button
          ><t-button variant="outline" @click="randomCover"
            >按类目随机选择</t-button
          ><t-button
            variant="outline"
            @click="
              openEditor(
                rowEdit.templateDocument
                  ? { document: rowEdit.templateDocument, name: '本条封面' }
                  : null,
                'cover',
              )
            "
            >调整当前封面</t-button
          >
        </div>
        <label class="block-field"
          >应用封面模板<t-select
            :popup-props="{
              hideEmptyPopup: false,
              attach: 'body',
              zIndex: 5500,
            }"
            :value="rowEdit.templateId"
            clearable
            placeholder="选择已保存模板"
            :options="
              db.templates
                .filter((t) => t.enabled)
                .map((t) => ({ label: t.name, value: t.id }))
            "
            @change="
              (v) => {
                const t = db.templates.find((t) => t.id === v);
                if (t) openEditor(t, 'cover');
                else {
                  rowEdit.templateId = null;
                  rowEdit.templateVersion = null;
                  rowEdit.templateDocument = null;
                  rowEdit.templateDocuments = {};
                  rowEdit.coverMode = '直接应用';
                }
              }
            "
        /></label>
        <div class="form-grid">
          <label
            >创作声明 *<t-select
              :popup-props="{
                hideEmptyPopup: false,
                attach: 'body',
                zIndex: 5500,
              }"
              v-model="rowEdit.declaration"
              placeholder="按实际内容选择"
              :options="declarationOptions(rowEdit.platform)" /></label
          ><label v-if="rowEdit.platform === 'B站'"
            >分区 *<t-select
              :popup-props="{
                hideEmptyPopup: false,
                attach: 'body',
                zIndex: 5500,
              }"
              v-model="rowEdit.partition"
              :options="
                ['游戏', '知识', '生活'].map((s) => ({ label: s, value: s }))
              " /></label
          ><label
            >{{ rowEdit.platform === "B站" ? "标签 *" : "话题（选填）"
            }}<t-input
              v-model="platformTopics"
              placeholder="仅填相关标签，多个用逗号分隔"
          /></label>
        </div>
        <t-alert
          v-if="rowEdit.declaration === '内容为转载'"
          theme="warning"
          message="转载的来源及条件字段尚未核验，当前演示配置不能代替正式投稿检查。"
        />
        <details class="more-settings">
          <summary>
            更多设置 · {{ rowEdit.visibility }} ·
            {{ rowEdit.schedule || "启动后尽快提交" }}
          </summary>
          <div class="form-grid">
            <label v-if="rowEdit.platform !== '百家号'"
              >可见范围<t-select
                :popup-props="{
                  hideEmptyPopup: false,
                  attach: 'body',
                  zIndex: 5500,
                }"
                v-model="rowEdit.visibility"
                :options="
                  (rowEdit.platform === '小红书'
                    ? ['公开', '仅自己']
                    : ['公开']
                  ).map((s) => ({ label: s, value: s }))
                " /></label
            ><label
              >系统启动时间（选填）<input
                type="datetime-local"
                v-model="rowEdit.schedule" /></label
            ><template v-if="rowEdit.platform === '百家号'">
              <p>
                挂载默认关闭；地点、活动默认不选。声明选项以平台实际页面核验为准。
              </p></template
            >
          </div>
        </details></template
      >
      <template #footer
        ><t-button variant="outline" @click="closeRow">取消修改</t-button
        ><t-button @click="saveRow">保存本条内容</t-button></template
      >
    </ResourceOverlay>
    <ResourceOverlay
      v-model:visible="imagePicker"
      :title="coverTarget === 'editor' ? '选择封面底图' : '选择现成封面'"
      :width="960"
    >
      <div class="wt-resource-page resource-visual-approved">
        <ResourceFilterBar
          ><label class="wt-filter-field wt-filter-field--wide"
            ><span class="wt-filter-field__label">名称</span
            ><t-input
              v-model="imageDraft.search"
              clearable
              placeholder="搜索图片名称"
              @enter="
                imageSearch = imageDraft.search;
                imageCategory = imageDraft.category;
                selectedImage = null;
              " /></label
          ><label class="wt-filter-field"
            ><span class="wt-filter-field__label">类目</span
            ><t-select
              :popup-props="{
                hideEmptyPopup: false,
                attach: 'body',
                zIndex: 5500,
              }"
              v-model="imageDraft.category"
              :options="[
                ...(imageContextCategory
                  ? [{ label: '当前类目及通用', value: '适用' }]
                  : []),
                { label: '全部类目', value: '' },
                ...categories.map((c) => ({ label: c, value: c })),
              ]"
          /></label>
          <div class="wt-filter-actions">
            <t-button
              @click="
                imageSearch = imageDraft.search;
                imageCategory = imageDraft.category;
                selectedImage = null;
              "
              >查询</t-button
            ><t-button
              theme="default"
              variant="outline"
              @click="
                imageDraft.search = '';
                imageDraft.category = '';
                imageSearch = '';
                imageCategory = '';
                selectedImage = null;
              "
              >重置</t-button
            >
          </div></ResourceFilterBar
        >
        <ResourceNotice
          >仅显示已启用且可用作{{
            coverTarget === "editor" ? "底图" : "现成封面"
          }}的图片。点击缩略图可查看完整效果。</ResourceNotice
        >
        <div class="image-picker-grid">
          <button
            v-for="im in availableImages"
            :key="im.id"
            class="image-choice"
            :class="{ selected: selectedImage === im.id }"
            @click="selectedImage = im.id"
          >
            <img :src="im.src" :alt="im.name" /><strong>{{ im.name }}</strong
            ><small
              >{{ im.category }} · {{ im.uses.join(" / ") }}</small
            ></button
          ><ResourceEmptyState
            v-if="!availableImages.length"
            title="没有可用图片"
            description="到图片库补充相应用途的图片。"
          />
        </div>
      </div>
      <template #footer
        ><t-button
          theme="default"
          variant="outline"
          @click="imagePicker = false"
          >返回</t-button
        ><t-button :disabled="!selectedImage" @click="useImage"
          >使用所选图片</t-button
        ></template
      >
    </ResourceOverlay>
    <ResourceOverlay
      :visible="imageForm.open"
      @update:visible="closeImage"
      :title="imageForm.id ? '编辑图片' : '上传图片'"
      :width="640"
      ><t-alert v-if="imageError" theme="warning" :message="imageError" />
      <div class="form-grid">
        <label class="full">名称 *<t-input v-model="imageForm.name" /></label
        ><label
          >内容类目<t-select
            :popup-props="{
              hideEmptyPopup: false,
              attach: 'body',
              zIndex: 5500,
            }"
            v-model="imageForm.category"
            :options="categories.map((c) => ({ label: c, value: c }))" /></label
        ><label
          >上传图片<input
            type="file"
            accept="image/png,image/jpeg,image/webp"
            @change="readImage" /></label
        ><label v-for="u in ['现成封面', '底图']" :key="u"
          ><input v-model="imageForm.uses" type="checkbox" :value="u" />{{
            u
          }}</label
        ><label
          ><input type="checkbox" v-model="imageForm.enabled" />启用</label
        >
      </div>
      <img
        v-if="imageForm.src"
        :src="imageForm.src"
        class="image-large"
        alt="上传预览"
      /><template #footer
        ><t-button variant="outline" @click="closeImage">取消</t-button
        ><t-button @click="saveImage">保存图片</t-button></template
      ></ResourceOverlay
    >
    <ResourceOverlay
      :visible="editorOpen"
      :title="editorPurpose === 'cover' ? '调整当前封面' : '封面模板设计'"
      :width="1360"
      :busy="editorSaving"
      @update:visible="cancelEditor"
      ><t-alert v-if="editorError" theme="warning" :message="editorError" />
      <div class="editor-heading">
        <label v-if="editorPurpose === 'template'"
          >模板名称<t-input v-model="editorName" /></label
        ><label v-if="editorPurpose === 'template'"
          >平台标签（可选）<select multiple v-model="editorPlatforms">
            <option v-for="p in platforms" :key="p">{{ p }}</option>
          </select></label
        ><label v-if="editorPurpose === 'template'"
          >类目标签（可选）<select multiple v-model="editorCategories">
            <option v-for="c in categories" :key="c">{{ c }}</option>
          </select></label
        ><t-button variant="outline" @click="openImages('editor')"
          >从图片库选底图</t-button
        >
      </div>
      <PrototypeEditor
        ref="imageEditorRef"
        v-if="editorOpen"
        v-model="editorDoc"
        :fixed-size="editorPurpose === 'cover'"
        @ready="editorReady"
      /><template #footer
        ><t-button variant="outline" @click="cancelEditor">取消</t-button
        ><t-button
          v-if="editorPurpose === 'template'"
          variant="outline"
          @click="
            editorId = null;
            editorName += ' 副本';
            saveEditor();
          "
          >另存为模板</t-button
        ><t-button :loading="editorSaving" @click="saveEditor">{{
          editorPurpose === "cover" ? "应用到本条封面" : "保存模板"
        }}</t-button></template
      ></ResourceOverlay
    >
    <ResourceOverlay
      v-model:visible="detailOpen"
      :title="
        detail?.items ? '计划详情' : detail?.assetId ? '任务详情' : '批次详情'
      "
      kind="drawer"
      :width="1040"
    >
      <template v-if="detail?.assetId"
        ><h2>{{ detail.title }}</h2>
        <ResourceDescriptions
          ><t-descriptions-item label="任务 ID">{{
            detail.id
          }}</t-descriptions-item
          ><t-descriptions-item label="发布平台">{{
            detail.platform
          }}</t-descriptions-item
          ><t-descriptions-item label="发布账号">{{
            account(detail.accountId)?.name
          }}</t-descriptions-item
          ><t-descriptions-item label="成片"
            >{{ detail.assetId }} · {{ detail.category }}</t-descriptions-item
          ><t-descriptions-item label="当前状态"
            >{{ detail.status }} / {{ detail.result }}</t-descriptions-item
          ><t-descriptions-item label="所属批次">{{
            detail.batchId
              ? db.batches.find((b) => b.id === detail.batchId)?.name
              : "独立任务"
          }}</t-descriptions-item>
          <t-descriptions-item label="关联计划">{{
            associatedPlan(db, {
              kind: detail.batchId ? "batch" : "task",
              id: detail.batchId || detail.id,
            })?.name || "未关联"
          }}</t-descriptions-item>
          <t-descriptions-item label="执行尝试"
            >{{ detail.attempts }} 次</t-descriptions-item
          ></ResourceDescriptions
        >
        <h3>最终内容快照</h3>
        <div class="snapshot">
          <button class="media-thumb" @click="openPreview({ row: detail })">
            <img :src="detail.cover" alt="查看任务最终内容" />
          </button>
          <div>
            <strong>{{ detail.title }}</strong>
            <p>{{ detail.description || "未填写简介" }}</p>
            <small
              >{{ detail.tags }} · {{ detail.partition }} ·
              {{ detail.declaration }}</small
            >
            <p>
              {{ detail.visibility }} ·
              {{ detail.schedule || "启动后尽快提交" }}
            </p>
          </div>
        </div>
        <h3>发布结果与核验依据</h3>
        <p>
          {{ detail.result }}
          {{
            detail.status === "平台审核中" ? "· 平台已接收，尚未确认公开" : ""
          }}
        </p>
        <p v-if="detail.evidence">
          <a
            v-if="detail.evidence.startsWith('https://')"
            :href="detail.evidence"
            target="_blank"
            rel="noopener noreferrer"
            >{{ detail.evidence }}</a
          ><span v-else>{{ detail.evidence }}</span>
        </p>
        <div
          v-if="['结果待确认', '平台审核中'].includes(detail.status)"
          class="inline-bar"
        >
          <t-input
            v-model="evidence"
            placeholder="补录 HTTPS 作品链接"
          /><t-button variant="outline" @click="linkEvidence"
            >保存核验依据</t-button
          >
        </div>
        <t-alert
          v-if="['执行失败', '结果待确认', '等待人工'].includes(detail.status)"
          theme="warning"
          :message="
            detail.status === '等待人工'
              ? `需核验登录或平台验证。对应窗口 ${account(detail.accountId)?.window}，处理后复核账号再继续。`
              : detail.status === '结果待确认'
                ? '提交结果不明确，请先查作品和核验，暂不可重试或释放。'
                : '已确认未发布，可编辑后重试；每次重试新增执行尝试。'
          "
        />
        <h3>执行时间线</h3>
        <ol v-if="detail.log?.length">
          <li v-for="(log, i) in detail.log" :key="i">
            {{ log.time }} · {{ log.text }}
          </li>
        </ol>
        <p v-else class="muted">尚无本次演示执行记录</p>
        <h3>执行尝试</h3>
        <details
          v-for="attempt in detail.attemptHistory || []"
          :key="attempt.number"
        >
          <summary>
            第 {{ attempt.number }} 次 · {{ attempt.started }} →
            {{ attempt.ended || "尚未结束" }} · {{ attempt.status }}
          </summary>
          <p>
            内容版本 {{ attempt.version }} · 发布标题
            {{ attempt.snapshot?.title }} · 封面短标题
            {{ attempt.snapshot?.shortTitle }} ·
            {{ attempt.errorCode || "无错误码" }} ·
            {{ attempt.receipt || "暂无回执" }}
          </p>
          <p v-for="(s, i) in attempt.steps || []" :key="i">
            {{ s.time }} · {{ s.status }} · {{ s.evidence }}
          </p>
        </details>
        <p v-if="!detail.attemptHistory?.length" class="muted">
          样例仅保留尝试计数；新的演示执行会记录起止时间。
        </p>
        <details v-if="demoTransitions(detail).length" class="demo-controls">
          <summary>演示控制 · 手动查看执行场景</summary>
          <p>以下仅切换本地场景，不核验真实账号或作品。</p>
          <div class="inline-bar">
            <t-button
              v-for="s in demoTransitions(detail)"
              :key="s"
              variant="outline"
              @click="taskResult(detail, s)"
              >{{ s }}</t-button
            >
          </div>
        </details></template
      >
      <template v-else-if="detail?.items"
        ><h2>{{ detail.name }}</h2>
        <div class="metric-grid">
          <div>
            <small>目标任务</small><strong>{{ planTarget(detail) }}</strong>
          </div>
          <div>
            <small>已公开发布</small><strong>{{ planStats.published }}</strong>
          </div>
          <div>
            <small>完成率</small><strong>{{ planStats.rate }}%</strong>
          </div>
          <div>
            <small>目标差额</small><strong>{{ planStats.gap }}</strong>
          </div>
        </div>
        <p>
          {{ detail.control }} ·
          {{ planStats.gap ? "目标未达成" : "目标已达成" }} · 自动衔接阈值
          10%（只读）
        </p>
        <div class="tabs">
          <button
            v-for="v in ['总览', '按平台', '按类目', '平台 × 类目']"
            :key="v"
            :class="{ active: planView === v }"
            @click="planView = v"
          >
            {{ v }}
          </button>
        </div>
        <PlanWorkbench
          v-if="planView === '总览'"
          :db="db"
          :plan="detail"
          @batch="configureBatch"
          @edit="editPlan"
        />
        <div v-else class="wt-resource-page">
          <t-table
            class="wt-resource-table"
            row-key="name"
            :data="planGroups"
            :columns="[
              { colKey: 'name', title: '维度', minWidth: 200 },
              { colKey: 'target', title: '目标', width: 85 },
              { colKey: 'total', title: '已创建', width: 90 },
              { colKey: 'published', title: '已公开', width: 100 },
              { colKey: 'reviewing', title: '审核中', width: 90 },
              { colKey: 'failed', title: '确认失败', width: 95 },
              { colKey: 'pending', title: '待执行及在途', width: 120 },
              { colKey: 'unresolved', title: '人工 / 未知', width: 120 },
              { colKey: 'gap', title: '差额', width: 80 },
            ]"
            ><template #published="{ row }"
              ><t-button
                variant="text"
                @click="
                  related(null, detail.id);
                  filterPlatform = row.platform || '';
                  filterCategory = row.category || '';
                  filterStatus = '已发布';
                  applyQuery();
                "
                >{{ row.published }}</t-button
              ></template
            ><template #unresolved="{ row }"
              >{{ row.human }} / {{ row.unknown }}</template
            ></t-table
          >
        </div></template
      >
      <template v-else-if="detail"
        ><h2>{{ detail.name }}</h2>
        <p>
          {{ detail.platform }} · {{ detail.control }} ·
          {{ batchSummary(db, detail).categories.join("、") }}
        </p>
        <p>
          批次 {{ detail.id }} · {{ batchSummary(db, detail).videos }} 条成片 ·
          {{ batchSummary(db, detail).total }} 条任务
        </p>
        <p>
          关联计划：<t-button
            v-if="associatedPlan(db, { kind: 'batch', id: detail.id })"
            variant="text"
            @click="
              returnToPlan(associatedPlan(db, { kind: 'batch', id: detail.id }))
            "
            >{{
              associatedPlan(db, { kind: "batch", id: detail.id }).name
            }}</t-button
          ><span v-else>未关联，可独立执行</span>
        </p>
        <t-alert
          v-if="detail.legacySource"
          theme="info"
          message="此旧演示分组已按实际任务平台兼容整理，原任务 ID、结果与执行记录保留。"
        />
        <div class="metric-grid">
          <div>
            <small>任务总数</small
            ><strong>{{
              stats(db.tasks.filter((t) => t.batchId === detail.id)).total
            }}</strong>
          </div>
          <div>
            <small>已发布</small
            ><strong>{{
              stats(db.tasks.filter((t) => t.batchId === detail.id)).published
            }}</strong>
          </div>
          <div>
            <small>发布成功率</small
            ><strong
              >{{
                stats(db.tasks.filter((t) => t.batchId === detail.id)).total
                  ? Math.round(
                      (stats(db.tasks.filter((t) => t.batchId === detail.id))
                        .published /
                        stats(db.tasks.filter((t) => t.batchId === detail.id))
                          .total) *
                        100,
                    )
                  : 0
              }}%</strong
            >
          </div>
          <div>
            <small>处理进度</small
            ><strong
              >{{
                stats(db.tasks.filter((t) => t.batchId === detail.id)).total
                  ? Math.round(
                      (stats(db.tasks.filter((t) => t.batchId === detail.id))
                        .finished /
                        stats(db.tasks.filter((t) => t.batchId === detail.id))
                          .total) *
                        100,
                    )
                  : 0
              }}%</strong
            >
          </div>
        </div>
        <p>处理进度包含失败、取消和放弃，不代表发布成功。</p>
        <div class="state-links">
          <t-button
            v-for="s in states"
            :key="s"
            variant="outline"
            @click="
              related(detail.id);
              filterStatus = s;
              applyQuery();
            "
            >{{ s }}
            {{
              db.tasks.filter((t) => t.batchId === detail.id && t.status === s)
                .length
            }}</t-button
          >
        </div>
        <p>
          配置阶段：{{ detail.phase }} · 目标 {{ targetTotal(detail) }} ·
          正式创建 {{ batchSummary(db, detail).total }} · 创建缺口
          {{ batchSummary(db, detail).creationGap }} · 执行收口
          {{ batchSummary(db, detail).closed }} /
          {{ batchSummary(db, detail).total }} · 公开目标
          {{ batchSummary(db, detail).published }} / {{ targetTotal(detail) }}
        </p>
        <t-button
          v-if="detail.phase !== '内容预检通过'"
          @click="configureBatch(detail)"
          >进入资源 / 内容配置</t-button
        >
        <BatchWorkbench :db="db" :batch="detail" @task="view" />
      </template>
      <template #footer
        ><t-button
          v-if="
            detail &&
            !detail.items &&
            associatedPlan(db, {
              kind: detail.assetId
                ? detail.batchId
                  ? 'batch'
                  : 'task'
                : 'batch',
              id: detail.batchId || detail.id,
            })
          "
          variant="text"
          @click="
            returnToPlan(
              associatedPlan(db, {
                kind: detail.assetId
                  ? detail.batchId
                    ? 'batch'
                    : 'task'
                  : 'batch',
                id: detail.batchId || detail.id,
              }),
            )
          "
          >返回关联计划</t-button
        ><template v-if="detail?.assetId"
          ><t-button
            v-if="detail.batchId"
            variant="text"
            @click="view(db.batches.find((b) => b.id === detail.batchId))"
            >返回所属批次</t-button
          >
          <t-button
            v-if="detail.status === '待执行' || canRetry(detail)"
            variant="outline"
            @click="editRow(detail, true)"
            >修订安全内容</t-button
          >
          <t-button
            v-if="
              detail.status === '待执行' ||
              (detail.status === '排队中' && !detail.claimed)
            "
            variant="outline"
            @click="taskResult(detail, '已取消')"
            >取消并保留占用</t-button
          >
          <t-button
            v-if="detail.status === '已取消' || canRetry(detail)"
            variant="outline"
            @click="release(detail)"
            >放弃并释放</t-button
          >
          <t-button
            v-if="detail.status === '待执行' || canRetry(detail)"
            @click="start(detail)"
            >启动 / 重试</t-button
          ><t-button
            v-if="['已取消', '已放弃'].includes(detail.status)"
            variant="outline"
            @click="cloneTask(detail)"
            >复制配置新建</t-button
          ></template
        ><template v-else-if="detail?.items"
          ><t-button variant="outline" @click="related(null, detail.id)"
            >查看所属任务</t-button
          >
          <t-button
            v-if="detail.control === '执行中'"
            variant="outline"
            @click="pausePlan(detail)"
            >暂停后续调度</t-button
          >
          <t-button
            v-else-if="!['已取消', '执行已结束'].includes(detail.control)"
            @click="startPlan(detail)"
            >启动 / 恢复计划</t-button
          >
          <t-button
            v-if="
              ['执行中', '等待继续确认', '等待跳过确认', '等待配置'].includes(
                detail.control,
              )
            "
            :disabled="currentDecision(detail) === '等待核实'"
            @click="advancePlan(detail)"
            >确认处理当前批次并衔接</t-button
          >
          <t-button
            v-if="!['已取消', '执行已结束'].includes(detail.control)"
            variant="outline"
            @click="cancelCurrentPlan(detail)"
            >取消计划</t-button
          > </template
        ><template v-else-if="detail"
          ><t-button
            v-if="!['已取消', '执行已结束'].includes(detail.control)"
            variant="outline"
            @click="batchAction(detail)"
            >{{
              detail.control === "执行中" ? "暂停批次" : "启动 / 恢复批次"
            }}</t-button
          ><t-button
            v-if="!['已取消', '执行已结束'].includes(detail.control)"
            variant="outline"
            @click="cancelBatch(detail)"
            >取消未提交任务</t-button
          ><t-button @click="related(detail.id)">查看任务</t-button></template
        ><t-button variant="outline" @click="detailOpen = false"
          >关闭</t-button
        ></template
      >
    </ResourceOverlay>
    <ResourceOverlay
      :visible="!!referenceRows"
      title="历史引用"
      @update:visible="referenceRows = null"
      ><p v-if="!referenceRows?.length">暂无正式引用；草稿不预占图片。</p>
      <p v-for="r in referenceRows || []" :key="r.kind + r.id">
        {{ r.kind }} #{{ r.id }} · {{ r.name }}
      </p>
      <template #footer
        ><t-button @click="referenceRows = null">关闭引用</t-button></template
      ></ResourceOverlay
    >
    <ResourceOverlay
      :visible="!!bulkResult"
      title="批量操作结果"
      :width="640"
      @update:visible="bulkResult = null"
      ><template v-if="bulkResult"
        ><p>
          {{ bulkResult.action }}：成功 {{ bulkResult.done.length }}，跳过
          {{ bulkResult.skip.length }}，失败 0，待确认 0。
        </p>
        <p v-for="r in bulkResult.skip" :key="r.id">
          {{ r.id }} · {{ r.reason }}
        </p></template
      ><template #footer
        ><t-button @click="bulkResult = null">完成</t-button></template
      ></ResourceOverlay
    >
  </div>
  <t-loading v-else fullscreen text="正在恢复原型配置…" />
</template>
<style scoped src="./publishing.css"></style>
