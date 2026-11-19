<script setup>
import { computed, onMounted, ref } from "vue";
import { useRoute, useRouter, onBeforeRouteLeave } from "vue-router";
import ResourcePageHeader from "../../../shared/ui/resource/ResourcePageHeader.vue";
import ResourceStatGrid from "../../../shared/ui/resource/ResourceStatGrid.vue";
import ResourceAdvancedToggle from "../../../shared/ui/resource/ResourceAdvancedToggle.vue";
import ResourceDateRange from "../../../shared/ui/resource/ResourceDateRange.vue";
import { createdRangeParams } from "../../compose/createdRange.js";
import ResourceCard from "../../../shared/ui/resource/ResourceCard.vue";
import ResourceFilterBar from "../../../shared/ui/resource/ResourceFilterBar.vue";
import ResourceEditorPage from "../../../shared/ui/resource/ResourceEditorPage.vue";
import ResourceDetailPage from "../../../shared/ui/resource/ResourceDetailPage.vue";
import ResourceRowActions from "../../../shared/ui/resource/ResourceRowActions.vue";
import ResourceStatusBadge from "../../../shared/ui/resource/ResourceStatusBadge.vue";
import FeedbackAlert from "../../../shared/ui/feedback/FeedbackAlert.vue";
import { feedback } from "../../../shared/ui/feedback/index.js";
import { createPublicationClient } from "../../../shared/api/publication.js";
import { createComposeClient } from "../../../shared/api/compose.js";
import { createMediaAccountClient } from "../../../shared/api/mediaAccounts.js";
import { createFilesClient } from "../../../shared/api/files.js";
import { formatDateTime } from "../../../shared/utils/datetime.js";
import {
  definitivePublicationRejection,
  contentProblem,
  outputUnavailable,
} from "../eligibility.js";
import { PublicationStatus, publicationStatuses, attemptStageLabels } from "../status.js";
import PublishingResourcePicker from "../components/PublishingResourcePicker.vue";
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
const attemptHistory = ref([]);
const summary = ref(null),
  advanced = ref(false);
