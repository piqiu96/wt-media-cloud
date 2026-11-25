<script setup>
import { computed, ref, onMounted, onBeforeUnmount, useSlots } from "vue";
import ResourceCard from "../../../shared/ui/resource/ResourceCard.vue";
import ResourceSection from "../../../shared/ui/resource/ResourceSection.vue";
import ResourceDescriptions from "../../../shared/ui/resource/ResourceDescriptions.vue";
import ResourceNotice from "../../../shared/ui/resource/ResourceNotice.vue";
import ResourceStatusBadge from "../../../shared/ui/resource/ResourceStatusBadge.vue";
import { batchStatus } from "../batchPresentation.js";
import { configurationLabels } from "../batchPreparation.js";
import BatchExecutionDetail from "./BatchExecutionDetail.vue";
import BatchFormalTasks from "./BatchFormalTasks.vue";
import BatchReadonlyCandidates from "./BatchReadonlyCandidates.vue";
import ResourceEmptyState from "../../../shared/ui/resource/ResourceEmptyState.vue";
import { candidateShortfalls } from "../batchPreparation.js";
import { createPublicationBatchClient } from "../../../shared/api/publication.js";
import {
  useBatchResourceCatalog,
  batchAccountName,
  batchOutputName,
} from "../useBatchResourceCatalog.js";
import { platformLabel } from "../../../shared/utils/platforms.js";
import { formatPublicationTime } from "../dateTime.js";
const props = defineProps({
  batch: Object,
  categoryName: Function,
  taskLink: Function,
  resourceCatalog: Object,
  coverURL: Function,
});
const emit = defineEmits(["tasks"]);
const slots = useSlots();
const catalog = props.resourceCatalog || useBatchResourceCatalog();
const started = computed(() => !!props.batch.execution?.first_started_at);
const cancelledBeforeStart = computed(() => !started.value && props.batch.execution_status === 'cancelled');
const configurationKnown = computed(() => Array.isArray(props.batch.config?.candidates) &&
  (props.batch.candidate_count == null || props.batch.candidate_count === props.batch.config.candidates.length));
const candidateCount = computed(() => configurationKnown.value ? props.batch.config.candidates.length : null);
const target = computed(() => props.batch.targets.reduce((total,row)=>total+row.target_count,0));
const shortfalls = computed(() => configurationKnown.value ? candidateShortfalls(props.batch) : []);
const scope = ref({ page: 1, page_size: 10 }),
  recordPage = ref(1),
  recordTotal = ref(0),
  records = ref([]),
  error = ref("");
