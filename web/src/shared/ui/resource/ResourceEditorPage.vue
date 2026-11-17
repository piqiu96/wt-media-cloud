<script setup>
import ResourceDetailPage from './ResourceDetailPage.vue'
defineProps({ title: { type: String, required: true }, backLabel: { type: String, default: '返回列表' }, hideTitle: Boolean, busy: Boolean })
defineEmits(['back'])
</script>
<template>
  <ResourceDetailPage class="resource-editor-page" :title="title" :hide-title="hideTitle" :busy="busy" :back-label="backLabel" @back="!busy && $emit('back')">
    <div class="resource-editor-page__body" :aria-busy="busy"><slot /></div>
    <template v-if="$slots.footer" #footer><slot name="footer" /></template>
  </ResourceDetailPage>
</template>
<style scoped>
.resource-editor-page { min-width: 0; }
.resource-editor-page__body { min-width: 0; }
.resource-editor-page :deep(.resource-detail-page__footer) { align-items: center; }
.resource-editor-page :deep(.resource-detail-page__back:disabled) { cursor: not-allowed; }
.resource-editor-page :deep(.resource-overlay__body) { max-width: none; margin: 0; }
.resource-editor-page :deep(.resource-detail-page__footer > .resource-overlay__footer) { display: contents; }
.resource-editor-page :deep(.t-input), .resource-editor-page :deep(.t-select) { font-weight: 400; }
.resource-editor-page :deep(.resource-editor-section) { min-width: 0; padding: 24px; border: 1px solid var(--wt-border); border-radius: 8px; background: var(--wt-bg-card); }
@media (max-width: 680px) { .resource-editor-page :deep(.resource-editor-section) { padding: 20px 16px; } }
</style>
