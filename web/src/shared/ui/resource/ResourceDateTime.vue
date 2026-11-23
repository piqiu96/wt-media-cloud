<script setup>
import { computed } from 'vue';
import { DatePicker } from 'tdesign-vue-next';
defineOptions({ inheritAttrs: false });
const props = defineProps({ value: { type: String, default: '' }, seconds: Boolean });
const emit = defineEmits(['update:value', 'change']);
// Keep the existing local ISO minute value at the component boundary.
const pickerFormat = computed(() => props.seconds ? 'YYYY-MM-DD HH:mm:ss' : 'YYYY-MM-DD HH:mm');
const pickerValue = computed(() => (props.value || '').replace('T', ' ').slice(0, props.seconds ? 19 : 16));
function change(value) {
  const next = value ? value.replace(' ', 'T') : '';
  emit('update:value', next);
  emit('change', next);
}
</script>
<template>
  <DatePicker class="resource-date-time" :value="pickerValue" enable-time-picker allow-input :value-type="pickerFormat" :format="pickerFormat" :time-picker-props="{ format: seconds ? 'HH:mm:ss' : 'HH:mm' }" :popup-props="{ attach: 'body', zIndex: 5500, overlayClassName: 'resource-date-time-popup' }" placeholder="请选择日期和时间" clearable v-bind="$attrs" @change="change" />
</template>
<style scoped>
.resource-date-time { width: 100%; min-width: 0; }
.resource-date-time :deep(.t-input) { height: 36px; }
</style>

<style>
@media (max-width: 680px) {
  .resource-date-time-popup .t-date-picker__panel { width: calc(100vw - 32px); max-height: calc(100dvh - 80px); overflow: auto; }
  .resource-date-time-popup .t-date-picker__panel-content { flex-direction: column; }
  .resource-date-time-popup .t-date-picker__panel-date { min-width: 0; width: 100%; }
  .resource-date-time-popup .t-date-picker__panel-time { border-left: 0; border-top: 1px solid var(--wt-color-border); align-items: center; }
  .resource-date-time-popup .t-date-picker__panel-time .t-time-picker__panel-body { height: 120px; }
}
</style>
