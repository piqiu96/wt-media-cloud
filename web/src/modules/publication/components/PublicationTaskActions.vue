<script setup>
import ResourceRowActions from "../../../shared/ui/resource/ResourceRowActions.vue";

defineProps({ actions: { type: Array, required: true } });
defineEmits(["action"]);
</script>

<template>
  <ResourceRowActions v-if="actions.length">
    <t-button v-for="item in actions" :key="item.key" size="small" variant="text"
      :class="{ 'is-primary': item.primary }" :theme="item.danger ? 'danger' : 'primary'"
      :disabled="item.disabled" :title="item.reason || undefined" @click="$emit('action', item.key)">
      {{ item.label }}
    </t-button>
  </ResourceRowActions>
</template>
