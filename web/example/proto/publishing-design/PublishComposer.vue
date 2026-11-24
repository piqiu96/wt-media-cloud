<script setup>
import { computed, reactive, ref, watch, nextTick } from "vue";
import {
  MoveIcon,
  CheckIcon,
  RefreshIcon,
} from "tdesign-icons-vue-next";
import ResourceOverlay from "./PrototypeOverlay.vue";
import { batchCreationDraft, promoteBatchDraft } from "./creationDrafts.js";
import { chooseDraft } from "./draftRecovery.js";
import ResourceRowActions from '../../../src/shared/ui/resource/ResourceRowActions.vue';
import ResourceFieldHelp from "../../../src/shared/ui/resource/ResourceFieldHelp.vue";
import ResourceWizardSteps from "../../../src/shared/ui/resource/ResourceWizardSteps.vue";
import BatchContentWorkspace from "./BatchContentWorkspace.vue";
import BatchContentTable from "./BatchContentTable.vue";
import ResourceCard from "../../../src/shared/ui/resource/ResourceCard.vue";
import ResourceSection from "../../../src/shared/ui/resource/ResourceSection.vue";
import ResourceNotice from "../../../src/shared/ui/resource/ResourceNotice.vue";
import { publishMethodLabel } from "./publishingSettings.js";
import { taskInterval } from "./batchExecutionSettings.js";
import { initCandidate } from "./content.js";
import { recommendCategoryQuantities } from "./batchRecommendations.js";
import { createPublication, nextId } from "./relationships.js";
import { currentPrototypeUser } from "./prototypeSession.js";
import { preparationRecord, previewExecutionSettings, previewGroupPolicies } from "./preparationPreview.js";
import { ensureCoverResults } from "./covers.js";
import { feedback } from "../../../src/shared/ui/feedback/index.js";
import {
  platforms,
  publishingCategories,
  validate,
  resourceStatus,
  suggestDistribution,
} from "./model.js";
import "./batchCreation.css";
const props = defineProps({
  db: { type: Object, required: true },
  persist: { type: Function, default: async () => true },
  preparationOnly: Boolean,
});
const emit = defineEmits(["edit", "preview", "created", "return-source", "closed"]);
const copy = (value) => JSON.parse(JSON.stringify(value));
const visible = ref(false),
  busy = ref(false),
  contentBusy = ref(false),
  resultSaved = ref(false),
  errors = ref([]),
  nameError = ref(false),
  matchingSection = ref(null),
  accountSelection = ref(null),
  activeCategory = ref(""),
  contentRef = ref(null);
const root = ref(null),
  createdResult = ref(null);
const availableCategory = ref("");
const draggedCategory = ref("");
const reviewPending = ref(false), reviewPage = ref(1), reviewPageSize = ref(10);
const lockOpen = ref(false),
  replaceRow = ref(null),
  replacement = ref(null),
  replaceSearch = ref("");
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
  desiredQuantity: null,
  sourceAssetIds: [],
  creationKey: null,
  active: false,
  categoryAccountIds: {},
  accountLimits: {},
  accountOrder: [],
  resourcesLocked: false,
  savedAt: null,
  taskIntervalSeconds: 0,
  executionSettings: previewExecutionSettings(),
});
const editBaseline = ref('');
watch(visible, value => { if (value) { createdResult.value = null; resultSaved.value = false; } });
if (props.db.publishDraft && props.db.publishDraft.mode !== "快速发布" && !props.db.publishDraft.batchId) Object.assign(state, copy(props.db.publishDraft));
if (state.step === 1) state.step = 0;
watch(
  () => state.step,
  async () => {
    await nextTick();
    if (visible.value && root.value?.closest(".resource-editor-page")) window.scrollTo(0, 0);
    else root.value?.closest(".t-drawer__body")?.scrollTo({ top: 0 });
  },
);

watch(
  state,
  () => {
    if (visible.value && !state.batchId) props.db.publishDraft = copy(state);
  },
  { deep: true },
);
const platform = computed(() => state.platforms[0]);
const plan = computed(() => props.db.plans.find((p) => p.id === state.planId));
const allowedPlatforms = computed(() =>
  plan.value
    ? [...new Set(plan.value.targets.map((t) => t.platform))]
    : props.preparationOnly ? ["百家号"] : platforms,
);
const allowedCategories = computed(() =>
  plan.value
    ? [
        ...new Set(
          plan.value.targets
            .filter((t) => t.platform === platform.value)
            .map((t) => t.category),
        ),
      ]
    : publishingCategories(props.db).filter(category => !props.preparationOnly || currentPrototypeUser().categories.includes(category)),
);
const asset = (id) => props.db.assets.find((v) => v.id === id);
const account = (id) => props.db.accounts.find((a) => a.id === id);
const eligibleAccounts = computed(() =>
  props.db.accounts
    .filter(
      (a) =>
        a.enabled &&
        (!props.preparationOnly || a.ownerId === currentPrototypeUser().id) &&
        a.loginValid !== false &&
        a.platform === platform.value &&
        a.categories.some((c) => state.categories.includes(c)),
    )
    .sort((a, b) => {
      const order = state.accountOrder;
      return (
        (order.indexOf(a.id) < 0 ? 999 : order.indexOf(a.id)) -
        (order.indexOf(b.id) < 0 ? 999 : order.indexOf(b.id))
      );
    }),
);
function accountsForCategory(category) {
  const selected = state.categoryAccountIds?.[category];
  return eligibleAccounts.value.filter(a => a.categories.includes(category) &&
    (Array.isArray(selected) ? selected.includes(a.id) : !state.excludedAccounts.includes(a.id))).map(a => a.id);
}
const accountIds = computed(() => eligibleAccounts.value
  .filter(a => state.categories.some(c => accountsForCategory(c).includes(a.id))).map(a => a.id));
