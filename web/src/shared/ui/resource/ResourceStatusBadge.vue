<script setup>
defineProps({
  tone: { type: String, default: 'neutral' },
  label: { type: String, required: true },
})
</script>

<template>
  <span class="resource-status-badge" :class="`is-${tone}`">{{ label }}</span>
</template>

<style scoped>
.resource-status-badge { display: inline-flex; align-items: center; min-height: 20px; padding: 2px 10px; border-radius: 12px; font-size: 12px; line-height: 16px; white-space: nowrap; }
.is-success { color: var(--wt-success); background: var(--wt-success-bg); }
.is-warning { color: var(--wt-warning); background: var(--wt-warning-bg); }
.is-danger { color: var(--wt-danger); background: var(--wt-danger-bg); }
.is-info { color: var(--wt-primary); background: var(--wt-info-bg); }
.is-neutral { color: var(--wt-text-tertiary); background: var(--wt-bg-page); }
</style>
