<template>
  <div class="resource-row-actions" role="group" aria-label="行操作"><slot /></div>
</template>

<style scoped>
.resource-row-actions { display: inline-flex; align-items: center; gap: 12px; white-space: nowrap; }
/* 仅作用于显式使用本容器的行操作，不改变已有页面按钮的默认密度。 */
.resource-row-actions :deep(.t-button--variant-text) { min-width: 0; min-height: 28px; height: 28px; padding: 0; border: 0; border-radius: 0; background: transparent; font-size: 12px; }
.resource-row-actions :deep(.t-button--variant-text:hover:not(:disabled)) { background: transparent; color: var(--wt-primary-hover); }
.resource-row-actions :deep(.t-button--variant-text.t-is-disabled) { background: transparent; color: var(--td-text-color-disabled); }
.resource-row-actions :deep(.is-danger) { color: var(--wt-text-secondary); }
.resource-row-actions :deep(.is-danger:hover:not(:disabled)) { color: var(--wt-danger); }
</style>
