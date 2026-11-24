<script setup>
import { computed, nextTick, onBeforeUnmount, ref, watch } from "vue";
import { onBeforeRouteLeave, onBeforeRouteUpdate, useRoute, useRouter } from "vue-router";
import ResourcePageHeader from "../../../shared/ui/resource/ResourcePageHeader.vue";
import ResourceCard from "../../../shared/ui/resource/ResourceCard.vue";
import ResourceDetailPage from "../../../shared/ui/resource/ResourceDetailPage.vue";
import ResourceEditorPage from "../../../shared/ui/resource/ResourceEditorPage.vue";
import ResourceFilterBar from "../../../shared/ui/resource/ResourceFilterBar.vue";
import ResourceId from "../../../shared/ui/resource/ResourceId.vue";
import ResourceNameLink from "../../../shared/ui/resource/ResourceNameLink.vue";
import ResourceRowActions from "../../../shared/ui/resource/ResourceRowActions.vue";
import ResourceSection from "../../../shared/ui/resource/ResourceSection.vue";
import ResourceStatusBadge from "../../../shared/ui/resource/ResourceStatusBadge.vue";
import ResourceWizardSteps from "../../../shared/ui/resource/ResourceWizardSteps.vue";
import ResourceDescriptions from "../../../shared/ui/resource/ResourceDescriptions.vue";
import ResourceFieldHelp from "../../../shared/ui/resource/ResourceFieldHelp.vue";
import FeedbackAlert from "../../../shared/ui/feedback/FeedbackAlert.vue";
import { feedback } from "../../../shared/ui/feedback/index.js";
import { createComposeClient } from "../../../shared/api/compose.js";
import { createSessionClient } from "../../../shared/api/session.js";
import { createPublicationBatchClient } from "../../../shared/api/publication.js";
import { createPublicationDraftClient } from "../../../shared/api/publicationDraft.js";
import { createFilesClient } from "../../../shared/api/files.js";
import { canRunLocalAgent } from "../../../utils.js";
import { platformLabel } from "../../../shared/utils/platforms.js";
import { formatPublicationTime } from "../dateTime.js";
import { batchAccountName, batchOutputName, useBatchResourceCatalog } from "../useBatchResourceCatalog.js";
import {
  batchFormProblem, batchInput, candidateShortfalls, configurationLabels, coverProfiles, emptyBatch, executionLabels, groupPolicies,
  savedBatchForRequest, sendNewBatch, snapshot,
} from "../batchPreparation.js";
import BatchResourceSetup from "../components/BatchResourceSetup.vue";
import BatchCandidateWorkspace from "../components/BatchCandidateWorkspace.vue";
import "../batchPreparation.css";

const api = createPublicationBatchClient();
const session = createSessionClient();
const compose = createComposeClient();
const files = createFilesClient();
const route = useRoute();
const router = useRouter();
const catalog = useBatchResourceCatalog();
const actor = ref(null);
const categories = ref([]);
const listFilters = ref({});
const list = ref({ items: [], total: 0 });
const detail = ref(null);
const review = ref(null);
const form = ref(null);
const records = ref({ items: [], total: 0 });
const recordsLoading = ref(false);
const recordsError = ref("");
const recordPage = ref(1);
const recordTab = ref("candidates");
const formBaseline = ref("");
const step = ref(0);
const pending = ref(null);
const recipes = ref({});
const coverFiles = ref({});
const loading = ref(false);
const saving = ref(false);
const preparing = ref(false);
const validating = ref(false);
const pageError = ref("");
const draftError = ref("");
const resourceError = ref("");
const formError = ref("");
const issueRowKey = ref("");
const listRoot = ref(null);
const stageHost = ref(null);
const newDraft = ref(null);
const persisted = ref(false);
let epoch = 0;
let recordEpoch = 0;
let listScroll = 0;
let allowLeave = false;
let disposed = false;
let routeDraftReady = false;

