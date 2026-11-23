<script setup>
import { computed, onMounted, onBeforeUnmount, ref, watch, nextTick } from "vue";
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
import BusinessStatus from "../../../shared/ui/BusinessStatus.vue";
import FormError from "../../../shared/ui/feedback/FormError.vue";
import FeedbackAlert from "../../../shared/ui/feedback/FeedbackAlert.vue";
import { feedback } from "../../../shared/ui/feedback/index.js";
import { createPublicationClient } from "../../../shared/api/publication.js";
import { createComposeClient } from "../../../shared/api/compose.js";
import { createMediaAccountClient } from "../../../shared/api/mediaAccounts.js";
import { createSessionClient } from "../../../shared/api/session.js";
import { canRunLocalAgent } from "../../../utils.js";
import { createImageClient } from "../../../shared/api/image.js";
import { handlePublicationHuman } from "../../../shared/api/publicationRun.js";
import { createPublicationDraftClient } from "../../../shared/api/publicationDraft.js";
import { choosePublicationDraft, capturePublicationDraft } from "../draftRecovery.js";
import { accountUnavailable } from "../eligibility.js";
import { createFilesClient } from "../../../shared/api/files.js";
import {
  definitivePublicationRejection,
  contentIssue,
  publicationTextLength,
  outputUnavailable,
} from "../eligibility.js";
import { PublicationStatus, publicationStatuses } from "../status.js";
import { publicationTitleForOutput } from "../title.js";
import PublicationContentGenerator from "../components/PublicationContentGenerator.vue";
import PublishingCoverEditor from "../components/PublishingCoverEditor.vue";
import { emptyCoverConfig, coverConfigFromReference, coverReference, coverMatchesRatio } from "../coverPreparation.js";
import PublishingExecutionHistory from "../components/PublishingExecutionHistory.vue";
import PublishingResourcePicker from "../components/PublishingResourcePicker.vue";
import PublishingRecoveryDialog from "../components/PublishingRecoveryDialog.vue";
import PublishingTerminationDialog from "../components/PublishingTerminationDialog.vue";
import AccountSummaryDialog from "../../accounts/components/AccountSummaryDialog.vue";
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
const currentUser = ref(null);
const canPublish = computed(() => canRunLocalAgent(currentUser.value));
async function publicationUser() {
  const user = await createSessionClient().me();
  if (disposed) throw new Error("页面已离开，请重新打开任务列表");
  currentUser.value = user;
  if (!canRunLocalAgent(user)) throw new Error("管理员仅可查看和管理云端记录，不承担发布执行");
  return user;
}
const attempts = ref([]);
const accountSummaryVisible = ref(false), accountSummaryId = ref("");
function viewAccount(id) {
  if (!id) return;
  accountSummaryId.value = String(id);
  accountSummaryVisible.value = true;
}
const humanVisible=ref(false),humanTask=ref(null),humanUserId=ref(""),humanError=ref("");
const recoveryVisible=ref(false),recoveryTask=ref(null),recoveryError=ref("");
const terminationVisible = ref(false), terminationTask = ref(null), terminationAction = ref(""), terminationError = ref(""), terminationSubmitted = ref(false);
const summary = ref(null),
  advanced = ref(false);
