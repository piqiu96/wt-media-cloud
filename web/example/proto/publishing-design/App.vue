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
import ResourceOverlay from "../../../src/shared/ui/resource/ResourceOverlay.vue";
import ResourcePageHeader from "../../../src/shared/ui/resource/ResourcePageHeader.vue";
import ResourceCard from "../../../src/shared/ui/resource/ResourceCard.vue";
import ImageEditor from "../../../src/shared/ui/media/ImageEditor.vue";
import {
  platforms,
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
let saveQueue = Promise.resolve();
async function persistNow() {
  const snapshot = clone(db);
  savingSnapshot.value = true;
  saveQueue = saveQueue.catch(() => {}).then(() => savePrototype(snapshot));
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
  } catch (e) {
    storageError.value = `演示记录未能恢复：${e.message}。当前显示初始示例。`;
  }
  hydrated.value = true;
});
watch(
  db,
  () => {
    if (hydrated.value) persistNow();
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
  batches: "发布任务",
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
      (!appliedFilters.platform || r.platform === appliedFilters.platform) &&
      (!appliedFilters.category ||
        r.category === appliedFilters.category ||
        r.categories?.includes(appliedFilters.category)) &&
      (!appliedFilters.status ||
        r.status === appliedFilters.status ||
        r.control === appliedFilters.status),
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
  if (row?.targets) planView.value = "总览";
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
function taskResult(t, status) {
  const attempt = t.attemptHistory?.at(-1);
  if (attempt) {
    attempt.status = status;
    if (
      ["已发布", "平台审核中", "执行失败", "已取消", "已放弃"].includes(status)
    )
      attempt.ended = new Date().toLocaleString("zh-CN");
  }
  t.status = status;
  t.result =
    status === "已发布"
      ? "已公开"
      : status === "平台审核中"
        ? "平台已接收"
        : status === "结果待确认"
          ? "未知"
          : status === "执行失败"
            ? "确认未发布"
            : "未提交";
  t.log.push({
    time: new Date().toLocaleTimeString("zh-CN"),
    text: `演示状态：${status}`,
  });
}
async function takeControl(item) {
  const plan = associatedPlan(db, item);
  if (["执行中", "等待继续确认", "等待跳过确认"].includes(plan?.control)) {
    if (
      !(await feedback.confirmWarning({
        title: "转为独立控制？",
        message: `此项目由计划“${plan.name}”调度。独立操作将暂停该计划后续调度，已在途任务不变。`,
        confirmText: "暂停计划并独立操作",
      }))
    )
      return false;
    plan.control = "已暂停";
  }
  return true;
}
async function start(t, silent = false) {
  if (
    !(await takeControl({
      kind: t.batchId ? "batch" : "task",
      id: t.batchId || t.id,
    }))
  )
    return false;
  if (!["待执行", "执行失败"].includes(t.status)) return;
  if (t.status === "执行失败" && !canRetry(t)) return;
  const errors = validate(t, db, t.id);
  if (errors.length) {
    if (!silent) warning(errors.join("；"));
    return false;
  }
  taskResult(t, "排队中");
  t.attempts++;
  t.attemptHistory ||= [];
  t.attemptHistory.push({
    number: t.attempts,
    started: new Date().toLocaleString("zh-CN"),
    status: "排队中",
  });
  if (!silent) toast("已进入本地演示队列，不会操作真实平台");
  return true;
}
async function release(t) {
  if (!["已取消", "执行失败"].includes(t.status)) return;
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
  taskResult(t, "已放弃");
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
  if (!(await takeControl({ kind: "batch", id: b.id }))) return;
  if (b.control === "执行中") {
    b.control = "已暂停";
    return;
  }
  try {
    dispatchBatch(db, b);
  } catch (error) {
    warning(error.message);
  }
}
async function cancelBatch(b) {
  if (!(await takeControl({ kind: "batch", id: b.id }))) return;
  const pending = batchTasks(db, b).filter((t) =>
    ["待执行", "排队中"].includes(t.status),
  );
  if (!pending.length) {
    warning("当前没有可取消的待执行或排队任务");
    return;
  }
  if (
    !(await feedback.confirmWarning({
      title: "取消本批次未提交任务？",
      message: `仅取消本批次 ${pending.length} 条待执行或排队任务；在途任务保留，成片占用不释放。`,
      confirmText: "取消未提交任务",
    }))
  )
    return;
  pending.forEach((t) => taskResult(t, "已取消"));
  b.control = "已暂停";
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
        p.control = "已暂停";
        warning(error.message);
      }
  },
  { flush: "post" },
);
const listColumns = computed(() => {
  const first = [
    ...(page.value === "tasks"
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
              ]
            : [
                { colKey: "ratio", title: "画布规格", width: 140 },
                { colKey: "layers", title: "图层", width: 100 },
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
const composerRef = ref(null);
const rowEdit = ref(null),
  rowEditOpen = ref(false),
  editingTaskId = ref(null),
  rowError = ref("");
const rowOriginal = ref("");
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
  try {
    await ensureCoverResults(rowEdit.value);
  } catch (e) {
    rowError.value = e.message;
    return;
  }
  const errors = validate(rowEdit.value, db, editingTaskId.value);
  if (errors.length) {
    rowError.value = errors.join("；");
    return;
  }
  if (editingTaskId.value) {
    Object.assign(
      db.tasks.find((t) => t.id === editingTaskId.value),
      rowEdit.value,
      { version: (rowEdit.value.version || 1) + 1 },
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
  } else {
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
  if (
    await feedback.confirmWarning({
      title: "删除图片？",
      message: "图片将不再用于新任务，已创建任务的封面快照保留。",
      confirmText: "删除图片",
      danger: true,
    })
  )
    db.images = db.images.filter((i) => i.id !== im.id);
}
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
  imageError.value = "";
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
  const reader = new FileReader();
  reader.onload = () => {
    imageForm.src = reader.result;
    imageForm.name ||= f.name;
  };
  reader.readAsDataURL(f);
}
async function saveImage() {
  if (!imageForm.name.trim() || !imageForm.src || !imageForm.uses.length) {
    imageError.value = "请填写名称、选择图片和至少一种用途";
    return;
  }
  const row = clone(imageForm);
  delete row.open;
  row.id ||= id();
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
      : null;
  if (purpose === "cover" && template?.document)
    editorDoc.value.layers
      .filter((l) => l.kind === "text" && l.id === "title")
      .forEach(
        (l) => (l.text = rowEdit.value.title || rowEdit.value.description),
      );
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
  editorPreview.value = template?.preview || "";
  editorError.value = "";
  editorOpen.value = true;
}
async function cancelEditor() {
  if (editorSaving.value) return;
  if (
    JSON.stringify({ name: editorName.value, document: editorDoc.value }) !==
      editorOriginal.value &&
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
    rendered = await imageEditorRef.value?.exportPng(false);
  } finally {
    editorSaving.value = false;
  }
  if (!rendered) {
    editorError.value = "封面未能生成，请检查溢出、字体及底图";
    return;
  }
  editorPreview.value = rendered;
  if (editorPurpose.value === "cover") {
    rowEdit.value.coverSource ||= rowEdit.value.cover;
    rowEdit.value.coverVariants ||= {};
    rowEdit.value.coverVariants[editorCoverSpec.value.key] =
      editorPreview.value;
    if (
      !coverSpecs(rowEdit.value.platform).length ||
      editorCoverSpec.value.key === coverSpecs(rowEdit.value.platform)[0]?.key
    )
      rowEdit.value.cover = editorPreview.value;
    rowEdit.value.templateDocument = clone(editorDoc.value);
    rowEdit.value.coverMode = "规则生成封面";
    editorOpen.value = false;
    return;
  }
  const item = {
    isDefault:
      db.templates.find((t) => t.id === editorId.value)?.isDefault || false,
    id: editorId.value || id(),
    name: editorName.value,
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
const planForm = reactive({
    open: false,
    id: null,
    name: "",
    targets: [],
    platforms: [],
    categories: [],
  }),
  planError = ref(""),
  planView = ref("总览");
const planOriginal = ref("");
function editPlan(p) {
  Object.assign(planForm, {
    open: true,
    id: p?.id || null,
    name: p?.name || "",
    targets: clone(
      p?.targets || [{ platform: "小红书", category: "三角洲行动", count: 5 }],
    ),
  });
  planForm.platforms = [...new Set(planForm.targets.map((t) => t.platform))];
  planForm.categories = [...new Set(planForm.targets.map((t) => t.category))];
  planOriginal.value = JSON.stringify({
    name: planForm.name,
    targets: planForm.targets,
  });
  planError.value = "";
}
function changePlanScope() {
  const previous = planForm.targets;
  planForm.targets = planForm.platforms.flatMap((platform) =>
    planForm.categories.map(
      (category) =>
        previous.find(
          (t) => t.platform === platform && t.category === category,
        ) || { platform, category, count: 5 },
    ),
  );
}
async function closePlan() {
  if (
    JSON.stringify({ name: planForm.name, targets: planForm.targets }) !==
      planOriginal.value &&
    !(await feedback.confirmWarning({
      title: "放弃计划修改？",
      message: "尚未保存的目标和名称将丢失，已有批次和执行结果不受影响。",
      confirmText: "放弃修改",
    }))
  )
    return;
  planForm.open = false;
}
async function savePlan() {
  const keys = planForm.targets.map((t) => t.platform + ":" + t.category);
  if (
    !planForm.name.trim() ||
    !keys.length ||
    new Set(keys).size !== keys.length ||
    planForm.targets.some(
      (t) => !Number.isInteger(Number(t.count)) || Number(t.count) < 1,
    )
  ) {
    planError.value = "请填写名称、正整数目标；同平台同类目只能配置一项目标";
    return;
  }
  const p = db.plans.find((p) => p.id === planForm.id);
  if (
    p &&
    planTasks(p).some(
      (r) =>
        !planForm.targets.some(
          (t) => t.platform === r.platform && t.category === r.category,
        ),
    )
  ) {
    planError.value = "现有关联任务的目标维度不能移除，请保留该平台与类目";
    return;
  }
  if (p)
    Object.assign(p, { name: planForm.name, targets: clone(planForm.targets) });
  else {
    planForm.id ||= id();
    db.plans.push({
      id: planForm.id,
      name: planForm.name,
      targets: clone(planForm.targets),
      items: [],
      skipped: [],
      control: "规划中",
      position: 0,
    });
  }
  if (!(await persistNow())) {
    planError.value = "计划尚未保存，请重试；当前输入已保留";
    return;
  }
  planForm.open = false;
}
function canAttach(b, p) {
  return canLink(db, p, { kind: "batch", id: b.id });
}
function attach(p, item) {
  try {
    linkItem(db, p, item);
  } catch (error) {
    warning(error.message);
  }
}
const planTasks = (p) => collectPlanTasks(db, p),
  planTarget = (p) => p.targets.reduce((sum, t) => sum + Number(t.count), 0);
const orderRows = computed(() =>
  detail.value?.targets
    ? planItems(detail.value).map((item, index) => {
        const r = itemRecord(db, item),
          rows = itemTasks(db, item),
          summary = stats(rows);
        return {
          ...item,
          key: `${item.kind}:${item.id}`,
          order: index + 1,
          index,
          name: r?.name || r?.title || "记录缺失",
          platform: r?.platform,
          categories: [...new Set(rows.map((t) => t.category))].join("、"),
          control: item.kind === "batch" ? r?.control : r?.status,
          ...summary,
        };
      })
    : [],
);
function currentDecision(p) {
  return nextBatch(itemTasks(db, p.items[p.position]));
}
function showItem(item) {
  const record = itemRecord(db, item);
  if (record) view(record);
}
function returnToPlan(p) {
  detailOpen.value = false;
  view(p);
}
const inventory = (p) =>
  p.targets.map((t) => ({
    ...t,
    key: `${t.platform}:${t.category}`,
    accounts: db.accounts.filter(
      (a) =>
        a.enabled &&
        a.platform === t.platform &&
        a.categories.includes(t.category),
    ),
    assets: db.assets.filter((a) => a.category === t.category),
    free: db.assets.filter(
      (a) => a.category === t.category && !resourceStatus(db, a, t.platform),
    ).length,
  }));
const planStats = computed(() =>
  detail.value?.targets
    ? stats(planTasks(detail.value), planTarget(detail.value))
    : null,
);
const planGroups = computed(() => {
  const p = detail.value;
  if (!p?.targets) return [];
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
        v-for="p in ['tasks', 'plans', 'images', 'templates']"
        :key="p"
        :class="{ active: page === p || (p === 'tasks' && page === 'batches') }"
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
          ><template v-if="['tasks', 'batches'].includes(page)"
            ><t-button
              v-if="composerRef?.hasDraft"
              variant="outline"
              @click="composerRef.resume()"
              >继续未完成配置</t-button
            ><t-button variant="outline" @click="newPublish('快速发布')"
              >快速发布</t-button
            ><t-button @click="newPublish('批量发布')"
              >新建批次</t-button
            ></template
          ><t-button v-if="page === 'plans'" @click="editPlan()"
            >新建计划</t-button
          ><t-button v-if="page === 'images'" @click="imageEdit()"
            >上传图片</t-button
          ><t-button v-if="page === 'templates'" @click="openEditor()"
            >新建模板</t-button
          ></template
        ></ResourcePageHeader
      >
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
      <t-tabs
        v-if="['tasks', 'batches'].includes(page)"
        :value="page"
        class="publishing-view-tabs"
        @change="navigate"
        ><t-tab-panel value="tasks" label="任务列表" /><t-tab-panel
          value="batches"
          label="任务批次"
      /></t-tabs>
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
              v-model="filterPlatform"
              :options="[
                { label: '全部平台', value: '' },
                ...platforms.map((p) => ({ label: p, value: p })),
              ]"
          /></label>
          <label v-if="page === 'tasks'" class="wt-filter-field"
            ><span class="wt-filter-field__label">执行状态</span
            ><t-select
              v-model="filterStatus"
              :options="[
                { label: '全部状态', value: '' },
                ...states.map((s) => ({ label: s, value: s })),
              ]"
          /></label>
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
        <t-table
          class="wt-resource-table"
          row-key="id"
          :data="records"
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
          <template #uses="{ row }">{{ row.uses.join("、") }}</template>
          <template #enabled="{ row }"
            ><t-tag
              :theme="row.enabled ? 'success' : 'default'"
              variant="light"
              >{{ row.enabled ? "启用" : "停用" }}</t-tag
            ></template
          >
          <template #layers="{ row }"
            >{{ row.document.layers.length }} 个图层</template
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
                  v-if="page === 'batches'"
                  variant="text"
                  @click="batchAction(row)"
                  >{{
                    row.control === "执行中" ? "暂停" : "启动 / 恢复"
                  }}</t-button
                ><t-button v-else variant="text" @click="editPlan(row)"
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
        <h3>最终封面</h3>
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
                    document: {
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
                rowEdit.templateId = v;
                const t = db.templates.find((t) => t.id === v);
                if (t) openEditor(t, 'cover');
              }
            "
        /></label>
        <div class="form-grid">
          <label
            >创作声明 *<t-select
              v-model="rowEdit.declaration"
              placeholder="按实际内容选择"
              :options="declarationOptions(rowEdit.platform)" /></label
          ><label v-if="rowEdit.platform === 'B站'"
            >分区 *<t-select
              v-model="rowEdit.partition"
              :options="
                ['游戏', '知识', '生活'].map((s) => ({ label: s, value: s }))
              " /></label
          ><label
            >{{ rowEdit.platform === "B站" ? "标签 *" : "话题（选填）"
            }}<t-input
              v-model="rowEdit.tags"
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
        <t-table
          class="wt-resource-table"
          row-key="id"
          :data="availableImages"
          :columns="[
            { colKey: 'row-select', type: 'single', width: 44 },
            { colKey: 'image', title: '图片', minWidth: 300 },
            { colKey: 'category', title: '类目', width: 120 },
            { colKey: 'uses', title: '用途', minWidth: 140 },
          ]"
          :selected-row-keys="selectedImage ? [selectedImage] : []"
          @select-change="(keys) => (selectedImage = keys.at(-1))"
          :pagination="{
            pageSize: 6,
            total: availableImages.length,
            showJumper: false,
            pageSizeOptions: [],
            showPageSize: false,
          }"
        >
          <template #image="{ row }"
            ><div class="title-cell">
              <button
                class="media-thumb"
                @click="openPreview({ src: row.src, title: row.name })"
              >
                <img :src="row.src" :alt="row.name" /></button
              ><span>{{ row.name }}</span>
            </div></template
          ><template #uses="{ row }">{{ row.uses.join("、") }}</template
          ><template #empty
            ><ResourceEmptyState
              title="没有匹配的可用图片"
              description="调整类目筛选，或返回图片库补充对应用途的图片。"
          /></template>
        </t-table>
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
      v-model:visible="imageForm.open"
      :title="imageForm.id ? '编辑图片' : '上传图片'"
      :width="640"
      ><t-alert v-if="imageError" theme="warning" :message="imageError" />
      <div class="form-grid">
        <label class="full">名称 *<t-input v-model="imageForm.name" /></label
        ><label
          >内容类目<t-select
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
        ><t-button variant="outline" @click="imageForm.open = false"
          >取消</t-button
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
        ><t-button variant="outline" @click="openImages('editor')"
          >从图片库选底图</t-button
        >
      </div>
      <ImageEditor
        ref="imageEditorRef"
        v-if="editorOpen"
        v-model="editorDoc"
        :fixed-size="editorPurpose === 'cover'"
        @export="editorPreview = $event"
        @ready="editorReady"
      /><template #footer
        ><t-button variant="outline" @click="cancelEditor">取消</t-button
        ><t-button :loading="editorSaving" @click="saveEditor">{{
          editorPurpose === "cover" ? "应用到本条封面" : "保存模板"
        }}</t-button></template
      ></ResourceOverlay
    >
    <ResourceOverlay
      :visible="planForm.open"
      @update:visible="closePlan"
      title="配置发布目标"
      kind="drawer"
      :width="800"
      ><t-alert v-if="planError" theme="warning" :message="planError" /><label
        class="block-field"
        >计划名称 *<t-input v-model="planForm.name"
      /></label>
      <div class="form-grid plan-scope">
        <label
          >规划平台（可多选）<t-select
            v-model="planForm.platforms"
            multiple
            :options="platforms.map((p) => ({ label: p, value: p }))"
            aria-label="计划平台"
            @change="changePlanScope"
        /></label>
        <label
          >内容类目（可多选）<t-select
            v-model="planForm.categories"
            multiple
            :options="
              categories.slice(0, 2).map((c) => ({ label: c, value: c }))
            "
            aria-label="计划类目"
            @change="changePlanScope"
        /></label>
      </div>
      <h3>平台 × 类目目标与资源</h3>
      <div
        v-for="(t, i) in planForm.targets"
        :key="t.platform + ':' + t.category"
        class="target-row"
      >
        <span>{{ t.platform }}</span
        ><span>{{ t.category }}</span>
        <t-input-number
          v-model="t.count"
          :min="1"
          :decimal-places="0"
          :aria-label="`${t.platform}${t.category}目标`"
        />
        <small
          >{{ inventory(planForm)[i].accounts.length }} 个账号 ·
          {{ inventory(planForm)[i].free }} 条可用成片</small
        >
      </div>
      <ResourceEmptyState
        v-if="!planForm.targets.length"
        title="先选择平台与类目"
        description="可同时规划多个平台，保存后分别创建单平台批次。"
      />
      <p class="muted">
        保存计划只保存目标，不创建批次或任务、不占用成片。同一平台可关联多个独立批次。
      </p>
      <template #footer
        ><t-button variant="outline" @click="closePlan">取消</t-button
        ><t-button @click="savePlan">保存计划</t-button></template
      ></ResourceOverlay
    >
    <ResourceOverlay
      v-model:visible="detailOpen"
      :title="
        detail?.targets ? '计划详情' : detail?.assetId ? '任务详情' : '批次详情'
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
        <p v-if="detail.evidence">{{ detail.evidence }}</p>
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
        <p v-for="attempt in detail.attemptHistory || []" :key="attempt.number">
          第 {{ attempt.number }} 次 · {{ attempt.started }} →
          {{ attempt.ended || "尚未结束" }} · {{ attempt.status }}
        </p>
        <p v-if="!detail.attemptHistory?.length" class="muted">
          样例仅保留尝试计数；新的演示执行会记录起止时间。
        </p>
        <details class="demo-controls">
          <summary>演示控制 · 手动查看执行场景</summary>
          <p>以下仅切换本地场景，不核验真实账号或作品。</p>
          <div class="inline-bar">
            <t-button
              v-for="s in [
                '执行中',
                '等待人工',
                '结果待确认',
                '平台审核中',
                '已发布',
                '执行失败',
              ]"
              :key="s"
              variant="outline"
              @click="taskResult(detail, s)"
              >{{ s }}</t-button
            >
          </div>
        </details></template
      >
      <template v-else-if="detail?.targets"
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
        <template v-if="planView === '总览'"
          ><h3>资源盘点</h3>
          <p>
            去重账号
            {{ new Set(planTasks(detail).map((t) => t.accountId)).size }} 个 ·
            去重成片
            {{ new Set(planTasks(detail).map((t) => t.assetId)).size }} 条 ·
            已创建 {{ planStats.total }} 条任务
          </p>
          <p>
            合格账号
            {{
              new Set(
                inventory(detail).flatMap((r) => r.accounts.map((a) => a.id)),
              ).size
            }}
            个 · 可用成片
            {{
              new Set(
                detail.targets.flatMap((t) =>
                  db.assets
                    .filter(
                      (a) =>
                        a.category === t.category &&
                        !resourceStatus(db, a, t.platform),
                    )
                    .map((a) => a.id),
                ),
              ).size
            }}
            条（去重）
          </p>
          <div class="wt-resource-page">
            <t-table
              class="wt-resource-table"
              row-key="key"
              :data="inventory(detail)"
              :columns="[
                { colKey: 'scope', title: '资源范围', minWidth: 200 },
                {
                  colKey: 'accounts',
                  title: '合格账号 · 优质 / 普通 / 新号',
                  minWidth: 230,
                },
                {
                  colKey: 'assets',
                  title: '匹配成片 · 优质 / 普通',
                  minWidth: 190,
                },
                { colKey: 'free', title: '可安排成片', width: 120 },
              ]"
              ><template #scope="{ row }"
                >{{ row.platform }} · {{ row.category }}</template
              ><template #accounts="{ row }"
                >{{ row.accounts.length }}（{{
                  row.accounts.filter((a) => a.quality === "优质").length
                }}
                /
                {{ row.accounts.filter((a) => a.quality === "普通").length }} /
                {{
                  row.accounts.filter((a) => a.quality === "新号").length
                }}）</template
              ><template #assets="{ row }"
                >{{ row.assets.length }}（{{
                  row.assets.filter((a) => a.quality === "优质").length
                }}
                /
                {{
                  row.assets.filter((a) => a.quality === "普通").length
                }}）</template
              ></t-table
            >
          </div>

          <h3>执行顺序 · {{ detail.items.length }} 个独立项目</h3>
          <p class="muted">
            同平台多个批次分别执行与统计；前一批无未决任务且失败率不超过 10%
            时自动衔接。
          </p>
          <t-table
            class="wt-resource-table plan-order-table"
            row-key="key"
            :data="orderRows"
            :columns="[
              { colKey: 'order', title: '顺序', width: 65 },
              { colKey: 'identity', title: '批次 / 独立任务', minWidth: 220 },
              {
                colKey: 'progress',
                title: '任务 / 已公开 / 失败 / 待处理',
                minWidth: 215,
              },
              { colKey: 'control', title: '调度 / 状态', width: 125 },
              { colKey: 'operation', title: '操作', minWidth: 190 },
            ]"
          >
            <template #identity="{ row }"
              ><strong>{{ row.name }}</strong
              ><small class="cell-secondary"
                >{{ row.kind === "batch" ? "批次" : "独立任务" }} {{ row.id }} ·
                {{ row.platform }} · {{ row.categories }}</small
              ><t-tag v-if="row.index === detail.position">当前项目</t-tag
              ><t-tag
                v-if="
                  detail.skipped.some(
                    (i) => i.kind === row.kind && i.id === row.id,
                  )
                "
                >调度已跳过</t-tag
              ></template
            >
            <template #progress="{ row }"
              >{{ row.total }} / {{ row.published }} / {{ row.failed }} /
              {{ row.human + row.unknown }}</template
            >
            <template #operation="{ row }"
              ><ResourceRowActions
                ><t-button variant="text" @click="showItem(row)"
                  >查看详情</t-button
                ><t-button
                  variant="text"
                  :disabled="
                    row.index === 0 ||
                    (detail.control !== '规划中' &&
                      row.index <= detail.position + 1)
                  "
                  @click="moveItem(detail, row.index, -1)"
                  >上移</t-button
                ><t-button
                  variant="text"
                  :disabled="
                    row.index === detail.items.length - 1 ||
                    (detail.control !== '规划中' &&
                      row.index <= detail.position)
                  "
                  @click="moveItem(detail, row.index, 1)"
                  >下移</t-button
                ></ResourceRowActions
              ></template
            >
            <template #empty
              ><ResourceEmptyState
                title="尚未安排执行项目"
                description="从本计划创建单平台批次，或关联符合目标的现有批次、独立任务。"
            /></template>
          </t-table>
          <h3>关联现有批次</h3>
          <div class="inline-bar">
            <t-button
              v-for="b in db.batches.filter((b) => canAttach(b, detail))"
              :key="b.id"
              variant="outline"
              @click="attach(detail, { kind: 'batch', id: b.id })"
              >{{ b.name }} · {{ b.platform }} ＋</t-button
            ><span
              v-if="!db.batches.some((b) => canAttach(b, detail))"
              class="muted"
              >没有可关联的批次，可继续创建同平台的新批次。</span
            >
          </div>
          <h3>
            关联独立任务
            <t-button variant="text" @click="newPublish('快速发布', detail.id)"
              >新建独立任务</t-button
            >
          </h3>
          <div class="inline-bar">
            <t-button
              v-for="t in db.tasks.filter(
                (t) =>
                  !t.batchId && canLink(db, detail, { kind: 'task', id: t.id }),
              )"
              :key="t.id"
              variant="outline"
              @click="attach(detail, { kind: 'task', id: t.id })"
              >{{ t.title }} · {{ t.platform }} ＋</t-button
            ><span
              v-if="
                !db.tasks.some(
                  (t) =>
                    !t.batchId &&
                    canLink(db, detail, { kind: 'task', id: t.id }),
                )
              "
              class="muted"
              >没有符合范围且未关联计划的独立任务。</span
            >
          </div>
          <p v-if="detail.items[detail.position]">
            当前项目衔接判断：{{ currentDecision(detail) }}
          </p></template
        >
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
        <h3>任务摘要</h3>
        <div class="wt-resource-page">
          <t-table
            class="wt-resource-table"
            row-key="id"
            :data="db.tasks.filter((t) => t.batchId === detail.id)"
            :columns="[
              { colKey: 'id', title: '任务 ID', width: 90 },
              {
                colKey: 'title',
                title: '发布内容',
                minWidth: 200,
                ellipsis: true,
              },
              { colKey: 'identity', title: '发布账号', minWidth: 170 },
              { colKey: 'status', title: '状态', width: 120 },
              {
                colKey: 'op',
                title: '操作',
                width: 90,
                fixed: compact ? undefined : 'right',
              },
            ]"
            :pagination="{
              pageSize: 8,
              total: db.tasks.filter((t) => t.batchId === detail.id).length,
              showJumper: false,
              pageSizeOptions: [],
              showPageSize: false,
            }"
            ><template #identity="{ row }"
              >{{ row.platform }} · {{ account(row.accountId)?.name }}</template
            ><template #status="{ row }"
              ><t-tag :theme="stateTheme(row.status)" variant="light">{{
                row.status
              }}</t-tag></template
            ><template #op="{ row }"
              ><ResourceRowActions
                ><t-button variant="text" @click="view(row)"
                  >详情</t-button
                ></ResourceRowActions
              ></template
            ></t-table
          >
        </div></template
      >
      <template #footer
        ><t-button
          v-if="
            detail &&
            !detail.targets &&
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
            v-if="detail.status === '待执行' || canRetry(detail)"
            @click="start(detail)"
            >启动 / 重试</t-button
          ><t-button
            v-if="detail.status === '已取消'"
            variant="outline"
            @click="cloneTask(detail)"
            >按此范围新建</t-button
          ></template
        ><template v-else-if="detail?.targets"
          ><t-button
            variant="outline"
            @click="newPublish('批量发布', detail.id)"
            >创建关联批次</t-button
          ><t-button variant="outline" @click="related(null, detail.id)"
            >查看全部任务</t-button
          ><t-button
            v-if="detail.control === '执行中'"
            variant="outline"
            @click="detail.control = '已暂停'"
            >暂停后续调度</t-button
          ><t-button v-else @click="startPlan(detail)">启动 / 恢复计划</t-button
          ><t-button
            v-if="
              ['执行中', '等待继续确认', '等待跳过确认'].includes(
                detail.control,
              )
            "
            :disabled="currentDecision(detail) === '等待核实'"
            @click="advancePlan(detail)"
            >处理当前批次并衔接</t-button
          ></template
        ><template v-else-if="detail"
          ><t-button variant="outline" @click="batchAction(detail)">{{
            detail.control === "执行中" ? "暂停批次" : "启动 / 恢复批次"
          }}</t-button
          ><t-button variant="outline" @click="cancelBatch(detail)"
            >取消未提交任务</t-button
          ><t-button @click="related(detail.id)">查看任务</t-button></template
        ><t-button variant="outline" @click="detailOpen = false"
          >关闭</t-button
        ></template
      >
    </ResourceOverlay>
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
