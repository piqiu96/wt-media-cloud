<script setup>
import { computed, inject, nextTick, onUnmounted, useAttrs, watch } from "vue";
import ResourceOverlay from "../../../src/shared/ui/resource/ResourceOverlay.vue";

import ResourceEditorPage from "../../../src/shared/ui/resource/ResourceEditorPage.vue";

defineOptions({ inheritAttrs: false });
const props = defineProps({ visible: Boolean, workspace: Boolean, page: Boolean, taskPage: Boolean, hideTitle: Boolean, backLabel: String });
const attrs = useAttrs();
const pages = inject('publishing-editor-pages', null);
const identity = Symbol('publishing-editor');
const topPage = computed(() => !pages || pages.value.at(-1)?.id === identity);
let pageScroll = 0;
async function requestClose() {
  if (attrs.busy) return;
  const handlers = attrs['onUpdate:visible'];
  for (const handler of Array.isArray(handlers) ? handlers : [handlers]) await handler?.(false);
}
watch(() => props.visible && props.page, async active => {
  if (!pages) return;
  if (active) {
    pageScroll = window.scrollY;
    pages.value.push({ id: identity, title: attrs.title, taskPage: props.taskPage, close: requestClose });
    await nextTick();
    window.scrollTo(0, 0);
  } else {
    const index = pages.value.findIndex(item => item.id === identity);
    if (index < 0) return;
    pages.value.splice(index, 1);
    await nextTick();
    window.scrollTo(0, pageScroll);
  }
}, { immediate: true });
onUnmounted(() => {
  if (pages) pages.value = pages.value.filter(item => item.id !== identity);
});
let opener = null;
let restoreTimer;
watch(
  () => props.visible,
  (visible) => {
    clearTimeout(restoreTimer);
    if (visible) {
      opener = document.activeElement;
      return;
    }
    // TDesign removes its focus sentinel at the end of the exit animation.
    // Restore once that has finished; the shared overlay still owns focus trapping.
    restoreTimer = setTimeout(() => {
      if (opener?.isConnected && opener.getClientRects().length)
        opener.focus?.({ preventScroll: true });
    }, 400);
  },
);
onUnmounted(() => clearTimeout(restoreTimer));
</script>

<template>
  <Teleport v-if="page && pages" to="#publishing-editor-root" defer>
    <component :is="ResourceEditorPage" v-if="visible" v-show="topPage" :title="attrs.title" :hide-title="hideTitle" :busy="attrs.busy" :back-label="backLabel || '返回列表'" :class="[attrs.class, { 'publishing-task-page': taskPage }]" @back="requestClose">
      <div class="resource-overlay__body"><slot /></div>
      <template v-if="$slots.footer" #footer><div class="resource-overlay__footer"><slot name="footer" /></div></template>
    </component>
  </Teleport>
  <ResourceOverlay v-else
    v-bind="attrs"
    :visible="visible"
    :class="{ 'publishing-workspace-overlay': workspace }"
  >
    <template v-for="(_, name) in $slots" #[name]="slotProps">
      <slot :name="name" v-bind="slotProps || {}" />
    </template>
  </ResourceOverlay>
</template>

<style>
.publishing-workspace-overlay .t-drawer__content-wrapper {
  width: calc(100vw - 216px) !important;
  max-width: 1280px !important;
  box-shadow: var(--wt-shadow-popup) !important;
}
.publishing-workspace-overlay .t-drawer__content,
.publishing-workspace-overlay .t-drawer__body {
  background: var(--wt-color-bg-container);
}
.publishing-workspace-overlay .t-drawer__header {
  height: 64px;
  min-height: 64px;
  padding: 0 28px;
  background: var(--wt-color-bg-container);
  border-bottom: 1px solid var(--wt-color-border);
  color: var(--wt-color-text-primary);
  font-size: 18px;
}
.publishing-workspace-overlay .t-drawer__body {
  padding: 24px 28px 32px;
}
.publishing-workspace-overlay .t-drawer__footer {
  padding: 16px 28px;
  background: var(--wt-color-bg-container);
  border-top: 1px solid var(--wt-color-border);
}
.publishing-workspace-overlay .t-drawer__mask {
  background: var(--wt-color-overlay-mask) !important;
}
.publishing-workspace-overlay .resource-overlay__body,
.publishing-workspace-overlay .resource-overlay__footer {
  max-width: 1200px;
  margin-inline: auto;
}
@media (max-width: 900px) {
  .publishing-workspace-overlay .t-drawer__content-wrapper {
    width: calc(100vw - 32px) !important;
  }
  .publishing-workspace-overlay .t-drawer__body {
    padding: 20px;
  }
}
@media (max-width: 680px) {
  .publishing-workspace-overlay .t-drawer__content-wrapper {
    width: 100vw !important;
  }
  .publishing-workspace-overlay .t-drawer__body {
    padding: 16px;
  }
}
</style>
