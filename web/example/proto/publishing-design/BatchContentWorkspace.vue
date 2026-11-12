<script setup>
import { computed, ref, watch } from "vue";
import {
  FilterIcon,
  StarIcon as SparklesIcon,
  CheckIcon,
  EditIcon,
} from "tdesign-icons-vue-next";
import ResourceOverlay from "./PrototypeOverlay.vue";
import {
  initCandidate,
  editField,
  requestSuggestion,
  receiveSuggestion,
  adopt,
  demoValues,
  invalidateCover,
} from "./content.js";
import { ensureCoverResults, coverSpecs, coverSignature } from "./covers.js";
import { validate, declarationOptions } from "./model.js";
import { feedback } from "../../../src/shared/ui/feedback/index.js";
const props = defineProps({
  rows: { type: Array, required: true },
  db: { type: Object, required: true },
});
const emit = defineEmits(["preview", "working"]);
const clone = (v) => JSON.parse(JSON.stringify(v));
const labels = {
  title: "发布标题",
  shortTitle: "封面短标题",
  description: "简介 / 正文",
  topics: "话题标签",
  tags: "平台标签",
  subTitle: "封面短标语",
};
const fieldKeys = Object.keys(labels);
const selected = ref([]),
  activeId = ref(null),
  category = ref("全部"),
  status = ref("全部"),
  search = ref(""),
  filterOpen = ref(false),
  error = ref(""),
  notice = ref("");
const aiOpen = ref(false),
  aiScope = ref("筛选结果"),
  aiFields = ref([...fieldKeys]),
  style = ref("实战攻略"),
  busy = ref(false),
  editOpen = ref(false),
  editing = ref(null),
  editOriginal = ref(""),
  saving = ref(false),
  editError = ref("");
const settingsOpen = ref(false),
  declaration = ref(""),
  visibility = ref("公开");
function applySettings() {
  if (!declaration.value) return;
  let n = 0;
  for (const row of props.rows) {
    if (!(row.overrides || []).includes("declaration"))
      editField(row, "declaration", declaration.value);
    if (!(row.overrides || []).includes("visibility"))
      editField(row, "visibility", visibility.value);
    n++;
  }
  settingsOpen.value = false;
  notice.value = `已应用到 ${n} 条任务，单条明确修改的字段保留；请重新确认当前内容。`;
}
const categories = computed(() => [
  ...new Set(props.rows.map((r) => r.category)),
]);
const account = (id) => props.db.accounts.find((a) => a.id === id);
const asset = (id) => props.db.assets.find((v) => v.id === id);
const hasSuggestion = (r) =>
  Object.values(r.ai?.suggestions || {}).some(
    (s) => s.status === "待采用" && !s.error && !s.expired,
  );
const rowStatus = (r) =>
  hasSuggestion(r)
    ? "有 AI 建议"
    : validate(r, props.db, r.id).length
      ? "待配置"
      : "已配置";
