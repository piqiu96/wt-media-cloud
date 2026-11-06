<script setup>
import ResourceAdvancedToggle from './ResourceAdvancedToggle.vue'
defineProps({ modelValue: Boolean, count: { type: Number, default: 0 }, controls: { type: String, required: true } })
defineEmits(['update:modelValue'])
</script>
<template><ResourceAdvancedToggle :model-value="modelValue" :count="count" :controls="controls" @update:model-value="$emit('update:modelValue', $event)" /></template>
