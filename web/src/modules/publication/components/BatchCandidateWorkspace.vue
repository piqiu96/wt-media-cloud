<script setup>
import { computed, ref } from "vue";
import ResourceCard from "../../../shared/ui/resource/ResourceCard.vue";
import ResourceSection from "../../../shared/ui/resource/ResourceSection.vue";
import ResourceOverlay from "../../../shared/ui/resource/ResourceOverlay.vue";
import ResourceRowActions from "../../../shared/ui/resource/ResourceRowActions.vue";
import ResourceSelectionToolbar from "../../../shared/ui/resource/ResourceSelectionToolbar.vue";
import ResourceStatusBadge from "../../../shared/ui/resource/ResourceStatusBadge.vue";
import ResourceNameLink from "../../../shared/ui/resource/ResourceNameLink.vue";
import FeedbackAlert from "../../../shared/ui/feedback/FeedbackAlert.vue";
import PublishingResourcePicker from "./PublishingResourcePicker.vue";
import BatchCandidateEditor from "./BatchCandidateEditor.vue";
import BatchSelectedPreparation from "./BatchSelectedPreparation.vue";
import { createPublicationBatchClient } from "../../../shared/api/publication.js";
import { createComposeClient } from "../../../shared/api/compose.js";
import { publicObjectReadUrl } from "../../../shared/utils/objectReadUrl.js";
import { createFilesClient } from "../../../shared/api/files.js";
import { feedback } from "../../../shared/ui/feedback/index.js";
import { batchAccountName, batchOutputID, batchOutputName } from "../useBatchResourceCatalog.js";
import { batchFormProblem, batchInput, coverProfiles, editCandidate, moveEntry, newCandidate, pairOf, replaceCandidatePair, reviewSelectedConfirmations } from "../batchPreparation.js";
import { coverConfigFromReference, emptyCoverConfig } from "../coverPreparation.js";
import { contentIssue } from "../eligibility.js";

const props = defineProps({ form: Object, catalog: Object, categories: Array, actor: Object, recipes: Object, coverFiles: Object, focusKey: String, validationContext: Object, disabled: Boolean });
const emit = defineEmits(["work"]);
const compose = createComposeClient();
const batch = createPublicationBatchClient();
const files = createFilesClient();
const selection = ref([]);
const activeKey = ref(props.focusKey || "");
const pairing = ref(null);
const picker = ref("");
const bulk = ref(false);
const preview = ref(null);
const pairError = ref("");
const confirmationFeedback = ref(null);
const confirmationRows = ref({});
let previewSequence = 0;
const rows = computed(() => props.form.config.candidates);
const active = computed(() => rows.value.find(row => row.row_key === activeKey.value));
const selected = computed(() => rows.value.filter(row => selection.value.includes(row.row_key)));
const pendingConfirmations = computed(() => selected.value.filter(row => !row.content.confirmed));
const confirmationProblem = computed(() => batchFormProblem(props.form, { requireName: false }));
const outputFor = row => props.catalog.outputs.get(row?.composite_output_id);
const accountName = row => batchAccountName(props.catalog.accounts.get(row?.media_account_id));
const categoryName = id => props.categories.find(item => item.value === id)?.label || "原类目暂不可用";
const outputName = row => batchOutputName(outputFor(row));
const rowLabel = row => `${outputName(row)} · ${accountName(row)}`;
const columns = [
  { colKey: "selected", type: "multiple", width: 52, fixed: "left" },
  { colKey: "video", title: "候选成片", width: 260 },
  { colKey: "account", title: "发布账号", width: 150 },
  { colKey: "category", title: "类目", width: 120 },
  { colKey: "status", title: "内容状态", width: 210 },
  { colKey: "op", title: "操作", width: 310, fixed: "right" },
];
function isManual(row) {
  return props.form.config.manual_pairs.some(pair => pair.composite_output_id === row.composite_output_id);
}
function statusIssue(row) {
  const serverIssue = confirmationRows.value[row.row_key];
  if (serverIssue?.version === JSON.stringify(row)) return serverIssue;
  return contentIssue({ ...row, content: { ...row.content, description: row.content.description || "",
    ...(row.content.cover_file_id === "0" && { cover_file_id: "" }),
    ...(row.content.portrait_cover_file_id === "0" && { portrait_cover_file_id: "" }),
  } });
}
function thumb(row) {
  try { return files.resourceURL(outputFor(row)?.thumbnail); } catch { return undefined; }
}

