<script setup>
import { ref, watch, onBeforeUnmount, computed } from "vue";
import { createImageClient } from "../../../shared/api/image.js";
import ResourceRequiredLabel from "../../../shared/ui/resource/ResourceRequiredLabel.vue";
import ResourceFieldHelp from "../../../shared/ui/resource/ResourceFieldHelp.vue";
import FeedbackAlert from "../../../shared/ui/feedback/FeedbackAlert.vue";
import PublishingResourcePicker from "./PublishingResourcePicker.vue";
import { coverRenderInput, coverResultMatches, coverMatchesRatio } from "../coverPreparation.js";
const props = defineProps({
  config: Object,
  aspect: {type:Array, default:()=>[16,9]},
  output: Object,
  category: String,
  disabled: Boolean,
});
const emit = defineEmits([
  "update:config",
  "rendered",
  "choose-image",
  "use-output",
]);
const client = createImageClient();
const templates = ref([]),
  templateSearch = ref(""),
  templatePage = ref(0),
  hasMoreTemplates = ref(false),
  selectedTemplate = ref(null),
  loading = ref(false),
  rendering = ref(false),
  error = ref(""),
  progress = ref(""),
  backgroundPicker = ref(false),
  backgroundName = ref("");
let selectionSequence = 0,
  lookupSequence = 0,
  selectedSequence = 0,
  renderSequence = 0,
  controller;
