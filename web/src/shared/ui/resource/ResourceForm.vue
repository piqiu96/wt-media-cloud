<template>
  <t-form label-align="top" class="resource-form"><slot /></t-form>
</template>

<style scoped>
.resource-form :deep(.t-form__item) { margin-bottom: 20px; }
.resource-form :deep(.t-form__item:last-child) { margin-bottom: 0; }
.resource-form :deep(.t-form__controls-content) { flex-direction: column; align-items: stretch; gap: 6px; min-width: 0; }
.resource-form :deep(.t-form__label) { padding-bottom: 8px; }
.resource-form :deep(.t-input-number), .resource-form :deep(.t-date-picker) { width: 100%; }
</style>
