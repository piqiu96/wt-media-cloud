<script setup>
defineProps({
  items: { type: Array, required: true },
})

const emit = defineEmits(['select'])
</script>

<template>
  <div class="resource-stat-grid">
    <button v-for="item in items" :key="item.key" type="button" class="resource-stat-card" :class="`is-${item.tone || 'info'}`" @click="emit('select', item.key)">
      <span>{{ item.label }}</span>
      <strong>{{ item.value }}</strong>
    </button>
  </div>
</template>

<style scoped>
.resource-stat-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 12px; margin-bottom: 16px; }
.resource-stat-card { min-height: 88px; padding: 16px; border: 1px solid var(--wt-border); border-radius: var(--wt-radius-md); background: var(--wt-bg-card); box-shadow: var(--wt-shadow-card); text-align: left; cursor: pointer; transition: border-color 0.2s ease, transform 0.2s ease; }
.resource-stat-card:hover { border-color: rgba(37, 99, 235, 0.24); transform: translateY(-1px); }
.resource-stat-card span { display: block; color: var(--wt-text-tertiary); font-size: 13px; }
.resource-stat-card strong { display: block; margin-top: 7px; color: var(--wt-text-primary); font-size: 24px; font-weight: 600; line-height: 1; }
.resource-stat-card.is-success strong { color: var(--wt-success); }
.resource-stat-card.is-warning strong { color: var(--wt-warning); }
.resource-stat-card.is-danger strong { color: var(--wt-danger); }
</style>
