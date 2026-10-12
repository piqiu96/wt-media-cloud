<script setup>
defineProps({ message: { type: String, default: '' }, theme: { type: String, default: 'info' } })
</script>

<template>
  <t-alert class="feedback-alert" :theme="theme" :message="message" v-bind="$attrs">
    <template v-if="$slots.default" #default><slot /></template>
    <template v-if="$slots.description" #description><slot name="description" /></template>
    <template v-if="$slots.message" #message><slot name="message" /></template>
    <template v-if="$slots.operation" #operation><slot name="operation" /></template>
  </t-alert>
</template>

<style scoped>
.feedback-alert{border:1px solid transparent;border-radius:8px}
.feedback-alert.t-alert--error{--td-error-color:var(--wt-feedback-error);border-color:var(--wt-feedback-error)}
.feedback-alert.t-alert--warning{--td-warning-color:var(--wt-feedback-warning);border-color:var(--wt-feedback-warning)}
.feedback-alert.t-alert--success{--td-success-color:var(--wt-feedback-success);border-color:var(--wt-feedback-success)}
.feedback-alert.t-alert--info{--td-brand-color:var(--wt-feedback-info);border-color:var(--wt-feedback-info)}
</style>
