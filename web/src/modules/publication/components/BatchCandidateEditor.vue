<script setup>
import { computed, ref, watch } from "vue";
import ResourceOverlay from "../../../shared/ui/resource/ResourceOverlay.vue";
import ResourceSection from "../../../shared/ui/resource/ResourceSection.vue";
import FeedbackAlert from "../../../shared/ui/feedback/FeedbackAlert.vue";
import PublicationContentGenerator from "./PublicationContentGenerator.vue";
import PublishingCoverEditor from "./PublishingCoverEditor.vue";
import PublishingResourcePicker from "./PublishingResourcePicker.vue";
import { createFilesClient } from "../../../shared/api/files.js";
import { coverConfigFromReference, coverMatchesRatio, coverReference, coverRenderInput, coverResultMatches, emptyCoverConfig } from "../coverPreparation.js";
import { coverProfiles } from "../batchPreparation.js";
import { contentIssue, publicationTextLength } from "../eligibility.js";

const props = defineProps({
  row: { type: Object, required: true },
  output: Object,
  outputName: String,
  accountName: String,
  platform: String,
  recipes: { type: Object, required: true },
  covers: { type: Object, required: true },
  disabled: Boolean,
});
const emit = defineEmits(["close", "edit", "recipe", "cover"]);
const files = createFilesClient();
const selection = ref(null);
const error = ref("");
const staged = ref({});
const values = computed(() => ({ ...props.row.content, cover_title: props.row.cover_title || "" }));
const check = computed(() => contentIssue({ ...props.row, content: {
  ...props.row.content,
  description: props.row.content.description || "",
  cover_file_id: props.row.content.cover_file_id === "0" ? "" : props.row.content.cover_file_id,
  portrait_cover_file_id: props.row.content.portrait_cover_file_id === "0" ? "" : props.row.content.portrait_cover_file_id,
} }));
const generationContext = computed(() => ({
  rowKey: props.row.row_key, platform: props.platform, categoryId: props.row.category_id,
  outputId: props.row.composite_output_id, identityKey: props.row.input_revision,
}));

function url(file) {
  try { return files.resourceURL(file); } catch { return undefined; }
}
const configs = computed(() => Object.fromEntries(coverProfiles.map(profile => {
  const saved = props.row.content[profile.render];
  const recipe = props.recipes[profile.key] || (saved ? coverConfigFromReference(saved) : emptyCoverConfig());
  return [profile.key, { ...recipe, title: props.row.cover_title || "" }];
})));
function renderBasis(profile) {
  return JSON.stringify({
    rowKey: props.row.row_key,
    inputRevision: props.row.input_revision,
    outputID: props.row.composite_output_id,
    titleRevision: props.row.field_revisions.cover_title,
    fileRevision: props.row.field_revisions[profile.file],
    renderRevision: props.row.field_revisions[profile.render],
    config: configs.value[profile.key],
  });
}
const stagedArtworks = computed(() => Object.fromEntries(coverProfiles.map(profile => {
  const entry = staged.value[profile.key];
  return [profile.key, entry?.basis === renderBasis(profile) ? entry.result : null];
})));
watch(() => coverProfiles.map(renderBasis), (bases) => {
  coverProfiles.forEach((profile, index) => {
    const entry = staged.value[profile.key];
    if (entry && entry.basis !== bases[index]) delete staged.value[profile.key];
  });
});

function setRecipe(profile, config) {
  if (config.title !== (props.row.cover_title || "")) emit("edit", { cover_title: config.title });
  emit("recipe", profile.key, config);
}
function useOutput(profile) {
  const thumbnail = props.output?.thumbnail;
  if (thumbnail?.file_id && coverMatchesRatio(thumbnail, ...profile.aspect)) {
    emit("cover", profile, thumbnail, undefined);
  }
}
function selectedImage(file) {
  if (!selection.value || !coverMatchesRatio(file, ...selection.value.aspect)) return;
  emit("cover", selection.value, { ...file, file_id: file.id }, undefined);
  selection.value = null;
}
function matchesCurrentRender(profile, result) {
  const config = configs.value[profile.key];
  if (config.method !== "template" || !result || !coverMatchesRatio(result.file, ...profile.aspect)) return false;
  try {
    return coverResultMatches(result, coverRenderInput(config, props.output, result.row_key, result.input_revision));
  } catch {
    return false;
  }
}
function stageArtwork(profile, result) {
  if (!matchesCurrentRender(profile, result)) {
    error.value = "封面产物与当前规格不符，请重新生成";
    return;
  }
  error.value = "";
  staged.value[profile.key] = { result, basis: renderBasis(profile) };
}
function applyArtwork(profile) {
  const result = stagedArtworks.value[profile.key];
  if (props.disabled || !matchesCurrentRender(profile, result)) return;
  emit("cover", profile, result.file, coverReference(result));
  delete staged.value[profile.key];
}
</script>

