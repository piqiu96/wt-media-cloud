<script setup>
import { computed } from "vue";
import PublishingExecutionHistory from "../../../src/modules/publication/components/PublishingExecutionHistory.vue";
import { Button, DescriptionsItem as Field } from "tdesign-vue-next";
import ResourceCard from "../../../src/shared/ui/resource/ResourceCard.vue";
import ResourceSection from "../../../src/shared/ui/resource/ResourceSection.vue";
import ResourceDescriptions from "../../../src/shared/ui/resource/ResourceDescriptions.vue";
import ResourceStatusBadge from "../../../src/shared/ui/resource/ResourceStatusBadge.vue";
import ResourceNotice from "../../../src/shared/ui/resource/ResourceNotice.vue";
import ResourceNameLink from "../../../src/shared/ui/resource/ResourceNameLink.vue";
import { publishMethodLabel } from "./publishingSettings.js";
import { taskContent, taskTitle, displayStatus } from "./taskExperience.js";
import { publicWork } from "./taskActions.js";
import { coverTextFields } from "./covers.js";
const props = defineProps({ task: Object, db: Object });
const emit = defineEmits(["batch", "preview", "account"]);
const account = computed(() =>
  props.db.accounts.find((a) => a.id === props.task.accountId),
);
const sourceAsset = computed(() =>
  props.db.assets.find((asset) => asset.id === props.task.assetId),
);
const batch = computed(() =>
  props.db.batches.find((b) => b.id === props.task.batchId),
);
const realStatus = status => ({ '待执行':'pending', '排队中':'queued', '执行中':'running', '等待人工':'awaiting_human', '结果待确认':'uncertain', '平台审核中':'reviewing', '执行失败':'failed', '已发布':'published' })[status] || 'uncertain';
const historyTask = computed(() => ({ id: props.task.id, status: realStatus(props.task.status), failure_message: props.task.evidence }));
const historyAttempts = computed(() => (props.task.attemptHistory || []).map(a => ({
  id: String(a.number), attempt_no: a.number, started_at: a.started, finished_at: a.ended,
  status: a.ended ? 'completed' : 'running', result_status: realStatus(a.status),
  stage: a.ended ? 'finished' : 'preparing', error_message: a.receipt,
  input_snapshot: a.snapshot ? { content: { title: props.task.platform !== '百家号' ? a.snapshot.title : null, description: a.snapshot.description || a.snapshot.title, cover: a.snapshot.cover, cover_file_id: a.snapshot.coverFileId, visibility: a.snapshot.visibility, partition: a.snapshot.partition, topics: a.snapshot.topics || a.snapshot.tags, declaration: a.snapshot.declaration } } : null,
  steps: (a.steps || []).map((step, i) => ({ seq:i + 1, at:step.time, label:[displayStatus(step), step.evidence].filter(Boolean).join(' · '), report:{ stage: step.status === '等待人工' ? 'awaiting_human' : undefined } })),
})));
const content = computed(() => taskContent(props.task));
const coverFields = computed(() =>
  (content.value.templateDocument
    ? coverTextFields(content.value.templateDocument)
    : [{ key: "shortTitle", label: "封面主标题" }]
  ).filter((field) => content.value[field.key]),
);
const work = computed(() => {
  try {
    return publicWork(
      props.task.platform,
      props.task.workUrl || props.task.evidence || "",
    ).url;
  } catch {
    return "";
  }
});
const date = (value) => {
  if (!value) return "尚无记录";
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime())
    ? "尚无记录"
    : parsed.toLocaleString("zh-CN", { year: "numeric", month: "numeric", day: "numeric", hour: "2-digit", minute: "2-digit", hour12: false });
};
const statusTone = computed(() => taskTone(props.task.status));
function taskTone(status) {
  return status === '已发布' ? 'success' : status === '执行失败' ? 'danger'
    : ['结果待确认', '等待人工', '平台审核中'].includes(status) ? 'warning'
    : ['执行中', '排队中'].includes(status) ? 'info' : 'neutral';
}
const attention = computed(() => {
  const t = props.task;
  if (t.status === '等待人工') return t.evidence || '任务等待人工确认，请返回任务列表继续处理。';
  if (t.status === '结果待确认') return '平台结果尚未确认，请返回任务列表核实结果，避免重复发布。';
  if (t.status === '执行失败') return `发布失败${t.failedStep ? ' · ' + t.failedStep : ''}。请在任务列表查看可用的处理操作。`;
  return '';
});
</script>
<template>
  <article class="task-detail">
    <ResourceNotice v-if="attention" tone="warning">{{ attention }}</ResourceNotice>
    <ResourceCard class="detail-card"><ResourceSection title="任务信息">
      <ResourceDescriptions>
        <Field label="任务名称">{{ taskTitle(task) }}</Field>
        <Field label="任务 ID">{{ task.id }}</Field>
        <Field label="任务状态"><ResourceStatusBadge :label="displayStatus(task)" :tone="statusTone" /></Field>
        <Field label="平台 / 类目">{{ task.platform }} · {{ task.category }}</Field>
        <Field label="发布账号"><ResourceNameLink v-if="account" :aria-label="`查看发布账号 ${account.name}`" @click="emit('account', account)">{{ account.name }}</ResourceNameLink><span v-else>账号记录缺失</span></Field>
        <Field label="发布方式">{{ publishMethodLabel }}</Field>
        <Field label="来源成片"><Button v-if="sourceAsset" class="task-entity-link" variant="text" :aria-label="`查看来源成片 ${sourceAsset.id}`" @click="emit('preview', { asset: sourceAsset })">{{ sourceAsset.id }}</Button><span v-else>{{ task.assetId }} · 成片记录缺失</span></Field>
        <Field label="所属批次"><Button v-if="batch" class="task-entity-link" variant="text" @click="emit('batch', batch)">{{ batch.name }}</Button><span v-else>独立任务</span></Field>
        <Field label="创建时间">{{ date(task.createdAt || task.created) }}</Field>
        <Field label="最近更新">{{ date(task.updatedAt || task.verifiedAt || task.log?.at(-1)?.time) }}</Field>
      </ResourceDescriptions>
    </ResourceSection></ResourceCard>
    <ResourceCard class="detail-card"><ResourceSection title="发布内容">
      <ResourceDescriptions>
        <Field v-if="task.platform !== '百家号'" label="标题" full>{{ content.title || '未填写' }}</Field>
        <Field :label="task.platform === '百家号' ? '发布标题' : '正文 / 简介'" full>{{ content.description || "未填写" }}</Field>
        <Field label="发布封面" full>
          <div class="task-cover-row">
            <button v-if="content.cover" class="task-cover" aria-label="预览发布封面" @click="emit('preview', { src: content.cover, title: '发布封面' })"><img :src="content.cover" alt="发布封面" /></button>
            <span v-else>暂无封面</span>
            <dl class="task-cover-meta"><div><dt>封面方式</dt><dd>{{ content.coverTreatment || content.coverMode || "素材原封面" }}</dd></div><div v-for="field in coverFields" :key="field.key"><dt>{{ field.label }}</dt><dd>{{ content[field.key] }}</dd></div></dl>
          </div>
        </Field>
        <Field :label="task.platform === 'B站' ? '标签' : '话题'" full>{{ (task.platform === "小红书" ? content.topics : content.tags) || "未填写" }}</Field>
        <Field label="创作声明">{{ (content.declaration || "未填写").replace("（原型设置）", "") }}</Field>
        <Field label="可见范围">{{ content.visibility || "公开" }}</Field>
        <Field v-if="task.platform === 'B站'" label="分区">{{ content.partition || "未填写" }}</Field>
      </ResourceDescriptions>
    </ResourceSection></ResourceCard>
    <ResourceCard class="detail-card"
      v-if="
        [
          '已发布',
          '平台审核中',
          '结果待确认',
          '执行失败',
          '已取消',
          '已放弃',
        ].includes(task.status)
      "
    >
    <ResourceSection title="平台记录与关联作品">

      <p v-if="work">
        <a :href="work" target="_blank" rel="noopener noreferrer">{{ work }}</a>
      </p>
      <p v-else>
        {{
          task.status === "已发布"
            ? "历史记录缺少有效作品链接，无法打开作品。"
            : "尚无公开作品链接。"
        }}
      </p>
      <p v-if="task.workId">已关联作品 #{{ task.workId }}</p>
      <p v-if="task.publicAt">公开时间：{{ date(task.publicAt) }}</p>
      <p v-if="task.cancelReason">终止原因：{{ task.cancelReason }}</p>
      <p v-if="task.evidence">平台反馈：{{ task.evidence }}</p>
      <details v-if="task.verifications?.length">
        <summary>核实记录</summary>
        <p v-for="(v, i) in task.verifications" :key="i">
          {{ v.actor }} · {{ date(v.at) }} · 尝试 {{ v.attempt }} ·
          {{ displayStatus({ status: v.status }) }} · {{ v.evidence }}
        </p>
      </details>
    </ResourceSection>
    </ResourceCard>
    <ResourceCard class="detail-card"><ResourceSection title="执行历史">
      <PublishingExecutionHistory v-if="historyAttempts.length || task.status === '待执行'" :task="historyTask" :attempts="historyAttempts" />
      <p v-else>暂无已保存的执行记录</p>
    </ResourceSection></ResourceCard>
  </article>
