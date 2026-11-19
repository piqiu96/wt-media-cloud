<script setup>
import { computed } from 'vue';
import { DescriptionsItem as Field } from 'tdesign-vue-next';
import { ImageIcon } from 'tdesign-icons-vue-next';
import ResourceSection from '../../../shared/ui/resource/ResourceSection.vue';
import ResourceDescriptions from '../../../shared/ui/resource/ResourceDescriptions.vue';
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue';
import { formatPublicationTime as date } from '../dateTime.js';
import { executionResult, executionTimeline, executionVerification, executionCoverName } from '../executionPresentation.js';
const props = defineProps({ attempt: { type: Object, required: true }, task: { type: Object, required: true } });
const result = computed(() => executionResult(props.attempt));
const timeline = computed(() => executionTimeline(props.attempt));
const verification = computed(() => executionVerification(props.attempt));
const content = computed(() => props.attempt.input_snapshot?.content);
const receipt = computed(() => props.attempt.evidence?.evidence?.receipt_id || props.attempt.evidence?.platform_work_id);
</script>
<template>
  <div class="publication-attempt-detail">
    <ResourceSection title="执行结果">
      <div class="attempt-result"><ResourceStatusBadge :label="result.label" :tone="result.tone" /><span>第 {{ attempt.attempt_no }} 次执行</span></div>
      <ResourceDescriptions>
        <Field label="开始时间" full>{{ date(attempt.started_at) }}</Field>
        <Field :label="attempt.finished_at ? '结束时间' : '最近反馈'" full>{{ date(attempt.finished_at || attempt.heartbeat_at) }}</Field>
        <Field v-if="receipt" label="平台回执" full>{{ receipt }}</Field>
      </ResourceDescriptions>
    </ResourceSection>
    <ResourceSection title="执行过程">
      <ol v-if="timeline.length" class="attempt-timeline">
        <li v-for="event in timeline" :key="event.key" :class="`is-${event.tone}`"><time>{{ date(event.at) }}</time><span>{{ event.label }}</span></li>
      </ol>
      <p v-else class="attempt-empty">暂无已保存的执行过程</p>
    </ResourceSection>
    <ResourceSection v-if="attempt.resolution_note" title="人工核实">
      <ResourceDescriptions v-if="verification">
        <Field label="核实结论" full>{{ verification.conclusion === 'failed' && attempt.resolved_at ? '已核实未发布' : '仍无法判断' }}</Field>
        <Field label="核实时间" full>{{ date(attempt.resolved_at) }}</Field>
        <Field label="核实说明" full>{{ verification.evidence?.reason || '—' }}</Field>
        <Field label="核实依据" full>{{ verification.evidence?.basis || '—' }}</Field>
      </ResourceDescriptions>
      <p v-else class="attempt-text">{{ attempt.resolution_note }}</p>
    </ResourceSection>
    <ResourceSection title="发布内容">
      <ResourceDescriptions v-if="content">
        <Field v-if="content.title" label="标题" full>{{ content.title }}</Field>
        <Field :label="content.title ? '正文 / 简介' : '发布标题'" full>{{ content.description || '—' }}</Field>
        <Field label="发布封面" full><img v-if="content.cover" :src="content.cover" alt="发布封面" class="attempt-cover-image" /><span v-if="!content.cover || content.cover_file_id" class="attempt-cover"><ImageIcon aria-hidden="true" />{{ executionCoverName(attempt, task) }}</span></Field>
        <Field label="话题" full>{{ content.topics || '—' }}</Field>
        <Field label="创作声明" full>{{ content.declaration || '—' }}</Field>
        <Field v-if="content.visibility" label="可见范围" full>{{ content.visibility }}</Field>
        <Field v-if="content.partition" label="分区" full>{{ content.partition }}</Field>
      </ResourceDescriptions>
      <p v-else class="attempt-empty">该次执行未保存发布内容快照</p>
    </ResourceSection>
  </div>
</template>
<style scoped>
.publication-attempt-detail { min-width: 0; }
.attempt-result { display:flex; align-items:center; flex-wrap:wrap; gap:8px; margin-bottom:8px; }
.attempt-result > span:last-child { color:var(--wt-text-tertiary); font-size:13px; }
.attempt-timeline { list-style:none; margin:0; padding:0; }
.attempt-timeline li { position:relative; display:grid; grid-template-columns:144px minmax(0,1fr); gap:16px; padding:0 0 16px 20px; margin-left:4px; border-left:1px solid var(--wt-border); }
.attempt-timeline li:last-child { padding-bottom:0; border-left-color:transparent; }
.attempt-timeline li::before { content:''; position:absolute; left:-4px; top:8px; width:7px; height:7px; border-radius:50%; background:var(--wt-success); }
.attempt-timeline .is-danger::before { background:var(--wt-danger); }
.attempt-timeline .is-warning::before { background:var(--wt-warning); }
.attempt-timeline .is-info::before { background:var(--wt-primary); }
.attempt-timeline time { color:var(--wt-text-tertiary); font-size:13px; font-variant-numeric:tabular-nums; }
.attempt-timeline span, .attempt-text { white-space:pre-wrap; overflow-wrap:anywhere; }
.attempt-cover-image { display:block; max-width:100%; width:160px; max-height:120px; object-fit:contain; }
.attempt-cover { display:inline-flex; align-items:center; gap:8px; }
.attempt-cover :deep(svg) { flex-shrink:0; color:var(--wt-text-tertiary); }
.attempt-empty, .attempt-text { margin:0; }
.attempt-empty { color:var(--wt-text-tertiary); }
@container (max-width: 480px) { .attempt-timeline li { grid-template-columns:1fr; gap:4px; } }
</style>