const filtered = computed(() =>
  props.rows.filter(
    (r) =>
      (category.value === "全部" || category.value === r.category) &&
      (status.value === "全部" || status.value === rowStatus(r)) &&
      `${r.title} ${r.category} ${account(r.accountId)?.name} ${r.assetId}`.includes(
        search.value,
      ),
  ),
);
const active = computed(() =>
  props.rows.find((r) => r.candidateId === activeId.value),
);
watch(
  () => props.rows,
  (rows) => {
    rows.forEach(initCandidate);
    if (!rows.some((r) => r.candidateId === activeId.value))
      activeId.value = rows[0]?.candidateId || null;
    selected.value = selected.value.filter((id) =>
      rows.some((r) => r.candidateId === id),
    );
  },
  { immediate: true, deep: false },
);
watch(filtered, (rows) => {
  if (!rows.some((r) => r.candidateId === activeId.value))
    activeId.value = rows[0]?.candidateId || null;
});
const ready = computed(
  () => props.rows.filter((r) => !validate(r, props.db, r.id).length).length,
);
const allSelected = computed(
  () =>
    filtered.value.length > 0 &&
    filtered.value.every((r) => selected.value.includes(r.candidateId)),
);
function selectAll(checked) {
  selected.value = checked
    ? [
        ...new Set([
          ...selected.value,
          ...filtered.value.map((r) => r.candidateId),
        ]),
      ]
    : selected.value.filter(
        (id) => !filtered.value.some((r) => r.candidateId === id),
      );
}
function scopeRows() {
  return aiScope.value === "勾选任务"
    ? props.rows.filter((r) => selected.value.includes(r.candidateId))
    : aiScope.value === "当前任务"
      ? [active.value].filter(Boolean)
      : filtered.value;
}
function openAI(single = false) {
  aiScope.value = single
    ? "当前任务"
    : selected.value.length
      ? "勾选任务"
      : "筛选结果";
  aiOpen.value = true;
  error.value = "";
}
async function generate() {
  const rows = scopeRows();
  if (!rows.length || !aiFields.value.length) {
    error.value = "请选择至少一条任务与一个生成字段。";
    return;
  }
  busy.value = true;
  error.value = "";
  // Local suggestions are labelled as demonstrations and never call a model service.
  for (const row of rows) {
    const req = requestSuggestion(row, aiFields.value);
    const values = {
      ...demoValues(row, style.value),
      subTitle: "实战记录 · 进阶思路",
    };
    receiveSuggestion(row, req, values);
    row.ai.progress = "演示建议已生成";
  }
  busy.value = false;
  aiOpen.value = false;
  notice.value = `已为 ${rows.length} 条任务生成本地演示建议，当前采用内容未改变。请预览后确认采用。`;
}
function adoptField(row, field) {
  if (adopt(row, field)) {
    row.contentVersion = (row.contentVersion || 1) + 1;
    notice.value = `已采用${labels[field]}，请重新确认内容。`;
  } else error.value = "建议已过期或已采用，请重新生成当前字段。";
}
function adoptBatch() {
  let n = 0;
  const rows = selected.value.length
    ? props.rows.filter((r) => selected.value.includes(r.candidateId))
    : filtered.value;
  for (const row of rows)
    for (const f of fieldKeys)
      if (!String(row[f] || "").trim() && adopt(row, f)) n++;
  notice.value = `已补齐 ${n} 个空白字段；已有人工内容与过期建议保持原样。请检查并确认内容。`;
}
function edit(row) {
  editing.value = clone(row);
  editOriginal.value = JSON.stringify(editing.value);
  editError.value = "";
  editOpen.value = true;
}
async function closeEdit() {
  if (
    JSON.stringify(editing.value) !== editOriginal.value &&
    !(await feedback.confirmWarning({
      title: "放弃本条未保存修改？",
      message: "当前采用版本和其他任务保持不变。",
      confirmText: "放弃修改",
    }))
  )
    return;
  editOpen.value = false;
}
function setCoverMode(value) {
  editField(editing.value, "coverMode", value);
  if (value !== "模板合成") return;
  if (!editing.value.imageId) {
    const background = props.db.images.find(
      (i) => i.enabled && i.pureBackground && i.uses.includes("底图"),
    );
    if (background) setImage(background.id);
  }
  if (!editing.value.templateId) {
    const spec = coverSpecs(editing.value.platform)[0];
    const template = props.db.templates.find(
      (t) =>
        t.enabled &&
        (!spec ||
          Math.abs(
            t.document.width / t.document.height - spec.width / spec.height,
          ) < 0.01),
    );
    if (template) setTemplate(template.id);
  }
}
function setTemplate(id) {
  const row = editing.value,
    t = props.db.templates.find((t) => t.id === id);
  editField(row, "templateId", t?.id || null);
  row.templateVersion = t?.version || 1;
  row.templateDocument = t ? clone(t.document) : null;
  row.templateDocuments = {};
  row.templateSnapshots = {};
  for (const spec of coverSpecs(row.platform)) {
    const candidate = [t, ...props.db.templates].find(
      (t) =>
        t?.enabled &&
        Math.abs(
          t.document.width / t.document.height - spec.width / spec.height,
        ) < 0.01,
    );
    if (candidate) {
      row.templateDocuments[spec.key] = clone(candidate.document);
      row.templateSnapshots[spec.key] = {
        id: candidate.id,
        version: candidate.version || 1,
      };
    }
  }
  invalidateCover(row);
}
function setImage(id) {
  const image = props.db.images.find((i) => i.id === id);
  if (image) {
    editField(editing.value, "coverSource", image.src);
    editing.value.imageId = image.id;
    editing.value.imageVersion = image.version || 1;
  } else {
    editField(
      editing.value,
      "coverSource",
      asset(editing.value.assetId)?.cover,
    );
    editing.value.imageId = null;
  }
}
const usableImages = computed(() =>
  props.db.images.filter(
    (i) =>
      i.enabled &&
      i.uses.includes(
        editing.value?.coverMode === "模板合成" ? "底图" : "现成封面",
      ),
  ),
);
const usableTemplates = computed(() =>
  props.db.templates.filter((t) => t.enabled),
);
async function renderEdit() {
  saving.value = true;
  editError.value = "";
  try {
    await ensureCoverResults(editing.value);
  } catch (e) {
    editError.value = e.message;
  } finally {
    saving.value = false;
  }
}
async function saveEdit() {
  saving.value = true;
  editError.value = "";
  try {
    await ensureCoverResults(editing.value);
    const row = editing.value;
    row.contentApproved = true;
    row.coverApproved = true;
    const errors = validate(row, props.db, row.id);
    if (errors.length) {
      editError.value = errors.join("；");
      return;
    }
    const current = props.rows.find((r) => r.candidateId === row.candidateId);
    if (!current) {
      editError.value = "该候选任务已移出，请关闭后重新选择。";
      return;
    }
    row.contentVersion = (current.contentVersion || 1) + 1;
    row.overrides = [
      ...new Set([
        ...(current.overrides || []),
        ...[
          ...fieldKeys,
          "declaration",
          "visibility",
          "schedule",
          "partition",
        ].filter((f) => current[f] !== row[f]),
      ]),
    ];
    Object.assign(current, clone(row));
    editOpen.value = false;
    notice.value = "已保存并采用此版本，内容与当前实际封面已确认。";
  } catch (e) {
    editError.value = e.message || "保存失败，当前输入已保留，可修复后重试。";
  } finally {
    saving.value = false;
  }
}
async function approve(row) {
  emit("working", true);
  error.value = "";
  try {
    await ensureCoverResults(row);
    row.contentApproved = true;
    row.coverApproved = true;
    const errors = validate(row, props.db, row.id);
    if (errors.length) {
      row.contentApproved = false;
      error.value = errors.join("；");
      edit(row);
      return;
    }
    notice.value = "";
    feedback.toast.success("当前内容与实际封面已确认");
  } catch (e) {
    error.value = e.message;
  } finally {
    emit("working", false);
  }
}
async function approveBatch() {
  const rows = selected.value.length
    ? props.rows.filter((r) => selected.value.includes(r.candidateId))
    : filtered.value;
  if (!rows.length) return;
  if (
    !(await feedback.confirmWarning({
      title: `确认 ${rows.length} 条内容与封面`,
      message:
        "请先逐条检查当前实际文案与封面。此操作只确认当前版本，后续修改会使确认失效。",
      confirmText: "确认已检查并采用",
    }))
  )
    return;
  emit("working", true);
  let n = 0;
  for (const row of rows) {
    try {
      await ensureCoverResults(row);
      const test = { ...row, contentApproved: true, coverApproved: true };
      if (!validate(test, props.db, test.id).length) {
        row.contentApproved = true;
        row.coverApproved = true;
        n++;
      }
    } catch (e) {
      error.value = `成片 ${row.assetId}：${e.message}，请编辑该条修复。`;
    }
  }
  notice.value = "";
  if (n === rows.length) feedback.toast.success(`${n} 条内容与封面已确认`);
  else
    error.value = `${n} / ${rows.length} 条已确认；请编辑未通过的任务，补充声明或必填内容。`;
  emit("working", false);
}
defineExpose({ edit });
</script>
<template>
  <section class="batch-content-workspace">
    <div class="batch-content-heading">
      <div>
        <h3>内容配置</h3>
        <p>
          以匹配的账号与成片为基础，按需生成建议、批量采用或逐条完善发布文案。
        </p>
      </div>
      <div>
        <t-button variant="outline" @click="settingsOpen = true"
          >批次平台设置</t-button
        ><t-tag
          :theme="ready === rows.length ? 'success' : 'warning'"
          variant="light"
          >{{ ready }} / {{ rows.length }} 条已配置</t-tag
        ><span>AI 为可选工具</span>
      </div>
    </div>
    <t-alert
      v-if="error"
      theme="warning"
      :message="error"
      close-btn
      @close="error = ''"
      class="batch-message"
    />
    <t-alert
      v-if="notice"
      theme="info"
      :message="notice"
      close-btn
      @close="notice = ''"
      class="batch-message"
    />
    <div class="batch-content-layout">
      <aside class="batch-task-list">
        <header class="batch-list-head">
          <strong>全部 ({{ rows.length }})</strong
          ><t-popup
            trigger="click"
            placement="bottom-left"
            v-model:visible="filterOpen"
            :z-index="5500"
            ><t-button variant="outline" size="small"
              ><template #icon><FilterIcon /></template>筛选{{
                category !== "全部" || status !== "全部" ? " · 已生效" : ""
              }}</t-button
            ><template #content
              ><div class="batch-filter-popup">
                <label
                  >类目<t-select
                    :popup-props="{ attach: 'body', zIndex: 5500 }"
                    v-model="category"
                    :options="
                      ['全部', ...categories].map((c) => ({
                        label: c,
                        value: c,
                      }))
                    " /></label
                ><label
                  >内容状态<t-select
                    :popup-props="{ attach: 'body', zIndex: 5500 }"
                    v-model="status"
                    :options="
                      ['全部', '待配置', '已配置', '有 AI 建议'].map((c) => ({
                        label: c,
                        value: c,
                      }))
                    " /></label
                ><t-input
                  v-model="search"
                  placeholder="搜索账号、成片或标题"
                /><t-button
                  variant="text"
                  @click="
                    category = '全部';
                    status = '全部';
                    search = '';
                  "
                  >重置筛选</t-button
                >
              </div></template
            ></t-popup
          ><small>{{ filtered.length }} 条</small>
        </header>
        <div class="batch-list-toolbar">
          <t-checkbox
            :checked="allSelected"
            :indeterminate="selected.length > 0 && !allSelected"
            @change="selectAll"
            >全选 <small>({{ selected.length }})</small></t-checkbox
          ><t-button size="small" variant="outline" @click="openAI()"
            ><template #icon><SparklesIcon /></template>批量 AI 生成</t-button
          ><t-button size="small" variant="outline" @click="adoptBatch"
            >批量采用</t-button
          >
        </div>
        <div class="batch-task-scroll">
          <div
            v-for="row in filtered"
            :key="row.candidateId"
            class="batch-task-item"
            :class="{ active: row.candidateId === activeId }"
          >
            <t-checkbox
              :checked="selected.includes(row.candidateId)"
              @change="
                (checked) => {
                  selected = checked
                    ? [...selected, row.candidateId]
                    : selected.filter((id) => id !== row.candidateId);
                }
              "
              :aria-label="'选择成片 ' + row.assetId"
            /><button
              class="batch-task-button"
              @click="activeId = row.candidateId"
            >
              <img :src="asset(row.assetId)?.cover" alt="" /><span
                ><b>{{ row.title || asset(row.assetId)?.title }}</b
                ><small
                  >{{ row.category }} · {{ account(row.accountId)?.name }} ·
                  {{ asset(row.assetId)?.duration }}</small
                ></span
              ><t-tag
                size="small"
                :theme="
                  rowStatus(row) === '已配置'
                    ? 'success'
                    : rowStatus(row) === '有 AI 建议'
                      ? 'primary'
                      : 'warning'
                "
                variant="light"
                >{{ rowStatus(row) }}</t-tag
              >
            </button>
          </div>
          <div v-if="!filtered.length" class="batch-empty">
            <p>没有符合条件的任务</p>
            <t-button
              variant="text"
              @click="
                category = '全部';
                status = '全部';
                search = '';
              "
              >清除筛选</t-button
            >
          </div>
        </div>
        <footer class="batch-list-footer">
          <span>已选 {{ selected.length }} 条</span
          ><t-button variant="text" size="small" @click="approveBatch"
            >确认{{ selected.length ? "所选" : "筛选结果" }}内容与封面</t-button
          >
        </footer>
      </aside>
      <article v-if="active" class="batch-content-detail">
        <header class="batch-detail-head">
          <div>
            <h3>{{ asset(active.assetId)?.title }}</h3>
            <p>
              {{ active.category }} · 第 {{ rows.indexOf(active) + 1 }} /
              {{ rows.length }} 条候选 · 当前采用 V{{
                active.contentVersion || 1
              }}
            </p>
          </div>
          <t-button variant="outline" @click="edit(active)"
            ><template #icon><EditIcon /></template>编辑内容</t-button
          >
        </header>
        <div class="batch-detail-body">
          <div class="batch-resource-context">
            <span
              >账号 <b>{{ account(active.accountId)?.name }}</b></span
            ><span
              >成片 <b>V-{{ active.assetId }}</b></span
            ><small>资源只读</small>
          </div>
          <div class="batch-reading-grid">
            <div class="batch-reading-content">
              <section
                v-for="field in fieldKeys"
                :key="field"
                class="batch-read-field"
              >
                <label
                  >{{ labels[field] }}
                  <t-tag
                    v-if="active.ai?.suggestions?.[field]?.status === '已采用'"
                    theme="success"
                    size="small"
                    variant="light"
                    >已采用</t-tag
                  ></label
                >
                <p
                  :class="{
                    'is-title': field === 'title',
                    'is-empty': !active[field],
                  }"
                >
                  {{ active[field] || "未设置" }}
                </p>
                <div
                  v-if="
                    active.ai?.suggestions?.[field] &&
                    active.ai.suggestions[field].status !== '已采用'
                  "
                  class="batch-suggestion"
                >
                  <small
                    >本地演示建议 ·
                    {{ active.ai.suggestions[field].status }}</small
                  >
                  <p>
                    {{
                      active.ai.suggestions[field].value ||
                      active.ai.suggestions[field].error
                    }}
                  </p>
                  <t-button
                    size="small"
                    variant="text"
                    :disabled="
                      active.ai.suggestions[field].expired ||
                      !!active.ai.suggestions[field].error
                    "
                    @click="adoptField(active, field)"
                    >采用此建议</t-button
                  >
                </div>
              </section>
              <section class="batch-read-field batch-platform-read">
                <label>平台设置</label>
                <p>
                  {{ active.declaration || "创作声明待确认" }} ·
                  {{ active.visibility || "公开" }}
                </p>
                <p>
                  {{ active.schedule || "启动后尽快发布"
                  }}<template v-if="active.partition">
                    · {{ active.partition }}</template
                  >
                </p>
              </section>
            </div>
            <aside class="batch-current-cover">
              <label>当前采用封面</label
              ><button @click="emit('preview', { kind: 'cover', row: active })">
                <img :src="active.cover" alt="当前实际封面" /></button
              ><t-tag
                size="small"
                :theme="
                  active.coverSignature === coverSignature(active)
                    ? 'success'
                    : 'warning'
                "
                variant="light"
                >{{
                  active.coverSignature === coverSignature(active)
                    ? "实际输出"
                    : "待重新生成"
                }}</t-tag
              ><small
                >{{ active.coverMode }} ·
                {{ active.coverApproved ? "已确认" : "待确认" }}</small
              >
              <p>
                封面文字与实际图片独立。修改短标题后，请在编辑内容中生成并确认新封面。
              </p>
              <t-button
                size="small"
                variant="outline"
                @click="emit('preview', { kind: 'cover', row: active })"
                >查看实际封面</t-button
              >
            </aside>
          </div>
          <div class="batch-detail-actions">
            <t-button variant="text" @click="openAI(true)"
              ><template #icon><SparklesIcon /></template
              >为当前任务生成建议</t-button
            ><t-button variant="outline" @click="approve(active)"
              ><template #icon><CheckIcon /></template
              >确认当前内容与封面</t-button
            >
          </div>
        </div>
      </article>
      <div v-else class="batch-content-detail batch-empty">
        选择左侧任务，查看完整内容与封面。
      </div>
    </div>
  </section>
  <ResourceOverlay
    v-model:visible="settingsOpen"
    title="批次平台设置"
    :width="560"
    ><div class="batch-ai-dialog">
      <p>
        {{ rows[0]?.platform }} ·
        {{ rows.length }} 条任务。统一设置平台字段，保留单条明确修改。
      </p>
      <label
        ><span>创作声明 <em>*</em></span
        ><t-select
          :popup-props="{ attach: 'body', zIndex: 5500 }"
          v-model="declaration"
          :options="declarationOptions(rows[0]?.platform)"
          placeholder="按本次实际内容确认"
          aria-label="批次创作声明" /></label
      ><label v-if="rows[0]?.platform !== '百家号'"
        >可见范围<t-select
          :popup-props="{ attach: 'body', zIndex: 5500 }"
          v-model="visibility"
          :options="
            ['公开', '仅自己可见'].map((v) => ({ label: v, value: v }))
          "
      /></label>
      <p class="batch-inline-note">
        声明需要按内容事实选择。应用后仍需检查每条任务的文案与实际封面。
      </p>
    </div>
    <template #footer
      ><t-button variant="outline" @click="settingsOpen = false">取消</t-button
      ><t-button :disabled="!declaration" @click="applySettings"
        >应用并保留单条修改</t-button
      ></template
    ></ResourceOverlay
  >
  <ResourceOverlay
    v-model:visible="aiOpen"
    title="批量 AI 生成"
    :width="640"
    :busy="busy"
    ><div class="batch-ai-dialog">
      <p>
        选择任务范围与生成字段。建议先预览再采用，当前内容版本不会被直接覆盖。
      </p>
      <label>生成范围</label
      ><t-radio-group v-model="aiScope" variant="default-filled"
        ><t-radio-button value="筛选结果"
          >筛选结果 ({{ filtered.length }})</t-radio-button
        ><t-radio-button value="勾选任务"
          >勾选任务 ({{ selected.length }})</t-radio-button
        ><t-radio-button value="当前任务"
          >当前任务</t-radio-button
        ></t-radio-group
      ><label>生成字段</label
      ><t-checkbox-group v-model="aiFields" class="batch-ai-fields"
        ><t-checkbox v-for="f in fieldKeys" :key="f" :value="f">{{
          labels[f]
        }}</t-checkbox></t-checkbox-group
      ><label
        >文案风格<t-select
          :popup-props="{ attach: 'body', zIndex: 5500 }"
          v-model="style"
          :options="
            ['实战攻略', '简洁'].map((s) => ({ label: s, value: s }))
          " /></label
      ><t-alert v-if="error" theme="warning" :message="error" />
      <p class="batch-inline-note">
        本原型使用明确标识的本地演示建议，不调用 AI
        服务。已采用内容与封面保留；生成后发生修改的字段需重新生成。
      </p>
    </div>
    <template #footer
      ><t-button variant="outline" @click="aiOpen = false">取消</t-button
      ><t-button
        :loading="busy"
        :disabled="!scopeRows().length || !aiFields.length"
        @click="generate"
        >生成 {{ scopeRows().length }} 条建议</t-button
      ></template
    ></ResourceOverlay
  >
  <ResourceOverlay
    :visible="editOpen"
    @update:visible="
      (v) => {
        if (!v) closeEdit();
      }
    "
    title="编辑发布内容"
    :width="960"
    :busy="saving"
    ><template v-if="editing"
      ><div class="batch-editor-context">
        <img :src="asset(editing.assetId)?.cover" alt="来源成片封面" />
        <div>
          <strong>{{ asset(editing.assetId)?.title }}</strong>
          <p>
            {{ editing.category }} · {{ account(editing.accountId)?.name }} ·
            V-{{ editing.assetId }}
          </p>
        </div>
        <t-tag variant="light">资源只读</t-tag>
      </div>
      <t-alert
        v-if="editError"
        theme="warning"
        :message="editError"
        class="batch-message"
      />
      <div class="batch-editor-grid">
        <div class="batch-editor-fields">
          <h3 class="batch-edit-section-heading"><span>01</span>发布文案</h3>
          <div class="batch-editor-copy">
            <label
              v-for="field in [
                'title',
                'description',
                'shortTitle',
                'subTitle',
                'topics',
                'tags',
              ]"
              :key="field"
              :class="{
                'is-wide': ['title', 'description', 'topics', 'tags'].includes(
                  field,
                ),
              }"
              >{{ labels[field]
              }}<t-textarea
                v-if="field === 'description'"
                :value="editing[field] || ''"
                :autosize="{ minRows: 3, maxRows: 6 }"
                @change="(v) => editField(editing, field, v)"
                :aria-label="labels[field]" /><t-input
                v-else
                :value="editing[field] || ''"
                @change="(v) => editField(editing, field, v)"
                :aria-label="labels[field]"
            /></label>
          </div>
        </div>
        <aside class="batch-editor-cover">
          <h3 class="batch-edit-section-heading"><span>02</span>发布封面</h3>
          <label>实际输出预览</label
          ><button
            class="batch-editor-cover-image"
            @click="emit('preview', { kind: 'cover', row: editing })"
          >
            <img :src="editing.cover" alt="编辑中的封面" /></button
          ><label
            >封面方式<t-select
              :popup-props="{ attach: 'body', zIndex: 5500 }"
              :value="editing.coverMode"
              :options="
                ['现成封面', '模板合成'].map((v) => ({ label: v, value: v }))
              "
              @change="setCoverMode" /></label
          ><label
            >{{ editing.coverMode === "模板合成" ? "封面底图" : "封面图片"
            }}<t-select
              :popup-props="{ attach: 'body', zIndex: 5500 }"
              :value="editing.imageId || 0"
              :options="[
                { label: '成片自带封面', value: 0 },
                ...usableImages.map((i) => ({ label: i.name, value: i.id })),
              ]"
              @change="setImage" /></label
          ><label v-if="editing.coverMode === '模板合成'"
            >封面模板<t-select
              :popup-props="{ attach: 'body', zIndex: 5500 }"
              :value="editing.templateId"
              :options="
                usableTemplates.map((t) => ({ label: t.name, value: t.id }))
              "
              placeholder="请选择模板"
              @change="setTemplate" /></label
          ><t-button variant="outline" :loading="saving" @click="renderEdit"
            >生成并预览新封面</t-button
          ><t-button
            variant="text"
            @click="emit('preview', { kind: 'cover', row: editing })"
            >放大检查实际输出</t-button
          ><small>{{
            editing.coverSignature === coverSignature(editing)
              ? "当前封面已生成"
              : "配置已变化，请生成当前版本"
          }}</small>
          <p>
            {{
              editing.coverMode === "现成封面"
                ? "现成图片保留原有文字；文案不会自动写入图片。"
                : "模板合成优先使用纯净底图，避免与原图文字重叠；按模板文字槽位与平台比例生成。"
            }}
          </p>
        </aside>
        <div class="batch-editor-platform">
          <h3 class="batch-edit-section-heading"><span>03</span>平台设置</h3>
          <label
            ><span>创作声明 <em>*</em></span
            ><t-select
              :popup-props="{ attach: 'body', zIndex: 5500 }"
              :value="editing.declaration"
              :options="declarationOptions(editing.platform)"
              placeholder="按内容事实选择"
              @change="(v) => editField(editing, 'declaration', v)"
              aria-label="创作声明" /></label
          ><label v-if="editing.platform === 'B站'"
            >分区<t-select
              :popup-props="{ attach: 'body', zIndex: 5500 }"
              :value="editing.partition"
              :options="[{ label: '游戏', value: '游戏' }]"
              @change="(v) => editField(editing, 'partition', v)" /></label
          ><label
            >发布时间（可选）<input
              type="datetime-local"
              :value="editing.schedule"
              @change="editField(editing, 'schedule', $event.target.value)"
          /></label>
        </div>
      </div>
      <p class="batch-inline-note">
        保存即确认采用当前文案与实际封面。未修改字段保留；未保存输入可取消放弃。
      </p></template
    ><template #footer
      ><t-button variant="outline" @click="closeEdit">取消</t-button
      ><t-button :loading="saving" @click="saveEdit"
        >保存并采用此版本</t-button
      ></template
    ></ResourceOverlay
  >
