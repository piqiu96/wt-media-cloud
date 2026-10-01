<script setup>
import { computed } from 'vue'

const props = defineProps({ avatarId: { type: String, default: 'sky' }, size: { type: Number, default: 36 } })
const presets = {
  sky: { label: '蓝天', colors: ['#2d80f7', '#67caff'], icon: 'M7 18 25 8l-6 18-5-7-7-1Zm7 1 11-11' },
  ocean: { label: '海洋', colors: ['#0c8fb4', '#67ddd2'], icon: 'M3 20c5-6 9-6 14 0s9 6 14 0M3 25c5-6 9-6 14 0s9 6 14 0' },
  sun: { label: '朝阳', colors: ['#f18e44', '#ffcc76'], icon: 'M17 7v4m0 12v4M7 17h4m12 0h4M10 10l3 3m8 8 3 3m0-14-3 3m-8 8-3 3M17 12a5 5 0 1 0 0 10 5 5 0 0 0 0-10Z' },
  mountain: { label: '远山', colors: ['#666bc8', '#aeb4f6'], icon: 'm3 25 9-15 6 9 3-4 10 10H3Z' },
  flower: { label: '花朵', colors: ['#d368aa', '#f2a6cf'], icon: 'M17 9c3-5 7-2 6 2 5-1 7 4 3 7 4 3 1 8-4 7-2 5-8 5-10 0-5 2-8-3-5-7-5-4-2-9 4-9-1-5 3-8 6-4Z' },
  star: { label: '星光', colors: ['#4764c8', '#8db8ff'], icon: 'm17 5 3.5 8 8.5 1-6.5 5.5 2 8.5-7.5-4.5-7.5 4.5 2-8.5L5 14l8.5-1L17 5Z' },
}
const selected = computed(() => presets[props.avatarId] || presets.sky)
const gradient = computed(() => `linear-gradient(140deg, ${selected.value.colors[0]}, ${selected.value.colors[1]})`)
</script>

<template>
  <span class="system-avatar" :style="{ width: `${size}px`, height: `${size}px`, background: gradient }" role="img" :aria-label="`${selected.label}头像`">
    <svg viewBox="0 0 34 34" aria-hidden="true"><path :d="selected.icon" fill="none" stroke="white" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" /></svg>
  </span>
</template>

<style scoped>
.system-avatar { display: inline-flex; flex: none; align-items: center; justify-content: center; border-radius: 50%; box-shadow: inset 0 0 0 1px #ffffff66, 0 2px 7px #2b528b24; }
.system-avatar svg { width: 63%; height: 63%; }
</style>
