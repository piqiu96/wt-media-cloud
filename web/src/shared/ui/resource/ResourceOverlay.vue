<script setup>
import { computed, useSlots } from 'vue'

const props = defineProps({
  visible: Boolean,
  title: { type: String, required: true },
  kind: { type: String, default: 'dialog', validator: value => ['dialog', 'drawer'].includes(value) },
  width: { type: Number, default: 0 },
  busy: Boolean,
  footer: { type: Boolean, default: true },
})
const emit = defineEmits(['update:visible', 'close'])
const slots = useSlots()
const size = computed(() => `min(calc(100vw - 32px), ${props.width || (props.kind === 'dialog' ? 640 : 800)}px)`)
const showFooter = computed(() => props.footer && Boolean(slots.footer))
function close() {
  if (props.busy) return
  emit('update:visible', false)
  emit('close')
}
</script>

<template>
  <t-dialog v-if="kind === 'dialog'" :visible="visible" :header="title" :width="size" placement="center"
    dialog-class-name="resource-overlay-dialog" :close-btn="!busy" :close-on-esc-keydown="!busy"
    :close-on-overlay-click="false" :footer="showFooter" destroy-on-close @close="close">
    <div class="resource-overlay__body"><slot /></div>
    <template v-if="showFooter" #footer><div class="resource-overlay__footer"><slot name="footer" /></div></template>
  </t-dialog>
  <t-drawer v-else :visible="visible" :header="title" :size="size" class="resource-overlay-drawer"
    :close-btn="!busy" :close-on-esc-keydown="false" :close-on-overlay-click="false"
    :footer="showFooter" destroy-on-close @close="close">
    <div class="resource-overlay__body"><slot /></div>
    <template v-if="showFooter" #footer><div class="resource-overlay__footer"><slot name="footer" /></div></template>
  </t-drawer>
</template>

<style scoped>
.resource-overlay__body { min-width: 0; color: var(--wt-text-secondary); }
.resource-overlay__footer { display: flex; justify-content: flex-end; align-items: center; flex-wrap: wrap; gap: 8px; }
.resource-overlay__body :deep(.t-descriptions__label) { color: var(--wt-text-tertiary); }
.resource-overlay__body :deep(.t-descriptions__content) { overflow-wrap: anywhere; }
:global(.resource-overlay-dialog) { padding: 0; max-height: calc(100dvh - 32px); display: flex; flex-direction: column; }
:global(.resource-overlay-dialog .t-dialog__header) { flex-shrink: 0; margin: 0; padding: 20px 24px; border-bottom: 1px solid var(--wt-border); font-size: 16px; font-weight: 600; }
:global(.resource-overlay-dialog .t-dialog__body) { padding: 24px; overflow: auto; min-height: 0; }
:global(.resource-overlay-dialog .t-dialog__footer) { flex-shrink: 0; margin: 0; padding: 16px 24px; box-sizing: border-box; width: 100%; border-top: 1px solid var(--wt-border); }
:global(.resource-overlay-dialog .t-dialog__close) { top: 19px; right: 20px; }
:global(.resource-overlay-drawer .t-drawer__header) { padding: 20px 24px; }
:global(.resource-overlay-drawer .t-drawer__body) { padding: 24px; }
:global(.resource-overlay-drawer .t-drawer__footer) { padding: 16px 24px; box-sizing: border-box; }
@media (max-width: 600px) {
  :global(.resource-overlay-dialog .t-dialog__header), :global(.resource-overlay-drawer .t-drawer__header) { padding: 16px; }
  :global(.resource-overlay-dialog .t-dialog__body), :global(.resource-overlay-drawer .t-drawer__body) { padding: 20px 16px; }
  :global(.resource-overlay-dialog .t-dialog__footer), :global(.resource-overlay-drawer .t-drawer__footer) { padding: 12px 16px; }
}
</style>
