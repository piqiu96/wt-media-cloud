<script setup>
import { computed } from 'vue'
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
  <div v-if="overview" class="compose-strategy-summary compose-strategy-summary--overview">
    <section class="strategy-overview-section">
      <h4>主体信息</h4>
      <div class="strategy-overview-grid">
        <div class="strategy-overview-field"><span>策略 ID</span><strong>{{ strategy.id }}</strong></div>
        <div class="strategy-overview-field"><span>模板版本</span><strong>v{{ strategy.compose_template_version }}</strong></div>
        <div class="strategy-overview-field"><span>状态</span><strong>{{ strategy.status === 'enabled' ? '已启用' : '已停用' }}</strong></div>
        <div class="strategy-overview-field"><span>所属团队</span><strong>{{ team || '—' }}</strong></div>
        <div class="strategy-overview-field"><span>所属类目</span><strong>{{ category || '—' }}</strong></div>
        <div class="strategy-overview-field"><span>负责人</span><strong>{{ strategy.operator_name || '未记录' }}</strong></div>
        <div class="strategy-overview-field"><span>合成模板</span><strong>{{ templateLabel(templates, strategy.compose_template_id) }}</strong></div>
        <div class="strategy-overview-field"><span>创建时间</span><strong>{{ formatDateTime(strategy.created_at) }}</strong></div>
        <div class="strategy-overview-field"><span>修改时间</span><strong>{{ formatDateTime(strategy.updated_at) }}</strong></div>
      </div>
    </section>
    <section class="strategy-overview-section">
      <h4>合成配置</h4>
      <div class="strategy-overview-grid">
        <div v-for="field in configFields" :key="field.key" class="strategy-overview-field"><span>{{ field.label }}</span><strong>{{ field.value }}</strong></div>
        <div v-if="guideUrl" class="strategy-overview-field"><span>引导视频</span><strong><a :href="guideUrl" target="_blank" rel="noopener noreferrer">{{ guideName }} · 预览</a></strong></div>
      </div>
    </section>
  </div>
  <div v-else class="compose-strategy-summary">
    <section class="strategy-detail-card">
      <h4 class="strategy-detail-section">策略信息</h4>
      <div class="strategy-detail-grid">
        <div class="strategy-drawer__field"><span>策略 ID</span><strong>{{ strategy.id }}</strong></div>
        <div class="strategy-drawer__field"><span>策略名称</span><strong>{{ strategy.name }}</strong></div>
        <div class="strategy-drawer__field"><span>模板版本</span><strong>v{{ strategy.compose_template_version }}</strong></div>
        <div class="strategy-drawer__field"><span>合成模板</span><strong>{{ templateLabel(templates, strategy.compose_template_id) }}</strong></div>
      </div>
    </section>
    <section class="strategy-detail-card">
      <h4 class="strategy-detail-section">使用范围</h4>
      <div class="strategy-detail-grid">
        <div class="strategy-drawer__field"><span>所属团队</span><strong>{{ team }}</strong></div>
        <div class="strategy-drawer__field"><span>所属类目</span><strong>{{ category }}</strong></div>
        <div class="strategy-drawer__field"><span>负责人</span><strong>{{ strategy.operator_name || '未记录' }}</strong></div>
      </div>
    </section>
    <section v-if="configFields.length || guideUrl" class="strategy-detail-card">
      <h4 class="strategy-detail-section">合成配置</h4>
      <div class="strategy-detail-grid">
        <div v-for="field in configFields" :key="field.key" class="strategy-drawer__field"><span>{{ field.label }}</span><strong>{{ field.value }}</strong></div>
        <div v-if="guideUrl" class="strategy-drawer__field"><span>引导视频</span><strong><a :href="guideUrl" target="_blank" rel="noopener noreferrer">{{ guideName }} · 预览</a></strong></div>
      </div>
    </section>
  </div>
</template>

<style scoped>
/* 样式跟着这段正文走：宿主页面的 scoped 样式进不来子组件，类名相同但各写一份。 */
.compose-strategy-summary { display: grid; gap: 16px; }
.strategy-detail-card { padding: 16px; border: 1px solid var(--wt-border); border-radius: var(--wt-radius-md); background: var(--wt-bg-card); }
.strategy-detail-section { margin: 0 0 16px; color: var(--wt-text-primary); font-size: 15px; font-weight: 600; }
.strategy-detail-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px 24px; }
.strategy-drawer__field { display: flex; flex-direction: column; gap: 4px; min-width: 0; line-height: 1.5; }
.strategy-drawer__field span { color: var(--wt-text-tertiary); font-size: 12px; }
.strategy-drawer__field strong { color: var(--wt-text-primary); font-size: 14px; font-weight: 500; overflow-wrap: anywhere; }
.strategy-drawer__field a { color: var(--wt-primary); }
.compose-strategy-summary--overview { display: grid; gap: 12px; }
.strategy-overview-section { border: 1px solid var(--wt-border); border-radius: var(--wt-radius-md); padding: 14px 16px; }
.strategy-overview-section h4 { margin: 0 0 12px; color: var(--wt-text-primary); font-size: 14px; }
.strategy-overview-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px 20px; }
.strategy-overview-field { display: grid; grid-template-columns: minmax(82px, 0.45fr) minmax(0, 1fr); gap: 8px; font-size: 13px; }
.strategy-overview-field span { color: var(--wt-text-tertiary); }
.strategy-overview-field strong { color: var(--wt-text-primary); font-weight: 500; overflow-wrap: anywhere; }
.strategy-overview-field a { color: var(--wt-primary); }
@media (max-width: 680px) { .strategy-detail-grid, .strategy-overview-grid { grid-template-columns: 1fr; } }
</style>