const metricItems = computed(() => [
  { key: "", label: "全部任务", value: summary.value?.all ?? "—" },
  {
    key: PublicationStatus.AwaitingHuman,
    label: "等待人工",
    value: summary.value?.[PublicationStatus.AwaitingHuman] ?? "—",
    tone: "warning",
  },
  { key: PublicationStatus.Running, label: "执行中", value: summary.value?.[PublicationStatus.Running] ?? "—" },
  {
    key: PublicationStatus.Uncertain,
    label: "结果待确认",
    value: summary.value?.[PublicationStatus.Uncertain] ?? "—",
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
  pageSize = ref(20),
  filters = ref(blankFilters()),
  applied = ref({ ...filters.value }),
  loading = ref(false),
  error = ref("");
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
const source = ref(false),
  detailReturn = ref("list");
const columns = [
  { colKey: "id", title: "任务 ID", width: 80 },
  { colKey: "content", title: "发布内容", width: 280 },
  { colKey: "platform", title: "平台 / 类目", width: 120 },
  { colKey: "account", title: "发布账号", width: 180 },
  { colKey: "status", title: "状态", width: 120 },
  { colKey: "created_at", title: "创建时间", width: 170 },
  { colKey: "op", title: "操作", width: 140, fixed: "right" },
];
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
    const [task, history] = await Promise.all([client.get(row.id), client.attempt(row.id)]);
    detail.value = task;
    attemptHistory.value = history;
    view.value = "detail";
  } catch (e) {
    error.value = e.message;
  } finally {
    loading.value = false;
  }
}
const phaseLabel = (value) => attemptStageLabels[value] || "等待执行器反馈";
async function startExecution() {
  const task = detail.value;
  if (busy.value || !task) return;
  busy.value = true;
  error.value = "";
  try {
    if (!(await feedback.confirmWarning({title:"确认执行发布",message:`将使用账号「${task.resources.account_name}」自动发布此任务。请确认文案、封面和声明；执行后不能再编辑。`,confirmText:"确认执行"}))) return;
    await client.start(task.id, {expected_revision: task.revision});
    await openDetail(task);
    feedback.toast.success("已授权执行，等待所属用户的 Agent 领取");
  } catch(e) {
    const message = e.message;
    // Resolve a lost response by reading state; never resend authorization.
    await openDetail(task);
    error.value = message;
  } finally { busy.value = false; }
}
async function edit(row) {
  busy.value = true;
  error.value = "";
  try {
    const v = await client.get(row.id);
    if (v.status !== PublicationStatus.Pending || v.current_attempt_id || v.authorized_at)
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
    schedule.value = v.not_before
      ? new Date(
          new Date(v.not_before) - new Date().getTimezoneOffset() * 60000,
        )
          .toISOString()
          .slice(0, 16)
      : "";
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
function picked(row) {
  if (picker.value === "account") {
    account.value = row;
    form.value.media_account_id = String(row.id);
    if (
      output.value &&
      outputUnavailable(output.value, form.value.category_id, row)
    ) {
      output.value = null;
      form.value.composite_output_id = "";
    }
    invalidate();
  } else if (picker.value === "output") {
    output.value = row;
    form.value.composite_output_id = String(row.id);
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
    const not_before = schedule.value
      ? new Date(schedule.value).toISOString()
      : null;
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
      output.value = data.output;
      form.value.category_id = String(data.output.category_id);
      form.value.composite_output_id = String(data.output.id);
    }
  } catch (e) {
    error.value = e.message;
  }
});
</script>
<template>
  <div class="wt-resource-page resource-visual-approved publishing-page">
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
      <ResourceCard>
        <ResourceFilterBar
          ><label class="wt-filter-field wt-filter-field--wide"
            ><span class="wt-filter-field__label">综合搜索</span><t-input
              v-model="filters.search"
              aria-label="任务搜索"
              placeholder="作品描述 / 任务 ID"
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
        <div class="table-scroll-wrap">
          <t-table
            class="wt-resource-table"
            :columns="columns"
            :data="rows"
            row-key="id"
            :loading="loading"
            :scroll="{ x: '1070px' }"
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
            <template #platform="{row}">{{({baidu:"百度（百家号）",bilibili:"B站",xiaohongshu:"小红书"})[row.platform] || row.platform}}<small class="publication-sub">{{categoryName(row.category_id)}}</small></template><template #account="{ row }"
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
            <template #op="{ row }"
              ><ResourceRowActions
                ><t-button
                  variant="text"
                  size="small"
                  @click="
                    detailReturn = 'list';
                    openDetail(row);
                  "
                  >详情</t-button
                ><t-button
                  v-if="
                    row.status === PublicationStatus.Pending &&
                    !row.current_attempt_id &&
                    !row.authorized_at
                  "
                  size="small"
                  variant="text"
                  @click="edit(row)"
                  >编辑</t-button
                ></ResourceRowActions
              ></template
            >
          </t-table>
        </div>
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
              >发布平台 *<t-select
                value="baidu"
                :options="[{ label: '百度（百家号）', value: 'baidu' }]"
                disabled /></label
            ><label
              >发布类目 *<t-select
                :value="form.category_id"
                aria-label="发布类目"
                :options="categoryOptions"
                :disabled="!!editing || frozen"
                @change="changeCategory" /></label
            ><label
              >发布账号 *<t-input
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
            >
          </div>
        </section>
        <section v-if="output" class="resource-editor-section quick-block">
          <h3><span>02</span>发布内容</h3>
          <label
            >作品描述 *<t-textarea
              v-model="form.content.description"
              aria-label="作品描述"
              :maxlength="1000"
              :autosize="{ minRows: 3, maxRows: 5 }"
              :disabled="frozen"
              @change="invalidate"
          /></label>
          <div class="quick-cover-field">
            <label>发布封面 *</label>
            <p class="publication-sub">使用现成封面</p>
            <div class="quick-cover-workspace">
              <img
                v-if="cover?.resource_path"
                :src="files.resourceURL(cover)"
                alt="当前发布封面"
              />
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
            >创作声明 *<t-select
              v-model="form.content.declaration"
              aria-label="创作声明"
              :options="[{ label: '无需声明', value: '无需声明' }]"
              placeholder="按内容事实选择"
              :disabled="frozen"
              @change="invalidate"
          /></label>
        </section>
        <section v-if="output" class="resource-editor-section quick-block">
          <h3><span>03</span>发布设置</h3>
          <label
            >最早执行时间<input
              v-model="schedule"
              type="datetime-local"
              aria-label="最早执行时间"
              :disabled="frozen"
              @input="invalidate"
          /></label>
          <p class="publication-sub">
            留空表示立即；创建任务后仍需单独确认执行。
          </p>
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
      <ResourceCard
        ><h3>
          任务 {{ detail.id }}
          <ResourceStatusBadge :label="statusLabel(detail.status)" />
        </h3>
        <p v-if="detail.attempt_count === 0">尚未执行</p>
        <div class="detail-grid">
          <label>发布平台<strong>百度（百家号）</strong></label
          ><label
            >类目<strong>{{ categoryName(detail.category_id) }}</strong></label
          ><label
            >发布账号<strong>{{ detail.resources.account_name || "—" }}</strong
            ><small>真实 UID：{{ detail.platform_account_uid }}</small></label
          ><label
            >成片<strong>{{
              detail.resources.output_name || detail.composite_output_id
            }}</strong></label
          >
        </div></ResourceCard
      >
      <ResourceCard
        ><h3>发布内容</h3>
        <p class="publication-description">{{ detail.content.description }}</p>
        <div class="quick-cover-workspace">
          <img
            v-if="detail.resources.cover_resource_path"
            :src="
              files.resourceURL({
                resource_path: detail.resources.cover_resource_path,
              })
            "
            alt="最终发布封面"
          />
          <p>
            封面：{{
              detail.resources.cover_name || detail.content.cover_file_id
            }}
          </p>
        </div>
        <p>话题：{{ detail.content.topics || "—" }}</p>
        <p>创作声明：{{ detail.content.declaration }}</p></ResourceCard
      >
      <ResourceCard
        ><h3>执行历史</h3>
        <p v-if="!attemptHistory.length">任务尚未执行</p>
        <p v-if="detail.authorized_at">已确认执行：{{formatDateTime(detail.authorized_at)}} · 仅由账号所属用户的 Agent 执行</p>
        <p v-if="detail.status === PublicationStatus.Uncertain">结果尚未确认，请保留现场，避免重复发布。</p>
        <p v-if="detail.status === PublicationStatus.Reviewing">平台已接收，正在审核；尚未确认为公开作品。</p>
        <p v-if="detail.failure_message">平台反馈：{{detail.failure_message}}</p>
        <section v-for="attempt in attemptHistory" :key="attempt.id" class="resource-editor-section">
          <h4>尝试 {{attempt.attempt_no}} · {{phaseLabel(attempt.stage)}}</h4>
          <p>开始：{{formatDateTime(attempt.started_at)}} · 最近反馈：{{formatDateTime(attempt.heartbeat_at)}}</p>
          <p v-for="step in attempt.steps" :key="step.seq">{{formatDateTime(step.at)}} · {{phaseLabel(step.report.stage)}}</p>
          <p v-if="attempt.error_message">{{attempt.error_message}}</p>
        </section>
        <t-button variant="text" :loading="loading" @click="openDetail(detail)">刷新进展</t-button>
        <p>
          创建时间：{{ formatDateTime(detail.created_at) }} · 修订
          {{ detail.revision }}
        </p>
        <p>
          最早执行时间：{{
            detail.not_before
              ? formatDateTime(detail.not_before)
              : detail.authorized_at ? "立即" : "立即（需单独确认执行）"
          }}
        </p></ResourceCard
      >
      <template #footer>
        <t-button v-if="detail.status === PublicationStatus.Pending && !detail.current_attempt_id && !detail.authorized_at" theme="primary" :loading="busy" @click="startExecution">确认执行</t-button>
        <t-button
          v-if="
            detail.status === PublicationStatus.Pending &&
            !detail.current_attempt_id &&
            !detail.authorized_at
          "
          :disabled="busy"
          @click="edit(detail)"
          >编辑</t-button
        ></template
      >
    </ResourceDetailPage>
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
  background: var(--wt-bg-subtle);
}
.quick-publish {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
}
.quick-block {
  display: flex;
  flex-direction: column;
  gap: 20px;
}
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
  font-size: 14px;
}
.quick-resource-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 20px;
}
.quick-selected-asset {
  display: flex;
  align-items: center;
  gap: 16px;
  border: 1px solid var(--wt-border);
  padding: 16px;
  background: var(--wt-bg-page);
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
  color: var(--wt-text-tertiary);
  font-size: 13px;
}
.quick-cover-field {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.quick-cover-workspace {
  display: flex;
  gap: 20px;
  align-items: center;
  padding: 16px;
  border: 1px solid var(--wt-border);
  border-radius: 6px;
  background: var(--wt-bg-page);
}
.quick-cover-workspace img {
  width: 180px;
  max-height: 130px;
  object-fit: contain;
}
.quick-cover-workspace > div {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
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
  white-space: pre-wrap;
}
.detail-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20px;
}
.detail-grid label {
  display: flex;
  flex-direction: column;
  gap: 6px;
  color: var(--wt-text-tertiary);
}
.detail-grid strong {
  font-weight: 400;
  color: var(--wt-text-primary);
}
input[type="datetime-local"] {
  min-height: 36px;
  padding: 0 12px;
  border: 1px solid var(--wt-border);
  border-radius: 4px;
  font: inherit;
  color: var(--wt-text-primary);
  background: var(--wt-bg-card);
}
@media (max-width: 720px) {
  .quick-resource-grid,
  .detail-grid {
    grid-template-columns: 1fr;
  }
  .quick-cover-workspace {
    align-items: flex-start;
    flex-wrap: wrap;
  }
}
</style>
