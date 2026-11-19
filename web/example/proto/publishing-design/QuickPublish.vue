<script setup>
import { computed, nextTick, ref, watch } from "vue";
import Overlay from "./PrototypeOverlay.vue";
import { chooseDraft } from "./draftRecovery.js";
import RequiredLabel from "./RequiredLabel.vue";
import TaskFieldHelp from "./TaskFieldHelp.vue";
import { accountAvailability } from "./accountAvailability.js";
import AccountPicker from "./AccountPicker.vue";
import PublishingScopeFilters from "./PublishingScopeFilters.vue";
import PublishingExecutionSettings from "./PublishingExecutionSettings.vue";
import { defaultPublishingResources } from "./quickPublishSelection.js";
import { canEditTask } from "./taskExperience.js";
import { feedback } from "../../../src/shared/ui/feedback/index.js";
import {
  platforms,
  defaults,
  declarationOptions,
  resourceStatus,
  validate,
} from "./model.js";
import { initCandidate, editField } from "./content.js";
import {
  ensureCoverResults,
  coverSpecs,
  coverSignature,
  coverTextFields,
} from "./covers.js";
import { createPublication, clone } from "./relationships.js";
const props = defineProps({ db: Object, persist: Function });
const emit = defineEmits(["created", "preview", "return-source"]);
const visible = ref(false),
  picker = ref(false),
  search = ref(""),
  unavailable = ref(false),
  assetPage = ref(1),
  busy = ref(false),
  generating = ref(false),
  rendering = ref(false),
  error = ref(""),
  notice = ref(""),
  checked = ref(false),
  failure = ref(""),
  form = ref(null);
const coverChoice = ref("替换图片");
const editBaseline = ref("");
const editingVersion = ref(null);
const savedDraft = ref(""),
  editing = ref(null),
  accountPicker = ref(null);
function openAccountPicker() {
  if (!category.value || editing.value) return;
  accountPicker.value?.open();
}
async function edit(task) {
  sourceReturn.value=false;
  coverChoice.value = task.coverTreatment || "替换图片";
  if (!canEditTask(task, props.db)) return;
  editingVersion.value = task.version || 1;
  editing.value = task;
  row.value = clone(task);
  editBaseline.value = JSON.stringify(row.value);
  platform.value = task.platform;
  category.value = task.category;
  accountId.value = task.accountId;
  error.value = "";
  notice.value = "";
  visible.value = true;
}
const row = ref(null),
  platform = ref(""),
  accountId = ref(null),
  category = ref(""),
  creationKey = ref(crypto.randomUUID()),
  copied = ref(null);