</template>
<style scoped>
.task-detail { display: flex; flex-direction: column; gap: 20px; min-width: 0; }
.task-detail > .resource-notice { margin: 0; }
.task-detail p { margin: 8px 0; overflow-wrap: anywhere; }
.task-detail a, .task-detail summary { color: var(--wt-primary); overflow-wrap: anywhere; }
.task-entity-link { height: auto; min-height: 24px; padding: 0; margin: 0 !important; color: var(--wt-primary); font-size: 14px; white-space: normal; text-align: left; }
.task-entity-link :deep(.t-button__text) { white-space: normal; overflow-wrap: anywhere; }
.task-cover-row { display: flex; align-items: flex-start; gap: 20px; white-space: normal; }
.task-cover { width: 180px; flex: 0 0 180px; border: 1px solid var(--wt-border); border-radius: var(--wt-radius-md); padding: 0; background: var(--wt-bg-page); cursor: zoom-in; overflow: hidden; }
.task-cover img { display: block; width: 100%; max-height: 180px; object-fit: contain; }
.task-cover:focus-visible { outline: 2px solid var(--wt-primary); outline-offset: 3px; }
.task-cover-meta { margin: 0; min-width: 0; font-size: 13px; }
.task-cover-meta > div { display: flex; flex-wrap: wrap; gap: 4px 12px; margin-bottom: 8px; }
.task-cover-meta dt { color: var(--wt-text-tertiary); }
.task-cover-meta dd { margin: 0; overflow-wrap: anywhere; }
.task-detail details { padding: 12px 0; border-bottom: 1px solid var(--wt-border); font-size: 13px; }
.task-detail summary { cursor: pointer; }
.v4-history p { display: grid; grid-template-columns: 160px minmax(0, 1fr); gap: 16px; padding: 10px 0; border-bottom: 1px solid var(--wt-border); font-size: 13px; }
.v4-history time { color: var(--wt-text-tertiary); }
@media (max-width: 680px) { .task-cover-row { flex-direction: column; gap: 12px; } .task-cover { flex-basis: auto; max-width: 100%; } .v4-history p { grid-template-columns: 1fr; gap: 4px; } }
</style>
