<script setup>
import { computed, ref, nextTick } from 'vue';
import { DescriptionsItem as Field } from 'tdesign-vue-next';
import ResourceSection from '../../../src/shared/ui/resource/ResourceSection.vue';
import ResourceDescriptions from '../../../src/shared/ui/resource/ResourceDescriptions.vue';
import ResourceStatusBadge from '../../../src/shared/ui/resource/ResourceStatusBadge.vue';
import ResourceProgress from '../../../src/shared/ui/resource/ResourceProgress.vue';
import ResourceStatGrid from '../../../src/shared/ui/resource/ResourceStatGrid.vue';
import ResourceNotice from '../../../src/shared/ui/resource/ResourceNotice.vue';
import ResourceCard from '../../../src/shared/ui/resource/ResourceCard.vue';
import ResourceFilterBar from '../../../src/shared/ui/resource/ResourceFilterBar.vue';
import ResourceEmptyState from '../../../src/shared/ui/resource/ResourceEmptyState.vue';
import BatchContentTable from "./BatchContentTable.vue";
import TaskRowActions from './TaskRowActions.vue';
import { taskActionItems } from './taskExperience.js';
import { opColumnWidth } from '../../../src/shared/ui/resource/opColumnWidth.js';
import { formatDateTime } from '../../../src/shared/utils/datetime.js';
import { batchState, batchMetrics, rowsFor, isPendingTask, executionChecks } from './batchLifecycle.js';
import '../../../src/shared/styles/resource-data-table.css';
const props=defineProps({db:Object,batch:Object,readOnly:Boolean,search:{type:String,default:''},filter:{type:String,default:''}});
const emit=defineEmits(['task','task-action','update:search','update:filter']);
const batch=computed(()=>props.batch),metrics=computed(()=>batchMetrics(props.db,props.batch));
const rows=computed(()=>rowsFor(props.db,props.batch));
const localSearch=ref(''),localFilter=ref('');
const taskSearch=computed({get:()=>props.readOnly?localSearch.value:props.search,set:value=>{localSearch.value=value;emit('update:search',value)}});
const taskFilter=computed({get:()=>props.readOnly?localFilter.value:props.filter,set:value=>{localFilter.value=value;emit('update:filter',value)}});
const account=id=>props.db.accounts.find(a=>a.id===id)?.name||'原账号不存在';
const asset=id=>props.db.assets.find(a=>a.id===id)?.title||'原成片不存在';
const plan=b=>props.db.plans.find(p=>p.id===b.planId);
const date=formatDateTime;
const stateLabel=computed(()=>{const state=batchState(props.db,props.batch);return state==='草稿'?'待配置':state;});
const successHelp='已确认发布成功的任务数 / 发布目标数。取消、放弃和结果待确认的任务不计入发布成功数。';
const tasks=computed(()=>rows.value.filter(t=>(!taskSearch.value||`${t.id} ${t.title} ${account(t.accountId)}`.includes(taskSearch.value))&&(!taskFilter.value||(taskFilter.value==='待处理'?isPendingTask(t,batch.value):t.status===taskFilter.value))));
const taskSection=ref(null);
function showPending(){taskFilter.value='待处理';nextTick(()=>taskSection.value?.$el?.scrollIntoView({block:'start'}));}
const taskColumns = computed(() => [
  { colKey: "identity", title: "任务 / 发布内容", minWidth: 220 },
  { colKey: "resource", title: "账号 / 成片", minWidth: 175 },
  { colKey: "status", title: "任务状态", width: 115 },
  { colKey: "issue", title: "异常原因", minWidth: 200 },
  ...(!props.readOnly ? [{ colKey: "operation", title: "操作", fixed: "right", width: Math.max(100, ...rows.value.map(t => {
    const a = taskActionItems(t, props.db);
    return opColumnWidth(a.length > 5 ? [...a.slice(0, 4).map(x => x.label), '更多⌄'] : a.map(x => x.label), { variant: 'text', primary: a.some(x => x.primary) });
  })) }] : []),
]);
const history = computed(() =>
  batch.value
    ? [
        ...(batch.value.log || []),
        ...rows.value.flatMap((t) =>
          (t.log || []).map((e) => ({
            ...e,
            type: `任务 T-${t.id}`,
            text: e.text,
            actor: e.actor || "演示执行器",
          })),
        ),
      ]
        .sort((a, b) => String(b.time).localeCompare(String(a.time)))
        .map((entry, index) => ({
          ...entry,
          rowKey: `${batch.value.id}-${index}`,
        }))
    : [],
);
</script>
<template>
    <div v-if="batch" class="batch-detail">
      <ResourceNotice v-if="metrics.pending" tone="warning" class="batch-pending-notice">
        有 {{ metrics.pending }} 条任务待处理
        <template #actions><t-button theme="primary" variant="text" size="small" @click="showPending">查看待处理任务</t-button></template>
      </ResourceNotice>
      <ResourceCard class="detail-card"><ResourceSection title="批次信息">
        <ResourceDescriptions>
          <Field label="批次名称">{{ batch.name }}</Field>
          <Field label="批次ID">{{ batch.id }}</Field>
          <Field label="批次状态"><ResourceStatusBadge :label="stateLabel" :tone="batchState(db, batch) === '待处理' ? 'warning' : batchState(db, batch) === '执行中' ? 'info' : 'neutral'" /></Field>
          <Field label="发布平台">{{ batch.platform }}</Field>
          <Field label="所属计划">{{ plan(batch)?.name || "独立批次" }}</Field>
          <Field label="创建人">{{ batch.createdBy || "历史记录未提供" }}</Field>
          <Field label="创建时间">{{ date(batch.created) }}</Field>
          <Field label="最近更新">{{ date(batch.updatedAt || batch.created) }}</Field>
          <Field v-if="batchState(db, batch) === '已结束'" label="发布结果">{{ metrics.result }}</Field>
        </ResourceDescriptions>
        <ResourceNotice v-if="batch.stoppedAt">已停止发布。未启动任务已取消，在途任务继续回传结果；不能重新启动。</ResourceNotice>
      </ResourceSection></ResourceCard>
      <ResourceCard v-if="batch.preparationOnly" class="detail-card">
        <ResourceSection title="候选配置">
          <ResourceDescriptions>
            <Field label="发布目标">{{ metrics.target }} 条</Field>
            <Field label="候选">{{ batch.candidates.length }} 条</Field>
            <Field label="目标缺口">{{ Math.max(0, metrics.target - batch.candidates.length) }} 条</Field>
            <Field label="正式发布任务">0 条</Field>
          </ResourceDescriptions>
          <BatchContentTable :rows="batch.candidates" :db="db" read-only />
        </ResourceSection>
        <ResourceSection title="执行设置">
          <ResourceDescriptions>
            <Field label="并发数">{{ batch.executionSettings.concurrency }}</Field>
            <Field label="组间发布间隔">{{ batch.executionSettings.groupInterval }} 秒</Field>
            <Field label="组间处理">{{ batch.executionSettings.groupPolicy }}</Field>
            <Field label="异常任务上限">{{ batch.executionSettings.exceptionLimit }}</Field>
          </ResourceDescriptions>
        </ResourceSection>
      </ResourceCard>
      <ResourceCard v-else class="detail-card"><ResourceSection title="执行情况">
        <ResourceStatGrid :items="[
          { key: 'target', label: '发布目标', value: metrics.target },
          { key: 'published', label: '已发布', value: metrics.published, tone: 'success' },
          { key: 'failed', label: '发布失败', value: metrics.failed, tone: metrics.failed ? 'danger' : 'neutral', help: '当前明确失败的任务，不含已取消、已放弃和结果待确认。未处置的失败同时计入待处理。' },
          { key: 'pending', label: '待处理', value: metrics.pending, tone: metrics.pending ? 'warning' : 'neutral' },
        ]" />
        <div class="batch-success-progress">
          <ResourceProgress label="发布成功率" :percentage="metrics.achievement" :value-text="`${metrics.published} / ${metrics.target} ·`" :help="successHelp" />
        </div>
      </ResourceSection></ResourceCard>
      <ResourceCard v-if="!batch.preparationOnly" class="detail-card"><ResourceSection ref="taskSection" id="batch-tasks" title="批次任务">
        <template #title-extra><span class="batch-section-count">{{ rows.length }} 条</span></template>
        <ResourceFilterBar
          ><label class="wt-filter-field wt-filter-field--wide"
            ><span class="wt-filter-field__label">搜索任务</span
            ><t-input
              v-model="taskSearch"
              placeholder="标题、账号或任务编号"
              clearable /></label
          ><label class="wt-filter-field"
            ><span class="wt-filter-field__label">任务状态</span
            ><t-select
              v-model="taskFilter"
              clearable
              placeholder="全部"
              :options="
                [
                  '待处理',
                  '待执行',
                  '排队中',
                  '执行中',
                  '等待人工',
                  '结果待确认',
                  '平台审核中',
                  '待补链接',
                  '执行失败',
                  '已发布',
                  '已取消',
                  '已放弃',
                ].map((x) => ({ label: x, value: x }))
              " /></label
        ></ResourceFilterBar>
        <t-table
          class="resource-data-table resource-embedded-table is-summary"
          :key="`tasks-${batch.id}-${tasks.length > 10}`"
          row-key="id"
          :data="tasks"
          :columns="taskColumns"
          :pagination="
            tasks.length > 10
              ? {
                  defaultCurrent: 1,
                  defaultPageSize: 10,
                  pageSizeOptions: [10, 20, 50],
                  total: tasks.length,
                  showJumper: false,
                }
              : undefined
          "
        >
          <template #identity="{ row }"
            ><div class="batch-task-identity">
              <img
                v-if="db.assets.find((v) => v.id === row.assetId)?.cover"
                :src="db.assets.find((v) => v.id === row.assetId).cover"
                alt="成片封面"
              />
              <div>
                <span v-if="readOnly" class="batch-name" :title="row.title">{{ row.title || "未配置标题" }}</span>
                <button v-else
                  class="batch-link batch-name"
                  :title="row.title"
                  @click="emit('task', row)"
                >
                  {{ row.title || "未配置标题" }}</button><small
                  >T-{{ row.id }} · {{ row.category }}</small
                >
              </div>
            </div></template
          >
          <template #resource="{ row }"
            >{{ account(row.accountId)
            }}<small
              >V-{{ row.assetId }} · {{ asset(row.assetId) }}</small
            ></template
          >
          <template #status="{ row }"
            ><t-tag
              variant="light"
              :theme="
                ['结果待确认', '执行失败', '等待人工'].includes(row.status)
                  ? 'warning'
                  : 'default'
              "
              >{{ row.status === "执行失败" ? "发布失败" : row.status }}</t-tag
            ></template
          >
          <template #issue="{ row }"
            ><span>{{
              ["执行失败", "结果待确认", "等待人工"].includes(row.status)
                ? row.evidence || "缺少执行依据，请在任务详情中核实原尝试"
                : row.status === "平台审核中"
                  ? "平台已接收，等待公开结果，不计入已发布"
                  : row.status === "待执行"
                    ? executionChecks(row, db).join("；") ||
                      (batch.firstStartedAt ? "本次未执行，可单条处理" : "—")
                    : "—"
            }}</span></template
          >
          <template #operation="{ row }"
            ><TaskRowActions :task="row" :db="db" @action="emit('task-action', row, $event)" />
            </template
          >
          <template #empty
            ><ResourceEmptyState
              :title="rows.length ? '没有匹配的任务' : '尚未创建任务'"
              :description="
                rows.length
                  ? '调整搜索或状态筛选。'
                  : '继续配置并完成预检后生成任务。'
              "
          /></template>
        </t-table>
      </ResourceSection></ResourceCard>
      <ResourceCard class="detail-card"><ResourceSection :title="batch.preparationOnly ? '操作记录' : '执行记录'" description="按操作时间倒序">
        <t-table
          class="resource-data-table resource-embedded-table"
          :key="`history-${batch.id}-${history.length > 10}`"
          row-key="rowKey"
          :data="history"
          :columns="[
            { colKey: 'time', title: '操作时间', width: 180 },
            { colKey: 'actor', title: '操作者', width: 130 },
            { colKey: 'type', title: '执行事件', width: 150 },
            { colKey: 'text', title: '结果与异常记录', minWidth: 250 },
          ]"
          :pagination="
            history.length > 10
              ? {
                  defaultCurrent: 1,
                  defaultPageSize: 10,
                  pageSizeOptions: [10, 20, 50],
                  total: history.length,
                  showJumper: false,
                }
              : undefined
          "
          ><template #time="{ row }">{{ date(row.time) }}</template
          ><template #text="{ row }"
            >{{ row.text
            }}<small v-if="row.beforeState"
              >{{ row.beforeState }} → {{ row.afterState }} ·
              {{ row.result || "已记录" }}</small
            ></template
          ><template #empty
            ><ResourceEmptyState
              title="暂无执行记录"
              description="配置保存、执行与异常处理后会记录操作。" /></template
        ></t-table>
      </ResourceSection></ResourceCard>
    </div>
