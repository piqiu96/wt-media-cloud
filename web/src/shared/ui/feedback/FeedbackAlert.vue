<script setup>
import { Alert } from 'tdesign-vue-next'
defineOptions({ inheritAttrs: false })
defineProps({ message: { type: String, default: '' }, theme: { type: String, default: 'info' } })
</script>

<template>
  <Alert class="feedback-alert" :theme="theme" :message="message" v-bind="$attrs">
    <template #default><slot name="message"><template v-if="message">{{ message }}</template><slot v-else /></slot><div v-if="$slots.description" class="feedback-alert__detail"><slot name="description" /></div></template>
    <template v-if="$slots.operation || (message && $slots.default)" #operation><slot name="operation"><slot /></slot></template>
  </Alert>
</template>

<style scoped>
.feedback-alert { border: 1px solid var(--alert-border); border-radius: 6px; padding: 12px 16px; min-width: 0; flex-shrink: 0; font-size: 12px; line-height: 20px; color: var(--wt-text-secondary); background: var(--alert-bg); }
.feedback-alert.t-alert--error { --alert-border: var(--wt-color-danger-border); --alert-bg: var(--wt-danger-bg); }
.feedback-alert.t-alert--warning { --alert-border: var(--wt-color-warning-border); --alert-bg: var(--wt-warning-bg); }
.feedback-alert.t-alert--success { --alert-border: var(--wt-color-success-border); --alert-bg: var(--wt-success-bg); }
.feedback-alert.t-alert--info { --alert-border: var(--wt-color-primary-border); --alert-bg: var(--wt-info-bg); }
.feedback-alert :deep(.t-alert__icon) { font-size: 16px; line-height: 20px; flex-shrink: 0; }
.feedback-alert :deep(.t-alert__content) { min-width: 0; overflow: visible; font-size: inherit; line-height: inherit; }
.feedback-alert :deep(.t-alert__message) { flex-wrap: wrap; gap: 4px 12px; min-width: 0; }
.feedback-alert :deep(.t-alert__description) { min-width: 0; overflow-wrap: anywhere; white-space: pre-wrap; color: inherit; }
.feedback-alert :deep(.t-alert__operation) { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 8px; padding: 0; max-width: 100%; }
.feedback-alert :deep(.t-alert__operation .t-button) { height: 24px; padding: 0 4px; font-size: 12px; }
.feedback-alert :deep(.t-alert__close) { flex-shrink: 0; margin-left: 8px; }
</style>
