<script setup>
import { computed, ref } from 'vue'
import ResourceCard from '../../src/shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../src/shared/ui/resource/ResourcePageHeader.vue'
import ResourceFilterBar from '../../src/shared/ui/resource/ResourceFilterBar.vue'
import ResourceRowActions from '../../src/shared/ui/resource/ResourceRowActions.vue'
import ResourceOverlay from '../../src/shared/ui/resource/ResourceOverlay.vue'
import ResourceForm from '../../src/shared/ui/resource/ResourceForm.vue'
const query = ref(''), applied = ref(''), visible = ref(false), title = ref('演示视频')
const items = computed(() => [{ id: 1, name: '演示视频' }, { id: 2, name: '长标题示例：用于确认公共组件在真实内容长度下的表现' }].filter(row => row.name.includes(applied.value)))
const columns = [{ colKey: 'id', title: 'ID', width: 80 }, { colKey: 'name', title: '名称', ellipsis: true }, { colKey: 'op', title: '操作', width: 120 }]
</script>
<template>
  <section id="components">
    <ResourcePageHeader title="正式组件组合示例" />
    <p>本例直接引用 src/shared/ui 中的组件和正式主题；筛选、表格与表单只操作演示数据。</p>
    <ResourceCard>
      <ResourceFilterBar><t-form layout="inline" @submit.prevent="applied = query"><t-form-item label="名称"><t-input v-model="query" clearable /></t-form-item><t-button @click="applied = query">查询</t-button><t-button variant="text" @click="query = ''; applied = ''">重置</t-button></t-form></ResourceFilterBar>
      <t-table row-key="id" :columns="columns" :data="items"><template #op="{ row }"><ResourceRowActions><t-button variant="text" @click="title = row.name; visible = true">编辑</t-button></ResourceRowActions></template></t-table>
    </ResourceCard>
    <ResourceOverlay v-model:visible="visible" title="编辑演示输入">
      <ResourceForm><t-form-item label="名称"><t-input v-model="title" /></t-form-item></ResourceForm>
      <template #footer><t-button @click="visible = false">关闭演示</t-button></template>
    </ResourceOverlay>
  </section>
</template>
