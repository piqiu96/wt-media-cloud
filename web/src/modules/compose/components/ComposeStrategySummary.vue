<script setup>
import { computed } from 'vue'
import { templateLabel } from '../strategyConfig.js'
import { schemaFields } from '../schemaFields.js'

// 策略详情的只读正文：合成策略页的「使用范围 / 合成配置」两段，单列出来给任务详情里的
// 关联策略抽屉复用（交互规范 §9.4：复用已有详情能力，不复制第二套业务详情）。
//
// 团队与游戏的名称由宿主解析后传进来 —— 两处宿主各有一份自己的分组/游戏表，组件不认识它们。
const props = defineProps({
  strategy: { type: Object, required: true },
  team: { type: String, default: '' },
  game: { type: String, default: '' },
  templates: { type: Array, default: () => [] },
  // 引导视频的下载地址只在策略管理页出现：它是带签名的地址（与任务快照同一条边界），
  // 任务视图里不渲染这一组。空串=这一组三行都不出现。
  guideUrl: { type: String, default: '' },
})
const configFields = computed(() => {
  const template = props.templates.find((item) => item.id === props.strategy.compose_template_id && item.version === props.strategy.compose_template_version)
  const labels = new Map(schemaFields(template?.schema).map((field) => [field.key, field.label]))
  return Object.entries(props.strategy.config || {})
    .filter(([key]) => key !== 'guide_video_url')
    .map(([key, value]) => ({ key, label: labels.get(key) || key, value: typeof value === 'boolean' ? (value ? '是' : '否') : String(value) }))
})
const guideName = computed(() => {
  try { return decodeURIComponent(new URL(props.guideUrl).pathname.split('/').pop()) || '引导视频' } catch { return '引导视频' }
})
</script>

<template>
  <div class="compose-strategy-summary">
    <h4 class="strategy-detail-section">使用范围</h4>
    <div class="strategy-drawer__field"><span>所属团队</span><strong>{{ team }}</strong></div>
    <div class="strategy-drawer__field"><span>所属游戏</span><strong>{{ game }}</strong></div>
    <h4 class="strategy-detail-section">合成配置</h4>
    <div class="strategy-drawer__field"><span>合成模板</span><strong>{{ templateLabel(templates, strategy.compose_template_id) }}</strong></div>
    <div class="strategy-drawer__field"><span>模板版本</span><strong>v{{ strategy.compose_template_version }}</strong></div>
    <div v-for="field in configFields" :key="field.key" class="strategy-drawer__field"><span>{{ field.label }}</span><strong>{{ field.value }}</strong></div>
    <template v-if="guideUrl">
      <div class="strategy-drawer__field"><span>引导视频</span><strong><a :href="guideUrl" target="_blank" rel="noopener noreferrer">{{ guideName }} · 预览</a></strong></div>
    </template>
  </div>
</template>

<style scoped>
/* 样式跟着这段正文走：宿主页面的 scoped 样式进不来子组件，类名相同但各写一份。 */
.compose-strategy-summary { display: block; }
.strategy-detail-section { margin: 24px 0 8px; padding-bottom: 8px; border-bottom: 1px solid var(--wt-border); color: var(--wt-text-primary); font-size: 14px; font-weight: 600; }
.strategy-drawer__field { display: grid; grid-template-columns: 110px minmax(0, 1fr); gap: 12px; padding: 8px 0; }
.strategy-drawer__field span { color: var(--wt-text-tertiary); font-size: 12px; }
.strategy-drawer__field strong { color: var(--wt-text-primary); font-size: 13px; font-weight: 500; overflow-wrap: anywhere; }
</style>
