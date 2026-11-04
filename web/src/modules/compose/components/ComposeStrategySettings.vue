<script setup>
import { ref } from 'vue'
import ComposeReorderActions from './ComposeReorderActions.vue'
import { draggedKey, reorderItems } from '../reorder.js'
import { preserveDraftScroll } from '../preserveDraftScroll.js'
import { OUTPUT_PLATFORMS, newSetting, copySetting } from '../strategySettings.js'
const props = defineProps({ modelValue: { type: Array, required: true }, fields: { type: Array, default: () => [] }, errors: { type: Object, default: () => ({}) } })
const emit = defineEmits(['update:modelValue'])
const root = ref(null)
const dropTarget = ref(null)
const limit = 20
function update(groups) { preserveDraftScroll(root.value, () => emit('update:modelValue', groups)) }
function add() {
  if (props.modelValue.length >= limit) return
  update([...props.modelValue, newSetting(props.fields, props.modelValue.length + 1)])
}
function copy(group) {
  if (props.modelValue.length >= limit) return
  const groups = [...props.modelValue]
  groups.splice(groups.indexOf(group) + 1, 0, copySetting(group))
  update(groups)
}
function move(index, delta) {
  const target = props.modelValue[index + delta]
  if (target) update(reorderItems(props.modelValue, props.modelValue[index].key, target.key))
}
function drop(event, group) {
  const source = draggedKey(event.dataTransfer, 'settings')
  if (source) update(reorderItems(props.modelValue, source, group.key))
  dropTarget.value = null
}
function remove(index) { if (props.modelValue.length > 1) update(props.modelValue.filter((_, position) => position !== index)) }
</script>
<template>
  <div ref="root" class="strategy-settings">
    <section v-for="(group, index) in modelValue" :key="group.key" class="strategy-setting" :class="{ 'is-drop-target': dropTarget === group.key }" @dragover.prevent="dropTarget = group.key" @dragleave.self="dropTarget = null" @drop.prevent="drop($event, group)">
      <ComposeReorderActions :item-key="group.key" scope="settings" :index="index" :count="modelValue.length" :remove-disabled="modelValue.length === 1" @move="move(index, $event)" @remove="remove(index)" @dragend="dropTarget = null">
        成片设置 {{ index + 1 }}
        <template #extra><t-button size="small" variant="text" :disabled="modelValue.length >= limit" @click="copy(group)">复制</t-button></template>
      </ComposeReorderActions>
      <t-form label-align="top">
        <t-form-item label="成片设置名称" required-mark>
          <div class="strategy-setting__field"><t-input v-model="group.name" :maxlength="120" placeholder="例如：通用版" :status="errors[group.key]?.name ? 'error' : undefined" /><p v-if="errors[group.key]?.name" class="strategy-setting__error">{{ errors[group.key].name }}</p></div>
        </t-form-item>
        <t-form-item label="适用平台" required-mark>
          <div class="strategy-setting__field"><t-checkbox-group v-model="group.platforms" :options="OUTPUT_PLATFORMS" /><p v-if="errors[group.key]?.platforms" class="strategy-setting__error">{{ errors[group.key].platforms }}</p></div>
        </t-form-item>
        <t-form-item label="每条素材生成份数" required-mark>
          <div class="strategy-setting__field"><t-input-number v-model="group.copies" :min="1" :max="20" suffix="份" :status="errors[group.key]?.copies ? 'error' : undefined" /><p v-if="errors[group.key]?.copies" class="strategy-setting__error">{{ errors[group.key].copies }}</p></div>
        </t-form-item>
        <div class="strategy-setting__subtitle">模板参数</div>
        <t-form-item v-for="field in fields" :key="field.key" :required-mark="field.required">
          <template #label><span>{{ field.label }} <t-tooltip v-if="field.description" :content="field.description"><span class="strategy-setting__help" tabindex="0" :aria-label="field.description">ⓘ</span></t-tooltip></span></template>
          <div class="strategy-setting__field">
            <t-input-number v-if="['integer', 'number'].includes(field.type)" v-model="group.params[field.key]" :min="field.minimum" :max="field.maximum" :decimal-places="field.type === 'integer' ? 0 : undefined" :status="errors[group.key]?.[field.key] ? 'error' : undefined" />
            <t-switch v-else-if="field.type === 'boolean'" v-model="group.params[field.key]" />
            <t-input v-else v-model="group.params[field.key]" :type="field.format === 'uri' || field.key === 'guide_video_url' ? 'url' : 'text'" :placeholder="field.key === 'guide_video_url' ? '输入 HTTPS 视频地址' : `请输入${field.label}`" :status="errors[group.key]?.[field.key] ? 'error' : undefined" />
            <p v-if="errors[group.key]?.[field.key]" class="strategy-setting__error">{{ errors[group.key][field.key] }}</p>
          </div>
        </t-form-item>
      </t-form>
    </section>
    <t-button variant="outline" class="strategy-settings__add" :disabled="modelValue.length >= limit" :title="modelValue.length >= limit ? '最多 20 组成片设置' : undefined" @click="add">新增成片设置</t-button>
  </div>
</template>
<style scoped>
.strategy-settings { display: grid; gap: 12px; }
.strategy-setting { padding: 12px; border: 1px solid var(--wt-border); border-radius: var(--wt-radius-md); overflow-anchor: none; }
.strategy-setting.is-drop-target { border-color: var(--wt-primary); }
.strategy-setting__subtitle { margin: 8px 0 12px; color: var(--wt-text-primary); font-weight: 600; }
.strategy-setting__field { display: grid; gap: 4px; width: 100%; }
.strategy-setting__error { margin: 0; color: var(--td-error-color); font-size: 12px; }
.strategy-setting__help { color: var(--wt-text-tertiary); cursor: help; }
.strategy-settings__add { width: 100%; }
</style>