function selectCategoryAccounts(category) {
  accountSelection.value = { category, ids: accountsForCategory(category), limits: copy(state.accountLimits) };
}
function applyCategoryAccounts() {
  if (!accountSelection.value || state.resourcesLocked) return;
  const selection = accountSelection.value;
  state.categoryAccountIds[selection.category] = [...selection.ids];
  state.accountLimits = copy(selection.limits);
  accountSelection.value = null;
}
const availableAssets = computed(() =>
  props.db.assets.filter(
    (v) =>
      state.categories.includes(v.category) &&
      (!state.sourceAssetIds?.length || state.sourceAssetIds.includes(v.id)) &&
      !resourceStatus(props.db, v, platform.value, accountIds.value),
  ),
);
const missingResources = computed(() =>
  categoryRows.value
    .filter((r) => !r.accounts || !r.available)
    .map(
      (r) =>
        `${r.category}：${!r.accounts ? "没有符合类目资格的可用账号" : ""}${!r.accounts && !r.available ? "，" : ""}${!r.available ? "没有可分配成片" : ""}`,
    )
    .join("；"),
);
const target = computed(() =>
  state.targets
    .filter((t) => state.categories.includes(t.category))
    .reduce((n, t) => n + Number(t.count || 0), 0),
);
const suggested = computed(() =>
  suggestDistribution(
    props.db,
    state.platforms,
    state.assets,
    accountIds.value,
    state.rows,
    state.accountLimits,
    state.categories,
    state.categoryAccountIds,
  ),
);
const allocated = computed(() => state.rows);
const gap = computed(() => Math.max(0, target.value - allocated.value.length));
const issues = computed(() =>
  state.rows.filter((r) => validate(r, props.db, r.id).length),
);
const ready = computed(() => state.rows.length - issues.value.length);
const stage = computed(() => (state.step < 2 ? 0 : state.step - 1));
const stages = computed(() => props.preparationOnly ? [["资源配置"], ["内容准备"], ["结果总览"]] : [["资源配置"], ["内容配置"], ["创建检查"]]);
const reviewRows = computed(()=>reviewPending.value ? issues.value : state.rows);
const categoryAssets = computed(()=>availableAssets.value.filter(asset=>asset.category===availableCategory.value));
function showReviewPending(){reviewPending.value=true;reviewPage.value=1;nextTick(()=>root.value?.querySelector('.batch-review-table')?.scrollIntoView({block:'start'}));}
const categoryRows = computed(() =>
  state.categories.map((category) => ({
    category,
    target: Number(
      state.targets.find((t) => t.category === category)?.count || 0,
    ),
    available: availableAssets.value.filter((v) => v.category === category)
      .length,
    accounts: accountsForCategory(category).length,
    allocated: allocated.value.filter((r) => r.category === category).length,
  })),
);
const matchingRows = computed(() =>
  state.rows.filter(
    (r) => r.category === (activeCategory.value || state.categories[0]),
  ),
);
const resourceProblems = (row) => {
  const problems = [];
  const a = account(row.accountId),
    v = asset(row.assetId);
  if (
    !a?.enabled ||
    a.loginValid === false ||
    a.platform !== row.platform ||
    !a.categories.includes(row.category) ||
    !accountsForCategory(row.category).includes(a.id)
  )
    problems.push("账号不可用或不符合类目，请更换账号");
  if (!v) problems.push("成片不存在，请更换成片");
  else {
    const reason = resourceStatus(props.db, v, row.platform, [row.accountId]);
    if (reason) problems.push(reason);
  }
  if (
    state.rows.filter(
      (r) => r.assetId === row.assetId && r.platform === row.platform,
    ).length > 1
  )
    problems.push("同一成片重复分配，请更换成片");
  if (
    state.accountLimits[row.accountId] &&
    state.rows.filter((r) => r.accountId === row.accountId).length >
      state.accountLimits[row.accountId]
  )
    problems.push("超过账号本批次上限，请调整账号或上限");
  return problems;
};
const resourceErrors = computed(() =>
  state.rows.flatMap((r) =>
    resourceProblems(r).map((p) => `${asset(r.assetId)?.title}：${p}`),
  ),
);
const softConflicts = computed(() =>
  props.db.batches.filter(
    (b) =>
      b.id !== state.batchId &&
      !props.db.migratedBatchDraftIds?.includes(b.id) &&
      b.platform === platform.value &&
      (b.candidates || []).some((r) => state.assets.includes(r.assetId)),
  ),
);
watch(
  () => state.categories.slice(),
  (list) => {
    state.targets = state.targets.filter((t) => list.includes(t.category));
    for (const c of list)
      if (!state.targets.some((t) => t.category === c))
        state.targets.push({ category: c, count: 0 });
    if (visible.value && state.step === 0 && !state.planId && !target.value) recommendTargets();
    if (!list.includes(activeCategory.value))
      activeCategory.value = list[0] || "";
  },
);
watch(
  () => [state.platforms, state.categories, state.targets, state.excludedAccounts, state.accountLimits, state.accountOrder, state.categoryAccountIds],
  syncResourceRows,
  { deep: true },
);
function syncResourceRows() {
  if (!visible.value || state.step >= 2 || state.resourcesLocked) return;
  recommendAssets();
  state.rows = copy(suggested.value);
  state.rows.forEach(initCandidate);
}
watch(() => state.name, () => { nameError.value = false; });
async function applyRecommendations() {
  recommendTargets();
  syncResourceRows();
  await nextTick();
  matchingSection.value?.scrollIntoView({ behavior: 'smooth', block: 'start' });
}
async function changePlatform(value) {
  if (value === platform.value) return;
  if (
    state.assets.length &&
    !(await feedback.confirmWarning({
      title: "切换发布平台？",
      message: "成片与账号分配将清空，批次名称和类目保留。",
      confirmText: "切换平台",
    }))
  )
    return;
  state.platforms = value ? [value] : [];
  state.assets = [];
  state.rows = [];
  state.excludedAccounts = [];
  state.categoryAccountIds = {};
  state.accountLimits = {};
  state.accountOrder = [];
  errors.value = [];
  if (!state.planId && state.categories.length) recommendTargets();
}
function setTarget(category, value) {
  const t = state.targets.find((t) => t.category === category);
  if (t) t.count = Math.max(0, Math.floor(Number(value) || 0));
}
function recommendTargets() {
  if (state.planId) return;
  const capacityRows = suggestDistribution(props.db, state.platforms,
    availableAssets.value.map(v => v.id), accountIds.value, [], state.accountLimits, state.categories, state.categoryAccountIds);
  const capacities = state.categories.map(category => ({ category,
    capacity: capacityRows.filter(row => row.category === category).length }));
  for (const row of recommendCategoryQuantities(capacities, state.desiredQuantity))
    setTarget(row.category, row.count);
  recommendAssets();
}
function recommendAssets() {
  const ids = [];
  for (const c of categoryRows.value) {
    const pinned = state.rows
      .filter(
        (r) =>
          r.category === c.category &&
          r.overrides?.includes("assetId") &&
          availableAssets.value.some((v) => v.id === r.assetId),
      )
      .map((r) => r.assetId);
    const pool = availableAssets.value
      .filter((v) => v.category === c.category && !pinned.includes(v.id))
      .sort(
        (a, b) =>
          Number(b.quality === "优质") - Number(a.quality === "优质") ||
          a.id - b.id,
      )
      .map((v) => v.id);
    ids.push(...[...pinned, ...pool].slice(0, c.target));
  }
  state.assets = ids;
}
function reorderCategory(from, to) {
  if(state.planId || state.resourcesLocked || from<0 || to<0 || to>=state.categories.length || from===to) return;
  const list=[...state.categories];const [category]=list.splice(from,1);list.splice(to,0,category);state.categories=list;
}
function dragCategory(event,category){
  if(state.planId||state.resourcesLocked){event.preventDefault();return;}
  draggedCategory.value=category;event.dataTransfer.effectAllowed='move';event.dataTransfer.setData('text/plain',category);
}
function dropCategory(category){reorderCategory(state.categories.indexOf(draggedCategory.value),state.categories.indexOf(category));draggedCategory.value='';}
function moveAccount(index, delta) {
  const list = accountIds.value.slice();
  [list[index], list[index + delta]] = [list[index + delta], list[index]];
  state.accountOrder = list;
}
const sourceReturn=ref(false);
let opening = null;
function open(source = null) {
  // Initial route recovery can arrive from both the page and its manager.
  // Share the pending choice so one navigation opens one recovery dialog.
  if (opening) return opening;
  opening = openCreation(source).finally(() => { opening = null; });
  return opening;
}
async function openCreation(source = null) {
  sourceReturn.value=!!source?.returnURL;
  promoteBatchDraft(props.db);
  if (props.db.publishDraft?.active && props.db.publishDraft.mode !== "快速发布" && !props.db.publishDraft.batchId) {
    const choice = await chooseDraft({ label: '发布批次', savedAt: props.db.publishDraft.savedAt });
    if (!choice) return;
    if (choice === 'continue') return resume();
    const previous = copy(props.db.publishDraft);
    props.db.publishDraft = { active: false };
    if (!(await props.persist())) {
      props.db.publishDraft = previous;
      feedback.toast.error('无法重新开始，原草稿已保留，请重试。');
      return;
    }
  }
  Object.assign(state, batchCreationDraft(source?.copyBatch), {
    creationKey: crypto.randomUUID(), executionSettings: previewExecutionSettings(),
  });
  if (props.preparationOnly) state.platforms = ["百家号"];

  if(source?.assetIds?.length){
    state.assets=[...source.assetIds];
    state.sourceAssetIds=[...source.assetIds];
    state.platforms=[source.platform];
    state.categories=[...new Set(source.assetIds.map(id=>props.db.assets.find(a=>a.id===id)?.category).filter(Boolean))];
    state.returnURL=source.returnURL;
    state.desiredQuantity=source.assetIds.length;
    state.targets=state.categories.map(category=>({category,count:source.assetIds.filter(id=>props.db.assets.find(a=>a.id===id)?.category===category).length}));
  }
  nameError.value = false;
  errors.value = [];
  visible.value = true;
}
function resume() {
  if (!props.db.publishDraft?.active || props.db.publishDraft.mode === "快速发布" || props.db.publishDraft.batchId) return;
  Object.assign(state, batchCreationDraft(), { executionSettings: previewExecutionSettings() }, copy(props.db.publishDraft), { creationKey: props.db.publishDraft.creationKey || crypto.randomUUID() });
  if (state.step === 1) state.step = 0;
  state.rows.forEach(initCandidate);
  activeCategory.value = state.categories[0] || "";
  errors.value = [];
  visible.value = true;
}
async function close() {
  if (!visible.value) return;
  if (busy.value || contentBusy.value) return;
  if (createdResult.value) {
    if (!resultSaved.value) return;
    visible.value = false;
    await nextTick();
    if(sourceReturn.value){sourceReturn.value=false;emit("return-source");}else emit("closed");
    return;
  }
  if (state.batchId) {
    if (JSON.stringify(state) !== editBaseline.value && !(await feedback.confirmWarning({
      title: "放弃未保存的修改？", message: "本次批次配置尚未保存，返回后将丢失。",
      confirmText: "放弃修改", cancelText: "继续编辑",
    }))) return;
  } else if (state.active) {
    if (!state.name.trim() && !state.platforms.length && !state.categories.length && !state.rows.length && !state.desiredQuantity) {
      state.active = false; props.db.publishDraft = { active: false };
      if (!(await props.persist())) return;
    } else if (!(await saveDraft(true))) return;
  }
  visible.value = false;
  await nextTick();
  if(sourceReturn.value){sourceReturn.value=false;emit("return-source");}else emit("closed");
}
function openBatch(batch) {
  sourceReturn.value=false;
  if (
    batch.phase === "内容预检通过" ||
    batch.firstStartedAt ||
    batch.control === "已取消"
  )
    return;
  const saved = batch.draft;
  Object.assign(state, {
    mode: "批量发布",
    step: props.preparationOnly || saved?.step < 2 ? 0 : saved?.step || 0,
    platforms: [batch.platform],
    categories: batch.targets.filter((t) => t.count > 0).map((t) => t.category),
    assets: saved?.assets || batch.candidates?.map((r) => r.assetId) || [],
    excludedAccounts: saved?.excludedAccounts || [],
    rows: copy(batch.candidates || []),
    name: batch.name,
    planId: batch.planId,
    batchId: batch.id,
    sourceBatchId: batch.sourceBatchId || null,
    sourcePlanId: batch.sourcePlanId || null,
    targets: copy(batch.targets),
    desiredQuantity: saved?.desiredQuantity ?? (batch.planId ? batch.targets.reduce((n,t)=>n+Number(t.count),0) : null),
    sourceAssetIds: [],
    returnURL: null,
    creationKey: saved?.creationKey || crypto.randomUUID(),
    active: true,
    categoryAccountIds: saved?.categoryAccountIds || {},
    accountLimits: saved?.accountLimits || {},
    accountOrder: saved?.accountOrder || [],
    resourcesLocked: !props.preparationOnly && (saved?.resourcesLocked || saved?.step >= 2),
    executionSettings: copy(batch.executionSettings || previewExecutionSettings()),
    taskIntervalSeconds: saved?.taskIntervalSeconds ?? batch.rules?.taskIntervalSeconds ?? 0,
  });
  state.rows.forEach(initCandidate);
  if (batch.needsPreflight) {
    state.step = 0;
    state.resourcesLocked = false;
  }
  activeCategory.value = state.categories[0] || "";
  errors.value = [];
  visible.value = true;
  editBaseline.value = JSON.stringify(state);
}
async function saveDraft(silent = false) {
  if (contentBusy.value || busy.value) return false;
  const batch = state.batchId ? props.db.batches.find(b => b.id === state.batchId) : null;
  if (state.batchId && (!batch || batch.firstStartedAt || batch.control === '已取消' || batch.phase === '内容预检通过')) {
    errors.value = ['当前批次已锁定，不能保存修改。'];
    return false;
  }
  if (batch && (!state.name.trim() || !platform.value || !target.value)) {
    errors.value = ["请填写批次名称、平台和有效目标数量后保存。"];
    return false;
  }
  try { taskInterval(state.taskIntervalSeconds); } catch(e) { errors.value=[e.message];return false; }
  const before = batch ? copy(batch) : null;
  if (batch) {
    if (!batch.planId) Object.assign(batch, { name: state.name, platform: platform.value, targets: copy(state.targets), categories: copy(state.categories) });
    Object.assign(batch, { candidates: copy(state.rows), draft: copy(state), phase: state.step >= 2 ? '待内容配置' : '待资源配置' });
  } else {
    state.savedAt = new Date().toISOString();
    props.db.publishDraft = copy(state);
  }
  busy.value = true;
  const saved = await props.persist();
  busy.value = false;
  if (!saved) {
    if (batch) Object.assign(batch, before);
    errors.value = ['保存失败，当前输入已保留，请重试。'];
    return false;
  }
  editBaseline.value = JSON.stringify(state);
  if (!silent) feedback.toast.success(batch ? '批次配置已保存' : '草稿已保存，不占用成片');
  return true;
}

