<script setup>
import { computed } from 'vue'
import { formatDateTime } from '../../../shared/utils/datetime.js'
import { formatBytes, formatDurationMs } from '../../../shared/utils/units.js'
import { usageStatusLabel } from '../../materials/labels.js'

// 成片详情的正文：我的成片页那个抽屉里的三个面板，单列出来给任务详情里的关联成片抽屉
// 复用（交互规范 §9.4）。标题行与页脚留在各自宿主：我的成片页有自己的头像行和归档/
// 发布动作，任务页的只读抽屉不带业务动作。
const props = defineProps({
  detail: { type: Object, default: null },
  previewUrl: { type: String, default: '' },
  previewLoading: { type: Boolean, default: false },
  // 面板白名单。任务详情里不开「配置快照」：那一页只允许业务可读的配置，原始 JSON
  // 是成片页自己的低层级入口（本 Prompt §二十七）。
  tabs: { type: Array, default: () => ['basic', 'source', 'snapshot'] },
  // 当前面板由宿主持有：宿主在每次打开时把它拨回第一屏。
  tab: { type: String, default: '' },
})
const emit = defineEmits(['update:tab', 'retry-preview'])
const activeTab = computed({
  get: () => props.tab || props.tabs[0] || 'basic',
  set: (value) => emit('update:tab', value),
})

// 任务状态是来源追踪里的投影，词表与合成任务页一致。
const taskStatusLabels = { pending: '排队中', running: '处理中', success: '成功', failed: '失败', cancelled: '已取消' }
</script>

<template>
  <div v-if="detail" class="output-detail">
    <t-tabs v-model="activeTab" class="output-detail__tabs">
      <t-tab-panel v-if="tabs.includes('basic')" value="basic" label="基本信息">
        <div class="output-detail__panel">
          <video v-if="previewUrl" :src="previewUrl" controls preload="metadata" class="output-detail__video">当前环境不支持视频预览</video>
          <div v-else class="output-detail__preview-empty"><span>暂时无法预览此成片</span><t-button class="wt-secondary-button" variant="outline" size="small" :loading="previewLoading" @click="emit('retry-preview')">重试预览</t-button></div>
          <h4 class="output-detail__title">文件信息</h4>
          <div class="output-detail__grid">
            <div><span class="output-detail__label">文件大小</span><strong>{{ formatBytes(detail.output.file_size_bytes) }}</strong></div>
            <div><span class="output-detail__label">视频时长</span><strong>{{ formatDurationMs(detail.output.media?.duration_ms) }}</strong></div>
            <div><span class="output-detail__label">生成时间</span><strong>{{ formatDateTime(detail.output.created_at) }}</strong></div>
          </div>
          <div class="output-detail__hash"><span class="output-detail__label">SHA-256</span><span>{{ detail.output.sha256 || '—' }}</span></div>
        </div>
      </t-tab-panel>
      <t-tab-panel v-if="tabs.includes('source')" value="source" label="来源追踪">
        <div class="output-detail__panel output-detail__grid">
          <div><span class="output-detail__label">素材</span><strong>{{ detail.trace.material.id }} {{ detail.trace.material.title }}</strong></div>
          <div><span class="output-detail__label">素材使用</span><strong>{{ detail.trace.material_usage.id }} {{ usageStatusLabel(detail.trace.material_usage.status) || detail.trace.material_usage.status || '—' }}</strong></div>
          <div><span class="output-detail__label">策略</span><strong>{{ detail.trace.strategy.id }} {{ detail.trace.strategy.name || '—' }}</strong></div>
          <div><span class="output-detail__label">模板</span><strong>{{ detail.trace.strategy.template_code }} v{{ detail.trace.strategy.template_version }}</strong></div>
          <div><span class="output-detail__label">合成任务</span><strong>{{ detail.trace.compose_task.id }} {{ taskStatusLabels[detail.trace.compose_task.status] || detail.trace.compose_task.status || '—' }}</strong></div>
          <div><span class="output-detail__label">任务创建</span><strong>{{ formatDateTime(detail.trace.compose_task.created_at) }}</strong></div>
        </div>
      </t-tab-panel>
      <t-tab-panel v-if="tabs.includes('snapshot')" value="snapshot" label="配置快照">
        <div class="output-detail__panel">
          <h4 class="output-detail__title">任务参数快照（创建当时）</h4>
          <pre class="output-detail__snapshot">{{ JSON.stringify(detail.trace.compose_task.config_snapshot, null, 2) }}</pre>
          <h4 class="output-detail__title">策略当前配置</h4>
          <pre class="output-detail__snapshot">{{ JSON.stringify(detail.trace.strategy.config_snapshot, null, 2) }}</pre>
        </div>
      </t-tab-panel>
    </t-tabs>
  </div>
</template>

<style scoped>
.output-detail__panel { padding: 20px 0; }
.output-detail__title { margin: 20px 0 12px; font-size: 14px; font-weight: 600; color: var(--wt-text-primary); }
.output-detail__title:first-child { margin-top: 0; }
.output-detail__video { width: 100%; border-radius: 8px; background: #000; max-height: 340px; }
.output-detail__preview-empty { min-height: 200px; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 12px; border: 1px dashed var(--wt-border); border-radius: 8px; color: var(--wt-text-secondary); font-size: 13px; }
.output-detail__grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 18px 24px; font-size: 13px; }
.output-detail__grid > div { display: flex; flex-direction: column; gap: 5px; min-width: 0; }
.output-detail__grid strong { color: var(--wt-text-primary); font-weight: 500; overflow-wrap: anywhere; }
.output-detail__label { color: var(--wt-text-tertiary); font-size: 12px; }
.output-detail__hash { display: flex; gap: 8px; align-items: baseline; margin-top: 10px; font-size: 12px; }
.output-detail__hash span:last-child { word-break: break-all; color: var(--wt-text-primary); }
.output-detail__snapshot { margin: 0; padding: 12px; background: var(--wt-bg-secondary, #f5f6f8); border-radius: 6px; font-size: 12px; overflow: auto; max-height: 220px; }
@media (max-width: 700px) { .output-detail__grid { grid-template-columns: 1fr; } }
</style>