const accountOptions = computed(() =>
  props.db.accounts.filter(
    (a) => a.enabled && a.loginValid !== false && a.platform === platform.value,
  ),
);
const selectedAccount = computed(() =>
  accountOptions.value.find((a) => a.id === accountId.value),
);
const selectedAvailability = computed(() =>
  selectedAccount.value
    ? accountAvailability(
        props.db,
        selectedAccount.value,
        platform.value,
        category.value,
        editing.value?.id,
      )
    : null,
);
const categoryOptions = computed(() => [
  ...new Set(accountOptions.value.flatMap((a) => a.categories || [])),
]);
const selectedAsset = computed(() =>
  props.db.assets.find((a) => a.id === row.value?.assetId),
);
const assets = computed(() =>
  props.db.assets
    .filter(
      (a) =>
        a.category === category.value &&
        `${a.id} ${a.title}`.includes(search.value),
    )
    .map((a) => ({
      ...a,
      reason: resourceStatus(props.db, a, platform.value, [accountId.value]),
    }))
    .filter((a) => (unavailable.value ? !!a.reason : !a.reason)),
);
const pageAssets = computed(() =>
  assets.value.slice((assetPage.value - 1) * 6, assetPage.value * 6),
);
const problems = computed(() =>
  row.value
    ? [
        ...(selectedAccount.value
          ? []
          : ["账号不可用，请重新选择合格的发布账号"]),
        ...validate(
          row.value,
          props.db,
          editing.value?.id ||
            props.db.tasks.find((t) => t.creationKey === creationKey.value)?.id,
        ),
      ]
    : [
        !platform.value
          ? "请选择发布平台"
          : !accountId.value
            ? "请选择发布账号"
            : !category.value
              ? "请选择内容类目"
              : "请选择一条可用成片",
      ],
);
const currentDraft = computed(() => props.db.quickDraft?.active ? props.db.quickDraft : props.db.publishDraft?.active && props.db.publishDraft.mode === '快速发布' ? props.db.publishDraft : null);
const hasDraft = computed(() => !!currentDraft.value);
const templateFields = computed(() =>
  coverTextFields(row.value?.templateDocument),
);
const manual = computed(() => row.value?.manualFields || []);
const labels = {
  title: "发布标题",
  description: "正文 / 简介",
  shortTitle: "封面主标题",
  subTitle: "封面副标题",
  topics: "话题",
  tags: "标签",
};
function draft() {
  return {
    active: true,
    platform: platform.value,
    accountId: accountId.value,
    category: category.value,
    row: row.value ? clone(row.value) : null,
    creationKey: creationKey.value,
    savedAt: props.db.quickDraft?.creationKey === creationKey.value ? props.db.quickDraft.savedAt || null : null,
  };
}
watch(
  [row, platform, accountId, category],
  () => {
    if (visible.value && !editing.value && !busy.value) {
      checked.value = false;
      props.db.quickDraft = draft();
    }
  },
  { deep: true },
);
watch([search, unavailable], () => (assetPage.value = 1));
watch([platform, category, accountId], () => { search.value = ""; unavailable.value = false; assetPage.value = 1; });
const sourceReturn=ref(false);
async function open(source = null) {
  sourceReturn.value=!!source?.returnURL;
  editing.value = null;
  if (hasDraft.value) {
    const choice = await chooseDraft({ label: '发布任务', savedAt: currentDraft.value?.savedAt });
    if (!choice) return;
    if (choice === 'continue') return resume();
    const previous = clone(props.db.quickDraft || null);
    props.db.quickDraft = { active: false };
    const legacyActive = props.db.publishDraft?.mode === '快速发布' && props.db.publishDraft.active;
    if (legacyActive) props.db.publishDraft.active = false;
    if (!(await persist())) {
      props.db.quickDraft = previous;
      if (legacyActive) props.db.publishDraft.active = true;
      feedback.toast.error('无法重新开始，原草稿已保留，请重试。');
      return;
    }
  }
  row.value = null;
  platform.value = source?.platform || "";
  accountId.value = source?.accountId || null;
  category.value = source?.category || "";
  if(source?.sourceOutputId!=null && !source.accountId){
    accountId.value=defaultPublishingResources(props.db,platform.value,category.value,null).account?.id||null;
    source={...source,accountId:accountId.value};
  }
  copied.value = source ? clone(source) : null;
  if (source) {
    const fields = [
      "platform",
      "accountId",
      "assetId",
      "category",
      "title",
      "description",
      "shortTitle",
      "subTitle",
      "tags",
      "topics",
      "declaration",
      "visibility",
      "schedule",
      "partition",
      "cover",
      "coverSource",
      "coverMode",
      "coverTreatment",
      "imageId",
      "templateId",
      "templateVersion",
      "templateDocument",
      "templateDocuments",
      "coverVariants",
      "coverSignature",
    ];
    row.value = initCandidate({
      ...defaults(platform.value, props.db.assets.find(a => a.id === source.assetId), props.db),
      ...Object.fromEntries(
        fields
          .filter((f) => source[f] !== undefined)
          .map((f) => [f, clone(source[f])]),
      ),
    });
    coverChoice.value = source.coverTreatment || "替换图片";
  }
  creationKey.value = crypto.randomUUID();
  error.value = "";
  notice.value =
    source?.status === "已发布"
      ? "复制已发布内容，请注意同账号重复投稿风险；资源与账号将重新校验。"
      : "";
  checked.value = false;
  visible.value = true;
  if (!row.value) await selectDefaultResources();
}
async function resume() {
  editing.value = null;
  const old = currentDraft.value;
  if (!old) return;
  platform.value = old.platform || old.platforms?.[0] || "";
  accountId.value = old.accountId || old.quickAccountId || null;
  category.value = old.category || old.categories?.[0] || "";
  row.value = old.row
    ? clone(old.row)
    : old.rows?.[0]
      ? clone(old.rows[0])
      : null;
  creationKey.value = old.creationKey || crypto.randomUUID();
  props.db.quickDraft = { ...clone(old), creationKey: creationKey.value };
  error.value = "";
  notice.value = "";
  feedback.toast.info({ content: "已恢复草稿，资源可用性会重新检查。", duration: 3000 });
  checked.value = false;
  coverChoice.value = row.value?.coverTreatment || "替换图片";
  visible.value = true;
  if (!row.value) await selectDefaultResources();
  savedDraft.value = JSON.stringify(draft());
  if (props.db.publishDraft?.mode === "快速发布")
    props.db.publishDraft.active = false;
}
async function changeResource(field, value) {
  if (
    row.value &&
    !(await feedback.confirmWarning({
      title: "更换发布资源？",
      message: "当前内容和封面配置将清空，请重新核对新资源。",
      confirmText: "更换资源",
    }))
  )
    return;
  row.value = null;
  error.value = "";
  notice.value = "";
  if (field === "platform") {
    platform.value = value;
    accountId.value = null;
    category.value =
      categoryOptions.value.length === 1 ? categoryOptions.value[0] : "";
  }
  if (field === "account") {
    accountId.value = value;
    category.value ||=
      categoryOptions.value.length === 1 ? categoryOptions.value[0] : "";
  }
  if (field === "category") {
    category.value = value;
    accountId.value = null;
  }
  await selectDefaultResources();
}
async function selectDefaultResources() {
  const selected = defaultPublishingResources(
    props.db,
    platform.value,
    category.value,
    accountId.value,
  );
  accountId.value = selected.account?.id || null;
  if (selected.asset) await chooseAsset(selected.asset);
}
async function chooseAsset(a) {
  if (a.reason) return;
  const next = initCandidate({
    platform: platform.value,
    accountId: accountId.value,
    assetId: a.id,
    category: a.category,
    ...defaults(platform.value, a, props.db),
    coverSource: a.cover,
    manualFields: [],
  });
  if (copied.value) {
    for (const f of [
      "title",
      "description",
      "shortTitle",
      "subTitle",
      "tags",
      "topics",
      "declaration",
      "coverMode",
      "coverTreatment",
      "imageId",
      "coverSource",
      "templateId",
      "templateVersion",
      "templateDocument",
      "templateDocuments",
      "templateSnapshots",
    ])
      if (copied.value[f] !== undefined) next[f] = clone(copied.value[f]);
    copied.value = null;
  }
  row.value = next;
  coverChoice.value = next.coverTreatment || "替换图片";
  picker.value = false;
  if (next.coverMode === "现成封面") await renderCover();
}
function setField(field, value) {
  editField(row.value, field, value);
  row.value.manualFields = [...new Set([...manual.value, field])];
  notice.value = "";
}
async function generate() {
  if (generating.value || rendering.value || busy.value || !row.value) return;
  generating.value = true;
  try {
    // 当前未接建议服务。短暂的操作反馈窗口防止连点和加载闪烁，不生成假内容。
    await new Promise((resolve) => setTimeout(resolve, 400));
    feedback.toast.error({ content: "暂时无法获取 AI 建议，请手动填写或稍后重试。", duration: 3000 });
  } finally {
    generating.value = false;
  }
}
async function renderCover() {
  const current = row.value;
  if (!current) return;
  rendering.value = true;
  error.value = "";
  try {
    if (failure.value === "封面生成失败")
      throw Error("封面生成失败，当前配置已保留，请重试。");
    if (current.coverMode === "图片库模板") {
      current.coverTreatment = "图片库图 + 模板";
      current.coverMode = "模板合成";
    }
    if (
      current.coverTreatment === "图片库图 + 模板" &&
      !props.db.images.some(
        (image) =>
          image.id === current.imageId &&
          image.enabled &&
          image.uses?.includes("底图"),
      )
    )
      throw Error("请选择图片库中可用的底图");
    await ensureCoverResults(current);
  } catch (e) {
    error.value = e.message;
    current.coverError = e.message;
  } finally {
    rendering.value = false;
  }
}
function setTemplate(value) {
  const t = props.db.templates.find((t) => t.id === value);
  setField("templateId", value);
  row.value.templateVersion = t?.version;
  row.value.templateDocument = t ? clone(t.document) : null;
  row.value.templateDocuments = {};
  for (const spec of coverSpecs(platform.value)) {
    const matching = [t, ...props.db.templates].find(
      (candidate) =>
        candidate?.enabled &&
        coverTextFields(candidate.document)
          .map((f) => f.key)
          .sort()
          .join() ===
          coverTextFields(t?.document)
            .map((f) => f.key)
            .sort()
            .join() &&
        Math.abs(
          candidate.document.width / candidate.document.height -
            spec.width / spec.height,
        ) < 0.01,
    );
    if (matching)
      row.value.templateDocuments[spec.key] = clone(matching.document);
  }
}
function setCoverChoice(value) {
  coverChoice.value = value;
  row.value.coverTreatment = value;
  setField("coverMode", value === "替换图片" ? "现成封面" : "模板合成");
  if (value === "当前素材图 + 模板") {
    row.value.imageId = null;
    row.value.coverSource = selectedAsset.value?.cover;
  } else if (value === "图片库图 + 模板") {
    const image = props.db.images.find(
      (i) => i.id === row.value.imageId && i.enabled && i.uses.includes("底图"),
    );
    row.value.imageId = image?.id || null;
    row.value.coverSource = image?.src || "";
  }
  row.value.coverError =
    value === "替换图片"
      ? ""
      : value === "图片库图 + 模板" && !row.value.imageId
        ? "请选择底图后生成封面"
        : "请点击生成封面";
}
function setCoverMethod(value) {
  const replacing = value === "替换图片";
  const selectedImage = props.db.images.find(
    (i) =>
      i.id === row.value.imageId &&
      i.enabled &&
      i.uses.includes(replacing ? "现成封面" : "底图"),
  );
  setCoverChoice(
    replacing
      ? "替换图片"
      : selectedImage
        ? "图片库图 + 模板"
        : "当前素材图 + 模板",
  );
  image(selectedImage?.id || 0);
}
function setCoverImage(value) {
  if (coverChoice.value !== "替换图片")
    setCoverChoice(value ? "图片库图 + 模板" : "当前素材图 + 模板");
  image(value);
}
function image(value) {
  const img = props.db.images.find((i) => i.id === value);
  row.value.imageId = value;
  setField("coverSource", img?.src || selectedAsset.value?.cover);
  if (row.value.coverMode === "现成封面") {
    row.value.cover = row.value.coverSource;
    row.value.coverVariants = {};
  } else row.value.coverError = "封面配置已更改，请点击生成封面";
}
async function approve(value) {
  checked.value = false;
  if (!value) return;
  await renderCover();
  if (error.value) return;
  row.value.contentApproved = true;
  row.value.coverApproved = true;
  await nextTick();
  checked.value = true;
}
async function persist() {
  if (failure.value === "保存失败") {
    error.value = "保存失败，当前输入已保留，请重试。";
    return false;
  }
  const ok = await props.persist();
  if (!ok) error.value = "保存失败，当前输入已保留，请重试。";
  return ok;
}
async function save() {
  error.value = "";
  busy.value = true;
  props.db.quickDraft = { ...draft(), savedAt: new Date().toISOString() };
  if (await persist()) {
    notice.value = "";
    savedDraft.value = JSON.stringify(draft());
    feedback.toast.success({ content: "草稿已保存，不占用成片。", duration: 3000 });
  }
  busy.value = false;
}
async function close() {
  if (generating.value || rendering.value || busy.value) return;
  if (editing.value) {
    if (JSON.stringify(row.value) !== editBaseline.value && !(await feedback.confirmWarning({
      title: "放弃未保存的内容？",
      message: "本次标题、封面等修改尚未保存，返回后将丢失。",
      confirmText: "放弃修改",
      cancelText: "继续编辑",
    }))) return;
    visible.value = false;
    editing.value = null;
    return;
  }
  if (generating.value || rendering.value || busy.value) return;
  if (!platform.value && !row.value) {
    props.db.quickDraft = { active: false };
    visible.value = false;
    if(sourceReturn.value){sourceReturn.value=false;emit("return-source");}
    return;
  }
  if (
    JSON.stringify(draft()) !== savedDraft.value &&
    (platform.value || row.value) &&
    !(await feedback.confirmWarning({
      title: "保存草稿后返回？",
      message: "保存后可继续配置；草稿不会占用成片或启动任务。",
      confirmText: "保存并返回",
      cancelText: "继续编辑",
    }))
  )
    return;
  busy.value = true;
  props.db.quickDraft = { ...draft(), savedAt: new Date().toISOString() };
  if (await persist()) {visible.value = false;if(sourceReturn.value){sourceReturn.value=false;emit("return-source");}}
  busy.value = false;
}
async function create() {
  if (busy.value) return;
  error.value = "";
  if (row.value) {
    if (
      row.value.coverMode !== "现成封面" &&
      row.value.coverSignature !== coverSignature(row.value)
    ) {
      error.value = "封面配置已更改，请先点击生成封面";
      return;
    }
    if (row.value.coverMode === "现成封面") await renderCover();
    if (error.value) return;
    row.value.contentApproved = true;
    row.value.coverApproved = true;
  }
  if (row.value) {
    row.value.contentApproved = true;
    row.value.coverApproved = true;
  }
  const account = selectedAccount.value;
  const access =
    account &&
    accountAvailability(
      props.db,
      account,
      platform.value,
      category.value,
      editing.value?.id,
    );
  if (access?.reason) {
    error.value = access.reason;
    return;
  }
  if (problems.value.length) {
    error.value = "请先处理发布前检查，并确认当前内容与封面。";
    locate(problems.value[0] || "审核");
    return;
  }
  busy.value = true;
  try {
    if (failure.value === "保存失败")
      throw Error("保存失败，尚未创建任务，当前输入已保留。");
    if (editing.value) {
      const current = props.db.tasks.find((t) => t.id === editing.value.id);
      if (!current || !canEditTask(current, props.db))
        throw Error("任务状态已变化，不能保存修改");
      if ((current.version || 1) !== editingVersion.value) throw Error("任务内容已由其他操作更新，请保留当前输入后重新打开任务");
      const before = clone(current);
      const fields = [
        "title",
        "description",
        "shortTitle",
        "subTitle",
        "tags",
        "topics",
        "declaration",
        "visibility",
        "schedule",
        "partition",
        "cover",
        "coverSource",
        "coverMode",
        "coverTreatment",
        "coverSignature",
        "coverVariants",
        "templateId",
        "templateVersion",
        "templateDocument",
        "templateDocuments",
        "imageId",
        "contentApproved",
        "coverApproved",
        "publishMethod",
        "publishingConfigVersion",
      ];
      for (const field of fields)
        if (row.value[field] !== undefined)
          current[field] = clone(row.value[field]);
      current.version = (current.version || 1) + 1;
      current.updatedAt = new Date().toISOString();
      const snapshot = clone(current);
      delete snapshot.snapshot;
      delete snapshot.attemptHistory;
      delete snapshot.log;
      current.snapshot = snapshot;
      if (!(await persist())) {
        Object.assign(current, before);
        throw Error("保存失败，修改仍在本页，请重试保存");
      }
      visible.value = false;
      editing.value = null;
      return;
    }
    const result = createPublication(props.db, {
      platform: platform.value,
      rows: [row.value],
      quick: true,
      creationKey: creationKey.value,
    });
    props.db.quickDraft = { ...draft(), active: false };
    if (!(await persist())) {
      props.db.quickDraft.active = true;
      error.value =
        "任务已在本页创建但尚未保存，请重试创建任务；不会重复生成。";
      return;
    }
    visible.value = false;
    emit("created", result);
  } catch (e) {
    error.value = e.message;
  } finally {
    busy.value = false;
  }
}
async function locate(issue) {
  const target = /账号|成片|平台|类目|占用/.test(issue)
    ? "resources"
    : /封面|审核/.test(issue)
      ? "cover"
      : /声明/.test(issue)
        ? "declaration"
        : /分区|标签/.test(issue)
          ? "platform"
          : /时间/.test(issue)
            ? "schedule"
            : /正文|描述/.test(issue)
              ? "description"
              : "title";
  await nextTick();
  const el = form.value?.querySelector(`[data-field="${target}"]`);
  el?.scrollIntoView({ behavior: "smooth", block: "center" });
  (el?.querySelector("input,textarea,button") || el)?.focus?.({
    preventScroll: true,
  });
}
defineExpose({ open, resume, edit, hasDraft });
</script>
<template>
  <Overlay
    :visible="visible"
    :title="editing ? '编辑发布内容' : '快速发布'"
    page
    task-page
    :back-label="sourceReturn ? '返回成片管理' : '返回发布任务列表'"
    kind="drawer"
    :width="1060"
    :busy="busy || generating || rendering"
    @update:visible="close"
  >
    <div ref="form" class="quick-publish">
      <t-alert v-if="error" theme="warning" :message="error" />
      <p v-if="notice" class="quick-notice" role="status">{{ notice }}</p>
      <section class="quick-block resource-editor-section" :class="{ 'is-locked': !!editing }" data-field="resources" tabindex="-1">
        <div class="quick-section-heading">
          <h3><span>01</span>{{ editing ? "发布资源（已锁定）" : "选择发布平台与账号" }}</h3>
        </div>
        <div class="quick-resource-grid">
          <label
            ><RequiredLabel>发布平台</RequiredLabel
            ><t-select
              :value="platform"
              :disabled="!!editing"
              :options="platforms.map((p) => ({ label: p, value: p }))"
              @change="(v) => changeResource('platform', v)" /></label
          ><label
            ><RequiredLabel>发布类目</RequiredLabel
            ><t-select
              :value="category"
              :disabled="!platform || !!editing"
              :options="categoryOptions.map((c) => ({ label: c, value: c }))"
              @change="(v) => changeResource('category', v)" /></label
          ><div class="quick-account-field"
            ><span class="quick-field-heading"><RequiredLabel>发布账号</RequiredLabel><TaskFieldHelp v-if="selectedAccount" label="发布账号" :content="`等级 ${selectedAccount.level ?? '未同步'}；今日余量 ${selectedAvailability?.remaining ?? '未知'}。`" /></span
            ><t-input
              class="quick-account-input"
              :value="selectedAccount?.name || ''"
              placeholder="请选择发布账号"
              readonly
              :disabled="!category || !!editing"
              @click="openAccountPicker"
              @enter="openAccountPicker"
            ><template #suffix><t-button variant="text" size="small" :disabled="!category || !!editing" :aria-label="selectedAccount ? '更换账号' : '选择账号'" @click.stop="openAccountPicker">{{ selectedAccount ? '更换' : '选择' }}</t-button></template></t-input>
            </div>
        </div>

        <p v-if="platform && !accountOptions.length" class="quick-warning">
          当前平台没有可用账号，请先补充账号或处理登录状态。
        </p>
        <p
          v-if="platform && category && !selectedAccount"
          class="quick-warning"
        >
          当前平台与类目没有可用账号，请更换类目或处理账号状态。
        </p>
        <div class="quick-selected-asset">
          <img v-if="selectedAsset" :src="selectedAsset.cover" alt="已选成片" />
          <div>
            <strong>{{ selectedAsset?.title || "选择本次要发布的成片" }}</strong
            ><small>{{
              selectedAsset
                ? `成片 ${selectedAsset.id} · ${selectedAsset.duration} · ${selectedAsset.category}`
                : category && selectedAccount
                  ? "当前类目没有可用成片，请更换类目或补充成片。"
                  : "只安排当前账号支持、同平台未占用的成片。"
            }}</small>
          </div>
          <t-button
            variant="outline"
            :disabled="!category || !accountId || !!editing"
            @click="
              picker = true;
            "
            >{{ selectedAsset ? "更换成片" : "选择成片" }}</t-button
          ><t-button
            v-if="selectedAsset"
            variant="text"
            @click="
              emit('preview', {
                src: selectedAsset.cover,
                title: selectedAsset.title,
              })
            "
            >预览</t-button
          >
        </div>
      </section>
      <section v-if="row" class="quick-block resource-editor-section">
        <div class="quick-section-heading">
          <h3><span>02</span>发布内容</h3>
          <t-button variant="outline" :loading="generating" :disabled="generating || rendering || busy" :aria-busy="generating" @click="generate()"
            >AI 填写</t-button
          >
        </div>
        <div class="quick-content-grid">
          <div class="quick-copy-fields">
            <label v-if="platform === '百家号'" data-field="description"
              ><RequiredLabel>发布标题</RequiredLabel>
              ><t-textarea
                :value="row.description"
                :autosize="{ minRows: 3, maxRows: 5 }"
                @change="(v) => setField('description', v)"
            /></label>
            <label v-if="platform !== '百家号'" data-field="title"
              ><span
                ><RequiredLabel>发布标题</RequiredLabel>
                <small
                  >{{ Array.from(row.title || "").length }}/{{
                    platform === "小红书" ? 20 : 80
                  }}</small
                ></span
              ><t-input
                :value="row.title"
                placeholder="让用户一眼看懂这条内容"
                @change="(v) => setField('title', v)"
            /></label>
            <div class="quick-cover-fields" data-field="cover" tabindex="-1">
              <h4 class="quick-field-heading"><RequiredLabel>发布封面</RequiredLabel><TaskFieldHelp label="发布封面" :content="platform === '百家号' ? '使用现成图片或选择模板添加文字。横竖封面规格待平台核验，当前按原图预览。' : '使用现成图片，或选择模板添加文字；文字字段由模板决定。'" /></h4>
              <div class="quick-cover-methods" role="radiogroup" aria-label="封面来源">
                <label class="quick-cover-method" v-for="choice in [{value:'替换图片', title:'使用现成封面', note:'默认使用成片封面，可从图片库更换'}, {value:'使用模板', title:'模板合成', note:'选择模板、底图并填写封面文字'}]" :key="choice.value" :class="{selected: (coverChoice === '替换图片' ? '替换图片' : '使用模板') === choice.value}">
                  <input type="radio" name="quick-cover-method" :value="choice.value" :checked="(coverChoice === '替换图片' ? '替换图片' : '使用模板') === choice.value" @change="setCoverMethod(choice.value)" />
                  <span><strong>{{ choice.title }}</strong><small>{{ choice.note }}</small></span>
                </label>
              </div>
              <div class="quick-cover-workspace">
                <button
                  class="quick-cover-image"
                  @click="emit('preview', { row, kind: 'cover' })"
                >
                  <img :src="row.cover" alt="当前发布封面" /><span
                    >查看最终封面</span
                  >
                </button>
                <div class="quick-cover-controls">
                  <label
                    v-if="coverChoice !== '替换图片'"
                    class="quick-cover-source"
                    ><RequiredLabel>封面模板</RequiredLabel
                    ><t-select
                      :value="row.templateId"
                      placeholder="选择封面模板"
                      :options="
                        db.templates
                          .filter((t) => t.enabled)
                          .map((t) => ({ label: t.name, value: t.id }))
                      "
                      @change="setTemplate"
                  /></label>
                  <label class="quick-cover-source"
                    ><span class="quick-field-heading"><RequiredLabel :required="false">{{
                      coverChoice === "替换图片" ? "选择图片" : "底图"
                    }}</RequiredLabel><TaskFieldHelp v-if="coverChoice !== '替换图片'" label="底图" content="不选择底图时使用素材默认图片，也可从图片库选择替换。" /></span
                    ><t-select
                      :value="
                        row.imageId ||
                        (coverChoice === '替换图片' ? 0 : undefined)
                      "
                      :clearable="coverChoice !== '替换图片'"
                      placeholder="使用素材默认图片"
                      :options="[
                        {
                          label:
                            coverChoice === '替换图片'
                              ? '素材自有图片'
                              : '使用素材默认图片',
                          value: 0,
                        },
                        ...db.images
                          .filter(
                            (i) =>
                              i.enabled &&
                              i.uses.includes(
                                coverChoice === '替换图片'
                                  ? '现成封面'
                                  : '底图',
                              ),
                          )
                          .map((i) => ({ label: i.name, value: i.id })),
                      ]"
                      @change="setCoverImage"
                  /></label>
                  <template v-if="row.coverMode !== '现成封面'">
                    <label v-for="field in templateFields" :key="field.key">
                      <RequiredLabel>{{ field.label }}</RequiredLabel>
                      <t-input
                        :value="row[field.key]"
                        :placeholder="`输入${field.label}`"
                        @change="(v) => setField(field.key, v)"
                      />
                    </label>
                    <t-button
                      class="quick-generate-cover"
                      variant="outline"
                      :loading="rendering"
                      @click="renderCover"
                      >生成封面</t-button
                    >
                  </template>
                </div>
              </div>
              <p v-if="row.coverError" class="quick-warning" role="alert">{{ row.coverError }}</p>
            </div>
            <label v-if="platform !== '百家号'" data-field="description"
              ><RequiredLabel :required="platform === '百家号'">{{
                platform === "B站" ? "简介" : "正文"
              }}</RequiredLabel
              ><t-textarea
                :value="row.description"
                :autosize="{ minRows: 3, maxRows: 5 }"
                @change="(v) => setField('description', v)" /></label
            ><label v-if="platform === 'B站'" data-field="platform"
              ><RequiredLabel>标签</RequiredLabel
              ><t-input
                :value="row.tags"
                placeholder="多个标签用逗号分隔"
                @change="(v) => setField('tags', v)" /></label
            ><label v-else
              >话题<t-input
                :value="platform === '百家号' ? row.tags : row.topics"
                placeholder="添加与内容相关的话题"
                @change="(v) => setField(platform === '百家号' ? 'tags' : 'topics', v)"
            /></label>
            <div class="quick-resource-grid two">
              <label data-field="declaration"
                ><RequiredLabel>创作声明</RequiredLabel
                ><t-select
                  :value="row.declaration"
                  :options="declarationOptions(platform)"
                  placeholder="按内容事实选择"
                  @change="(v) => setField('declaration', v)" /></label
              ><label v-if="platform === 'B站'"
                ><RequiredLabel>平台分区</RequiredLabel
                ><t-select
                  :value="row.partition"
                  :options="[{ label: '游戏', value: '游戏' }]"
                  @change="(v) => setField('partition', v)"
              /></label>
            </div>
            <label v-if="platform !== '百家号'">可见范围<t-select :value="row.visibility" :options="[{ label: '公开', value: '公开' }, { label: '仅自己可见', value: '仅自己可见' }]" @change="value => setField('visibility', value)" /></label>

          </div>
        </div>
      </section>
      <PublishingExecutionSettings v-if="row" :row="row" number="03" @change="setField" />
    </div>
    <template #footer
      ><span v-if="problems.length" class="quick-footer-note" role="status">还有 {{ problems.length }} 项待处理</span
      ><t-button v-if="!editing" theme="default" variant="outline" :disabled="busy || generating || rendering" @click="save"
        >保存草稿</t-button
      ><t-button
        :disabled="generating || rendering"
        :loading="busy"
        @click="create"
        >{{ editing ? "保存" : "创建任务" }}</t-button
      ></template
    >
  </Overlay>
  <AccountPicker
    ref="accountPicker"
    :db="db"
    :platform="platform"
    :category="category"
    @select="(a) => changeResource('account', a.id)"
  />
  <Overlay v-model:visible="picker" title="选择成片" :width="900"
    ><PublishingScopeFilters :platform="platform" :category="category" />
    <div class="quick-picker-tools">
      <t-input
        v-model="search"
        clearable
        placeholder="搜索成片名称或 ID"
      /><t-radio-group v-model="unavailable"
        ><t-radio-button :value="false">可用成片</t-radio-button
        ><t-radio-button :value="true"
          >不可用原因</t-radio-button
        ></t-radio-group
      >
    </div>
    <p class="quick-help">
      {{ platform }} · {{ selectedAccount?.name }} · {{ category }}
    </p>
    <div class="quick-picker-grid">
      <article
        v-for="a in pageAssets"
        :key="a.id"
        class="quick-picker-item"
        :class="{ unavailable: !!a.reason }"
      >
        <button @click="emit('preview', { src: a.cover, title: a.title })">
          <img :src="a.cover" :alt="a.title" />
        </button>
        <div>
          <strong>{{ a.title }}</strong
          ><small>#{{ a.id }} · {{ a.duration }} · {{ a.category }}</small>
          <p v-if="a.reason">{{ a.reason }}</p>
          <t-button
            v-else
            size="small"
            variant="outline"
            @click="chooseAsset(a)"
            >选择这条</t-button
          >
        </div>
      </article>
    </div>
    <div v-if="!assets.length" class="quick-empty">
      <h3>
        {{
          search
            ? "未找到匹配成片"
            : unavailable
              ? "没有不可用成片"
              : "没有可发布的成片"
        }}
      </h3>
      <p>
        {{
          search
            ? "请调整搜索词，或清空搜索查看当前范围内的成片。"
            : "可更换账号或类目，或补充已完成且未被同平台占用的成片。"
        }}
      </p>
      <t-button v-if="search" variant="text" @click="search = ''"
        >清空搜索</t-button
      >
    </div>
    <t-pagination
      v-if="assets.length"
      v-model:current="assetPage"
      :total="assets.length"
      :page-size="6"
      :show-page-size="false"
      :show-jumper="false"
    /><template #footer
      ><t-button variant="outline" @click="picker = false"
        >返回配置</t-button
      ></template
    ></Overlay
  >