const steps = [{ title: "资源配置" }, { title: "内容准备" }, { title: "结果总览" }];
const listColumns = [
  { colKey: "id", title: "批次 ID", width: 140 },
  { colKey: "name", title: "批次名称", ellipsis: true, width: 210 },
  { colKey: "platform", title: "平台", width: 92 },
  { colKey: "coverage", title: "目标 / 候选", width: 120 },
  { colKey: "configuration_status", title: "配置状态", width: 106 },
  { colKey: "execution_status", title: "执行状态", width: 106 },
  { colKey: "updated_at", title: "更新时间", width: 180 },
  { colKey: "op", title: "操作", width: 130, fixed: "right" },
];
const candidateColumns = [
  { colKey: "output", title: "候选成片", width: 244 },
  { colKey: "account", title: "发布账号", width: 140 },
  { colKey: "category", title: "类目", width: 100 },
  { colKey: "content", title: "文案", width: 210, ellipsis: true },
  { colKey: "covers", title: "封面", width: 130 },
];
const recordColumns = [
  { colKey: "created_at", title: "时间", width: 190 },
  { colKey: "event", title: "事件", width: 250 },
  { colKey: "actor", title: "操作人", width: 160 },
];
const eventNames = { "publication_batch.create": "保存批次", "publication_batch.configure": "保存配置修改", created: "保存批次", configuration_updated: "保存配置修改",
  publication_batch_created: "保存批次", publication_batch_configuration_updated: "保存配置修改" };
const platformOptions = ["baidu", "bilibili", "xiaohongshu"].map(value => ({ value, label: platformLabel(value) }));
const options = labels => Object.entries(labels).map(([value, label]) => ({ value, label }));
const categoryOptions = computed(() => categories.value.filter(item => item.enabled !== false).map(item => ({ label: item.name, value: String(item.id) })));
const canPrepare = computed(() => canRunLocalAgent(actor.value));
const rawQuery = (key) => typeof route.query[key] === "string" ? route.query[key] : "";
const view = computed(() => {
  const mode = rawQuery("view");
  if (mode === "create") return mode;
  return ["edit", "detail", "copy"].includes(mode) && /^[1-9][0-9]*$/.test(rawQuery("id")) ? mode : "list";
});
const pageNum = computed(() => Math.max(1, Number.parseInt(rawQuery("page"), 10) || 1));
const pageSize = computed(() => [10, 20, 50].find(size => size === Number(rawQuery("page_size"))) || 10);
const dirty = computed(() => !!form.value && JSON.stringify(dirtyInput()) !== formBaseline.value);
const freeze = computed(() => saving.value || preparing.value || !!pending.value);
const canEdit = item => canPrepare.value && item?.allowed_actions?.includes("edit");
const canCopy = item => canPrepare.value && item?.allowed_actions?.includes("copy");
const isCreating = computed(() => view.value === "create" || view.value === "copy");

function categoryName(id) {
  return categories.value.find(item => String(item.id) === id)?.name || "原类目暂不可用";
}
const targetSum = targets => (targets || []).reduce((sum, target) => sum + BigInt(target.target_count), 0n).toString();
const accountName = row => batchAccountName(catalog.accounts.get(row.media_account_id));
const outputName = row => batchOutputName(catalog.outputs.get(row.composite_output_id));
const rowLabel = row => `${outputName(row)} · ${accountName(row)}`;
const configState = item => configurationLabels[item.configuration_status] || "配置状态未知";
const executionState = item => executionLabels[item.execution_status] || "执行状态未知";
const groupLabel = setting => groupPolicies.find(option => option.value === setting.between_group_policy)?.label || "未知";
const currentStage = computed(() => detail.value ? `编辑批次 · ${detail.value.name}` : "新建发布批次");
const readiness = computed(() => review.value?.readiness || null);

function problem(error, fallback) {
  const issue = error?.details?.issue;
  if (issue?.row_key) issueRowKey.value = issue.row_key;
  return issue?.message || error?.message || fallback;
}

function listLocation() {
  return Object.fromEntries(["search", "platform", "configuration_status", "execution_status", "page", "page_size"]
    .flatMap(key => rawQuery(key) ? [[key, rawQuery(key)]] : []));
}
function scrollHost(element) {
  let parent = element?.parentElement;
  while (parent && !/(auto|scroll)/.test(getComputedStyle(parent).overflowY)) parent = parent.parentElement;
  return parent || document.scrollingElement;
}
function open(mode, item) {
  if (view.value === "list") listScroll = scrollHost(listRoot.value)?.scrollTop || 0;
  const query = { ...listLocation(), view: mode };
  if (item) query.id = item.id;
  return router.push({ query });
}
function back() {
  if (view.value === "edit" && detail.value) return open("detail", detail.value);
  return router.push({ query: listLocation() });
}
function updateFilters(page = 1, size = pageSize.value) {
  const query = Object.fromEntries(Object.entries(listFilters.value).filter(([, value]) => value));
  if (page > 1) query.page = String(page);
  if (size !== 10) query.page_size = String(size);
  if (JSON.stringify(query) === JSON.stringify(listLocation())) return refresh();
  return router.replace({ query });
}
async function loadList(run) {
  const values = {};
  for (const key of ["search", "platform", "configuration_status", "execution_status"]) values[key] = rawQuery(key);
  listFilters.value = values;
  const result = await api.list({ ...values, page: pageNum.value, page_size: pageSize.value });
  if (run === epoch) list.value = result;
  await nextTick();
  if (run === epoch && listScroll) {
    const element = scrollHost(listRoot.value);
    if (element) element.scrollTop = listScroll;
    listScroll = 0;
  }
}
async function loadRecords(id = detail.value?.id, page = 1) {
  if (!id) return;
  const run = ++recordEpoch;
  recordsLoading.value = true;
  recordsError.value = "";
  recordPage.value = page;
  try {
    const data = await api.records(id, { page, page_size: 10 });
    if (run === recordEpoch) records.value = data;
  } catch (error) {
    if (run === recordEpoch) recordsError.value = problem(error, "读取记录失败");
  } finally {
    if (run === recordEpoch) recordsLoading.value = false;
  }
}
function showRecords() {
  recordTab.value = "records";
  return loadRecords(detail.value.id, recordPage.value);
}

