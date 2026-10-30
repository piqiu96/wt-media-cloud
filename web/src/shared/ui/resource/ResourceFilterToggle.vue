<script setup>
import { ChevronDownIcon, ChevronUpIcon } from 'tdesign-icons-vue-next'
defineProps({ modelValue: Boolean, count: { type: Number, default: 0 }, controls: { type: String, required: true } })
defineEmits(['update:modelValue'])
</script>

<template>
  <t-button class="resource-filter-toggle" theme="default" variant="text" :aria-expanded="modelValue" :aria-controls="controls" @click="$emit('update:modelValue', !modelValue)">
    {{ modelValue ? '收起筛选' : '更多筛选' }}<span v-if="count">（{{ count }}）</span>
    <template #suffix><ChevronUpIcon v-if="modelValue" /><ChevronDownIcon v-else /></template>
  </t-button>
</template>

<style scoped>
.resource-filter-toggle { min-height: 36px; }
</style>
