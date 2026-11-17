<script setup>
defineProps({
  title: { type: String, required: true },
  description: { type: String, default: '' },
  hideTitle: Boolean,
})
</script>

<template>
  <header class="resource-page-header" :class="{ 'is-title-hidden': hideTitle }">
    <div v-if="!hideTitle">
      <h1>{{ title }}</h1>
      <p v-if="description">{{ description }}</p>
    </div>
    <div v-if="$slots.actions" class="resource-page-header__actions"><slot name="actions" /></div>
  </header>
</template>

<style scoped>
.resource-page-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; margin-bottom: var(--wt-space-page-heading, 20px); }
.resource-page-header.is-title-hidden { justify-content: flex-end; }
.resource-page-header h1 { margin: 0; color: var(--wt-text-primary); font-size: var(--wt-font-size-title); font-weight: 500; letter-spacing: 0; line-height: var(--wt-line-height-title); }
.resource-page-header p { margin: 8px 0 0; color: var(--wt-text-tertiary); font-size: 14px; line-height: 1.5; }
.resource-page-header__actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; justify-content: flex-end; }
@media (max-width: 720px) { .resource-page-header { flex-direction: column; align-items: flex-start; } .resource-page-header__actions { justify-content: flex-start; } }
</style>
