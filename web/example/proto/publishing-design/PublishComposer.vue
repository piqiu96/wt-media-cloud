<script setup>
import { computed, reactive, ref, watch } from "vue";
import { useMediaQuery } from "../../../src/shared/composables/useMediaQuery.js";
import ResourceOverlay from "./PrototypeOverlay.vue";
import ResourceWizardSteps from "../../../src/shared/ui/resource/ResourceWizardSteps.vue";
import ResourceNotice from "../../../src/shared/ui/resource/ResourceNotice.vue";
import ResourceEmptyState from "../../../src/shared/ui/resource/ResourceEmptyState.vue";
import ResourceRowActions from "../../../src/shared/ui/resource/ResourceRowActions.vue";
import ResourceSelectionToolbar from "../../../src/shared/ui/resource/ResourceSelectionToolbar.vue";
import AIWorkspace from "./AIWorkspace.vue";
import { initCandidate, editField } from "./content.js";
import { createPublication, targetTotal, nextId } from "./relationships.js";
import { ensureCoverResults } from "./covers.js";
import { feedback } from "../../../src/shared/ui/feedback/index.js";
import {
  platforms,
  categories,
  validate,
  resourceStatus,
  resourceInventory,
  suggestDistribution,
  mergeContentDefaults,
  declarationOptions,
} from "./model.js";
const props = defineProps({ db: { type: Object, required: true } });
const emit = defineEmits(["edit", "preview", "created"]);
const compact = useMediaQuery("(max-width:700px)");
const copy = (v) => JSON.parse(JSON.stringify(v));
const visible = ref(false),
  closing = ref(false),
  preparing = ref(false),
  batchNameInput = ref(null),
  batchNameError = ref("");
const state = reactive({
  mode: "批量发布",
  step: 0,
  platforms: [],
  categories: [],
  assets: [],
  excludedAccounts: [],
  rows: [],
  name: "",
  planId: null,
  batchId: null,
  targets: [],
  creationKey: null,
  active: false,
  accountLimits: {},
  accountOrder: [],
  quickAccountId: null,
});
const appliedNotice = reactive({ platform: "", text: "" });
if (props.db.publishDraft) {
  Object.assign(state, copy(props.db.publishDraft));
  state.active =
    props.db.publishDraft.active ??
    !!(state.rows.length || state.assets.length || state.platforms.length);
}
watch(
  state,
  () => {
    props.db.publishDraft = copy(state);
    const b = props.db.batches.find((b) => b.id === state.batchId);
    if (b && b.phase !== "内容预检通过") {
      if (!b.planId) {
        b.platform = state.platforms[0];
        b.name = state.name;
        b.targets = copy(state.targets);
        b.categories = copy(state.categories);
      }
      b.candidates = copy(state.rows);
      b.draft = copy(state);
      b.phase = state.step >= 2 ? "待内容配置" : "待资源配置";
    }
  },
  { deep: true },
);
const errors = ref([]),
  onlyErrors = ref(false),
  rowPlatform = ref("全部"),
  contentPlatform = ref("小红书"),
  search = ref(""),
  showUnavailable = ref(false),
  assetPage = ref(1),
  contentPage = ref(1);
const platformDefaults = reactive(
  Object.fromEntries(
    platforms.map((p) => [p, { declaration: "", tags: "", partition: "游戏" }]),
  ),
);
const isQuick = computed(() => state.mode === "快速发布");
const steps = computed(() =>
  isQuick.value
    ? [
        { title: "选择资源", value: 0 },
        { title: "配置内容", value: 2 },
        { title: "最终检查", value: 3 },
      ]
    : [
        { title: "范围与资源", value: 0 },
        { title: "检查分配", value: 1 },
        { title: "配置内容", value: 2 },
        { title: "最终检查", value: 3 },
      ],
);
const activeStep = computed(() =>
  steps.value.findIndex((s) => s.value === state.step),
);
const selectedPlatform = computed({
  get: () => state.platforms[0],
  set: (v) => {
    changePlatform(v);
  },
});
async function changePlatform(value) {
  if (value === state.platforms[0]) return;
  if (
    (state.assets.length || state.rows.length) &&
    !(await feedback.confirmWarning({
      title: "切换发布平台？",
      message: "当前成片选择、账号分配和内容配置将清空；类目和批次名称保留。",
      confirmText: "清空配置并切换",
    }))
  )
    return;
  state.platforms = value ? [value] : [];
  state.assets = [];
  state.rows = [];
  state.excludedAccounts = [];
  state.accountLimits = {};
  state.accountOrder = [];
  state.quickAccountId = null;
  for (const p of platforms)
    Object.assign(platformDefaults[p], {
      declaration: "",
      tags: "",
      partition: "游戏",
    });
  errors.value = [];
  rowPlatform.value = "全部";
  assetPage.value = 1;
  appliedNotice.platform = "";
  if (state.planId) {
    const plan = props.db.plans.find((p) => p.id === state.planId);
    state.categories = state.categories.filter((c) =>
      plan?.targets.some((t) => t.platform === value && t.category === c),
    );
  }
}
const allowedPlatforms = computed(() => {
  const p = props.db.plans.find((p) => p.id === state.planId);
  return p ? [...new Set(p.targets.map((t) => t.platform))] : platforms;
});
const allowedCategories = computed(() => {
  const p = props.db.plans.find((p) => p.id === state.planId);
  return p
    ? [
        ...new Set(
          p.targets
            .filter((t) => t.platform === selectedPlatform.value)
            .map((t) => t.category),
        ),
      ]
    : categories;
});
const selectedCategory = computed({
  get: () => state.categories[0],
  set: (v) => {
    state.categories = v ? [v] : [];
    state.quickAccountId = null;
    state.assets = [];
    state.rows = [];
  },
});
const eligibleAccounts = computed(() =>
  props.db.accounts.filter(
    (a) =>
      a.enabled &&
      a.loginValid !== false &&
      state.platforms.includes(a.platform) &&
      a.categories.some((c) => state.categories.includes(c)),
  ),
);
const accountIds = computed(() =>
  isQuick.value
    ? (state.quickAccountId ? [state.quickAccountId] : [])
    : eligibleAccounts.value
    .filter((a) => !state.excludedAccounts.includes(a.id))
    .map((a) => a.id)
    .sort((a, b) => {
      const order = state.accountOrder.length
        ? state.accountOrder
        : eligibleAccounts.value.map((a) => a.id);
      return order.indexOf(a) - order.indexOf(b);
    }),
);
const scopeAssets = computed(() =>
  props.db.assets.filter((v) => state.categories.includes(v.category)),
);
const availability = (v) =>
  state.platforms.map((platform) => ({
    platform,
    reason: resourceStatus(props.db, v, platform, accountIds.value),
  }));
