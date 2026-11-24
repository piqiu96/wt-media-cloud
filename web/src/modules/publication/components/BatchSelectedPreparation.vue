<script setup>
import { computed, onBeforeUnmount, ref } from "vue";
import ResourceOverlay from "../../../shared/ui/resource/ResourceOverlay.vue";
import ResourceSection from "../../../shared/ui/resource/ResourceSection.vue";
import FeedbackAlert from "../../../shared/ui/feedback/FeedbackAlert.vue";
import PublishingCoverEditor from "./PublishingCoverEditor.vue";
import { createImageClient } from "../../../shared/api/image.js";
import { createFilesClient } from "../../../shared/api/files.js";
import { createPublicationBatchClient } from "../../../shared/api/publication.js";
import { coverMatchesRatio, coverReference, coverRenderInput, coverResultMatches, emptyCoverConfig } from "../coverPreparation.js";
import { coverProfiles, generationFields, generationInput, matchesCandidateField, publicContentPatch } from "../batchPreparation.js";

const props = defineProps({
  rows: { type: Array, required: true },
  platform: String,
  outputFor: Function,
  labelFor: Function,
});
const emit = defineEmits(["close", "apply", "work"]);
const image = createImageClient();
const batch = createPublicationBatchClient();
const files = createFilesClient();
const chosen = ref([]);
const defaults = ref({ description: "", topics: "", cover_title: "" });
const overwrite = ref(false);
const fields = ref(Object.keys(generationFields));
const instruction = ref("");
const coverKeys = ref([]);
const coverTitle = ref("");
const coverConfig = ref(Object.fromEntries(coverProfiles.map(profile => [profile.key, emptyCoverConfig()])));
const rowsReviewed = ref({});
const phase = ref("setup");
const busy = ref(false);
const progress = ref("");
const streamProblem = ref("");
let controller;

const modeOptions = [
  { label: "公共设置", value: "public" },
  { label: "AI 文案", value: "ai" },
  { label: "封面设置", value: "cover" },
];
const fieldOptions = Object.entries(generationFields).map(([value, label]) => ({ value, label }));
const profileOptions = coverProfiles.map(profile => ({ value: profile.key, label: profile.label }));
const ready = computed(() => chosen.value.length && (!chosen.value.includes("ai") || fields.value.length) &&
  (!chosen.value.includes("cover") || coverKeys.value.length));
const review = computed(() => props.rows.map(row => ({ row, info: rowsReviewed.value[row.row_key] })));
const resultCount = computed(() => review.value.filter(({ info }) => info?.result || info?.errors.length).length);
const canApply = computed(() => review.value.some(({ row, info }) => {
  if (!info) return false;
  const { patch, covers } = currentChanges(row, info);
  return Object.keys(patch).length > 0 || covers.length > 0;
}));

function valueFor(row, field) {
  return field === "cover_title" ? row.cover_title : row.content[field];
}
function fieldsAllowed(row, keys) {
  return keys.filter(key => overwrite.value || !valueFor(row, key)?.trim());
}
function fileURL(file) {
  try { return files.resourceURL(file); } catch { return undefined; }
}
function stop() {
  controller?.abort();
}
onBeforeUnmount(stop);

async function stream(input, send, accept) {
  for (let i = 0; i < input.length; i += 50) {
    if (controller.signal.aborted) return;
    await send({ items: input.slice(i, i + 50) }, {
      signal: controller.signal,
      onEvent: ({ event, data }) => {
        if (controller.signal.aborted) return;
        if (event === "item_result" || event === "item_error") accept(event, data);
        progress.value = `已收到 ${resultCount.value} / ${props.rows.length} 条候选的结果`;
      },
    });
  }
}

