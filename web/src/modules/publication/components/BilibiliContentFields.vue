<script setup>
import { computed, ref, watch } from 'vue';
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue';
import { usePublicationPlatforms } from '../usePublicationPlatforms.js';

const props = defineProps({ content: { type: Object, required: true }, disabled: Boolean });
const emit = defineEmits(['patch']);
const platforms = usePublicationPlatforms();
const rule = computed(() => platforms.ruleFor('bilibili'));
const options = computed(() => props.content.bilibili || {});
const tagsText = ref((options.value.tags || []).join('\n'));
let lastTags = JSON.stringify(options.value.tags || []);
watch(() => JSON.stringify(options.value.tags || []), value => {
  if (value !== lastTags) tagsText.value = (options.value.tags || []).join('\n');
  lastTags = value;
});
function change(patch) { emit('patch', { bilibili: { ...options.value, ...patch } }); }
// An older unfinished draft may have no statement. Complete that editable
// input through the normal patch path, without rewriting a saved choice.
watch(() => [props.disabled, options.value.statement], () => {
  if (!props.disabled && !options.value.statement) change({ statement: '内容无需标注' });
}, { immediate: true });
function tags(value) {
  tagsText.value = value;
  const next = value.split('\n').map(tag => tag.trim()).filter(Boolean);
  lastTags = JSON.stringify(next);
  change({ tags: next });
}
function partition(id) {
  const selected = rule.value?.partition_options?.find(row => row.id === Number(id));
  change({ partition_id: selected?.id || 0, partition_name: selected?.name || '' });
}
</script>

<template>
  <FeedbackAlert v-if="platforms.error.value" theme="error" :message="platforms.error.value">
    <t-button variant="text" @click="platforms.reload">重新读取平台规则</t-button>
  </FeedbackAlert>
  <div class="batch-form-grid bilibili-fields">
    <label class="batch-form-wide">发布标题
      <t-input :value="content.title || ''" aria-label="B站发布标题" :maxlength="80" :disabled="disabled"
        @change="emit('patch', { title: $event })" />
      <small>{{ (content.title || '').length }}/80 字</small>
    </label>
    <label class="batch-form-wide">简介（选填）
      <t-textarea :value="content.description || ''" aria-label="B站简介" placeholder="没有补充说明可留空，不必重复标题或填写素材来源" :maxlength="2000" :disabled="disabled"
        :autosize="{ minRows: 3, maxRows: 8 }" @change="emit('patch', { description: $event })" />
      <small>{{ (content.description || '').length }}/2000 字</small>
    </label>
    <label>发布分区
      <t-select :value="Number(options.partition_id) || undefined" aria-label="B站发布分区" :disabled="disabled || platforms.loading.value"
        :options="(rule?.partition_options || []).map(row => ({ value: row.id, label: row.name }))" filterable clearable
        placeholder="选择适合内容的分区" @change="partition" />
    </label>
    <label>可见范围
      <t-select :value="options.visibility || ''" aria-label="B站可见范围" :disabled="disabled"
        :options="[{ value: 'public', label: '公开可见' }, { value: 'private', label: '仅自己可见' }]"
        @change="change({ visibility: $event })" />
      <small v-if="options.visibility === 'private'">仅自己可见的稿件不会计为公开成功。</small>
    </label>
    <label class="batch-form-wide">标签
      <t-textarea :value="tagsText" aria-label="B站标签" placeholder="每行一个，最多10个，每个不超过20字" :disabled="disabled"
        :autosize="{ minRows: 2, maxRows: 5 }" @change="tags" />
    </label>
    <label>创作声明
      <t-input :value="options.statement || '内容无需标注'" aria-label="B站创作声明" readonly />
      <small v-if="!options.statement || options.statement === '内容无需标注'">普通发布固定使用“内容无需标注”。</small>
      <small v-else>保留已保存内容的原声明。</small>
    </label>
    <label class="batch-form-wide" v-if="options.statement === '内容为转载'">转载来源说明
      <t-textarea :value="options.source || ''" aria-label="B站转载来源" :maxlength="200" :disabled="disabled"
        placeholder="注明来源、时间、地点等信息" @change="change({ source: $event })" />
    </label>
    <t-checkbox class="batch-form-wide" :checked="options.rights_reserved === true" :disabled="disabled || options.statement === '内容为转载'"
      @change="change({ rights_reserved: $event })">内容为自制：未经作者允许，禁止转载</t-checkbox>
  </div>
</template>

<style scoped>
.bilibili-fields small { color: var(--wt-text-secondary); font-weight: 400; line-height: 1.6; }
</style>
