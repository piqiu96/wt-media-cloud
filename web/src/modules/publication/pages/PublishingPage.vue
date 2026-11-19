<script setup>
import { computed, onMounted, ref, watch, nextTick } from "vue";
import { useRoute, useRouter, onBeforeRouteLeave } from "vue-router";
import ResourcePageHeader from "../../../shared/ui/resource/ResourcePageHeader.vue";
import ResourceStatGrid from "../../../shared/ui/resource/ResourceStatGrid.vue";
import ResourceAdvancedToggle from "../../../shared/ui/resource/ResourceAdvancedToggle.vue";
import ResourceDateRange from "../../../shared/ui/resource/ResourceDateRange.vue";
import { publicationCreatedRangeParams as createdRangeParams, publicationScheduleInput, publicationScheduleUTC, formatPublicationTime as formatDateTime } from "../dateTime.js";
import ResourceCard from "../../../shared/ui/resource/ResourceCard.vue";
import ResourceFilterBar from "../../../shared/ui/resource/ResourceFilterBar.vue";
import ResourceRequiredLabel from "../../../shared/ui/resource/ResourceRequiredLabel.vue";
import ResourceDateTime from "../../../shared/ui/resource/ResourceDateTime.vue";
import ResourceFieldHelp from "../../../shared/ui/resource/ResourceFieldHelp.vue";
import ResourceSection from "../../../shared/ui/resource/ResourceSection.vue";
import ResourceDescriptions from "../../../shared/ui/resource/ResourceDescriptions.vue";
import ResourceOverlay from "../../../shared/ui/resource/ResourceOverlay.vue";
import { publicObjectReadUrl } from "../../../shared/utils/objectReadUrl.js";
import MediaPreview from "../../../shared/ui/media/MediaPreview.vue";
import { platformLabel } from "../../../shared/utils/platforms.js";
import ResourceEditorPage from "../../../shared/ui/resource/ResourceEditorPage.vue";
import ResourceDetailPage from "../../../shared/ui/resource/ResourceDetailPage.vue";
import ResourceRowActions from "../../../shared/ui/resource/ResourceRowActions.vue";
import { opColumnWidth } from "../../../shared/ui/resource/opColumnWidth.js";
import ResourceStatusBadge from "../../../shared/ui/resource/ResourceStatusBadge.vue";
import FeedbackAlert from "../../../shared/ui/feedback/FeedbackAlert.vue";
import { feedback } from "../../../shared/ui/feedback/index.js";
import { createPublicationClient } from "../../../shared/api/publication.js";
import { createComposeClient } from "../../../shared/api/compose.js";
import { createMediaAccountClient } from "../../../shared/api/mediaAccounts.js";
import { createFilesClient } from "../../../shared/api/files.js";
import {
  definitivePublicationRejection,
  contentProblem,
  publicationStatuses,
  outputUnavailable,
} from "../eligibility.js";
import { publicationTitleForOutput } from "../title.js";
import PublishingExecutionHistory from "../components/PublishingExecutionHistory.vue";
import PublishingResourcePicker from "../components/PublishingResourcePicker.vue";
import PublishingRecoveryDialog from "../components/PublishingRecoveryDialog.vue";
const client = createPublicationClient(),
  compose = createComposeClient(),
  accounts = createMediaAccountClient(),
  files = createFilesClient(),
  route = useRoute(),
  router = useRouter();
const blankFilters = () => ({
  search: "",
  status: "",
  platform: "",
  category_id: "",
  created_range: [],
});
const executionEnabled = ref(false), attempts = ref([]);
const recoveryVisible=ref(false),recoveryTask=ref(null),recoveryError=ref("");
const summary = ref(null),
  advanced = ref(false);
const metricItems = computed(() => [
  { key: "", label: "全部任务", value: summary.value?.all ?? "—" },
  {
    key: "awaiting_human",
    label: "等待人工",
    value: summary.value?.awaiting_human ?? "—",
    tone: "warning",
  },
  { key: "running", label: "执行中", value: summary.value?.running ?? "—" },
  {
    key: "uncertain",
    label: "结果待确认",
    value: summary.value?.uncertain ?? "—",
    tone: "warning",
  },
]);
function selectMetric(status) {
  filters.value.status = status;
  query();
}
const rows = ref([]),
  total = ref(0),
  page = ref(1),
  pageSize = ref(10),
  filters = ref(blankFilters()),
  applied = ref({ ...filters.value }),
  loading = ref(false),
  error = ref("");
