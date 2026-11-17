<script setup>
import ResourceDateTime from '../../../src/shared/ui/resource/ResourceDateTime.vue';
import ResourceFieldHelp from '../../../src/shared/ui/resource/ResourceFieldHelp.vue';
import { publishMethodLabel } from './publishingSettings.js';
defineProps({ row: { type: Object, required: true }, number: { type: String, default: '' } });
const emit = defineEmits(['change']);
</script>
<template>
  <section class="publishing-execution-settings resource-editor-section" aria-label="发布设置">
    <h3><span v-if="number" class="publishing-settings-number">{{ number }}</span>发布设置</h3>
    <div class="publishing-settings-grid">
      <label class="publishing-settings-field"><span class="publishing-settings-label">发布方式</span><t-input :value="publishMethodLabel" disabled /></label>
      <label class="publishing-settings-field" data-field="schedule"><span class="publishing-settings-label">发布时间<ResourceFieldHelp label="发布时间" content="留空表示启动后尽快发布；指定时间为系统开始执行时间。" /></span><ResourceDateTime :value="row.schedule || ''" aria-label="发布时间" @change="value => emit('change', 'schedule', value)" /></label>
    </div>
  </section>
</template>
<style scoped>
.publishing-execution-settings { margin-top: var(--wt-space-block, 16px); }
.publishing-execution-settings h3 { display: flex; align-items: center; gap: 10px; margin: 0 0 16px; font-size: var(--wt-font-size-section, 16px); line-height: 24px; font-weight: 500; }
.publishing-settings-number { display: grid; place-items: center; width: 24px; height: 24px; border-radius: 4px; font-size: 12px; color: var(--wt-color-primary); background: var(--wt-color-primary-light); }
.publishing-settings-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 20px 24px; }
.publishing-settings-field { display: flex; flex-direction: column; gap: 8px; min-width: 0; }
.publishing-settings-label { display: flex; align-items: center; gap: 6px; font-size: 13px; line-height: 20px; color: var(--wt-color-text-primary); font-weight: 500; }
@media (max-width: 680px) { .publishing-settings-grid { grid-template-columns: 1fr; gap: 16px; } }
</style>