async function next() {
  errors.value = [];
  if (state.step < 2) {
    nameError.value = !state.name.trim();
    if (nameError.value) {
      await nextTick();
      root.value?.querySelector('[data-field="batch-name"] input')?.focus();
      return;
    }
    if (!platform.value || !state.categories.length) {
      errors.value = ["请选择一个平台和至少一个内容类目。"];
      return;
    }
    if (!props.preparationOnly && !state.rows.length) {
      errors.value = ["当前没有可匹配的账号与成片，请调整类目数量或补充可用资源。"];
      return;
    }
    if (resourceErrors.value.length) {
      errors.value = resourceErrors.value;
      return;
    }
    if (props.preparationOnly) state.step = 2;
    else lockOpen.value = true;
    return;
  }
  if (state.step === 2) {
    reviewPending.value=false;reviewPage.value=1;state.step=3;
  }
}
async function confirmResources() {
  if (resourceErrors.value.length) {
    lockOpen.value = false;
    errors.value = resourceErrors.value;
    return;
  }
  state.resourcesLocked = true;
  state.step = 2;
  lockOpen.value = false;
  busy.value = true;
  try {
    for (const r of state.rows) await ensureCoverResults(r);
  } catch (e) {
    errors.value = [`资源已确认；${e.message}。可在内容页修复封面。`];
  } finally {
    busy.value = false;
  }
}
function back() {
  errors.value = [];
  if (state.step === 3) state.step = 2;
  else if (props.preparationOnly && state.step === 2) state.step = 0;
}
function rematch() {
  recommendAssets();
  state.rows = copy(suggested.value);
  state.rows.forEach(initCandidate);
  errors.value = [];
  feedback.toast.success("已重新匹配，兼容的人工分配与内容已保留");
}
function checkResources() {
  errors.value = resourceErrors.value;
  if (!errors.value.length)
    feedback.toast.success(
      `${state.rows.length} 条资源组合有效，内容将在下一阶段配置`,
    );
}
function updateRow(row) {
  const i = state.rows.findIndex((r) => r.key === row.key);
  if (i >= 0) state.rows[i] = copy(row);
}
function openReplace(row) {
  replaceRow.value = row;
  replacement.value = row.assetId;
  replaceSearch.value = "";
}
const replacementOptions = computed(() =>
  availableAssets.value.filter(
    (v) =>
      v.category === replaceRow.value?.category &&
      (!state.rows.some((r) => r.assetId === v.id) ||
        v.id === replaceRow.value?.assetId) &&
      `${v.title} ${v.id}`.includes(replaceSearch.value),
  ),
);
function applyReplacement() {
  const old = replaceRow.value,
    v = asset(replacement.value);
  if (!old || !v) return;
  if (v.id !== old.assetId) {
    const replacementRow = suggestDistribution(
      props.db,
      [platform.value],
      [v.id],
      [old.accountId],
    )[0];
    if (!replacementRow) {
      errors.value = ["所选账号无法承接该成片，请先更换账号。"];
      return;
    }
    const i = state.rows.indexOf(old);
    initCandidate(replacementRow);
    if (props.preparationOnly) Object.assign(replacementRow, { key: old.key, candidateId: old.candidateId });
    replacementRow.overrides = ["assetId"];
    state.rows[i] = replacementRow;
    state.assets = state.rows.map((r) => r.assetId);
  }
  replaceRow.value = null;
}
async function savePreparation() {
  busy.value = true;
  const index = props.db.batches.findIndex(batch => batch.id === state.batchId);
  const previous = props.db.batches[index];
  const localDraft = props.db.publishDraft;
  let record;
  try {
    record = preparationRecord(state, { id: previous?.id || nextId(props.db), actor: currentPrototypeUser(), existing: previous });
    if (previous) props.db.batches.splice(index, 1, record);
    else {
      props.db.batches.unshift(record);
      props.db.publishDraft = { active: false };
    }
    if (!(await props.persist())) throw Error("保存失败，当前输入已保留。");
    visible.value = false;
    await nextTick();
    emit("created", { batch: record, tasks: [] });
  } catch (error) {
    if (record) {
      if (previous) props.db.batches.splice(index, 1, previous);
      else props.db.batches.splice(0, 1);
      props.db.publishDraft = localDraft;
    }
    errors.value = [error.message];
  } finally {
    busy.value = false;
  }
}
async function create() {
  if (busy.value || createdResult.value) return;
  if (props.preparationOnly) return savePreparation();
  errors.value = [];
  if (!state.rows.length || issues.value.length) {
    errors.value = [
      `还有 ${issues.value.length} 条任务需处理，请返回内容配置修复，或明确移出问题项。`,
    ];
    return;
  }
  if (!state.name.trim()) {
    errors.value = ["请填写批次名称。"];
    return;
  }
  let partial = false;
  if (gap.value) {
    partial = await feedback.confirmWarning({
      title: "确认部分创建",
      message: `原始目标 ${target.value} 条，实际创建 ${state.rows.length} 条，缺口 ${gap.value} 条。目标保持不变，不自动补量。`,
      confirmText: `保留缺口并创建 ${state.rows.length} 条`,
    });
    if (!partial) return;
  }
  busy.value = true;
  try {
    const result = createPublication(props.db, {
      platform: platform.value,
      rows: state.rows,
      name: state.name,
      planId: state.planId,
      quick: false,
      batchId: state.batchId,
      sourceBatchId: state.sourceBatchId,
      sourcePlanId: state.sourcePlanId,
      targets: state.targets,
      partial,
      creationKey: state.creationKey,
      taskIntervalSeconds: state.taskIntervalSeconds,
    });
    state.active = false;
    if (!state.batchId) props.db.publishDraft = { active: false };
    state.rows.forEach((row, index) => { row.id = result.tasks[index].id; });
    createdResult.value = result;
    resultSaved.value = false;
    resultSaved.value = await props.persist();
    if (!resultSaved.value)
      errors.value = [
        "任务已生成，但创建结果尚未保存。请重试保存，不要刷新或重复创建。",
      ];
  } catch (e) {
    errors.value = [e.message];
  } finally {
    busy.value = false;
  }
}
async function removeRow(row) {
  if (state.resourcesLocked) {
    errors.value = ["资源已锁定，请创建任务后取消或放弃，不可移出或替换资源。"];
    return;
  }
  if (
    await feedback.confirmWarning({
      title: "移出这条候选任务？",
      message: `不会创建此任务；原类目目标保留，缺口将增加 1 条。`,
      confirmText: "移出并保留缺口",
    })
  )
    state.rows = state.rows.filter((r) => r.key !== row.key);
}
async function editContent(row) {
  state.step = 2;
  await nextTick();
  contentRef.value?.edit(row);
}
async function retrySaveCreated() {
  if (busy.value || !createdResult.value) return;
  busy.value = true;
  try {
    resultSaved.value = await props.persist();
    if (resultSaved.value) errors.value = [];
  } finally { busy.value = false; }
}
function finish() {
  if (!resultSaved.value) return;
  visible.value = false;
  emit("created", createdResult.value);
}
defineExpose({
  open,
  openBatch,
  resume,
  updateRow,
  hasDraft: computed(() => props.db.publishDraft?.active && props.db.publishDraft.mode !== "快速发布" && !props.db.publishDraft.batchId),
  draftMode: computed(() => state.mode),
});
</script>

