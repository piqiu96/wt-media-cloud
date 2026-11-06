<template>
  <section class="resource-card"><slot /></section>
</template>

<style scoped>
.resource-card { padding: 20px; background: var(--wt-bg-card); border: 1px solid var(--wt-border); border-radius: var(--wt-radius-md); box-shadow: var(--wt-shadow-card); }
@media (max-width: 680px) { .resource-card { padding: 16px; } }
</style>