const metricItems = computed(() => [
  { key: "", label: "全部任务", value: summary.value?.all ?? "—" },
  {
    key: PublicationStatus.AwaitingHuman,
    label: "等待人工",
    value: summary.value?.awaiting_human ?? "—",
    tone: "warning",
  },
  { key: PublicationStatus.Running, label: "执行中", value: summary.value?.running ?? "—" },
  {
    key: PublicationStatus.Uncertain,
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
const coverConfig = ref(emptyCoverConfig());
const portraitCover=ref(null), portraitCoverConfig=ref(emptyCoverConfig()), coverTarget=ref("horizontal");
const generationContext = computed(() => ({ rowKey: form.value?.request_key || String(editing.value?.id || ''), platform: form.value?.platform, categoryId: form.value?.category_id, outputId: form.value?.composite_output_id, identityKey: JSON.stringify([form.value?.media_account_id,coverConfig.value.method,coverConfig.value.template_id,coverConfig.value.published_revision]) }));
const generationValues = computed(() => ({ description: form.value?.content.description || '', topics: form.value?.content.topics || '', cover_title: coverConfig.value.title || '' }));
function adoptGeneratedContent(field,value) {
 if (frozen.value || !form.value) return;
 if (field === 'cover_title') { updateCoverConfig({ ...coverConfig.value, title: value }); return; }
 if (!['description','topics'].includes(field) || form.value.content[field] === value) return;
 form.value.content[field] = value; invalidate();
}

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
  if (videoVisible.value && videoURL.value && event.target.src === videoURL.value) {
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
  { colKey: "op", title: "操作", width: opColumnWidth(["执行", "编辑", "终止", "详情"], { variant: "text", primary: true }), fixed: "right" },
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
    JSON.stringify({ form: form.value, schedule: schedule.value, cover_config:coverConfig.value,portrait_cover_config:portraitCoverConfig.value }) !==
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
    portrait_cover_file_id: "",
    topics: "",
    declaration: "",
    confirmed: false,
  },
});
let loadSequence = 0;
async function load({ background = false } = {}) {
  const n = ++loadSequence;
  if (!background) {
    loading.value = true;
    error.value = "";
  }
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
    error.value = "";
  } catch (e) {
    if (n === loadSequence) error.value = e.message;
  } finally {
    if (n === loadSequence && !background) loading.value = false;
  }
}
function query() {
  applied.value = { ...filters.value };
  page.value = 1;
  load();
}
function invalidate() {
  formError.value = "";
  validationField.value = "";
  if (form.value) form.value.content.confirmed = false;
}
async function changeCategory(category) {
  if (frozen.value || String(category) === String(form.value.category_id)) return;
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
  draftResourceIssue.value = {};
  form.value = emptyForm();
  form.value.category_id = category;
  account.value = null;
  output.value = null;
  cover.value = null;
  portraitCover.value=null;
  portraitCoverConfig.value=emptyCoverConfig();
  coverConfig.value = emptyCoverConfig();
  schedule.value = "";
}
function initializeCreate() {
  source.value = false;
  editing.value = null;
  detail.value = null;
  attempts.value = [];
  account.value = null;
  output.value = null;
  cover.value = null;
  portraitCover.value=null;
  portraitCoverConfig.value=emptyCoverConfig();
  coverConfig.value = emptyCoverConfig();
  schedule.value = "";
  form.value = emptyForm();
  formError.value = "";
  validationField.value = "";
  pendingRequest.value = null;
  baseline.value = JSON.stringify({ form: form.value, schedule: "", cover_config:coverConfig.value,portrait_cover_config:portraitCoverConfig.value });
  view.value = "form";
}
const validationField = ref("");
const draftPersistent = ref(false), draftReady = ref(false);
const draftResourceIssue = ref({}), draftConflict = ref(false);
let draftClient = null;
function draftPayload() {
  return capturePublicationDraft(form.value, schedule.value, cover.value, pendingRequest.value, coverConfig.value,portraitCover.value,portraitCoverConfig.value);
}
async function openCreate(sourceOutput = null, copyInput = null, copyUserId = null) {
  if (busy.value) return false;
  busy.value = true;
  draftReady.value = false;
  error.value = "";
  try {
    const user = await publicationUser();
    if (disposed) return false;
    if (copyInput && String(user.id) !== String(copyUserId)) throw new Error("登录账号已变化，已丢弃原账号的复制内容，请重新打开任务列表");
    const verifyOwner = async () => {
      const current = await publicationUser();
      if (disposed) throw new Error("页面已离开，已丢弃迟到内容");
      if (String(current.id) !== String(user.id)) throw new Error("登录账号已变化，原草稿和复制内容未写入，请重新打开任务列表");
    };
    draftClient = createPublicationDraftClient({ userId: user.id });
    draftPersistent.value = draftClient.persistent;
    const previous = await draftClient.get();
    let choice = null;
    if (previous.payload) {
      choice = await choosePublicationDraft(previous.saved_at);
      if (!choice) return false;
    }
    await verifyOwner();
    if (choice === "restart") {
      await draftClient.clear();
      await verifyOwner();
    }
    initializeCreate();
    draftResourceIssue.value = {};
    draftConflict.value = false;
    if (choice === "continue") await restoreDraft(previous.payload);
    else if (copyInput) {
      form.value.platform = copyInput.platform;
      form.value.category_id = String(copyInput.category_id);
      form.value.content.description = copyInput.content.description;
      form.value.content.topics = copyInput.content.topics;
      form.value.content.declaration = copyInput.content.declaration;
      feedback.toast.info("已复制文案、话题和声明；请重新选择账号、成片和封面，并核对确认内容。");
    } else if (sourceOutput?.id) {
      source.value = true;
      setOutput(sourceOutput);
      form.value.category_id = String(sourceOutput.category_id);
    }
    baseline.value = JSON.stringify({ form: form.value, schedule: schedule.value, cover_config:coverConfig.value,portrait_cover_config:portraitCoverConfig.value });
    draftReady.value = true;
    return true;
  } catch (e) {
    if (disposed) return false;
    error.value = e.message || "无法读取续编，原输入已保留，请重试";
    if (view.value === "form") formError.value = error.value;
    return false;
  } finally { busy.value = false; }
}
async function restoreDraft(payload) {
  if (!payload?.form?.content || !payload.form.request_key || payload.form.platform !== "baidu") throw new Error("续编输入无法读取，请保留原草稿后核对");
  form.value = capturePublicationDraft(payload.form, payload.schedule, null, null).form;
  coverConfig.value = payload.cover_config || (form.value.content.cover_render ? coverConfigFromReference(form.value.content.cover_render) : emptyCoverConfig());
  portraitCoverConfig.value=payload.portrait_cover_config || emptyCoverConfig();
  schedule.value = String(payload.schedule || "");
  pendingRequest.value = payload.pending_request || null;
  // Restoring user input never re-confirms resources or replaces human text.
  if (!pendingRequest.value) form.value.content.confirmed = false;
  const results = await Promise.allSettled([
    form.value.media_account_id ? accounts.get(form.value.media_account_id) : null,
    form.value.composite_output_id ? compose.getComposeOutput(form.value.composite_output_id) : null,
    payload.image_asset_id ? createImageClient().getImageAsset(payload.image_asset_id) : null,
    form.value.content.cover_render ? createImageClient().getCover(form.value.content.cover_file_id) : null,
  ]);
  const [a, o, c, artwork] = results.map(result => result.status === "fulfilled" ? result.value : null);
  account.value = a;
  output.value = o?.output || null;
  if (form.value.media_account_id) draftResourceIssue.value.account = !a ? "原账号不可用，请恢复原账号或重新选择" : accountUnavailable(a, form.value.category_id);
  if (form.value.composite_output_id) draftResourceIssue.value.output = !output.value ? "原成片不可用，填写内容已保留" : outputUnavailable(output.value, form.value.category_id, a);
  if (form.value.content.cover_file_id) {
    cover.value = { id: form.value.content.cover_file_id, image_asset_id: String(payload.image_asset_id || ""), origin: String(payload.cover_origin || ""), output_id: String(payload.cover_output_id || ""), name: "原封面暂不可用" };
    const thumbnail = output.value?.thumbnail;
    if (artwork && JSON.stringify(coverReference(artwork)) === JSON.stringify(form.value.content.cover_render)) {
      cover.value = {...artwork.file,origin:"template"};
      coverConfig.value = payload.cover_config || coverConfigFromReference(form.value.content.cover_render, artwork.snapshot);
    } else if (payload.cover_origin === "output" && String(payload.cover_output_id) === form.value.composite_output_id && thumbnail && String(thumbnail.file_id) === form.value.content.cover_file_id) {
      cover.value = { ...thumbnail, id: String(thumbnail.file_id), origin: "output", output_id: form.value.composite_output_id, name: "成片封面" };
    } else if (c && String(c.file_id) === form.value.content.cover_file_id && c.enabled && c.allow_as_cover && (!c.category_ids?.length || c.category_ids.map(String).includes(form.value.category_id))) {
      cover.value = { ...c, id: c.file_id, image_asset_id: c.id };
    } else {
      draftResourceIssue.value.cover = "原封面不可用，请恢复原图片或重新选择";
    }
  }
  await restorePortraitCover(payload);
  feedback.toast.info("已恢复填写，请重新核对内容；不可用资源已在对应位置标明。");
}
async function restorePortraitCover(payload) {
  portraitCover.value = null;
  const id = form.value.content.portrait_cover_file_id;
  if (!id) return;
  try {
    const ref = form.value.content.portrait_cover_render;
    if (ref) {
      const item = await createImageClient().getCover(id);
      if (JSON.stringify(coverReference(item)) !== JSON.stringify(ref))
        throw new Error();
      portraitCover.value = { ...item.file, origin: "template" };
      portraitCoverConfig.value =
        payload.portrait_cover_config ||
        coverConfigFromReference(ref, item.snapshot);
    } else if (
      payload.portrait_cover_origin === "output" &&
      String(output.value?.thumbnail?.file_id) === id
    ) {
      portraitCover.value = {
        ...output.value.thumbnail,
        id,
        origin: "output",
        output_id: form.value.composite_output_id,
      };
    } else {
      const item = await createImageClient().getImageAsset(
        payload.portrait_image_asset_id,
      );
      if (
        String(item.file_id) !== id ||
        !item.enabled ||
        !item.allow_as_cover ||
        (item.category_ids?.length &&
          !item.category_ids.map(String).includes(form.value.category_id))
      )
        throw new Error();
      portraitCover.value = {
        ...item,
        id,
        item_asset_id: item.id,
        image_asset_id: item.id,
      };
    }
    if (!coverMatchesRatio(portraitCover.value, 3, 4)) throw new Error();
  } catch {
    draftResourceIssue.value.portrait_cover =
      "原竖版封面不可用，请重新选择或生成";
  }
}
watch([form, schedule, pendingRequest, coverConfig,portraitCoverConfig], () => {
  if (draftReady.value && draftClient && view.value === "form" && !editing.value && !busy.value) {
    const hasInput = form.value.category_id || form.value.media_account_id || form.value.composite_output_id || form.value.content.description || schedule.value;
    draftClient.remember(hasInput ? draftPayload() : null);
  }
}, { deep: true });
async function saveDraft() {
  if (busy.value || !draftReady.value || editing.value) return false;
  busy.value = true;
  formError.value = "";
  validationField.value = "";
  try {
    await draftClient.save(draftPayload());
    baseline.value = JSON.stringify({ form: form.value, schedule: schedule.value, cover_config:coverConfig.value,portrait_cover_config:portraitCoverConfig.value });
    feedback.toast.success(draftPersistent.value ? "草稿已保存，重启后可继续；不占用成片" : "已暂存当前填写；刷新、关闭或退出后清空");
    return true;
  } catch (e) { draftConflict.value = e.code === "draft_revision_conflict"; formError.value = e.message || "保存失败，填写内容已保留"; return false; }
  finally { busy.value = false; }
}
async function reloadDraft() {
  if (busy.value || editing.value || !draftClient) return;
  if (!(await feedback.confirmWarning({ title: "读取最新草稿？", message: "最新草稿会替换当前填写，当前未保存修改将丢失。", confirmText: "读取最新草稿", cancelText: "保留当前填写" }))) return;
  busy.value = true;
  formError.value = "";
  validationField.value = "";
  try {
    const current = await draftClient.get({ reload: true });
    draftResourceIssue.value = {};
    if (current.payload) await restoreDraft(current.payload);
    else initializeCreate();
    draftConflict.value = false;
    baseline.value = JSON.stringify({ form: form.value, schedule: schedule.value, cover_config:coverConfig.value,portrait_cover_config:portraitCoverConfig.value });
  } catch (e) { formError.value = e.message || "读取失败，填写内容已保留"; }
  finally { busy.value = false; }
}
async function clearCreatedDraft() {
  if (!draftClient || editing.value) return "";
  let warning = "";
  try { await draftClient.clear(); }
  catch (e) { warning = `任务已创建；续编未清除，请重新打开后核对原任务。${e.message || ""}`; }
  draftReady.value = false;
  return warning;
}

