<script setup>
defineProps({ id: { type: String, required: true }, message: { type: String, default: '' } })
</script>

<template>
  <p v-if="message" :id="id" class="form-error" role="alert">{{ message }}</p>
</template>

<style scoped>
.form-error{margin:6px 0 0;color:var(--wt-feedback-error);font-size:13px;line-height:1.4}
</style>