<template>
  <ResourceOverlay
    :visible="visible"
    @update:visible="value => { if (!value) close(); }"
    :title="state.batchId ? '编辑发布批次' : '新建发布批次'"
    page
    :back-label="sourceReturn ? '返回成片管理' : '返回发布批次列表'"
    kind="drawer"
    :width="1440"
    class="batch-v2-overlay"
    :busy="busy || contentBusy || (!!createdResult && !resultSaved)"
  >
    <div ref="root" class="batch-create">
      <ResourceWizardSteps v-if="state.step < 4" class="batch-stagebar" :current="stage" :steps="stages.map(([title])=>({title}))" />
      <t-alert
        v-if="errors.length"
        theme="warning"
        :message="errors.join('；')"
        close-btn
        @close="errors = []"
        class="batch-message"
      />
      <div style="display: contents" :inert="!!createdResult">
      <t-alert
        v-if="softConflicts.length"
        theme="warning"
        :message="`同平台草稿「${softConflicts.map((b) => b.name).join('、')}」也选择了这些成片。草稿不占用，最终创建会重新检查。`"
        class="batch-message"
      />
      <template v-if="state.step === 0">
        <section class="batch-section" data-section="batch-scope">
          <header class="batch-section-head"><div><span class="batch-number">01</span><h3>发布范围 <ResourceFieldHelp label="发布范围" content="选择单个平台与参与分配的类目。" /></h3></div></header>
          <div class="batch-section-body">
            <div class="batch-form-grid batch-scope-grid">
              <label data-field="batch-name"><span><em>*</em> 批次名称</span><t-input v-model="state.name" :status="nameError ? 'error' : undefined" placeholder="请输入批次名称" :maxlength="60" /><small v-if="nameError" class="batch-field-error" role="alert">请输入批次名称</small></label>
              <label><span><em>*</em> 发布平台</span><t-select :popup-props="{ attach: 'body', zIndex: 5500 }" :value="platform" :options="allowedPlatforms.map(p => ({label:p,value:p}))" placeholder="选择一个平台" :disabled="!!state.planId" @change="changePlatform" aria-label="批次发布平台" /></label>
            </div>
            <div class="batch-category-field"><label><span><em>*</em> 发布类目</span></label><t-checkbox-group v-model="state.categories" :disabled="!!state.planId"><t-checkbox v-for="c in allowedCategories" :key="c" :value="c" class="batch-category-chip">{{ c }}</t-checkbox></t-checkbox-group></div>
          </div>
        </section>
        <section v-if="platform && state.categories.length" class="batch-section">
          <header class="batch-section-head">
            <div>
              <span class="batch-number">02</span>
              <h3>类目数量与优先级 <ResourceFieldHelp label="类目数量与优先级" content="按目标数量与可匹配资源推荐；类目数量可自由调整，无需与目标数量相等。上方类目优先匹配。" /></h3>
            </div>
          </header>
          <div class="batch-section-body">
            <div class="batch-goal-row">
              <label class="batch-goal-field"><span>目标数量 <ResourceFieldHelp label="目标数量" content="用于推荐类目数量与提示差额。未填写、与类目合计不一致或资源不足以达到目标，均可继续配置。" /></span><t-input-number v-model="state.desiredQuantity" :min="1" :max="9999" :decimal-places="0" clearable :disabled="!!state.planId" placeholder="请输入目标数量" aria-label="目标数量" /></label>
            <t-button
              variant="outline"
              :disabled="!!state.planId"
              @click="applyRecommendations"
              ><template #icon><RefreshIcon /></template>采用系统推荐</t-button
            >
            </div>
            <div class="batch-total-line">
              <div>
                <small>类目数量合计</small
                ><strong>{{ target }} <span>条任务</span></strong>
              </div>
              <p>
                <template v-if="state.planId">目标来自计划</template>
                <template v-else-if="state.desiredQuantity > 0">
                  目标 {{ state.desiredQuantity }} 条 ·
                  {{ target === Number(state.desiredQuantity) ? '已按目标分配' : target < state.desiredQuantity ? `比目标少 ${state.desiredQuantity - target} 条` : `超出目标 ${target - state.desiredQuantity} 条` }}
                </template>
              </p>
            </div>
            <div class="resource-data-table resource-embedded-table is-editable batch-target-container" role="region" aria-label="类目目标分配，可横向滚动" tabindex="0"><table class="batch-table batch-target-table">
              <thead>
                <tr>
                  <th>优先级</th>
                  <th>内容类目</th>
                  <th class="resource-table-number">可用账号</th>
                  <th class="resource-table-number">可用成片</th>
                  <th class="resource-table-number">本批次目标</th>
                  <th class="resource-table-number">预计分配</th>
                  <th class="resource-table-number">缺口</th>

                </tr>
              </thead>
              <tbody>
                <tr v-for="(r, i) in categoryRows" :key="r.category" :class="{'is-dragging':draggedCategory===r.category}" @dragover.prevent @drop.prevent="dropCategory(r.category)">
                  <td>
                    <button class="batch-drag-handle" type="button" :draggable="!state.planId && !state.resourcesLocked" :disabled="!!state.planId || state.resourcesLocked" :aria-label="`调整${r.category}优先级`" title="拖拽排序，或使用上下方向键" @dragstart="dragCategory($event,r.category)" @dragend="draggedCategory=''" @keydown.up.prevent="reorderCategory(i,i-1)" @keydown.down.prevent="reorderCategory(i,i+1)"><MoveIcon /></button><span class="batch-priority">P{{ i + 1 }}</span>
                  </td>
                  <td>
                    <b>{{ r.category }}</b>
                  </td>
                  <td class="resource-table-number"><t-button variant="text" :aria-label="`选择${r.category}发布账号`" @click="selectCategoryAccounts(r.category)">{{ r.accounts }} 个</t-button></td>
                  <td class="resource-table-number"><t-button variant="text" theme="primary" :aria-label="`查看${r.category}可用成片`" @click="availableCategory=r.category">{{r.available}} 条</t-button></td>
                  <td class="resource-table-number">
                    <t-input-number
                      :value="r.target"
                      :min="0"
                      :max="9999"
                      :decimal-places="0"
                      :disabled="!!state.planId"
                      :aria-label="r.category + '批次目标'"
                      @change="
                        (v) => {
                          setTarget(r.category, v);
                          recommendAssets();
                        }
                      "
                    />
                  </td>
                  <td class="resource-table-number">{{ r.allocated }} 条</td>
                  <td class="resource-table-number" :class="{ 'batch-warning-text': r.target > r.allocated }">
                    {{ Math.max(0, r.target - r.allocated) }} 条
                  </td>

                </tr>
              </tbody>
            </table></div>
            <ResourceNotice v-if="gap" tone="warning">
              目标 {{ target }} 条，可匹配 {{ suggested.length }} 条，缺口 {{ gap }} 条。{{ state.planId ? '可调整账号选择，或补充可用资源。' : '可调整账号选择、数量或补充可用资源。' }}
            </ResourceNotice>
          </div>
        </section>
        <section v-if="platform && state.categories.length" ref="matchingSection" class="batch-section" data-section="batch-matching">
          <header class="batch-section-head">
            <div>
              <span class="batch-number">03</span>
              <h3>账号与成片匹配 <ResourceFieldHelp label="账号与成片匹配" content="系统自动匹配，可人工调整后确认资源。" /></h3>
            </div>
            <div class="batch-head-actions">
              <t-button variant="outline" @click="rematch"
                ><template #icon><RefreshIcon /></template
                >重新自动匹配</t-button
              ><t-button variant="outline" @click="checkResources"
                >检查资源冲突</t-button
              >
            </div>
          </header>
          <div class="batch-section-body">
            <div class="batch-match-summary">
              <t-tag :theme="gap ? 'warning' : 'success'" variant="light"
                >已匹配 {{ state.rows.length }} / {{ target }}</t-tag
              ><span
                >账号 {{ new Set(state.rows.map((r) => r.accountId)).size }} 个
                · 成片 {{ state.rows.length }} 条</span
              ><span v-if="gap" class="batch-warning-text"
                >目标缺口 {{ gap }} 条</span
              >
            </div>
            <t-tabs v-model="activeCategory"
              ><t-tab-panel
                v-for="c in state.categories"
                :key="c"
                :value="c"
                :label="`${c}（${state.rows.filter((r) => r.category === c).length}）`"
            /></t-tabs>
            <div class="batch-table-scroll batch-matching-container resource-data-table resource-embedded-table is-editable">
              <table class="batch-table batch-matching-table">
                <thead>
                  <tr>
                    <th>序号</th>
                    <th>发布账号</th>
                    <th>成片资源</th>
                    <th>匹配来源</th>
                    <th>资源检查</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(r, i) in matchingRows" :key="r.key">
                    <td>{{ i + 1 }}</td>
                    <td>
                      <t-select
                        :popup-props="{ attach: 'body', zIndex: 5500 }"
                        v-model="r.accountId"
                        :options="
                          eligibleAccounts
                            .filter(
                              (a) =>
                                accountsForCategory(r.category).includes(a.id) &&
                                a.categories.includes(r.category),
                            )
                            .map((a) => ({
                              label: `${a.name} · ${a.quality}`,
                              value: a.id,
                            }))
                        "
                        :aria-label="`第${i + 1}条发布账号`"
                        @change="
                          r.overrides = [
                            ...new Set([...(r.overrides || []), 'accountId']),
                          ]
                        "
                      />
                    </td>
                    <td>
                      <div class="batch-video-cell">
                        <img :src="asset(r.assetId)?.cover" alt="成片封面" />
                        <div>
                          <b>{{ asset(r.assetId)?.title }}</b
                          ><small
                            >V-{{ r.assetId }} ·
                            {{ asset(r.assetId)?.duration }} ·
                            {{ asset(r.assetId)?.quality }}</small
                          >
                        </div>
                      </div>
                    </td>
                    <td>
                      <t-tag
                        variant="light"
                        :theme="r.overrides?.length ? 'primary' : 'default'"
                        >{{
                          r.overrides?.length ? "人工指定" : "系统匹配"
                        }}</t-tag
                      >
                    </td>
                    <td>
                      <t-tooltip
                        v-if="resourceProblems(r).length"
                        :content="resourceProblems(r).join('；')"
                        ><t-tag theme="warning" variant="light"
                          >需调整</t-tag
                        ></t-tooltip
                      ><t-tag v-else theme="success" variant="light"
                        >有效</t-tag
                      >
                    </td>
                    <td>
                      <ResourceRowActions><t-button variant="text" @click="openReplace(r)"
                        >更换成片</t-button
                      ><t-button
                        variant="text"
                        theme="danger"
                        @click="removeRow(r)"
                        >移出</t-button
                      >
                    </ResourceRowActions>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <p v-if="!matchingRows.length" class="batch-empty">
              此类目尚无可分配资源。可在上方调整数量与账号，或补充成片。
            </p>

          </div>
        </section>
        <div v-if="platform && state.categories.length && missingResources" class="batch-shortage"><span>{{ missingResources }}。可调整发布范围或补充资源。</span></div>
      </template>
      <BatchContentWorkspace
        v-else-if="state.step === 2"
        ref="contentRef"
        :rows="state.rows"
        :db="db"
        @preview="emit('preview', $event)"
        @working="contentBusy = $event"
      />
      <template v-else-if="state.step === 3">
        <ResourceNotice v-if="issues.length" tone="warning" class="batch-review-notice">{{issues.length}} 条内容待处理<template #actions><t-button variant="text" theme="primary" size="small" @click="showReviewPending">查看待处理</t-button></template></ResourceNotice>
        <ResourceCard class="detail-card batch-review-card"><ResourceSection :title="preparationOnly ? '准备结果' : '创建检查'">
          <template #title-extra><span class="batch-review-ready">已就绪 {{ready}} / {{state.rows.length}} 条</span></template>
          <div class="batch-review-summary"><label><span>批次名称 <em>*</em></span><t-input v-model="state.name" placeholder="请输入批次名称" :maxlength="60" /></label><div><span>发布平台</span><strong>{{platform}}</strong></div><div><span>{{ preparationOnly ? "候选配置" : "本次创建" }}</span><strong>{{state.rows.length}} 条</strong></div><div v-if="gap" class="batch-warning-text"><span>目标缺口</span><strong>{{gap}} 条</strong></div></div>
        </ResourceSection></ResourceCard>
        <ResourceCard class="detail-card batch-review-card"><ResourceSection title="发布设置">
          <div v-if="preparationOnly" class="batch-settings-grid">
            <label><span>并发数</span><t-input-number v-model="state.executionSettings.concurrency" :min="1" :decimal-places="0" aria-label="并发数" /></label>
            <label><span>组间发布间隔</span><t-input-number v-model="state.executionSettings.groupInterval" :min="0" :decimal-places="0" suffix="秒" aria-label="组间发布间隔" /></label>
            <label><span>组间处理</span><t-select v-model="state.executionSettings.groupPolicy" :options="previewGroupPolicies.map(label => ({ label, value: label }))" aria-label="组间处理" /></label>
            <label><span>异常任务上限<ResourceFieldHelp label="异常任务上限" content="异常任务达到该数量时，暂停发布，人工决定继续或终止。" /></span><t-input-number v-model="state.executionSettings.exceptionLimit" :min="1" :decimal-places="0" aria-label="异常任务上限" /></label>
          </div>
          <div v-else class="batch-settings-grid"><label><span>发布方式</span><t-input :value="publishMethodLabel" disabled /></label><label><span>任务间隔</span><t-input-number v-model="state.taskIntervalSeconds" :min="0" :decimal-places="0" suffix="秒" aria-label="任务间隔" /></label></div>
        </ResourceSection></ResourceCard>
        <ResourceCard class="detail-card batch-review-card batch-review-table"><ResourceSection title="发布内容">
          <template #title-extra><span class="batch-review-ready">{{reviewRows.length}} 条</span><t-button v-if="reviewPending" variant="text" theme="primary" size="small" @click="reviewPending=false;reviewPage=1">查看全部</t-button></template>
          <BatchContentTable :rows="reviewRows" :db="db" :pagination="{current:reviewPage,pageSize:reviewPageSize,pageSizeOptions:[10,20,50],total:reviewRows.length,showJumper:false}" @page-change="value=>{reviewPage=value.current;reviewPageSize=value.pageSize}" @edit="editContent" @preview="emit('preview',$event)" />
        </ResourceSection></ResourceCard>
      </template>
    </div>
    </div>
    <template #footer
      ><template v-if="createdResult"
        ><span class="batch-footer-note">{{ resultSaved ? "已保存为待执行" : "任务已生成，请重试保存" }}</span
        ><t-button v-if="!resultSaved" :loading="busy" @click="retrySaveCreated">重试保存</t-button
        ></template
      ><template v-else
        ><span class="batch-footer-note">{{
          state.step < 2
            ? `类目合计 ${target} · 已匹配 ${allocated.length} · 缺口 ${gap}`
            : state.step === 3 ? (preparationOnly ? "保存候选配置，不生成正式发布任务" : "创建后待执行，不会立即发布") : `已就绪 ${ready} / ${state.rows.length} 条`
        }}</span
        ><t-button
          v-if="!preparationOnly || !state.batchId"
          variant="outline"
          :loading="busy"
          :disabled="contentBusy"
          @click="saveDraft(false)"
          >{{ state.batchId ? "保存" : "保存草稿" }}</t-button
        ><t-button
          v-if="
            state.step === 3 || (preparationOnly && state.step === 2)
          "
          variant="outline"
          @click="back"
          >{{ preparationOnly && state.step === 2 ? "返回资源配置" : "返回内容配置" }}</t-button
        ><t-button
          v-if="state.step < 3"
          :loading="busy || contentBusy"
          :disabled="
            contentBusy ||
            (!preparationOnly && (state.step === 0
              ? !platform || !state.categories.length || !state.rows.length
              : !state.rows.length))
          "
          @click="next"
          >{{
            preparationOnly ? "下一步" : state.step < 2 ? "确认资源" : "预览检查"
          }}</t-button
        ><t-button
          v-else
          :loading="busy || contentBusy"
          :disabled="contentBusy || (!preparationOnly && (!state.rows.length || !!issues.length))"
          @click="create"
          >{{ preparationOnly ? (state.batchId ? "保存修改" : "保存批次") : "创建批次" }}</t-button
        ></template
      ></template
    >
  </ResourceOverlay>
  <ResourceOverlay :visible="visible && resultSaved" title="批次与任务已创建" :width="560" :busy="busy" @update:visible="value => { if (!value) close(); }">
    <div class="batch-created-summary">
      <div class="batch-created-check"><CheckIcon /></div>
      <strong>{{ state.name }}</strong>
      <p>{{ createdResult?.tasks.length || 0 }} 条任务待执行</p>
      <p v-if="target > (createdResult?.tasks.length || 0)" class="batch-warning-text">目标 {{ target }} 条，缺口 {{ target - (createdResult?.tasks.length || 0) }} 条</p>
    </div>
    <template #footer><t-button variant="outline" @click="close">返回</t-button><t-button @click="finish">查看批次</t-button></template>
  </ResourceOverlay>
  <ResourceOverlay :visible="!!accountSelection" @update:visible="value => { if (!value) accountSelection = null; }" :title="`选择发布账号 · ${accountSelection?.category || ''}`" :width="760">
    <template v-if="accountSelection">
      <div class="batch-account-dialog-head"><span>已选 {{ accountSelection.ids.length }} 个账号</span><t-button variant="text" @click="accountSelection.ids = eligibleAccounts.filter(a => a.categories.includes(accountSelection.category)).map(a => a.id)">全选</t-button><t-button variant="text" @click="accountSelection.ids = []">清空</t-button></div>
      <div class="batch-table-scroll resource-data-table resource-embedded-table is-editable is-flush"><table class="batch-table batch-account-selector-table"><thead><tr><th>发布账号</th><th>适用类目</th><th>本批次上限 <ResourceFieldHelp label="本批次上限" content="同一账号跨类目共用此上限。留空不设置本批次上限，不代表平台额度。" /></th></tr></thead><tbody>
        <tr v-for="a in eligibleAccounts.filter(a => a.categories.includes(accountSelection.category))" :key="a.id"><td><t-checkbox :checked="accountSelection.ids.includes(a.id)" @change="checked => accountSelection.ids = checked ? [...new Set([...accountSelection.ids, a.id])] : accountSelection.ids.filter(id => id !== a.id)">{{ a.name }}</t-checkbox></td><td>{{ a.categories.join(' / ') }}</td><td><t-input-number v-model="accountSelection.limits[a.id]" :min="1" :decimal-places="0" clearable placeholder="不限制" :aria-label="`${a.name}本批次上限`" /></td></tr>
        <tr v-if="!eligibleAccounts.some(a => a.categories.includes(accountSelection.category))"><td colspan="3" class="batch-empty">暂无符合当前平台与类目的可用账号</td></tr>
      </tbody></table></div>
    </template>
    <template #footer><t-button variant="outline" @click="accountSelection = null">取消</t-button><t-button @click="applyCategoryAccounts">确定选择</t-button></template>
  </ResourceOverlay>
  <ResourceOverlay v-model:visible="lockOpen" title="确认资源" :width="560">
    <div class="batch-resource-confirm"><strong>已匹配 {{state.rows.length}} 条任务</strong><p>确认后，发布范围及账号、成片分配将锁定，文案与封面仍可编辑。</p><p v-if="gap" class="batch-warning-text">目标缺口 {{gap}} 条</p></div>
    <template #footer><t-button variant="outline" theme="default" @click="lockOpen=false">继续调整</t-button><t-button @click="confirmResources">进入内容配置</t-button></template>
  </ResourceOverlay>
  <ResourceOverlay :visible="!!availableCategory" @update:visible="value=>{if(!value)availableCategory=''}" :title="`${availableCategory} · 可用成片`" :width="960">
    <t-table class="resource-data-table resource-embedded-table is-flush batch-available-assets" row-key="id" :data="categoryAssets" :columns="[{colKey:'title',title:'发布成片',width:320},{colKey:'id',title:'成片编号',width:120},{colKey:'duration',title:'时长',width:100},{colKey:'quality',title:'质量',width:100}]" :pagination="{pageSize:10,pageSizeOptions:[10,20,50],total:categoryAssets.length,showJumper:false}">
      <template #title="{row}"><t-button variant="text" theme="primary" @click="emit('preview',{asset:row})">{{row.title}}</t-button></template>
    </t-table>
  </ResourceOverlay>
  <ResourceOverlay
    :visible="!!replaceRow"
    @update:visible="
      (v) => {
        if (!v) replaceRow = null;
      }
    "
    title="更换成片"
    :width="800"
    ><p class="batch-dialog-copy">
      仅显示当前类目的可用、未重复分配成片。更换后需在下一阶段重新配置内容。
    </p>
    <t-input v-model="replaceSearch" placeholder="搜索可用成片" />
    <div class="batch-replacement-list">
      <label v-for="v in replacementOptions" :key="v.id"
        ><input type="radio" v-model="replacement" :value="v.id" /><img
          :src="v.cover"
          alt=""
        /><span
          ><b>{{ v.title }}</b
          ><small
            >V-{{ v.id }} · {{ v.duration }} · {{ v.quality }}</small
          ></span
        ></label
      >
      <p v-if="!replacementOptions.length" class="batch-empty">
        没有其他可用成片。可返回补充资源或移出当前候选。
      </p>
    </div>
    <template #footer
      ><t-button variant="outline" @click="replaceRow = null">取消</t-button
      ><t-button :disabled="!replacement" @click="applyReplacement"
        >确认更换</t-button
      ></template
    ></ResourceOverlay
  >
</template>
