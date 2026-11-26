<script setup>
import { usePublicationPlatforms } from "../usePublicationPlatforms.js";
const platforms = usePublicationPlatforms();
// Layout and slots adapted from example/proto/publishing-design/plan/PlanList.vue.
import ResourceNameLink from "../../../shared/ui/resource/ResourceNameLink.vue";
import ResourceId from "../../../shared/ui/resource/ResourceId.vue";
import { formatDateTime } from "../../../shared/utils/datetime.js";
import ResourceStatusBadge from "../../../shared/ui/resource/ResourceStatusBadge.vue";
import { Button, Input, Select, Table, Progress } from "tdesign-vue-next";
import ResourcePageHeader from "../../../shared/ui/resource/ResourcePageHeader.vue";
import ResourceCard from "../../../shared/ui/resource/ResourceCard.vue";
import ResourceFilterBar from "../../../shared/ui/resource/ResourceFilterBar.vue";
import ResourceRowActions from "../../../shared/ui/resource/ResourceRowActions.vue";
import ResourceEmptyState from "../../../shared/ui/resource/ResourceEmptyState.vue";

const props = defineProps({
  rows: Array,
  filters: Object,
  categories: Array,
  page: Number,
  pageSize: Number,
  total: Number,
  loading: Boolean,
  canCreate: Boolean,
  progressLabel: Function,
});
const emit = defineEmits(["open", "query", "reset", "action"]);
const count = (row) =>
  (row.goals || []).reduce((n, g) => n + g.target_count, 0);
const rate = (row) =>
  count(row) ? Math.round((row.published_count / count(row)) * 10000) / 100 : 0;
const states = {
  not_started: "待执行",
  running: "进行中",
  paused: "进行中",
  stopping: "停止中",
  ended: "已结束",
};
const tone = (row) => row.status === "ended"
  ? (row.end_reason === "achieved" ? "success" : "neutral")
  : row.status === "not_started" ? "neutral" : "info";
const ends = {
  achieved: "目标达成",
  unmet: "未达成结束",
  cancelled: "已终止",
  terminated: "已终止",
};
const statuses = Object.entries(states)
  .filter(([value]) => value !== "paused")
  .map(([value, label]) => ({ value, label }));
const operations = (row) => {
  const allowed = row.allowed_actions || [];
  return [
    ...(row.status === 'not_started' && (allowed.includes('start') || allowed.includes('edit')) ? ['start'] : []),
    'detail', ...['edit','cancel','copy'].filter(a => allowed.includes(a)),
  ];
};
const blocked = (row, key) => key !== 'detail' && !(row.allowed_actions || []).includes(key);
const blockReason = (row, key) => blocked(row,key)
  ? row.action_blockers?.find(b=>b.action===key)?.message || '请先完成全部待执行批次配置' : '';
const labels = {
  detail: "详情",
  edit: "编辑",
  start: "执行",
  cancel: "终止",
  copy: "复制",
};
function operation(row, key) {
  if (key === "cancel" || key === "start") emit("action", row, key);
  else emit("open", key === "copy" ? "copy" : key, row.id);
}
const columns = [
  { colKey: "id", title: "计划ID", width: 190 },
  { colKey: "identity", title: "计划名称", width: 240 },
  { colKey: "status", title: "计划状态", width: 100 },
  { colKey: "note", title: "当前进展", width: 230 },
  { colKey: "progress", title: "发布进度", width: 170 },
  { colKey: "batches", title: "批次进度", width: 100 },
  { colKey: "dates", title: "执行周期", width: 130 },
  { colKey: "updated", title: "更新时间", width: 155 },
  { colKey: "operation", title: "操作", width: 216, fixed: "right" },
];
</script>

