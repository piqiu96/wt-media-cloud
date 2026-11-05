<script setup>
const props = defineProps({ modelValue: { type: Object, required: true } })
const emit = defineEmits(['update:modelValue'])
const fields = [{key:'view_count',label:'浏览量'},{key:'like_count',label:'点赞量'},{key:'favorite_count',label:'收藏量'}]
function update(key, value) { emit('update:modelValue', { ...props.modelValue, [key]: value }) }
</script>
<template>
  <label v-for="field in fields" :key="field.key" class="material-metric-filter">
    <span class="wt-filter-field__label">{{ field.label }}</span>
    <div><t-input-number theme="column" :model-value="modelValue[`${field.key}_min`]" :min="0" placeholder="最小值" @update:model-value="update(`${field.key}_min`, $event)" /><span>至</span><t-input-number theme="column" :model-value="modelValue[`${field.key}_max`]" :min="0" placeholder="最大值" @update:model-value="update(`${field.key}_max`, $event)" /></div>
  </label>
</template>
<style scoped>
.material-metric-filter { display:grid;gap:8px;flex:0 1 290px;min-width:0; }
.material-metric-filter > div { display:flex;align-items:center;gap:8px;min-width:0; }
.material-metric-filter :deep(.t-input-number) { width:128px;min-width:0; }
.material-metric-filter > div > span { color:var(--wt-text-tertiary);font-size:12px; }
</style>
