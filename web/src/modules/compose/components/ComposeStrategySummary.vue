<script setup>
import { computed } from 'vue'
import ResourceSection from '../../../shared/ui/resource/ResourceSection.vue'
import ResourceDescriptions from '../../../shared/ui/resource/ResourceDescriptions.vue'
import { templateLabel } from '../strategyConfig.js'
import { schemaFields } from '../schemaFields.js'
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
const configFields = computed(() => {
  const template = props.templates.find((item) => item.id === props.strategy.compose_template_id && item.version === props.strategy.compose_template_version)
  const fields = schemaFields(template?.schema)
  const labels = new Map(fields.map((field) => [field.key, field.label]))
  const order = new Map(fields.map((field, index) => [field.key, index]))
  return Object.entries(props.strategy.config || {})
    .filter(([key]) => key !== 'guide_video_url')
    .map(([key, value]) => ({ key, label: labels.get(key) || key, value: typeof value === 'boolean' ? (value ? '是' : '否') : String(value) }))
    .sort((left, right) => (order.get(left.key) ?? fields.length) - (order.get(right.key) ?? fields.length))
})
const guideName = computed(() => {
  try { return decodeURIComponent(new URL(props.guideUrl).pathname.split('/').pop()) || '引导视频' } catch { return '引导视频' }
})
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
    <ResourceSection v-if="showConfig && (configFields.length || guideUrl)" title="合成配置"><ResourceDescriptions>
      <t-descriptions-item v-for="field in configFields" :key="field.key" :label="field.label">{{ field.value }}</t-descriptions-item>
      <t-descriptions-item v-if="guideUrl" label="引导视频" :span="2"><a :href="guideUrl" target="_blank" rel="noopener noreferrer">{{ guideName }}</a></t-descriptions-item>
    </ResourceDescriptions></ResourceSection>
  </div>
</template>
<style scoped>
.compose-strategy-summary { display: grid; gap: 28px; }
.compose-strategy-summary :deep(.resource-section) { margin-top: 0; }
.compose-strategy-summary a { color: var(--wt-primary); }
</style>
