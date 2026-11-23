<script setup>
import { computed, useSlots, ref, watch, nextTick, onUnmounted } from 'vue'
import { CloseIcon } from 'tdesign-icons-vue-next'
import { overlayStack } from './overlayStack.js'
import { nextOverlayFocus } from './overlayFocus.js'
import { setOverlayScrollLock } from './overlayScrollLock.js'

const props = defineProps({
  visible: Boolean,
  formActions: Boolean,
  title: { type: String, required: true },
  kind: { type: String, default: 'dialog', validator: value => ['dialog', 'drawer'].includes(value) },
  width: { type: Number, default: 0 },
  fullWidth: Boolean,
  attach: { type: String, default: 'body' },
  showInAttachedElement: Boolean,
  busy: Boolean,
  escClosable: { type: Boolean, default: true },
  footer: { type: Boolean, default: true },
})
const emit = defineEmits(['update:visible', 'close'])
const slots = useSlots()
const body = ref(null), identity = Symbol('resource-overlay')
let returnFocus = null, focusTimer, focusFrame, focusEpoch = 0, disposed = false
const isTop = computed(() => overlayStack.isTop(identity))
const zIndex = computed(() => 2000 + Math.max(0, overlayStack.items.value.indexOf(identity)) * 20)
const focusRoot = () => body.value?.closest('.t-drawer__content-wrapper, .t-dialog')
function higherModal() {
 if (typeof document === 'undefined') return false
 const nativeModal=document.querySelector('dialog:modal')
 if(nativeModal&&!focusRoot()?.contains(nativeModal))return true
 return [...document.querySelectorAll('.t-dialog__ctx, .t-drawer')].some(node => node.getClientRects().length && Number(getComputedStyle(node).zIndex) > zIndex.value)
}
function focusInside() {
 const root=focusRoot()
 if(!root)return
 root.setAttribute('role', 'dialog')
 root.setAttribute('aria-modal', 'true')
 root.setAttribute('aria-label', props.title)
 root.tabIndex=-1
 nextOverlayFocus(root, null, false)?.focus?.({preventScroll:true})
}
function trapTab(event) {
 if(!props.visible||!isTop.value||higherModal())return
 // Let an open TDesign dropdown consume Escape before closing its parent.
 if(event.key==='Escape'&&(focusRoot()?.querySelector('.t-select-input--popup-visible')||[...document.querySelectorAll('.t-popup')].some(node=>node.getClientRects().length&&getComputedStyle(node).visibility!=='hidden'&&getComputedStyle(node).display!=='none')))return
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
function cancelFocus() {
 clearTimeout(focusTimer)
 if(typeof cancelAnimationFrame==='function')cancelAnimationFrame(focusFrame)
 focusEpoch++
}
async function scheduleFocus(callback) {
 const epoch=focusEpoch
 await nextTick()
 if(epoch!==focusEpoch)return
 // TDesign shows drawers in a timer and focuses its closing sentinel in
 // nextTick. Run after both so the visible top overlay owns the focus.
 focusTimer=setTimeout(async()=>{
  await nextTick()
  if(epoch===focusEpoch)callback()
 },0)
}
function focusWhenVisible(epoch) {
 if(typeof document==='undefined'||typeof requestAnimationFrame!=='function'||epoch!==focusEpoch||!props.visible||!isTop.value||higherModal())return
 const root=focusRoot()
 if(!root||getComputedStyle(root).visibility!=='visible'){
  focusFrame=requestAnimationFrame(()=>focusWhenVisible(epoch))
  return
 }
 focusInside()
}
function removeAndRestore() {
 const wasTop=overlayStack.isTop(identity),opener=returnFocus
 overlayStack.close(identity)
 const parent=overlayStack.items.value.at(-1)
 if(wasTop)scheduleFocus(()=>{
  if((disposed||!props.visible)&&overlayStack.items.value.at(-1)===parent&&opener?.isConnected)
   opener.focus?.({preventScroll:true})
 })
}
watch(() => props.visible, visible => {
 cancelFocus()
 setOverlayScrollLock(identity,visible&&props.kind==='drawer'&&props.showInAttachedElement)
 if(visible){
  returnFocus=typeof document!=='undefined'?document.activeElement:null
  overlayStack.open(identity)
  scheduleFocus(()=>focusWhenVisible(focusEpoch))
 }else removeAndRestore()
},{immediate:true})
if(typeof document!=='undefined'){
 document.addEventListener('keydown',trapTab,true)
 document.addEventListener('focusin',keepFocus)
}
onUnmounted(()=>{
 disposed=true
 cancelFocus()
 setOverlayScrollLock(identity,false)
 if(typeof document!=='undefined'){
  document.removeEventListener('keydown',trapTab,true)
  document.removeEventListener('focusin',keepFocus)
 }
 removeAndRestore()
})
const size = computed(() => props.kind === 'drawer' && props.fullWidth ? '100%' : `min(calc(100vw - 32px), ${props.width || (props.kind === 'dialog' ? 560 : 800)}px)`)
const showFooter = computed(() => props.footer && Boolean(slots.footer))
function close() {
  if (props.busy) return
  emit('update:visible', false)
  emit('close')
}
</script>

<template>
  <t-dialog :class="{ 'resource-form-actions-scope': formActions }" class="resource-overlay-context" v-if="kind === 'dialog'" :visible="visible" :header="title" :width="size" placement="center"
    dialog-class-name="resource-overlay-dialog" :close-btn="!busy" :close-on-esc-keydown="false" :show-overlay="isTop" :z-index="zIndex"
    :close-on-overlay-click="false" :footer="showFooter" destroy-on-close @close="close">
    <template #closeBtn><button type="button" class="resource-overlay__close" :aria-label="`关闭${title}`"><CloseIcon aria-hidden="true" /></button></template>
    <div ref="body" class="resource-overlay__body"><slot /></div>
    <template v-if="showFooter" #footer><div class="resource-overlay__footer"><slot name="footer" /></div></template>
  </t-dialog>
  <t-drawer v-else :visible="visible" :header="title" :size="size" :attach="attach" :show-in-attached-element="showInAttachedElement" class="resource-overlay-drawer" :class="{ 'resource-overlay-drawer--full-width': fullWidth, 'resource-form-actions-scope': formActions }"
    :close-btn="!busy" :close-on-esc-keydown="false" :show-overlay="isTop" :z-index="zIndex" :close-on-overlay-click="false"
    :footer="showFooter" destroy-on-close @close="close">
    <template #closeBtn><button type="button" class="resource-overlay__close" :aria-label="`关闭${title}`"><CloseIcon aria-hidden="true" /></button></template>
    <div ref="body" class="resource-overlay__body"><slot /></div>
    <template v-if="showFooter" #footer><div class="resource-overlay__footer"><slot name="footer" /></div></template>
  </t-drawer>
</template>

<style scoped>
.resource-overlay__close { display:inline-flex; align-items:center; justify-content:center; width:24px; height:24px; padding:0; border:0; border-radius:4px; background:transparent; color:var(--wt-text-secondary); cursor:pointer; }
.resource-overlay__close:focus-visible { outline:2px solid var(--wt-primary); outline-offset:2px; }
.resource-overlay__body { min-width: 0; color: var(--wt-text-secondary); font-family: var(--wt-font-family); font-size: 14px; line-height: 24px; }
.resource-overlay__body > :deep(.feedback-alert:not(:last-child)) { margin-bottom: 16px; }
.resource-overlay__footer { display: flex; justify-content: flex-end; align-items: center; flex-wrap: wrap; gap: 8px; }
.resource-overlay__body :deep(.t-descriptions__label) { color: var(--wt-text-tertiary); }
.resource-overlay__body :deep(.t-descriptions__content) { overflow-wrap: anywhere; }
:global(.resource-overlay-context .t-dialog__position) { padding: 16px; height: 100%; }
:global(.resource-overlay-dialog) { padding: 0; max-height: calc(100dvh - 32px); display: flex; flex-direction: column; border-radius: 8px; box-shadow: var(--wt-shadow-popup); }
:global(.resource-overlay-dialog .t-dialog__header) { flex-shrink: 0; margin: 0; height: 64px; min-height: 64px; box-sizing: border-box; padding: 0 24px; display: flex; align-items: center; border-bottom: 1px solid var(--wt-border); font-size: 18px; font-weight: 600; }
:global(.resource-overlay-dialog .t-dialog__body) { padding: 24px; overflow: auto; min-height: 0; }
:global(.resource-overlay-dialog .t-dialog__footer) { flex-shrink: 0; margin: 0; padding: 16px 24px; box-sizing: border-box; width: 100%; border-top: 1px solid var(--wt-border); }
:global(.resource-overlay-dialog .t-dialog__close) { top: 19px; right: 20px; }
:global(.resource-overlay-drawer--full-width .t-drawer__body) { background: var(--wt-bg-page); }
:global(.resource-overlay-drawer .t-drawer__header) { height: 64px; min-height: 64px; box-sizing: border-box; padding: 0 24px; display: flex; align-items: center; font-size: 18px; font-weight: 600; border-bottom: 1px solid var(--wt-border); }
:global(.resource-overlay-drawer .t-drawer__body) { padding: 24px; overscroll-behavior: contain; }
:global(.resource-overlay-drawer .t-drawer__footer) { padding: 16px 24px; box-sizing: border-box; }
@media (max-width: 600px) {
  :global(.resource-overlay-dialog .t-dialog__header), :global(.resource-overlay-drawer .t-drawer__header) { padding: 16px; }
  :global(.resource-overlay-dialog .t-dialog__body), :global(.resource-overlay-drawer .t-drawer__body) { padding: 20px 16px; }
  :global(.resource-overlay-dialog .t-dialog__footer), :global(.resource-overlay-drawer .t-drawer__footer) { padding: 12px 16px; }
}
</style>
