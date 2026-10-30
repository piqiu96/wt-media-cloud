<script setup>
import { Select as TSelect } from 'tdesign-vue-next'
// 只统一公共字段的输入体验；目录来源、可见范围和值类型由调用方负责。
defineOptions({ inheritAttrs: false })
defineProps({
  kind: { type: String, default: 'category' },
  filter: Boolean,
})
const names = { category: '分类', team: '运营团队', member: '成员' }
</script>

<template>
  <TSelect class="business-select" filterable clearable
    :placeholder="filter ? '全部' : `选择${names[kind] || '选项'}`" v-bind="$attrs">
    <template v-for="(_, name) in $slots" #[name]="scope"><slot :name="name" v-bind="scope || {}" /></template>
  </TSelect>
</template>

<style scoped>
.business-select { width: 100%; min-width: 0; }
</style>
