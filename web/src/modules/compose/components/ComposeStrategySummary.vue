<script setup>
import { platformText, settingFields, strategySettings } from '../strategySettings.js'
import { scheduleLabel } from '../scheduleForm.js'

import ResourceSection from '../../../shared/ui/resource/ResourceSection.vue'
import ResourceDescriptions from '../../../shared/ui/resource/ResourceDescriptions.vue'
import { templateLabel } from '../strategyConfig.js'
import { formatDateTime } from '../../../shared/utils/datetime.js'

// 策略详情的只读正文：合成策略页的「使用范围 / 合成配置」两段，单列出来给任务详情里的
// 关联策略抽屉复用（交互规范 §9.4：复用已有详情能力，不复制第二套业务详情）。
//
// 团队与类目的名称由宿主解析后传进来 —— 两处宿主各有一份自己的团队/类目表，组件不认识它们。
const props = defineProps({
  strategy: { type: Object, required: true },
  team: { type: String, default: '' },
  category: { type: String, default: '' },
  templates: { type: Array, default: () => [] },
  // 引导视频的下载地址只在策略管理页出现：它是带签名的地址（与任务快照同一条边界），
  // 任务视图里不渲染这一组。空串=这一组三行都不出现。
  guideUrl: { type: String, default: '' },
  overview: { type: Boolean, default: false },
  showConfig: { type: Boolean, default: true },
})
const conditionFields = [
  { label: '发布时间', value: 'published_at' },
  { label: '点赞量', value: 'like_count' },
  { label: '收藏量', value: 'favorite_count' },
]

function conditionOperators(field) {
  return field === 'published_at'
    ? [{ label: '不早于', value: 'gte' }, { label: '早于', value: 'lt' }, { label: '近 N 小时', value: 'recent_hours' }, { label: '近 N 天', value: 'recent_days' }]
    : [{ label: '等于', value: 'eq' }, { label: '大于等于', value: 'gte' }, { label: '小于等于', value: 'lte' }]
}

function conditionParts(condition) {
  const field = conditionFields.find((item) => item.value === condition.field)?.label || condition.field
  const operator = conditionOperators(condition.field).find((item) => item.value === condition.operator)?.label || condition.operator
  const value = condition.field === 'published_at' && !['recent_hours', 'recent_days'].includes(condition.operator)
    ? formatDateTime(condition.value)
    : ['recent_hours', 'recent_days'].includes(condition.operator)
      ? `${condition.value} ${condition.operator === 'recent_hours' ? '小时' : '天'}`
      : condition.value
  return { field, operator, value }
}

function orderedRules(row) { return [...(row.priority_rules || [])].sort((a, b) => a.priority - b.priority) }

</script>