async function openDetail(row) {
  loading.value = true;
  error.value = "";
  try {
    const [task, history] = await Promise.all([client.get(row.id), client.attempt(row.id)]);
    detail.value = task;
    attempts.value = history;
    view.value = "detail";
  } catch (e) {
    error.value = e.message;
  } finally {
    loading.value = false;
  }
}
const editable = (task) => task.status === PublicationStatus.Pending && !task.current_attempt_id && !task.authorized_at && !task.cancel_requested_at;
const retryable = (task) => task.status === PublicationStatus.Failed && !task.cancel_requested_at;
const cancellable = (task) => task.can_cancel === true && !task.cancel_requested_at;
const abandonable = (task) => task.status === PublicationStatus.Failed && !!task.current_attempt_id;
const copyable = (task) => [PublicationStatus.Failed, PublicationStatus.Published, PublicationStatus.Cancelled, PublicationStatus.Abandoned].includes(task.status);
async function copyTask(row) {
  if (busy.value) return;
  busy.value = true; error.value = '';
  let input, userId;
  try {
    userId = String((await publicationUser()).id);
    if (disposed) return;
    input = await client.copy(row.id, row.revision);
  }
  catch (e) { if (!disposed) { await load(); error.value = e.message; } }
  finally { busy.value = false; }
  if (input && !disposed) await openCreate(null, input, userId);
}

