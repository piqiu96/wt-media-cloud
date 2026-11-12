<script setup>
import { computed, reactive, ref, watch, nextTick } from "vue";
import {
  ArrowLeftIcon,
  ArrowRightIcon,
  CheckIcon,
  RefreshIcon,
} from "tdesign-icons-vue-next";
import ResourceOverlay from "./PrototypeOverlay.vue";
import BatchContentWorkspace from "./BatchContentWorkspace.vue";
import { initCandidate } from "./content.js";
import { createPublication, nextId } from "./relationships.js";
import { ensureCoverResults } from "./covers.js";
import { feedback } from "../../../src/shared/ui/feedback/index.js";
import {
  platforms,
  categories,
  validate,
  resourceStatus,
  resourceInventory,
  suggestDistribution,
} from "./model.js";
import "./batchCreation.css";
const props = defineProps({
  db: { type: Object, required: true },
  persist: { type: Function, default: async () => true },
});
const emit = defineEmits(["edit", "preview", "created"]);
const copy = (value) => JSON.parse(JSON.stringify(value));
const visible = ref(false),
  busy = ref(false),
  contentBusy = ref(false),
  resultSaved = ref(false),
  errors = ref([]),
  resourcePane = ref(0),
  activeCategory = ref(""),
  contentRef = ref(null);
const inventoryExpanded = ref(false),
  root = ref(null),
  createdResult = ref(null);
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
  creationKey: null,
  active: false,
  accountLimits: {},
  accountOrder: [],
  resourcesLocked: false,
});
if (props.db.publishDraft) Object.assign(state, copy(props.db.publishDraft));
watch(
  () => [state.step, resourcePane.value],
  async () => {
    await nextTick();
    root.value?.closest(".t-drawer__body")?.scrollTo({ top: 0 });
  },
);

