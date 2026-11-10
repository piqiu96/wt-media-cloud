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
  width: calc(100vw - 208px) !important;
  max-width: none !important;
  box-shadow: -2px 0 18px #16335b16 !important;
}
.publishing-workspace-overlay .t-drawer__content,
.publishing-workspace-overlay .t-drawer__body {
  background: #f3f6fb;
}
.publishing-workspace-overlay .t-drawer__header {
  height: 68px;
  min-height: 68px;
  padding: 0 34px;
  background: #fff;
  border-bottom: 1px solid #e3eaf3;
  color: #20375b;
  font-size: 19px;
}
.publishing-workspace-overlay .t-drawer__body {
  padding: 26px 34px 34px;
}
.publishing-workspace-overlay .t-drawer__footer {
  padding: 14px 34px;
  background: #fff;
  border-top: 1px solid #e3eaf3;
  box-shadow: 0 -3px 14px #24446b0a;
}
.publishing-workspace-overlay .t-drawer__mask {
  background: transparent !important;
}
.publishing-workspace-overlay .resource-overlay__body,
.publishing-workspace-overlay .resource-overlay__footer {
  max-width: 1620px;
  margin-inline: auto;
}
@media (max-width: 900px) {
  .publishing-workspace-overlay .t-drawer__content-wrapper { width: 100vw !important; }
  .publishing-workspace-overlay .t-drawer__body { padding: 16px; }
}
</style>
