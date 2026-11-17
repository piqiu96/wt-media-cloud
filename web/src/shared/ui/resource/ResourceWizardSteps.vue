<script setup>
defineProps({
  current: { type: Number, default: 0 },
  steps: { type: Array, required: true },
});
</script>
<template>
  <div class="resource-wizard-steps" aria-label="配置步骤">
    <t-steps class="wizard-steps-wide" :current="current" readonly
      ><t-step-item v-for="step in steps" :key="step.title" :title="step.title" :content="step.description || undefined"
    /></t-steps>
    <div class="wizard-steps-compact">
      <div>
        <span>步骤 {{ current + 1 }} / {{ steps.length }}</span
        ><strong>{{ steps[current]?.title }}</strong
        ><small v-if="steps[current + 1]"
          >下一步：{{ steps[current + 1].title }}</small
        >
      </div>
      <div
        class="wizard-progress"
        role="progressbar"
        aria-label="创建进度"
        :aria-valuemin="0"
        :aria-valuemax="steps.length"
        :aria-valuenow="current + 1"
      >
        <span :style="{ width: `${((current + 1) / steps.length) * 100}%` }" />
      </div>
    </div>
  </div>
</template>
<style scoped>
.wizard-steps-wide :deep(.t-steps-item__title) { font-size: 14px; font-weight: 500; }
.wizard-steps-wide :deep(.t-steps-item__description) { font-size: 12px; line-height: 20px; color: var(--wt-text-tertiary); }
.wizard-steps-compact {
  display: none;
}
.wizard-steps-compact > div:first-child {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px 12px;
}
.wizard-steps-compact span {
  color: var(--wt-primary);
  font-size: 12px;
}
.wizard-steps-compact strong {
  font-size: 14px;
  color: var(--wt-text-primary);
  font-weight: 600;
}
.wizard-steps-compact small {
  flex-basis: 100%;
  font-size: 12px;
  color: var(--wt-text-tertiary);
}
.wizard-progress {
  height: 4px;
  background: var(--wt-bg-page);
  border-radius: 3px;
  overflow: hidden;
  margin-top: 14px;
}
.wizard-progress > span {
  display: block;
  height: 100%;
  background: var(--wt-primary);
  border-radius: 3px;
}
@media (max-width: 700px) {
  .wizard-steps-wide {
    display: none;
  }
  .wizard-steps-compact {
    display: block;
  }
}
</style>