async function showVideo(row) {
  const run = ++previewSequence;
  preview.value = { label: outputName(row), url: "", loading: true, error: "" };
  try {
    const data = await compose.getComposeOutputVideoUrl(row.composite_output_id);
    if (run === previewSequence) preview.value.url = publicObjectReadUrl(data.url);
  } catch (error) {
    if (run === previewSequence) preview.value.error = error.message || "成片暂时无法预览";
  } finally {
    if (run === previewSequence) preview.value.loading = false;
  }
}
function closePreview() {
  previewSequence++;
  preview.value = null;
}
function openPair(row = null) {
  pairError.value = "";
  pairing.value = row ? { row_key: row.row_key, ...pairOf(row) } : {
    row_key: "", category_id: props.form.targets.length === 1 ? props.form.targets[0].category_id : "",
    media_account_id: "", composite_output_id: "",
  };
}
function choosePairResource(row) {
  if (!pairing.value) return;
  props.catalog.remember(picker.value, row);
  if (picker.value === "account") pairing.value.media_account_id = String(row.id);
  else pairing.value.composite_output_id = batchOutputID(row);
}
function selectPairCategory(category) {
  if (category === pairing.value.category_id) return;
  pairing.value.category_id = category;
  pairing.value.media_account_id = "";
  pairing.value.composite_output_id = "";
}
const blockedOutputIds = computed(() => rows.value.filter(row => row.row_key !== pairing.value?.row_key).map(row => row.composite_output_id));

function acceptPair() {
  if (!pairing.value?.category_id || !pairing.value.media_account_id || !pairing.value.composite_output_id) return;
  if (blockedOutputIds.value.includes(pairing.value.composite_output_id)) {
    pairError.value = "该成片已在此批次候选中";
    return;
  }
  const config = props.form.config;
  const index = rows.value.findIndex(row => row.row_key === pairing.value.row_key);
  const current = rows.value[index];
  const pair = pairOf(pairing.value);
  const carried = rows.value.filter(row => row.row_key !== current?.row_key && row.media_account_id === pair.media_account_id).length;
  const account = config.accounts.find(row => row.media_account_id === pair.media_account_id);
  if (account && carried >= account.limit && current?.media_account_id !== pair.media_account_id) {
    pairError.value = "该账号已达到本批上限，请到资源配置调整上限";
    return;
  }
  // When a copied batch has no allocation inventory, seed it from all retained rows.
  for (const row of rows.value) {
    if (!config.accounts.some(item => item.media_account_id === row.media_account_id)) {
      const count = rows.value.filter(item => item.media_account_id === row.media_account_id).length;
      config.accounts.push({ media_account_id: row.media_account_id, limit: count });
    }
    if (!config.output_ids.includes(row.composite_output_id)) config.output_ids.push(row.composite_output_id);
  }
  if (!config.accounts.some(row => row.media_account_id === pair.media_account_id))
    config.accounts.push({ media_account_id: pair.media_account_id, limit: 1 });
  if (!config.output_ids.includes(pair.composite_output_id)) config.output_ids.push(pair.composite_output_id);
  config.manual_pairs = config.manual_pairs.filter(row => row.composite_output_id !== current?.composite_output_id);
  config.manual_pairs.push(pair);
  if (!current) rows.value.push(newCandidate(pair));
  else {
    const updated = replaceCandidatePair(current, pair);
    if (current.composite_output_id !== updated.composite_output_id || current.category_id !== updated.category_id) {
      delete props.recipes[current.row_key];
      delete props.coverFiles[current.row_key];
    }
    rows.value.splice(index, 1, updated);
  }
  pairing.value = null;
}

