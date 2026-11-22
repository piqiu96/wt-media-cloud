<script setup>
import { computed } from "vue";
import FeedbackAlert from "../../../shared/ui/feedback/FeedbackAlert.vue";
import Overlay from "../../../shared/ui/resource/ResourceOverlay.vue";
import ResourceSection from "../../../shared/ui/resource/ResourceSection.vue";
import ResourceDescriptions from "../../../shared/ui/resource/ResourceDescriptions.vue";
import ResourceEmptyState from "../../../shared/ui/resource/ResourceEmptyState.vue";
import { formatDateTime } from "../../../shared/utils/datetime.js";
const props = defineProps({
  image: Object,
  loading: Boolean,
  error: String,
  references: { type: Array, default: () => [] },
});
defineEmits(["close", "retry"]);
const fileSize = computed(() => {
  const n = Number(props.image?.size);
  return n > 0
    ? n >= 1048576
      ? `${(n / 1048576).toFixed(1)} MB`
      : `${Math.ceil(n / 1024)} KB`
    : "—";
});
</script>
<template>
  <Overlay
    :visible="!!image"
    kind="drawer"
    title="图片详情"
    :width="800"
    @update:visible="$emit('close')"
  >
    <FeedbackAlert v-if="error" theme="error" :message="error"
      ><t-button variant="text" @click="$emit('retry')"
        >重试</t-button
      ></FeedbackAlert
    >
    <div v-if="image" class="image-library-detail">
      <div class="image-detail-preview">
        <img :src="image.src" :alt="image.name" />
      </div>
      <ResourceSection title="基本信息">
        <ResourceDescriptions>
          <t-descriptions-item label="ID">{{ image.id }}</t-descriptions-item>
          <t-descriptions-item label="名称">{{
            image.name || "—"
          }}</t-descriptions-item>
          <t-descriptions-item label="内容类目">{{
            image.category || "—"
          }}</t-descriptions-item>
          <t-descriptions-item label="用途">{{
            image.uses?.join("、") || "—"
          }}</t-descriptions-item>
          <t-descriptions-item label="使用状态">{{
            image.enabled ? "启用" : "停用"
          }}</t-descriptions-item>
        </ResourceDescriptions>
      </ResourceSection>
      <ResourceSection title="文件信息">
        <ResourceDescriptions>
          <t-descriptions-item label="尺寸">{{
            image.width && image.height
              ? `${image.width} × ${image.height} px`
              : "—"
          }}</t-descriptions-item>
          <t-descriptions-item label="格式">{{
            image.format || "—"
          }}</t-descriptions-item>
          <t-descriptions-item label="文件大小">{{
            fileSize
          }}</t-descriptions-item>
          <t-descriptions-item label="来源">{{
            image.source || "—"
          }}</t-descriptions-item>
          <t-descriptions-item label="创建时间">{{
            image.createdAt ? formatDateTime(image.createdAt) : "—"
          }}</t-descriptions-item>
          <t-descriptions-item label="更新时间">{{
            image.updatedAt ? formatDateTime(image.updatedAt) : "—"
          }}</t-descriptions-item>
        </ResourceDescriptions>
      </ResourceSection>
      <ResourceSection title="引用记录"><template #title-extra><span class="reference-count">{{ references.length }}</span></template>
        <t-loading v-if="loading" />
        <ul v-else-if="references.length" class="image-reference-list">
          <li v-for="r in references" :key="r.kind + r.id">
            <span
              >{{
                {
                  publication: "发布任务",
                  publication_attempt: "执行记录",
                  composite_output: "成片缩略图",
 cover: "模板成品",
                  cover_template: "封面模板",
                }[r.kind] || r.kind
              }}
              {{ r.id }}</span
            >
          </li>
        </ul>
        <ResourceEmptyState v-else-if="!error" title="暂无可查看的引用" />
      </ResourceSection>
    </div>
    <template #footer
      ><t-button theme="default" variant="outline" @click="$emit('close')"
        >关闭</t-button
      ></template
    >
  </Overlay>
</template>
<style scoped>
.image-library-detail {
  display: flex;
  flex-direction: column;
  gap: 24px;
}
.image-detail-preview {
  height: 260px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--wt-bg-page);
  border: 1px solid var(--wt-border);
  border-radius: 4px;
  overflow: hidden;
}
.image-detail-preview img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}
.reference-count { font-size:13px; font-weight:400; color:var(--wt-text-tertiary); }

.image-reference-list {
  padding: 0;
  margin: 0;
  list-style: none;
}
.image-reference-list li {
  display: grid;
  grid-template-columns: 1fr;
  gap: 16px;
  padding: 12px 0;
  border-bottom: 1px solid var(--wt-border);
  font-size: 13px;
  overflow-wrap: anywhere;
}
</style>
