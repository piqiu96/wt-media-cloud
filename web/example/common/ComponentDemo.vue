<script setup>
import ResourceNameLink from '../../src/shared/ui/resource/ResourceNameLink.vue'
import ResourceId from '../../src/shared/ui/resource/ResourceId.vue'
import ResourceDateRange from '../../src/shared/ui/resource/ResourceDateRange.vue'
import { computed, ref } from 'vue'
import ResourceFieldHelp from '../../src/shared/ui/resource/ResourceFieldHelp.vue'
import '../../src/shared/styles/resource-list.css'
import ResourceCard from '../../src/shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../src/shared/ui/resource/ResourcePageHeader.vue'
import ResourceFilterBar from '../../src/shared/ui/resource/ResourceFilterBar.vue'
import ResourceRowActions from '../../src/shared/ui/resource/ResourceRowActions.vue'
import ResourceOverlay from '../../src/shared/ui/resource/ResourceOverlay.vue'
import ResourceForm from '../../src/shared/ui/resource/ResourceForm.vue'
import { DescriptionsItem as Field } from 'tdesign-vue-next'
import ResourceDetailPage from '../../src/shared/ui/resource/ResourceDetailPage.vue'
import ResourceEditorPage from '../../src/shared/ui/resource/ResourceEditorPage.vue'
import ResourceSection from '../../src/shared/ui/resource/ResourceSection.vue'
import ResourceDescriptions from '../../src/shared/ui/resource/ResourceDescriptions.vue'
import ResourceStatusBadge from '../../src/shared/ui/resource/ResourceStatusBadge.vue'
import ResourceProgress from '../../src/shared/ui/resource/ResourceProgress.vue'
import '../../src/shared/styles/resource-data-table.css'
const period=ref([])
const editorPage = ref(false)
const selected = ref(null)
const query = ref(''), applied = ref(''), visible = ref(false), title = ref('演示视频')
const items = computed(() => [{ id: 1, name: '演示视频' }, { id: 2, name: '长标题示例：用于确认公共组件在真实内容长度下的表现' }].filter(row => row.name.includes(applied.value)))
const columns = [{ colKey: 'id', title: 'ID', width: 80 }, { colKey: 'name', title: '名称', ellipsis: true }, { colKey: 'op', title: '操作', width: 180 }]
</script>
<template>
  <section id="components" :class="{ 'resource-list-page': !selected && !editorPage }">
    <ResourceEditorPage v-if="editorPage" title="编辑资源示例" back-label="返回公共示例" @back="editorPage = false">
      <ResourceCard><ResourceSection title="资源内容" variant="form"><ResourceForm><t-form-item label="资源名称"><t-input v-model="title" /></t-form-item><t-form-item label="执行周期"><ResourceDateRange v-model="period" /></t-form-item><t-form-item><template #label>说明<ResourceFieldHelp label="说明" content="补充资源描述，留空不影响保存。" /></template><t-textarea placeholder="此页演示完整编辑工作区的公共结构" /></t-form-item></ResourceForm></ResourceSection></ResourceCard>
      <template #footer><span style="margin-right:auto;color:var(--wt-text-tertiary);font-size:12px">仅保存本地演示输入</span><t-button @click="editorPage = false">完成演示</t-button></template>
    </ResourceEditorPage>
    <ResourceDetailPage v-else-if="selected" title="资源详情示例" back-label="返回示例列表" @back="selected = null">
      <ResourceCard><ResourceSection title="资源信息"><ResourceDescriptions>
        <Field label="资源名称">{{ selected.name }}</Field><Field label="资源 ID">{{ selected.id }}</Field><Field label="资源状态"><ResourceStatusBadge label="可用" tone="success" /></Field><Field label="创建人">运营负责人</Field><Field label="创建时间">2026-11-14 10:00</Field><Field label="说明" full>长内容占满一行，短字段按当前区域宽度排列；此处直接使用共享组件。</Field>
      </ResourceDescriptions></ResourceSection></ResourceCard>
      <ResourceCard class="demo-flush-card"><ResourceSection title="关联资源" class="demo-flush-section"><t-table class="resource-data-table resource-embedded-table is-flush" row-key="id" :columns="columns" :data="items"><template #name="{row}"><ResourceNameLink @click="title=row.name;visible=true">{{row.name}}</ResourceNameLink></template><template #op="{row}"><ResourceRowActions><t-button variant="text" @click="title=row.name;visible=true">编辑</t-button></ResourceRowActions></template></t-table></ResourceSection></ResourceCard>
      <ResourceCard><ResourceSection title="进度示例"><ResourceProgress label="整体进度" :percentage="60" value-text="3 / 5 ·" /></ResourceSection></ResourceCard>
      <template #footer><t-button theme="primary" @click="title = selected.name; visible = true">编辑资源</t-button></template>
    </ResourceDetailPage>
    <template v-else>
    <ResourcePageHeader title="正式组件组合示例"><template #actions><t-button @click="editorPage = true">页面编辑示例</t-button></template></ResourcePageHeader>
    <p>本例直接引用 src/shared/ui 中的组件和正式主题；筛选、表格与表单只操作演示数据。</p>
    <ResourceCard class="resource-list-panel">
      <ResourceFilterBar><t-form layout="inline" @submit.prevent="applied = query"><t-form-item label="名称"><t-input v-model="query" clearable /></t-form-item><t-button @click="applied = query">查询</t-button><t-button variant="text" @click="query = ''; applied = ''">重置</t-button></t-form></ResourceFilterBar>
      <t-table class="resource-data-table" row-key="id" :columns="columns" :data="items"><template #id="{row}"><ResourceId :value="row.id" /></template><template #name="{row}"><ResourceNameLink @click="selected=row">{{row.name}}</ResourceNameLink></template><template #op="{ row }"><ResourceRowActions><t-button variant="text" @click="selected = row">独立详情</t-button><t-button variant="text" @click="title = row.name; visible = true">编辑</t-button></ResourceRowActions></template></t-table>
    </ResourceCard>
    </template>
    <ResourceOverlay v-model:visible="visible" title="编辑演示输入">
      <ResourceForm><t-form-item label="名称"><t-input v-model="title" /></t-form-item></ResourceForm>
      <template #footer><t-button @click="visible = false">关闭演示</t-button></template>
    </ResourceOverlay>
  </section>
</template>

<style scoped>
.demo-flush-card { padding: 0; overflow: hidden; }
.demo-flush-section :deep(.resource-section__header) { padding: 20px 24px 16px; margin: 0; }
@media (max-width: 680px) { .demo-flush-section :deep(.resource-section__header) { padding-inline: 16px; } }
</style>