const availableAssets = computed(() =>
  scopeAssets.value.filter((v) => availability(v).some((p) => !p.reason)),
);
const visibleAssets = computed(() =>
  (showUnavailable.value
    ? scopeAssets.value.filter((v) => availability(v).every((p) => p.reason))
    : availableAssets.value
  ).filter((v) => `${v.id} ${v.title}`.includes(search.value)),
);
const selectedAssets = computed(() =>
  state.assets.filter((id) => scopeAssets.value.some((v) => v.id === id)),
);
const unavailableCount = computed(
  () => scopeAssets.value.length - availableAssets.value.length,
);
const inventory = computed(() =>
  resourceInventory(
    props.db,
    state.platforms,
    state.categories,
    accountIds.value,
  ),
);
const suggested = computed(() =>
  suggestDistribution(
    props.db,
    state.platforms,
    selectedAssets.value,
    accountIds.value,
    state.rows,
    state.accountLimits,
  ),
);
const batchTarget = computed(() =>
  state.targets.reduce((sum, target) => sum + Number(target.count || 0), 0),
);
const resourceRows = computed(() =>
  state.categories.map((category) => {
    const target = Number(state.targets.find((t) => t.category === category)?.count || 0);
    const available = availableAssets.value.filter((asset) => asset.category === category).length;
    const allocated = suggested.value.filter((row) => row.category === category).length;
    return { category, target, available, allocated, gap: Math.max(0, target - allocated) };
  }),
);
function recommendAssets() {
  const selected = [];
  for (const { category, target } of resourceRows.value) {
    selected.push(
      ...availableAssets.value
        .filter((asset) => asset.category === category)
        .sort((a, b) => Number(b.quality === "优质") - Number(a.quality === "优质") || a.id - b.id)
        .slice(0, target)
        .map((asset) => asset.id),
    );
  }
  state.assets = selected;
  state.rows = state.rows.filter((row) => selected.includes(row.assetId));
}
const resourceScopeKey = computed(() =>
  [state.mode, state.platforms.join("/"), state.categories.join("/"),
    state.targets.map((target) => `${target.category}:${target.count}`).join("/")].join("|"),
);
watch(resourceScopeKey, () => {
  if (!isQuick.value && state.step === 0 && (!state.batchId || !state.assets.length))
    recommendAssets();
}, { flush: "post" });
function resourceProblems(row) {
  const video = asset(row.assetId), selectedAccount = account(row.accountId);
  const problems = [];
  if (!video) problems.push("成片不存在");
  if (!selectedAccount?.enabled || selectedAccount.platform !== row.platform ||
      !selectedAccount.categories.includes(row.category))
    problems.push("账号不支持当前平台或类目");
  if (video) {
    const reason = resourceStatus(props.db, video, row.platform, [row.accountId]);
    if (reason) problems.push(reason);
  }
  return problems;
}
const resourceErrors = computed(() => state.rows.flatMap((row) =>
  resourceProblems(row).map((problem) => `${asset(row.assetId)?.title || row.assetId}：${problem}`),
));
const invalidSelections = computed(() =>
  selectedAssets.value.filter(
    (id) => !availableAssets.value.some((v) => v.id === id),
  ),
);
const rows = computed(() =>
  state.rows.filter(
    (r) =>
      (rowPlatform.value === "全部" || r.platform === rowPlatform.value) &&
      (!(state.step > 1 && onlyErrors.value) || validate(r, props.db).length),
  ),
);
const issues = computed(() =>
  state.rows.filter((r) => validate(r, props.db).length),
);
const unassigned = computed(() =>
  state.platforms.flatMap((platform) =>
    selectedAssets.value.flatMap((id) => {
      const video = asset(id);
      if (
        !video ||
        resourceStatus(props.db, video, platform, accountIds.value) ||
        suggested.value.some((r) => r.platform === platform && r.assetId === id)
      )
        return [];
      return [
        {
          key: `${platform}:${id}`,
          platform,
          title: video.title,
          reason: "参与账号已达到本批次安排上限",
        },
      ];
    }),
  ),
);
function reorderAccount(id, delta) {
  const ids = state.accountOrder.length
    ? [...state.accountOrder]
    : eligibleAccounts.value.map((a) => a.id);
  const from = ids.indexOf(id),
    to = from + delta;
  if (to < 0 || to >= ids.length) return;
  [ids[from], ids[to]] = [ids[to], ids[from]];
  state.accountOrder = ids;
}
const orderedAccounts = computed(() =>
  [...eligibleAccounts.value].sort((a, b) => {
    const order = state.accountOrder.length
      ? state.accountOrder
      : eligibleAccounts.value.map((a) => a.id);
    return order.indexOf(a.id) - order.indexOf(b.id);
  }),
);
const uniqueVideos = computed(
  () => new Set(state.rows.map((r) => r.assetId)).size,
);
const readyCount = computed(() => state.rows.length - issues.value.length);
const quickRow = computed(() => isQuick.value ? state.rows[0] : null);
const quickIssues = computed(() => quickRow.value ? validate(quickRow.value, props.db) : ["请选择发布资源"]);
const asset = (id) => props.db.assets.find((v) => v.id === id);
const account = (id) => props.db.accounts.find((a) => a.id === id);
const accountOptions = (row) =>
  eligibleAccounts.value
    .filter(
      (a) =>
        a.platform === row.platform &&
        a.categories.includes(row.category) &&
        accountIds.value.includes(a.id),
    )
    .map((a) => ({ label: `${a.name} · ${a.quality}`, value: a.id }));
