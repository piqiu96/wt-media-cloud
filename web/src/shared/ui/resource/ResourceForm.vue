<template>
  <t-form label-align="top" class="resource-form"><slot /></t-form>
</template>

<style scoped>
.resource-form :deep(.t-form__item) { margin-bottom: 20px; }
.resource-form :deep(.t-form__item:last-child) { margin-bottom: 0; }
.resource-form :deep(.t-form__controls-content) { flex-direction: column; align-items: stretch; gap: 6px; min-width: 0; }
.resource-form :deep(.t-form__controls-content > .t-switch) { align-self: flex-start; }
.resource-form :deep(.t-form__label) { padding-bottom: 8px; color: var(--wt-text-primary); font-size: var(--wt-font-size-table); font-weight: 500; line-height: var(--wt-line-height-table); }
.resource-form :deep(.t-input-number), .resource-form :deep(.t-date-picker) { width: 100%; }
.resource-form :deep(input[type="file"]) { width:100%; min-width:0; font-size:14px; line-height:36px; color:var(--wt-text-secondary); }
.resource-form :deep(input[type="file"]::file-selector-button) { height:36px; padding:0 12px; margin-right:12px; border:1px solid transparent; border-radius:4px; background:var(--wt-color-bg-subtle); color:var(--wt-text-primary); cursor:pointer; font:inherit; }
</style>
