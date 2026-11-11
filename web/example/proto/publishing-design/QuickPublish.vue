<script setup>
import { computed, nextTick, ref, watch } from "vue";
import Overlay from "./PrototypeOverlay.vue";
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
  if(next.coverMode === "现成封面") await renderCover();
}
function setField(field, value) {
  editField(row.value, field, value);
  row.value.manualFields = [...new Set([...manual.value, field])];
  notice.value = "";
}
async function generate() {
  error.value = "AI 服务尚未接入，未生成或覆盖任何内容。请手动填写。";
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
    :title="editing ? `编辑发布内容 · #${editing.id}` : '快速发布'"
    workspace
    kind="drawer"
    :width="1060"
    :busy="busy || generating || rendering"
    @update:visible="close"
  >
    <div ref="form" class="quick-publish">
      <header class="quick-intro">
        <h2>{{ editing ? "编辑发布内容" : "快速发布" }}</h2>
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
            >发布平台<t-select
              :value="platform"
              :disabled="!!editing"
              :options="platforms.map((p) => ({ label: p, value: p }))"
              @change="(v) => changeResource('platform', v)" /></label
          ><label
            >发布类目<t-select
              :value="category"
              :disabled="!platform || !!editing"
              :options="categoryOptions.map((c) => ({ label: c, value: c }))"
              @change="(v) => changeResource('category', v)" /></label
          ><label
            >发布账号<t-button
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
          {{ selectedAccount.remainingToday ?? "未知" }}
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
                ? `成片 #${selectedAsset.id} · ${selectedAsset.duration} · ${selectedAsset.category}`
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
        <p class="quick-help">
          AI 服务未接入时不会生成假建议；创作声明请按实际内容确认。
        </p>
        <div class="quick-content-grid">
          <div class="quick-copy-fields">
            <label v-if="platform !== '百家号'" data-field="title"
              ><span
                >发布标题
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
              >{{
                platform === "百家号"
                  ? "作品描述 *"
                  : platform === "B站"
                    ? "简介（选填）"
                    : "正文（选填）"
              }}<t-textarea
                :value="row.description"
                :autosize="{ minRows: 3, maxRows: 5 }"
                @change="(v) => setField('description', v)" /></label
            ><label v-if="platform === 'B站'" data-field="platform"
              >标签 *<t-input
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
                >创作声明 *<t-select
                  :value="row.declaration"
                  :options="declarationOptions(platform)"
                  placeholder="按内容事实选择"
                  @change="(v) => setField('declaration', v)" /></label
              ><label v-if="platform === 'B站'"
                >平台分区 *<t-select
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
            <t-radio-group
              :value="coverChoice"
              @change="
                (v) => {
                  coverChoice = v;
                  row.coverTreatment = v;
                  setField(
                    'coverMode',
                    v === '替换图片' ? '现成封面' : '模板合成',
                  );
                  if (v === '当前素材图 + 模板')
                    row.coverSource = selectedAsset?.cover;
                  row.coverError = v === '替换图片' ? '' : '请点击生成封面';
                }
              "
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
              >{{ row.coverMode !== "现成封面" ? "选择底图" : "封面来源"
              }}<t-select
                :value="row.imageId || 0"
                :options="[
                  { label: '成片自带封面', value: 0 },
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
                >封面短语 *<t-input
                  :value="row.shortTitle"
                  @change="(v) => setField('shortTitle', v)" /></label
              ><label
                >合成模板 *<t-select
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
      <details class="quick-demo">
        <summary>本地演示 · 异常场景</summary>
        <t-select
          v-model="failure"
          :options="
            ['正常', 'AI生成失败', '封面生成失败', '保存失败'].map((v) => ({
              label: v,
              value: v === '正常' ? '' : v,
            }))
          "
        />
      </details>
    </div>
    <template #footer
      ><span class="quick-footer-note">{{
        problems.length
          ? `还有 ${problems.length} 项待处理`
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
  color: #263d5b;
  max-width: 1000px;
  margin: auto;
}
.quick-intro {
  margin: 0 0 24px;
}
.quick-intro > span {
  font-size: 11px;
  letter-spacing: 0.08em;
  color: #4c7cbd;
}
.quick-intro h2 {
  margin: 7px 0 8px;
  font-size: 25px;
  font-weight: 600;
}
.quick-intro p,
.quick-help {
  font-size: 12px;
  line-height: 1.7;
  color: #7b8ba0;
}
.quick-block {
  padding: 22px 0;
  border-top: 1px solid #e4eaf2;
}
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
  font-size: 16px;
  margin: 0;
}
.quick-section-heading h3 > span {
  font-size: 11px;
  color: #527fbd;
  background: #eef4fc;
  padding: 5px;
  border-radius: 4px;
}
.quick-section-heading > small {
  font-size: 11px;
  color: #8492a5;
}
.quick-resource-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
}
.quick-resource-grid.two {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}
.quick-publish label:not(.t-radio):not(.t-checkbox) {
  display: flex;
  flex-direction: column;
  gap: 8px;
  color: #536880;
  font-size: 12px;
  min-width: 0;
}
.quick-selected-asset {
  margin-top: 16px;
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 14px;
  background: #f7f9fc;
  border: 1px solid #e5ebf3;
  border-radius: 8px;
}
.quick-selected-asset img {
  width: 82px;
  height: 55px;
  object-fit: cover;
  border-radius: 5px;
}
.quick-selected-asset > div {
  flex: 1;
  min-width: 0;
}
.quick-selected-asset strong {
  font-size: 13px;
  display: block;
}
.quick-selected-asset small {
  font-size: 11px;
  display: block;
  color: #8592a5;
  margin-top: 6px;
}
.quick-content-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 290px;
  gap: 28px;
}
.quick-copy-fields,
.quick-cover-fields {
  display: flex;
  flex-direction: column;
  gap: 17px;
}
.quick-copy-fields label > small {
  color: #8a98aa;
}
.quick-cover-fields {
  padding: 16px;
  background: #f7f9fc;
  border-radius: 8px;
  gap: 12px;
  align-self: start;
}
.quick-cover-fields h4 {
  margin: 0;
  font-size: 13px;
}
.quick-cover-image {
  border: 0;
  padding: 0;
  border-radius: 6px;
  overflow: hidden;
  background: #edf2f8;
  cursor: zoom-in;
}
.quick-cover-image img {
  display: block;
  width: 100%;
  max-height: 190px;
  object-fit: contain;
}
.quick-cover-image span {
  display: block;
  padding: 8px;
  color: #547eaf;
  font-size: 11px;
}
.quick-settings {
  padding-top: 6px;
}
.quick-settings summary {
  color: #3c679f;
  cursor: pointer;
  font-size: 12px;
  padding-bottom: 13px;
}
.quick-settings summary > span {
  color: #8a97aa;
  margin-left: 10px;
  font-size: 11px;
}
.quick-publish input[type="datetime-local"] {
  padding: 8px;
  border: 1px solid #dce3ed;
  border-radius: 5px;
  min-width: 0;
}
.quick-preflight {
  padding: 18px;
  background: #f7faff;
  border: 1px solid #dfe8f6;
  border-radius: 8px;
  margin-top: 7px;
}
.quick-preflight ul {
  list-style: none;
  padding: 0;
  margin: 0 0 12px;
}
.quick-preflight li {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 15px;
  font-size: 12px;
}
.quick-preflight p {
  font-size: 12px;
  color: #4c8064;
}
.quick-notice {
  padding: 10px 12px;
  background: #edf8f2;
  color: #397a5c;
  font-size: 12px;
  border-radius: 6px;
}
.quick-warning {
  color: #a66c2d;
  font-size: 12px;
}
.quick-footer-note {
  margin-right: auto;
  color: #8390a4;
  font-size: 12px;
}
.quick-demo {
  margin-top: 22px;
  font-size: 11px;
  color: #8794a5;
}
.quick-demo summary {
  cursor: pointer;
}
.quick-demo .t-select {
  margin-top: 10px;
}
.quick-picker-tools > .t-input__wrap {
  flex: 1;
  min-width: 0;
}
.quick-picker-tools > .t-radio-group {
  flex: none;
  white-space: nowrap;
}
.quick-picker-tools {
  display: flex;
  gap: 14px;
}
.quick-picker-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
  margin: 18px 0;
}
.quick-picker-item {
  padding: 12px;
  border: 1px solid #e0e7f0;
  border-radius: 8px;
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
  width: 100px;
  height: 72px;
  object-fit: cover;
  border-radius: 5px;
}
.quick-picker-item strong {
  font-size: 13px;
  line-height: 1.6;
}
.quick-picker-item small {
  font-size: 11px;
  display: block;
  color: #8390a5;
  margin: 5px 0 9px;
}
.quick-picker-item p {
  font-size: 12px;
  color: #a66c2d;
}
.quick-empty {
  text-align: center;
  padding: 40px 10px;
  color: #72849a;
}
.quick-empty h3 {
  font-size: 16px;
}
.quick-empty p {
  font-size: 12px;
}
@media (max-width: 700px) {
  .quick-resource-grid,
  .quick-content-grid,
  .quick-picker-grid {
    grid-template-columns: 1fr;
  }
  .quick-resource-grid.two {
    grid-template-columns: 1fr;
  }
  .quick-section-heading {
    align-items: flex-start;
    flex-wrap: wrap;
  }
  .quick-selected-asset {
    display: grid;
    grid-template-columns: 72px minmax(0, 1fr);
    gap: 12px;
  }
  .quick-selected-asset img {
    width: 72px;
    height: 52px;
  }
  .quick-selected-asset strong {
    overflow-wrap: anywhere;
    line-height: 1.6;
  }
  .quick-selected-asset small {
    line-height: 1.5;
  }
  .quick-selected-asset > .t-button {
    justify-self: start;
  }
  .quick-picker-tools > .t-input__wrap {
    flex: 1;
    min-width: 0;
  }
  .quick-picker-tools > .t-radio-group {
    flex: none;
    white-space: nowrap;
  }
  .quick-picker-tools {
    flex-direction: column;
  }
  .quick-footer-note {
    display: none;
  }
  .quick-intro h2 {
    font-size: 22px;
  }
  .quick-cover-fields {
    max-width: 100%;
  }
  .quick-settings summary > span {
    display: block;
    margin: 7px 0;
  }
}
</style>
