<script setup>
import { HelpCircleIcon } from 'tdesign-icons-vue-next'
defineProps({ content: { type: String, required: true } })
</script>
<template><t-tooltip :content="content"><span class="resource-field-help" tabindex="0" :aria-label="content"><HelpCircleIcon /></span></t-tooltip></template>
<style scoped>
.resource-field-help { display: inline-flex; align-items: center; margin-left: 4px; color: var(--wt-text-tertiary); cursor: help; vertical-align: middle; }
.resource-field-help .t-icon { width: 15px; height: 15px; }
.resource-field-help:focus-visible { outline: 2px solid var(--wt-primary); outline-offset: 2px; border-radius: 50%; }
</style>
