<script setup>
import { computed, ref } from 'vue'
import ResourceForm from '../../../shared/ui/resource/ResourceForm.vue'
import ResourceFieldHelp from '../../../shared/ui/resource/ResourceFieldHelp.vue'
import ResourceOverlay from '../../../shared/ui/resource/ResourceOverlay.vue'
import { platformText, settingFields, strategySettings } from '../strategySettings.js'
import { validateTaskGroup } from '../taskGroupOverrides.js'
const props = defineProps({ strategy: { type: Object, required: true }, template: Object, materialCount: { type: Number, default: 0 }, modelValue: { type: Object, default: () => ({}) } })
const emit = defineEmits(['update:modelValue'])
const groups = computed(() => strategySettings(props.strategy))
const fields = computed(() => settingFields(props.template?.schema))
const activeGroup = ref(null), visible = ref(false), draft = ref({}), errors = ref({})
const editable = (field) => field.editable === true && field.key !== 'guide_video_url'
const hasEditable = computed(() => fields.value.some(editable))
const columns = [{ colKey: 'name', title: '成片设置', width: 170 }, { colKey: 'platforms', title: '适用平台', width: 180 }, { colKey: 'copies', title: '每条份数', width: 100 }, { colKey: 'estimated', title: '预计任务', width: 100 }, { colKey: 'op', title: '操作', width: 80 }]
function open(group) {
  activeGroup.value = group
  draft.value = { ...group.params, ...(props.modelValue[group.key] || {}) }
  errors.value = {}
  visible.value = true
}
function save() {
  errors.value = validateTaskGroup(activeGroup.value, draft.value, fields.value)
  if (Object.keys(errors.value).length) return
  emit('update:modelValue', { ...props.modelValue, [activeGroup.value.key]: Object.fromEntries(fields.value.filter(editable).map((field) => [field.key, draft.value[field.key]])) })
  visible.value = false
}
</script>
<template>
  <div class="task-settings">
    <div class="table-scroll-wrap"><t-table class="wt-resource-table" :data="groups" :columns="columns" row-key="key" :scroll="{x:'630px'}" size="small"><template #platforms="{row}">{{ platformText(row.platforms) }}</template><template #estimated="{row}">{{ materialCount * row.copies }}</template><template #op="{row}"><t-link theme="primary" @click="open(row)">参数</t-link></template></t-table></div>
    <ResourceOverlay :visible="visible" :title="`${activeGroup?.name || ''} · 模板参数`" :width="600" :footer="hasEditable" @close="visible = false">
      <p class="task-settings__notice">仅本次任务生效，不修改策略。</p>
      <ResourceForm class="task-settings__form">
        <t-form-item v-for="field in fields" :key="field.key" :required-mark="field.required && editable(field)">
          <template #label>{{ field.label }}<ResourceFieldHelp v-if="!editable(field) || field.description" :content="!editable(field) ? '固定参数，任务内不可调整。' : field.description" /></template>
          <div class="task-settings__field">

              <t-input-number v-if="['integer','number'].includes(field.type)" v-model="draft[field.key]" theme="column" :min="field.minimum" :max="field.maximum" :decimal-places="field.type === 'integer' ? 0 : undefined" :disabled="!editable(field)" :status="errors[field.key] ? 'error' : undefined" />
              <t-switch v-else-if="field.type === 'boolean'" v-model="draft[field.key]" :disabled="!editable(field)" />
              <t-input v-else-if="field.key === 'guide_video_url'" :value="activeGroup?.params?.[field.key] ? '已配置（固定）' : '未记录'" disabled />
              <t-input v-else v-model="draft[field.key]" :disabled="!editable(field)" :status="errors[field.key] ? 'error' : undefined" />

<p v-if="errors[field.key]" class="task-settings__error">{{ errors[field.key] }}</p>
          </div>
        </t-form-item>
        <p v-if="!fields.length" class="task-settings__hint">此版本暂无可展示的参数定义。</p>
      </ResourceForm>
      <template #footer><t-button class="wt-secondary-button" variant="outline" @click="visible = false">取消</t-button><t-button theme="primary" @click="save">确定</t-button></template>
    </ResourceOverlay>
  </div>
</template>
<style scoped>
.task-settings__notice { margin:0 0 20px;font-size:13px;color:var(--wt-text-secondary); }
.task-settings__form { display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:20px 24px; }
.task-settings__form :deep(.t-form__item) { margin-bottom:0;min-width:0; }
@media(max-width:560px){.task-settings__form{grid-template-columns:1fr;}}
.task-settings__field { display: grid; gap: 5px; width: 100%; }
.task-settings__hint { color: var(--wt-text-tertiary); font-size: 12px; }
.task-settings__error { margin: 0; color: var(--td-error-color); font-size: 12px; }
</style>
