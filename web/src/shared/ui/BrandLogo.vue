<script setup>
defineProps({ compact: { type: Boolean, default: false } })
</script>

<template>
  <span class="brand-logo" :class="{ compact }">
    <img class="brand-logo__mark" src="/brand-mark.svg" alt="" />
    <span v-if="!compact" class="brand-logo__copy">
      <strong>起飞</strong><small>内容运营平台</small>
    </span>
  </span>
</template>

<style scoped>
.brand-logo { display: inline-flex; align-items: center; gap: 11px; color: #11294c; white-space: nowrap; }
.brand-logo__mark { display: block; width: 42px; height: 42px; flex: none; }
.brand-logo__copy { display: flex; flex-direction: column; align-items: flex-start; line-height: 1.1; }
.brand-logo__copy strong { font-size: 25px; font-weight: 800; letter-spacing: .03em; }
.brand-logo__copy small { margin-top: 3px; color: #7b90ad; font-size: 11px; letter-spacing: .12em; }
.brand-logo.compact .brand-logo__mark { width: 35px; height: 35px; }
</style>
