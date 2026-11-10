<script setup>
import { ref, computed } from "vue";
import ImageEditor from "../../../src/shared/ui/media/ImageEditor.vue";
import MediaPreview from "../../../src/shared/ui/media/MediaPreview.vue";
import { renderDocument } from "./covers.js";
const props = defineProps({ modelValue: Object, fixedSize: Boolean });
const emit = defineEmits(["update:modelValue", "ready"]);
const engine = ref(null),
  error = ref(""),
  busy = ref(false),
  previewOpen = ref(false),
  image = ref(""),
  sample = ref("");
const doc = computed({
  get: () => props.modelValue,
  set: (v) => emit("update:modelValue", v),
});
async function render(download = false) {
  error.value = "";
  busy.value = true;
  try {
    image.value = await renderDocument(doc.value, {
      shortTitle: sample.value || null,
    });
    if (download) {
      const a = document.createElement("a");
      a.href = image.value;
      a.download = "cover-" + doc.value.width + "x" + doc.value.height + ".png";
      a.click();
    } else previewOpen.value = true;
  } catch (e) {
    error.value = e.message;
  } finally {
    busy.value = false;
  }
}
function slot(layer, value) {
  doc.value = {
    ...doc.value,
    layers: doc.value.layers.map((l) =>
      l.id === layer.id ? { ...l, slot: value } : l,
    ),
  };
}
defineExpose({ setBackground: (src) => engine.value?.setBackground(src) });
</script>
<template>
  <div class="prototype-editor">
    <div class="unified-tools">
      <label
        >安全区（像素）<input
          type="number"
          :value="doc.safe ?? 16"
          min="0"
          max="150"
          @change="
            doc = { ...doc, safe: Number($event.target.value) }
          " /></label
      ><label
        >预览短标题<input
          v-model="sample"
          placeholder="空白使用画布文字" /></label
      ><t-button variant="text" @click="sample = '实战技巧'"
        >短标题样例</t-button
      ><t-button
        variant="text"
        @click="sample = '三角洲行动新手进阶转点与实战技巧完整指南'"
        >长标题样例</t-button
      ><t-button :loading="busy" variant="outline" @click="render(false)"
        >检查统一渲染成品</t-button
      ><t-button variant="outline" @click="render(true)">导出审核 PNG</t-button>
    </div>
    <p>
      坐标和安全区均为画布像素。可见图层必须完全位于安全区内；保存、预览和导出使用同一文字布局与渲染规则。仅支持已验证的系统字体。
    </p>
    <div class="unified-tools">
      <label
        v-for="l in doc.layers.filter((l) => l.kind === 'text')"
        :key="l.id"
        >{{ l.name
        }}<select :value="l.slot || ''" @change="slot(l, $event.target.value)">
          <option value="">固定文字</option>
          <option value="main_title">动态主标题</option>
          <option value="sub_title">动态副标题</option>
        </select></label
      >
    </div>
    <t-alert v-if="error" theme="warning" :message="error" />
    <ImageEditor
      ref="engine"
      v-model="doc"
      :fixed-size="fixedSize"
      @ready="emit('ready', $event)"
    />
    <MediaPreview
      v-model:visible="previewOpen"
      title="统一渲染成品 · 大图 / 小图"
      :images="[
        {
          src: image,
          label: '实际 PNG',
          note: '与内容工作区和正式快照共用渲染规则',
        },
      ]"
    />
  </div>
</template>
<style scoped>
.unified-tools {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
  align-items: center;
  margin: 16px 0;
}
.unified-tools input,
.unified-tools select {
  padding: 7px;
  border: 1px solid #d6deea;
  border-radius: 4px;
}
.unified-tools input[type="number"] {
  width: 70px;
}
.prototype-editor :deep(.editor-export) {
  display: none;
}
.prototype-editor > p {
  font-size: 12px;
  color: #63718a;
}
</style>
