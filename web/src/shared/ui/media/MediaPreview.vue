<script setup>
import { ref, watch, computed } from "vue";
import ResourceOverlay from "../resource/ResourceOverlay.vue";
import ResourceEmptyState from "../resource/ResourceEmptyState.vue";
const props = defineProps({
  visible: Boolean,
  title: { type: String, default: "图片预览" },
  images: { type: Array, default: () => [] },
});
const emit = defineEmits(["update:visible"]);
const index = ref(0),
  size = ref("fit"),
  failed = ref(false);
const current = computed(() => props.images[index.value]);
watch(
  () => props.visible,
  () => {
    index.value = 0;
    size.value = "fit";
    failed.value = false;
  },
);
watch(index, () => {
  failed.value = false;
});
</script>
<template>
  <ResourceOverlay
    :visible="visible"
    :title="title"
    :width="1120"
    @update:visible="emit('update:visible', $event)"
  >
    <div class="media-preview" :class="{ 'has-details': !!$slots.details }">
      <div class="preview-image-area">
        <div class="preview-toolbar">
          <t-select
            v-if="images.length > 1"
            v-model="index"
            :options="images.map((im, i) => ({ label: im.label, value: i }))"
            aria-label="选择预览图片"
          /><span v-else>{{ current?.label || "图片" }}</span
          ><t-radio-group v-model="size" variant="default-filled"
            ><t-radio-button value="fit">适应窗口</t-radio-button
            ><t-radio-button value="small">小图效果</t-radio-button
            ><t-radio-button value="actual"
              >实际尺寸</t-radio-button
            ></t-radio-group
          >
        </div>
        <div class="preview-image-stage" :class="`is-${size}`">
          <img
            v-if="current?.src && !failed"
            :src="current.src"
            :alt="current.label || title"
            @error="failed = true"
          /><ResourceEmptyState
            v-else
            title="图片无法预览"
            description="返回更换图片后重试。"
          />
        </div>
        <p class="preview-image-caption">
          {{
            current?.note || "预览当前图片结果；小图模式用于检查标题可读性。"
          }}
        </p>
      </div>
      <aside v-if="$slots.details" class="preview-details">
        <slot name="details" />
      </aside>
    </div>
    <template #footer
      ><t-button
        theme="default"
        variant="outline"
        @click="emit('update:visible', false)"
        >关闭预览</t-button
      ></template
    >
  </ResourceOverlay>
</template>
<style scoped>
.media-preview {
  display: grid;
  gap: 24px;
  min-width: 0;
}
.media-preview.has-details {
  grid-template-columns: minmax(0, 1fr) 300px;
}
.preview-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  font-size: 12px;
  color: var(--wt-text-tertiary);
  margin-bottom: 16px;
}
.preview-toolbar > .t-select__wrap {
  max-width: 210px;
}
.preview-image-stage {
  height: 440px;
  overflow: auto;
  background: var(--wt-bg-page);
  border: 1px solid var(--wt-border);
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  box-sizing: border-box;
}
.preview-image-stage img {
  display: block;
  object-fit: contain;
  max-width: 100%;
  max-height: 100%;
  box-shadow: var(--wt-shadow-card);
}
.preview-image-stage.is-small img {
  width: 200px;
  max-height: 270px;
}
.preview-image-stage.is-actual {
  display: block;
}
.preview-image-stage.is-actual img {
  max-width: none;
  max-height: none;
}
.preview-image-caption {
  font-size: 12px;
  line-height: 1.7;
  color: var(--wt-text-tertiary);
  margin: 12px 0 0;
}
.preview-details {
  border-left: 1px solid var(--wt-border);
  padding-left: 24px;
  max-height: 540px;
  overflow: auto;
}
@media (max-width: 800px) {
  .media-preview.has-details {
    grid-template-columns: 1fr;
  }
  .preview-details {
    border-left: 0;
    border-top: 1px solid var(--wt-border);
    padding: 20px 0 0;
    max-height: none;
  }
  .preview-image-stage {
    height: 340px;
  }
}
@media (max-height: 800px) {
  .preview-image-stage {
    height: 350px;
  }
}
</style>
