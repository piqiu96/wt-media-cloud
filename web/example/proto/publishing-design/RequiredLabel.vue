<script setup>
defineProps({ required: { type: Boolean, default: true } });
</script>
<template>
  <span class="publishing-field-label" :class="{ 'is-required': required }"
    ><slot
  /></span>
</template>
<style scoped>
.publishing-field-label {
  display: inline;
}
.publishing-field-label.is-required::before {
  content: "*";
  color: var(--wt-color-danger-text);
  margin-right: 4px;
}
</style>