const options = computed(() => {
  const items = templates.value.map((t) => ({
    label: `${t.name} · ${t.canvas_width}×${t.canvas_height}`,
    value: String(t.id),
  }));
  if (
    props.config.template_id &&
    !items.some((t) => t.value === props.config.template_id)
  )
    items.unshift({
      label:
        selectedTemplate.value?.name || `原模板 ${props.config.template_id}`,
      value: props.config.template_id,
    });
  return items;
});
function update(patch) {
  emit("update:config", { ...props.config, ...patch });
}
async function loadTemplate(search = "", append = false) {
  if (append && (loading.value || !hasMoreTemplates.value)) return;
  const sequence = ++lookupSequence;
  if (!append) {
    templateSearch.value = search;
    templatePage.value = 0;
    templates.value = [];
    hasMoreTemplates.value = false;
  }
  loading.value = true;
  error.value = "";
  try {
    // The API's width/height filters require exact dimensions. Keep accepting
    // every resolution of the requested ratio and page past other ratios.
    do {
      const nextPage = templatePage.value + 1;
      const page = await client.listCoverTemplate({
        enabled: true,
        category_id: props.category,
        search: templateSearch.value,
        page: nextPage,
        page_size: 100,
      });
      if (sequence !== lookupSequence) return;
      const items = page.items || [];
      const matching = items.filter((t) =>
        coverMatchesRatio(
          { width: t.canvas_width, height: t.canvas_height },
          ...props.aspect,
        ),
      );
      templates.value = [...templates.value, ...matching];
      templatePage.value = nextPage;
      hasMoreTemplates.value =
        items.length > 0 && nextPage * 100 < Number(page.total);
      if (matching.length) break;
    } while (hasMoreTemplates.value);
  } catch (e) {
    if (sequence === lookupSequence)
      error.value = e.message || "模板读取失败，请重试";
  } finally {
    if (sequence === lookupSequence) loading.value = false;
  }
}
function loadMoreTemplates() {
  return loadTemplate(templateSearch.value, true);
}
async function readSelected(id) {
  const sequence = ++selectedSequence;
  selectedTemplate.value = null;
  if (!id) return;
  try {
    const item = await client.getCoverTemplate(id);
    if (sequence === selectedSequence) selectedTemplate.value = item;
  } catch (e) {
    if (sequence === selectedSequence)
      error.value = e.message || "原模板暂不可用，已生成封面仍可核对使用";
  }
}
async function selectTemplate(id) {
  if (props.disabled) return;
  const sequence = ++selectionSequence;
  const context = JSON.stringify({
    method: props.config.method,
    category: props.category,
  });
  const current = () =>
    sequence === selectionSequence &&
    !props.disabled &&
    context ===
      JSON.stringify({ method: props.config.method, category: props.category });
  let item = templates.value.find((t) => String(t.id) === String(id));
  if (!item && id) {
    try {
      item = await client.getCoverTemplate(id);
    } catch (e) {
      if (current()) error.value = e.message;
      return;
    }
  }
  if (!current()) return;
  if (!item) {
    update({ template_id: "", published_revision: 0 });
    return;
  }
  if(!coverMatchesRatio({width:item.canvas_width,height:item.canvas_height},...props.aspect)) {error.value="模板比例不匹配，请重新选择";return;}
  selectedTemplate.value = item;
  update({
    template_id: String(item.id),
    published_revision: item.published_revision,
    background_mode: item.published_config?.backgroundMode || "business",
  });
}
function chooseBackground(row) {
  backgroundName.value = row.name;
  update({
    background_source: "image",
    background_image_asset_id: String(row.image_asset_id),
  });
}
async function generate() {
  if (rendering.value || props.disabled) return;
  const sequence = ++renderSequence;
  const signature = JSON.stringify({
    config: props.config,
    output: props.output?.id,
  });
  const config = { ...props.config },
    output = props.output;
  controller = new AbortController();
  rendering.value = true;
  error.value = "";
  progress.value = "正在检查模板";
  try {
    const current = await client.getCoverTemplate(config.template_id);
    if (
      sequence !== renderSequence ||
      props.disabled ||
      signature !==
        JSON.stringify({ config: props.config, output: props.output?.id })
    )
      return;
    if (
      !coverMatchesRatio({width:current.canvas_width,height:current.canvas_height},...props.aspect) ||
      !current.enabled ||
      current.published_revision !== config.published_revision
    )
      throw new Error("模板已更新或停用，请重新选择可用模板");
    const input = coverRenderInput(
      config,
      output,
      crypto.randomUUID(),
      String(sequence),
    );
    progress.value = "正在生成封面";
    const response = await client.renderCover(input, {
      signal: controller.signal,
      onEvent: ({ event, data }) => {
        if (sequence === renderSequence && event === "progress")
          progress.value = data.message || "正在生成封面";
      },
    });
    const result = response.artwork;
    if (
      sequence !== renderSequence ||
      props.disabled ||
      signature !==
        JSON.stringify({ config: props.config, output: props.output?.id })
    )
      return;
    if (!coverResultMatches(result, input))
      throw new Error("封面结果与当前配置不一致，请重新生成");
    emit("rendered", result);
    progress.value = "已生成实际封面，请预览并核对";
  } catch (e) {
    if (sequence === renderSequence && e.name !== "AbortError") {
      progress.value = "";
      error.value = e.message || "封面生成失败，配置已保留，请重试";
    }
  } finally {
    if (sequence === renderSequence) rendering.value = false;
  }
}
watch(
  () => [props.config, props.output?.id],
  () => {
    renderSequence++;
    controller?.abort();
    rendering.value = false;
    progress.value = "";
    error.value = "";
  },
  { deep: true, flush: "sync" },
);
watch(
  () => props.config.template_id,
  (id) => readSelected(id),
  { immediate: true },
);
watch(
  [
    () => props.config.method,
    () => props.category,
    () => props.aspect[0],
    () => props.aspect[1],
  ],
  () => {
    lookupSequence++;
    loading.value = false;
    if (props.config.method === "template") loadTemplate();
  },
  { immediate: true, flush: "sync" },
);
onBeforeUnmount(() => {
  selectionSequence++;
  lookupSequence++;
  selectedSequence++;
  renderSequence++;
  controller?.abort();
});
</script>
<template>
  <div class="publication-cover-controls">
    <div class="quick-cover-methods" role="radiogroup" aria-label="封面来源">
      <label
        v-for="choice in [
          {
            value: 'image',
            title: '使用现成封面',
            note: '默认使用成片封面，可从图片库更换',
          },
          {
            value: 'template',
            title: '模板合成',
            note: '选择模板、底图并填写封面文字',
          },
        ]"
        :key="choice.value"
        class="quick-cover-method"
        :class="{ selected: config.method === choice.value }"
      >
        <input
          type="radio"
          :name="`publication-cover-method-${aspect.join('-')}`"
          :value="choice.value"
          :checked="config.method === choice.value"
          :disabled="disabled"
          @change="update({ method: choice.value })"
        />
        <span
          ><strong>{{ choice.title }}</strong
          ><small>{{ choice.note }}</small></span
        >
      </label>
    </div>
    <template v-if="config.method === 'image'">
      <div class="publication-cover-image-actions">
        <t-button
          variant="outline"
          :disabled="disabled || !output?.thumbnail?.file_id || !coverMatchesRatio(output.thumbnail,...aspect)"
          @click="emit('use-output')"
          >使用成片封面</t-button
        ><t-button
          variant="outline"
          :disabled="disabled"
          @click="emit('choose-image')"
          >选择图片</t-button
        >
      </div>
    </template>
    <template v-else>
      <label class="publication-cover-field"
        ><ResourceRequiredLabel>封面模板</ResourceRequiredLabel
        ><t-select
          :value="config.template_id"
          :options="options"
          filterable
          :loading="loading"
          :disabled="disabled || rendering"
          placeholder="搜索并选择封面模板"
          @search="loadTemplate($event)"
          @change="selectTemplate"
      /></label>
      <div v-if="hasMoreTemplates">
        <t-button
          variant="text"
          :loading="loading"
          :disabled="disabled || rendering || loading"
          @click="loadMoreTemplates"
          >加载更多模板</t-button
        >
      </div>
      <div v-if="error && !templates.length">
        <t-button variant="text" :disabled="disabled || loading" @click="loadTemplate(templateSearch)"
          >重新读取模板</t-button
        >
      </div>
      <label
        v-if="config.background_mode === 'business'"
        class="publication-cover-field"
        ><span class="quick-field-heading"
          >底图<ResourceFieldHelp
            label="底图"
            content="默认使用成片真实封面，也可从图片库选择底图。"
        /></span>
        <div class="publication-cover-image-actions">
          <t-button
            :variant="
              config.background_source === 'output' ? 'base' : 'outline'
            "
            :disabled="disabled || !output?.thumbnail?.file_id"
            @click="
              update({
                background_source: 'output',
                background_image_asset_id: '',
              })
            "
            >使用成片封面</t-button
          ><t-button
            variant="outline"
            :disabled="disabled"
            @click="backgroundPicker = true"
            >{{
              config.background_source === "image"
                ? backgroundName || "更换底图"
                : "从图片库选择"
            }}</t-button
          >
        </div></label
      >
      <p v-else class="publication-cover-note">
        {{
          config.background_mode === "fixed"
            ? "使用模板固定图片"
            : "使用模板纯色背景"
        }}
      </p>
      <label class="publication-cover-field"
        ><ResourceRequiredLabel>封面标题</ResourceRequiredLabel
        ><t-input
          :value="config.title"
          :disabled="disabled"
          placeholder="输入封面主标题"
          @change="update({ title: $event })"
      /></label>
      <div>
        <t-button
          variant="outline"
          :loading="rendering"
          :disabled="disabled"
          @click="generate"
          >生成封面</t-button
        >
      </div>
      <p v-if="progress" class="publication-cover-note" role="status">
        {{ progress }}
      </p>
    </template>
    <FeedbackAlert v-if="error" theme="error" :message="error" />
    <PublishingResourcePicker
      :visible="backgroundPicker"
      kind="image"
      usage="background"
      :category="category"
      @update:visible="backgroundPicker = $event"
      @selected="chooseBackground"
    />
  </div>
</template>
<style scoped>
.publication-cover-controls {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
  width: 100%;
}
.quick-cover-methods {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}
.quick-cover-method {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 16px;
  border: 1px solid var(--td-component-border);
  border-radius: var(--td-radius-default);
  cursor: pointer;
}
.quick-cover-method.selected {
  border-color: var(--td-brand-color);
  background: var(--td-brand-color-light);
}
.quick-cover-method span {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}
.quick-cover-method strong {
  font-weight: 500;
  font-size: 14px;
}
.quick-cover-method small,
.publication-cover-note {
  color: var(--td-text-color-secondary);
  font-size: 12px;
  margin: 0;
}
.publication-cover-image-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
.publication-cover-field {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}
.quick-field-heading {
  display: flex;
  align-items: center;
  gap: 8px;
}
@media (max-width: 600px) {
  .quick-cover-methods {
    grid-template-columns: 1fr;
  }
}
</style>
