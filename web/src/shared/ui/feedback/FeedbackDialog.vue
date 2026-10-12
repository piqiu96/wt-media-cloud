<script setup>
defineProps({
  visible: { type: Boolean, default: false },
  title: { type: String, required: true },
  message: { type: String, required: true },
  confirmText: { type: String, required: true },
  pending: { type: Boolean, default: false },
})
const emit = defineEmits(['update:visible', 'confirm'])
</script>

<template>
  <t-dialog
    :visible="visible"
    theme="warning"
    :header="title"
    :close-on-overlay-click="false"
    :close-on-esc-keydown="!pending"
    :close-btn="!pending"
    :confirm-btn="{ content: confirmText, loading: pending }"
    :cancel-btn="{ content: '取消', disabled: pending }"
    @confirm="emit('confirm')"
    @cancel="emit('update:visible', false)"
    @close="emit('update:visible', false)"
  >
    <p class="feedback-dialog-message">{{ message }}</p>
    <slot />
  </t-dialog>
</template>

<style scoped>
.feedback-dialog-message{margin:0;line-height:1.6}
</style>
