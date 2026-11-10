<script setup>
import { onUnmounted, useAttrs, watch } from "vue";
import ResourceOverlay from "../../../src/shared/ui/resource/ResourceOverlay.vue";

defineOptions({ inheritAttrs: false });
const props = defineProps({ visible: Boolean });
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
  <ResourceOverlay v-bind="attrs" :visible="visible">
    <template v-for="(_, name) in $slots" #[name]="slotProps">
      <slot :name="name" v-bind="slotProps || {}" />
    </template>
  </ResourceOverlay>
</template>
