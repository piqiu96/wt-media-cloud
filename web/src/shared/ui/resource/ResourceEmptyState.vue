<script setup>
import { FolderOpenIcon } from 'tdesign-icons-vue-next'
defineProps({ title: { type: String, required: true }, description: { type: String, default: '' } })
</script>

<template>
  <div class="resource-empty-state" role="status">
    <FolderOpenIcon class="resource-empty-state__icon" />
    <h3>{{ title }}</h3><p v-if="description">{{ description }}</p>
    <div v-if="$slots.default" class="resource-empty-state__actions"><slot /></div>
  </div>
</template>

<style scoped>
.resource-empty-state { display: flex; flex-direction: column; align-items: center; text-align: center; padding: 32px 20px; color: var(--wt-text-tertiary); }
.resource-empty-state__icon { width: 32px; height: 32px; color: var(--wt-text-tertiary); margin-bottom: 12px; }
.resource-empty-state h3 { margin: 0; color: var(--wt-text-secondary); font-size: 14px; font-weight: 500; }
.resource-empty-state p { margin: 6px 0 0; max-width: 420px; font-size: 12px; line-height: 1.7; }
.resource-empty-state__actions { margin-top: 14px; }
</style>
