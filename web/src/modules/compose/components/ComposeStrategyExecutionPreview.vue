<script setup>
import ComposeBatchProgress from './ComposeBatchProgress.vue'
import ResourceDescriptions from '../../../shared/ui/resource/ResourceDescriptions.vue'
import ResourceSection from '../../../shared/ui/resource/ResourceSection.vue'
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'
import { platformText } from '../strategySettings.js'
import { formatDateTime } from '../../../shared/utils/datetime.js'
defineProps({ view: { type: Object, required: true } })
defineEmits(['progress'])
const reasons = { matched: '符合选入规则', filtered: '命中排除规则', no_match: '未命中选入规则', metrics_missing: '缺少判断所需指标', other_category: '不属于策略类目', material_unavailable: '素材不可用', previously_run: '本策略已执行', per_run_limit: '超出每次素材上限', daily_limit: '每日额度已用完', task_create_failed: '任务创建失败', operator_unavailable: '负责人不可用', template_unavailable: '模板不可用' }
const fieldLabels = { published_at: '发布时间', like_count: '点赞量', favorite_count: '收藏量' }
const operatorLabels = { gte: '≥', lte: '≤', eq: '=', lt: '<' }
function conditionText(condition) {
  const field = fieldLabels[condition.field] || condition.field
  if (condition.actual === undefined || condition.actual === null) return `缺少${field}`
  const actual = condition.field === 'published_at' ? formatDateTime(condition.actual) : Number(condition.actual).toLocaleString('zh-CN')
  if (['recent_hours', 'recent_days'].includes(condition.operator)) return `${field} ${actual}，近 ${condition.expected} ${condition.operator === 'recent_hours' ? '小时' : '天'}`
  const expected = condition.field === 'published_at' ? formatDateTime(condition.expected) : Number(condition.expected).toLocaleString('zh-CN')
  return `${field} ${actual} ${operatorLabels[condition.operator] || condition.operator} ${expected}`
}
</script>
<template>
  <div class="execution-preview wt-resource-page resource-visual-approved production-page">
    <template v-if="view.active_batch">
      <t-alert theme="info" message="该策略有任务执行中" />
      <ResourceDescriptions><t-descriptions-item label="任务批次">{{ view.active_batch.name || view.active_batch.id }}</t-descriptions-item><t-descriptions-item label="创建时间">{{ formatDateTime(view.active_batch.created_at) }}</t-descriptions-item></ResourceDescriptions>
      <ComposeBatchProgress :counts="view.active_batch.counts || {}" />
      <t-link theme="primary" @click="$emit('progress', view.active_batch)">查看详情</t-link>
    </template>
    <template v-else>
      <div class="execution-preview__summary"><span>本次素材 <strong>{{ view.selected_count }} 条</strong></span><span>预计成片 <strong>{{ view.estimated_output_count }} 个</strong></span></div>
      <t-alert v-if="!view.selected_count" theme="info" message="没有可选素材" />
      <ResourceDescriptions><t-descriptions-item label="策略">{{ view.strategy?.name }}</t-descriptions-item><t-descriptions-item v-if="view.daily_remaining !== null && view.daily_remaining !== undefined" label="每日剩余">{{ view.daily_remaining }} 条</t-descriptions-item></ResourceDescriptions>
      <ResourceSection title="成片设置"><div class="table-scroll-wrap"><t-table class="wt-resource-table" :data="view.settings || []" :columns="[{colKey:'name',title:'成片设置',width:240},{colKey:'platforms',title:'适用平台',width:180},{colKey:'copies',title:'每条份数',width:100},{colKey:'estimate',title:'预计成片',width:110}]" row-key="key" size="small"><template #platforms="{row}">{{ platformText(row.platforms) }}</template><template #estimate="{row}">{{ view.selected_count * row.copies }} 个</template></t-table></div></ResourceSection>
      <ResourceSection title="选材结果"><div class="table-scroll-wrap"><t-table class="wt-resource-table" :data="view.materials || []" :columns="[{colKey:'title',title:'素材',width:240},{colKey:'result',title:'结果',width:100},{colKey:'basis',title:'依据',width:320}]" :scroll="{x:'660px'}" row-key="material_id" size="small" empty="暂无候选素材">
        <template #title="{row}"><div class="execution-preview__material"><strong :title="row.title">{{ row.title || `素材 ${row.material_id}` }}</strong><span>ID {{ row.material_id }}</span></div></template>
        <template #result="{row}"><ResourceStatusBadge :tone="row.selected ? 'success':'neutral'" :label="row.selected ? '选入':'跳过'" /></template>
        <template #basis="{row}"><div class="execution-preview__basis"><span>{{ reasons[row.reason] || row.reason }}</span><span v-for="(condition,index) in row.conditions || []" :key="index">{{ conditionText(condition) }}<template v-if="condition.state && !['missing','unknown'].includes(condition.state)"> · {{ condition.state === 'matched' ? '满足' : ['unmatched','not_matched'].includes(condition.state) ? '不满足' : condition.state }}</template></span></div></template>
      </t-table></div></ResourceSection>
    </template>
  </div>
</template>
<style scoped>
.execution-preview { display:grid;gap:20px;min-width:0; }
.execution-preview__summary { display:flex;gap:40px;flex-wrap:wrap;padding:20px;background:var(--wt-bg-page);border-radius:var(--wt-radius-sm); }
.execution-preview__summary > span { display:flex;align-items:center;gap:12px;font-size:14px; }
.execution-preview__summary strong { color:var(--wt-text-primary);font-size:22px;font-weight:600; }
.execution-preview__material,.execution-preview__basis { display:grid;gap:4px;min-width:0; }
.execution-preview__material strong { white-space:nowrap;overflow:hidden;text-overflow:ellipsis;font-weight:500; }
.execution-preview__material span,.execution-preview__basis span:not(:first-child) { color:var(--wt-text-tertiary);font-size:12px; }
.execution-preview :deep(.resource-section) { margin-top:0; }
</style>