</template>
<style scoped>
small {
  display: block;
  color: var(--wt-color-text-secondary);
  font-size: 12px;
  font-weight: 400;
  line-height: 20px;
  margin-top: 4px;
}
.batch-link {
  padding: 0;
  border: 0;
  background: none;
  text-align: left;
  color: var(--wt-color-primary);
  font: inherit;
  cursor: pointer;
  line-height: 1.6;
}
.batch-link:hover {
  text-decoration: underline;
}
.batch-link:focus-visible {
  outline: 2px solid var(--wt-color-primary);
  outline-offset: 3px;
  border-radius: 4px;
}
.batch-detail { display: flex; flex-direction: column; gap: 16px; min-width: 0; }
.batch-detail > .resource-notice { margin: 0; }
.batch-detail :deep(.resource-section .resource-filter-bar) { padding: 16px 0; gap: 16px; margin: 0; }
.batch-detail :deep(.resource-section .resource-filter-bar > .wt-filter-field) { flex: 0 1 180px; max-width: 180px; }
.batch-detail :deep(.resource-section .resource-filter-bar > .wt-filter-field--wide) { flex: 0 1 320px; max-width: 360px; }
.batch-detail :deep(.resource-section .resource-filter-bar .wt-filter-field__label) { font-size: 13px; font-weight: 400; color: var(--wt-text-tertiary); }
.batch-detail :deep(.resource-data-table .t-table__empty) { padding: 0; }
.batch-detail .batch-name { overflow-wrap: anywhere; max-width: 280px; font-weight: 400; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; text-align: left; }
.batch-detail :deep(.t-alert) {
  margin-bottom: 20px;
  font-size: 13px;
  line-height: 1.7;
}
.batch-section-count { color: var(--wt-text-tertiary); font-size: 12px; font-weight: 400; margin-left: 6px; }
.batch-task-identity {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}
.batch-task-identity > img {
  width: 38px;
  height: 50px;
  object-fit: cover;
  flex: none;
  border-radius: 4px;
  background: var(--wt-color-bg-subtle);
}
.batch-task-identity > div {
  min-width: 0;
}
.batch-pending-notice { align-items: center; }
.batch-pending-notice :deep(.resource-notice__actions .t-button) { font-size: 13px; }
@media (max-width: 680px) { .batch-detail :deep(.resource-section .resource-filter-bar > .wt-filter-field) { flex-basis: 100%; max-width: none; } }
</style>