function payload() {
  return { form: form.value, recipes: recipes.value, step: step.value, pending: pending.value };
}
function dirtyInput() {
  return view.value === "edit" ? { form: form.value, recipes: recipes.value } : payload();
}
function updateCoverResources(configuration, resources) {
  for (const entry of resources || []) {
    const row = configuration.config.candidates.find(item => item.row_key === entry.row_key);
    if (!row) continue;
    for (const profile of coverProfiles) {
      const resource_path = entry[profile.key === "horizontal" ? "cover_resource_path" : "portrait_cover_resource_path"];
      if (resource_path) {
        coverFiles.value[row.row_key] = {
          ...coverFiles.value[row.row_key],
          [profile.key]: { fileId: row.content[profile.file], resource_path },
        };
      }
    }
  }
}
function savedCoverURL(row, profile) {
  const selected = coverFiles.value[row.row_key]?.[profile.key];
  try { return selected?.fileId === row.content[profile.file] ? files.resourceURL(selected) : undefined; }
  catch { return undefined; }
}
function restore(value) {
  if (!value?.form) return;
  form.value = emptyBatch(value.form);
  recipes.value = value.recipes || {};
  step.value = Math.max(0, Math.min(2, value.step || 0));
  pending.value = value.pending || null;
  if (pending.value) step.value = 2;
}
async function finishCreated(saved) {
  routeDraftReady = false;
  const key = pending.value?.request_key;
  if (newDraft.value && key) {
    try {
      const local = await newDraft.value.get();
      if (local.payload?.pending?.request_key === key) await newDraft.value.clear();
    } catch (error) {
      feedback.toast.info(problem(error, "批次已保存，续编清理未完成"));
    }
  }
  pending.value = null;
  feedback.toast.success("批次已保存");
  allowLeave = true;
  await router.replace({ query: { ...listLocation(), view: "detail", id: saved.id } });
}

