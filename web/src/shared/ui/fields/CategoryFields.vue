<script setup>
import { computed } from 'vue'
import { CheckboxGroup as TCheckboxGroup, Checkbox as TCheckbox, Option as TOption, Tag as TTag } from 'tdesign-vue-next'
import BusinessSelect from './BusinessSelect.vue'
import { CATEGORY_TYPE_OPTIONS } from '../../utils/categories.js'
const props = defineProps({ categories: { type: Array, default: () => [] }, filter: Boolean })
const type = defineModel('type', { type: String, default: '' })
const ids = defineModel({ type: Array, default: () => [] })
const unavailable = computed(() => ids.value.filter(id => !props.categories.some(c => c.id === id)))
const options = computed(() => props.categories.filter(c => c.type === type.value))
function changeType(value) { type.value = value || ''; ids.value = [] }
</script>

<template>
  <div class="category-fields" :class="{ 'category-fields--filter': filter }">
    <label class="category-fields__parent"><span>一级分类</span>
      <BusinessSelect kind="category" :filter="filter" :model-value="type" :options="CATEGORY_TYPE_OPTIONS" :placeholder="filter ? '全部' : '选择一级分类'" @change="changeType" />
    </label>
    <component :is="filter ? 'label' : 'div'" class="category-fields__children">
      <span>二级分类<span v-if="!filter && type" class="category-fields__optional"> · 可多选</span></span>
      <BusinessSelect v-if="filter" v-model="ids" kind="category" filter multiple :disabled="!type" :placeholder="type ? '全部' : '先选择一级分类'">
        <TOption v-for="c in options" :key="c.id" :value="c.id" :label="c.name" />
      </BusinessSelect>
      <template v-else>
        <TCheckboxGroup v-if="type && options.length" v-model="ids" class="category-fields__options">
          <TCheckbox v-for="c in options" :key="c.id" :value="c.id">{{ c.name }}</TCheckbox>
        </TCheckboxGroup>
        <div v-else class="category-fields__empty">{{ type ? '该分类下暂无可选类目' : '请先选择一级分类' }}</div>
      </template>
      <div v-if="unavailable.length" class="category-fields__unavailable">原分类已不可用，请重新选择：<TTag v-for="id in unavailable" :key="id" closable @close="ids = ids.filter(value => value !== id)">分类 {{ id }}</TTag></div>
    </component>
  </div>
</template>

<style scoped>
.category-fields { width: 100%; min-width: 0; display: grid; grid-template-columns: 170px minmax(0, 1fr); gap: 20px; }
.category-fields__parent, .category-fields__children { display: flex; flex-direction: column; gap: 8px; min-width: 0; font-size: 13px; color: var(--wt-text-tertiary); }
.category-fields__optional { color: var(--wt-text-tertiary); font-size: 12px; }
.category-fields__options { display: flex; flex-wrap: wrap; gap: 12px 20px; }
.category-fields__empty { box-sizing: border-box; min-height: 36px; display: flex; align-items: center; background: var(--wt-bg-page); padding: 8px 12px; border-radius: var(--wt-radius-sm); font-size: 12px; }
.category-fields--filter { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 24px; }
@media (max-width: 520px) { .category-fields { grid-template-columns: 1fr; } }
</style>