watch(
  state,
  () => {
    props.db.publishDraft = copy(state);
    const batch = props.db.batches.find((b) => b.id === state.batchId);
    if (batch && batch.phase !== "内容预检通过") {
      if (!batch.planId)
        Object.assign(batch, {
          platform: state.platforms[0],
          name: state.name,
          targets: copy(state.targets),
          categories: copy(state.categories),
        });
      batch.candidates = copy(state.rows);
      batch.draft = copy(state);
      batch.phase = state.step >= 2 ? "待内容配置" : "待资源配置";
    }
  },
  { deep: true },
);
const platform = computed(() => state.platforms[0]);
const plan = computed(() => props.db.plans.find((p) => p.id === state.planId));
const allowedPlatforms = computed(() =>
  plan.value
    ? [...new Set(plan.value.targets.map((t) => t.platform))]
    : platforms,
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
    : categories,
);
const asset = (id) => props.db.assets.find((v) => v.id === id);
const account = (id) => props.db.accounts.find((a) => a.id === id);
const eligibleAccounts = computed(() =>
  props.db.accounts
    .filter(
      (a) =>
        a.enabled &&
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
const accountIds = computed(() =>
  eligibleAccounts.value
    .filter((a) => !state.excludedAccounts.includes(a.id))
    .map((a) => a.id),
);
const availableAssets = computed(() =>
  props.db.assets.filter(
    (v) =>
      state.categories.includes(v.category) &&
      !resourceStatus(props.db, v, platform.value, accountIds.value),
  ),
);
const inventory = computed(() =>
  resourceInventory(
    props.db,
    state.platforms,
    state.categories,
    accountIds.value,
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
  ),
);
const allocated = computed(() =>
  state.step === 0 ? suggested.value : state.rows,
);
const gap = computed(() => Math.max(0, target.value - allocated.value.length));
const issues = computed(() =>
  state.rows.filter((r) => validate(r, props.db).length),
);
const ready = computed(() => state.rows.length - issues.value.length);
const stage = computed(() => (state.step < 2 ? 0 : state.step - 1));
const stages = [
  ["资源配置", "选择范围 · 数量 · 匹配"],
  ["内容配置", "批量生成与精修内容"],
  ["结果总览", "预览实际内容 · 确认创建"],
];
const categoryRows = computed(() =>
  state.categories.map((category) => ({
    category,
    target: Number(
      state.targets.find((t) => t.category === category)?.count || 0,
    ),
    available: availableAssets.value.filter((v) => v.category === category)
      .length,
    accounts: eligibleAccounts.value.filter(
      (a) => accountIds.value.includes(a.id) && a.categories.includes(category),
    ).length,
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
    !accountIds.value.includes(a.id)
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
    if (!list.includes(activeCategory.value))
      activeCategory.value = list[0] || "";
  },
);
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
  state.accountLimits = {};
  state.accountOrder = [];
  errors.value = [];
}
function setTarget(category, value) {
  const t = state.targets.find((t) => t.category === category);
  if (t) t.count = Math.max(0, Math.floor(Number(value) || 0));
}
function recommendTargets() {
  for (const c of categoryRows.value)
    setTarget(c.category, Math.min(c.available, 3));
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
function moveCategory(index, delta) {
  const list = [...state.categories];
  [list[index], list[index + delta]] = [list[index + delta], list[index]];
  state.categories = list;
}
function moveAccount(index, delta) {
  const list = accountIds.value.slice();
  [list[index], list[index + delta]] = [list[index + delta], list[index]];
  state.accountOrder = list;
}
async function open() {
  if (
    state.active &&
    (state.rows.length || state.assets.length) &&
    !(await feedback.confirmWarning({
      title: "开始新的批次配置？",
      message: "可取消后继续上次配置；开始新配置会替换尚未保存的输入。",
      confirmText: "开始新配置",
    }))
  )
    return;
  Object.assign(state, {
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
    creationKey: crypto.randomUUID(),
    active: true,
    accountLimits: {},
    accountOrder: [],
    resourcesLocked: false,
  });
  resourcePane.value = 0;
  errors.value = [];
  visible.value = true;
}
function openBatch(batch) {
  if (
    batch.phase === "内容预检通过" ||
    batch.firstStartedAt ||
    batch.control === "已取消"
  )
    return;
  const saved = batch.draft;
  Object.assign(state, {
    mode: "批量发布",
    step: saved?.step || 0,
    platforms: [batch.platform],
    categories: batch.targets.filter((t) => t.count > 0).map((t) => t.category),
    assets: saved?.assets || batch.candidates?.map((r) => r.assetId) || [],
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
    resourcesLocked: saved?.resourcesLocked || saved?.step >= 2,
  });
  state.rows.forEach(initCandidate);
  if (batch.needsPreflight) {
    state.step = 0;
    state.resourcesLocked = false;
  }
  resourcePane.value = state.categories.length ? 1 : 0;
  activeCategory.value = state.categories[0] || "";
  errors.value = [];
  visible.value = true;
}
async function saveDraft(silent = false) {
  if (contentBusy.value || busy.value) return false;
  if (!platform.value || !target.value) {
    errors.value = ["请选择平台，并设置至少一个类目的目标数量后保存。"];
    return false;
  }
  if (!state.name.trim())
    state.name = `${platform.value}发布批次 · ${new Date().toLocaleDateString("zh-CN")}`;
  let batch = props.db.batches.find((b) => b.id === state.batchId);
  if (!batch) {
    batch = {
      id: nextId(props.db),
      name: state.name,
      platform: platform.value,
      planId: state.planId,
      targets: copy(state.targets),
      categories: copy(state.categories),
      phase: "待资源配置",
      control: "待配置",
      candidates: [],
    };
    props.db.batches.push(batch);
    state.batchId = batch.id;
  }
  Object.assign(batch, {
    candidates: copy(state.rows),
    draft: copy(state),
    phase: state.step >= 2 ? "待内容配置" : "待资源配置",
  });
  busy.value = true;
  await nextTick();
  const saved = await props.persist();
  busy.value = false;
  if (!saved) {
    errors.value = [
      "草稿尚未保存，当前输入已保留，请重试。保存成功前请勿刷新。",
    ];
    return false;
  }
  if (!silent) feedback.toast.success("草稿已保存，尚未创建任务或占用成片");
  return true;
}
async function next() {
  errors.value = [];
  if (state.step === 0) {
    if (!platform.value || !state.categories.length) {
      errors.value = ["请选择一个平台和至少一个内容类目。"];
      return;
    }
    if (resourcePane.value === 0) {
      resourcePane.value = 1;
      if (!target.value) recommendTargets();
      return;
    }
    if (!target.value) {
      errors.value = ["请设置至少一条任务目标。"];
      return;
    }
    recommendAssets();
    if (!suggested.value.length) {
      errors.value = [
        "当前没有可匹配的账号与成片。请查看资源缺口，调整账号池或补充成片。",
      ];
      return;
    }
    state.rows = copy(suggested.value);
    state.rows.forEach(initCandidate);
    activeCategory.value = state.categories[0];
    state.step = 1;
    return;
  }
  if (state.step === 1) {
    if (resourceErrors.value.length) {
      errors.value = resourceErrors.value;
      return;
    }
    lockOpen.value = true;
    return;
  }
  if (state.step === 2) {
    busy.value = true;
    try {
      for (const r of state.rows) await ensureCoverResults(r);
      state.step = 3;
    } catch (e) {
      errors.value = [e.message || "封面生成失败，请返回内容配置重试。"];
    } finally {
      busy.value = false;
    }
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
  else if (state.step === 1) {
    state.step = 0;
    resourcePane.value = 1;
  } else resourcePane.value = 0;
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
    replacementRow.overrides = ["assetId"];
    state.rows[i] = replacementRow;
    state.assets = state.rows.map((r) => r.assetId);
  }
  replaceRow.value = null;
}
async function create() {
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
      targets: state.targets,
      partial,
      creationKey: state.creationKey,
    });
    state.active = false;
    createdResult.value = result;
    resultSaved.value = false;
    state.step = 4;
    await nextTick();
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
  busy.value = true;
  resultSaved.value = await props.persist();
  busy.value = false;
  if (resultSaved.value) errors.value = [];
}
function finish() {
  if (!resultSaved.value) return;
  visible.value = false;
  emit("created", createdResult.value);
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
    v-model:visible="visible"
    title="新建发布批次"
    workspace
    kind="drawer"
    :width="1440"
    class="batch-v2-overlay"
    :busy="busy || contentBusy"
  >
    <div ref="root" class="batch-create">
      <header v-if="state.step < 4" class="batch-create-intro">
        <div>
          <h2>
            {{ state.name || "新建批量发布" }}
            <t-tag size="small" variant="light">草稿</t-tag>
          </h2>
          <p>
            {{ platform || "选择一个发布平台"
            }}<template v-if="state.categories.length">
              · {{ state.categories.join(" / ") }}</template
            >
            · {{ state.planId ? "目标继承自发布计划" : "批次资源准备" }}
          </p>
        </div>
      </header>
      <ol
        v-if="state.step < 4"
        class="batch-stagebar"
        aria-label="批次创建阶段"
      >
        <li
          v-for="(s, i) in stages"
          :key="s[0]"
          :class="{ active: stage === i, finished: stage > i }"
          :aria-current="stage === i ? 'step' : undefined"
        >
          <span class="batch-stage-index"
            ><CheckIcon v-if="stage > i" /><template v-else>{{
              i + 1
            }}</template></span
          >
          <div>
            <strong>{{ s[0] }}</strong
            ><small>{{ s[1] }}</small>
          </div>
        </li>
      </ol>
      <t-alert
        v-if="errors.length"
        theme="warning"
        :message="errors.join('；')"
        close-btn
        @close="errors = []"
        class="batch-message"
      />
      <t-alert
        v-if="softConflicts.length"
        theme="warning"
        :message="`同平台草稿「${softConflicts.map((b) => b.name).join('、')}」也选择了这些成片。草稿不占用，最终创建会重新检查。`"
        class="batch-message"
      />
      <template v-if="state.step === 0">
        <section
          class="batch-section"
          :class="{ emphasis: resourcePane === 0 }"
        >
          <header class="batch-section-head">
            <div>
              <span class="batch-number">01</span>
              <h3>选择发布范围</h3>
              <p v-if="resourcePane === 1">
                {{ platform }} · {{ state.categories.join(" / ") }}
              </p>
              <p v-else>确定单个平台与参与分配的类目</p>
            </div>
            <t-button
              v-if="resourcePane === 1"
              variant="text"
              @click="resourcePane = 0"
              >调整范围</t-button
            >
          </header>
          <div v-if="resourcePane === 0" class="batch-section-body">
            <div class="batch-form-grid">
              <label
                >批次名称<t-input
                  v-model="state.name"
                  placeholder="例如：游戏内容 · 首轮发布（可稍后命名）"
                  :maxlength="60" /></label
              ><label
                ><span>发布平台 <em>*</em></span
                ><t-select
                  :popup-props="{ attach: 'body', zIndex: 5500 }"
                  :value="platform"
                  :options="
                    allowedPlatforms.map((p) => ({ label: p, value: p }))
                  "
                  placeholder="选择一个平台"
                  :disabled="!!state.planId"
                  @change="changePlatform"
                  aria-label="批次发布平台"
              /></label>
            </div>
            <div class="batch-category-field">
              <label
                ><span>发布类目 <em>*</em></span></label
              ><t-checkbox-group
                v-model="state.categories"
                :disabled="!!state.planId"
                ><t-checkbox
                  v-for="c in allowedCategories"
                  :key="c"
                  :value="c"
                  class="batch-category-chip"
                  >{{ c }}</t-checkbox
                ></t-checkbox-group
              >
            </div>
          </div>
        </section>
        <section
          v-if="platform && state.categories.length"
          class="batch-section"
        >
          <header class="batch-section-head">
            <div>
              <span class="batch-number">{{
                resourcePane === 0 ? "02" : "资源"
              }}</span>
              <h3>资源自动盘点</h3>
              <p>
                {{
                  resourcePane === 0
                    ? "按所选平台和类目实时计算"
                    : `${accountIds.length} 个账号 · ${availableAssets.length} 条可用成片`
                }}
              </p>
            </div>
            <t-button
              v-if="resourcePane === 1"
              variant="text"
              @click="inventoryExpanded = !inventoryExpanded"
              >{{ inventoryExpanded ? "收起盘点" : "查看盘点" }}</t-button
            ><t-tag v-else theme="success" variant="light">盘点完成</t-tag>
          </header>
          <div
            v-if="resourcePane === 0 || inventoryExpanded"
            class="batch-section-body"
          >
            <div class="batch-metrics">
              <div>
                <small>可参与账号</small
                ><strong>{{ accountIds.length }}<span>个账号</span></strong>
                <p>已排除停用与登录失效账号</p>
              </div>
              <div>
                <small>合格可用成片</small
                ><strong
                  >{{ availableAssets.length }}<span>条成片</span></strong
                >
                <p>已排除同平台占用与缺失文件</p>
              </div>
              <div>
                <small>当前目标</small
                ><strong>{{ target }}<span>条任务</span></strong>
                <p>各类目目标合计</p>
              </div>
              <div :class="{ warning: gap }">
                <small>{{
                  resourcePane === 0 ? "建议本次目标" : "可匹配 / 目标缺口"
                }}</small
                ><strong
                  >{{
                    resourcePane === 0
                      ? categoryRows.reduce(
                          (n, r) => n + Math.min(r.available, 3),
                          0,
                        )
                      : suggested.length
                  }}<span>{{
                    resourcePane === 0 ? "条任务" : `/ 缺口 ${gap} 条`
                  }}</span></strong
                >
                <p>
                  {{
                    resourcePane === 0
                      ? "按可用成片建议，下一步可调整"
                      : "按本批次账号上限计算"
                  }}
                </p>
              </div>
            </div>
            <table class="batch-table">
              <thead>
                <tr>
                  <th>参与类目</th>
                  <th>适配账号</th>
                  <th>可用成片</th>
                  <th>同平台占用</th>
                  <th>其他不可用</th>
                  <th>资源情况</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="r in inventory" :key="r.key">
                  <td>
                    <b>{{ r.category }}</b>
                  </td>
                  <td>{{ r.accounts }} 个</td>
                  <td>{{ r.available }} 条</td>
                  <td>{{ r.occupied }} 条</td>
                  <td>{{ r.unavailable }} 条</td>
                  <td>
                    <t-tag
                      :theme="r.available ? 'success' : 'warning'"
                      variant="light"
                      >{{ r.available ? "可安排" : "需补充资源" }}</t-tag
                    >
                  </td>
                </tr>
              </tbody>
            </table>
            <p class="batch-inline-note">
              每日发布额度未配置，本页按可用成片与本批次账号上限匹配，不推算平台日额度。跨类目账号会共享本批次上限。
            </p>
          </div>
        </section>
        <div
          v-if="resourcePane === 1 && missingResources"
          class="batch-shortage"
        >
          <span
            >{{
              missingResources
            }}。请调整发布范围，或在对应资源库补充后重新盘点。</span
          >
          <t-button variant="text" @click="resourcePane = 0"
            >调整发布范围</t-button
          >
        </div>
        <section v-if="resourcePane === 1" class="batch-section emphasis">
          <header class="batch-section-head">
            <div>
              <span class="batch-number">02</span>
              <h3>类目数量与优先级</h3>
              <p>系统推荐数量，人工可调整；上方类目优先匹配</p>
            </div>
            <t-button
              variant="outline"
              :disabled="!!state.planId"
              @click="recommendTargets"
              ><template #icon><RefreshIcon /></template>采用系统推荐</t-button
            >
          </header>
          <div class="batch-section-body">
            <div class="batch-total-line">
              <div>
                <small>本次目标任务数</small
                ><strong>{{ target }} <span>条任务</span></strong>
              </div>
              <p>
                总目标由各类目数量自动合计{{
                  state.planId ? "，目标来自计划，只读" : ""
                }}
              </p>
            </div>
            <table class="batch-table batch-target-table">
              <thead>
                <tr>
                  <th>优先级</th>
                  <th>内容类目</th>
                  <th>可用成片</th>
                  <th>本批次目标</th>
                  <th>预计分配</th>
                  <th>缺口</th>
                  <th>排序</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(r, i) in categoryRows" :key="r.category">
                  <td>
                    <span class="batch-priority">P{{ i + 1 }}</span>
                  </td>
                  <td>
                    <b>{{ r.category }}</b>
                  </td>
                  <td>{{ r.available }} 条</td>
                  <td>
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
                  <td>{{ r.allocated }} 条</td>
                  <td :class="{ 'batch-warning-text': r.target > r.allocated }">
                    {{ Math.max(0, r.target - r.allocated) }} 条
                  </td>
                  <td>
                    <t-button
                      variant="text"
                      :disabled="i === 0 || !!state.planId"
                      @click="moveCategory(i, -1)"
                      >上移</t-button
                    ><t-button
                      variant="text"
                      :disabled="
                        i === categoryRows.length - 1 || !!state.planId
                      "
                      @click="moveCategory(i, 1)"
                      >下移</t-button
                    >
                  </td>
                </tr>
              </tbody>
            </table>
            <t-alert
              v-if="gap"
              theme="warning"
              :message="`目标 ${target} 条，当前可匹配 ${suggested.length} 条，缺口 ${gap} 条。可调整账号池与数量；保留缺口继续时，最终创建需要明确确认。`"
            />
            <details class="batch-account-pool">
              <summary>
                调整参与账号与本批次上限
                <span
                  >{{ accountIds.length }} 个账号参与 ·
                  默认使用全部合格账号</span
                >
              </summary>
              <div
                v-for="(a, i) in eligibleAccounts"
                :key="a.id"
                class="batch-account-setting"
              >
                <t-checkbox
                  :checked="accountIds.includes(a.id)"
                  @change="
                    (checked) => {
                      state.excludedAccounts = checked
                        ? state.excludedAccounts.filter((id) => id !== a.id)
                        : [...state.excludedAccounts, a.id];
                    }
                  "
                  >{{ a.name }}</t-checkbox
                ><span>{{ a.quality }} · {{ a.categories.join(" / ") }}</span
                ><label
                  >本批次上限<t-input-number
                    v-model="state.accountLimits[a.id]"
                    :min="1"
                    :decimal-places="0"
                    clearable
                    placeholder="不限制" /></label
                ><t-button
                  variant="text"
                  :disabled="
                    !accountIds.includes(a.id) || accountIds.indexOf(a.id) === 0
                  "
                  @click="moveAccount(accountIds.indexOf(a.id), -1)"
                  >上移</t-button
                ><t-button
                  variant="text"
                  :disabled="
                    !accountIds.includes(a.id) ||
                    accountIds.indexOf(a.id) === accountIds.length - 1
                  "
                  @click="moveAccount(accountIds.indexOf(a.id), 1)"
                  >下移</t-button
                >
              </div>
            </details>
          </div>
        </section>
      </template>
      <template v-else-if="state.step === 1">
        <section class="batch-section">
          <header class="batch-section-head">
            <div>
              <span class="batch-number">01</span>
              <h3>发布范围</h3>
              <p>{{ platform }} · {{ state.categories.join(" / ") }}</p>
            </div>
            <t-button
              variant="text"
              @click="
                state.step = 0;
                resourcePane = 0;
              "
              >调整范围</t-button
            >
          </header>
        </section>
        <section class="batch-section">
          <header class="batch-section-head">
            <div>
              <span class="batch-number">02</span>
              <h3>类目数量与优先级</h3>
              <p>
                目标 {{ target }} 条 ·
                {{
                  categoryRows
                    .map((r) => `${r.category} ${r.target}`)
                    .join(" / ")
                }}
              </p>
            </div>
            <t-button variant="text" @click="back">调整数量</t-button>
          </header>
        </section>
        <section class="batch-section emphasis">
          <header class="batch-section-head">
            <div>
              <span class="batch-number">03</span>
              <h3>账号与成片匹配</h3>
              <p>系统自动匹配，人工调整后确认资源</p>
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
            <div class="batch-table-scroll">
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
                                accountIds.includes(a.id) &&
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
                      <t-button variant="text" @click="openReplace(r)"
                        >更换成片</t-button
                      ><t-button
                        variant="text"
                        theme="danger"
                        @click="removeRow(r)"
                        >移出</t-button
                      >
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <p v-if="!matchingRows.length" class="batch-empty">
              此类目尚无可分配资源。可返回调整账号与目标，或保留缺口继续。
            </p>
            <p class="batch-inline-note">
              这里只检查资源组合。标题、创作声明和封面问题在内容阶段处理；资源确认前不创建正式任务。
            </p>
          </div>
        </section>
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
        <div class="batch-content-heading">
          <div>
            <h3>结果总览</h3>
            <p>按类目检查每条任务的实际内容与封面，确认后创建待执行任务。</p>
          </div>
          <t-tag :theme="issues.length ? 'warning' : 'success'" variant="light"
            >{{ ready }} / {{ state.rows.length }} 条已就绪</t-tag
          >
        </div>
        <section class="batch-section">
          <div class="batch-section-body batch-overview-top">
            <label
              ><span>批次名称 <em>*</em></span
              ><t-input
                v-model="state.name"
                placeholder="为本次发布命名"
                :maxlength="60"
            /></label>
            <div>
              <small>批次目标</small><strong>{{ target }}</strong>
            </div>
            <div>
              <small>正式创建</small><strong>{{ state.rows.length }}</strong>
            </div>
            <div :class="{ 'batch-warning-text': gap }">
              <small>目标缺口</small><strong>{{ gap }}</strong>
            </div>
          </div>
        </section>
        <t-alert
          v-if="issues.length"
          theme="warning"
          :message="`${issues.length} 条任务尚未通过预检。展开类目查看具体原因，点击“修复内容”处理。`"
          class="batch-message"
        />
        <section v-for="c in state.categories" :key="c" class="batch-section">
          <header class="batch-section-head">
            <div>
              <h3>{{ c }}</h3>
              <p>
                目标
                {{ state.targets.find((t) => t.category === c)?.count || 0 }} ·
                候选 {{ state.rows.filter((r) => r.category === c).length }}
              </p>
            </div>
          </header>
          <div class="batch-table-scroll">
            <table class="batch-table batch-results-table">
              <thead>
                <tr>
                  <th>账号 / 成片</th>
                  <th>实际发布内容</th>
                  <th>实际封面</th>
                  <th>最终预检</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="r in state.rows.filter((r) => r.category === c)"
                  :key="r.key"
                >
                  <td>
                    <b>{{ account(r.accountId)?.name }}</b
                    ><small
                      >V-{{ r.assetId }} · {{ asset(r.assetId)?.title }}</small
                    >
                  </td>
                  <td>
                    <b>{{ r.title }}</b>
                    <p>{{ r.description || "未填写简介" }}</p>
                    <small>{{ r.topics }} {{ r.tags }}</small
                    ><small
                      >封面短标题：{{ r.shortTitle || "未设置" }} · 短标语：{{
                        r.subTitle || "未设置"
                      }}</small
                    ><small
                      >{{ r.declaration || "声明待确认" }} ·
                      {{ r.schedule || "启动后尽快发布" }}</small
                    >
                  </td>
                  <td>
                    <button
                      class="batch-cover-preview"
                      @click="emit('preview', { kind: 'cover', row: r })"
                    >
                      <img :src="r.cover" alt="实际封面" />
                    </button>
                  </td>
                  <td>
                    <t-tag
                      :theme="validate(r, db).length ? 'warning' : 'success'"
                      variant="light"
                      >{{ validate(r, db).length ? "待处理" : "已通过" }}</t-tag
                    >
                    <ul v-if="validate(r, db).length" class="batch-error-list">
                      <li v-for="e in validate(r, db)" :key="e">{{ e }}</li>
                    </ul>
                  </td>
                  <td>
                    <t-button variant="text" @click="editContent(r)">{{
                      validate(r, db).length ? "修复内容" : "编辑内容"
                    }}</t-button
                    ><t-button
                      variant="text"
                      @click="emit('preview', { kind: 'cover', row: r })"
                      >预览封面</t-button
                    ><t-button
                      v-if="!state.resourcesLocked"
                      variant="text"
                      theme="danger"
                      @click="removeRow(r)"
                      >移出</t-button
                    >
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
        <p class="batch-inline-note">
          创建只生成批次与待执行任务，不启动发布。资源占用将在创建时重新检查；{{
            gap
              ? "将保留原始目标与缺口，提交前需确认部分创建。"
              : "所有任务通过预检后方可确认。"
          }}
        </p>
      </template>
      <section v-if="state.step === 4" class="batch-created-result">
        <div class="batch-created-check"><CheckIcon /></div>
        <h2>{{ resultSaved ? "批次与任务已创建" : "创建结果等待保存" }}</h2>
        <p>{{ state.name }}</p>
        <t-tag theme="primary" variant="light">待执行 · 尚未启动</t-tag>
        <div class="batch-created-metrics">
          <div>
            <small>原始目标</small><strong>{{ target }}</strong>
          </div>
          <div>
            <small>已创建任务</small
            ><strong>{{ createdResult?.tasks.length || 0 }}</strong>
          </div>
          <div
            :class="{
              'batch-warning-text': target > (createdResult?.tasks.length || 0),
            }"
          >
            <small>目标缺口</small
            ><strong>{{
              Math.max(0, target - (createdResult?.tasks.length || 0))
            }}</strong>
          </div>
        </div>
        <p class="batch-inline-note">
          {{
            resultSaved
              ? "任务已保存为待执行。进入批次详情后可检查任务与账号队列，明确启动后才开始执行。"
              : "任务已生成，正在保存创建结果；保存成功后可进入批次详情。"
          }}{{
            target > (createdResult?.tasks.length || 0)
              ? "已保留原始目标与缺口，不自动补量。"
              : ""
          }}
        </p>
      </section>
    </div>
    <template #footer
      ><template v-if="state.step === 4"
        ><span class="batch-footer-note"
          >批次 {{ createdResult?.batch.id }} ·
          {{ createdResult?.tasks.length }} 条待执行任务</span
        ><t-button v-if="resultSaved" @click="finish"
          >进入批次详情<template #suffix><ArrowRightIcon /></template></t-button
        ><t-button v-else :loading="busy" @click="retrySaveCreated"
          >重试保存创建结果</t-button
        ></template
      ><template v-else
        ><span class="batch-footer-note">{{
          state.step < 2
            ? `目标 ${target} · 已匹配 ${allocated.length} · 缺口 ${gap}`
            : `已就绪 ${ready} / ${state.rows.length} 条 · 创建后待执行`
        }}</span
        ><t-button
          variant="outline"
          :loading="busy"
          :disabled="contentBusy"
          @click="saveDraft(false)"
          >保存草稿</t-button
        ><t-button
          v-if="
            state.step === 1 ||
            state.step === 3 ||
            (state.step === 0 && resourcePane === 1)
          "
          variant="outline"
          @click="back"
          ><template #icon><ArrowLeftIcon /></template
          >{{ state.step === 3 ? "返回内容配置" : "上一步" }}</t-button
        ><t-button
          v-if="state.step < 3"
          :loading="busy || contentBusy"
          :disabled="
            contentBusy ||
            (state.step === 0
              ? !platform ||
                !state.categories.length ||
                (resourcePane === 1 && !target)
              : !state.rows.length)
          "
          @click="next"
          >{{
            state.step === 0
              ? resourcePane === 0
                ? "下一步：配置类目数量"
                : "下一步：匹配账号与成片"
              : state.step === 1
                ? "确认资源并进入内容配置"
                : "下一步：结果总览"
          }}<template #suffix><ArrowRightIcon /></template></t-button
        ><t-button
          v-else
          :loading="busy || contentBusy"
          :disabled="contentBusy || !state.rows.length || !!issues.length"
          @click="create"
          >确认创建 {{ state.rows.length }} 条待执行任务</t-button
        ></template
      ></template
    >
  </ResourceOverlay>
  <ResourceOverlay
    v-model:visible="lockOpen"
    title="确认资源，进入内容配置"
    :width="640"
    ><div class="batch-lock-summary">
      <CheckIcon />
      <h3>{{ state.rows.length }} 条账号与成片组合有效</h3>
      <p>
        {{ platform }} · 目标 {{ target }} 条 · 已分配
        {{ state.rows.length }} 条<template v-if="gap">
          · 缺口 {{ gap }} 条</template
        >
      </p>
    </div>
    <p class="batch-dialog-copy">
      确认后，平台、类目、目标、账号与成片在本草稿中变为只读。接下来配置文案与封面；锁定后只能恢复原账号、原成片的可用性；如永久不可用，请取消或放弃对应任务，再由新批次补足目标。
    </p>
    <t-alert
      theme="info"
      message="此操作不创建正式任务、不占用成片，也不会启动发布。"
    /><template #footer
      ><t-button variant="outline" @click="lockOpen = false"
        >继续调整资源</t-button
      ><t-button @click="confirmResources"
        >确认并进入内容配置</t-button
      ></template
    ></ResourceOverlay
  >
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