const assetColumns = [
  { colKey: "row-select", type: "multiple", width: 44 },
  { colKey: "asset", title: "可用成片", minWidth: 270 },
  { colKey: "category", title: "类目", width: 120 },
  { colKey: "quality", title: "质量", width: 85 },
  { colKey: "availability", title: "平台可用性", minWidth: 230 },
];
const candidateColumns = computed(() =>
  state.step === 1
    ? [
        { colKey: "asset", title: "推荐成片", minWidth: 260 },
        { colKey: "platform", title: "平台 / 类目", width: 150 },
        { colKey: "account", title: "推荐账号（可调整）", minWidth: 245 },
        { colKey: "check", title: "资源检查", width: 130 },
        { colKey: "op", title: "调整", width: 110 },
      ]
    : [
        { colKey: "asset", title: "成片", minWidth: 210 },
        { colKey: "platform", title: "平台 / 类目", width: 135 },
        { colKey: "account", title: "发布账号", minWidth: 205 },
        { colKey: "content", title: "发布内容", minWidth: 245 },
        { colKey: "check", title: "内容检查", width: 95 },
        { colKey: "op", title: "操作", width: 180, fixed: compact.value ? undefined : "right" },
      ],
);
function selectAssets(ids) {
  const unavailableIds = props.db.assets
    .filter((v) => !availableAssets.value.includes(v))
    .map((v) => v.id);
  state.assets = (isQuick.value ? ids.slice(-1) : ids).filter(
    (id) => !unavailableIds.includes(id),
  );
  errors.value = [];
}
function toggleAccount(ids) {
  state.excludedAccounts = eligibleAccounts.value
    .filter((a) => !ids.includes(a.id))
    .map((a) => a.id);
}
async function open(mode = "批量发布", planId = null, source = null) {
  if (
    state.active &&
    (state.assets.length || state.rows.length) &&
    !(await feedback.confirmWarning({
      title: "开始新的发布配置？",
      message: "当前未创建的配置将被替换，也可以取消后继续原来的配置。",
      confirmText: "开始新配置",
    }))
  )
    return;
  state.copySource = source ? copy(source) : null;
  Object.assign(state, {
    mode,
    step: 0,
    platforms: source ? [source.platform] : [],
    categories: source ? [source.category] : [],
    assets: [],
    excludedAccounts: [],
    rows: [],
    name: "",
    planId: null,
    batchId: null,
    targets: [],
    creationKey: crypto.randomUUID(),
    active: true,
    accountLimits: {},
    accountOrder: [],
    quickAccountId: null,
  });
  for (const p of platforms)
    Object.assign(platformDefaults[p], {
      declaration: "",
      tags: "",
      partition: "游戏",
    });
  onlyErrors.value = false;
  rowPlatform.value = "全部";
  search.value = "";
  showUnavailable.value = false;
  errors.value = [];
  assetPage.value = 1;
  closing.value = false;
  visible.value = true;
}
async function allocate() {
  errors.value = [];
  if (state.platforms.length !== 1 || !state.categories.length) {
    errors.value = ["请选择一个发布平台和至少一个内容类目"];
    return;
  }
  if (isQuick.value && !state.quickAccountId) {
    errors.value = ["请选择本次发布使用的账号"];
    return;
  }
  if (!isQuick.value && !batchTarget.value) {
    errors.value = ["请先设置至少一个类目的批次目标"];
    return;
  }
  if (!selectedAssets.value.length) {
    errors.value = [isQuick.value ? "请选择一条可用成片" : "当前目标尚未分配成片，请检查可用资源或手动调整"];
    return;
  }
  if (isQuick.value && selectedAssets.value.length !== 1) {
    errors.value = ["快速发布只能选择一条成片"];
    return;
  }
  if (invalidSelections.value.length) {
    errors.value = ["部分已选成片当前不可安排，请移除或调整账号范围"];
    return;
  }
  if (!suggested.value.length) {
    errors.value = ["当前选择没有可分配的任务，请调整账号或资源范围"];
    return;
  }
  const removed = state.rows.filter(
    (r) => r.overrides?.length && !suggested.value.some((s) => s.key === r.key),
  );
  if (
    removed.length &&
    !(await feedback.confirmWarning({
      title: "移出不再符合范围的任务？",
      message: `重新分配将移出 ${removed.length} 条有人工修改的候选。其余兼容任务的修改会保留。`,
      confirmText: "调整并继续",
    }))
  )
    return;
  const plan = props.db.plans.find((p) => p.id === state.planId);
  state.rows = copy(
    suggested.value.filter(
      (r) =>
        !plan ||
        plan.targets.some(
          (t) => t.platform === r.platform && t.category === r.category,
        ),
    ),
  );
  if (!state.rows.length) {
    errors.value = ["当前选择不在计划目标范围内"];
    return;
  }
  state.rows.forEach(initCandidate);
  if (isQuick.value && state.copySource) {
    const source = state.copySource;
    const r = state.rows[0];
    for (const f of [
      "title",
      "shortTitle",
      "description",
      "tags",
      "topics",
      "partition",
      "declaration",
      "coverSource",
      "coverMode",
      "templateId",
      "templateVersion",
      "templateDocument",
      "templateDocuments",
      "templateSnapshots",
    ])
      if (source[f] !== undefined) editField(r, f, copy(source[f]));
    r.schedule = "";
    state.copySource = null;
  }
  if (!isQuick.value) {
    state.targets = state.categories.map((category) => ({
      category,
      count: Number(
        state.targets.find((t) => t.category === category)?.count || 0,
      ),
    }));
    for (const t of state.targets)
      if (
        state.rows.filter((r) => r.category === t.category).length > t.count
      ) {
        errors.value = [t.category + "候选超过目标，请调整选择或目标"];
        return;
      }
  }
  contentPlatform.value = state.rows[0].platform;
  rowPlatform.value = "全部";
  state.step = isQuick.value ? 2 : 1;
}
async function next() {
  errors.value = [];
  if (state.step === 0) {
    await allocate();
    return;
  }
  if (state.step === 1) {
    if (resourceErrors.value.length) {
      errors.value = resourceErrors.value;
      return;
    }
    state.step = 2;
    return;
  }
  preparing.value = true;
  try {
    await Promise.all(state.rows.map(ensureCoverResults));
    state.step = 3;
  } catch (e) {
    errors.value = [e.message || "部分封面未能生成，请检查并重试"];
  } finally {
    preparing.value = false;
  }
}
function back() {
  errors.value = [];
  state.step = steps.value[activeStep.value - 1].value;
}
async function applyDefaults(overwrite = false) {
  const p = contentPlatform.value,
    values = platformDefaults[p];
  if (!values.declaration) {
    errors.value = ["请先按实际内容确认创作声明"];
    return;
  }
  if (
    overwrite &&
    !(await feedback.confirmWarning({
      title: "覆盖当前平台的人工修改？",
      message: `将更新 ${state.rows.filter((r) => r.platform === p).length} 条 ${p} 任务的声明${p === "B站" ? "、标签与分区" : ""}。标题与封面不受影响。`,
      confirmText: "覆盖并应用",
    }))
  )
    return;
  const applied =
    p === "B站"
      ? {
          declaration: values.declaration,
          partition: values.partition,
          ...(values.tags.trim() ? { tags: values.tags } : {}),
        }
      : { declaration: values.declaration };
  const merged = mergeContentDefaults(state.rows, p, applied, overwrite);
  for (let i = 0; i < merged.length; i++)
    for (const f of Object.keys(applied))
      if (merged[i][f] !== state.rows[i][f]) {
        merged[i].fieldVersions = {
          ...merged[i].fieldVersions,
          [f]: (merged[i].fieldVersions?.[f] || 0) + 1,
        };
        merged[i].contentApproved = false;
      }
  state.rows = merged;
  errors.value = [];
  appliedNotice.platform = p;
  appliedNotice.text = `已应用到 ${state.rows.filter((r) => r.platform === p).length} 条${p}候选${overwrite ? "，所选字段已覆盖" : "，单条修改已保留"}。`;
}
function updateRow(row) {
  const index = state.rows.findIndex((r) => r.key === row.key);
  if (index < 0) return;
  const old = state.rows[index],
    overrides = new Set(old.overrides || []);
  for (const key of [
    "accountId",
    "shortTitle",
    "title",
    "description",
    "cover",
    "coverSource",
    "coverVariants",
    "declaration",
    "tags",
    "partition",
    "visibility",
    "schedule",
    "watermark",
    "templateId",
    "templateDocument",
  ])
    if (JSON.stringify(old[key]) !== JSON.stringify(row[key]))
      overrides.add(key);
  row.candidateId = old.candidateId;
  row.ai = old.ai;
  for (const key of overrides)
    if (JSON.stringify(old[key]) !== JSON.stringify(row[key])) {
      row.fieldVersions ||= {};
      row.fieldVersions[key] = (old.fieldVersions?.[key] || 0) + 1;
    }
  row.contentApproved = true;
  state.rows[index] = {
    ...copy(row),
    overrides: [...overrides],
    modified: overrides.size > 0,
  };
}
function removeRow(row) {
  state.rows = state.rows.filter((r) => r.key !== row.key);
}
async function create() {
  errors.value = [];
  if (!state.rows.length) {
    errors.value = ["没有可创建的任务"];
    return;
  }
  if (issues.value.length) {
    errors.value = [
      `还有 ${issues.value.length} 条任务需要处理，请补齐内容或明确移出本批次。`,
    ];
    return;
  }
  if (!isQuick.value && !state.name.trim()) {
    batchNameError.value = "请填写批次名称";
    batchNameInput.value?.focus();
    return;
  }
  batchNameError.value = "";
  let partial = false;
  if (
    !isQuick.value &&
    state.rows.length < state.targets.reduce((n, t) => n + Number(t.count), 0)
  ) {
    const target = state.targets.reduce((n, t) => n + Number(t.count), 0);
    partial = await feedback.confirmWarning({
      title: "明确确认部分创建",
      message: `目标 ${target}，实际正式创建 ${state.rows.length}，创建缺口 ${target - state.rows.length}；保留目标、不补造任务、不自动补量。`,
      confirmText: "保留缺口并创建",
    });
    if (!partial) return;
  }
  let result;
  try {
    result = createPublication(props.db, {
      platform: selectedPlatform.value,
      rows: state.rows,
      name: state.name,
      planId: state.planId,
      quick: isQuick.value,
      batchId: state.batchId,
      targets: state.targets,
      partial,
      creationKey: state.creationKey,
    });
  } catch (error) {
    errors.value = [error.message];
    return;
  }
  closing.value = true;
  visible.value = false;
  state.active = false;
  emit("created", result);
}
function saveDraft() {
  if (isQuick.value) {
    props.db.publishDraft = copy(state);
    feedback.toast.success("快速发布草稿已保留，不占用成片");
    return;
  }
  if (
    !state.name.trim() ||
    !state.platforms.length ||
    !state.targets.some((t) => t.count > 0)
  ) {
    errors.value = ["请填写批次名称、平台和目标后保存草稿"];
    return;
  }
  let b = props.db.batches.find((b) => b.id === state.batchId);
  if (!b) {
    b = {
      id: nextId(props.db),
      name: state.name,
      platform: state.platforms[0],
      planId: null,
      targets: copy(state.targets),
      categories: copy(state.categories),
      phase: "待资源配置",
      control: "待配置",
      candidates: copy(state.rows),
    };
    props.db.batches.push(b);
    state.batchId = b.id;
  }
  b.candidates = copy(state.rows);
  b.draft = copy(state);
  b.phase = state.step >= 2 ? "待内容配置" : "待资源配置";
  feedback.toast.success("草稿已保留，不占用成片");
}
const softConflicts = computed(() =>
  props.db.batches.filter(
    (b) =>
      b.id !== state.batchId &&
      b.platform === selectedPlatform.value &&
      (b.candidates || []).some((r) => state.assets.includes(r.assetId)),
  ),
);
function openBatch(batch) {
  if (batch.phase === "内容预检通过") return;
  const saved = batch.draft;
  contentPage.value = 1;
  Object.assign(state, {
    mode: "批量发布",
    step: saved?.step || 0,
    platforms: [batch.platform],
    categories: batch.targets.filter((t) => t.count > 0).map((t) => t.category),
    assets: saved?.assets || [],
    excludedAccounts: saved?.excludedAccounts || [],
    rows: copy(batch.candidates || []),
    name: batch.name,
    planId: batch.planId,
    batchId: batch.id,
    targets: copy(batch.targets),
    creationKey: saved?.creationKey || crypto.randomUUID(),
    active: true,
    accountLimits: saved?.accountLimits || {},
    accountOrder: saved?.accountOrder || [],
    quickAccountId: null,
  });
  state.rows.forEach(initCandidate);
  contentPlatform.value = batch.platform;
  errors.value = batch.needsPreflight
    ? ["计划目标已变更，兼容候选已保留，请回到资源阶段重新预检"]
    : [];
  if (batch.needsPreflight) state.step = 0;
  if (state.step === 0 && !state.assets.length) recommendAssets();
  closing.value = false;
  visible.value = true;
}
defineExpose({
  open,
  openBatch,
  resume: () => (visible.value = true),
  updateRow,
  hasDraft: computed(() => state.active),
  draftMode: computed(() => state.mode),
});
</script>
<template>
  <ResourceOverlay
    v-if="!closing"
    v-model:visible="visible"
    :title="state.mode === '快速发布' ? '快速发布' : '创建发布批次'"
    workspace
    kind="drawer"
    :width="1280"
    :busy="preparing"
  >
    <div class="composer wt-resource-page resource-visual-approved">
      <ResourceWizardSteps
        :current="activeStep"
        :steps="steps"
        class="composer-steps"
      />
      <t-alert
        v-if="errors.length"
        theme="warning"
        :message="errors.join('；')"
        class="composer-alert"
      />
      <t-alert
        v-if="!isQuick && softConflicts.length"
        theme="warning"
        :message="
          '软冲突：同平台草稿 ' +
          softConflicts.map((b) => b.name).join('、') +
          ' 也选择了这些成片；草稿不占用，正式创建将重查。'
        "
      />
      <div class="composer-workspace-grid">
        <div class="composer-workspace-main">
      <AIWorkspace
        v-if="state.step === 2"
        :rows="state.rows"
        :db="db"
        :single="isQuick"
        @preview="emit('preview', $event)"
      />
      <template v-if="isQuick && state.step === 0">
        <section class="composer-section quick-identity-card">
          <div class="section-heading"><div><span class="rail-kicker">SINGLE TASK / IDENTITY</span><h3>为这一条任务选择发布身份</h3><p>依次确认平台、类目、账号，再选择该平台尚未占用的一条成片。</p></div></div>
          <div class="quick-identity-grid">
            <label class="composer-field"><span>发布平台 <em>*</em></span><t-select v-model="selectedPlatform" placeholder="选择发布平台" :options="allowedPlatforms.map((p) => ({ label: p, value: p }))" aria-label="快速发布平台" /></label>
            <label class="composer-field"><span>内容类目 <em>*</em></span><t-select v-model="selectedCategory" :disabled="!selectedPlatform" placeholder="选择内容类目" :options="allowedCategories.map((c) => ({ label: c, value: c }))" aria-label="快速发布类目" /></label>
            <label class="composer-field"><span>发布账号 <em>*</em></span><t-select v-model="state.quickAccountId" :disabled="!selectedCategory" placeholder="明确选择一个账号" :options="eligibleAccounts.map((a) => ({ label: `${a.name} · ${a.quality} · 窗口 ${a.window}`, value: a.id }))" aria-label="快速发布账号" @change="state.assets = []; state.rows = []" /></label>
          </div>
          <ResourceNotice v-if="selectedCategory && !eligibleAccounts.length" tone="warning">当前平台与类目没有可用账号。请检查账号启停、登录状态和类目资格。</ResourceNotice>
          <p v-else-if="state.quickAccountId" class="quick-account-note">已选账号支持 {{ account(state.quickAccountId)?.categories.join('、') }}；成片列表只显示此账号可发布且同平台未占用的资源。</p>
        </section>
        <section class="composer-section quick-assets-card">
          <div class="section-heading"><div><span class="rail-kicker">SINGLE ASSET</span><h3>选择一条成片</h3><p>预览成片封面后确认；这一步不会占用资源。</p></div></div>
          <div v-if="state.quickAccountId" class="quick-asset-tools"><t-input v-model="search" clearable placeholder="搜索成片名称或 ID" aria-label="搜索快速发布成片" /><t-radio-group v-model="showUnavailable" variant="default-filled"><t-radio-button :value="false">可用 {{ availableAssets.length }}</t-radio-button><t-radio-button :value="true">不可用 {{ unavailableCount }}</t-radio-button></t-radio-group></div>
          <div v-if="state.quickAccountId" class="quick-asset-grid"><label v-for="video in visibleAssets" :key="video.id" class="quick-asset-choice" :class="{ selected: state.assets[0] === video.id, unavailable: showUnavailable }"><input type="radio" name="quick-asset" :checked="state.assets[0] === video.id" :disabled="showUnavailable" @change="selectAssets([video.id])" /><img :src="video.cover" :alt="video.title" /><span><strong>{{ video.title }}</strong><small>成片 {{ video.id }} · {{ video.quality }} · {{ video.duration }}</small><small v-if="showUnavailable" class="quick-unavailable-reason">{{ availability(video)[0]?.reason }}</small></span><t-button variant="text" @click.prevent="emit('preview', { src: video.cover, title: video.title })">预览</t-button></label><ResourceEmptyState v-if="!visibleAssets.length" :title="showUnavailable ? '没有不可用成片' : '没有可发布的成片'" :description="showUnavailable ? '当前范围中的成片均可使用。' : '更换账号或类目，或补充已完成且未占用的成片。'" /></div>
          <ResourceEmptyState v-else title="先选择发布账号" description="系统将按账号类目资格和同平台占用情况筛选成片。" />
        </section>
      </template>
      <template v-else-if="state.step === 0">
        <div class="batch-resource-overview" aria-live="polite"><div><small>本批次目标</small><strong>{{ batchTarget }}</strong><span>各类目目标合计</span></div><div><small>范围内可用成片</small><strong>{{ availableAssets.length }}</strong><span>已排除同平台占用</span></div><div><small>系统建议分配</small><strong>{{ suggested.length }}</strong><span>按账号与成片质量匹配</span></div><div :class="{ 'has-gap': Math.max(0, batchTarget - suggested.length) }"><small>当前分配缺口</small><strong>{{ Math.max(0, batchTarget - suggested.length) }}</strong><span>目标保持不变</span></div></div>
        <section class="composer-section">
          <div class="section-heading">
            <div>
              <h3>{{ state.planId ? '计划批次目标' : '设定单平台批次目标' }}</h3>
              <p>{{ state.planId ? '平台、类目和目标来自计划，当前只配置账号与成片。' : '先设定单平台和各类目目标，系统会按资源资格自动推荐。' }}</p>
            </div>
          </div>
          <div class="scope-grid">
            <div class="composer-field">
              <span>发布平台 <em>*</em></span>
              <t-select
                :popup-props="{
                  hideEmptyPopup: false,
                  attach: 'body',
                  zIndex: 5500,
                }"
                v-model="selectedPlatform"
                placeholder="选择一个发布平台"
                :disabled="!!state.planId"
                :options="allowedPlatforms.map((p) => ({ label: p, value: p }))"
                aria-label="批次发布平台"
              />
              <small class="muted"
                >一个批次只发布到一个平台；多个平台在计划中分别创建批次。</small
              >
            </div>
            <label class="composer-field"
              ><span>内容类目 <em>*</em></span
              ><t-select
                :popup-props="{
                  hideEmptyPopup: false,
                  attach: 'body',
                  zIndex: 5500,
                }"
                v-if="isQuick"
                v-model="selectedCategory"
                :options="
                  allowedCategories.map((c) => ({ label: c, value: c }))
                " /><t-select
                :popup-props="{
                  hideEmptyPopup: false,
                  attach: 'body',
                  zIndex: 5500,
                }"
                v-else
                v-model="state.categories"
                :disabled="!!state.planId"
                multiple
                clearable
                placeholder="可选择多个类目"
                :options="
                  allowedCategories.map((c) => ({ label: c, value: c }))
                "
            /></label>
          </div>
        </section>
        <section
          v-if="!isQuick && state.categories.length"
          class="composer-section"
        >
          <h3>
            {{
              state.planId ? "计划继承目标（只读）" : "各类目目标（手动输入）"
            }}
          </h3>
          <label v-for="c in state.categories" :key="c" class="target-row"
            >{{ c }}
            <t-input-number
              :value="state.targets.find((t) => t.category === c)?.count || 0"
              :disabled="!!state.planId"
              :min="0"
              :decimal-places="0"
              :aria-label="c + '批次目标'"
              @change="
                (v) => {
                  const t = state.targets.find((t) => t.category === c);
                  if (t) t.count = v;
                  else state.targets.push({ category: c, count: v });
                }
              "
            />
          </label>
          <p>目标、候选、正式任务分别统计。保存配置不占用成片。</p>
        </section>
        <template v-if="state.platforms.length && state.categories.length">
          <section class="composer-section batch-category-overview"><div class="section-heading"><div><h3>各类目资源与分配</h3><p>目标不随候选数量变化；缺口会保留到最终确认。</p></div></div><div class="batch-category-grid"><div v-for="row in resourceRows" :key="row.category"><strong>{{ row.category }}</strong><span>目标 {{ row.target }} · 可用 {{ row.available }}</span><span>建议分配 {{ row.allocated }} · 缺口 <b :class="{ 'has-gap': row.gap }">{{ row.gap }}</b></span></div></div><ResourceNotice v-if="resourceRows.some((row) => row.gap)" tone="warning">部分目标尚无可分配资源。可调整账号池、成片池或批次上限；也可以保留缺口进入内容配置，最终创建时需要明确确认部分创建。</ResourceNotice></section>
          <details class="inventory-details">
            <summary>
              平台与类目盘点
              <span>仅盘点当前平台；其他平台可在计划中单独安排</span>
            </summary>
            <t-table
              row-key="key"
              :data="inventory"
              size="small"
              :columns="[
                { colKey: 'platform', title: '平台', width: 110 },
                { colKey: 'category', title: '内容类目', minWidth: 140 },
                { colKey: 'accounts', title: '合格账号', width: 110 },
                { colKey: 'available', title: '可用成片', width: 110 },
                { colKey: 'occupied', title: '已占用', width: 110 },
                { colKey: 'unavailable', title: '其他不可用', width: 130 },
              ]"
            />
          </details>
          <section class="composer-section">
            <div class="section-heading">
              <div>
                <h3>系统推荐的成片池</h3>
                <p>已按类目目标和视频质量自动选择；需要时可人工增减。</p>
              </div>
              <t-button
                variant="outline"
                :disabled="!availableAssets.length || !batchTarget"
                @click="recommendAssets"
                >重新按目标推荐</t-button
              >
            </div>
            <div class="composer-table-tools">
              <t-input
                v-model="search"
                clearable
                placeholder="搜索成片名称或 ID"
                aria-label="搜索成片"
              /><t-radio-group
                v-model="showUnavailable"
                variant="default-filled"
                ><t-radio-button :value="false"
                  >可用 {{ availableAssets.length }}</t-radio-button
                ><t-radio-button :value="true"
                  >不可用 {{ unavailableCount }}</t-radio-button
                ></t-radio-group
              >
            </div>
            <ResourceSelectionToolbar
              :count="state.assets.length"
              unit="条成片"
              @clear="state.assets = []"
              ><span
                >建议分配 {{ suggested.length }} / 目标 {{ batchTarget }}；下一步可逐条调整账号</span
              ></ResourceSelectionToolbar
            >
            <t-alert
              v-if="invalidSelections.length"
              theme="warning"
              :message="`${invalidSelections.length} 条已选成片已不可安排，请恢复资源条件或移除。`"
              ><template #operation
                ><t-button
                  variant="text"
                  @click="
                    state.assets = state.assets.filter(
                      (id) => !invalidSelections.includes(id),
                    )
                  "
                  >移除失效项</t-button
                ></template
              ></t-alert
            >
            <t-table
              class="wt-resource-table"
              row-key="id"
              :data="visibleAssets"
              :columns="
                showUnavailable
                  ? assetColumns.filter((c) => c.type !== 'multiple')
                  : assetColumns
              "
              :selected-row-keys="state.assets"
              @select-change="selectAssets"
              :pagination="{
                current: assetPage,
                pageSize: 6,
                total: visibleAssets.length,
                showJumper: false,
                pageSizeOptions: [],
                showPageSize: false,
              }"
              @page-change="(p) => (assetPage = p.current)"
              :select-on-row-click="false"
            >
              <template #asset="{ row }"
                ><div class="title-cell">
                  <button
                    class="media-thumb"
                    @click="
                      emit('preview', { src: row.cover, title: row.title })
                    "
                  >
                    <img :src="row.cover" :alt="`预览${row.title}`" />
                  </button>
                  <div>
                    <strong>{{ row.title }}</strong
                    ><small>成片 {{ row.id }} · {{ row.duration }}</small>
                  </div>
                </div></template
              >
              <template #availability="{ row }"
                ><div class="availability-list">
                  <span
                    v-for="p in availability(row)"
                    :key="p.platform"
                    :class="{ 'is-unavailable': p.reason }"
                    ><i :class="{ 'is-unavailable': p.reason }" />{{ p.platform
                    }}<small v-if="p.reason">{{ p.reason }}</small></span
                  >
                </div></template
              >
              <template #empty
                ><ResourceEmptyState
                  :title="
                    showUnavailable ? '没有不可用成片' : '当前范围没有可用成片'
                  "
                  :description="
                    showUnavailable
                      ? '所有匹配成片均可安排。'
                      : '调整平台、类目或账号范围，也可以补充成片后继续。'
                  "
              /></template>
            </t-table>
          </section>
          <details class="account-scope">
            <summary>
              参与分配的账号
              <span
                >{{ accountIds.length }} / {{ eligibleAccounts.length }} 个 ·
                默认使用全部合格账号</span
              >
            </summary>
            <t-checkbox-group :value="accountIds" @change="toggleAccount"
              ><t-checkbox
                v-for="a in eligibleAccounts"
                :key="a.id"
                :value="a.id"
                >{{ a.platform }} · {{ a.name }}
                <span class="muted">{{ a.quality }}</span></t-checkbox
              ></t-checkbox-group
            >
            <div class="account-allocation-settings">
              <div v-for="(a, i) in orderedAccounts" :key="a.id">
                <span>{{ a.platform }} · {{ a.name }}</span
                ><label
                  >本批次上限<t-input-number
                    v-model="state.accountLimits[a.id]"
                    :min="1"
                    :decimal-places="0"
                    allow-input-over-limit
                    placeholder="不限制"
                    clearable /></label
                ><ResourceRowActions
                  ><t-button
                    variant="text"
                    :disabled="i === 0"
                    @click="reorderAccount(a.id, -1)"
                    >上移</t-button
                  ><t-button
                    variant="text"
                    :disabled="i === orderedAccounts.length - 1"
                    @click="reorderAccount(a.id, 1)"
                    >下移</t-button
                  ></ResourceRowActions
                >
              </div>
            </div>
            <ResourceNotice
              >账号按类目和质量匹配；未配置每日容量，不推算账号日额度。同一账号的任务依次执行。</ResourceNotice
            >
          </details>
        </template>
        <ResourceEmptyState
          v-else
          title="先选择平台与类目"
          description="资源盘点、可用成片和分配建议会随选择自动出现。"
        />
      </template>
      <template v-else-if="isQuick && state.step === 3">
        <section class="composer-section quick-preflight"><div class="section-heading"><div><span class="rail-kicker">FINAL PREFLIGHT</span><h3>预检这一条发布任务</h3><p>核对账号、成片、内容和封面。创建后只进入待执行，不会立即提交平台。</p></div><t-tag :theme="quickIssues.length ? 'warning' : 'success'" variant="light">{{ quickIssues.length ? `${quickIssues.length} 项待处理` : '可以创建' }}</t-tag></div>
          <div v-if="quickRow" class="quick-preflight-grid"><div class="quick-preflight-facts"><div><small>发布平台</small><strong>{{ quickRow.platform }}</strong></div><div><small>发布账号</small><strong>{{ account(quickRow.accountId)?.name }} · {{ account(quickRow.accountId)?.quality }}</strong></div><div><small>成片</small><strong>{{ asset(quickRow.assetId)?.title }}</strong></div><div><small>发布标题</small><strong>{{ quickRow.title || '待填写' }}</strong></div><div><small>{{ quickRow.coverMode !== '模板合成' ? 'AI 短标题（不写入现成封面）' : '封面短标题' }}</small><strong>{{ quickRow.shortTitle || '待填写' }}</strong></div><div><small>创作声明</small><strong>{{ quickRow.declaration || '待确认' }}</strong></div><div><small>可见范围 / 时间</small><strong>{{ quickRow.visibility || '默认' }} · {{ quickRow.schedule || '启动后尽快提交' }}</strong></div></div><button class="quick-cover-preview" @click="emit('preview', { row: quickRow })"><img :src="quickRow.cover" alt="最终封面预览" /><span>查看各比例成品</span></button></div>
          <div v-if="quickIssues.length" class="quick-issues"><h4>需要处理后才能创建</h4><div v-for="problem in quickIssues" :key="problem"><span>{{ problem }}</span><t-button variant="text" @click="state.step = /账号|成片|类目|占用/.test(problem) ? 0 : 2">去修复</t-button></div></div><div v-else class="quick-ready">账号、成片、发布内容与当前封面版本已通过预检。确认后生成一条独立待执行任务。</div>
        </section>
      </template>
      <template v-else-if="!isQuick">
        <div class="composer-context">
          <div>
            <strong>{{ state.platforms.join(" / ") }}</strong
            ><span
              >{{ state.categories.join("、") }} · {{ uniqueVideos }} 条成片 ·
              {{ state.rows.length }} 条任务</span
            >
          </div>
          <t-button variant="text" @click="state.step = 0"
            >调整资源范围</t-button
          >
        </div>
        <ResourceNotice v-if="unassigned.length" tone="warning"
          >有 {{ unassigned.length }} 个可用的平台分发组合因本批次上限未安排。
          <details>
            <summary>查看未安排内容</summary>
            <p v-for="item in unassigned" :key="item.key">
              {{ item.platform }} · {{ item.title }} · {{ item.reason }}
            </p>
          </details>
          本次只创建下表中已安排的
          {{ state.rows.length }}
          条任务；可返回提高上限或调整账号。</ResourceNotice
        >
        <template v-if="state.step === 1"
          ><div class="section-heading">
            <div>
              <h3>确认资源分配</h3>
              <p>
                系统已按质量、类目资格和账号顺序推荐。这里只检查账号与成片；标题和封面在下一阶段配置。
              </p>
            </div>
          </div><div class="batch-resource-overview compact"><div><small>批次目标</small><strong>{{ batchTarget }}</strong></div><div><small>已分配候选</small><strong>{{ state.rows.length }}</strong></div><div :class="{ 'has-gap': Math.max(0, batchTarget - state.rows.length) }"><small>剩余缺口</small><strong>{{ Math.max(0, batchTarget - state.rows.length) }}</strong></div></div><ResourceNotice v-if="Math.max(0, batchTarget - state.rows.length)" tone="warning">当前分配仍有缺口。可以返回资源范围补选；保留缺口继续配置内容时，最终创建必须明确确认部分创建。</ResourceNotice></template
        >
        <section v-if="state.step === 2 && !isQuick" class="platform-defaults">
          <div class="section-heading">
            <div>
              <h3>配置当前平台内容</h3>
              <p>按平台统一设置创作声明；单条文案与封面在上方工作区编辑。</p>
            </div>
          </div>
          <p>
            <strong>{{ selectedPlatform }}</strong> ·
            {{ state.rows.length }} 条任务
          </p>
          <div class="defaults-fields">
            <label class="composer-field"
              ><span>创作声明 <em>*</em></span
              ><t-select
                :popup-props="{
                  hideEmptyPopup: false,
                  attach: 'body',
                  zIndex: 5500,
                }"
                v-model="platformDefaults[contentPlatform].declaration"
                placeholder="按本次实际内容确认"
                :options="declarationOptions(contentPlatform)" /></label
            ><label v-if="contentPlatform === 'B站'" class="composer-field"
              ><span>分区 <em>*</em></span
              ><t-select
                :popup-props="{
                  hideEmptyPopup: false,
                  attach: 'body',
                  zIndex: 5500,
                }"
                v-model="platformDefaults[contentPlatform].partition"
                :options="[{ label: '游戏', value: '游戏' }]" /></label
            ><label v-if="contentPlatform === 'B站'" class="composer-field"
              ><span>标签</span
              ><t-input
                v-model="platformDefaults[contentPlatform].tags"
                placeholder="留空沿用各成片类目" /></label
            ><t-button variant="outline" @click="applyDefaults(false)"
              >应用并保留单条修改</t-button
            ><t-button variant="text" @click="applyDefaults(true)"
              >覆盖这些字段</t-button
            >
          </div>
          <ResourceNotice v-if="appliedNotice.platform === contentPlatform">{{
            appliedNotice.text
          }}</ResourceNotice
          ><ResourceNotice v-if="contentPlatform === '百家号'"
            >截图只确认了“无需声明”；其他内容类型待核验。挂载关闭、地点和活动不选、水印不添加。</ResourceNotice
          >
        </section>
        <template v-if="state.step === 3">
          <div class="final-summary">
            <div>
              <h3>检查最终发布内容</h3>
              <p>
                {{ readyCount }} 条内容已就绪<span v-if="issues.length"
                  >，还有 {{ issues.length }} 条需要处理</span
                >。创建后处于待执行，不会立即发布。
              </p>
            </div>
            <t-tag
              :theme="issues.length ? 'warning' : 'success'"
              variant="light"
              >{{ issues.length ? "有待处理项" : "可以创建" }}</t-tag
            >
          </div>
          <label v-if="!isQuick" class="composer-field batch-name"
            ><span>批次名称 <em>*</em></span
            ><t-input
              ref="batchNameInput"
              v-model="state.name"
              :status="batchNameError ? 'error' : 'default'"
              :tips="batchNameError"
              @change="batchNameError = ''"
              placeholder="例如：百度批次 A · 首轮发布"
              :maxlength="60"
          /></label>
        </template>
        <div v-if="state.step !== 2" class="composer-table-tools">
          <span>{{ selectedPlatform }} · {{ state.rows.length }} 条任务</span
          ><t-checkbox v-if="state.step > 1" v-model="onlyErrors"
            >只看待处理 {{ issues.length }}</t-checkbox
          >
        </div>
        <t-table
          v-if="state.step !== 2"
          class="wt-resource-table"
          row-key="key"
          :data="rows.slice((contentPage - 1) * 6, contentPage * 6)"
          :disable-data-page="true"
          @page-change="(p) => (contentPage = p.current)"
          :columns="candidateColumns"
          :pagination="{
            current: contentPage,
            pageSize: 6,
            total: rows.length,
            showJumper: false,
            pageSizeOptions: [],
            showPageSize: false,
          }"
        >
          <template #asset="{ row }"
            ><div class="candidate-asset">
              <button
                class="media-thumb"
                @click="emit('preview', { src: row.cover, title: row.title })"
              >
                <img :src="row.cover" alt="预览最终封面" />
              </button>
              <div>
                {{ asset(row.assetId)?.title
                }}<small
                  >成片 {{ row.assetId }} ·
                  {{ asset(row.assetId)?.quality }}</small
                >
              </div>
            </div></template
          >
          <template #platform="{ row }"
            >{{ row.platform
            }}<small class="cell-secondary">{{ row.category }}</small></template
          >
          <template #account="{ row }"
            ><t-select
              :popup-props="{
                hideEmptyPopup: false,
                attach: 'body',
                zIndex: 5500,
              }"
              v-if="state.step === 1"
              v-model="row.accountId"
              :options="accountOptions(row)"
              @change="
                row.overrides = [
                  ...new Set([...(row.overrides || []), 'accountId']),
                ]
              "
            /><template v-else
              >{{ account(row.accountId)?.name
              }}<small class="cell-secondary"
                >{{ account(row.accountId)?.quality }} · 窗口
                {{ account(row.accountId)?.window }}</small
              ></template
            ></template
          >
          <template #content="{ row }"
            ><div class="candidate-content">
              {{ row.platform === "百家号" ? row.description : row.title
              }}<small>{{ row.declaration || "创作声明待确认" }}</small
              ><small v-if="row.platform === 'B站'"
                >{{ row.partition }} · {{ row.tags }}</small
              ><small
                >{{ row.schedule || "启动后尽快提交"
                }}<span v-if="row.platform !== '百家号'">
                  · {{ row.visibility }}</span
                ></small
              ><t-tag
                v-if="row.overrides?.length"
                size="small"
                variant="outline"
                >有单条修改</t-tag
              >
            </div></template
          >
          <template #check="{ row }"
            ><t-tag
              :theme="(state.step === 1 ? resourceProblems(row) : validate(row, db)).length ? 'warning' : 'success'"
              variant="light"
              >{{ (state.step === 1 ? resourceProblems(row) : validate(row, db)).length ? "待处理" : "已就绪" }}</t-tag
            ><t-tooltip
              v-if="(state.step === 1 ? resourceProblems(row) : validate(row, db)).length"
              :content="(state.step === 1 ? resourceProblems(row) : validate(row, db)).join('；')"
              ><t-button
                class="issue-count"
                variant="text"
                @click="state.step === 1 ? state.step = 0 : emit('edit', row)"
                >{{ (state.step === 1 ? resourceProblems(row) : validate(row, db)).length }} 项问题</t-button
              ></t-tooltip
            ></template
          >
          <template #op="{ row }"
            ><ResourceRowActions
              ><t-button variant="text" @click="emit('preview', { row })"
                >预览</t-button
              ><t-button v-if="state.step !== 1" variant="text" @click="emit('edit', row)"
                >编辑</t-button
              ><t-button
                variant="text"
                class="is-danger"
                @click="removeRow(row)"
                >移出</t-button
              ></ResourceRowActions
            ></template
          >
          <template #empty
            ><ResourceEmptyState
              :title="onlyErrors ? '当前筛选下没有待处理项' : '没有任务候选'"
              description="返回调整当前平台的资源范围，或取消仅看待处理。"
          /></template>
        </t-table>
        <ResourceNotice v-if="state.step === 3"
          >最终检查包含账号、文案、封面和平台设置。点击“预览”检查完整内容；创建前会再次校验资源占用。</ResourceNotice
        >
      </template>
        </div>
        <aside class="composer-workspace-rail">
          <template v-if="isQuick">
            <section class="composer-rail-card"><span class="rail-kicker">ONE TASK</span><h3>独立任务</h3><dl><div><dt>平台</dt><dd>{{ selectedPlatform || '待选择' }}</dd></div><div><dt>账号</dt><dd>{{ account(state.quickAccountId)?.name || '待选择' }}</dd></div><div><dt>成片</dt><dd>{{ asset(state.assets[0])?.title || '待选择' }}</dd></div><div><dt>所属批次</dt><dd>无</dd></div></dl></section>
            <section class="composer-rail-card"><span class="rail-kicker">NEXT STEP</span><h3>{{ state.step === 0 ? '先确定发布身份' : state.step === 2 ? '完成内容与封面' : '核对并创建' }}</h3><ul><li :class="{ done: !!selectedPlatform }">一个平台与类目</li><li :class="{ done: !!state.quickAccountId }">明确选择一个账号</li><li :class="{ done: state.assets.length === 1 }">选择一条可用成片</li><li :class="{ done: state.step === 3 && !quickIssues.length }">内容与封面预检通过</li></ul></section>
            <section class="composer-rail-card rail-note"><strong>创建结果</strong><p>确认后仅创建一条待执行任务并占用该平台的这条成片；不会进入计划或批次，也不会自动发布。</p></section>
          </template>
          <template v-else>
          <section class="composer-rail-card"><span class="rail-kicker">SCOPE & PROGRESS</span><h3>本次配置</h3><dl><div><dt>创建方式</dt><dd>{{ isQuick ? '单任务快速发布' : state.planId ? '计划内批次' : '独立批次' }}</dd></div><div><dt>发布平台</dt><dd>{{ selectedPlatform || '待选择' }}</dd></div><div><dt>内容类目</dt><dd>{{ state.categories.join('、') || '待选择' }}</dd></div><div><dt>已选成片</dt><dd>{{ selectedAssets.length }} 条</dd></div><div><dt>候选任务</dt><dd>{{ state.rows.length || suggested.length }} 条</dd></div></dl></section>
          <section class="composer-rail-card"><span class="rail-kicker">READINESS</span><h3>{{ state.step <= 1 ? '资源检查' : '内容检查' }}</h3><ul><li :class="{ done: !!selectedPlatform }">选择单一发布平台</li><li :class="{ done: !!state.categories.length }">确认内容类目</li><li :class="{ done: !!selectedAssets.length }">选择可用成片</li><li :class="{ done: state.step <= 1 ? !!state.rows.length && !resourceErrors.length : !issues.length }">{{ state.step <= 1 ? '确认账号与成片分配' : '完成内容与封面预检' }}</li></ul><p v-if="state.step === 1 && resourceErrors.length" class="rail-warning">{{ resourceErrors.length }} 项资源问题仍需处理</p><p v-else-if="state.step === 1" class="rail-success">{{ state.rows.length }} 条资源分配成立，可进入内容配置</p><p v-else-if="state.step > 1 && issues.length" class="rail-warning">{{ issues.length }} 项内容仍需处理</p><p v-else-if="state.step > 1" class="rail-success">{{ readyCount }} 条内容已就绪</p></section>
          <section class="composer-rail-card rail-note"><strong>创建后会怎样？</strong><p>创建后进入待执行队列。本地原型不会向真实平台提交内容。</p></section>
          </template>
        </aside>
      </div>
    </div>
    <template #footer
      ><span class="composer-footer-note">{{ isQuick ? (state.step === 0 ? '选择一个账号和一条成片' : state.step === 2 ? '完成单条内容和封面审核' : quickIssues.length ? `${quickIssues.length} 项待处理` : '预检通过 · 待创建') : state.step === 0 ? `目标 ${batchTarget} · 建议分配 ${suggested.length} · 缺口 ${Math.max(0, batchTarget - suggested.length)}` : state.step === 1 ? `目标 ${batchTarget} · 已分配 ${state.rows.length} · 资源问题 ${resourceErrors.length}` : `目标 ${batchTarget} · 候选 ${state.rows.length} · 已就绪 ${readyCount}` }}</span
      ><t-button variant="outline" @click="saveDraft">保存草稿</t-button
      ><t-button theme="default" variant="outline" @click="visible = false"
        >保留配置并关闭</t-button
      ><t-button
        v-if="state.step > 0"
        theme="default"
        variant="outline"
        @click="back"
        >上一步</t-button
      ><t-button
        v-if="state.step < 3"
        :loading="preparing"
        :disabled="state.step === 0 ? (isQuick ? !state.quickAccountId || state.assets.length !== 1 : !suggested.length) : !state.rows.length"
        @click="next"
        >{{
          state.step === 0
            ? isQuick ? "配置这条任务的内容" : "检查资源分配"
            : state.step === 1
              ? "进入 AI 内容配置"
              : "进入最终预检"
        }}</t-button
      ><t-button
        v-else
        :disabled="!state.rows.length || !!issues.length"
        @click="create"
        >{{ isQuick ? '创建一条待执行任务' : `创建 ${state.rows.length} 条待执行任务` }}</t-button
      ></template
    >
  </ResourceOverlay>