async function remove(row) {
  const accepted = await feedback.confirmWarning({ title: "移除候选", message: `移除「${outputName(row)}」后，发布目标仍保留。`, confirmText: "移除", danger: true });
  if (!accepted || props.disabled) return;
  const index = rows.value.findIndex(item => item.row_key === row.row_key);
  if (index < 0) return;
  rows.value.splice(index, 1);
  props.form.config.manual_pairs = props.form.config.manual_pairs.filter(pair => pair.composite_output_id !== row.composite_output_id);
  selection.value = selection.value.filter(key => key !== row.row_key);
  delete props.recipes[row.row_key];
  delete props.coverFiles[row.row_key];
}
function edit(key, patch) {
  const index = rows.value.findIndex(row => row.row_key === key);
  if (index >= 0) rows.value.splice(index, 1, editCandidate(rows.value[index], patch));
}
function storeRecipe(key, profileKey, config) {
  const profile = coverProfiles.find(item => item.key === profileKey);
  const row = rows.value.find(item => item.row_key === key);
  const old = props.recipes[key]?.[profileKey] || (row?.content[profile.render] ? coverConfigFromReference(row.content[profile.render]) : emptyCoverConfig());
  if (JSON.stringify(config) === JSON.stringify(old)) return;
  props.recipes[key] = { ...props.recipes[key], [profileKey]: config };
  const basis = ({ title, ...settings }) => JSON.stringify(settings);
  if (row?.content[profile.render] && basis(old) !== basis(config))
    edit(key, { [profile.file]: undefined, [profile.render]: undefined });
}
function applyCover(key, profile, file, render) {
  const id = String(file.file_id ?? file.id);
  if (!/^[1-9][0-9]*$/.test(id)) return;
  const identity = render ? { cover_title: render.title } : {};
  edit(key, { ...identity, [profile.file]: id, [profile.render]: render });
  props.coverFiles[key] = { ...props.coverFiles[key], [profile.key]: { fileId: id, resource_path: file.resource_path } };
}
function activeCovers(row) {
  return Object.fromEntries(coverProfiles.map(profile => {
    const selected = props.coverFiles[row.row_key]?.[profile.key];
    return [profile.key, selected?.fileId === row.content[profile.file] ? selected : null];
  }));
}
async function confirmSelected() {
  if (props.disabled || confirmationProblem.value || !pendingConfirmations.value.length) return;
  const selectedRows = pendingConfirmations.value;
  const configuration = batchInput(props.form);
  const version = JSON.stringify(configuration);
  const agreed = await feedback.confirmWarning({
    title: `确认 ${selectedRows.length} 条候选内容`,
    message: "请确认你已核对所选候选的文案、实际封面和创作声明。系统会重新检查各条配置。",
    confirmText: "确认已核对内容",
  });
  if (!agreed || props.disabled || JSON.stringify(batchInput(props.form)) !== version) return;
  emit("work", true);
  confirmationFeedback.value = null;
  try {
    const result = await batch.validate({ ...configuration, ...props.validationContext });
    if (JSON.stringify(batchInput(props.form)) !== version) {
      confirmationFeedback.value = { theme: "warning", message: "候选配置已变化，请重新核对后确认" };
      return;
    }
    const { confirmable, blocked } = reviewSelectedConfirmations(selectedRows, result.issues);
    for (const entry of blocked) {
      const row = selectedRows.find(item => item.row_key === entry.rowKey);
      confirmationRows.value[entry.rowKey] = { message: entry.message, version: JSON.stringify(row) };
    }
    for (const key of confirmable) {
      delete confirmationRows.value[key];
      edit(key, { confirmed: true });
    }
    confirmationFeedback.value = {
      theme: blocked.length ? "warning" : "success",
      message: blocked.length ? `已确认 ${confirmable.length} 条，${blocked.length} 条待完善。请查看对应候选。` : `已确认 ${confirmable.length} 条候选内容`,
    };
  } catch (error) {
    confirmationFeedback.value = { theme: "error", message: error?.details?.issue?.message || error.message || "核对失败，候选内容已保留" };
  } finally {
    emit("work", false);
  }
}
function applyBulk(key, patch, covers) {
  edit(key, patch);
  for (const { profile, file, render } of covers) applyCover(key, profile, file, render);
}
</script>

