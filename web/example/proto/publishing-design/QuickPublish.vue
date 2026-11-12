<script setup>
import { computed, nextTick, ref, watch } from "vue";
import Overlay from "./PrototypeOverlay.vue";
import RequiredLabel from "./RequiredLabel.vue";
import { accountAvailability } from "./accountAvailability.js";
import AccountPicker from "./AccountPicker.vue";
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
import { ensureCoverResults, coverSpecs, coverSignature } from "./covers.js";
import { createPublication, clone } from "./relationships.js";
const props = defineProps({ db: Object, persist: Function });
const emit = defineEmits(["created", "preview"]);
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
const savedDraft = ref(""),
  editing = ref(null),
  accountPicker = ref(null);
async function edit(task) {
  coverChoice.value = task.coverTreatment || "替换图片";
  if (!canEditTask(task)) return;
  editing.value = task;
  row.value = clone(task);
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
const hasDraft = computed(
  () =>
    !!props.db.quickDraft?.active ||
    (props.db.publishDraft?.active &&
      props.db.publishDraft?.mode === "快速发布"),
);
const manual = computed(() => row.value?.manualFields || []);
const labels = {
  title: "发布标题",
  description: "正文 / 简介",
  shortTitle: "封面短标题",
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
  };
}
watch(
  [row, platform, accountId, category],
  () => {
    if (visible.value && !editing.value) {
      checked.value = false;
      props.db.quickDraft = draft();
    }
  },
  { deep: true },
);
watch([search, unavailable], () => (assetPage.value = 1));
async function open(source = null) {
  editing.value = null;
  if (hasDraft.value && !source) {
    await resume();
    return;
  }
  if (
    hasDraft.value &&
    source &&
    !(await feedback.confirmWarning({
      title: "替换未完成的快速发布配置？",
      message: "当前草稿会被替换；可取消并继续原草稿。",
      confirmText: "使用复制配置",
    }))
  )
    return;
  row.value = null;
  platform.value = source?.platform || "";
  accountId.value = source?.accountId || null;
  category.value = source?.category || "";
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
      "templateId",
      "templateVersion",
      "templateDocument",
      "templateDocuments",
      "coverVariants",
      "coverSignature",
    ];
    row.value = initCandidate(
      Object.fromEntries(
        fields
          .filter((f) => source[f] !== undefined)
          .map((f) => [f, clone(source[f])]),
      ),
    );
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
}
async function resume() {
  editing.value = null;
  const old = props.db.quickDraft || props.db.publishDraft;
  platform.value = old.platform || old.platforms?.[0] || "";
  accountId.value = old.accountId || old.quickAccountId || null;
  category.value = old.category || old.categories?.[0] || "";
  row.value = old.row
    ? clone(old.row)
    : old.rows?.[0]
      ? clone(old.rows[0])
      : null;
  creationKey.value = old.creationKey || crypto.randomUUID();
  error.value = "";
  notice.value = "已恢复草稿，资源可用性会重新检查。";
  checked.value = false;
  coverChoice.value = row.value?.coverTreatment || "替换图片";
  visible.value = true;
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
    category.value = "";
  }
  if (field === "account") {
    accountId.value = value;
    category.value ||=
      categoryOptions.value.length === 1 ? categoryOptions.value[0] : "";
  }
  if (field === "category") {
    category.value = value;
    accountId.value = null;
    notice.value = "类目已变更，请重新选择匹配账号。";
  }
}
async function chooseAsset(a) {
  if (a.reason) return;
  const next = initCandidate({
    platform: platform.value,
    accountId: accountId.value,
    assetId: a.id,
    category: a.category,
    ...defaults(platform.value, a),
    coverSource: a.cover,
    manualFields: [],
  });
  if (copied.value) {
    for (const f of [
      "title",
      "description",
      "shortTitle",
      "tags",
      "topics",
      "declaration",
      "coverMode",
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
  error.value = "暂时无法获取 AI 建议，请手动填写或稍后重试。";
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
      (t) =>
        t?.enabled &&
        Math.abs(
          t.document.width / t.document.height - spec.width / spec.height,
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
  props.db.quickDraft = draft();
  if (await persist()) {
    notice.value = "草稿已保存，不占用成片。";
    savedDraft.value = JSON.stringify(draft());
    feedback.toast.success(notice.value);
  }
  busy.value = false;
}
async function close() {
  if (editing.value) {
    visible.value = false;
    editing.value = null;
    return;
  }
  if (generating.value || rendering.value || busy.value) return;
  if (!platform.value && !row.value) {
    props.db.quickDraft = { active: false };
    visible.value = false;
    return;
  }
  if (
    JSON.stringify(draft()) !== savedDraft.value &&
    (platform.value || row.value) &&
    !(await feedback.confirmWarning({
      title: "保存草稿后关闭？",
      message: "保存后可继续配置；草稿不会占用成片或启动任务。",
      confirmText: "保存并关闭",
      cancelText: "继续编辑",
    }))
  )
    return;
  busy.value = true;
  props.db.quickDraft = draft();
  if (await persist()) visible.value = false;
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
      if (!current || !canEditTask(current))
        throw Error("任务状态已变化，不能保存修改");
      const before = clone(current);
      const fields = [
        "title",
        "description",
        "shortTitle",
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
    :title="editing ? `编辑发布内容 · ${editing.id}` : '快速发布'"
    workspace
    kind="drawer"
    :width="1060"
    :busy="busy || generating || rendering"
    @update:visible="close"
  >
    <div ref="form" class="quick-publish">
      <header class="quick-intro">
        <p>
          {{
            editing
              ? "修改当前待执行内容，保存产生新版本。"
              : "选择平台与账号，配置内容。创建任务后保持待执行。"
          }}
        </p>
      </header>
      <t-alert v-if="error" theme="warning" :message="error" />
      <p v-if="notice" class="quick-notice" role="status">{{ notice }}</p>
      <section class="quick-block" data-field="resources" tabindex="-1">
        <div class="quick-section-heading">
          <h3><span>01</span>选择发布平台与账号</h3>
          <small>一个平台 · 一个账号 · 一条成片</small>
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
          ><label
            ><RequiredLabel>发布账号</RequiredLabel
            ><t-button
              variant="outline"
              :disabled="!category || !!editing"
              @click="accountPicker.open()"
              >{{ selectedAccount?.name || "选择账号" }}</t-button
            ></label
          >
        </div>
        <p v-if="selectedAccount" class="quick-help">
          {{ selectedAccount.name }} · 等级
          {{ selectedAccount.level ?? "未同步" }} · 今日余量
          {{ selectedAvailability?.remaining ?? "未知" }}
        </p>
        <p v-if="platform && !accountOptions.length" class="quick-warning">
          当前平台没有可用账号，请先补充账号或处理登录状态。
        </p>
        <div class="quick-selected-asset">
          <img v-if="selectedAsset" :src="selectedAsset.cover" alt="已选成片" />
          <div>
            <strong>{{ selectedAsset?.title || "选择本次要发布的成片" }}</strong
            ><small>{{
              selectedAsset
                ? `成片 ${selectedAsset.id} · ${selectedAsset.duration} · ${selectedAsset.category}`
                : "只安排当前账号支持、同平台未占用的成片。"
            }}</small>
          </div>
          <t-button
            variant="outline"
            :disabled="!category || !accountId || !!editing"
            @click="
              picker = true;
              search = '';
              unavailable = false;
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
      <section v-if="row" class="quick-block">
        <div class="quick-section-heading">
          <h3><span>02</span>发布内容</h3>
          <t-button variant="outline" :loading="generating" @click="generate()"
            >AI 填写</t-button
          >
        </div>
        <div class="quick-content-grid">
          <div class="quick-copy-fields">
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
                @change="(v) => setField('title', v)" /></label
            ><label data-field="description"
              ><RequiredLabel :required="platform === '百家号'">{{
                platform === "百家号"
                  ? "作品描述"
                  : platform === "B站"
                    ? "简介（选填）"
                    : "正文（选填）"
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
              >话题（选填）<t-input
                :value="row.topics"
                placeholder="添加与内容相关的话题"
                @change="(v) => setField('topics', v)"
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
            <details class="quick-settings">
              <summary>
                更多设置
                <span
                  >{{ row.visibility || "公开" }} ·
                  {{ row.schedule || "启动后尽快提交" }}</span
                >
              </summary>
              <div class="quick-resource-grid two">
                <label v-if="platform !== '百家号'"
                  >可见范围<t-select
                    :value="row.visibility"
                    :options="[
                      { label: '公开', value: '公开' },
                      { label: '仅自己可见', value: '仅自己可见' },
                    ]"
                    @change="(v) => setField('visibility', v)" /></label
                ><label data-field="schedule"
                  >指定发布时间（选填）<input
                    type="datetime-local"
                    :value="row.schedule"
                    @change="setField('schedule', $event.target.value)"
                /></label>
              </div>
            </details>
          </div>
          <div class="quick-cover-fields" data-field="cover" tabindex="-1">
            <h4>发布封面</h4>
            <t-radio-group :value="coverChoice" @change="setCoverChoice"
              ><t-radio-button value="替换图片">替换图片</t-radio-button
              ><t-radio-button value="当前素材图 + 模板"
                >当前素材图 + 模板</t-radio-button
              ><t-radio-button value="图片库图 + 模板"
                >图片库图 + 模板</t-radio-button
              ></t-radio-group
            ><button
              class="quick-cover-image"
              @click="emit('preview', { row })"
            >
              <img :src="row.cover" alt="当前发布封面" /><span
                >点击检查各比例效果</span
              ></button
            ><label v-if="coverChoice === '替换图片'"
              >上传替换图片<input
                type="file"
                accept="image/png,image/jpeg,image/webp"
                @change="
                  async (e) => {
                    const f = e.target.files?.[0];
                    if (!f) return;
                    if (
                      !['image/png', 'image/jpeg', 'image/webp'].includes(
                        f.type,
                      )
                    ) {
                      error = '请选择 PNG、JPEG 或 WebP 图片';
                      return;
                    }
                    if (f.size > 10 * 1024 * 1024) {
                      error = '图片过大，请选择 10MB 以内图片';
                      return;
                    }
                    const reader = new FileReader();
                    reader.onload = () => {
                      row.coverSource = reader.result;
                      row.cover = reader.result;
                      row.coverVariants = {};
                      row.imageId = null;
                    };
                    reader.readAsDataURL(f);
                  }
                " /></label
            ><label v-if="coverChoice !== '当前素材图 + 模板'"
              ><RequiredLabel :required="coverChoice === '图片库图 + 模板'">{{
                row.coverMode !== "现成封面" ? "选择底图" : "封面来源"
              }}</RequiredLabel
              ><t-select
                :value="
                  row.imageId ||
                  (coverChoice === '图片库图 + 模板' ? undefined : 0)
                "
                :options="[
                  ...(coverChoice === '图片库图 + 模板'
                    ? []
                    : [{ label: '成片自带封面', value: 0 }]),
                  ...db.images
                    .filter(
                      (i) =>
                        i.enabled &&
                        i.uses.includes(
                          row.coverMode !== '现成封面' ? '底图' : '现成封面',
                        ),
                    )
                    .map((i) => ({ label: i.name, value: i.id })),
                ]"
                @change="image" /></label
            ><template v-if="row.coverMode !== '现成封面'"
              ><label
                ><RequiredLabel>封面短语</RequiredLabel
                ><t-input
                  :value="row.shortTitle"
                  @change="(v) => setField('shortTitle', v)" /></label
              ><label
                ><RequiredLabel>合成模板</RequiredLabel
                ><t-select
                  :value="row.templateId"
                  :options="
                    db.templates
                      .filter((t) => t.enabled)
                      .map((t) => ({ label: t.name, value: t.id }))
                  "
                  @change="setTemplate" /></label
              ><t-button
                variant="text"
                :loading="rendering"
                @click="renderCover"
                >生成封面</t-button
              ></template
            >
            <p v-else class="quick-help">直接使用图片，无需封面短标题。</p>
            <small v-if="platform === '百家号'" class="quick-help"
              >横竖封面规格待平台核验，当前按原图预览。</small
            >
          </div>
        </div>
      </section>
      <t-alert v-if="error" theme="warning" :message="error" />
      <p v-if="row?.coverError" class="quick-warning">{{ row.coverError }}</p>
    </div>
    <template #footer
      ><span class="quick-footer-note">{{
        problems.length
          ? `还有 ${problems.length} 项待处理`
          : editing
            ? "保存后更新内容版本，任务保持待执行"
            : "创建后待执行，不会立即发布"
      }}</span
      ><t-button
        theme="default"
        variant="outline"
        @click="editing ? (visible = false) : close()"
        >{{ editing ? "取消修改" : "取消" }}</t-button
      ><t-button v-if="!editing" variant="outline" @click="save"
        >保存草稿</t-button
      ><t-button
        :disabled="generating || rendering"
        :loading="busy"
        @click="create"
        >{{ editing ? "保存修改" : "创建任务" }}</t-button
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
    ><div class="quick-picker-tools">
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
.quick-publish {
  color: var(--wt-color-text-primary);
  max-width: 1120px;
  margin: auto;
  font-size: 14px;
}
.quick-intro {
  margin: 0 0 24px;
}
.quick-intro p,
.quick-help {
  font-size: 12px;
  line-height: 1.7;
  color: var(--wt-color-text-secondary);
  margin: 8px 0 16px;
}
.quick-intro p {
  margin: 0;
}
.quick-block {
  padding: 24px 0 0;
  margin-top: 24px;
  border-top: 1px solid var(--wt-color-border);
}
.quick-block[data-field="resources"] {
  padding-top: 0;
  margin-top: 0;
  border-top: 0;
}
.quick-section-heading {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: center;
  margin-bottom: 20px;
}
.quick-section-heading h3 {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 14px;
  font-weight: 600;
  margin: 0;
}
.quick-section-heading h3 > span {
  font-size: 13px;
  color: var(--wt-color-primary);
  font-weight: 500;
}
.quick-section-heading > small {
  font-size: 12px;
  color: var(--wt-color-text-secondary);
}
.quick-resource-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 20px 24px;
}
.quick-resource-grid.two {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}
.quick-publish label:not(.t-radio):not(.t-radio-button):not(.t-checkbox) {
  display: flex;
  flex-direction: column;
  gap: 8px;
  color: var(--wt-color-text-primary);
  font-size: 13px;
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
  font-size: 13px;
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
  display: grid;
  grid-template-columns: minmax(0, 1fr) 320px;
  gap: 28px;
}
.quick-copy-fields,
.quick-cover-fields {
  display: flex;
  flex-direction: column;
  gap: 20px;
}
.quick-cover-fields {
  padding: 16px;
  border: 1px solid var(--wt-color-border);
  border-radius: 4px;
  gap: 16px;
  align-self: start;
}
.quick-cover-fields h4 {
  margin: 0;
  font-size: 14px;
  font-weight: 500;
}
.quick-cover-fields :deep(.t-radio-group) {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.quick-cover-fields :deep(.t-radio-button) {
  width: 100%;
  justify-content: flex-start;
  border: 1px solid var(--wt-color-border-control);
  border-radius: 4px;
  margin: 0;
  box-sizing: border-box;
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
}
.quick-cover-image img {
  display: block;
  width: 100%;
  max-height: 180px;
  object-fit: contain;
}
.quick-cover-image span {
  display: block;
  padding: 8px;
  color: var(--wt-color-primary);
  font-size: 12px;
}
.quick-settings {
  padding-top: 4px;
}
.quick-settings summary {
  color: var(--wt-color-primary);
  cursor: pointer;
  font-size: 13px;
  padding-bottom: 16px;
}
.quick-settings summary > span {
  color: var(--wt-color-text-secondary);
  margin-left: 12px;
  font-size: 12px;
}
.quick-publish input[type="datetime-local"] {
  box-sizing: border-box;
  height: 36px;
  padding: 0 12px;
  border: 1px solid var(--wt-color-border-control);
  border-radius: 4px;
  color: var(--wt-color-text-primary);
  background: var(--wt-color-bg-container);
  min-width: 0;
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
@media (max-width: 900px) {
  .quick-content-grid {
    grid-template-columns: minmax(0, 1fr) 280px;
    gap: 24px;
  }
}
@media (max-width: 700px) {
  .quick-resource-grid,
  .quick-content-grid,
  .quick-picker-grid,
  .quick-resource-grid.two {
    grid-template-columns: 1fr;
  }
  .quick-section-heading {
    align-items: flex-start;
    flex-wrap: wrap;
  }
  .quick-selected-asset {
    flex-wrap: wrap;
  }
  .quick-selected-asset strong {
    overflow-wrap: anywhere;
  }
  .quick-picker-tools {
    flex-direction: column;
  }
  .quick-footer-note {
    display: none;
  }
  .quick-settings summary > span {
    display: block;
    margin: 8px 0;
  }
}
</style>
