<script setup>
defineProps({ modelValue: { type: [String, Number], default: '' }, options: { type: Array, required: true }, disabled: Boolean })
const emit = defineEmits(['update:modelValue'])
</script>
<template>
  <t-radio-group class="resource-choice-cards" :value="modelValue" :disabled="disabled" @change="value => emit('update:modelValue', value)">
    <t-radio v-for="option in options" :key="option.value" :value="option.value" :disabled="option.disabled" class="resource-choice-card" :class="{ 'is-selected': modelValue === option.value }">
      <span class="resource-choice-card__label">{{ option.label }}</span>
      <span v-if="option.description" class="resource-choice-card__description">{{ option.description }}</span>
    </t-radio>
  </t-radio-group>
</template>
<style scoped>
.resource-choice-cards { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; width: 100%; }
.resource-choice-card { box-sizing: border-box; display: flex; align-items: flex-start; height: auto; min-height: 76px; margin: 0; padding: 14px; border: 1px solid var(--wt-color-border-control); border-radius: 6px; background: var(--wt-bg-card); transition: border-color .15s, background .15s; }
.resource-choice-card:hover:not(.t-is-disabled) { border-color: var(--wt-primary); }
.resource-choice-card.is-selected { border-color: var(--wt-primary); background: var(--wt-info-bg); }
.resource-choice-card :deep(.t-radio__input) { margin-top: 3px; flex-shrink: 0; }
.resource-choice-card :deep(.t-radio__label) { min-width: 0; white-space: normal; padding-left: 8px; }
.resource-choice-card__label { display: block; color: var(--wt-text-primary); font-size: 14px; line-height: 22px; font-weight: 500; }
.resource-choice-card__description { display: block; margin-top: 4px; color: var(--wt-text-tertiary); font-size: 12px; line-height: 20px; }
.resource-choice-card.t-is-disabled { opacity: .55; }
@media (max-width: 440px) { .resource-choice-cards { grid-template-columns: 1fr; } }
</style>
