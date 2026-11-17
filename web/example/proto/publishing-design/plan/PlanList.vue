<script setup>
import ResourceNameLink from '../../../../src/shared/ui/resource/ResourceNameLink.vue'
import ResourceId from '../../../../src/shared/ui/resource/ResourceId.vue'
import {formatDateTime} from '../../../../src/shared/utils/datetime.js'
import { ref } from 'vue'
import { Button, Input, Select, Table, Tag, Progress, Tooltip } from 'tdesign-vue-next'
import ResourcePageHeader from '../../../../src/shared/ui/resource/ResourcePageHeader.vue'
import ResourceCard from '../../../../src/shared/ui/resource/ResourceCard.vue'
import ResourceFilterBar from '../../../../src/shared/ui/resource/ResourceFilterBar.vue'
import ResourceRowActions from '../../../../src/shared/ui/resource/ResourceRowActions.vue'
import ResourceEmptyState from '../../../../src/shared/ui/resource/ResourceEmptyState.vue'

const props = defineProps({ view: { type: Object, required: true } })
const emit = defineEmits(['filter-change', 'query', 'page-change'])
const draft = ref({ ...props.view.draft })
const current = ref(props.view.pageNo), pageSize = ref(props.view.pageSize)
function filter(key, value) { draft.value[key] = value; emit('filter-change', key, value) }
function pageChange(page) { current.value = page.pageSize === pageSize.value ? page.current : 1; pageSize.value = page.pageSize; emit('page-change', { ...page, current: current.value }) }
const columns = [
  { colKey: 'id', title: '计划ID', width: 190 },
  { colKey: 'identity', title: '计划名称', width: 240 },
  { colKey: 'status', title: '计划状态', width: 100 },
  { colKey: 'note', title: '当前进展', width: 230 },
  { colKey: 'progress', title: '发布进度', width: 170 },
  { colKey: 'batches', title: '批次进度', width: 100 },
  { colKey: 'dates', title: '执行周期', width: 130 },
  { colKey: 'updated', title: '更新时间', width: 155 },
  { colKey: 'operation', title: '操作', width: 216, fixed: 'right' },
]
</script>

<template>
  <ResourcePageHeader title="发布计划">
    <template #actions><Button theme="primary" data-act="new">新建发布计划</Button></template>
  </ResourcePageHeader>
  <ResourceCard class="resource-list-panel plan-list-panel">
    <ResourceFilterBar>
      <label class="wt-filter-field wt-filter-field--wide"><span class="wt-filter-field__label">综合搜索</span><Input :value="draft.search" placeholder="输入计划名称或 ID" clearable @update:value="filter('search', $event)" @enter="emit('query')" /></label>
      <label class="wt-filter-field"><span class="wt-filter-field__label">计划状态</span><Select :value="draft.status" :options="view.statuses" @update:value="filter('status', $event)" /></label>
      <label class="wt-filter-field"><span class="wt-filter-field__label">发布平台</span><Select :value="draft.platform" :options="view.platforms" @update:value="filter('platform', $event)" /></label>
      <label class="wt-filter-field"><span class="wt-filter-field__label">发布类目</span><Select :value="draft.category" :options="view.categories" @update:value="filter('category', $event)" /></label>
      <div class="wt-filter-actions"><Button id="plan-query" theme="primary" @click="emit('query')">查询</Button><Button theme="default" variant="outline" data-act="reset-filters">重置</Button></div>
    </ResourceFilterBar>
    <Table class="resource-data-table" row-key="id" :data="view.rows" :columns="columns" table-layout="fixed" :hover="true" :pagination="{ current, pageSize, total: view.rows.length, pageSizeOptions: [10, 20, 50], showJumper: false }" @page-change="pageChange">
      <template #id="{ row }"><ResourceId :value="row.id" label="计划ID" /></template>
      <template #identity="{ row }"><ResourceNameLink class="plan-list-name" :title="row.name" data-act="detail" :data-id="row.id">{{ row.name }}</ResourceNameLink></template>
      <template #updated="{ row }">{{formatDateTime(row.updated)}}</template>
      <template #status="{ row }"><Tag :theme="row.statusTheme" variant="light">{{ row.statusLabel }}</Tag><small v-if="row.result">{{ row.result }}</small></template>
      <template #note="{ row }"><button class="plan-list-link plan-list-note" :class="`is-${row.note.type}`" :title="row.note.txt" data-act="focus-progress" :data-id="row.id">{{ row.note.txt }}</button><small class="plan-list-secondary" :title="row.note.reason">{{ row.note.reason }}</small></template>
      <template #progress="{ row }"><div class="plan-list-progress"><span>{{ row.success }} / {{ row.target }}</span><span>{{ row.rate }}%</span></div><Progress :percentage="Math.min(row.rate, 100)" :label="false" :stroke-width="6" color="var(--wt-primary)" /></template>
      <template #batches="{ row }"><span class="plan-list-number">{{ row.ended }} / {{ row.batches }} 批</span></template>
      <template #dates="{ row }"><span class="plan-list-date">{{ row.dateStart }}</span><small class="plan-list-date">{{ row.dateEnd }}</small></template>
      <template #operation="{ row }"><ResourceRowActions><Tooltip v-for="(action, i) in row.actions" :key="action.key" :content="action.reason" :disabled="!action.reason"><span><Button :disabled="action.disabled" :data-batch="action.batch" variant="text" :class="{ 'is-primary': i === 0, 'is-danger': action.danger }" :data-act="action.key" :data-id="row.id">{{ action.label }}</Button></span></Tooltip></ResourceRowActions></template>
      <template #empty><ResourceEmptyState title="没有符合条件的计划" description="调整搜索条件，或使用右上角“新建发布计划”创建计划。"><Button theme="primary" variant="text" data-act="reset-filters">重置筛选</Button></ResourceEmptyState></template>
    </Table>
  </ResourceCard>
</template>

<style scoped>
.plan-list-link { padding: 0; border: 0; background: transparent; color: var(--wt-primary); font: inherit; line-height: 22px; text-align: left; cursor: pointer; }
.plan-list-link:hover { color: var(--wt-primary-hover); }
.plan-list-link:focus-visible { outline: 2px solid var(--wt-primary); outline-offset: 2px; }
.plan-list-name, .plan-list-note, .plan-list-secondary { display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; overflow-wrap: anywhere; }
.plan-list-note.is-alert { color: var(--wt-color-warning-text); }
.plan-list-note.is-complete { color: var(--wt-color-success-text); }
.plan-list-note.is-neutral { color: var(--wt-text-secondary); }
.plan-list-progress { display: flex; justify-content: space-between; gap: 8px; margin-bottom: 6px; font-size: 12px; font-variant-numeric: tabular-nums; }
.plan-list-progress span:last-child { color: var(--wt-text-tertiary); }
.plan-list-number, .plan-list-date { font-variant-numeric: tabular-nums; white-space: nowrap; }
.plan-list-panel :deep(.resource-data-table .t-table__empty) { padding: 0; }
</style>
