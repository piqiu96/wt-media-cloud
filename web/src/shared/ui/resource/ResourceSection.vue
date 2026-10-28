<script setup>
defineProps({ title: { type: String, required: true }, description: { type: String, default: '' } })
</script>

<template>
  <section class="resource-section" :aria-label="title">
    <header class="resource-section__header"><h3>{{ title }}</h3><p v-if="description">{{ description }}</p></header>
    <div class="resource-section__body"><slot /></div>
  </section>
</template>

<style scoped>
.resource-section + .resource-section { margin-top: 24px; }
.resource-section__header { padding-bottom: 12px; margin-bottom: 16px; border-bottom: 1px solid var(--wt-border); }
.resource-section__header h3 { margin: 0; color: var(--wt-text-primary); font-size: 14px; font-weight: 600; line-height: 1.6; }
.resource-section__header p { margin: 4px 0 0; color: var(--wt-text-tertiary); font-size: 12px; line-height: 1.6; }
.resource-section__body { min-width: 0; }
.resource-section :deep(.t-descriptions__label) { color: var(--wt-text-tertiary); font-size: 13px; }
.resource-section :deep(.t-descriptions__content) { color: var(--wt-text-secondary); font-size: 13px; overflow-wrap: anywhere; }
.resource-section :deep(.t-descriptions__label), .resource-section :deep(.t-descriptions__content) { vertical-align: top; }
.resource-section :deep(.t-form__item:last-child) { margin-bottom: 0; }
.resource-section :deep(.resource-form-grid) { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 20px 16px; }
.resource-section :deep(.resource-form-grid .t-form__item) { min-width: 0; margin-bottom: 0; }
.resource-section :deep(.resource-form-grid .t-form__controls-content) { flex-direction: column; align-items: stretch; gap: 4px; }
.resource-section :deep(.resource-form-full) { grid-column: 1 / -1; }
.resource-section :deep(.resource-form-value) { display: block; min-height: 32px; line-height: 32px; color: var(--wt-text-secondary); }
.resource-section :deep(.resource-form-grid .t-input-number), .resource-section :deep(.resource-form-grid .t-date-picker) { width: 100%; }
@media (max-width: 600px) { .resource-section :deep(.resource-form-grid) { grid-template-columns: 1fr; } }
</style>
