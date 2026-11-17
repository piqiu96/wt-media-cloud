<script setup>
import { computed } from 'vue';
import { DescriptionsItem as Field } from 'tdesign-vue-next';
import ResourceOverlay from '../../../src/shared/ui/resource/ResourceOverlay.vue';
import ResourceDescriptions from '../../../src/shared/ui/resource/ResourceDescriptions.vue';
import { batchSummary } from './relationships.js';
import { batchState } from './batchLifecycle.js';
const props = defineProps({ batch: Object, db: Object });
const emit = defineEmits(['close', 'navigate']);
const plan = computed(() => props.db.plans.find(p => p.id === props.batch?.planId));
const metrics = computed(() => props.batch ? batchSummary(props.db, props.batch) : {});
</script>
<template>
  <ResourceOverlay :visible="!!batch" title="关联批次" :width="640" @update:visible="value => { if (!value) emit('close'); }">
    <ResourceDescriptions v-if="batch">
      <Field label="批次名称">{{ batch.name }}</Field>
      <Field label="批次 ID">{{ batch.id }}</Field>
      <Field label="发布平台">{{ batch.platform }}</Field>
      <Field label="批次状态">{{ batchState(db, batch) }}</Field>
      <Field label="所属计划">{{ plan?.name || '独立批次' }}</Field>
      <Field label="已公开发布">{{ metrics.published || 0 }}</Field>
      <Field label="目标数量">{{ metrics.target || 0 }}</Field>
    </ResourceDescriptions>
    <template #footer><t-button theme="default" variant="outline" @click="emit('navigate', batch)">前往批次详情</t-button></template>
  </ResourceOverlay>
</template>