<template>
  <ResourcePageHeader title="发布计划">
    <template v-if="canCreate" #actions
      ><Button theme="primary" @click="emit('open', 'create')"
        >新建发布计划</Button
      ></template
    >
  </ResourcePageHeader>
  <ResourceCard class="resource-list-panel plan-list-panel">
    <ResourceFilterBar>
      <label class="wt-filter-field wt-filter-field--wide"
        ><span class="wt-filter-field__label">综合搜索</span
        ><Input
          v-model="filters.search"
          placeholder="输入计划名称或 ID"
          clearable
          @enter="emit('query')"
      /></label>
      <label class="wt-filter-field"
        ><span class="wt-filter-field__label">计划状态</span
        ><Select
          v-model="filters.status"
          :options="statuses"
          clearable
          placeholder="全部"
      /></label>
      <label class="wt-filter-field"
        ><span class="wt-filter-field__label">发布平台</span
        ><Select
          v-model="filters.platform"
          :options="platforms.options.value"
          clearable
          placeholder="全部"
      /></label>
      <label class="wt-filter-field"
        ><span class="wt-filter-field__label">发布类目</span
        ><Select
          v-model="filters.category_id"
          :options="
            categories.map((c) => ({ value: String(c.id), label: c.name }))
          "
          filterable
          clearable
          placeholder="全部"
      /></label>
      <div class="wt-filter-actions">
        <Button id="plan-query" theme="primary" @click="emit('query')"
          >查询</Button
        ><Button theme="default" variant="outline" @click="emit('reset')"
          >重置</Button
        >
      </div>
    </ResourceFilterBar>
    <Table
      class="resource-data-table"
      row-key="id"
      :data="rows"
      :loading="loading"
      disable-data-page
      :scroll="{ x: '1530px' }"
      :columns="columns"
      table-layout="fixed"
      :hover="true"
      :pagination="{
        current: page,
        pageSize,
        total,
        pageSizeOptions: [10, 20, 50],
        showJumper: false,
      }"
      @page-change="(p) => emit('query', p.current, p.pageSize)"
    >
      <template #id="{ row }"
        ><ResourceId :value="row.id" label="计划ID"
      /></template>
      <template #identity="{ row }"
        ><ResourceNameLink
          class="plan-list-name"
          :title="row.name"
          @click="emit('open', 'detail', row.id)"
          >{{ row.name }}</ResourceNameLink
        ></template
      >
      <template #updated="{ row }">{{
        formatDateTime(row.updated_at)
      }}</template>
      <template #status="{ row }"
        ><ResourceStatusBadge
          :tone="tone(row)"
          :label="states[row.status] || row.status"
        /><small v-if="row.end_reason">{{
          ends[row.end_reason] || row.end_reason
        }}</small></template
      >
      <template #note="{ row }"
        ><button
          class="plan-list-link plan-list-note"
          :title="progressLabel(row)"
          @click="emit('open', 'detail', row.id)"
        >
          {{ progressLabel(row) }}</button
        ><small class="plan-list-secondary">{{
          row.end_note ||
          (row.current_batch_id
            ? "当前批次 ID：" + row.current_batch_id
            : "按批次顺序推进")
        }}</small></template
      >
      <template #progress="{ row }"
        ><div class="plan-list-progress">
          <span>{{ row.published_count }} / {{ count(row) }}</span
          ><span>{{ rate(row) }}%</span>
        </div>
        <Progress
          :percentage="Math.min(rate(row), 100)"
          :label="false"
          :stroke-width="6"
          color="var(--wt-primary)"
      /></template>
      <template #batches="{ row }"
        ><span class="plan-list-number"
          >{{ row.processed_batch_count }} / {{ row.batch_count }} 批</span
        ></template
      >
      <template #dates="{ row }"
        ><span class="plan-list-date">{{ row.date_start }}</span
        ><small class="plan-list-date">{{ row.date_end }}</small></template
      >
      <template #operation="{ row }"
        ><ResourceRowActions
          ><Button
            v-for="(key, i) in operations(row)"
            :key="key"
            :disabled="blocked(row,key)" :title="blockReason(row,key)"
            variant="text"
            :class="{ 'is-primary': i === 0, 'is-danger': key === 'cancel' }"
            @click="operation(row, key)"
            >{{ labels[key] }}</Button
          ></ResourceRowActions
        ></template
      >
      <template #empty
        ><ResourceEmptyState
          title="没有符合条件的计划"
          description="调整搜索条件，或使用右上角“新建发布计划”创建计划。"
          ><Button theme="primary" variant="text" @click="emit('reset')"
            >重置筛选</Button
          ></ResourceEmptyState
        ></template
      >
    </Table>
  </ResourceCard>
</template>

<style scoped>
.plan-list-link {
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--wt-primary);
  font: inherit;
  line-height: 22px;
  text-align: left;
  cursor: pointer;
}
.plan-list-link:hover {
  color: var(--wt-primary-hover);
}
.plan-list-link:focus-visible {
  outline: 2px solid var(--wt-primary);
  outline-offset: 2px;
}
.plan-list-name,
.plan-list-note,
.plan-list-secondary {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  overflow-wrap: anywhere;
}
.plan-list-note.is-alert {
  color: var(--wt-color-warning-text);
}
.plan-list-note.is-complete {
  color: var(--wt-color-success-text);
}
.plan-list-note.is-neutral {
  color: var(--wt-text-secondary);
}
.plan-list-progress {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 6px;
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}
.plan-list-progress span:last-child {
  color: var(--wt-text-tertiary);
}
.plan-list-number,
.plan-list-date {
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.plan-list-panel :deep(.resource-data-table .t-table__empty) {
  padding: 0;
}
</style>
