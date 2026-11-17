<script setup>
import { computed } from 'vue';
import ResourceRowActions from '../../../src/shared/ui/resource/ResourceRowActions.vue';
import { taskActionItems } from './taskExperience.js';
const props = defineProps({ task: { type: Object, required: true }, db: { type: Object, required: true } });
const emit = defineEmits(['action']);
const actions = computed(() => taskActionItems(props.task, props.db));
const visible = computed(() => actions.value.slice(0, actions.value.length > 5 ? 4 : 5));
const more = computed(() => actions.value.slice(4).map(a => ({ content: a.label, value: a.key, disabled: a.disabled, theme: a.danger ? 'error' : 'default' })));
</script>
<template>
  <ResourceRowActions>
    <template v-for="a in visible" :key="a.key">
      <t-tooltip v-if="a.reason" :content="a.reason"><span class="task-action-disabled"><t-button variant="text" :disabled="a.disabled" :class="{ 'is-primary': a.primary, 'is-danger': a.danger }" @click="emit('action', a.key)">{{ a.label }}</t-button></span></t-tooltip>
      <t-button v-else variant="text" :class="{ 'is-primary': a.primary, 'is-danger': a.danger }" @click="emit('action', a.key)">{{ a.label }}</t-button>
    </template>
    <t-dropdown v-if="actions.length > 5" :options="more" @click="({ value }) => emit('action', value)"><t-button variant="text">更多⌄</t-button></t-dropdown>
  </ResourceRowActions>
</template>
<style scoped>.task-action-disabled { display: inline-flex; cursor: not-allowed; }</style>