let recordSequence = 0;
const recordLabels = {
  "publication_batch.create": "保存批次",
  "publication_batch.configure": "保存配置修改",
  "publication_batch.start": "开始发布",
  "publication_batch.pause": "暂停发布",
  "publication_batch.resume": "继续发布",
  "publication_batch.stop": "终止发布",
  "publication_batch.finish": "结束批次",
  "publication_batch.plan_skeleton": "创建计划批次",
  created: "保存批次",
  configuration_updated: "保存配置修改",
  publication_batch_created: "保存批次",
  publication_batch_configuration_updated: "保存配置修改",
};
async function loadRecords(page = 1) {
  const run = ++recordSequence,
    id = props.batch.id;
  recordPage.value = page;
  try {
    const result = await createPublicationBatchClient().records(id, {
      page,
      page_size: 10,
    });
    if (run === recordSequence && id === props.batch.id) {
      records.value = result.items;
      recordTotal.value = result.total;
    }
  } catch (e) {
    if (run === recordSequence) error.value = e.message;
  }
}
onBeforeUnmount(() => recordSequence++);
onMounted(async () => {
  if (props.resourceCatalog && slots.records) return;
  try {
    await catalog.hydrate(
      {
        accounts: props.batch.config?.accounts || [],
        output_ids: props.batch.config?.output_ids || [],
        candidates: props.batch.config?.candidates || [],
      },
      {
        owner: props.batch.operator_id,
        platform: props.batch.platform,
        readOnly: true,
      },
    );
  } catch (e) {
    error.value = "资源名称暂不可用：" + e.message;
  }
  if (!slots.records) await loadRecords();
});
</script>
<template>
  <ResourceCard
    ><ResourceSection title="批次信息"
      ><ResourceDescriptions
        ><t-descriptions-item
          v-for="item in [
            { label: '批次名称', value: batch.name },
            { label: '批次ID', value: batch.id },
            {
              label: '配置状态',
              value:
                configurationLabels[batch.configuration_status] || '待配置',
            },
            {
              label: '执行状态',
              value: batchStatus(batch),
            },
            { label: '发布平台', value: batch.platform === 'baidu' ? '百家号' : platformLabel(batch.platform) },
            { label: '发布类目', value: batch.targets.map(row=>categoryName(row.category_id)).join('、') || '未选择' },
            { label: '所属计划', value: batch.plan_name || '独立批次' },
            { label: '创建人 ID', value: batch.created_by || '历史记录未提供' },
            {
              label: '创建时间',
              value: formatPublicationTime(batch.created_at),
            },
            {
              label: '最近更新',
              value: formatPublicationTime(batch.updated_at),
            },
          ]"
          :key="item.label"
          :label="item.label"
          ><ResourceStatusBadge v-if="['配置状态','执行状态'].includes(item.label)" :label="item.value"/><template v-else>{{ item.value }}</template></t-descriptions-item
        ></ResourceDescriptions>
      <ResourceNotice v-if="cancelledBeforeStart"><span>此批次在开始发布前已取消。</span><p v-if="batch.execution?.stop_reason">{{batch.execution.stop_reason}}</p></ResourceNotice>
      </ResourceSection
    ></ResourceCard
  ><BatchExecutionDetail
    v-if="started"
    :batch="batch"
    @tasks="emit('tasks', $event)"
  /><ResourceCard v-if="!started"
    ><ResourceSection title="候选配置"
      ><p v-if="batch.execution_status === 'not_started'" class="batch-muted">
        批次尚未开始，正式发布任务 0 条。
      </p>
      <ResourceDescriptions>
        <t-descriptions-item label="发布目标">{{target}} 条</t-descriptions-item>
        <t-descriptions-item label="候选">{{candidateCount === null ? '待刷新' : candidateCount+' 条'}}</t-descriptions-item>
        <t-descriptions-item label="目标缺口">{{candidateCount === null ? '待刷新' : Math.max(0,target-candidateCount)+' 条'}}</t-descriptions-item>
        <t-descriptions-item label="正式发布任务">0 条</t-descriptions-item>
      </ResourceDescriptions>
      <div v-if="configurationKnown && batch.targets.length>1" class="resource-data-table resource-embedded-table is-summary">
        <t-table row-key="category_id" :data="shortfalls" :columns="[{colKey:'category',title:'内容类目'},{colKey:'target_count',title:'目标'},{colKey:'candidate_count',title:'候选'},{colKey:'shortfall',title:'缺口'}]">
          <template #category="{row}">{{categoryName(row.category_id)}}</template>
        </t-table>
      </div>
      <slot name="candidates">
        <ResourceEmptyState v-if="!configurationKnown" title="内容配置暂时无法读取" description="请重新读取批次详情，当前未将缺失内容视为零条。"/>
        <ResourceEmptyState v-else-if="!candidateCount" title="尚未配置发布内容" :description="cancelledBeforeStart ? '批次已取消，不会生成发布任务。' : '该批次尚未保存发布内容；开始发布前需完成资源匹配及内容准备。'"/>
        <BatchReadonlyCandidates v-else :batch="batch" :catalog="catalog" :category-name="categoryName"/>
      </slot>
    </ResourceSection>
    <ResourceSection v-if="!cancelledBeforeStart && candidateCount && batch.readiness?.issues?.some(issue=>!issue.row_key)" title="配置问题"
      ><ul class="batch-issues">
        <li v-for="(issue, i) in batch.readiness.issues.filter(issue=>!issue.row_key)" :key="i">
          {{ issue.message }}
        </li>
      </ul></ResourceSection
    >
    <ResourceSection title="执行设置"
      ><ResourceDescriptions
        ><t-descriptions-item
          v-for="item in [
            {
              label: '并发数',
              value: batch.execution_setting.concurrency_limit,
            },
            {
              label: '组间发布间隔',
              value: batch.execution_setting.group_interval_seconds + '秒',
            },
            {
              label: '组间处理',
              value:
                batch.execution_setting.between_group_policy === 'auto_continue'
                  ? '自动继续'
                  : '异常暂停',
            },
            {
              label: '异常任务上限',
              value: batch.execution_setting.exception_task_limit,
            },
          ]"
          :key="item.label"
          :label="item.label"
          >{{ item.value }}</t-descriptions-item
        ></ResourceDescriptions
      ></ResourceSection
    ></ResourceCard
  ><ResourceCard v-if="batch.execution?.first_started_at"
    ><ResourceSection title="批次任务"
      ><slot name="tasks"
        ><BatchFormalTasks
          :batch="batch"
          :account-name="
            (r) => batchAccountName(catalog.accounts.get(r.media_account_id))
          "
          :output-name="
            (r) => batchOutputName(catalog.outputs.get(r.composite_output_id))
          "
          :scope="scope"
          active
          :task-link="taskLink"
          @scope="scope = $event" /></slot></ResourceSection></ResourceCard
  ><ResourceCard
    ><ResourceSection title="操作记录"
      ><slot name="records"
        ><p v-if="error" role="alert">{{ error }}</p>
        <div v-for="r in records" :key="r.id" class="plan-record">
          {{ formatPublicationTime(r.created_at) }} ·
          {{ recordLabels[r.event] || "批次业务记录" }}
        </div>
        <ResourceNotice v-if="!records.length && !error"
          >暂无操作记录</ResourceNotice
        >
        <t-pagination
          v-if="recordTotal > 10"
          :current="recordPage"
          :page-size="10"
          :total="recordTotal"
          :show-page-size="false"
          @current-change="loadRecords"
        /> </slot></ResourceSection
  ></ResourceCard>
</template>