async function openTermination(row, action) {
  if (busy.value) return;
  busy.value = true; error.value = '';
  try {
    const task = await client.get(row.id);
    if (task.revision !== row.revision || !(action === 'cancel' ? cancellable(task) : action === 'abandon' && abandonable(task))) throw new Error('任务内容或状态已变化，请核对更新后的列表再操作');
    terminationTask.value = JSON.parse(JSON.stringify(task));
    terminationAction.value = action; terminationError.value = ''; terminationSubmitted.value = false; terminationVisible.value = true;
  } catch (e) { await load(); error.value = e.message; }
  finally { busy.value = false; }
}
async function saveTermination(input) {
  if (busy.value || terminationSubmitted.value || !terminationTask.value) return;
  busy.value = true; terminationError.value = ''; terminationSubmitted.value = true;
  try {
    const action = terminationAction.value;
    const result = await client[action](terminationTask.value.id, input);
    terminationVisible.value = false;
    await load();
    feedback.toast.success(action === 'abandon' ? '已安全放弃，成片占用已释放' : result.status === PublicationStatus.Cancelled ? '已终止任务，保留成片占用' : '已请求安全停止，等待执行端确认；请查看任务结果');
  } catch (e) {
    await load();
    terminationError.value = e.message + '；请保留依据，返回任务列表查询核对结果。';
  } finally { busy.value = false; }
}
async function startExecution(row) {
  if (busy.value) return;
  busy.value = true;
  error.value = "";
  try {
    await publicationUser();
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
async function openHumanScene(row) {
  if (busy.value) return;
  busy.value=true;error.value='';
  try {
    const [task,user]=await Promise.all([client.get(row.id),publicationUser()]);
    if (disposed) return;
    if (task.status!==PublicationStatus.AwaitingHuman || !task.current_attempt_id) throw new Error('任务状态已变化，请刷新后核对');
    await handlePublicationHuman({task,userId:String(user.id)});
    if (disposed) return;
    humanTask.value=task;humanUserId.value=String(user.id);humanError.value='';humanVisible.value=true;
  } catch(e) {if (!disposed) error.value=e.message || '原发布现场不可用，请刷新后核实原任务';}
  finally {busy.value=false;}
}
async function continueHumanScene() {
  if (busy.value || !humanTask.value) return;
  busy.value=true;humanError.value='';
  try {
    const user=await publicationUser();
    if (disposed) return;
    if (String(user.id)!==humanUserId.value) throw new Error('登录身份已变化，请关闭后重新打开原任务');
    await handlePublicationHuman({task:humanTask.value,userId:humanUserId.value,resume:true});
    if (disposed) return;
    humanVisible.value=false;
    feedback.toast.success('已继续原发布尝试，请在任务中查看后续结果');
    await load();
    if (view.value==='detail' && detail.value?.id===humanTask.value.id) detail.value=await client.get(humanTask.value.id);
  } catch(e) {if (!disposed) humanError.value=e.message || '未收到继续确认，请保留原现场后重试';}
  finally {busy.value=false;}
}
async function openVerification(row) {
  if (busy.value) return;
  busy.value = true;
  error.value = "";
  try {
    const task = await client.get(row.id);
    if (![PublicationStatus.Uncertain,PublicationStatus.Reviewing].includes(task.status)) throw new Error("任务状态已变化，请核对列表后操作");
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
  if (busy.value) return;
  busy.value = true;
  error.value = "";
  try {
    await publicationUser();
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
    draftReady.value = false;
    draftResourceIssue.value = {};
    draftConflict.value = false;
    editing.value = v;
    account.value = a;
    output.value = o.output;
    cover.value = {
      id: v.content.cover_file_id,
      name: v.resources.cover_name,
      resource_path: v.resources.cover_resource_path,
    };
    coverConfig.value = v.content.cover_render ? coverConfigFromReference(v.content.cover_render,v.resources.cover_render) : emptyCoverConfig();
    if(v.content.cover_render) cover.value.origin="template";
    portraitCover.value={id:v.content.portrait_cover_file_id,name:v.resources.portrait_cover_name,resource_path:v.resources.portrait_cover_resource_path};
    portraitCoverConfig.value=v.content.portrait_cover_render ? coverConfigFromReference(v.content.portrait_cover_render,v.resources.portrait_cover_render) : emptyCoverConfig();
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
      cover_config:coverConfig.value,portrait_cover_config:portraitCoverConfig.value,
    });
    formError.value = "";
  validationField.value = "";
    pendingRequest.value = null;
    view.value = "form";
  } catch (e) {
    error.value = e.message;
  } finally {
    busy.value = false;
  }
}
function setOutput(row) {
  const backgroundChanged=String(output.value?.id || "")!==String(row?.id || "") || String(output.value?.thumbnail?.file_id || "")!==String(row?.thumbnail?.file_id || "");
  form.value.content.description = publicationTitleForOutput(form.value.content.description, output.value, row);
  output.value = row;
  if(portraitCoverConfig.value.method==="template" && portraitCoverConfig.value.background_mode==="business" && portraitCoverConfig.value.background_source==="output" && backgroundChanged)clearPortraitCover();
  else if(portraitCover.value?.origin==="output" && backgroundChanged)clearPortraitCover();
  form.value.composite_output_id = row ? String(row.id) : "";
  if (coverConfig.value.method === "template" && coverConfig.value.background_mode === "business" && coverConfig.value.background_source === "output" && backgroundChanged) {
    clearRenderedCover();
  } else if (coverConfig.value.method === "image" && (!cover.value || cover.value.origin === "output")) {
    const thumbnail = row?.thumbnail;
    cover.value = thumbnail?.file_id && coverMatchesRatio(thumbnail,16,9) ? { ...thumbnail, id: String(thumbnail.file_id), origin: "output", output_id: String(row.id), name: "成片封面" } : null;
    form.value.content.cover_file_id = cover.value ? cover.value.id : "";
    delete draftResourceIssue.value.cover;
  }
}
function clearRenderedCover() {
  cover.value=null;
  form.value.content.cover_file_id="";
  delete form.value.content.cover_render;
  delete draftResourceIssue.value.cover;
  invalidate();
}
function updateCoverConfig(config) {
  if(frozen.value || JSON.stringify(config)===JSON.stringify(coverConfig.value))return;
  coverConfig.value=config;
  clearRenderedCover();
  if(config.method==="image")useOutputCover();
}
function useOutputCover() {
  if(frozen.value || !output.value?.thumbnail?.file_id || !coverMatchesRatio(output.value.thumbnail,16,9))return;
  const thumbnail=output.value.thumbnail;
  const previous=reviewedInput();
  cover.value={...thumbnail,id:String(thumbnail.file_id),origin:"output",output_id:String(output.value.id),name:"成片封面"};
  form.value.content.cover_file_id=cover.value.id;
  delete form.value.content.cover_render;
  delete draftResourceIssue.value.cover;
  if(reviewedInput()!==previous)invalidate();
}
function useRenderedCover(artwork) {
  if(frozen.value || coverConfig.value.method!=="template")return;
  cover.value={...artwork.file,origin:"template"};
  form.value.content.cover_file_id=String(artwork.file.id);
  form.value.content.cover_render=coverReference(artwork);
  delete draftResourceIssue.value.cover;
  invalidate();
}
function reviewedInput() {
  return JSON.stringify({ platform: form.value.platform, category: form.value.category_id, account: form.value.media_account_id, output: form.value.composite_output_id, description: form.value.content.description, topics: form.value.content.topics, declaration: form.value.content.declaration, cover: form.value.content.cover_file_id, cover_render:form.value.content.cover_render,portrait_cover:form.value.content.portrait_cover_file_id,portrait_cover_render:form.value.content.portrait_cover_render });
}
function clearPortraitCover() {
  portraitCover.value = null;
  form.value.content.portrait_cover_file_id = "";
  delete form.value.content.portrait_cover_render;
  delete draftResourceIssue.value.portrait_cover;
  invalidate();
}
function updatePortraitConfig(config) {
  if (
    frozen.value ||
    JSON.stringify(config) === JSON.stringify(portraitCoverConfig.value)
  )
    return;
  portraitCoverConfig.value = config;
  clearPortraitCover();
  if (config.method === "image") usePortraitOutput();
}
function usePortraitOutput() {
  const image = output.value?.thumbnail;
  if (frozen.value || !image?.file_id || !coverMatchesRatio(image, 3, 4))
    return;
  portraitCover.value = {
    ...image,
    id: String(image.file_id),
    origin: "output",
    output_id: String(output.value.id),
  };
  form.value.content.portrait_cover_file_id = String(image.file_id);
  delete form.value.content.portrait_cover_render;
  delete draftResourceIssue.value.portrait_cover;
  invalidate();
}
function usePortraitArtwork(item) {
  if (frozen.value || portraitCoverConfig.value.method !== "template") return;
  portraitCover.value = { ...item.file, origin: "template" };
  form.value.content.portrait_cover_file_id = String(item.file.id);
  form.value.content.portrait_cover_render = coverReference(item);
  delete draftResourceIssue.value.portrait_cover;
  invalidate();
}
function chooseCoverImage(target) {
  coverTarget.value = target;
  picker.value = "image";
}
function picked(row) {
  const previousInput = reviewedInput();
  if (picker.value === "account") {
    delete draftResourceIssue.value.account;
    account.value = row;
    form.value.media_account_id = String(row.id);
    if (
      output.value &&
      outputUnavailable(output.value, form.value.category_id, row)
    ) {
      setOutput(null);
    }
  } else if (picker.value === "output") {
    delete draftResourceIssue.value.output;
    setOutput(row);
  } else if(coverTarget.value==="portrait") {
    if(!coverMatchesRatio(row,3,4)){feedback.toast.error("请选择3:4且不超过5MB的竖版封面");return;}
    portraitCover.value=row;
    portraitCoverConfig.value={...portraitCoverConfig.value,method:"image"};
    form.value.content.portrait_cover_file_id=String(row.id);
    delete form.value.content.portrait_cover_render;
    delete draftResourceIssue.value.portrait_cover;
  } else {
    if(!coverMatchesRatio(row,16,9)){feedback.toast.error("请选择16:9且不超过5MB的横版封面");return;}
    delete draftResourceIssue.value.cover;
    delete form.value.content.cover_render;
    coverConfig.value = {...coverConfig.value,method:"image"};
    cover.value = row;
    form.value.content.cover_file_id = String(row.id);
  }
  if (reviewedInput() !== previousInput) invalidate();
  else formError.value = "";
  picker.value = "";
}
async function submit() {
  if (busy.value) return;
  formError.value = "";
  validationField.value = "";
  const resourceProblem = !editing.value && Object.values(draftResourceIssue.value).find(Boolean);
  const issue = !pendingRequest.value && contentIssue(form.value);
  if (!pendingRequest.value && (resourceProblem || issue)) {
    // Resource restoration already shows the exact reason next to that resource.
    if (!resourceProblem) { formError.value = issue.message; validationField.value = issue.field; }
    return;
  }
  busy.value = true;
  let sent = false;
  const wasPending = !!pendingRequest.value;
  try {
    if (wasPending && !editing.value) {
      let existing = null;
      try { existing = await client.findByRequest(pendingRequest.value.request_key); }
      catch (e) { if (e.errcode !== 18002) throw e; }
      if (existing) {
        const cleanupWarning = await clearCreatedDraft();
        error.value = cleanupWarning;
        pendingRequest.value = null;
        attempts.value = [];
        detail.value = existing;
        view.value = "detail";
        try { attempts.value = await client.attempt(existing.id); } catch (e) { error.value = e.message; }
        feedback.toast.success("已找到原任务，未重复创建");
        return;
      }
    }
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
    if (!editing.value) await draftClient.save(draftPayload());
    sent = true;
    const saved = editing.value
      ? await client.edit(editing.value.id, payload)
      : await client.create(payload);
    const cleanupWarning = await clearCreatedDraft();
    pendingRequest.value = null;
    baseline.value = JSON.stringify({
      form: form.value,
      schedule: schedule.value,
      cover_config:coverConfig.value,portrait_cover_config:portraitCoverConfig.value,
    });
    // Created or editable tasks have no execution history. Drop any records
    // retained from a different task detail before showing the saved task.
    attempts.value = [];
    detail.value = saved;
    view.value = "detail";
    detailReturn.value = source.value ? "source" : "list";
    feedback.toast.success(
      editing.value ? "发布内容已保存" : "任务已创建，尚未执行",
    );
    await load();
    if (cleanupWarning) error.value = cleanupWarning;
  } catch (e) {
    formError.value = e.message;
    draftConflict.value = e.code === "draft_revision_conflict";
    if (definitivePublicationRejection(e) || (!sent && !wasPending)) {
      pendingRequest.value = null;
      if (!editing.value && draftClient) {
        draftClient.remember(draftPayload());
        if (sent && definitivePublicationRejection(e)) {
          try { await draftClient.save(draftPayload()); }
          catch (saveError) { draftConflict.value = saveError.code === "draft_revision_conflict"; formError.value += `；本机输入未更新，请保留当前填写。${saveError.message || ""}`; }
        }
      }
    } else formError.value += "。结果尚未确认，原请求和输入已保留；请先查询原任务。";
  } finally {
    busy.value = false;
  }
}
async function canLeave() {
  if (busy.value) return false;
  if (!dirty.value && !pendingRequest.value) return true;
  if (!editing.value && !pendingRequest.value) {
    if (!(await feedback.confirmWarning({ title: "保存草稿后返回？", message: draftPersistent.value ? "保存后重启可继续；不会占用成片或启动任务。" : "当前应用中可继续；刷新、关闭或退出后清空。", confirmText: "保存并返回", cancelText: "继续编辑" }))) return false;
    return saveDraft();
  }
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
const isExecuting = task => [PublicationStatus.Queued, PublicationStatus.Running].includes(task?.status);
const observedTasks = computed(() => view.value === "list" ? rows.value : view.value === "detail" && detail.value ? [detail.value] : []);
const needsExecutionRefresh = computed(() => observedTasks.value.some(task => isExecuting(task) || [PublicationStatus.AwaitingHuman,PublicationStatus.Reviewing].includes(task.status)));
const executionRefreshInterval = computed(() => !needsExecutionRefresh.value ? 0 : observedTasks.value.some(isExecuting) ? 5000 : 30000);
let executionTimer = null, refreshInFlight = false, disposed = false;
async function refreshExecution() {
  if (disposed || !needsExecutionRefresh.value || busy.value || loading.value || recoveryVisible.value || humanVisible.value || terminationVisible.value || refreshInFlight
      || (typeof document !== "undefined" && document.hidden)) return;
  refreshInFlight = true;
  const originalDetail = detail.value;
  try {
    if (view.value === "list") await load({ background: true });
    else {
      const [task, history] = await Promise.all([client.get(originalDetail.id), client.attempt(originalDetail.id)]);
      if (!disposed && view.value === "detail" && detail.value === originalDetail && !busy.value && !loading.value && !recoveryVisible.value && !humanVisible.value && !terminationVisible.value) {
        detail.value = task;
        attempts.value = history;
        error.value = "";
      }
    }
  } catch (e) {
    if (!disposed && view.value === "detail" && detail.value === originalDetail) error.value = e.message;
  } finally { refreshInFlight = false; }
}
watch(executionRefreshInterval, interval => {
  if (executionTimer) clearInterval(executionTimer);
  executionTimer = interval ? setInterval(refreshExecution, interval) : null;
});
function invalidateCreation() { disposed = true; }
onBeforeUnmount(() => {
  invalidateCreation();
  if (executionTimer) clearInterval(executionTimer);
  loadSequence++;
});
onMounted(async () => {
  await Promise.all([
    load(),
    createSessionClient().me().then(user => { if (!disposed) currentUser.value = user; }).catch(e => { if (!disposed) error.value = e.message; }),
  ]);
  try {
    categories.value = await compose.listCategories({
      scope: "all",
      enabled: true,
    });
    const publicationId=String(route.query.publication_id||"");
 if(/^[1-9][0-9]*$/.test(publicationId)){await openDetail({id:publicationId});return;}
 const id = String(route.query.output_id || "");
    if (canPublish.value && /^[1-9][0-9]*$/.test(id)) {
      const data = await compose.getComposeOutput(id);
      await openCreate(data.output);
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
          ><t-button v-if="canPublish" :loading="busy" @click="openCreate"
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
              ><t-button v-if="row.media_account_id" theme="primary" variant="text" size="small" @click.stop="viewAccount(row.media_account_id)">{{ row.resources.account_name || "查看账号" }}</t-button><span v-else>—</span><small class="publication-sub">{{
                row.resources.account_owner || "—"
              }}</small></template
            >
            <template #status="{ row }"
              ><BusinessStatus
                :status="row.status" :label="statusLabel(row.status)" /><small v-if="row.cancel_requested_at && row.status === PublicationStatus.Running" class="publication-sub">已请求安全停止，等待确认</small></template
            ><template #created_at="{ row }">{{
              formatDateTime(row.created_at)
            }}</template>
            <template #op="{ row }">
              <ResourceRowActions>
                <t-button v-if="canPublish && editable(row)" class="is-primary" variant="text" size="small" :disabled="busy" @click="startExecution(row)">执行</t-button>
                <t-button v-if="canPublish && row.status===PublicationStatus.AwaitingHuman" class="is-primary" variant="text" size="small" :disabled="busy" @click="openHumanScene(row)">去处理</t-button>
                <t-button v-if="[PublicationStatus.Uncertain,PublicationStatus.Reviewing].includes(row.status)" class="is-primary" variant="text" size="small" :disabled="busy" @click="openVerification(row)">{{row.status===PublicationStatus.Reviewing?'补录结果':'核实结果'}}</t-button>
                <t-button v-if="canPublish && retryable(row)" class="is-primary" variant="text" size="small" :disabled="busy" @click="retryExecution(row)">重试</t-button>
                <t-button v-if="editable(row)" variant="text" size="small" :disabled="busy" @click="edit(row)">编辑</t-button>
                <t-button v-if="cancellable(row)" theme="danger" variant="text" size="small" :disabled="busy" @click="openTermination(row, 'cancel')">终止</t-button>
                <t-button v-if="abandonable(row)" theme="danger" variant="text" size="small" :disabled="busy" @click="openTermination(row, 'abandon')">放弃</t-button>
                <t-button v-if="row.work_id" variant="text" size="small" @click="router.push({path:'/work',query:{work_id:row.work_id}})">查看作品</t-button>
                <t-button v-if="canPublish && copyable(row)" variant="text" size="small" :disabled="busy" @click="copyTask(row)">复制</t-button>
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
      <FeedbackAlert v-if="formError && !validationField" theme="error" :message="formError" />
      <t-button v-if="draftConflict" theme="default" variant="outline" :disabled="busy" @click="reloadDraft">读取最新草稿</t-button>
      <div class="quick-publish">
        <section class="resource-editor-section quick-block">
          <h3>
            <span>01</span>{{ editing ? "发布资源（已锁定）" : "选择发布平台与账号" }}
          </h3>
          <FeedbackAlert v-if="draftResourceIssue.account || draftResourceIssue.output" theme="error" :message="[draftResourceIssue.account, draftResourceIssue.output].filter(Boolean).join('；')" />
          <FormError id="publication-resource-error" v-if="validationField === 'resource'" :message="formError" />
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
            ><div class="quick-account-field"
              ><ResourceRequiredLabel>发布账号</ResourceRequiredLabel><t-input
                :value="account?.platform_name || ''"
                aria-label="发布账号"
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
              ><t-button v-if="account" theme="primary" variant="text" size="small" @click="viewAccount(account.id)">查看账号信息</t-button></div
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
        <section v-if="output || form.composite_output_id" class="resource-editor-section quick-block">
          <div class="quick-content-heading"><h3><span>02</span>发布内容</h3><PublicationContentGenerator :context="generationContext" :values="generationValues" :template="coverConfig.method === 'template'" :disabled="frozen" @adopt="adoptGeneratedContent" /></div>
          <div class="quick-copy-fields">
          <label
            ><ResourceRequiredLabel>发布标题</ResourceRequiredLabel><t-textarea
              v-model="form.content.description"
              aria-label="发布标题"
              :aria-describedby="validationField === 'text' && formError ? 'publication-text-error' : undefined"
              placeholder="默认带入素材标题，可修改"
              :maxlength="1000"
              :autosize="{ minRows: 3, maxRows: 5 }"
              :disabled="frozen"
              @change="invalidate"
          /></label>
          <FormError id="publication-text-error" v-if="validationField === 'text'" :message="formError" />
          <div class="quick-cover-field">
            <label class="quick-field-heading"><ResourceRequiredLabel>发布封面</ResourceRequiredLabel><ResourceFieldHelp label="发布封面" content="使用现成图片或选择模板添加文字。默认使用成片真实封面；百度需要16:9横版和3:4竖版封面，单张不超过5MB；分别选择图片或匹配模板生成。" /></label>
            <h4>横版封面 · 16:9</h4>
            <FormError id="publication-cover-error" v-if="validationField === 'cover'" :message="formError" />
            <FeedbackAlert v-if="draftResourceIssue.cover" theme="error" :message="draftResourceIssue.cover" />
            <div class="quick-cover-workspace">
              <div class="publication-cover-preview"><button v-if="cover?.resource_path" type="button" class="publication-image-button" aria-label="预览发布封面" @click="previewImage(cover.resource_path, cover.name)"><img :src="files.resourceURL(cover)" alt="当前发布封面" /><span>预览封面</span></button><div v-else class="publication-cover-empty">请选择图片或生成封面</div></div>
              <PublishingCoverEditor :config="coverConfig" :output="output" :category="form.category_id" :disabled="frozen" @update:config="updateCoverConfig" @rendered="useRenderedCover" @choose-image="chooseCoverImage('horizontal')" @use-output="useOutputCover" />
            </div>
            <h4>竖版封面 · 3:4</h4>
            <FormError id="publication-portrait-error" v-if="validationField === 'portrait'" :message="formError" />
            <FeedbackAlert v-if="draftResourceIssue.portrait_cover" theme="error" :message="draftResourceIssue.portrait_cover" />
            <div class="quick-cover-workspace">
              <div class="publication-cover-preview"><button v-if="portraitCover?.resource_path" type="button" class="publication-image-button" aria-label="预览竖版封面" @click="previewImage(portraitCover.resource_path, portraitCover.name)"><img :src="files.resourceURL(portraitCover)" alt="当前竖版封面" /><span>预览封面</span></button><div v-else class="publication-cover-empty">请选择图片或生成竖版封面</div></div>
              <PublishingCoverEditor :config="portraitCoverConfig" :aspect="[3,4]" :output="output" :category="form.category_id" :disabled="frozen" @update:config="updatePortraitConfig" @rendered="usePortraitArtwork" @choose-image="chooseCoverImage('portrait')" @use-output="usePortraitOutput" />
            </div>
          </div>
          <label
            >话题<t-input
              v-model="form.content.topics"
              aria-label="话题" :aria-describedby="validationField === 'text' && formError ? 'publication-text-error' : undefined"
              :maxlength="200"
              :disabled="frozen"
              @change="invalidate"
          /><small>标题与话题合计 {{ publicationTextLength(form.content.description, form.content.topics) }}/50 字（含分隔空格）</small></label>
          <label
            ><ResourceRequiredLabel>创作声明</ResourceRequiredLabel><t-select
              v-model="form.content.declaration"
              aria-label="创作声明"
              :aria-describedby="validationField === 'declaration' && formError ? 'publication-declaration-error' : undefined"
              :options="[{ label: '无需声明', value: '无需声明' }]"
              placeholder="按内容事实选择"
              :disabled="frozen"
              @change="invalidate"
          /></label>
          <FormError id="publication-declaration-error" v-if="validationField === 'declaration'" :message="formError" />
          </div>
        </section>
        <section v-if="output || form.composite_output_id" class="resource-editor-section quick-block">
          <h3><span>03</span>发布设置</h3>
          <div class="quick-resource-grid two">
            <label>发布方式<t-input value="自动发布" disabled /></label>
            <label><span class="quick-field-heading">最早执行时间<ResourceFieldHelp label="最早执行时间" content="留空表示确认执行后尽快发布；指定时间为最早开始执行时间。创建任务后仍需单独确认执行。" /></span><ResourceDateTime v-model:value="schedule" aria-label="最早执行时间" :disabled="frozen" /></label>
          </div>
          <t-checkbox v-model="form.content.confirmed" :disabled="frozen" :aria-describedby="validationField === 'confirmation' && formError ? 'publication-confirmation-error' : undefined" @change="formError = ''"
            >已核对当前文案、封面和声明，确认内容可用</t-checkbox
          >
          <FormError id="publication-confirmation-error" v-if="validationField === 'confirmation'" :message="formError" />
        </section>
      </div>
      <template #footer
        ><span v-if="!editing" class="publication-draft-help">{{ draftPersistent ? '保存草稿后可重启续编，不占用成片或启动发布。' : '仅暂存当前填写；刷新、关闭或退出登录后清空。' }}</span><t-button v-if="!editing" theme="default" variant="outline" :disabled="busy" @click="saveDraft">保存草稿</t-button
        ><t-button :loading="busy" @click="submit">{{
          pendingRequest ? "查询创建结果" : editing ? "保存" : "创建任务"
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
          <t-descriptions-item label="任务状态"><BusinessStatus :status="detail.status" :label="statusLabel(detail.status)" /><small v-if="detail.cancel_requested_at && detail.status === PublicationStatus.Running" class="publication-sub">已请求安全停止，等待确认</small></t-descriptions-item>
          <t-descriptions-item label="发布平台">{{ platformLabel(detail.platform) }}</t-descriptions-item>
          <t-descriptions-item label="内容类目">{{ categoryName(detail.category_id) }}</t-descriptions-item>
          <t-descriptions-item label="发布账号"><t-button v-if="detail.media_account_id" theme="primary" variant="text" size="small" @click="viewAccount(detail.media_account_id)">{{ detail.resources.account_name || "查看账号" }}</t-button><span v-else>—</span></t-descriptions-item>
          <t-descriptions-item label="真实 UID">{{ detail.platform_account_uid }}</t-descriptions-item>
          <t-descriptions-item v-if="detail.last_checked_at" label="最近核验">{{formatDateTime(detail.last_checked_at)}}</t-descriptions-item>
          <t-descriptions-item v-if="detail.last_check_message && detail.status !== PublicationStatus.Published" label="核验说明" full>{{detail.last_check_message}}</t-descriptions-item>
          <t-descriptions-item v-if="detail.platform_submission_id" label="投稿回执 ID">{{detail.platform_submission_id}}</t-descriptions-item>
          <t-descriptions-item v-if="detail.work_id" label="关联作品"><t-button variant="text" @click="router.push({path:'/work',query:{work_id:detail.work_id}})">作品 {{detail.work_id}}</t-button></t-descriptions-item>
          <t-descriptions-item v-if="detail.published_at" label="平台实际发布时间">{{formatDateTime(detail.published_at)}}</t-descriptions-item>
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
          <t-descriptions-item label="竖版封面" full><button v-if="detail.resources.portrait_cover_resource_path" type="button" class="publication-image-button" @click="previewImage(detail.resources.portrait_cover_resource_path,detail.resources.portrait_cover_name)"><img :src="files.resourceURL({resource_path:detail.resources.portrait_cover_resource_path})" alt="竖版封面" /><span>预览封面</span></button><span>{{detail.resources.portrait_cover_name || '—'}}</span></t-descriptions-item>
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
    <PublishingTerminationDialog v-model:visible="terminationVisible" :task="terminationTask" :action="terminationAction" :busy="busy" :error="terminationError" :submitted="terminationSubmitted" @submit="saveTermination" />
    <ResourceOverlay :visible="humanVisible" title="处理人工验证" :width="560" :busy="busy" form-actions @update:visible="humanVisible=$event">
      <FeedbackAlert v-if="humanError" theme="error" :message="humanError" />
      <p>请在已打开的原账号页面完成登录或安全验证，然后继续原发布尝试。</p>
      <p v-if="humanTask">任务 {{humanTask.id}} · 账号 {{humanTask.resources?.account_name || humanTask.platform_account_uid}} · 原尝试 {{humanTask.current_attempt_id}}</p>
      <p>请保留原页面，完成验证后不要自行点击平台的发布按钮。继续后会再次核对账号，并按原任务执行。</p>
      <template #footer><t-button theme="default" variant="outline" :disabled="busy" @click="humanVisible=false">暂不处理</t-button><t-button :loading="busy" @click="continueHumanScene">已经处理，继续原尝试</t-button></template>
    </ResourceOverlay>
    <PublishingRecoveryDialog v-model:visible="recoveryVisible" :task="recoveryTask" :busy="busy" :error="recoveryError" @submit="saveVerification" />
    <PublishingResourcePicker
      :visible="!!picker"
      :kind="picker"
      :aspect="picker==='image' ? (coverTarget==='portrait' ? [3,4] : [16,9]) : undefined"
      :category="form?.category_id"
      :account="account"
      @update:visible="!$event && (picker = '')"
      @selected="picked"
      @view-account="viewAccount"
    />
    <AccountSummaryDialog v-model:visible="accountSummaryVisible" :account-id="accountSummaryId" />
  </div>
</template>
<style scoped>
.publication-draft-help { margin-right: auto; color: var(--wt-text-secondary); font-size: 12px; line-height: 20px; overflow-wrap: anywhere; }
@media (max-width: 680px) { .publication-draft-help { flex-basis: 100%; } }
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
.quick-content-heading { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
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
.quick-block label:not(.t-checkbox), .quick-account-field {
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
.publication-cover-empty { width:100%;min-height:160px;display:grid;place-items:center;background:var(--td-bg-color-secondarycontainer);color:var(--td-text-color-secondary);font-size:12px; }
.quick-cover-workspace { display: grid; grid-template-columns: 220px minmax(0, 1fr); align-items: start; gap: 24px; min-width: 0; }
.quick-cover-workspace > .publication-cover-preview { display: flex; flex-direction: column; align-items: flex-start; gap: 12px; min-width: 0; }
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