async function prepare(onlyKeys) {
  if (busy.value || !ready.value) return;
  const working = props.rows.filter(row => !onlyKeys || onlyKeys.includes(row.row_key));
  controller = new AbortController();
  busy.value = true;
  emit("work", true);
  phase.value = "review";
  streamProblem.value = "";
  progress.value = "";
  const ai = [];
  const renders = [];
  const renderRows = [];
  const requestedCovers = coverProfiles.filter(profile => chosen.value.includes("cover") && coverKeys.value.includes(profile.key));
  const imageSelections = new Map();
  try {
    for (const row of working) {
      const prior = rowsReviewed.value[row.row_key];
      const sameInput = prior?.basis.input_revision === row.input_revision;
      const priorValues = sameInput ? Object.fromEntries(Object.entries(prior.values).filter(([field]) => matchesCandidateField(row, prior.basis, field))) : {};
      const priorCovers = sameInput ? Object.fromEntries(Object.entries(prior.covers).filter(([, item]) => currentCover(row, prior, item))) : {};
      const appliedFields = sameInput ? prior.appliedFields : [];
      const appliedCovers = sameInput ? prior.appliedCovers : [];
      const generationKeys = chosen.value.includes("ai")
        ? fieldsAllowed(row, fields.value).filter(field => !(field in priorValues) && !appliedFields.includes(field)) : [];
      const publicPatch = chosen.value.includes("public") ? publicContentPatch(row, defaults.value, overwrite.value) : {};
      const values = { ...Object.fromEntries(Object.entries(publicPatch).filter(([field]) => !appliedFields.includes(field))), ...priorValues };
      const newCovers = requestedCovers.filter(profile => !(profile.key in priorCovers) && !appliedCovers.includes(profile.key));
      const coverFields = requestedCovers.flatMap(profile => [profile.file, profile.render]);
      const coverBasis = requestedCovers.some(profile => coverConfig.value[profile.key].method === "template") ? ["cover_title"] : [];
      const basis = generationInput(row, props.platform, [...new Set([...generationKeys, ...Object.keys(values), ...coverFields, ...coverBasis])], instruction.value);
      rowsReviewed.value[row.row_key] = {
        basis, values, covers: priorCovers, errors: [], appliedFields, appliedCovers,
        result: !!(Object.keys(values).length || Object.keys(priorCovers).length),
        requested: !!(generationKeys.length || Object.keys(values).length || newCovers.length),
      };
      const stored = rowsReviewed.value[row.row_key];
      if (generationKeys.length) ai.push(generationInput(row, props.platform, generationKeys, instruction.value));
      const profiles = [];
      for (const profile of newCovers) {
        if (controller.signal.aborted) break;
        const config = coverConfig.value[profile.key];
        try {
          if (config.method === "template") {
            profiles.push(profile);
          } else {
            const key = `${row.category_id}:${profile.key}`;
            if (!imageSelections.has(key)) {
              imageSelections.set(key, image.selectImageAsset({
                usage: "cover", category_id: row.category_id, aspect: profile.aspect.join(":"),
              }));
            }
            const file = await imageSelections.get(key);
            if (controller.signal.aborted) break;
            if (!file?.file_id || !coverMatchesRatio(file, ...profile.aspect) || !fileURL(file))
              throw new Error("没有符合规格的可用现成封面");
            stored.covers[profile.key] = { file, profile };
            stored.result = true;
          }
        } catch (failure) {
          stored.errors.push(`${profile.label} · ${failure.message}`);
        }
      }
      if (profiles.length) renderRows.push({ row, profiles, coverFields });
    }
    await stream(ai, batch.generate, (event, data) => {
      const info = rowsReviewed.value[data.row_key];
      if (event === "item_error") {
        info.errors.push(data.message || "文案生成失败");
        return;
      }
      const limits = { description: 1000, topics: 200, cover_title: 100 };
      for (const field of Object.keys(data.field_revisions)) {
        const value = data.result?.content?.[field];
        if (typeof value !== "string" || [...value].length > limits[field] ||
          (!value.trim() && field !== "topics") || data.result.field_revisions?.[field] !== data.field_revisions[field]) {
          info.errors.push(`${generationFields[field]}生成结果不完整`);
          continue;
        }
        info.values[field] = value;
        info.result = true;
      }
    });
    for (const { row, profiles, coverFields } of renderRows) {
      const info = rowsReviewed.value[row.row_key];
      const output = props.outputFor(row);
      const specs = [];
      for (const profile of profiles) {
        try {
          const title = coverTitle.value.trim() || info.values.cover_title?.trim() || row.cover_title || "";
          const { row_key, input_revision, ...spec } = coverRenderInput({ ...coverConfig.value[profile.key], title },
            output && { ...output, id: row.composite_output_id }, row.row_key, row.input_revision);
          specs.push({ ...spec, specification: profile.key === "horizontal" ? "landscape" : "portrait" });
        } catch (failure) {
          info.errors.push(`${profile.label} · ${failure.message}`);
        }
      }
      if (specs.length === profiles.length) renders.push({
        row_key: row.row_key, input_revision: row.input_revision, category_id: row.category_id, platform: props.platform,
        field_revisions: Object.fromEntries(coverFields.map(field => [field, info.basis.field_revisions[field]])), specs,
      });
    }
    await stream(renders, image.renderCover, (event, data) => {
      const info = rowsReviewed.value[data.row_key];
      if (event === "item_error") {
        info.errors.push(data.message || "封面生成失败");
        return;
      }
      const request = renders.find(item => item.row_key === data.row_key);
      for (const spec of request.specs) {
        const result = data.result?.artworks?.find(item => item.specification === spec.specification);
        const profile = coverProfiles.find(item => item.key === (spec.specification === "landscape" ? "horizontal" : "portrait"));
        if (!coverResultMatches(result, { ...spec, row_key: request.row_key, input_revision: request.input_revision }) ||
          !coverMatchesRatio(result.file, ...profile.aspect) || !fileURL(result.file)) {
          info.errors.push(`${profile.label}生成结果不匹配`);
          continue;
        }
        info.covers[profile.key] = { file: result.file, render: coverReference(result), profile };
        info.result = true;
      }
    });
  } catch (failure) {
    streamProblem.value = failure.name === "AbortError" ? "已停止，已收到的结果可以继续审阅" : failure.message || "准备中断，请手动重试";
  } finally {
    if (controller.signal.aborted) streamProblem.value = "已停止，已收到的结果可以继续审阅";
    controller = undefined;
    busy.value = false;
    emit("work", false);
  }
}

