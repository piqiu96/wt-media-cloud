<script setup>
import ResourceRequiredLabel from '../../../src/shared/ui/resource/ResourceRequiredLabel.vue';
defineProps({ required: { type: Boolean, default: true } });
</script>
<template><ResourceRequiredLabel class="publishing-field-label" :required="required"><slot /></ResourceRequiredLabel></template>
