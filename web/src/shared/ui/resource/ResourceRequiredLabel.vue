<script setup>
defineProps({ required: { type: Boolean, default: true } });
</script>
<template><span class="resource-required-label" :class="{ 'is-required': required }"><slot /></span></template>
<style scoped>
.resource-required-label.is-required::before { content: "*"; color: var(--wt-color-danger-text); margin-right: 4px; }
</style>