function currentCover(row, info, { profile, render }) {
  return matchesCandidateField(row, info.basis, profile.file) &&
    matchesCandidateField(row, info.basis, profile.render) &&
    (!render || matchesCandidateField(row, info.basis, "cover_title"));
}

function currentChanges(row, info) {
  return {
    patch: Object.fromEntries(Object.entries(info.values).filter(([field]) => matchesCandidateField(row, info.basis, field))),
    covers: Object.values(info.covers).filter(item => currentCover(row, info, item)),
  };
}

function apply(row, info) {
  const { patch, covers } = currentChanges(row, info);
  if (!Object.keys(patch).length && !covers.length) return;
  emit("apply", row.row_key, patch, covers);
  for (const field of Object.keys(patch)) {
    info.appliedFields.push(field);
    delete info.values[field];
  }
  for (const { profile } of covers) {
    info.appliedCovers.push(profile.key);
    delete info.covers[profile.key];
  }
}
function applyAll() {
  for (const { row, info } of review.value) if (info) apply(row, info);
}
</script>

<template>
  <ResourceOverlay form-actions :visible="true" kind="drawer" :width="960" :busy="busy"
    :title="`批量配置 · 已选 ${rows.length} 条候选`" @update:visible="!$event && emit('close')">
    <div v-if="phase === 'setup'" class="batch-stack">
      <t-checkbox-group v-model="chosen" :options="modeOptions" aria-label="批量配置内容类型" />
      <ResourceSection v-if="chosen.includes('public')" title="公共设置">
        <div class="batch-form-grid">
          <label v-for="option in fieldOptions" :key="option.value">{{ option.label }}
            <t-input v-model="defaults[option.value]" :aria-label="`公共${option.label}`" placeholder="留空则保留原值"
              :maxlength="option.value === 'description' ? 1000 : option.value === 'topics' ? 200 : 100" />
          </label>
        </div>
      </ResourceSection>
      <ResourceSection v-if="chosen.includes('ai')" title="AI 文案">
        <div class="batch-stack">
          <t-checkbox-group v-model="fields" :options="fieldOptions" aria-label="生成字段" />
          <label>文案要求<t-textarea v-model="instruction" aria-label="文案要求" :maxlength="500" placeholder="描述风格或提供需要保留的信息" /></label>
        </div>
      </ResourceSection>
      <t-checkbox v-if="chosen.some(mode => mode !== 'cover')" v-model="overwrite">替换已填写的所选文案</t-checkbox>
      <ResourceSection v-if="chosen.includes('cover')" title="封面设置">
        <t-checkbox-group v-model="coverKeys" :options="profileOptions" aria-label="需要准备的封面规格" />
        <label v-if="coverKeys.some(key => coverConfig[key].method === 'template')" class="batch-result-field">封面标题
          <t-input v-model="coverTitle" aria-label="批量封面标题" placeholder="留空时使用各候选的封面短标题" :maxlength="100" />
        </label>
        <div v-for="profile in coverProfiles.filter(item => coverKeys.includes(item.key))" :key="profile.key" class="batch-cover-row">
          <h4>{{ profile.label }}</h4>
          <PublishingCoverEditor :config="coverConfig[profile.key]" :aspect="profile.aspect" :category="rows[0]?.category_id"
            batch-settings group-name="batch-selected" @update:config="coverConfig[profile.key] = $event" />
          <p v-if="coverConfig[profile.key].method === 'image'" class="batch-muted">从图片库为各类目选择符合规格的现成封面。</p>
        </div>
      </ResourceSection>
    </div>
    <div v-else class="batch-stack">
      <p v-if="busy" role="status">{{ progress || "正在准备所选内容" }}</p>
      <FeedbackAlert v-if="streamProblem" theme="warning" :message="streamProblem" />
      <ResourceSection title="逐条审阅">
        <div v-for="{ row, info } in review" :key="row.row_key" class="batch-review-row">
          <h4>{{ labelFor(row) }}</h4>
          <p v-if="info?.appliedFields.length || info?.appliedCovers.length" class="batch-muted">已应用配置，等待内容确认</p>
          <p v-else-if="!info?.result && !info?.errors.length" class="batch-muted">{{ info?.requested ? '尚未收到结果' : '没有需要更改的字段' }}</p>
          <ul v-if="info?.errors.length" class="batch-issues"><li v-for="(problem, index) in info.errors" :key="index">{{ problem }}</li></ul>
          <template v-if="info">
            <label v-for="(value, field) in info.values" :key="field" class="batch-result-field">
              {{ generationFields[field] }}
              <t-input v-model="info.values[field]" :disabled="busy || !matchesCandidateField(row, info.basis, field)"
                :maxlength="field === 'description' ? 1000 : field === 'topics' ? 200 : 100" />
              <small v-if="!matchesCandidateField(row, info.basis, field)" class="batch-muted">当前输入已变化，此项需重新生成</small>
            </label>
            <div v-if="Object.keys(info.covers).length" class="batch-inline">
              <figure v-for="item in info.covers" :key="item.profile.key" class="batch-result-cover">
                <img v-if="fileURL(item.file)" :src="fileURL(item.file)" :alt="item.profile.label" />
                <figcaption>{{ item.profile.label }}</figcaption>
              </figure>
            </div>
            <t-button v-if="Object.keys(info.values).length || Object.keys(info.covers).length" size="small" variant="outline" :disabled="busy" @click="apply(row, info)">应用本条配置</t-button>
            <t-button v-if="info.errors.length || (info.requested && !info.result)" size="small" variant="text" :disabled="busy" @click="prepare([row.row_key])">重试本条</t-button>
          </template>
        </div>
      </ResourceSection>
    </div>
    <template #footer>
      <t-button v-if="busy" theme="default" variant="outline" @click="stop">停止准备</t-button>
      <template v-else>
        <t-button v-if="phase === 'setup'" theme="primary" :disabled="!ready" @click="prepare()">预览配置</t-button>
        <t-button v-else theme="primary" :disabled="!canApply" @click="applyAll">应用可用配置</t-button>
      </template>
    </template>
  </ResourceOverlay>
</template>
