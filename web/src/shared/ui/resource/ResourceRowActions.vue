<template>
  <div class="resource-row-actions" role="group" aria-label="行操作"><slot /></div>
</template>

<style scoped>
.resource-row-actions { display: inline-flex; align-items: center; gap: 8px; white-space: nowrap; }
/* 仅作用于显式使用本容器的行操作，不改变已有页面按钮的默认密度。 */
.resource-row-actions :deep(.t-button--variant-text) { min-width: 0; min-height: 32px; height: 32px; padding: 0 4px; border: 0; border-radius: 4px; background: transparent; font-size: 13px; color: var(--wt-primary); }
.resource-row-actions :deep(.t-button--variant-text:hover:not(:disabled)) { background: var(--wt-info-bg); color: var(--wt-primary-hover); }
.resource-row-actions :deep(.t-button--variant-text.t-is-disabled) { background: transparent; color: var(--td-text-color-disabled); }
.resource-row-actions :deep(.is-primary) { padding-inline: 10px; background: var(--wt-info-bg); font-weight: 500; }
.resource-row-actions :deep(.is-danger) { color: var(--wt-text-primary); }
.resource-row-actions :deep(.is-danger:hover:not(:disabled)) { color: var(--wt-danger); background: var(--wt-danger-bg); }
</style>
