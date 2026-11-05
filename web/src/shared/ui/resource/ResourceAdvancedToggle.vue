<script setup>
import { ChevronRightIcon } from 'tdesign-icons-vue-next'
defineProps({ modelValue: Boolean, controls: String })
const emit = defineEmits(['update:modelValue'])
</script>
<template>
  <div class="resource-advanced-toggle">
    <t-button theme="primary" variant="text" :aria-expanded="modelValue" :aria-controls="controls" @click="emit('update:modelValue', !modelValue)">
      <ChevronRightIcon :class="{ 'is-expanded': modelValue }" />高级筛选
    </t-button>
  </div>
</template>
<style scoped>
.resource-advanced-toggle { flex: 0 0 100%; }
.resource-advanced-toggle .t-button { height: 32px; padding: 0; font-size: 14px; font-weight: 400; }
.resource-advanced-toggle .t-icon { margin-right: 6px; transition: transform .15s; }
.resource-advanced-toggle .is-expanded { transform: rotate(90deg); }
</style>