const pageRoot = ref(null);
let listScroll = 0;
const view = ref("list"),
  detail = ref(null),
  categories = ref([]),
  busy = ref(false),
  formError = ref(""),
  editing = ref(null),
  form = ref(null),
  account = ref(null),
  output = ref(null),
  cover = ref(null),
  schedule = ref(""),
  picker = ref(""),
  baseline = ref(""),
  pendingRequest = ref(null);
const videoVisible = ref(false), videoURL = ref(""), videoLoading = ref(false), videoError = ref("");
let videoSequence = 0;
async function previewOutput() {
  if (!output.value || videoLoading.value) return;
  const sequence = ++videoSequence;
  videoURL.value = "";
  videoError.value = "";
  videoLoading.value = true;
  videoVisible.value = true;
  try {
    const link = await compose.getComposeOutputVideoUrl(output.value.id);
    if (sequence === videoSequence && videoVisible.value) videoURL.value = publicObjectReadUrl(link?.url);
  } catch (error) {
    if (sequence === videoSequence && videoVisible.value) videoError.value = error.message || "暂时无法预览，请重试";
  } finally { if (sequence === videoSequence) videoLoading.value = false; }
}
function videoFailed(event) {
  if (videoVisible.value && videoURL.value && event.target.currentSrc === videoURL.value) {
    videoError.value = "视频暂时无法读取，请重试预览";
  }
}
function closeVideo(visible) {
  videoVisible.value = visible;
  if (!visible) { videoSequence++; videoLoading.value = false; videoURL.value = ""; }
}
const preview = ref(false);
const previewImages = ref([]);
function previewImage(path, label) {
  if (!path) return;
  previewImages.value = [{ src: files.resourceURL({ resource_path: path }), label }];
  preview.value = true;
}
const source = ref(false),
  detailReturn = ref("list");
const columns = [
  { colKey: "id", title: "任务 ID", width: 80 },
  { colKey: "content", title: "发布内容", width: 280 },
  { colKey: "platform", title: "平台 / 类目", width: 120 },
  { colKey: "account", title: "发布账号", width: 180 },
  { colKey: "status", title: "状态", width: 120 },
  { colKey: "created_at", title: "创建时间", width: 170 },
  { colKey: "op", title: "操作", width: opColumnWidth(["执行", "编辑", "详情"], { variant: "text", primary: true }), fixed: "right" },
];
watch(view, async (next, previous) => {
  const container = pageRoot.value?.closest(".content-area");
  if (previous === "list") listScroll = container?.scrollTop || 0;
  await nextTick();
  if (container) container.scrollTop = next === "list" ? listScroll : 0;
});
const categoryOptions = computed(() =>
  categories.value.map((c) => ({ label: c.name, value: String(c.id) })),
);
const frozen = computed(() => busy.value || !!pendingRequest.value);
const dirty = computed(
  () =>
    form.value &&
    JSON.stringify({ form: form.value, schedule: schedule.value }) !==
      baseline.value,
);
const statusLabel = (v) => publicationStatuses[v] || v;
const categoryName = (id) =>
  categories.value.find((c) => String(c.id) === String(id))?.name || "—";