<template>
  <ResourceCard>
    <ResourceSection title="候选内容">
      <div class="batch-inline batch-section-actions">
        <t-button theme="default" variant="outline" :disabled="disabled || !form.targets.length" @click="openPair()">添加候选</t-button>
        <span class="batch-muted">{{ rows.length }} 条候选 · 需要核对文案、封面和声明</span>
      </div>
      <ResourceSelectionToolbar :count="selected.length" unit="条候选"
        :hint="pendingConfirmations.length && confirmationProblem ? '请先在结果总览填写执行设置，再检查并确认所选内容。' : ''"
        @clear="selection = []">
        <t-button size="small" :disabled="disabled" @click="bulk = true">批量配置</t-button>
        <t-button size="small" variant="outline" theme="default"
          :disabled="disabled || !pendingConfirmations.length || !!confirmationProblem" @click="confirmSelected">确认内容</t-button>
      </ResourceSelectionToolbar>
      <FeedbackAlert v-if="confirmationFeedback" :theme="confirmationFeedback.theme" :message="confirmationFeedback.message" />
      <div class="table-scroll-wrap resource-data-table">
        <t-table class="wt-resource-table" :data="rows" :columns="columns" row-key="row_key" :selected-row-keys="selection"
          :scroll="{ x: '1102px' }" empty="暂无候选，可添加候选或返回资源配置配对"
          @select-change="selection = $event">
          <template #video="{ row }">
            <div class="batch-video-cell">
              <img v-if="thumb(row)" :src="thumb(row)" alt="" />
              <div>
                <ResourceNameLink v-if="outputFor(row)" @click="showVideo(row)">{{ outputName(row) }}</ResourceNameLink>
                <span v-else>{{ outputName(row) }}</span>
                <small v-if="isManual(row)" class="batch-muted">人工配对</small>
              </div>
            </div>
          </template>
          <template #account="{ row }">{{ accountName(row) }}</template>
          <template #category="{ row }">{{ categoryName(row.category_id) }}</template>
          <template #status="{ row }">
            <ResourceStatusBadge :tone="statusIssue(row) ? 'warning' : 'success'"
              :label="statusIssue(row)?.field === 'confirmation' ? '待确认' : statusIssue(row) ? '待完善' : '已确认'" />
            <small v-if="statusIssue(row)" class="batch-muted">{{ statusIssue(row).message }}</small>
          </template>
          <template #op="{ row, rowIndex }"><ResourceRowActions>
            <t-button size="small" variant="text" :disabled="disabled" @click="activeKey = row.row_key">编辑内容</t-button>
            <t-button size="small" variant="text" :disabled="disabled" @click="openPair(row)">调整配对</t-button>
            <t-button size="small" variant="text" :disabled="disabled || rowIndex === 0" @click="moveEntry(rows, rowIndex, -1)">上移</t-button>
            <t-button size="small" variant="text" :disabled="disabled || rowIndex === rows.length - 1" @click="moveEntry(rows, rowIndex, 1)">下移</t-button>
            <t-button size="small" variant="text" theme="danger" :disabled="disabled" @click="remove(row)">移除</t-button>
          </ResourceRowActions></template>
        </t-table>
      </div>
    </ResourceSection>
  </ResourceCard>

  <BatchCandidateEditor v-if="active" :key="`${active.row_key}:${active.input_revision}`" :row="active"
    :output="outputFor(active) && { ...outputFor(active), id: active.composite_output_id }" :platform="form.platform"
    :output-name="outputName(active)" :account-name="accountName(active)" :recipes="recipes[active.row_key] || {}"
    :covers="activeCovers(active)" :disabled="disabled"
    @edit="edit(active.row_key, $event)" @recipe="(key, config) => storeRecipe(active.row_key, key, config)"
    @cover="(profile, file, render) => applyCover(active.row_key, profile, file, render)"
    @close="activeKey = ''" />
  <BatchSelectedPreparation v-if="bulk" :rows="selected" :platform="form.platform" :output-for="outputFor" :label-for="rowLabel"
    @apply="applyBulk" @work="emit('work', $event)" @close="bulk = false" />

  <ResourceOverlay v-if="pairing" form-actions :visible="true" :title="pairing.row_key ? '调整候选配对' : '添加候选'"
    :width="640" @update:visible="!$event && (pairing = null)">
    <div class="batch-stack">
      <FeedbackAlert v-if="pairError" theme="error" :message="pairError" />
      <label>类目<t-select :value="pairing.category_id" aria-label="候选类目" :options="categories.filter(item => form.targets.some(target => target.category_id === item.value))"
        placeholder="选择目标类目" @change="selectPairCategory" /></label>
      <div class="batch-form-grid">
        <div class="batch-stack">
          <div>发布账号 · {{ pairing.media_account_id ? batchAccountName(catalog.accounts.get(pairing.media_account_id)) : '未选择' }}</div>
          <t-button variant="outline" theme="default" :disabled="!pairing.category_id" @click="picker = 'account'">选择发布账号</t-button>
        </div>
        <div class="batch-stack">
          <div>成片 · {{ pairing.composite_output_id ? batchOutputName(catalog.outputs.get(pairing.composite_output_id)) : '未选择' }}</div>
          <t-button variant="outline" theme="default" :disabled="!pairing.category_id" @click="picker = 'output'">选择成片</t-button>
        </div>
      </div>
    </div>
    <template #footer><t-button theme="primary" :disabled="!pairing.category_id || !pairing.media_account_id || !pairing.composite_output_id" @click="acceptPair">应用配对</t-button></template>
  </ResourceOverlay>
  <PublishingResourcePicker v-if="picker && pairing" batch visible :kind="picker" :platform="form.platform"
    :category="pairing.category_id" :team-id="String(actor.team_id || '')" :excluded-ids="picker === 'output' ? blockedOutputIds : []"
    @selected="choosePairResource" @update:visible="picker = ''" />
  <ResourceOverlay v-if="preview" visible :title="preview.label" :width="880" :footer="false" @update:visible="!$event && closePreview()">
    <p v-if="preview.loading" role="status">正在读取成片</p>
    <FeedbackAlert v-else-if="preview.error" theme="error" :message="preview.error" />
    <video v-else-if="preview.url" :src="preview.url" class="batch-preview-video" controls preload="metadata" @error="preview.error = '成片暂时无法读取'" />
  </ResourceOverlay>
</template>
