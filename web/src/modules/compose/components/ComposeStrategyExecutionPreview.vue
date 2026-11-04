<script setup>
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
  <div class="execution-preview">
    <template v-if="view.active_batch">
      <t-alert theme="info" message="该策略有任务执行中" />
      <div class="execution-preview__setting"><strong>{{ view.active_batch.name || `批次 ${view.active_batch.id}` }}</strong><span v-if="view.active_batch.created_at">{{ formatDateTime(view.active_batch.created_at) }}</span><span>排队 {{ view.active_batch.counts?.pending ?? '—' }} · 执行中 {{ view.active_batch.counts?.running ?? '—' }}</span><t-link theme="primary" @click="$emit('progress', view.active_batch)">查看进度</t-link></div>
    </template>
    <template v-else>
      <strong>{{ view.strategy?.name }}</strong>
      <p v-if="view.selected_count">选入 {{ view.selected_count }} 条素材，预计生成 {{ view.estimated_output_count }} 个成片</p>
      <t-alert v-else theme="info" message="没有可选素材" />
      <p v-if="view.daily_remaining !== null && view.daily_remaining !== undefined">每日剩余 {{ view.daily_remaining }} 条素材</p>
      <div v-for="setting in view.settings || []" :key="setting.key" class="execution-preview__setting"><strong>{{ setting.name }}</strong><span>{{ platformText(setting.platforms) }} · 每条 {{ setting.copies }} 份 · 预计 {{ view.selected_count * setting.copies }} 个成片</span></div>
    </template>
    <div class="execution-preview__materials">
      <div v-for="material in view.materials || []" :key="material.material_id" class="execution-preview__material">
        <div class="execution-preview__heading"><strong>{{ material.title || `素材 ${material.material_id}` }}</strong><t-tag :theme="material.selected ? 'primary' : 'default'" variant="light">{{ material.selected ? '选入' : '跳过' }}</t-tag></div>
        <p>{{ reasons[material.reason] || material.reason }}</p>
        <p v-for="(condition, index) in material.conditions || []" :key="index">{{ conditionText(condition) }}<template v-if="condition.state && !['missing', 'unknown'].includes(condition.state)"> · {{ condition.state === 'matched' ? '满足' : ['unmatched', 'not_matched'].includes(condition.state) ? '不满足' : condition.state }}</template></p>
      </div>
    </div>
  </div>
</template>
<style scoped>
.execution-preview { display: grid; gap: 12px; }
.execution-preview p { margin: 0; color: var(--wt-text-secondary); font-size: 13px; overflow-wrap: anywhere; }
.execution-preview__setting { display: flex; align-items: center; flex-wrap: wrap; gap: 8px 16px; padding: 12px; border: 1px solid var(--wt-border); border-radius: var(--wt-radius-md); }
.execution-preview__setting span { color: var(--wt-text-secondary); }
.execution-preview__materials { display: grid; gap: 12px; max-height: 40vh; overflow: auto; }
.execution-preview__material { display: grid; gap: 6px; padding: 12px; border: 1px solid var(--wt-border); border-radius: var(--wt-radius-md); }
.execution-preview__heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
</style>