async function refresh() {
  const run = ++epoch;
  recordEpoch++;
  validating.value = false;
  loading.value = true;
  form.value = null;
  detail.value = null;
  review.value = null;
  pending.value = null;
  newDraft.value = null;
  pageError.value = "";
  draftError.value = "";
  resourceError.value = "";
  formError.value = "";
  issueRowKey.value = "";
  recordTab.value = "candidates";
  records.value = { items: [], total: 0 };
  recordsLoading.value = false;
  recordPage.value = 1;
  recipes.value = {};
  coverFiles.value = {};
  persisted.value = false;
  step.value = 0;
  routeDraftReady = false;
  try {
    const user = await session.me();
    if (run !== epoch) return;
    actor.value = user;
    if (view.value === "list") {
      await loadList(run);
      return;
    }
    const id = rawQuery("id");
    if (view.value !== "create") {
      const value = await api.get(id);
      if (run !== epoch) return;
      detail.value = value;
      if (view.value === "edit" && !canEdit(value)) throw new Error("此批次当前不允许编辑");
      if (view.value === "copy" && !canCopy(value)) throw new Error("仅可复制本人已结束或已取消的批次");
    }
    if (isCreating.value && !canPrepare.value) throw new Error("当前角色可查看批次记录");
    const loadingCategories = compose.listCategories({ scope: view.value === "detail" ? "all" : undefined })
      .then(result => { if (run === epoch) categories.value = result || []; })
      .catch(error => { if (run === epoch) resourceError.value = problem(error, "类目读取失败"); });
    if (view.value === "detail") {
      const configuration = emptyBatch(detail.value);
      updateCoverResources(configuration, detail.value.readiness?.cover_resources);
      await Promise.all([
        loadingCategories,
        catalog.hydrate(configuration.config, { owner: detail.value.operator_id, platform: detail.value.platform, readOnly: !canEdit(detail.value) })
          .catch(error => { if (run === epoch) resourceError.value = problem(error, "部分资源名称读取失败"); }),
      ]);
      return;
    }
    if (view.value === "edit") {
      form.value = emptyBatch(detail.value);
      updateCoverResources(form.value, detail.value.readiness?.cover_resources);
    } else {
      const client = createPublicationDraftClient({ userId: user.id, draftType: "batch" });
      newDraft.value = client;
      persisted.value = client.persistent;
      let local;
      let draftReadable = false;
      try {
        local = (await client.get()).payload;
        draftReadable = true;
      } catch (error) {
        if (run === epoch) draftError.value = problem(error, "读取续编失败");
      }
      if (run !== epoch) return;
      if (view.value === "copy") {
        if (!draftReadable) return;
        if (local?.pending) throw new Error("有待确认的批次保存，请先在新建批次中确认原请求");
        if (local?.form && !await feedback.confirmWarning({
          title: "替换新建批次续编", message: "复制将替换当前新建批次的续编输入。", confirmText: "复制批次",
        })) {
          allowLeave = true;
          return open("detail", detail.value);
        }
        if (run !== epoch) return;
        const source = await api.copy(id, detail.value.revision);
        if (run !== epoch) return;
        detail.value = null;
        form.value = emptyBatch(source);
        form.value.name = `${source.name.slice(0, 95)}（副本）`;
      } else {
        form.value = emptyBatch();
        restore(local);
      }
    }
    formBaseline.value = JSON.stringify(dirtyInput());
    if (pending.value) {
      try {
        const recovered = await savedBatchForRequest(api, pending.value.request_key);
        if (run !== epoch) return;
        if (recovered) return finishCreated(recovered);
        formError.value = "已保留原保存请求，请查询或重试";
      } catch (error) {
        if (run === epoch) formError.value = problem(error, "暂时无法查询原保存结果");
      }
    }
    const configuration = form.value;
    await Promise.all([
      loadingCategories,
      catalog.hydrate(configuration.config, { owner: user.id, platform: configuration.platform })
        .catch(error => { if (run === epoch) resourceError.value = problem(error, "部分资源读取失败，已保留原配置"); }),
    ]);
    if (run === epoch) routeDraftReady = true;
  } catch (error) {
    if (run === epoch) pageError.value = problem(error, "读取批次失败");
  } finally {
    if (run === epoch) loading.value = false;
  }
}
watch(() => route.fullPath, refresh, { immediate: true });
watch(() => JSON.stringify(payload()), () => {
  if (!routeDraftReady || !form.value || !newDraft.value || newDraft.value.persistent || !dirty.value) return;
  try { newDraft.value.remember(snapshot(payload())); } catch (error) { draftError.value = problem(error, "暂存输入失败"); }
});

async function checkIdentity() {
  const current = await session.me();
  if (disposed || String(current.id) !== String(actor.value?.id) || !canRunLocalAgent(current))
    throw new Error("登录账号已变化，请重新打开发布批次");
}
async function saveDraft() {
  if (!newDraft.value || saving.value || pending.value) return;
  saving.value = true;
  draftError.value = "";
  try {
    await checkIdentity();
    await newDraft.value.save(snapshot(payload()));
    formBaseline.value = JSON.stringify(dirtyInput());
    feedback.toast.success(persisted.value ? "已保存本机续编" : "已暂存当前填写");
  } catch (error) {
    draftError.value = problem(error, "续编保存失败，当前输入已保留");
  } finally {
    saving.value = false;
  }
}
async function save() {
  if (saving.value || preparing.value || !form.value) return;
  formError.value = "";
  issueRowKey.value = "";
  const localProblem = batchFormProblem(form.value);
  if (!pending.value && localProblem) {
    formError.value = localProblem;
    if (!form.value.name.trim() || !form.value.targets.length) step.value = 0;
    return;
  }
  saving.value = true;
  try {
    await checkIdentity();
    if (detail.value && view.value === "edit") {
      const saved = await api.update(detail.value.id, {
        ...batchInput(form.value), name: form.value.name.trim(), expected_revision: detail.value.revision,
      });
      if (!saved?.id || !saved?.revision) throw new Error("保存响应不完整，请重新核对批次记录");
      feedback.toast.success("批次修改已保存");
      allowLeave = true;
      await router.replace({ query: { ...listLocation(), view: "detail", id: saved.id } });
      return;
    }
    const retry = !!pending.value;
    if (!pending.value) {
      const input = { ...batchInput(form.value), name: form.value.name.trim(), request_key: crypto.randomUUID() };
      // Persist the original frozen request before sending it.
      await newDraft.value.save(snapshot({ ...payload(), pending: input }));
      pending.value = input;
    }
    await finishCreated(await sendNewBatch(api, pending.value, { retry }));
  } catch (error) {
    if (error.batchSaveRejected) {
      pending.value = null;
      try { await newDraft.value.save(snapshot(payload())); }
      catch (failure) { draftError.value = problem(failure, "续编保存失败，当前输入已保留"); }
    }
    formError.value = problem(error, "保存失败，当前输入已保留");
  } finally {
    saving.value = false;
  }
}

