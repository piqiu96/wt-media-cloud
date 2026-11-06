<script setup>
import { computed, useSlots, ref, watch, nextTick, onUnmounted } from 'vue'
import { overlayStack } from './overlayStack.js'
import { nextOverlayFocus } from './overlayFocus.js'

const props = defineProps({
  visible: Boolean,
  title: { type: String, required: true },
  kind: { type: String, default: 'dialog', validator: value => ['dialog', 'drawer'].includes(value) },
  width: { type: Number, default: 0 },
  busy: Boolean,
  escClosable: { type: Boolean, default: true },
  footer: { type: Boolean, default: true },
})
const emit = defineEmits(['update:visible', 'close'])
const slots = useSlots()
const body = ref(null), identity = Symbol('resource-overlay')
let returnFocus = null
const isTop = computed(() => overlayStack.isTop(identity))
const zIndex = computed(() => 2000 + Math.max(0, overlayStack.items.value.indexOf(identity)) * 20)
const focusRoot = () => body.value?.closest('.t-drawer__content-wrapper, .t-dialog')
function higherModal() {
 if (typeof document === 'undefined') return false
 return [...document.querySelectorAll('.t-dialog__ctx, .t-drawer')].some(node => node.getClientRects().length && Number(getComputedStyle(node).zIndex) > zIndex.value)
}
function focusInside() {
 const root=focusRoot()
 if(!root)return
 root.tabIndex=-1
 nextOverlayFocus(root, null, false)?.focus?.({preventScroll:true})
}
function trapTab(event) {
 if(!props.visible||!isTop.value||higherModal())return
 if(event.key==='Escape'){event.preventDefault();event.stopImmediatePropagation();if(!props.busy&&props.escClosable)close();return}
 if(event.key!=='Tab')return
 // TDesign dropdowns are teleported; let their own keyboard controller finish.
 if(document.activeElement?.closest('.t-popup'))return
 event.preventDefault()
 nextOverlayFocus(focusRoot(),document.activeElement,event.shiftKey)?.focus?.({preventScroll:true})
}
function keepFocus(event) {
 if(!props.visible||!isTop.value||higherModal()||!focusRoot())return
 if(focusRoot().contains(event.target)||event.target?.closest('.t-popup'))return
 focusInside()
}
async function removeAndRestore() {
 const wasTop=overlayStack.isTop(identity)
 overlayStack.close(identity)
 await nextTick()
 if(wasTop&&returnFocus?.isConnected)returnFocus.focus?.({preventScroll:true})
}
watch(() => props.visible, async visible => {
 if(visible){
  returnFocus=typeof document!=='undefined'?document.activeElement:null
  overlayStack.open(identity)
  await nextTick()
  if(isTop.value&&!higherModal())focusInside()
 }else removeAndRestore()
},{immediate:true})
if(typeof document!=='undefined'){
 document.addEventListener('keydown',trapTab,true)
 document.addEventListener('focusin',keepFocus)
}
onUnmounted(()=>{
 if(typeof document!=='undefined'){
  document.removeEventListener('keydown',trapTab,true)
  document.removeEventListener('focusin',keepFocus)
 }
 removeAndRestore()
})
const size = computed(() => `min(calc(100vw - 32px), ${props.width || (props.kind === 'dialog' ? 560 : 800)}px)`)
const showFooter = computed(() => props.footer && Boolean(slots.footer))
function close() {
  if (props.busy) return
  emit('update:visible', false)
  emit('close')
}
</script>

<template>
  <t-dialog class="resource-overlay-context" v-if="kind === 'dialog'" :visible="visible" :header="title" :width="size" placement="center"
    dialog-class-name="resource-overlay-dialog" :close-btn="!busy" :close-on-esc-keydown="false" :show-overlay="isTop" :z-index="zIndex"
    :close-on-overlay-click="false" :footer="showFooter" destroy-on-close @close="close">
    <div ref="body" class="resource-overlay__body"><slot /></div>
    <template v-if="showFooter" #footer><div class="resource-overlay__footer"><slot name="footer" /></div></template>
  </t-dialog>
  <t-drawer v-else :visible="visible" :header="title" :size="size" class="resource-overlay-drawer"
    :close-btn="!busy" :close-on-esc-keydown="false" :show-overlay="isTop" :z-index="zIndex" :close-on-overlay-click="false"
    :footer="showFooter" destroy-on-close @close="close">
    <div ref="body" class="resource-overlay__body"><slot /></div>
    <template v-if="showFooter" #footer><div class="resource-overlay__footer"><slot name="footer" /></div></template>
  </t-drawer>
</template>

<style scoped>
.resource-overlay__body { min-width: 0; color: var(--wt-text-secondary); }
.resource-overlay__footer { display: flex; justify-content: flex-end; align-items: center; flex-wrap: wrap; gap: 8px; }
.resource-overlay__body :deep(.t-descriptions__label) { color: var(--wt-text-tertiary); }
.resource-overlay__body :deep(.t-descriptions__content) { overflow-wrap: anywhere; }
:global(.resource-overlay-context .t-dialog__position) { padding: 16px; height: 100%; }
:global(.resource-overlay-dialog) { padding: 0; max-height: calc(100dvh - 32px); display: flex; flex-direction: column; border-radius: 8px; box-shadow: var(--wt-shadow-popup); }
:global(.resource-overlay-dialog .t-dialog__header) { flex-shrink: 0; margin: 0; height: 64px; min-height: 64px; box-sizing: border-box; padding: 0 24px; display: flex; align-items: center; border-bottom: 1px solid var(--wt-border); font-size: 18px; font-weight: 600; }
:global(.resource-overlay-dialog .t-dialog__body) { padding: 24px; overflow: auto; min-height: 0; }
:global(.resource-overlay-dialog .t-dialog__footer) { flex-shrink: 0; margin: 0; padding: 16px 24px; box-sizing: border-box; width: 100%; border-top: 1px solid var(--wt-border); }
:global(.resource-overlay-dialog .t-dialog__close) { top: 19px; right: 20px; }
:global(.resource-overlay-drawer .t-drawer__header) { height: 64px; min-height: 64px; box-sizing: border-box; padding: 0 24px; display: flex; align-items: center; font-size: 18px; font-weight: 600; border-bottom: 1px solid var(--wt-border); }
:global(.resource-overlay-drawer .t-drawer__body) { padding: 24px; }
:global(.resource-overlay-drawer .t-drawer__footer) { padding: 16px 24px; box-sizing: border-box; }
@media (max-width: 600px) {
  :global(.resource-overlay-dialog .t-dialog__header), :global(.resource-overlay-drawer .t-drawer__header) { padding: 16px; }
  :global(.resource-overlay-dialog .t-dialog__body), :global(.resource-overlay-drawer .t-drawer__body) { padding: 20px 16px; }
  :global(.resource-overlay-dialog .t-dialog__footer), :global(.resource-overlay-drawer .t-drawer__footer) { padding: 12px 16px; }
}
</style>
