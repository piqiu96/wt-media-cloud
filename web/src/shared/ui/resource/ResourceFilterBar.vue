<template>
  <div class="resource-filter-bar"><slot /></div>
</template>
<style scoped>
.resource-filter-bar { display:flex; flex-wrap:wrap; align-items:flex-end; gap:20px 24px; padding-bottom:24px; margin-bottom:20px; border-bottom:1px solid var(--wt-border); min-width:0; }
.resource-filter-bar :deep(.wt-filter-field) { display:flex; flex-direction:column; align-items:stretch; gap:8px; min-width:0; width:100%; flex:0 1 180px; white-space:normal; }
.resource-filter-bar :deep(.wt-filter-field--wide) { flex:1 1 280px; max-width:400px; }
.resource-filter-bar :deep(.wt-filter-field__label) { color:var(--wt-text-tertiary); font-size:13px; font-weight:400; line-height:20px; }
.resource-filter-bar :deep(.wt-filter-field:has(.resource-date-range)) { flex:0 1 300px; max-width:100%; }
.resource-filter-bar :deep(.wt-filter-actions) { display:flex; flex-wrap:wrap; align-items:center; gap:8px; margin-left:auto; }
.resource-filter-bar :deep(.wt-advanced-filters) { display:flex; flex-wrap:wrap; align-items:flex-end; gap:20px 24px; flex-basis:100%; min-width:0; }
.resource-filter-bar :deep(.t-input), .resource-filter-bar :deep(.t-select-input .t-input) { min-height:36px; }
@media(max-width:680px) { .resource-filter-bar :deep(.wt-filter-field), .resource-filter-bar :deep(.wt-filter-field--wide) { flex:1 1 180px; max-width:none; } }
</style>
