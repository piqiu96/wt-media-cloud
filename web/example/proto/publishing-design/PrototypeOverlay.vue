<script setup>
import { onUnmounted, useAttrs, watch } from "vue";
import ResourceOverlay from "../../../src/shared/ui/resource/ResourceOverlay.vue";

defineOptions({ inheritAttrs: false });
const props = defineProps({ visible: Boolean, workspace: Boolean });
const attrs = useAttrs();
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
  <ResourceOverlay
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
