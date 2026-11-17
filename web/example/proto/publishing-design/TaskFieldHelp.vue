<script setup>
import ResourceFieldHelp from '../../../src/shared/ui/resource/ResourceFieldHelp.vue';
defineProps({ label: { type: String, required: true }, content: { type: String, required: true } });
</script>
<template><ResourceFieldHelp :label="label" :content="content" /></template>
