<script setup>
import { ChevronDownIcon } from 'tdesign-icons-vue-next'
defineProps({ modelValue: Boolean, count: { type: Number, default: 0 }, controls: String })
const emit = defineEmits(['update:modelValue'])
</script>
<template>
  <div class="resource-advanced-toggle">
    <t-button theme="primary" variant="text" :aria-expanded="modelValue" :aria-controls="controls" @click="emit('update:modelValue', !modelValue)">
      高级筛选<span v-if="count">（已生效 {{ count }} 项）</span><ChevronDownIcon :class="{ 'is-expanded': modelValue }" />
    </t-button>
  </div>
</template>
<style scoped>
.resource-advanced-toggle { flex:0 0 100%; }
.resource-advanced-toggle .t-button { height:28px; padding:0; font-size:14px; font-weight:400; }
.resource-advanced-toggle .t-icon { margin-left:6px; transition:transform .15s; }
.resource-advanced-toggle .is-expanded { transform:rotate(180deg); }
</style>
