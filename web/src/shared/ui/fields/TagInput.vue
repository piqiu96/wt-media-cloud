<script setup>
import { ref } from 'vue'
import { Input as TInput, Tag as TTag } from 'tdesign-vue-next'
const tags = defineModel({ type: Array, default: () => [] })
const draft = ref('')
function add() { const value = draft.value.trim(); if (value && !tags.value.includes(value)) tags.value = [...tags.value, value]; draft.value = '' }
</script>

<template>
  <div class="business-tag-input">
    <TTag v-for="tag in tags" :key="tag" closable variant="outline" @close="tags = tags.filter(t => t !== tag)">{{ tag }}</TTag>
    <TInput v-model="draft" borderless placeholder="输入标签，按回车添加" @enter="add" />
  </div>
  <p class="business-tag-input__hint">回车添加一个标签，可点击 × 移除。</p>
</template>

<style scoped>
.business-tag-input { display: flex; align-items: center; flex-wrap: wrap; gap: 6px; padding: 4px 8px; border: 1px solid var(--wt-border); border-radius: var(--wt-radius-sm); }
.business-tag-input:focus-within { border-color: var(--wt-primary); }
.business-tag-input :deep(.t-input__wrap) { flex: 1; min-width: 160px; }
.business-tag-input__hint { margin: 6px 0 0; font-size: 12px; color: var(--wt-text-tertiary); }
</style>