</template>

<style scoped>
/* Same 36px controls and 14/13/12 typography as quick publish and production. */
.batch-content-workspace,
.batch-editor-grid,
.batch-ai-dialog {
  --batch-line: var(--wt-color-border);
  --batch-muted: var(--wt-color-text-secondary);
  --batch-blue: var(--wt-color-primary);
  color: var(--wt-color-text-primary);
  font-size: 13px;
}
.batch-content-workspace {
  margin-top: 24px;
}
.batch-content-heading {
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 20px;
  flex-wrap: wrap;
}
.batch-content-heading h3 {
  font-size: 14px;
  font-weight: 600;
  line-height: 24px;
  margin: 0;
}
.batch-content-heading p {
  margin: 4px 0 0;
  color: var(--wt-color-text-secondary);
}
.batch-content-heading > div:last-child {
  flex-wrap: wrap;
  justify-content: flex-start;
  gap: 8px;
}
.batch-content-layout {
  grid-template-columns: minmax(300px, 0.85fr) minmax(0, 1.65fr);
  gap: 20px;
}
.batch-task-list,
.batch-content-detail {
  border-radius: 6px;
  border-color: var(--wt-color-border);
  background: var(--wt-color-bg-container);
}
.batch-list-head {
  height: 52px;
  padding: 0 16px;
}
.batch-list-head strong {
  font-weight: 500;
  color: var(--wt-color-text-primary);
}
.batch-list-toolbar {
  padding: 12px;
  gap: 8px;
  flex-wrap: wrap;
  background: var(--wt-color-bg-subtle);
}
.batch-list-toolbar :deep(.t-checkbox__label) {
  color: var(--wt-color-text-secondary);
  font-size: 12px;
}
.batch-list-toolbar :deep(.t-button) {
  padding-inline: 8px;
}
.batch-task-item {
  min-height: 78px;
  padding: 12px;
  border-bottom-color: var(--wt-color-border);
}
.batch-task-item.active {
  background: var(--wt-color-primary-light);
  border-left-color: var(--wt-color-primary);
}
.batch-task-button b {
  font-weight: 500;
  font-size: 13px;
  line-height: 21px;
  color: var(--wt-color-text-primary);
}
.batch-task-button small {
  font-size: 12px;
  line-height: 20px;
  color: var(--wt-color-text-secondary);
}
.batch-task-button > img {
  width: 38px;
  height: 50px;
  border-radius: 4px;
}
.batch-detail-head {
  padding: 16px 20px;
  background: var(--wt-color-bg-container);
  border-bottom: 1px solid var(--wt-color-border);
}
.batch-detail-head h3 {
  margin: 0;
  font-size: 16px;
  line-height: 26px;
  font-weight: 500;
  color: var(--wt-color-text-primary);
}
.batch-detail-head p {
  font-size: 12px;
  line-height: 20px;
  margin: 4px 0 0;
  color: var(--wt-color-text-secondary);
}
.batch-resource-context {
  flex-wrap: wrap;
  color: var(--wt-color-text-secondary);
  border-color: var(--wt-color-border);
}
.batch-resource-context b {
  color: var(--wt-color-text-primary);
}
.batch-resource-context > small {
  font-size: 12px;
  background: var(--wt-color-bg-subtle);
  color: var(--wt-color-text-secondary);
}
.batch-reading-grid {
  grid-template-columns: minmax(0, 1fr) 190px;
  gap: 24px;
}
.batch-read-field {
  margin-bottom: 20px;
}
.batch-read-field > label {
  color: var(--wt-color-text-secondary);
  font-size: 13px;
  font-weight: 400;
  line-height: 20px;
}
.batch-read-field > p {
  margin: 8px 0 0;
  font-size: 13px;
  line-height: 1.7;
  color: var(--wt-color-text-primary);
}
.batch-read-field > p.is-title {
  font-size: 16px;
  font-weight: 500;
  line-height: 26px;
  color: var(--wt-color-text-primary);
}
.batch-read-field > p.is-empty {
  color: var(--wt-color-text-assist);
  font-size: 13px;
}
.batch-current-cover {
  padding: 16px;
  background: var(--wt-color-bg-subtle);
  border-radius: 6px;
  gap: 12px;
}
.batch-current-cover > label,
.batch-current-cover > small,
.batch-current-cover > p {
  color: var(--wt-color-text-secondary);
  font-size: 12px;
  line-height: 20px;
}
.batch-detail-actions {
  flex-wrap: wrap;
  gap: 8px;
  padding-top: 16px;
  border-top: 1px solid var(--wt-color-border);
}
.batch-editor-context {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px;
  margin: 0 0 24px;
  background: var(--wt-color-bg-subtle);
  border: 1px solid var(--wt-color-border);
  border-radius: 6px;
  font-size: 12px;
  color: var(--wt-color-text-secondary);
}
.batch-editor-context > img {
  width: 38px;
  height: 50px;
  object-fit: cover;
  border-radius: 4px;
  flex: none;
}
.batch-editor-context > div:not(.t-tag) {
  min-width: 0;
  flex: 1;
}
.batch-editor-context strong {
  display: block;
  font-size: 14px;
  font-weight: 500;
  line-height: 22px;
  color: var(--wt-color-text-primary);
  overflow-wrap: anywhere;
}
.batch-editor-context p {
  margin: 4px 0 0;
  line-height: 20px;
}
.batch-editor-grid {
  grid-template-columns: minmax(0, 1fr) 260px;
  grid-template-areas: "copy cover" "platform cover";
  gap: 28px;
}
.batch-editor-fields {
  grid-area: copy;
  display: block;
}
.batch-edit-section-heading {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 0 0 20px;
  font-size: 14px;
  font-weight: 600;
  line-height: 24px;
  color: var(--wt-color-text-primary);
}
.batch-edit-section-heading > span {
  display: grid;
  place-items: center;
  width: 24px;
  height: 24px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
  color: var(--wt-color-primary);
  background: var(--wt-color-primary-light);
}
.batch-editor-copy,
.batch-editor-platform {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20px 24px;
}
.batch-editor-copy .is-wide,
.batch-editor-platform h3 {
  grid-column: 1/-1;
}
.batch-editor-platform {
  grid-area: platform;
  margin-top: 0;
  padding-top: 24px;
  border-color: var(--wt-color-border);
}
.batch-editor-platform h3 {
  margin: 0;
}
.batch-editor-grid label,
.batch-ai-dialog > label {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
  font-size: 13px;
  font-weight: 500;
  line-height: 20px;
  color: var(--wt-color-text-primary);
}
.batch-editor-grid :deep(.t-input),
.batch-ai-dialog :deep(.t-input) {
  min-height: 36px;
}
.batch-editor-platform input[type="datetime-local"] {
  color: var(--wt-color-text-primary);
  border-color: var(--wt-color-border-control);
  background: var(--wt-color-bg-container);
  font-size: 14px;
}
.batch-editor-platform input[type="datetime-local"]:focus-visible {
  outline: 2px solid var(--wt-color-primary);
  outline-offset: 2px;
}
.batch-editor-cover {
  grid-area: cover;
  max-width: none;
  gap: 16px;
  padding-left: 24px;
  border-left: 1px solid var(--wt-color-border);
}
.batch-editor-cover .batch-edit-section-heading {
  margin: 0 0 4px;
}
.batch-editor-cover-image {
  border: 1px solid var(--wt-color-border);
  border-radius: 6px;
  background: var(--wt-color-bg-subtle);
}
.batch-editor-cover-image img {
  max-height: 260px;
  border-radius: 6px;
}
.batch-editor-cover > small,
.batch-editor-cover > p,
.batch-inline-note {
  font-size: 12px;
  line-height: 20px;
  color: var(--wt-color-text-secondary);
}
@media (max-width: 1100px) {
  .batch-content-layout {
    grid-template-columns: 300px minmax(0, 1fr);
  }
  .batch-reading-grid {
    grid-template-columns: minmax(0, 1fr);
  }
  .batch-current-cover {
    max-width: 260px;
  }
}
@media (max-width: 800px) {
  .batch-content-layout {
    grid-template-columns: minmax(0, 1fr);
  }
  .batch-task-scroll {
    max-height: 330px;
  }
  .batch-editor-grid {
    grid-template-areas: "copy" "cover" "platform";
    grid-template-columns: minmax(0, 1fr);
  }
  .batch-editor-cover {
    padding: 24px 0 0;
    border-left: 0;
    border-top: 1px solid var(--wt-color-border);
    max-width: none;
  }
  .batch-editor-cover-image {
    max-width: 320px;
    align-self: start;
  }
  .batch-editor-cover-image img {
    max-height: 240px;
  }
}
@media (max-width: 540px) {
  .batch-editor-copy,
  .batch-editor-platform {
    grid-template-columns: minmax(0, 1fr);
  }
  .batch-editor-context {
    flex-wrap: wrap;
  }
  .batch-content-heading > div:last-child > span:last-child {
    display: none;
  }
}
</style>