<template>
  <ResourceOverlay form-actions :visible="true" :width="960" title="编辑发布内容"
    @update:visible="!$event && emit('close')">
    <div class="batch-stack">
      <div class="batch-content-source"><div><small>发布账号</small><strong>{{accountName}}</strong><span>百家号</span></div><div><small>发布成片</small><strong :title="outputName">{{outputName}}</strong><span>成片 ID：{{row.composite_output_id}}</span></div></div>
      <ResourceSection title="文案与创作声明">
        <div class="batch-form-grid">
          <label class="batch-form-wide">发布文案
            <t-textarea :value="row.content.description || ''" :maxlength="50" aria-label="发布文案" :disabled="disabled"
              @change="emit('edit', { description: $event })" />
          </label>
          <label>话题<t-input :value="row.content.topics || ''" :maxlength="50" aria-label="话题" :disabled="disabled"
            @change="emit('edit', { topics: $event })" /></label>
          <label>封面短标题<t-input :value="row.cover_title || ''" :maxlength="100" aria-label="封面短标题" :disabled="disabled"
            @change="emit('edit', { cover_title: $event })" /></label>
          <small class="batch-form-wide" :class="publicationTextLength(row.content.description,row.content.topics)>50 ? 'batch-text-error' : 'batch-muted'" role="status">文案和话题合计 {{ publicationTextLength(row.content.description, row.content.topics) }}/50 字（含分隔空格） · {{publicationTextLength(row.content.description,row.content.topics)>50 ? '已超出，请缩短后确认' : '剩余 '+(50-publicationTextLength(row.content.description,row.content.topics))+' 字'}}</small>
          <label>创作声明
            <t-select :value="row.content.declaration || ''" :options="[{ label: '无需声明', value: '无需声明' }]"
              clearable placeholder="按内容事实选择" aria-label="创作声明" :disabled="disabled"
              @change="emit('edit', { declaration: $event || '' })" />
          </label>
        </div>
        <PublicationContentGenerator :context="generationContext" :values="values" template :disabled="disabled"
          @adopt="(field, value) => emit('edit', { [field]: value })" />
      </ResourceSection>
      <ResourceSection title="封面">
        <FeedbackAlert v-if="error" theme="error" :message="error" />
        <div v-for="profile in coverProfiles" :key="profile.key" class="batch-cover-row">
          <h4>{{ profile.label }}</h4>
          <div class="batch-cover-panes">
            <div class="batch-cover-figure">
              <img v-if="url(covers[profile.key])" :src="url(covers[profile.key])" :alt="`已采用${profile.label}`" />
              <p v-else class="batch-muted">{{ row.content[profile.file] && row.content[profile.file] !== "0" ? "已选择封面，预览暂不可用" : "未选择封面" }}</p>
              <template v-if="stagedArtworks[profile.key]">
                <img v-if="url(stagedArtworks[profile.key].file)" :src="url(stagedArtworks[profile.key].file)" :alt="`生成的${profile.label}`" />
                <t-button theme="primary" variant="outline" :disabled="disabled" @click="applyArtwork(profile)">采用生成封面</t-button>
              </template>
            </div>
            <PublishingCoverEditor :config="configs[profile.key]" :aspect="profile.aspect" :output="output"
              :category="row.category_id" :disabled="disabled" :group-name="row.row_key"
              @update:config="setRecipe(profile, $event)" @rendered="stageArtwork(profile, $event)"
              @use-output="useOutput(profile)" @choose-image="selection = profile" />
          </div>
        </div>
      </ResourceSection>
      <t-checkbox :checked="!!row.content.confirmed" :disabled="disabled || (!row.content.confirmed && check?.field !== 'confirmation')" @change="emit('edit', { confirmed: $event })">
        已核对当前文案、封面和声明，确认内容可用
      </t-checkbox>
      <small v-if="check && check.field !== 'confirmation'" class="batch-muted">{{ check.message }}</small>
    </div>
    <template #footer><t-button theme="primary" @click="emit('close')">完成</t-button></template>
  </ResourceOverlay>
  <PublishingResourcePicker v-if="selection" visible kind="image" :category="row.category_id" :aspect="selection.aspect"
    @selected="selectedImage" @update:visible="selection = null" />
</template>