</template>
<style scoped>
.quick-identity-grid { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:15px; }
.quick-account-note { margin:15px 0 0; padding:10px 12px; background:#eff6ff; border-radius:6px; color:#57759d; font-size:12px; }
.quick-asset-tools { display:flex; justify-content:space-between; gap:12px; align-items:center; flex-wrap:wrap; margin:0 0 13px; }
.quick-asset-tools :deep(.t-input__wrap) { max-width:320px; }
.quick-asset-grid { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:10px; }
.quick-asset-choice { display:flex; align-items:center; gap:12px; min-width:0; padding:10px 12px; border:1px solid #e1e8f2; border-radius:8px; cursor:pointer; }
.quick-asset-choice.selected { border-color:#286bd5; background:#edf5ff; box-shadow:inset 0 0 0 1px #286bd5; }
.quick-asset-choice.unavailable { cursor:default; opacity:.65; }
.quick-asset-choice img { width:88px; height:58px; object-fit:cover; border-radius:5px; flex:none; }
.quick-asset-choice span { display:flex; flex-direction:column; gap:5px; min-width:0; flex:1; }
.quick-asset-choice strong { overflow:hidden; white-space:nowrap; text-overflow:ellipsis; color:#263d60; font-size:13px; }
.quick-asset-choice small { color:#8593a7; font-size:11px; }
.quick-asset-choice .quick-unavailable-reason { color:#a65e31; }
.quick-preflight-grid { display:grid; grid-template-columns:minmax(0,1fr) 220px; gap:16px; }
.quick-preflight-facts { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:8px; }
.quick-preflight-facts > div { display:flex; flex-direction:column; gap:5px; min-width:0; padding:10px 12px; background:#f8faff; border-radius:6px; }
.quick-preflight-facts small { color:#8190a6; font-size:11px; }
.quick-preflight-facts strong { color:#344866; font-size:13px; overflow-wrap:anywhere; }
.quick-cover-preview { display:flex; flex-direction:column; align-items:stretch; padding:0; border:1px solid #dce5f2; border-radius:7px; background:#fff; cursor:zoom-in; overflow:hidden; }
.quick-cover-preview img { display:block; width:100%; aspect-ratio:4/3; object-fit:cover; }
.quick-cover-preview span { padding:9px; color:#286bd5; font-size:12px; }
.quick-issues { margin-top:17px; padding:15px; border-radius:8px; background:#fff6ea; }
.quick-issues h4 { margin:0 0 9px; color:#955f0e; }
.quick-issues > div { display:flex; justify-content:space-between; align-items:center; gap:10px; padding:6px 0; border-top:1px solid #f1ddbe; font-size:12px; }
.quick-ready { margin-top:17px; padding:13px; border-radius:7px; background:#ecf9f2; color:#2a8054; font-size:12px; }
.batch-resource-overview { display:grid; grid-template-columns:repeat(4,minmax(0,1fr)); gap:10px; margin:0 0 15px; }
.batch-resource-overview > div { display:flex; flex-direction:column; gap:3px; min-width:0; padding:14px 16px; border:1px solid #e2e9f3; border-radius:9px; background:#fff; box-shadow:0 3px 14px #24446b0a; }
.batch-resource-overview small,.batch-resource-overview span { color:#7b8ba3; font-size:11px; }
.batch-resource-overview strong { color:#236bd7; font-size:27px; line-height:1.2; }
.batch-resource-overview .has-gap strong,.batch-category-grid .has-gap { color:#d38722; }
.batch-resource-overview.compact { grid-template-columns:repeat(3,minmax(0,1fr)); margin:12px 0 17px; }
.batch-category-grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(180px,1fr)); gap:9px; }
.batch-category-grid > div { display:flex; flex-direction:column; gap:5px; padding:12px; border:1px solid #e5ebf4; border-radius:7px; background:#fbfcff; }
.batch-category-grid strong { color:#2d466c; font-size:13px; }
.batch-category-grid span { color:#6e819d; font-size:12px; }
.batch-category-overview :deep(.t-alert) { margin-top:12px; }
.composer-workspace-grid { display:grid; grid-template-columns:minmax(0,1fr) 295px; gap:18px; align-items:start; }
.composer-workspace-main { min-width:0; }
.composer-workspace-rail { display:flex; flex-direction:column; gap:12px; position:sticky; top:0; }
.composer-rail-card { padding:18px; border:1px solid #e2e9f3; border-radius:10px; background:#fff; box-shadow:0 3px 14px #24446b0a; }
.composer-rail-card h3 { margin:4px 0 13px; font-size:15px; color:#20385e; }
.rail-kicker { color:#3976d6; font-size:10px; font-weight:700; letter-spacing:.1em; }
.composer-rail-card dl { margin:0; }
.composer-rail-card dl div { display:flex; justify-content:space-between; gap:10px; padding:9px 0; border-top:1px solid #edf1f7; font-size:12px; }
.composer-rail-card dt { color:#8290a5; }
.composer-rail-card dd { margin:0; max-width:165px; color:#344866; text-align:right; font-weight:600; overflow-wrap:anywhere; }
.composer-rail-card ul { margin:0; padding:0; list-style:none; }
.composer-rail-card li { position:relative; padding:8px 0 8px 21px; color:#72829a; font-size:12px; border-top:1px solid #edf1f7; }
.composer-rail-card li:before { content:'○'; position:absolute; left:0; color:#a4b0c0; }
.composer-rail-card li.done { color:#2d6b56; }
.composer-rail-card li.done:before { content:'✓'; color:#35a976; font-weight:700; }
.rail-warning,.rail-success { padding:9px 11px; border-radius:6px; font-size:12px; margin:10px 0 0; }
.rail-warning { background:#fff6e8; color:#955f0e; }
.rail-success { background:#edf9f3; color:#2d7955; }
.rail-note { background:#f7faff; }
.rail-note strong { display:block; margin-bottom:7px; font-size:13px; color:#2d5fa9; }
.rail-note p { margin:0; color:#69809d; font-size:12px; line-height:1.7; }
.composer {
  color: var(--wt-text-primary);
}
.composer-steps {
  margin: 0 0 28px;
}
.composer-alert {
  margin-bottom: 20px;
}
.composer-section {
  margin: 0 0 14px;
  padding:22px;
  border:1px solid #e2e9f3;
  border-radius:10px;
  background:#fff;
  box-shadow:0 3px 14px #24446b0a;
}
@media (max-width:1100px) { .composer-workspace-grid { grid-template-columns:1fr; } .composer-workspace-rail { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); position:static; } .composer-workspace-rail .rail-note { grid-column:1/-1; } }
@media (max-width:700px) { .quick-identity-grid,.quick-asset-grid,.quick-preflight-grid,.batch-resource-overview { grid-template-columns:1fr; } .quick-preflight-facts,.composer-workspace-rail { grid-template-columns:1fr; } }
.section-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  margin-bottom: 20px;
}
.section-heading h3,
.final-summary h3 {
  font-size: 16px;
  margin: 0;
  font-weight: 600;
}
.section-heading p,
.final-summary p {
  font-size: 13px;
  color: var(--wt-text-tertiary);
  margin: 6px 0 0;
  line-height: 1.7;
}
.scope-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 24px;
}
.composer-field {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
  font-size: 13px;
}
.composer-field em {
  font-style: normal;
  color: var(--wt-danger);
}
.composer-field .t-checkbox-group {
  min-height: 36px;
  align-items: center;
  display: flex;
  flex-wrap: wrap;
  gap: 18px;
}
.inventory-summary {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  border: 1px solid var(--wt-border);
  border-radius: 8px;
  background: var(--wt-bg-page);
  margin: 24px 0;
}
.inventory-summary > div {
  padding: 18px 22px;
  border-right: 1px solid var(--wt-border);
}
.inventory-summary > div:last-child {
  border-right: 0;
}
.inventory-summary span {
  display: block;
  font-size: 12px;
  color: var(--wt-text-tertiary);
}
.inventory-summary strong {
  font-size: 28px;
  line-height: 1.5;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}
.inventory-summary small {
  font-size: 12px;
  font-weight: 400;
  margin-left: 8px;
  color: var(--wt-text-tertiary);
}
.is-highlight strong {
  color: var(--wt-primary);
}
.inventory-details {
  border-bottom: 1px solid var(--wt-border);
  padding-bottom: 18px;
}
.inventory-details summary,
.account-scope summary {
  cursor: pointer;
  font-weight: 500;
  margin-bottom: 14px;
  font-size: 13px;
}
.inventory-details summary span,
.account-scope summary span {
  color: var(--wt-text-tertiary);
  font-size: 12px;
  font-weight: 400;
  margin-left: 10px;
}
.composer-table-tools {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
  margin: 20px 0 16px;
}
.composer-table-tools > .t-input__wrap {
  max-width: 290px;
}
.availability-list {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}
.availability-list > span {
  font-size: 12px;
  display: flex;
  align-items: center;
  gap: 5px;
}
.availability-list i {
  width: 6px;
  height: 6px;
  background: var(--wt-success);
  border-radius: 50%;
}
.availability-list i.is-unavailable {
  background: var(--wt-text-placeholder);
}
.availability-list .is-unavailable {
  color: var(--wt-text-tertiary);
}
.availability-list small {
  font-size: 11px;
}
.account-scope {
  border-top: 1px solid var(--wt-border);
  padding-top: 20px;
  margin-top: 24px;
}
.account-allocation-settings {
  margin-top: 20px;
  border-top: 1px solid var(--wt-border);
}
.account-allocation-settings > div {
  display: flex;
  gap: 20px;
  align-items: center;
  padding: 12px 0;
  border-bottom: 1px solid var(--wt-border);
  flex-wrap: wrap;
}
.account-allocation-settings > div > span {
  flex: 1;
  min-width: 160px;
  font-size: 13px;
}
.account-allocation-settings label {
  display: flex;
  gap: 10px;
  align-items: center;
  font-size: 12px;
  color: var(--wt-text-tertiary);
}
.account-allocation-settings .t-input-number {
  width: 150px;
}
.account-scope .t-checkbox-group {
  display: flex;
  flex-wrap: wrap;
  gap: 16px 24px;
}
.composer-context {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 20px;
  background: var(--wt-bg-page);
  border: 1px solid var(--wt-border);
  border-radius: 6px;
  margin-bottom: 24px;
  gap: 16px;
}
.composer-context strong {
  display: block;
  font-size: 14px;
}
.composer-context span {
  display: block;
  font-size: 12px;
  color: var(--wt-text-tertiary);
  margin-top: 4px;
}
.defaults-fields {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 16px;
  margin: 20px 0;
}
.defaults-fields > .composer-field {
  flex: 1 1 190px;
}
.platform-defaults {
  padding-bottom: 8px;
  border-bottom: 1px solid var(--wt-border);
}
.candidate-asset {
  display: flex;
  gap: 10px;
  align-items: center;
  min-width: 0;
}
.candidate-asset > div {
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 13px;
}
.candidate-asset small,
.cell-secondary,
.candidate-content small {
  display: block;
  font-size: 12px;
  color: var(--wt-text-tertiary);
  margin-top: 5px;
}
.candidate-content {
  line-height: 1.7;
  overflow-wrap: anywhere;
}
.candidate-content .t-tag {
  margin-top: 6px;
}
.issue-count {
  display: block;
  font-size: 12px;
  color: var(--wt-warning);
  margin-top: 6px;
  cursor: help;
}
.final-summary {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 20px;
  margin-bottom: 24px;
}
.batch-name {
  max-width: 560px;
  margin-bottom: 24px;
}
.composer-footer-note {
  margin-right: auto;
  color: var(--wt-text-tertiary);
  font-size: 12px;
}
.media-thumb {
  border: 0;
  padding: 0;
  background: transparent;
  cursor: zoom-in;
  flex-shrink: 0;
}
.media-thumb img {
  display: block;
  width: 64px;
  height: 48px;
  object-fit: cover;
  border-radius: 4px;
}
.title-cell {
  display: flex;
  align-items: center;
  gap: 12px;
}
.title-cell strong {
  font-size: 13px;
  font-weight: 500;
}
.title-cell small {
  display: block;
  font-size: 12px;
  color: var(--wt-text-tertiary);
  margin-top: 5px;
}
@media (max-width: 700px) {
  .scope-grid {
    grid-template-columns: 1fr;
  }
  .inventory-summary {
    grid-template-columns: 1fr 1fr;
  }
  .inventory-summary > div:nth-child(2) {
    border-right: 0;
  }
  .inventory-summary > div {
    padding: 14px 16px;
  }
  .inventory-summary > div:nth-child(-n + 2) {
    border-bottom: 1px solid var(--wt-border);
  }
  .inventory-details summary span {
    display: block;
    margin: 6px 0;
  }
  .composer-footer-note {
    flex-basis: 100%;
  }
  .composer-context {
    align-items: flex-start;
  }
  .composer-steps {
    margin-bottom: 24px;
  }
}
</style>
