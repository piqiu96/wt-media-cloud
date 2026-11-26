<script setup>
import { computed, toRef } from 'vue';
import ResourceOverlay from '../../../shared/ui/resource/ResourceOverlay.vue';
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue';
import { createPublicationClient } from '../../../shared/api/publication.js';
import { createSessionClient } from '../../../shared/api/session.js';
import { usePublicationGeneration } from '../usePublicationGeneration.js';
import { platformGenerationFields } from '../platformContent.js';
const props = defineProps({ context: { type: Object, required: true }, values: { type: Object, required: true }, template: Boolean, disabled: Boolean });
const emit = defineEmits(['adopt']);
const labels = computed(() => platformGenerationFields(props.context.platform));
const fields = computed(() => Object.keys(labels.value).filter(field => props.template || field !== 'cover_title'));
const client = createPublicationClient(), session = createSessionClient();
const { visible, selected, instruction, busy, error, suggestion, stage, start, cancel, canAdopt, apply } = usePublicationGeneration({ context: toRef(props, 'context'), values: toRef(props, 'values'), fields, disabled: toRef(props, 'disabled'), generate: client.generate, getIdentity: async () => String((await session.me()).id), adopt: (field, value) => emit('adopt', field, value) });
function open() { if (props.disabled) return; selected.value = [...fields.value]; visible.value = true; }
</script>
<template>
  <t-button size="small" theme="default" variant="outline" :disabled="disabled" @click="open">AI 填写</t-button>
  <ResourceOverlay :visible="visible" title="AI 文案" :width="640" @update:visible="visible = $event">
    <div class="publication-generation">
      <p>根据当前成片的素材标题和已填文案生成建议。请核对事实后逐项采用，采用后仍需确认发布内容。</p>
      <FeedbackAlert v-if="error" theme="error" :message="error" />
      <label>生成字段<t-checkbox-group v-model="selected" :disabled="busy" :options="fields.map(value => ({ value, label: labels[value] }))" /></label>
      <label>文案要求<t-textarea v-model="instruction" aria-label="文案要求" placeholder="例如：简洁自然，突出操作要点" :maxlength="500" :autosize="{ minRows: 2, maxRows: 4 }" :disabled="busy" /></label>
      <p v-if="stage" role="status">{{ stage }}</p>
      <section v-for="(item, field) in suggestion" :key="field" class="publication-suggestion">
        <div><strong>{{ labels[field] }}</strong><t-button size="small" variant="text" :disabled="!canAdopt(field)" @click="apply(field)">{{ item.adopted ? '已采用' : canAdopt(field) ? '采用' : '内容已变化，请重新生成' }}</t-button></div>
        <p>{{ item.value || '不添加话题' }}</p>
      </section>
    </div>
    <template #footer>
      <t-button theme="default" variant="outline" @click="visible = false">关闭</t-button>
      <t-button v-if="busy" theme="default" variant="outline" @click="cancel">停止生成</t-button>
      <t-button :loading="busy" :disabled="disabled || !selected.length" @click="start">{{ suggestion ? '重新生成' : '生成建议' }}</t-button>
    </template>
  </ResourceOverlay>
</template>
<style scoped>
.publication-generation { display: flex; flex-direction: column; gap: 16px; }
.publication-generation > p { margin: 0; color: var(--wt-text-secondary); line-height: 1.6; }
.publication-generation label { display: flex; flex-direction: column; gap: 8px; font-size: 13px; font-weight: 500; }
.publication-suggestion { padding: 16px; border: 1px solid var(--wt-border); border-radius: var(--wt-radius-md); }
.publication-suggestion > div { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.publication-suggestion p { margin: 8px 0 0; white-space: pre-wrap; overflow-wrap: anywhere; line-height: 1.6; }
</style>
