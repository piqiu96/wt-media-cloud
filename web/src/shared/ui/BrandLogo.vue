<script setup>
import { computed } from 'vue'
import { desktopEnvironment, desktopBrand } from './desktopBrand.js'
defineProps({ compact: { type: Boolean, default: false } })
const brand = computed(() => desktopBrand(desktopEnvironment.value))
</script>

<template>
  <span class="brand-logo" :class="{ compact }" :aria-label="brand.name">
    <span class="brand-logo__mark" aria-hidden="true"><img src="/images/brand/logo.png" alt="" /><b v-if="brand.badge" class="brand-logo__badge">{{ brand.badge }}</b></span>
    <span v-if="!compact" class="brand-logo__copy">
      <strong>{{ brand.name }}</strong><small>内容运营平台</small>
    </span>
  </span>
</template>

<style scoped>
.brand-logo { display: inline-flex; align-items: center; gap: 11px; color: #11294c; white-space: nowrap; }
.brand-logo__mark { position: relative; display: block; width: 42px; height: 42px; flex: none; overflow: hidden; background: #fff; border-radius: 10px; mix-blend-mode: multiply; }
.brand-logo__mark img { position: absolute; top: -27%; left: -25%; display: block; width: 150%; max-width: none; height: 150%; object-fit: cover; }
.brand-logo__badge { position: absolute; z-index: 1; right: 0; bottom: 0; padding: 2px 3px; border-radius: 3px; background: #922b09; color: #fff; font: 800 10px/1.2 sans-serif; }
.brand-logo__copy { display: flex; flex-direction: column; align-items: flex-start; line-height: 1.1; }
.brand-logo__copy strong { font-size: 25px; font-weight: 800; letter-spacing: .03em; }
.brand-logo__copy small { margin-top: 3px; color: #7b90ad; font-size: 11px; letter-spacing: .12em; }
.brand-logo.compact .brand-logo__mark { width: 35px; height: 35px; }
</style>
