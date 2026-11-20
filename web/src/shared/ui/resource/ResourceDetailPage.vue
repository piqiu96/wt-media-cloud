<script setup>
import { ref, watch, onMounted, nextTick, onBeforeUnmount } from 'vue'
import { Button } from 'tdesign-vue-next'
import ResourcePageHeader from './ResourcePageHeader.vue'
defineProps({ title: { type: String, required: true }, backLabel: { type: String, default: '返回列表' }, hideTitle: Boolean, busy: Boolean, returnButton: { type: Boolean, default: true } })
const emit = defineEmits(['back'])
const footer = ref(null), heading = ref(null), footerHeight = ref(69)
let observer, hostObserver
const footerBounds = ref({})
function updateFooterBounds() {
  let element = footer.value?.parentElement?.parentElement
  while (element && element !== document.body) {
    if (/(auto|scroll)/.test(getComputedStyle(element).overflowY)) {
      const bounds = element.getBoundingClientRect()
      footerBounds.value = { left: `${Math.max(0, bounds.left)}px`, right: `${Math.max(0, innerWidth - bounds.right)}px` }
      return element
    }
    element = element.parentElement
  }
  footerBounds.value = {}
  return null
}
watch(footer, element => {
  observer?.disconnect()
  hostObserver?.disconnect()
  if (!element) return
  const host = updateFooterBounds()
  if (host) { hostObserver = new ResizeObserver(updateFooterBounds); hostObserver.observe(host) }
  footerHeight.value = element.getBoundingClientRect().height
  observer = new ResizeObserver(() => { footerHeight.value = element.getBoundingClientRect().height })
  observer.observe(element)
}, { flush: 'post' })
onMounted(async () => { await nextTick(); heading.value?.focus({ preventScroll: true }); window.addEventListener('resize', updateFooterBounds) })
onBeforeUnmount(() => { observer?.disconnect(); hostObserver?.disconnect(); window.removeEventListener('resize', updateFooterBounds) })
</script>
<template>
  <section class="resource-detail-page" :class="{ 'has-footer': returnButton || !!$slots.footer }" :style="{ '--detail-footer-height': `${footerHeight}px` }" :aria-label="title" :aria-busy="busy">
    <div v-if="!hideTitle" ref="heading" class="resource-detail-page__heading" tabindex="-1" :aria-label="title"><ResourcePageHeader :title="title" /></div>
    <div class="resource-detail-page__content"><slot /></div>
    <footer v-if="returnButton || $slots.footer" ref="footer" :style="footerBounds" class="resource-detail-page__footer" aria-label="页面操作">
      <Button v-if="returnButton" class="resource-detail-page__back" theme="default" variant="outline" :aria-description="backLabel" :disabled="busy" @click="emit('back')">返回</Button>
      <slot name="footer" />
    </footer>
  </section>
</template>
<style scoped>
.resource-detail-page { min-width: 0; color: var(--wt-text-primary); font-size: 14px; font-weight: 400; line-height: 1.7; }
.resource-detail-page.has-footer { padding-bottom: calc(var(--detail-footer-height, 69px) + 20px); }
.resource-detail-page__heading { min-height: 36px; margin-bottom: var(--wt-space-page-heading, 20px); }
.resource-detail-page__heading:focus { outline: none; }
.resource-detail-page__heading :deep(.resource-page-header) { align-items: center; margin-bottom: 0; }
.resource-detail-page__content { display: flex; flex-direction: column; gap: var(--wt-space-block, 16px); min-width: 0; }
.resource-detail-page__content > :deep(.resource-notice) { margin: 0; }
.resource-detail-page__content :deep(.detail-card) { padding: var(--wt-space-card, 24px); }
.resource-detail-page__content :deep(.task-detail), .resource-detail-page__content :deep(.batch-detail-layout) { gap: var(--wt-space-block, 16px); }
.resource-detail-page__footer { position: fixed; left: var(--resource-detail-sidebar-width, 0px); right: 0; bottom: 0; z-index: 20; display: flex; justify-content: flex-end; align-items: center; flex-wrap: wrap; gap: 8px; padding: 16px 28px max(16px, env(safe-area-inset-bottom)); border-top: 1px solid var(--wt-border); background: var(--wt-bg-card); }
.resource-detail-page__footer :deep(.t-button) { height: 36px; font-size: 14px; font-weight: 400; }
.resource-detail-page__footer :deep(.resource-overlay__footer) { display: contents; }
.resource-detail-page__footer :deep(.resource-footer-status), .resource-detail-page__footer :deep(.quick-footer-note), .resource-detail-page__footer :deep(.batch-footer-note), .resource-detail-page__footer :deep(.cover-save-status) { order: -1; margin-right: auto; }
.resource-detail-page__footer :deep(.is-danger) { color: var(--wt-danger); background: var(--wt-danger-bg); }
.resource-detail-page__footer :deep(.is-danger:hover:not(:disabled)) { color: var(--wt-danger); background: var(--wt-danger-bg); }
@media (max-width: 1100px) { .resource-detail-page__footer { padding-inline: 20px; } }
@media (max-width: 720px) { .resource-detail-page__heading :deep(.resource-page-header) { align-items: flex-start; } }
@media (max-width: 680px) { .resource-detail-page__content :deep(.detail-card) { padding: 20px 16px; } .resource-detail-page__footer { padding: 12px 16px max(12px, env(safe-area-inset-bottom)); } }
</style>
