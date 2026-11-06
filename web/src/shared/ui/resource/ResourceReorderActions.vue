<script setup>
const REORDER_MIME = 'application/x-wt-resource-reorder'
const props = defineProps({ itemKey: { type: String, required: true }, scope: { type: String, required: true }, index: { type: Number, required: true }, count: { type: Number, required: true }, removeDisabled: Boolean, draggable: { type: Boolean, default: true } })
const emit = defineEmits(['move', 'remove', 'dragend'])
function startDrag(event) {
  event.dataTransfer.effectAllowed = 'move'
  event.dataTransfer.setData(REORDER_MIME, JSON.stringify({ scope: props.scope, key: props.itemKey }))
}
</script>
<template>
  <div class="resource-reorder">
    <div class="resource-reorder__heading"><strong><slot /></strong></div>
    <div class="resource-reorder__actions">
      <t-tooltip v-if="draggable" content="拖动排序，也可使用上移、下移"><span class="resource-reorder__handle" draggable="true" role="img" aria-label="拖动排序" @dragstart="startDrag" @dragend="emit('dragend')">⠿<span>拖动排序</span></span></t-tooltip>
      <t-button size="small" variant="text" :disabled="index === 0" @click="emit('move', -1)">上移</t-button>
      <t-button size="small" variant="text" :disabled="index === count - 1" @click="emit('move', 1)">下移</t-button>
      <slot name="extra" />
      <t-button size="small" theme="danger" variant="text" :disabled="removeDisabled" @click="emit('remove')">移除</t-button>
    </div>
  </div>
</template>
<style scoped>
.resource-reorder { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-bottom: 16px; }
.resource-reorder__heading { display: flex; align-items: center; gap: 8px; min-width: 0; }
.resource-reorder__handle { display: inline-flex; align-items: center; justify-content: center; gap:4px; height: 28px; color: var(--wt-primary); cursor: grab; font-size: 22px; user-select: none; }
.resource-reorder__handle > span { font-size:13px; }
.resource-reorder__actions { display:flex;align-items:center;gap:8px;flex-wrap:wrap;justify-content:flex-end; }
.resource-reorder__handle:active { cursor: grabbing; }
</style>
