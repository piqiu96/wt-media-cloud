<script setup>
import { computed, ref, watch } from "vue";
import { ImageIcon } from "tdesign-icons-vue-next";
import Overlay from "./PrototypeOverlay.vue";
import { coverPreviewState } from "./coverPreview.js";

const props = defineProps({ visible: Boolean, row: Object });
const emit = defineEmits(["update:visible"]);
const selected = ref(0),
  actual = ref(false),
  failed = ref(false),
  dimensions = ref(null);
const state = computed(() => coverPreviewState(props.row));
const current = computed(() => state.value.images[selected.value]);
const resolution = computed(
  () =>
    dimensions.value ||
    (current.value?.width
      ? `${current.value.width} × ${current.value.height} px`
      : ""),
);
watch(
  () => props.visible,
  () => {
    selected.value = 0;
    actual.value = false;
    failed.value = false;
    dimensions.value = null;
  },
);
watch(selected, () => {
  actual.value = false;
  failed.value = false;
  dimensions.value = null;
});
function loaded(event) {
  dimensions.value = `${event.target.naturalWidth} × ${event.target.naturalHeight} px`;
}
function close() {
  emit("update:visible", false);
}
</script>

<template>
  <Overlay
    :visible="visible"
    title="封面预览"
    :width="960"
    @update:visible="close"
  >
    <div class="publishing-cover-preview">
      <div class="cover-preview-toolbar">
        <div class="cover-preview-formats">
          <t-radio-group
            v-if="state.images.length > 1"
            v-model="selected"
            aria-label="封面展示位置"
          >
            <t-radio-button
              v-for="(image, index) in state.images"
              :key="image.key"
              :value="index"
              >{{ image.label }}</t-radio-button
            >
          </t-radio-group>
          <span v-else class="cover-preview-format"
            >{{ row?.platform
            }}<span class="cover-preview-ratio">{{
              current?.key === "source" ? "原图" : current?.key
            }}</span></span
          >
        </div>
        <div class="cover-preview-zoom" role="group" aria-label="预览尺寸">
          <t-button
            variant="text"
            :class="{ 'is-active': !actual }"
            :aria-pressed="!actual"
            @click="actual = false"
            >适应窗口</t-button
          >
          <t-button
            variant="text"
            :class="{ 'is-active': actual }"
            :aria-pressed="actual"
            @click="actual = true"
            >100%</t-button
          >
        </div>
      </div>
      <div
        class="cover-preview-stage"
        :class="{ 'is-actual': actual }"
        tabindex="0"
        :aria-label="actual ? '封面原始尺寸，可滚动查看' : '封面适应窗口预览'"
      >
        <img
          v-if="current?.src && !failed"
          :src="current.src"
          :alt="current.label"
          @load="loaded"
          @error="failed = true"
        />
        <div v-else class="cover-preview-empty" role="status">
          <ImageIcon />
          <strong>{{
            state.pending
              ? "封面待重新生成"
              : failed
                ? "图片无法加载"
                : "此比例暂无封面"
          }}</strong>
          <p>
            {{
              state.pending
                ? "封面配置已更新，请返回编辑生成封面。"
                : "请返回编辑检查图片并重新生成。"
            }}
          </p>
        </div>
      </div>
      <div class="cover-preview-caption">
        <span
          >{{ row?.platform
          }}<span v-if="state.images.length > 1">
            · {{ current?.label }}</span
          ></span
        >
        <span v-if="current?.src && !failed">{{ resolution }}</span>
        <span v-else>尚无可预览的生成结果</span>
      </div>
    </div>
    <template #footer
      ><t-button variant="outline" @click="close">返回编辑</t-button></template
    >
  </Overlay>
</template>

<style scoped>
.publishing-cover-preview {
  min-width: 0;
}
.cover-preview-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}
.cover-preview-formats {
  min-width: 0;
}
.cover-preview-format {
  display: flex;
  align-items: center;
  gap: 10px;
  color: var(--wt-color-text-primary);
  font-size: 14px;
  font-weight: 500;
}
.cover-preview-ratio {
  padding: 3px 8px;
  border-radius: 4px;
  background: var(--wt-color-bg-subtle);
  color: var(--wt-color-text-secondary);
  font-size: 12px;
  font-weight: 400;
}
.cover-preview-zoom {
  display: flex;
  flex-shrink: 0;
  gap: 4px;
}
.cover-preview-zoom :deep(.t-button) {
  color: var(--wt-color-text-secondary);
}
.cover-preview-zoom :deep(.t-button.is-active) {
  color: var(--wt-color-primary);
  background: var(--wt-color-primary-light);
}
.cover-preview-stage {
  height: clamp(280px, 52vh, 500px);
  overflow: auto;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  box-sizing: border-box;
  background: var(--wt-color-bg-subtle);
  border-radius: 6px;
}
.cover-preview-stage img {
  display: block;
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
  box-shadow: var(--wt-shadow-card);
}
.cover-preview-stage.is-actual {
  display: block;
}
.cover-preview-stage.is-actual img {
  max-width: none;
  max-height: none;
}
.cover-preview-stage:focus-visible {
  outline: 2px solid var(--wt-color-primary);
  outline-offset: 2px;
}
.cover-preview-caption {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  padding-top: 12px;
  font-size: 12px;
  line-height: 20px;
  color: var(--wt-color-text-assist);
}
.cover-preview-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 100%;
  text-align: center;
  color: var(--wt-color-text-assist);
}
.cover-preview-empty :deep(svg) {
  width: 36px;
  height: 36px;
  margin-bottom: 16px;
}
.cover-preview-empty strong {
  color: var(--wt-color-text-secondary);
  font-size: 14px;
  font-weight: 500;
}
.cover-preview-empty p {
  margin: 8px 0 0;
  font-size: 12px;
  line-height: 20px;
}
@media (max-width: 600px) {
  .cover-preview-toolbar {
    align-items: flex-start;
    flex-wrap: wrap;
    gap: 12px;
  }
  .cover-preview-formats :deep(.t-radio-group) {
    display: flex;
    flex-wrap: wrap;
  }
  .cover-preview-stage {
    padding: 16px;
    height: 42vh;
    min-height: 220px;
  }
  .cover-preview-caption {
    flex-wrap: wrap;
    gap: 4px;
  }
}
</style>
