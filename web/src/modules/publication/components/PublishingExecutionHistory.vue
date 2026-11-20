<script setup>
import { computed, ref, watch, useId, onMounted, onBeforeUnmount } from 'vue';
import { Button } from 'tdesign-vue-next';
import ResourceNotice from '../../../shared/ui/resource/ResourceNotice.vue';
import ResourceOverlay from '../../../shared/ui/resource/ResourceOverlay.vue';
import ResourceRowActions from '../../../shared/ui/resource/ResourceRowActions.vue';
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue';
import PublishingAttemptDetail from './PublishingAttemptDetail.vue';
import { formatPublicationTime as date } from '../dateTime.js';
import { executionResult as result, executionSummary as summary } from '../executionPresentation.js';
const props = defineProps({ task: { type: Object, required: true }, attempts: { type: Array, default: () => [] } });
const ordered = computed(() => [...props.attempts].sort((a, b) => b.attempt_no - a.attempt_no));
const root = ref(null), selectedKey = ref(null), mounted = ref(false), bounds = ref({}), availableWidth = ref(800);
const key = attempt => String(attempt.id ?? attempt.attempt_no);
const selected = computed(() => ordered.value.find(attempt => key(attempt) === selectedKey.value));
const layerId = `publication-attempt-${useId().replace(/[^a-zA-Z0-9-]/g, '-')}`;
let observer;
function measure() {
  const host = root.value?.closest('.content-area') || document.getElementById('operations-detail-layer');
  if (!host) return;
  const rect = host.getBoundingClientRect();
  bounds.value = { top: `${rect.top}px`, left: `${rect.left}px`, width: `${rect.width}px`, height: `${rect.height}px` };
  availableWidth.value = Math.max(0, Math.min(800, rect.width));
}
onMounted(() => {
  measure(); mounted.value = true;
  const host = root.value?.closest('.content-area') || document.getElementById('operations-detail-layer');
  if (host) { observer = new ResizeObserver(measure); observer.observe(host); }
  window.addEventListener('resize', measure);
});
onBeforeUnmount(() => { observer?.disconnect(); window.removeEventListener('resize', measure); });
watch(() => props.task.id, () => { selectedKey.value = null; });
</script>
<template>
  <div ref="root" class="publication-history">
    <template v-if="!ordered.length">
      <ResourceNotice v-if="task.failure_message" title="任务反馈">{{ task.failure_message }}</ResourceNotice>
      <p class="history-empty">{{ task.status === 'pending' || task.status === 'queued' ? '任务尚未执行' : '暂无已保存的执行记录' }}</p>
    </template>
    <table v-else class="history-table" aria-label="执行历史">
      <colgroup><col class="history-record-col" /><col /><col class="history-action-col" /></colgroup>
      <thead><tr><th scope="col">执行记录</th><th scope="col">执行结果</th><th scope="col">操作</th></tr></thead>
      <tbody><tr v-for="attempt in ordered" :key="key(attempt)">
        <td><span class="history-number">第 {{ attempt.attempt_no }} 次</span><time>开始 {{ date(attempt.started_at) }}</time><time>{{ attempt.finished_at ? '结束' : '最近反馈' }} {{ date(attempt.finished_at || attempt.heartbeat_at) }}</time></td>
        <td><ResourceStatusBadge :label="result(attempt).label" :tone="result(attempt).tone" /><p class="history-summary">{{ summary(attempt) }}</p></td>
        <td><ResourceRowActions><Button variant="text" :aria-label="`查看第 ${attempt.attempt_no} 次执行详情`" @click="selectedKey = key(attempt)">查看执行详情</Button></ResourceRowActions></td>
      </tr></tbody>
    </table>
    <Teleport v-if="mounted" to="body"><div :id="layerId" class="history-drawer-layer" :style="bounds"></div></Teleport>
    <ResourceOverlay v-if="mounted" :visible="!!selected" title="执行详情" kind="drawer" :width="availableWidth" :attach="`#${layerId}`" show-in-attached-element :footer="false" @update:visible="value => { if (!value) selectedKey = null }">
      <PublishingAttemptDetail v-if="selected" :attempt="selected" :task="task" />
    </ResourceOverlay>
  </div>
</template>
<style scoped>
.publication-history { min-width:0; color:var(--wt-text-primary); font-size:14px; line-height:24px; }
.history-table { width:100%; border-collapse:collapse; table-layout:fixed; text-align:left; }
.history-record-col { width:32%; }
.history-action-col { width:136px; }
.history-table th { padding:10px 12px; background:var(--wt-bg-page); font-weight:500; }
.history-table td { padding:16px 12px; vertical-align:top; border-bottom:1px solid var(--wt-border); overflow-wrap:anywhere; }
.history-table tr:last-child td { border-bottom:0; }
.history-number { display:block; font-weight:500; margin-bottom:6px; }
.history-table time { display:block; color:var(--wt-text-tertiary); font-size:13px; font-variant-numeric:tabular-nums; }
.history-summary { margin:6px 0 0; font-size:13px; white-space:pre-wrap; overflow-wrap:anywhere; display:-webkit-box; -webkit-line-clamp:2; -webkit-box-orient:vertical; overflow:hidden; }
.history-empty { margin:0; color:var(--wt-text-tertiary); }
.history-drawer-layer { position:fixed; inset:0; z-index:1800; pointer-events:none; }
@container (max-width: 600px) {
  .history-table colgroup, .history-table thead { display:none; }
  .history-table, .history-table tbody, .history-table tr, .history-table td { display:block; width:100%; box-sizing:border-box; }
  .history-table tr { padding:16px 0; border-bottom:1px solid var(--wt-border); }
  .history-table tr:last-child { border-bottom:0; }
  .history-table td { padding:0; border:0; }
  .history-table td + td { margin-top:12px; }
}
</style>