<template>
  <div class="compose-strategy-summary">
    <ResourceSection title="基本信息"><ResourceDescriptions>
      <t-descriptions-item label="策略 ID">{{ strategy.id }}</t-descriptions-item>
      <t-descriptions-item label="状态">{{ strategy.status === 'enabled' ? '已启用' : '已停用' }}</t-descriptions-item>
      <t-descriptions-item v-if="!overview" label="策略名称" :span="2">{{ strategy.name }}</t-descriptions-item>
      <t-descriptions-item label="所属团队">{{ team || '—' }}</t-descriptions-item>
      <t-descriptions-item label="所属类目">{{ category || '—' }}</t-descriptions-item>
      <t-descriptions-item label="负责人">{{ strategy.operator_name || '未记录' }}</t-descriptions-item>
      <t-descriptions-item label="合成模板">{{ templateLabel(templates, strategy.compose_template_id, strategy.compose_template_version) }}</t-descriptions-item>
      <t-descriptions-item label="创建时间">{{ formatDateTime(strategy.created_at) }}</t-descriptions-item>
      <t-descriptions-item label="修改时间">{{ formatDateTime(strategy.updated_at) }}</t-descriptions-item>
    </ResourceDescriptions></ResourceSection>
              <ResourceSection v-if="showConfig" title="默认成片设置">
                <div v-for="setting in strategySettings(strategy)" :key="setting.key" class="strategy-overview__rule">
                  <div class="strategy-overview__rule-head"><strong>{{ setting.name }}</strong><span>{{ platformText(setting.platforms) }}</span><span>每条 {{ setting.copies }} 份</span></div>
                  <div v-for="(value, key) in setting.params" :key="key" class="strategy-overview__field"><span>{{ settingFields(templates.find((item) => item.id === strategy.compose_template_id)?.schema).find((field) => field.key === key)?.label || key }}</span><strong>{{ key === 'guide_video_url' ? (value ? '已配置（固定）' : '未记录') : typeof value === 'boolean' ? (value ? '开启' : '关闭') : value }}</strong></div>
                </div>
              </ResourceSection>
              <ResourceSection title="执行设置">
                <div class="strategy-overview__grid">
                  <div class="strategy-overview__field"><span>执行方式</span><strong>{{ strategy.execution_mode === 'scheduled' ? '定时执行' : '手动执行' }}</strong></div>
                  <template v-if="strategy.execution_mode === 'scheduled'">
                    <div class="strategy-overview__field"><span>执行计划</span><strong>{{ scheduleLabel(strategy.schedule) }}</strong></div>
                    <div class="strategy-overview__field"><span>时区</span><strong>{{ strategy.schedule?.timezone || '—' }}</strong></div>
                    <div class="strategy-overview__field"><span>下次执行</span><strong>{{ strategy.next_run_at ? formatDateTime(strategy.next_run_at) : '—' }}</strong></div>
                  </template>
                </div>
              </ResourceSection>
              <ResourceSection title="执行条件">
                <div class="strategy-overview__grid">
                  <div class="strategy-overview__field strategy-overview__field--wide"><span>候选范围</span><strong>当前策略业务范围内可生产素材</strong></div>
                  <div class="strategy-overview__field"><span>单次上限</span><strong>{{ strategy.single_limit }} 条</strong></div>
                  <div v-if="strategy.execution_mode === 'scheduled'" class="strategy-overview__field"><span>每日上限</span><strong>{{ strategy.daily_limit }} 条</strong></div>
                </div>
                <div v-for="group in orderedRules(strategy)" :key="group.priority" class="strategy-overview__rule">
                  <div class="strategy-overview__rule-head"><strong>优先级 {{ group.priority }}</strong><span>{{ group.action === 'filter' ? '排除' : '选入' }}</span><span>{{ group.logic === 'all' ? '全部满足' : '任一满足' }}</span></div>
                  <div v-for="(condition, index) in group.conditions || []" :key="index" class="strategy-overview__condition">
                    <span>{{ conditionParts(condition).field }}</span><span>{{ conditionParts(condition).operator }}</span><strong>{{ conditionParts(condition).value }}</strong>
                  </div>
                </div>
              </ResourceSection>
  </div>
</template>
<style scoped>
.compose-strategy-summary { display: grid; gap: 28px; }
.compose-strategy-summary :deep(.resource-section) { margin-top: 0; }
.compose-strategy-summary a { color: var(--wt-primary); }
.strategy-overview__grid { display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:16px 24px; }
.strategy-overview__field { display:grid;grid-template-columns:112px minmax(0,1fr);gap:16px;font-size:14px;line-height:24px; }
.strategy-overview__field span {color:var(--wt-text-tertiary);}
.strategy-overview__field strong {font-weight:400;overflow-wrap:anywhere;}
.strategy-overview__field--wide {grid-column:1/-1;}
.strategy-overview__rule {margin-top:12px;padding:16px;border:1px solid var(--wt-border);border-radius:var(--wt-radius-sm);}
.strategy-overview__rule-head {display:flex;flex-wrap:wrap;gap:8px 16px;margin-bottom:12px;font-size:13px;}
.strategy-overview__rule-head strong {margin-right:auto;}
.strategy-overview__condition {display:flex;gap:16px;padding:6px 0;font-size:13px;}

</style>
