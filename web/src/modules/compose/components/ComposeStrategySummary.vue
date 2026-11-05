<script setup>
import { platformText, settingFields, strategySettings } from '../strategySettings.js'
import { scheduleLabel } from '../scheduleForm.js'
import ResourceSection from '../../../shared/ui/resource/ResourceSection.vue'
import ResourceDescriptions from '../../../shared/ui/resource/ResourceDescriptions.vue'
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'
import { templateLabel } from '../strategyConfig.js'
import { formatDateTime } from '../../../shared/utils/datetime.js'

// 策略管理与任务内的关联策略使用同一份只读详情，不展示带签名的视频地址。
const props = defineProps({
  strategy: { type: Object, required: true },
  team: { type: String, default: '' },
  category: { type: String, default: '' },
  templates: { type: Array, default: () => [] },
  overview: { type: Boolean, default: false },
  showConfig: { type: Boolean, default: true },
})
const ruleColumns = [
  { colKey: 'priority', title: '优先级', width: 80 },
  { colKey: 'action', title: '命中后', width: 80 },
  { colKey: 'logic', title: '条件关系', width: 100 },
  { colKey: 'conditions', title: '条件', width: 400 },
]
const conditionLabels = { published_at: '发布时间', like_count: '点赞量', favorite_count: '收藏量' }
function conditionText(condition) {
  const field = conditionLabels[condition.field] || condition.field
  if (['recent_hours', 'recent_days'].includes(condition.operator)) {
    return `${field} 最近 ${condition.value} ${condition.operator === 'recent_hours' ? '小时' : '天'}`
  }
  const operator = (condition.field === 'published_at'
    ? { gte: '不早于', lt: '早于' }
    : { eq: '等于', gte: '大于等于', lte: '小于等于' })[condition.operator] || condition.operator
  const value = condition.field === 'published_at' ? formatDateTime(condition.value) : condition.value
  return `${field} ${operator} ${value}`
}
function orderedRules() { return [...(props.strategy.priority_rules || [])].sort((a, b) => a.priority - b.priority) }
function parameters(setting) {
  const template = props.templates.find((item) => item.id === props.strategy.compose_template_id)
  const fields = settingFields(template?.schema)
  const params = setting.params || {}
  const keys = [...fields.map((field) => field.key).filter((key) => key in params), ...Object.keys(params).filter((key) => !fields.some((field) => field.key === key))]
  return keys.map((key) => ({ key, label: fields.find((field) => field.key === key)?.label || key, value: parameterValue(key, params[key]) }))
}
function parameterValue(key, value) {
  if (key === 'guide_video_url') return value ? '已配置' : '未记录'
  if (typeof value === 'boolean') return value ? '开启' : '关闭'
  return value ?? '—'
}
</script>

<template>
  <div class="compose-strategy-summary">
    <ResourceSection title="基本信息">
      <ResourceDescriptions>
        <t-descriptions-item v-if="!overview" label="策略名称" full>{{ strategy.name }}</t-descriptions-item>
        <t-descriptions-item label="策略 ID">{{ strategy.id }}</t-descriptions-item>
        <t-descriptions-item label="状态"><ResourceStatusBadge :tone="strategy.status === 'enabled' ? 'success' : 'neutral'" :label="strategy.status === 'enabled' ? '启用' : '停用'" /></t-descriptions-item>
        <t-descriptions-item label="所属类目">{{ category || '—' }}</t-descriptions-item>
        <t-descriptions-item label="合成模板">{{ templateLabel(templates, strategy.compose_template_id, strategy.compose_template_version) }}</t-descriptions-item>
        <t-descriptions-item label="所属团队">{{ team || '—' }}</t-descriptions-item>
        <t-descriptions-item label="负责人">{{ strategy.operator_name || '未记录' }}</t-descriptions-item>
        <t-descriptions-item label="创建时间">{{ formatDateTime(strategy.created_at) }}</t-descriptions-item>
        <t-descriptions-item label="修改时间">{{ formatDateTime(strategy.updated_at) }}</t-descriptions-item>
      </ResourceDescriptions>
    </ResourceSection>
    <ResourceSection v-if="showConfig" title="默认成片设置">
      <div class="strategy-output-groups">
        <section v-for="setting in strategySettings(strategy)" :key="setting.key" class="strategy-output-group">
          <h4>{{ setting.name }}</h4>
          <ResourceDescriptions>
            <t-descriptions-item label="适用平台">{{ platformText(setting.platforms) }}</t-descriptions-item>
            <t-descriptions-item label="每条生成份数">{{ setting.copies }} 份</t-descriptions-item>
            <t-descriptions-item v-for="parameter in parameters(setting)" :key="parameter.key" :label="parameter.label" :full="parameter.key === 'guide_video_url'">{{ parameter.value }}</t-descriptions-item>
          </ResourceDescriptions>
        </section>
      </div>
    </ResourceSection>
    <ResourceSection title="执行设置">
      <ResourceDescriptions>
        <t-descriptions-item label="执行方式">{{ strategy.execution_mode === 'scheduled' ? '定时执行' : '手动执行' }}</t-descriptions-item>
        <t-descriptions-item label="每次素材上限">{{ strategy.single_limit }} 条</t-descriptions-item>
        <template v-if="strategy.execution_mode === 'scheduled'">
          <t-descriptions-item label="执行计划">{{ scheduleLabel(strategy.schedule) }}</t-descriptions-item>
          <t-descriptions-item label="每日素材上限">{{ strategy.daily_limit }} 条</t-descriptions-item>
          <t-descriptions-item label="时区">{{ strategy.schedule?.timezone || '—' }}</t-descriptions-item>
          <t-descriptions-item label="下次执行">{{ strategy.next_run_at ? formatDateTime(strategy.next_run_at) : '—' }}</t-descriptions-item>
        </template>
      </ResourceDescriptions>
    </ResourceSection>
    <ResourceSection title="选材条件">
      <div class="table-scroll-wrap">
        <t-table class="wt-resource-table" :data="orderedRules()" :columns="ruleColumns" row-key="priority" size="small" :scroll="{ x: '660px' }" empty="暂无选材条件">
          <template #action="{ row }">{{ row.action === 'filter' ? '排除' : '选入' }}</template>
          <template #logic="{ row }">{{ row.logic === 'all' ? '全部满足' : '任一满足' }}</template>
          <template #conditions="{ row }"><div v-for="(condition, index) in row.conditions || []" :key="index" class="strategy-condition">{{ conditionText(condition) }}</div></template>
        </t-table>
      </div>
    </ResourceSection>
  </div>
</template>

<style scoped>
.compose-strategy-summary { display: grid; gap: 24px; padding-top: 24px; min-width: 0; }
.compose-strategy-summary :deep(.resource-section) { margin-top: 0; }
.strategy-output-groups { display: grid; gap: 16px; }
.strategy-output-group { padding: 16px 20px; border: 1px solid var(--wt-border); border-radius: var(--wt-radius-md); }
.strategy-output-group h4 { margin: 0 0 12px; font-size: 14px; font-weight: 600; line-height: 24px; }
.strategy-condition { overflow-wrap: anywhere; line-height: 24px; }
</style>