const emptyForm = () => ({
  request_key: crypto.randomUUID(),
  platform: "baidu",
  category_id: "",
  media_account_id: "",
  composite_output_id: "",
  content: {
    description: "",
    cover_file_id: "",
    topics: "",
    declaration: "",
    confirmed: false,
  },
});
let loadSequence = 0;
async function load() {
  const n = ++loadSequence;
  loading.value = true;
  error.value = "";
  try {
    const data = await client.list({
      ...applied.value,
      ...createdRangeParams(applied.value.created_range),
      created_range: undefined,
      page: page.value,
      page_size: pageSize.value,
    });
    if (n !== loadSequence) return;
    rows.value = data.items;
    total.value = data.total;
    summary.value = data.summary;
    executionEnabled.value = data.execution_enabled === true;
  } catch (e) {
    if (n === loadSequence) error.value = e.message;
  } finally {
    if (n === loadSequence) loading.value = false;
  }
}
function query() {
  applied.value = { ...filters.value };
  page.value = 1;
  load();
}
function invalidate() {
  formError.value = "";
  if (form.value) form.value.content.confirmed = false;
}
async function changeCategory(category) {
  if (frozen.value) return;
  if (account.value || output.value) {
    if (
      !(await feedback.confirmWarning({
        title: "更换发布类目",
        message: "账号、成片和内容将清空，请重新核对。",
        confirmText: "更换",
      }))
    )
      return;
  }
  form.value = emptyForm();
  form.value.category_id = category;
  account.value = null;
  output.value = null;
  cover.value = null;
  schedule.value = "";
}
function openCreate() {
  source.value = false;
  editing.value = null;
  detail.value = null;
  attempts.value = [];
  account.value = null;
  output.value = null;
  cover.value = null;
  schedule.value = "";
  form.value = emptyForm();
  formError.value = "";
  pendingRequest.value = null;
  baseline.value = JSON.stringify({ form: form.value, schedule: "" });
  view.value = "form";
}
async function openDetail(row) {
  loading.value = true;
  error.value = "";
  try {
    const [task, history] = await Promise.all([client.get(row.id), client.attempts(row.id)]);
    detail.value = task;
    attempts.value = history;
    view.value = "detail";
  } catch (e) {
    error.value = e.message;
  } finally {
    loading.value = false;
  }
}
const editable = (task) => task.status === "pending" && !task.current_attempt_id && !task.authorized_at && !task.cancel_requested_at;
const retryable = (task) => task.status === "failed" && !task.cancel_requested_at && executionEnabled.value;
async function startExecution(row) {
  if (busy.value || !executionEnabled.value) return;
  busy.value = true;
  error.value = "";
  try {
    const task = await client.get(row.id);
    if (!editable(task) || task.revision !== row.revision) throw new Error("任务内容或状态已变化，请核对更新后的列表再操作");
    if (!(await feedback.confirmWarning({title:"确认执行发布",message:`将使用账号「${task.resources.account_name}」自动发布此任务。请确认文案、封面和声明；执行后不能再编辑。`,confirmText:"确认执行"}))) return;
    await client.start(task.id, {expected_revision: task.revision});
    await load();
    feedback.toast.success("已授权执行，等待所属用户的 Agent 领取");
  } catch (e) {
    const message = e.message;
    // Read back state after an uncertain response; never resend authorization.
    await load();
    error.value = message;
  } finally { busy.value = false; }
}
async function openVerification(row) {
  if (busy.value) return;
  busy.value = true;
  error.value = "";
  try {
    const task = await client.get(row.id);
    if (task.status !== "uncertain") throw new Error("任务状态已变化，请核对列表后操作");
    recoveryTask.value = JSON.parse(JSON.stringify(task));
    recoveryError.value = "";
    recoveryVisible.value = true;
  } catch (e) {
    const message = e.message;
    await load();
    error.value = message;
  } finally { busy.value = false; }
}
async function saveVerification(input) {
  if(busy.value || !recoveryTask.value)return;
  busy.value=true;recoveryError.value='';
  const task=recoveryTask.value;
  try {
    await client.resolve(task.id,input);
    recoveryVisible.value=false;
    await load();
    feedback.toast.success('已保存核实结果，原尝试与资源占用保留');
  } catch(e) {
    // Preserve the form. A missing response is not a reason to resend a write.
    recoveryError.value=e.message+'；请保留依据，关闭后在任务列表查询核对结果。';
  } finally {busy.value=false;}
}
async function retryExecution(row) {
  if (busy.value || !executionEnabled.value) return;
  busy.value = true;
  error.value = "";
  try {
    const task = await client.get(row.id);
    if (!retryable(task) || task.revision !== row.revision) throw new Error("任务内容或状态已变化，请核对更新后的列表再操作");
    if (!(await feedback.confirmWarning({title:"重试任务",message:"确认原尝试无可发布稿件、执行现场已结束。将复查原账号、成片、封面与内容，沿用本任务重新执行；历史尝试与占用保留。",confirmText:"确认重试"}))) return;
    await client.retry(task.id, {expected_revision: task.revision});
    await load();
    feedback.toast.success("原任务已重新排队，等待所属用户的 Agent 领取");
  } catch (e) {
    const message = e.message;
    await load();
    error.value = message;
  } finally { busy.value = false; }
}
async function edit(row) {
  busy.value = true;
  error.value = "";
  try {
    const v = await client.get(row.id);
    if (!editable(v))
      throw Error("当前任务已不能编辑，请刷新列表");
    const [a, o] = await Promise.all([
      accounts.get(v.media_account_id),
      compose.getComposeOutput(v.composite_output_id),
    ]);
    editing.value = v;
    account.value = a;
    output.value = o.output;
    cover.value = {
      id: v.content.cover_file_id,
      name: v.resources.cover_name,
      resource_path: v.resources.cover_resource_path,
    };
    form.value = {
      request_key: v.request_key,
      platform: v.platform,
      category_id: v.category_id,
      media_account_id: v.media_account_id,
      composite_output_id: v.composite_output_id,
      content: { ...v.content },
    };
    schedule.value = publicationScheduleInput(v.not_before);
    baseline.value = JSON.stringify({
      form: form.value,
      schedule: schedule.value,
    });
    formError.value = "";
    pendingRequest.value = null;
    view.value = "form";
  } catch (e) {
    error.value = e.message;
  } finally {
    busy.value = false;
  }
}
function setOutput(row) {
  form.value.content.description = publicationTitleForOutput(form.value.content.description, output.value, row);
  output.value = row;
  form.value.composite_output_id = row ? String(row.id) : "";
}
function picked(row) {
  if (picker.value === "account") {
    account.value = row;
    form.value.media_account_id = String(row.id);
    if (
      output.value &&
      outputUnavailable(output.value, form.value.category_id, row)
    ) {
      setOutput(null);
    }
    invalidate();
  } else if (picker.value === "output") {
    setOutput(row);
    invalidate();
  } else {
    cover.value = row;
    form.value.content.cover_file_id = String(row.id);
    invalidate();
  }
  picker.value = "";
}
async function submit() {
  formError.value = "";
  const problem = contentProblem(form.value);
  if (problem) {
    formError.value = problem;
    return;
  }
  busy.value = true;
  try {
    const not_before = publicationScheduleUTC(schedule.value);
    const payload =
      pendingRequest.value ||
      (editing.value
        ? {
            expected_revision: editing.value.revision,
            content: { ...form.value.content },
            not_before,
          }
        : { ...form.value, content: { ...form.value.content }, not_before });
    pendingRequest.value = payload;
    const saved = editing.value
      ? await client.edit(editing.value.id, payload)
      : await client.create(payload);
    pendingRequest.value = null;
    baseline.value = JSON.stringify({
      form: form.value,
      schedule: schedule.value,
    });
    detail.value = saved;
    view.value = "detail";
    detailReturn.value = source.value ? "source" : "list";
    feedback.toast.success(
      editing.value ? "发布内容已保存" : "任务已创建，尚未执行",
    );
    await load();
  } catch (e) {
    formError.value = e.message;
    if (definitivePublicationRejection(e)) pendingRequest.value = null;
    else formError.value += "。结果尚未确认，请保留当前输入并重试。";
  } finally {
    busy.value = false;
  }
}
async function canLeave() {
  if (busy.value) return false;
  if (!dirty.value && !pendingRequest.value) return true;
  return feedback.confirmWarning({
    title: "离开发布内容",
    message: pendingRequest.value
      ? "创建结果尚未确认，请优先重试核实。离开后可在任务列表查找。"
      : "当前修改尚未保存，离开后将丢失。",
    confirmText: "离开",
  });
}
async function back() {
  if (busy.value) return;
  if (view.value === "form" && !(await canLeave())) return;
  if (source.value || detailReturn.value === "source") {
    view.value = "list";
    await router.push("/finished-media");
    return;
  }
  view.value = "list";
  detail.value = null;
}
onBeforeRouteLeave(async () => !busy.value && (view.value !== "form" || (await canLeave())));
onMounted(async () => {
  await load();
  try {
    categories.value = await compose.listCategories({
      scope: "all",
      enabled: true,
    });
    const id = String(route.query.output_id || "");
    if (/^[1-9][0-9]*$/.test(id)) {
      const data = await compose.getComposeOutput(id);
      openCreate();
      source.value = true;
      setOutput(data.output);
      form.value.category_id = String(data.output.category_id);
    }
  } catch (e) {
    error.value = e.message;
  }
});
</script>
<template>
  <div ref="pageRoot" class="wt-resource-page resource-visual-approved publishing-page resource-form-actions-scope" :class="{ 'resource-list-page': view === 'list' }">
    <template v-if="view === 'list'">
      <ResourcePageHeader title="发布任务"
        ><template #actions
          ><t-button @click="openCreate"
            >快速发布</t-button
          ></template
        ></ResourcePageHeader
      >
      <FeedbackAlert v-if="error" theme="error" :message="error" />
      <ResourceStatGrid :items="metricItems" interactive :active-key="applied.status" @select="selectMetric" />
      <ResourceCard class="resource-list-panel">
        <ResourceFilterBar
          ><label class="wt-filter-field wt-filter-field--wide"
            ><span class="wt-filter-field__label">综合搜索</span><t-input
              v-model="filters.search"
              aria-label="任务搜索"
              placeholder="发布标题 / 任务 ID"
              clearable
              @enter="query" /></label
          ><label class="wt-filter-field"
            ><span class="wt-filter-field__label">发布平台</span><t-select
              v-model="filters.platform"
              :options="[
                { label: '百度（百家号）', value: 'baidu' },
                { label: 'B站', value: 'bilibili' },
                { label: '小红书', value: 'xiaohongshu' },
              ]"
              clearable
              placeholder="全部" /></label
          ><label class="wt-filter-field"
            ><span class="wt-filter-field__label">任务状态</span><t-select
              v-model="filters.status"
              :options="
                Object.entries(publicationStatuses).map(([value, label]) => ({
                  value,
                  label,
                }))
              "
              clearable
              placeholder="全部"
          /></label>
          <label class="wt-filter-field"
            ><span class="wt-filter-field__label">内容类目</span><t-select
              v-model="filters.category_id"
              :options="categoryOptions"
              clearable
              placeholder="全部"
          /></label>
          <div class="wt-filter-actions">
            <t-button theme="primary" @click="query">查询</t-button
            ><t-button
              variant="outline"
              @click="
                filters = blankFilters();
                query();
              "
              >重置</t-button
            >
          </div>
          <ResourceAdvancedToggle
            v-model="advanced"
            :count="applied.created_range.length ? 1 : 0"
            controls="publication-advanced" />
          <div
            v-if="advanced"
            id="publication-advanced"
            class="wt-advanced-filters"
          >
            <label class="wt-filter-field"
              ><span class="wt-filter-field__label">创建时间</span><ResourceDateRange v-model="filters.created_range"
            /></label></div
        ></ResourceFilterBar>
        <div class="table-scroll-wrap resource-data-table">
          <t-table
            class="wt-resource-table"
            :columns="columns"
            :data="rows"
            row-key="id"
            :loading="loading"
            :scroll="{ x: columns.reduce((sum, column) => sum + column.width, 0) + 'px' }"
            empty="暂无发布任务"
          >
            <template #content="{ row }"
              ><div class="publication-content-cell">
                <img
                  v-if="row.resources.cover_resource_path"
                  :src="
                    files.resourceURL({
                      resource_path: row.resources.cover_resource_path,
                    })
                  "
                  alt="发布封面"
                  class="publication-thumbnail"
                />
                <div class="publication-content">
                  <strong>{{ row.content.description }}</strong
                  ><small>成片 {{ row.composite_output_id }}</small>
                </div>
              </div></template
            >
            <template #platform="{row}">{{ platformLabel(row.platform) }}<small class="publication-sub">{{categoryName(row.category_id)}}</small></template><template #account="{ row }"
              >{{ row.resources.account_name || "—"
              }}<small class="publication-sub">{{
                row.resources.account_owner || "—"
              }}</small></template
            >
            <template #status="{ row }"
              ><ResourceStatusBadge
                :label="statusLabel(row.status)" /></template
            ><template #created_at="{ row }">{{
              formatDateTime(row.created_at)
            }}</template>
            <template #op="{ row }">
              <ResourceRowActions>
                <t-button v-if="editable(row) && executionEnabled" class="is-primary" variant="text" size="small" :disabled="busy" @click="startExecution(row)">执行</t-button>
                <t-button v-if="row.status === 'uncertain'" class="is-primary" variant="text" size="small" :disabled="busy" @click="openVerification(row)">核实结果</t-button>
                <t-button v-if="retryable(row)" class="is-primary" variant="text" size="small" :disabled="busy" @click="retryExecution(row)">重试</t-button>
                <t-button v-if="editable(row)" variant="text" size="small" :disabled="busy" @click="edit(row)">编辑</t-button>
                <t-button variant="text" size="small" :disabled="busy" @click="detailReturn = 'list'; openDetail(row)">详情</t-button>
              </ResourceRowActions>
            </template>
          </t-table>
        <div class="pagination-bar">
          <t-pagination
            :current="page"
            :page-size="pageSize"
            :total="total"
            :show-jumper="false"
            :page-size-options="[10, 20, 50]"
            @change="
              page = $event.current;
              pageSize = $event.pageSize;
              load();
            "
          />
        </div>
        </div>
      </ResourceCard>
    </template>
    <ResourceEditorPage
      v-else-if="view === 'form'"
      :title="editing ? '编辑发布内容' : '快速发布'"
      :busy="busy"
      :back-label="source ? '返回成片管理' : '返回发布任务列表'"
      @back="back"
    >
      <FeedbackAlert v-if="formError" theme="error" :message="formError" />
      <div class="quick-publish">
        <section class="resource-editor-section quick-block">
          <h3>
            <span>01</span>{{ editing ? "发布资源（已锁定）" : "选择发布平台与账号" }}
          </h3>
          <div class="quick-resource-grid">
            <label
              ><ResourceRequiredLabel>发布平台</ResourceRequiredLabel><t-select
                value="baidu"
                :options="[{ label: '百度（百家号）', value: 'baidu' }]"
                disabled /></label
            ><label
              ><ResourceRequiredLabel>发布类目</ResourceRequiredLabel><t-select
                :value="form.category_id"
                aria-label="发布类目"
                :options="categoryOptions"
                :disabled="!!editing || frozen"
                @change="changeCategory" /></label
            ><label
              ><ResourceRequiredLabel>发布账号</ResourceRequiredLabel><t-input
                :value="account?.platform_name || ''"
                placeholder="请选择发布账号"
                readonly
                :disabled="!form.category_id || !!editing || frozen"
                @click="picker = 'account'"
                ><template #suffix
                  ><t-button
                    variant="text"
                    size="small"
                    :disabled="!form.category_id || !!editing || frozen"
                    @click.stop="picker = 'account'"
                    >选择</t-button
                  ></template
                ></t-input
              ></label
            >
          </div>
          <div class="quick-selected-asset">
            <div>
              <strong>{{ output?.name || "选择本次要发布的成片" }}</strong
              ><small>{{
                output
                  ? `成片 ${output.id} · ${categoryName(output.category_id)}`
                  : "只安排账号支持、同平台未占用的成片。"
              }}</small>
            </div>
            <t-button
              variant="outline"
              :disabled="!form.media_account_id || !!editing || frozen"
              @click="picker = 'output'"
              >{{ output ? "更换成片" : "选择成片" }}</t-button
            ><t-button v-if="output" theme="default" variant="outline" :disabled="busy" @click="previewOutput">预览成片</t-button>
          </div>
        </section>
        <section v-if="output" class="resource-editor-section quick-block">
          <h3><span>02</span>发布内容</h3>
          <div class="quick-copy-fields">
          <label
            ><ResourceRequiredLabel>发布标题</ResourceRequiredLabel><t-textarea
              v-model="form.content.description"
              aria-label="发布标题"
              placeholder="默认带入素材标题，可修改"
              :maxlength="1000"
              :autosize="{ minRows: 3, maxRows: 5 }"
              :disabled="frozen"
              @change="invalidate"
          /></label>
          <div class="quick-cover-field">
            <label class="quick-field-heading"><ResourceRequiredLabel>发布封面</ResourceRequiredLabel><ResourceFieldHelp label="发布封面" content="本阶段使用现成图片，可从已有文件中选择。按原图预览，不改变文件内容。" /></label>
            <div class="quick-cover-workspace">
              <button v-if="cover?.resource_path" type="button" class="publication-image-button" aria-label="预览发布封面" @click="previewImage(cover.resource_path, cover.name)"><img :src="files.resourceURL(cover)" alt="当前发布封面" /><span>预览封面</span></button>
              <div>
                <strong>{{ cover?.name || "请选择现成图片" }}</strong
                ><t-button
                  variant="outline"
                  :disabled="frozen"
                  @click="picker = 'image'"
                  >{{ cover ? "更换图片" : "选择图片" }}</t-button
                >
              </div>
            </div>
          </div>
          <label
            >话题<t-input
              v-model="form.content.topics"
              aria-label="话题"
              :maxlength="200"
              :disabled="frozen"
              @change="invalidate"
          /></label>
          <label
            ><ResourceRequiredLabel>创作声明</ResourceRequiredLabel><t-select
              v-model="form.content.declaration"
              aria-label="创作声明"
              :options="[{ label: '无需声明', value: '无需声明' }]"
              placeholder="按内容事实选择"
              :disabled="frozen"
              @change="invalidate"
          /></label>
          </div>
        </section>
        <section v-if="output" class="resource-editor-section quick-block">
          <h3><span>03</span>发布设置</h3>
          <div class="quick-resource-grid two">
            <label>发布方式<t-input value="自动发布" disabled /></label>
            <label><span class="quick-field-heading">最早执行时间<ResourceFieldHelp label="最早执行时间" content="留空表示确认执行后尽快发布；指定时间为最早开始执行时间。创建任务后仍需单独确认执行。" /></span><ResourceDateTime v-model:value="schedule" aria-label="最早执行时间" :disabled="frozen" @change="invalidate" /></label>
          </div>
          <t-checkbox v-model="form.content.confirmed" :disabled="frozen"
            >已核对当前文案、封面和声明，确认内容可用</t-checkbox
          >
        </section>
      </div>
      <template #footer
        ><t-button :loading="busy" @click="submit">{{
          pendingRequest ? "重试确认结果" : editing ? "保存" : "创建任务"
        }}</t-button></template
      >
    </ResourceEditorPage>
    <ResourceDetailPage
      v-else-if="detail"
      title="发布任务详情"
      :busy="busy"
      :back-label="
        detailReturn === 'source' ? '返回成片管理' : '返回发布任务列表'
      "
      @back="back"
    >
      <FeedbackAlert v-if="error" theme="error" :message="error" />
      <ResourceCard class="detail-card"><ResourceSection title="任务信息">
        <ResourceDescriptions>
          <t-descriptions-item label="任务 ID">{{ detail.id }}</t-descriptions-item>
          <t-descriptions-item label="任务状态"><ResourceStatusBadge :label="statusLabel(detail.status)" /></t-descriptions-item>
          <t-descriptions-item label="发布平台">{{ platformLabel(detail.platform) }}</t-descriptions-item>
          <t-descriptions-item label="内容类目">{{ categoryName(detail.category_id) }}</t-descriptions-item>
          <t-descriptions-item label="发布账号">{{ detail.resources.account_name || "—" }}</t-descriptions-item>
          <t-descriptions-item label="真实 UID">{{ detail.platform_account_uid }}</t-descriptions-item>
          <t-descriptions-item label="成片" full>{{ detail.resources.output_name || detail.composite_output_id }}</t-descriptions-item>
        </ResourceDescriptions>
      </ResourceSection></ResourceCard>
      <ResourceCard class="detail-card"><ResourceSection title="发布内容">
        <ResourceDescriptions>
          <t-descriptions-item label="发布标题" full><span class="publication-description">{{ detail.content.description }}</span></t-descriptions-item>
          <t-descriptions-item label="发布封面" full>
            <div class="publication-cover-summary">
              <button v-if="detail.resources.cover_resource_path" type="button" class="publication-image-button" aria-label="预览最终发布封面" @click="previewImage(detail.resources.cover_resource_path, detail.resources.cover_name)"><img :src="files.resourceURL({ resource_path: detail.resources.cover_resource_path })" alt="最终发布封面" /><span>预览封面</span></button>
              <span>{{ detail.resources.cover_name || detail.content.cover_file_id }}</span>
            </div>
          </t-descriptions-item>
          <t-descriptions-item label="话题" full>{{ detail.content.topics || "—" }}</t-descriptions-item>
          <t-descriptions-item label="创作声明">{{ detail.content.declaration }}</t-descriptions-item>
        </ResourceDescriptions>
      </ResourceSection></ResourceCard>
      <ResourceCard class="detail-card"><ResourceSection title="发布设置">
        <ResourceDescriptions>
          <t-descriptions-item label="发布方式">自动发布</t-descriptions-item>
          <t-descriptions-item label="最早执行时间">{{ detail.not_before ? formatDateTime(detail.not_before) : detail.authorized_at ? "立即" : "立即（需单独确认执行）" }}</t-descriptions-item>
          <t-descriptions-item label="创建时间">{{ formatDateTime(detail.created_at) }}</t-descriptions-item>
          <t-descriptions-item label="修订">{{ detail.revision }}</t-descriptions-item>
        </ResourceDescriptions>
      </ResourceSection></ResourceCard>
      <ResourceCard class="detail-card"><ResourceSection title="执行历史">
        <PublishingExecutionHistory :task="detail" :attempts="attempts" />
      </ResourceSection></ResourceCard>
    </ResourceDetailPage>
    <ResourceOverlay :visible="videoVisible" title="成片预览" :width="960" :footer="!!videoError" form-actions @update:visible="closeVideo">
      <t-loading :loading="videoLoading"><video v-if="videoURL" class="publication-video" :src="videoURL" controls preload="metadata" @error="videoFailed" /><FeedbackAlert v-if="videoError" theme="error" :message="videoError" /></t-loading>
      <template #footer><t-button v-if="videoError" theme="default" variant="outline" @click="previewOutput">重试预览</t-button></template>
    </ResourceOverlay>
    <MediaPreview form-actions v-model:visible="preview" :images="previewImages" title="发布封面预览" />
    <PublishingRecoveryDialog v-model:visible="recoveryVisible" :task="recoveryTask" :busy="busy" :error="recoveryError" @submit="saveVerification" />
    <PublishingResourcePicker
      :visible="!!picker"
      :kind="picker"
      :category="form?.category_id"
      :account="account"
      @update:visible="!$event && (picker = '')"
      @selected="picked"
    />
  </div>
