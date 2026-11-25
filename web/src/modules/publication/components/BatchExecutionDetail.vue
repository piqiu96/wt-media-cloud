<script setup>
import { computed } from "vue";
import ResourceStatGrid from "../../../shared/ui/resource/ResourceStatGrid.vue";
import ResourceCard from "../../../shared/ui/resource/ResourceCard.vue";
import ResourceSection from "../../../shared/ui/resource/ResourceSection.vue";
import ResourceDescriptions from "../../../shared/ui/resource/ResourceDescriptions.vue";
import ResourceProgress from "../../../shared/ui/resource/ResourceProgress.vue";
import ResourceStatusBadge from "../../../shared/ui/resource/ResourceStatusBadge.vue";
import FeedbackAlert from "../../../shared/ui/feedback/FeedbackAlert.vue";
import { formatPublicationTime } from "../dateTime.js";
import { batchPhases, batchProgress, batchPendingCount } from "../batchExecution.js";

const props = defineProps({ batch: { type: Object, required: true } });
const emit = defineEmits(["tasks"]);
const execution = computed(() => props.batch.execution);
const counts = computed(() => batchProgress(execution.value));
const pendingCount = computed(() => batchPendingCount(props.batch));
const currentGroup = computed(() => execution.value?.group_task_ids?.length || 0);
const phase = computed(() => execution.value?.phase);
const checkedCounts = computed(() => [
  { status: "failed", label: "发布失败", count: execution.value?.task_counts?.failed || 0 },
  { status: "awaiting_human", label: "等待人工", count: execution.value?.task_counts?.awaiting_human || 0 },
  { status: "uncertain", label: "结果待确认", count: execution.value?.task_counts?.uncertain || 0 },
]);
</script>

<template>
  <ResourceCard v-if="counts && (execution.first_started_at || execution.stopped_at)" class="detail-card">
    <ResourceSection title="执行情况">
      <ResourceDescriptions>
        <t-descriptions-item label="执行阶段"><ResourceStatusBadge :label="batchPhases[phase] || '等待状态更新'" /></t-descriptions-item>
        <t-descriptions-item v-if="execution.stopped_at" label="终止时间">{{ formatPublicationTime(execution.stopped_at) }}</t-descriptions-item>
        <t-descriptions-item v-if="execution.first_started_at" label="首次开始">{{ formatPublicationTime(execution.first_started_at) }}</t-descriptions-item>
        <t-descriptions-item v-if="execution.execution_group_no" label="执行组">第 {{ execution.execution_group_no }} 组</t-descriptions-item>
        <t-descriptions-item v-if="currentGroup" label="本组已安排">{{ currentGroup }} 条</t-descriptions-item>
        <t-descriptions-item v-if="execution.group_ended_at" label="本组自动处理结束">{{ formatPublicationTime(execution.group_ended_at) }}</t-descriptions-item>
        <t-descriptions-item v-if="execution.next_group_at && !execution.stopped_at && !execution.ended_at" label="最早下一组">
          {{ formatPublicationTime(execution.next_group_at) }}
        </t-descriptions-item>
        <t-descriptions-item label="并发数">{{batch.execution_setting.concurrency_limit}}</t-descriptions-item>
        <t-descriptions-item label="组间发布间隔">{{batch.execution_setting.group_interval_seconds}} 秒</t-descriptions-item>
        <t-descriptions-item label="组间处理">{{batch.execution_setting.between_group_policy==='auto_continue'?'自动继续':'异常暂停'}}</t-descriptions-item>
        <t-descriptions-item label="异常任务上限">{{batch.execution_setting.exception_task_limit}}</t-descriptions-item>
        <t-descriptions-item label="正式发布任务">{{ counts.created }} 条</t-descriptions-item>
        <t-descriptions-item v-if="counts.creationShortfall != null" label="创建缺口">{{ counts.creationShortfall }} 条</t-descriptions-item>
        <t-descriptions-item label="待平台公开">{{ counts.reviewing }} 条</t-descriptions-item>
        <t-descriptions-item v-if="execution.ended_at" label="结束时间">{{ formatPublicationTime(execution.ended_at) }}</t-descriptions-item>
      </ResourceDescriptions>
      <FeedbackAlert v-if="execution.pause" theme="warning" :message="execution.pause.reason" />
      <FeedbackAlert v-if="execution.stopped_at" :message="execution.stop_reason || (phase === 'stopping' ? '后续派发已终止，在途任务等待原尝试结果。' : '后续派发已终止。')" />
      <FeedbackAlert v-if="execution.finish_reason" :message="execution.finish_reason" />
      <ResourceStatGrid :items="[
        {key:'target',label:'发布目标',value:counts.target},
        {key:'published',label:'已发布',value:counts.published,tone:'success'},
        {key:'failed',label:'发布失败',value:execution.task_counts?.failed||0,tone:execution.task_counts?.failed?'danger':'neutral'},
        {key:'pending',label:'待处理',value:pendingCount===null?'待刷新':pendingCount,tone:pendingCount===0?'neutral':'warning'},
      ]"/>
      <div v-if="execution.first_started_at" class="batch-execution-progress">
        <ResourceProgress label="公开发布率" :percentage="counts.rate"
          :value-text="counts.published + ' / ' + counts.target + ' ·'"
          help="以原发布目标为分母，只计已核实公开并关联作品的正式任务。" />
        <small class="batch-muted">公开目标差额 {{ counts.targetShortfall }} 条</small>
      </div>
    </ResourceSection>
    <ResourceSection v-if="counts.exceptions && ['waiting_interval','awaiting_processing','paused','stopping'].includes(phase)" title="待处理任务">
      <div class="batch-inline">
        <t-button v-for="item in checkedCounts.filter(row => row.count)" :key="item.status"
          theme="default" variant="outline" size="small" @click="emit('tasks', item.status)">
          {{ item.label }} {{ item.count }}
        </t-button>
      </div>
    </ResourceSection>
  </ResourceCard>
</template>
