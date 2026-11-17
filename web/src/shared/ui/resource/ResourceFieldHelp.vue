<script setup>
import { ref } from 'vue';
import { HelpCircleIcon } from 'tdesign-icons-vue-next';
defineProps({ label: { type: String, required: true }, content: { type: String, required: true } });
const visible = ref(false);
</script>

<template>
  <t-tooltip v-model:visible="visible" :content="content" placement="top-left" trigger="hover" :overlay-inner-style="{ maxWidth: '320px', fontSize: '12px', fontWeight: 400, lineHeight: '20px' }">
    <button type="button" class="resource-field-help" :aria-label="`${label}说明`" :aria-expanded="visible" @focus="visible = true" @blur="visible = false" @click.prevent="visible = true" @keydown.esc="visible = false">
      <HelpCircleIcon aria-hidden="true" />
    </button>
  </t-tooltip>
</template>

<style scoped>
.resource-field-help { display: inline-flex; align-items: center; justify-content: center; flex: 0 0 16px; width: 16px; height: 16px; padding: 0; border: 0; border-radius: 3px; background: transparent; color: var(--wt-text-tertiary); cursor: pointer; vertical-align: middle; }
.resource-field-help .t-icon { width: 14px; height: 14px; }
.resource-field-help:hover, .resource-field-help:focus-visible { color: var(--wt-primary); }
.resource-field-help:focus-visible { outline: 2px solid var(--wt-primary); outline-offset: 2px; }
</style>
