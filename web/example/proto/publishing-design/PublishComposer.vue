<script setup>
import { computed, reactive, ref, watch } from "vue";
import { useMediaQuery } from "../../../src/shared/composables/useMediaQuery.js";
import ResourceOverlay from "../../../src/shared/ui/resource/ResourceOverlay.vue";
import ResourceWizardSteps from "../../../src/shared/ui/resource/ResourceWizardSteps.vue";
import ResourceNotice from "../../../src/shared/ui/resource/ResourceNotice.vue";
import ResourceEmptyState from "../../../src/shared/ui/resource/ResourceEmptyState.vue";
import ResourceRowActions from "../../../src/shared/ui/resource/ResourceRowActions.vue";
import ResourceSelectionToolbar from "../../../src/shared/ui/resource/ResourceSelectionToolbar.vue";
import { createPublication } from "./relationships.js";
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
  active: false,
  accountLimits: {},
  accountOrder: [],
});
const appliedNotice = reactive({ platform: "", text: "" });
if (props.db.publishDraft) {
  Object.assign(state, copy(props.db.publishDraft));
  state.active =
    props.db.publishDraft.active ??
    !!(state.rows.length || state.assets.length || state.platforms.length);
}
watch(state, () => (props.db.publishDraft = copy(state)), { deep: true });
const errors = ref([]),
  onlyErrors = ref(false),
  rowPlatform = ref("全部"),
  contentPlatform = ref("小红书"),
  search = ref(""),
  showUnavailable = ref(false),
  assetPage = ref(1);
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
  set: (v) => (state.categories = v ? [v] : []),
});
const eligibleAccounts = computed(() =>
  props.db.accounts.filter(
    (a) =>
      a.enabled &&
      state.platforms.includes(a.platform) &&
      a.categories.some((c) => state.categories.includes(c)),
  ),
);
const accountIds = computed(() =>
  eligibleAccounts.value
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
const candidateColumns = computed(() => [
  { colKey: "asset", title: "成片", minWidth: 210 },
  { colKey: "platform", title: "平台 / 类目", width: 135 },
  { colKey: "account", title: "发布账号", minWidth: 205 },
  { colKey: "content", title: "发布内容", minWidth: 245 },
  { colKey: "check", title: "检查", width: 95 },
  {
    colKey: "op",
    title: "操作",
    width: 180,
    fixed: compact.value ? undefined : "right",
  },
]);
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
  const plan = props.db.plans.find((p) => p.id === planId);
  Object.assign(state, {
    mode,
    step: 0,
    platforms: source
      ? [source.platform]
      : mode === "快速发布"
        ? [plan?.targets[0]?.platform || "小红书"]
        : [],
    categories: source
      ? [source.category]
      : mode === "快速发布"
        ? [plan?.targets[0]?.category || "三角洲行动"]
        : [],
    assets: [],
    excludedAccounts: [],
    rows: [],
    name: "",
    planId,
    active: true,
    accountLimits: {},
    accountOrder: [],
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
  visible.value = true;
}
async function allocate() {
  errors.value = [];
  if (state.platforms.length !== 1 || !state.categories.length) {
    errors.value = ["请选择一个发布平台和至少一个内容类目"];
    return;
  }
  if (!selectedAssets.value.length) {
    errors.value = ["请选择本次要发布的可用成片"];
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
  contentPlatform.value = state.rows[0].platform;
  rowPlatform.value = "全部";
  state.step = isQuick.value ? 2 : 1;
  if (isQuick.value) emit("edit", state.rows[0]);
}
async function next() {
  errors.value = [];
  if (state.step === 0) {
    await allocate();
    return;
  }
  if (state.step === 1) {
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
  state.rows = mergeContentDefaults(state.rows, p, applied, overwrite);
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
  state.rows[index] = {
    ...copy(row),
    overrides: [...overrides],
    modified: overrides.size > 0,
  };
}
function removeRow(row) {
  state.rows = state.rows.filter((r) => r.key !== row.key);
}
function create() {
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
  let result;
  try {
    result = createPublication(props.db, {
      platform: selectedPlatform.value,
      rows: state.rows,
      name: state.name,
      planId: state.planId,
      quick: isQuick.value,
    });
  } catch (error) {
    errors.value = [error.message];
    return;
  }
  visible.value = false;
  state.rows = [];
  state.assets = [];
  state.active = false;
  emit("created", result);
}
defineExpose({
  open,
  resume: () => (visible.value = true),
  updateRow,
  hasDraft: computed(() => state.active),
});
</script>
<template>
  <ResourceOverlay
    v-model:visible="visible"
    :title="state.mode === '快速发布' ? '快速发布' : '创建发布批次'"
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
      <template v-if="state.step === 0">
        <section class="composer-section">
          <div class="section-heading">
            <div>
              <h3>确定发布范围</h3>
              <p>选择后自动盘点资源，再决定本次要安排哪些成片。</p>
            </div>
          </div>
          <div class="scope-grid">
            <div class="composer-field">
              <span>发布平台 <em>*</em></span>
              <t-select
                v-model="selectedPlatform"
                placeholder="选择一个发布平台"
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
                v-if="isQuick"
                v-model="selectedCategory"
                :options="
                  allowedCategories.map((c) => ({ label: c, value: c }))
                " /><t-select
                v-else
                v-model="state.categories"
                multiple
                clearable
                placeholder="可选择多个类目"
                :options="
                  allowedCategories.map((c) => ({ label: c, value: c }))
                "
            /></label>
          </div>
        </section>
        <template v-if="state.platforms.length && state.categories.length">
          <div class="inventory-summary" aria-live="polite">
            <div>
              <span>合格账号</span
              ><strong>{{ eligibleAccounts.length }}<small>个</small></strong>
            </div>
            <div>
              <span>可用成片 · 去重</span
              ><strong>{{ availableAssets.length }}<small>条</small></strong>
            </div>
            <div>
              <span>已选成片</span
              ><strong>{{ selectedAssets.length }}<small>条</small></strong>
            </div>
            <div class="is-highlight">
              <span>预计生成任务</span
              ><strong>{{ suggested.length }}<small>条</small></strong>
            </div>
          </div>
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
                <h3>选择本次成片</h3>
                <p>仅展示当前平台可用的成片；同平台其他批次的占用也会排除。</p>
              </div>
              <t-button
                v-if="!isQuick && !showUnavailable"
                variant="outline"
                :disabled="!availableAssets.length"
                @click="state.assets = availableAssets.map((v) => v.id)"
                >选择全部可用</t-button
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
                >可形成 {{ suggested.length }} 条平台任务</span
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
      <template v-else>
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
              <h3>检查分配建议</h3>
              <p>
                每条成片在当前平台只安排一次；仅分配当前平台账号，可逐条调整。
              </p>
            </div>
          </div></template
        >
        <section v-if="state.step === 2 && !isQuick" class="platform-defaults">
          <div class="section-heading">
            <div>
              <h3>配置当前平台内容</h3>
              <p>
                先统一确认必要字段，再处理单条差异。人工修改的字段默认保留。
              </p>
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
                v-model="platformDefaults[contentPlatform].declaration"
                placeholder="按本次实际内容确认"
                :options="declarationOptions(contentPlatform)" /></label
            ><label v-if="contentPlatform === 'B站'" class="composer-field"
              ><span>分区 <em>*</em></span
              ><t-select
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
        <div class="composer-table-tools">
          <span>{{ selectedPlatform }} · {{ state.rows.length }} 条任务</span
          ><t-checkbox v-if="state.step > 1" v-model="onlyErrors"
            >只看待处理 {{ issues.length }}</t-checkbox
          >
        </div>
        <t-table
          class="wt-resource-table"
          row-key="key"
          :data="rows"
          :columns="candidateColumns"
          :pagination="{
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
              :theme="validate(row, db).length ? 'warning' : 'success'"
              variant="light"
              >{{ validate(row, db).length ? "待处理" : "已就绪" }}</t-tag
            ><t-tooltip
              v-if="validate(row, db).length"
              :content="validate(row, db).join('；')"
              ><t-button
                class="issue-count"
                variant="text"
                @click="emit('edit', row)"
                >{{ validate(row, db).length }} 项问题</t-button
              ></t-tooltip
            ></template
          >
          <template #op="{ row }"
            ><ResourceRowActions
              ><t-button variant="text" @click="emit('preview', { row })"
                >预览</t-button
              ><t-button variant="text" @click="emit('edit', row)"
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
    <template #footer
      ><span class="composer-footer-note">{{
        state.step === 0
          ? `已选 ${selectedAssets.length} 条成片 · 预计 ${suggested.length} 条任务`
          : `${state.rows.length} 条任务 · ${readyCount} 条已就绪`
      }}</span
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
        :disabled="state.step === 0 ? !suggested.length : !state.rows.length"
        @click="next"
        >{{
          state.step === 0
            ? "采用建议并继续"
            : state.step === 1
              ? "配置发布内容"
              : "最终检查"
        }}</t-button
      ><t-button
        v-else
        :disabled="!state.rows.length || !!issues.length"
        @click="create"
        >创建 {{ state.rows.length }} 条待执行任务</t-button
      ></template
    >
  </ResourceOverlay>
</template>
<style scoped>
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
  margin: 24px 0;
}
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
