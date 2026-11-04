<script setup>
import { computed, onUnmounted, watch } from 'vue'
import BusinessSelect from '../../../shared/ui/fields/BusinessSelect.vue'
import { createComposeClient } from '../../../shared/api/compose.js'
import { useStrategyDirectory } from '../strategyDirectory.js'
const props = defineProps({ modelValue: [String, Number], filters: { type: Object, default: () => ({}) }, disabled: Boolean, placeholder: String, status: String })
const emit = defineEmits(['update:modelValue', 'detail'])
const directory = useStrategyDirectory(createComposeClient())
const { rows, total, loading, error, selected, hasMore } = directory
const options = computed(() => [...new Map([...(selected.value ? [selected.value] : []), ...rows.value].map(row => [row.id, {value: row.id, label: row.name}])).values()])
let timer
function search(term) { clearTimeout(timer); timer = setTimeout(() => directory.load(props.filters, term), 250) }
watch([() => JSON.stringify(props.filters), () => props.disabled], () => { clearTimeout(timer); directory.invalidate(); if (!props.disabled) directory.load(props.filters) }, { immediate: true })
watch([() => props.modelValue, () => JSON.stringify(props.filters)], async ([id]) => { emit('detail', null); const detail = await directory.resolve(id); if (detail) emit('detail', detail) }, { immediate: true })
onUnmounted(() => { clearTimeout(timer); directory.invalidate() })
</script>
<template>
 <BusinessSelect :value="modelValue ? Number(modelValue) : undefined" :options="options" :disabled="disabled" :loading="loading" :status="status" :placeholder="placeholder" remote @search="search" @change="emit('update:modelValue', $event)">
  <template #panelBottomContent><div class="strategy-picker__footer" @mousedown.prevent>
   <span v-if="error" class="strategy-picker__error">{{ error }}</span><span v-else>已显示 {{ rows.length }} / {{ total }} 条</span>
   <t-button v-if="error" size="small" variant="text" @click="directory.load(filters)">重试</t-button><t-button v-else-if="hasMore" size="small" variant="text" :loading="loading" @click="directory.more()">加载更多</t-button>
  </div></template>
 </BusinessSelect>
</template>
<style scoped>
.strategy-picker__footer { display:flex; align-items:center; justify-content:space-between; gap:8px; padding:8px 12px; border-top:1px solid var(--wt-border); color:var(--wt-text-tertiary); font-size:12px; }
.strategy-picker__error { color:var(--td-error-color); }
</style>