async function validate() {
  if (!form.value || saving.value || pending.value) return;
  const configuration = form.value;
  const localProblem = batchFormProblem(configuration, { requireName: false });
  if (localProblem) {
    review.value = { error: localProblem };
    return;
  }
  const input = batchInput(configuration);
  const fingerprint = JSON.stringify(input);
  const run = epoch;
  validating.value = true;
  review.value = null;
  try {
    const editContext = view.value === "edit"
      ? { batch_id: detail.value.id, expected_revision: detail.value.revision } : {};
    const result = await api.validate({ ...input, ...editContext });
    if (run !== epoch || JSON.stringify(batchInput(form.value)) !== fingerprint) return;
    review.value = { readiness: result };
    updateCoverResources(configuration, result.cover_resources);
  } catch (error) {
    if (run === epoch) review.value = { error: problem(error, "批次检查失败") };
  } finally {
    if (run === epoch) validating.value = false;
  }
}
function goStep(next) {
  if (freeze.value) return;
  if (next === 1 && !form.value.targets.length) {
    formError.value = "请先添加目标类目";
    return;
  }
  formError.value = "";
  step.value = next;
  if (next === 2) validate();
  nextTick(() => stageHost.value?.focus({ preventScroll: true }));
}
async function guardNavigation(to, from) {
  if (allowLeave) {
    allowLeave = false;
    return true;
  }
  if (to.fullPath === from.fullPath) return true;
  if (saving.value || preparing.value) return false;
  if (!dirty.value || !form.value || pending.value) return true;
  if (isCreating.value && !persisted.value) return true;
  const creation = isCreating.value;
  return feedback.confirmWarning({
    title: creation ? "离开批次配置" : "离开批次编辑",
    message: creation ? "当前修改尚未保存到本机续编，返回后只能恢复上次保存的输入。" : "当前修改尚未保存，离开后将丢失。",
    confirmText: "离开",
  });
}
onBeforeRouteUpdate(guardNavigation);
onBeforeRouteLeave(guardNavigation);
onBeforeUnmount(() => {
  disposed = true;
  epoch++;
  recordEpoch++;
});
</script>

