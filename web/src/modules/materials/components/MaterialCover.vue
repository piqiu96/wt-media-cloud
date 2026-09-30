<script setup>
// 封面缩略图。无封面或加载失败显示占位，不算错误（CHG-20260930-069）。
import { ref, watch } from 'vue'

const props = defineProps({
  url: { type: String, default: '' },
})

const failed = ref(false)
watch(() => props.url, () => { failed.value = false })
</script>

<template>
  <span class="material-cover">
    <img v-if="url && !failed" :src="url" alt="" loading="lazy" referrerpolicy="no-referrer" @error="failed = true">
    <span v-else class="material-cover__placeholder">无封面</span>
  </span>
</template>

<style scoped>
.material-cover { display: inline-flex; width: 48px; height: 36px; flex: 0 0 auto; overflow: hidden; border-radius: var(--wt-radius-sm, 4px); background: var(--wt-bg-secondary, #f3f3f3); }
.material-cover img { width: 100%; height: 100%; object-fit: cover; }
.material-cover__placeholder { display: flex; align-items: center; justify-content: center; width: 100%; color: var(--wt-text-tertiary); font-size: 11px; }
</style>