</template>
<style scoped>
.publication-content-cell {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}
.publication-thumbnail {
  width: 64px;
  height: 48px;
  object-fit: contain;
  border: 1px solid var(--wt-border);
  border-radius: 4px;
  background: var(--wt-color-bg-subtle);
}
.quick-publish {
  container-type: inline-size;
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
}
.quick-block {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.quick-copy-fields { display: flex; flex-direction: column; gap: 24px; }
.quick-block :deep(.t-input.t-is-disabled), .quick-block :deep(.t-input__inner:disabled) { color: var(--wt-text-secondary); -webkit-text-fill-color: var(--wt-text-secondary); background: var(--wt-color-bg-subtle); }
.quick-block h3 {
  margin: 0;
  display: flex;
  gap: 10px;
  align-items: center;
  font-size: 16px;
  font-weight: 500;
}
.quick-block h3 > span {
  font-size: 12px;
  color: var(--wt-color-primary);
  background: var(--wt-color-primary-light);
  width: 24px;
  height: 24px;
  border-radius: 4px;
  display: grid;
  place-items: center;
  font-weight: 500;
}
.quick-block label {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
  font-size: 13px;
  line-height: 20px;
  font-weight: 500;
}
.quick-block :deep(.t-textarea__inner), .quick-block :deep(.t-input) { font-size: 14px; font-weight: 400; }
.quick-field-heading { display: flex !important; flex-direction: row !important; align-items: center; gap: 6px !important; }
.quick-resource-grid.two { grid-template-columns: repeat(2, minmax(0, 1fr)); }
.quick-resource-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 20px 24px;
}
.quick-selected-asset {
  display: flex;
  align-items: center;
  gap: 16px;
  border: 1px solid var(--wt-border);
  padding: 12px 16px;
  background: var(--wt-color-bg-subtle);
  border-radius: 4px;
}
.quick-selected-asset > div {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.quick-selected-asset small,
.publication-sub {
  display: block;
  color: var(--wt-text-secondary);
  font-size: 12px;
  font-weight: 400;
  line-height: 20px;
}
.quick-cover-field {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.quick-cover-workspace { display: grid; grid-template-columns: 220px minmax(0, 1fr); align-items: start; gap: 24px; min-width: 0; }
.quick-cover-workspace > div { display: flex; flex-direction: column; align-items: flex-start; gap: 12px; min-width: 0; }
.quick-cover-workspace strong { font-size: 14px; font-weight: 400; overflow-wrap: anywhere; }
.publication-video { display: block; width: 100%; max-height: 65vh; background: var(--wt-color-bg-subtle); }
.publication-image-button { display: block; padding: 0; border: 1px solid var(--wt-border); border-radius: 4px; overflow: hidden; background: var(--wt-color-bg-subtle); cursor: zoom-in; max-width: 100%; }
.publication-image-button img { display: block; width: 220px; height: 165px; object-fit: contain; max-width: 100%; }
.publication-image-button span { display: block; padding: 8px; color: var(--wt-primary); font-size: 12px; }
.quick-selected-asset .publication-video { display: block; width: 100%; max-height: 65vh; background: var(--wt-color-bg-subtle); }
.publication-image-button { flex: none; }
.quick-selected-asset .publication-image-button img { width: 72px; height: 52px; }
.publication-cover-summary { display: flex; align-items: center; gap: 24px; flex-wrap: wrap; }
.publication-cover-summary > span { overflow-wrap: anywhere; }
.publishing-page .pagination-bar { border-top: 1px solid var(--wt-border); padding: 0; }
.publication-content {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.publication-content strong {
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.publication-content small {
  color: var(--wt-text-tertiary);
}
.publication-description {
  overflow-wrap: anywhere;
  white-space: pre-wrap;
}
@media (max-width: 720px) {
  .quick-resource-grid,
  .quick-resource-grid.two {
    grid-template-columns: 1fr;
  }
  .quick-cover-workspace { grid-template-columns: 1fr; }
  .quick-selected-asset { flex-wrap: wrap; }
  .quick-selected-asset > div { min-width: 0; flex: 1 1 160px; }
  .quick-selected-asset strong { overflow-wrap: anywhere; }
}
@container (max-width: 560px) {
  .quick-resource-grid, .quick-resource-grid.two, .quick-cover-workspace { grid-template-columns: 1fr; }
  .quick-selected-asset { flex-wrap: wrap; }
  .quick-selected-asset > div { flex: 1 1 100%; }
  .quick-selected-asset strong { overflow-wrap: anywhere; }
}
</style>