<template>
  <div v-if="view === 'list'" ref="listRoot" class="publication-batches">
    <ResourcePageHeader title="发布批次">
      <template #actions><t-button v-if="canPrepare" theme="primary" @click="open('create')">新建批次</t-button></template>
    </ResourcePageHeader>
    <ResourceCard class="resource-list-card">
      <ResourceFilterBar>
        <label class="wt-filter-field wt-filter-field--wide"><span class="wt-filter-field__label">搜索批次</span>
          <t-input v-model="listFilters.search" aria-label="搜索批次" placeholder="输入批次名称或批次 ID" :maxlength="100" clearable @enter="updateFilters()" />
        </label>
        <label v-for="filter in [
          { key: 'platform', label: '平台', options: platformOptions },
          { key: 'configuration_status', label: '配置状态', options: options(configurationLabels) },
          { key: 'execution_status', label: '执行状态', options: options(executionLabels) },
        ]" :key="filter.key" class="wt-filter-field">
          <span class="wt-filter-field__label">{{ filter.label }}</span>
          <t-select v-model="listFilters[filter.key]" :options="filter.options" :aria-label="filter.label" placeholder="全部" clearable />
        </label>
        <div class="wt-filter-actions">
          <t-button theme="primary" :disabled="loading" @click="updateFilters()">查询</t-button>
          <t-button theme="default" variant="outline" :disabled="loading" @click="listFilters = {}; updateFilters()">重置</t-button>
        </div>
      </ResourceFilterBar>
      <FeedbackAlert v-if="pageError" theme="error" :message="pageError" />
      <div class="table-scroll-wrap resource-data-table">
        <t-table class="wt-resource-table" :columns="listColumns" :data="list.items || []" row-key="id" :loading="loading"
           :scroll="{ x: '1084px' }" :empty="pageError ? '读取失败' : '暂无匹配的发布批次'">
          <template #id="{ row }"><ResourceId :value="row.id" label="批次 ID" /></template>
          <template #name="{ row }"><ResourceNameLink @click="open('detail', row)">{{ row.name }}</ResourceNameLink></template>
          <template #platform="{ row }">{{ platformLabel(row.platform) }}</template>
          <template #coverage="{ row }">{{ targetSum(row.targets) }} / {{ row.candidate_count }}</template>
          <template #configuration_status="{ row }">
            <ResourceStatusBadge :label="configState(row)" :tone="row.configuration_status === 'ready' ? 'success' : 'warning'" />
          </template>
          <template #execution_status="{ row }"><ResourceStatusBadge :label="executionState(row)" /></template>
          <template #updated_at="{ row }">{{ formatPublicationTime(row.updated_at) }}</template>
          <template #op="{ row }"><ResourceRowActions>
            <t-button v-if="canEdit(row)" variant="text" size="small" @click="open('edit', row)">编辑配置</t-button>
            <t-button v-if="canCopy(row)" variant="text" size="small" @click="open('copy', row)">复制批次</t-button>
            <t-button v-if="!canEdit(row) && !canCopy(row)" variant="text" size="small" @click="open('detail', row)">查看</t-button>
          </ResourceRowActions></template>
        </t-table>
      </div>
      <div class="pagination-bar">
        <t-pagination :current="pageNum" :page-size="pageSize" :total="list.total" :page-size-options="[10, 20, 50]"
          :show-jumper="false" @current-change="updateFilters($event)" @page-size-change="updateFilters(1, $event)" />
      </div>
    </ResourceCard>
  </div>

  <ResourceDetailPage v-else-if="view === 'detail' && detail" :title="detail.name" @back="back">
    <FeedbackAlert v-if="resourceError" theme="warning" :message="resourceError" />
    <ResourceCard>
      <ResourceSection title="批次信息">
        <ResourceDescriptions><t-descriptions-item v-for="item in [
          { label: '发布平台', value: platformLabel(detail.platform) },
          { label: '配置状态', value: configState(detail) },
          { label: '执行状态', value: executionState(detail) },
          { label: '发布目标', value: `${targetSum(detail.targets)} 条` },
          { label: '候选配置', value: `${detail.candidate_count} 条` },
          { label: '更新时间', value: formatPublicationTime(detail.updated_at) },
        ]" :key="item.label" :label="item.label">{{ item.value }}</t-descriptions-item></ResourceDescriptions>
      </ResourceSection>
      <ResourceSection title="执行设置">
        <ResourceDescriptions><t-descriptions-item v-for="item in [
          { label: '并发数', value: detail.execution_setting.concurrency_limit },
          { label: '组间发布间隔', value: `${detail.execution_setting.group_interval_seconds} 秒` },
          { label: '组间处理', value: groupLabel(detail.execution_setting) },
          { label: '异常任务上限', value: detail.execution_setting.exception_task_limit },
        ]" :key="item.label" :label="item.label">{{ item.value }}</t-descriptions-item></ResourceDescriptions>
      </ResourceSection>
    </ResourceCard>
    <ResourceCard>
      <ResourceSection title="目标与缺口">
        <ul class="batch-categories">
          <li v-for="entry in detail.readiness?.shortfalls || []" :key="entry.category_id">
            <strong>{{ categoryName(entry.category_id) }}</strong>
            <span>目标 {{ entry.target_count }} · 候选 {{ entry.candidate_count }} · 缺口 {{ entry.shortfall }}</span>
          </li>
        </ul>
      </ResourceSection>
      <ResourceSection v-if="detail.readiness?.issues?.length" title="配置问题">
        <ul class="batch-issues">
          <li v-for="(issue, index) in detail.readiness.issues" :key="index">
            <strong v-if="detail.config.candidates?.find(row => row.row_key === issue.row_key)">{{ rowLabel(detail.config.candidates.find(row => row.row_key === issue.row_key)) }} · </strong>{{ issue.message }}
          </li>
        </ul>
      </ResourceSection>
    </ResourceCard>
    <ResourceCard>
      <t-tabs :value="recordTab" @change="$event === 'records' ? showRecords() : (recordTab = $event)">
        <t-tab-panel value="candidates" label="候选配置">
          <div class="batch-tab">
            <p v-if="detail.execution_status === 'not_started'" class="batch-muted">批次尚未开始，正式发布任务 0 条。</p>
            <div class="table-scroll-wrap resource-data-table">
              <t-table class="wt-resource-table" :columns="candidateColumns" :data="detail.config.candidates || []" row-key="row_key"
                :scroll="{ x: '824px' }" empty="暂无候选配置">
                <template #output="{ row }">{{ outputName(row) }}</template>
                <template #account="{ row }">{{ accountName(row) }}</template>
                <template #category="{ row }">{{ categoryName(row.category_id) }}</template>
                <template #content="{ row }">{{ row.content.description || '待完善' }}</template>
                <template #covers="{ row }">
                  <div class="batch-cell-covers">
                    <template v-for="profile in coverProfiles" :key="profile.key">
                      <img v-if="savedCoverURL(row, profile)" :src="savedCoverURL(row, profile)" :alt="profile.label" />
                    </template>
                    <span v-if="!coverProfiles.some(profile => savedCoverURL(row, profile))">—</span>
                  </div>
                </template>
              </t-table>
            </div>
          </div>
        </t-tab-panel>
        <t-tab-panel value="records" label="操作记录">
          <div class="batch-tab">
            <FeedbackAlert v-if="recordsError" theme="error" :message="recordsError" />
            <t-button v-if="recordsError" variant="text" @click="loadRecords(detail.id, recordPage)">重新读取</t-button>
            <div class="table-scroll-wrap resource-data-table">
              <t-table class="wt-resource-table" :columns="recordColumns" :data="records.items || []" row-key="id"
                :loading="recordsLoading" :scroll="{ x: '600px' }" :empty="recordsError ? '读取失败' : '暂无操作记录'">
                <template #created_at="{ row }">{{ formatPublicationTime(row.created_at) }}</template>
                <template #event="{ row }">{{ eventNames[row.event] || row.event }}</template>
                <template #actor="{ row }">{{ row.actor_name || (row.actor_id === String(actor?.id) ? '本人' : row.actor_id ? '操作用户' : '系统') }}</template>
              </t-table>
            </div>
            <div class="pagination-bar">
              <t-pagination :current="recordPage" :page-size="10" :total="records.total" :show-jumper="false" :show-page-size="false"
                @current-change="loadRecords(detail.id, $event)" />
            </div>
          </div>
        </t-tab-panel>
      </t-tabs>
    </ResourceCard>
    <template #footer>
      <span v-if="!canEdit(detail) && !canCopy(detail)" class="resource-footer-status batch-muted">
        {{ !canPrepare ? '当前角色可查看批次' : detail.operator_id !== String(actor?.id) ? '批次配置由所属运营处理' : '当前状态不允许编辑或复制' }}
      </span>
      <t-button v-if="canCopy(detail)" theme="primary" @click="open('copy', detail)">复制批次</t-button>
      <t-button v-if="canEdit(detail)" theme="primary" @click="open('edit', detail)">编辑配置</t-button>
    </template>
  </ResourceDetailPage>

  <ResourceEditorPage v-else-if="form && !loading" :title="currentStage" :back-label="view === 'edit' ? '返回批次详情' : '返回列表'"
    :busy="saving || preparing" @back="back">
    <div ref="stageHost" tabindex="-1" class="batch-stack batch-editor">
      <FeedbackAlert v-if="resourceError" theme="warning" :message="resourceError" />
      <FeedbackAlert v-if="draftError" theme="error" :message="draftError" />
      <FeedbackAlert v-if="formError" :theme="pending ? 'warning' : 'error'" :message="formError" />
      <p v-if="issueRowKey && form.config.candidates.some(row => row.row_key === issueRowKey)" class="batch-muted">
        {{ rowLabel(form.config.candidates.find(row => row.row_key === issueRowKey)) }}
      </p>
      <ResourceCard><ResourceWizardSteps :steps="steps" :current="step" /></ResourceCard>
      <BatchResourceSetup v-if="step === 0" :form="form" :catalog="catalog" :categories="categoryOptions"
        :actor="actor" :disabled="freeze" @work="preparing = $event" />
      <BatchCandidateWorkspace v-if="step === 1" :form="form" :catalog="catalog" :categories="categoryOptions"
        :actor="actor" :recipes="recipes" :cover-files="coverFiles" :disabled="freeze" :focus-key="issueRowKey" @work="preparing = $event" />
      <template v-if="step === 2">
        <ResourceCard>
          <ResourceSection title="准备结果">
            <ul class="batch-categories">
              <li v-for="entry in candidateShortfalls(form)" :key="entry.category_id">
                <strong>{{ categoryName(entry.category_id) }}</strong>
                <span>目标 {{ entry.target_count }} · 候选 {{ entry.candidate_count }} · 缺口 {{ entry.shortfall }}</span>
              </li>
            </ul>
            <p v-if="pending" class="batch-muted">原保存请求已保留，查询或重试期间配置保持原样。</p>
            <div v-else class="batch-inline batch-section-actions">
              <t-button theme="default" variant="outline" :loading="validating" :disabled="freeze" @click="validate">检查当前配置</t-button>
              <span v-if="validating" role="status" class="batch-muted">正在检查资源和内容</span>
              <ResourceStatusBadge v-else-if="readiness" :tone="readiness.ready ? 'success' : 'warning'"
                :label="readiness.ready ? '配置就绪' : '配置待完善'" />
            </div>
            <FeedbackAlert v-if="review?.error" theme="warning" :message="review.error" />
            <ul v-if="readiness?.issues?.length" class="batch-issues">
              <li v-for="(issue, index) in readiness.issues" :key="index">
                <strong v-if="form.config.candidates.find(row => row.row_key === issue.row_key)">{{ rowLabel(form.config.candidates.find(row => row.row_key === issue.row_key)) }} · </strong>
                {{ issue.message }}
                <t-button v-if="issue.row_key" size="small" variant="text" :disabled="freeze" @click="issueRowKey = issue.row_key; goStep(1)">查看候选</t-button>
              </li>
            </ul>
          </ResourceSection>
        </ResourceCard>
        <ResourceCard>
          <ResourceSection title="执行设置">
            <div class="batch-form-grid">
              <label>并发数<t-input-number v-model="form.execution_setting.concurrency_limit" :min="1" :decimal-places="0"
                aria-label="并发数" :disabled="freeze" @change="review = null" /></label>
              <label>组间发布间隔（秒）<t-input-number v-model="form.execution_setting.group_interval_seconds" :min="0" :decimal-places="0"
                aria-label="组间发布间隔（秒）" :disabled="freeze" @change="review = null" /></label>
              <label>组间处理<t-select v-model="form.execution_setting.between_group_policy" aria-label="组间处理" :options="groupPolicies"
                :disabled="freeze" @change="review = null" /></label>
              <label><span class="batch-field-title">异常任务上限<ResourceFieldHelp label="异常任务上限" content="异常任务达到该数量时，暂停发布，人工决定继续或终止。" /></span>
                <t-input-number v-model="form.execution_setting.exception_task_limit" aria-label="异常任务上限" :min="1" :max="4294967295"
                  :decimal-places="0" placeholder="输入正整数" :disabled="freeze" @change="review = null" />
              </label>
            </div>
          </ResourceSection>
        </ResourceCard>
      </template>
    </div>
    <template #footer>
      <span v-if="isCreating" class="batch-footer-note batch-muted">{{ persisted ? "续编保存在本机，保存批次后进入批次列表。" : "未保存的输入仅在当前页面会话保留。" }}</span>
      <t-button v-if="isCreating && persisted && !pending" theme="default" variant="outline" :disabled="freeze" @click="saveDraft">保存续编</t-button>
      <t-button v-if="step > 0 && !pending" theme="default" variant="outline" :disabled="freeze" @click="goStep(step - 1)">上一步</t-button>
      <t-button v-if="step < 2" theme="primary" :disabled="freeze" @click="goStep(step + 1)">下一步</t-button>
      <t-button v-else theme="primary" :loading="saving" :disabled="preparing" @click="save">
        {{ pending ? "查询或重试原请求" : detail ? "保存修改" : "保存批次" }}
      </t-button>
    </template>
  </ResourceEditorPage>

  <ResourceDetailPage v-else :title="view === 'detail' ? '发布批次详情' : '发布批次配置'" :busy="loading" @back="back">
    <p v-if="loading" role="status">正在读取批次</p>
    <FeedbackAlert v-if="pageError || draftError" theme="error" :message="pageError || draftError" />
    <t-button v-if="pageError || draftError" theme="default" variant="outline" @click="refresh">重新读取</t-button>
  </ResourceDetailPage>
</template>
