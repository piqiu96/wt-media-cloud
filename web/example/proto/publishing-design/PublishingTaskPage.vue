<script setup>
import ResourceDetailPage from '../../../src/shared/ui/resource/ResourceDetailPage.vue';
defineProps({ title: { type: String, required: true }, busy: Boolean });
defineEmits(['back']);
</script>
<template>
  <ResourceDetailPage class="publishing-task-page" :title="title" :busy="busy" @back="$emit('back')">
    <slot /><template v-if="$slots.footer" #footer><slot name="footer" /></template>
  </ResourceDetailPage>
</template>