</template>
<style scoped>
.quick-account-input :deep(input:not(:disabled)) { cursor:pointer; }
.quick-cover-methods { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:12px; }
.quick-cover-methods > label { display:flex; flex-direction:row; align-items:flex-start; gap:10px; padding:14px; border:1px solid var(--wt-color-border); border-radius:6px; cursor:pointer; }
.quick-cover-methods > label.selected { border-color:var(--wt-color-primary); background:var(--wt-color-primary-light); }
.quick-cover-methods input { flex:none; margin:4px 0 0; accent-color:var(--wt-color-primary); }
.quick-publish .quick-cover-methods label.quick-cover-method > span { display:block; }
.quick-cover-methods strong { display:block; font-size:13px; font-weight:400; }
.quick-cover-methods small { display:block; margin-top:4px; font-size:12px; color:var(--wt-color-text-secondary); }
@media(max-width:680px){.quick-cover-methods{grid-template-columns:1fr;}}

.quick-publish {
  color: var(--wt-color-text-primary);
  max-width: none;
  margin: auto;
  font-size: 14px;
}
.quick-help {
  font-size: 12px;
  line-height: 1.7;
  color: var(--wt-color-text-secondary);
  margin: 8px 0 16px;
}
.quick-block {
  margin-top: var(--wt-space-block, 16px);
}
.quick-block.is-locked :deep(.t-input.t-is-disabled),
.quick-block.is-locked :deep(.t-input__inner:disabled) { color: var(--wt-color-text-secondary); -webkit-text-fill-color: var(--wt-color-text-secondary); background: var(--wt-color-bg-subtle); }
.quick-block.is-locked :deep(.t-button:disabled) { color: var(--wt-color-text-secondary); background: var(--wt-color-bg-subtle); }
.quick-block[data-field="resources"] {
  margin-top: 0;
}
.quick-field-heading { display: flex; align-items: center; justify-content: flex-start !important; gap: 6px !important; }
.quick-section-heading {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: center;
  margin-bottom: 16px;
}
.quick-section-heading h3 {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: var(--wt-font-size-section, 16px);
  line-height: var(--wt-line-height-section, 24px);
  font-weight: 500;
  margin: 0;
}
.quick-section-heading h3 > span {
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
.quick-resource-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 20px 24px;
}
.quick-resource-grid.two {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}
.quick-publish label:not(.t-radio):not(.t-radio-button):not(.t-checkbox):not(.quick-cover-method),
.quick-account-field {
  display: flex;
  flex-direction: column;
  gap: 8px;
  color: var(--wt-color-text-primary);
  font-size: 13px;
  line-height: 20px;
  font-weight: 500;
  min-width: 0;
}
.quick-publish label > span:not(.publishing-field-label) {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}
.quick-publish label small {
  color: var(--wt-color-text-assist);
  font-weight: 400;
  font-size: 12px;
}
.quick-selected-asset {
  margin-top: 20px;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  background: var(--wt-color-bg-subtle);
  border: 1px solid var(--wt-color-border);
  border-radius: 4px;
}
.quick-selected-asset img {
  width: 72px;
  height: 52px;
  object-fit: cover;
  border-radius: 4px;
}
.quick-selected-asset > div {
  flex: 1;
  min-width: 0;
}
.quick-selected-asset strong {
  font-size: 14px;
  font-weight: 500;
  display: block;
}
.quick-selected-asset small {
  font-size: 12px;
  display: block;
  color: var(--wt-color-text-secondary);
  margin-top: 4px;
}
.quick-content-grid {
  min-width: 0;
}
.quick-copy-fields {
  display: flex;
  flex-direction: column;
  gap: 24px;
}
.quick-cover-fields {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
}
.quick-cover-fields h4 {
  margin: 0;
  font-size: 13px;
  line-height: 20px;
  font-weight: 500;
}
.quick-cover-fields :deep(.t-radio-group) {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  width: fit-content;
  max-width: 100%;
}
.quick-cover-fields :deep(.t-radio-button) {
  justify-content: center;
  border: 1px solid var(--wt-color-border-control);
  border-radius: 4px;
  margin: 0;
  box-sizing: border-box;
}
.quick-cover-workspace {
  display: grid;
  grid-template-columns: 220px minmax(0, 1fr);
  align-items: start;
  gap: 24px;
}
.quick-cover-controls {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20px 24px;
  align-items: start;
  min-width: 0;
}
.quick-cover-source {
  grid-column: 1 / -1;
}
.quick-generate-cover {
  align-self: end;
  justify-self: start;
}
.quick-cover-fields :deep(.t-radio-button::before),
.quick-cover-fields :deep(.t-radio-group__bg) {
  display: none;
}
.quick-cover-fields :deep(.t-radio-button.t-is-checked) {
  background: var(--wt-color-primary-light);
  border-color: var(--wt-color-primary);
  color: var(--wt-color-primary);
}
.quick-cover-image {
  border: 1px solid var(--wt-color-border);
  padding: 0;
  border-radius: 4px;
  overflow: hidden;
  background: var(--wt-color-bg-subtle);
  cursor: zoom-in;
  width: 100%;
}
.quick-cover-image img {
  display: block;
  width: 100%;
  height: 165px;
  object-fit: contain;
}
.quick-cover-image span {
  display: block;
  padding: 8px;
  color: var(--wt-color-primary);
  font-size: 12px;
}
.quick-publish input[type="file"] {
  font-size: 12px;
  color: var(--wt-color-text-secondary);
  font-weight: 400;
  width: 100%;
}
.quick-notice {
  padding: 10px 12px;
  background: var(--wt-color-info-bg);
  color: var(--wt-color-info-text);
  font-size: 13px;
  border-radius: 4px;
}
.quick-warning {
  color: var(--wt-color-warning-text);
  font-size: 12px;
}
.quick-footer-note {
  margin-right: auto;
  color: var(--wt-color-text-secondary);
  font-size: 12px;
}
.quick-demo {
  margin-top: 24px;
  font-size: 12px;
  color: var(--wt-color-text-assist);
}
.quick-demo summary {
  cursor: pointer;
}
.quick-demo .t-select {
  margin-top: 12px;
}
.quick-picker-tools {
  display: flex;
  gap: 16px;
}
.quick-picker-tools > .t-input__wrap {
  flex: 1;
  min-width: 0;
}
.quick-picker-tools > .t-radio-group {
  flex: none;
  white-space: nowrap;
}
.quick-picker-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
  margin: 20px 0;
}
.quick-picker-item {
  padding: 12px;
  border: 1px solid var(--wt-color-border);
  border-radius: 4px;
  display: flex;
  gap: 12px;
  align-items: start;
}
.quick-picker-item > button {
  border: 0;
  padding: 0;
  background: none;
  cursor: zoom-in;
  flex: none;
}
.quick-picker-item img {
  display: block;
  width: 88px;
  height: 66px;
  object-fit: cover;
  border-radius: 4px;
}
.quick-picker-item strong {
  font-size: 13px;
  line-height: 1.6;
  font-weight: 500;
}
.quick-picker-item small {
  font-size: 12px;
  display: block;
  color: var(--wt-color-text-secondary);
  margin: 4px 0 8px;
}
.quick-picker-item p {
  font-size: 12px;
  color: var(--wt-color-warning-text);
}
.quick-empty {
  text-align: center;
  padding: 40px 16px;
  color: var(--wt-color-text-secondary);
}
.quick-empty h3 {
  font-size: 14px;
  font-weight: 500;
}
.quick-empty p {
  font-size: 13px;
}
@media (max-width: 700px) {
  .quick-resource-grid,
  .quick-cover-workspace,
  .quick-cover-controls,
  .quick-picker-grid,
  .quick-resource-grid.two {
    grid-template-columns: 1fr;
  }
  .quick-cover-image {
    max-width: 220px;
  }
  .quick-section-heading {
    align-items: flex-start;
    flex-wrap: wrap;
  }
  .quick-selected-asset {
    flex-wrap: wrap;
  }
  .quick-selected-asset > div { flex: 1 1 calc(100% - 90px); }
  .quick-selected-asset strong {
    overflow-wrap: anywhere;
  }
  .quick-picker-tools {
    flex-direction: column;
  }
  .quick-footer-note {
    display: none;
  }
}
</style>
