<script setup>
import { computed, ref, watch } from 'vue'
import { formatDateTime } from '../../../shared/utils/datetime.js'
import { formatBytes, formatDurationMs } from '../../../shared/utils/units.js'
import { usageStatusLabel } from '../../materials/labels.js'
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'
import { schemaFields } from '../schemaFields.js'
import { triggerLabel } from '../labels.js'

// 成片详情正文由成片管理和任务关联抽屉共用。管理视图按资产操作顺序展示，
// 任务视图保留任务上下文所需的来源面板；标题栏和页脚由各自宿主控制。
const props = defineProps({
  detail: { type: Object, default: null },
  previewUrl: { type: String, default: '' },
  previewLoading: { type: Boolean, default: false },
  templates: { type: Array, default: () => [] },
  linkable: { type: Boolean, default: false },
  management: { type: Boolean, default: false },
  title: { type: String, default: '' },
  triggerType: { type: String, default: '' },
  taskName: { type: String, default: '' },
  downloadState: { type: Object, default: () => ({ label: '状态暂不可用', tone: 'neutral' }) },
  // 任务上下文的面板白名单；管理视图使用同一组件的资产布局。
  tabs: { type: Array, default: () => ['basic', 'source', 'snapshot'] },
  // 当前面板由宿主持有：宿主在每次打开时把它拨回第一屏。
  tab: { type: String, default: '' },
})
const emit = defineEmits(['update:tab', 'retry-preview', 'open-material', 'open-strategy', 'open-task', 'open-folder'])
const activeTab = computed({
  get: () => props.tab || props.tabs[0] || 'basic',
  set: (value) => emit('update:tab', value),
})

// 任务状态是来源追踪里的投影，词表与合成任务页一致。
const taskStatusLabels = { pending: '排队中', running: '处理中', success: '成功', failed: '失败', cancelled: '已取消' }
const outputStatusLabels = { pending_publish: '待发布', published: '已发布', archived: '已归档' }
const outputStatusTones = { pending_publish: 'warning', published: 'success', archived: 'neutral' }
const playbackFailed = ref(false)
watch(() => props.previewUrl, () => { playbackFailed.value = false })

const fields = computed(() => {
  const strategy = props.detail?.trace?.strategy
  const template = props.templates.find((item) => item.code === strategy?.template_code && item.version === strategy?.template_version)
  return schemaFields(template?.schema).filter((field) => field.key !== 'guide_video_url')
})

function snapshotFields(snapshot) {
  return fields.value.filter((field) => Object.hasOwn(snapshot || {}, field.key)).map((field) => {
    const value = snapshot[field.key]
    let text = '—'
    if (typeof value === 'boolean') text = value ? '开启' : '关闭'
    else if (typeof value === 'number' && field.key === 'guide_duration_seconds' && value === 0) text = '完整视频'
    else if (typeof value === 'number' && field.key === 'max_duration_seconds' && value === 0) text = '不限'
    else if (typeof value === 'number' && field.key.endsWith('_seconds')) text = `${value} 秒`
    else if (typeof value === 'number' || (typeof value === 'string' && !value.includes('://'))) text = String(value)
    return { key: field.key, label: field.label, text }
  })
}

const taskFields = computed(() => snapshotFields(props.detail?.trace?.compose_task?.config_snapshot))
const strategyFields = computed(() => snapshotFields(props.detail?.trace?.strategy?.config_snapshot))
const sameConfig = computed(() => JSON.stringify(taskFields.value) === JSON.stringify(strategyFields.value))

function retryPreview() {
  playbackFailed.value = false
  emit('retry-preview')
}
</script>

<template>
  <div v-if="detail" class="output-detail">
    <template v-if="management">
      <section class="output-detail__panel output-detail__panel--preview">
        <video v-if="previewUrl && !playbackFailed" :src="previewUrl" controls preload="metadata" class="output-detail__video" @error="playbackFailed = true">当前环境不支持视频预览</video>
        <div v-else class="output-detail__preview-empty">
          <span>{{ playbackFailed ? '当前环境无法播放，可下载后查看' : '暂时无法预览此成片' }}</span>
          <t-button class="wt-secondary-button" variant="outline" size="small" :loading="previewLoading" @click="retryPreview">重试预览</t-button>
        </div>
      </section>
      <section class="output-detail__panel">
        <h4 class="output-detail__title">核心信息</h4>
        <div class="output-detail__grid">
          <div class="output-detail__wide"><span class="output-detail__label">标题</span><strong>{{ title }}</strong></div>
          <div><span class="output-detail__label">成片 ID</span><strong>{{ detail.output.id }}</strong></div>
          <div><span class="output-detail__label">生成方式</span><strong>{{ triggerLabel(triggerType) }}</strong></div>
          <div><span class="output-detail__label">负责人</span><strong>{{ detail.output.operator_name || '未记录' }}</strong></div>
          <div><span class="output-detail__label">生成时间</span><strong>{{ formatDateTime(detail.output.created_at) }}</strong></div>
          <div><span class="output-detail__label">视频时长</span><strong>{{ formatDurationMs(detail.output.media?.duration_ms) }}</strong></div>
        </div>
      </section>
      <section class="output-detail__panel">
        <h4 class="output-detail__title">生成来源</h4>
        <div class="output-detail__sources">
          <div><span class="output-detail__label">来源素材</span><strong>素材 {{ detail.trace?.material?.id || detail.output.material_id }}</strong><span class="output-detail__source-sub" :title="detail.trace?.material?.title || ''">{{ detail.trace?.material?.title || '—' }}</span><t-link v-if="linkable" theme="primary" @click="emit('open-material', detail.output.material_id)">查看素材详情 ›</t-link></div>
          <div><span class="output-detail__label">合成策略</span><strong :title="detail.trace?.strategy?.name || ''">策略 {{ detail.trace?.strategy?.id || detail.output.compose_strategy_id }} · {{ detail.trace?.strategy?.name || '—' }}</strong><t-link v-if="linkable" theme="primary" @click="emit('open-strategy', detail.output.compose_strategy_id)">查看策略详情 ›</t-link></div>
          <div><span class="output-detail__label">合成任务</span><strong :title="taskName || ''">任务 {{ detail.trace?.compose_task?.id || detail.output.compose_task_id }} · {{ taskName || `合成任务 ${detail.output.compose_task_id}` }}</strong><t-link v-if="linkable" theme="primary" @click="emit('open-task', detail.output.compose_task_id)">查看任务详情 ›</t-link></div>
        </div>
      </section>
      <section class="output-detail__panel">
        <h4 class="output-detail__title">文件信息</h4>
        <div class="output-detail__grid">
          <div><span class="output-detail__label">文件大小</span><strong>{{ formatBytes(detail.output.file_size_bytes) }}</strong></div>
          <div><span class="output-detail__label">下载状态</span><ResourceStatusBadge :tone="downloadState.tone" :label="downloadState.label" /></div>
          <div><span class="output-detail__label">云端视频</span><a v-if="previewUrl" class="wt-primary-link output-detail__file-link" :href="previewUrl" :title="previewUrl" target="_blank" rel="noopener noreferrer">打开云端视频</a><strong v-else>暂不可用</strong></div>
          <div v-if="downloadState.key === 'downloaded'"><span class="output-detail__label">本地文件</span><t-link class="output-detail__file-link" theme="primary" title="打开文件所在文件夹" @click="emit('open-folder')">{{ downloadState.fileName }}</t-link></div>
          <div class="output-detail__wide"><span class="output-detail__label">文件校验信息</span><strong>{{ detail.output.sha256 ? `SHA-256 · ${detail.output.sha256}` : '暂无校验值' }}</strong></div>
        </div>
      </section>
    </template>
    <t-tabs v-else v-model="activeTab" class="output-detail__tabs">
      <t-tab-panel v-if="tabs.includes('basic')" value="basic" label="基本信息">
        <section class="output-detail__panel">
          <h4 class="output-detail__title">成片预览</h4>
          <video v-if="previewUrl && !playbackFailed" :src="previewUrl" controls preload="metadata" class="output-detail__video" @error="playbackFailed = true">当前环境不支持视频预览</video>
          <div v-else class="output-detail__preview-empty">
            <span>{{ playbackFailed ? '当前环境无法播放，可下载后查看' : '暂时无法预览此成片' }}</span>
            <t-button class="wt-secondary-button" variant="outline" size="small" :loading="previewLoading" @click="retryPreview">重试预览</t-button>
          </div>
        </section>
        <section class="output-detail__panel">
          <h4 class="output-detail__title">文件信息</h4>
          <div class="output-detail__grid">
            <div><span class="output-detail__label">成片 ID</span><strong>{{ detail.output.id }}</strong></div>
            <div><span class="output-detail__label">状态</span><ResourceStatusBadge :tone="outputStatusTones[detail.output.publish_status] || 'neutral'" :label="outputStatusLabels[detail.output.publish_status] || detail.output.publish_status || '—'" /></div>
            <div><span class="output-detail__label">文件大小</span><strong>{{ formatBytes(detail.output.file_size_bytes) }}</strong></div>
            <div><span class="output-detail__label">视频时长</span><strong>{{ formatDurationMs(detail.output.media?.duration_ms) }}</strong></div>
            <div><span class="output-detail__label">生成时间</span><strong>{{ formatDateTime(detail.output.created_at) }}</strong></div>
          </div>
          <div class="output-detail__hash"><span class="output-detail__label">SHA-256</span><span>{{ detail.output.sha256 || '—' }}</span></div>
        </section>
      </t-tab-panel>
      <t-tab-panel v-if="tabs.includes('source')" value="source" label="来源追踪">
        <section class="output-detail__panel">
          <h4 class="output-detail__title">来源素材</h4>
          <div class="output-detail__grid">
            <div class="output-detail__wide"><span class="output-detail__label">素材名称</span><strong>{{ detail.trace.material.title || '—' }}</strong></div>
            <div><span class="output-detail__label">素材 ID</span><t-link v-if="linkable" theme="primary" @click="emit('open-material', detail.trace.material.id)">{{ detail.trace.material.id }}</t-link><strong v-else>{{ detail.trace.material.id }}</strong></div>
            <div><span class="output-detail__label">素材使用</span><strong v-if="detail.trace?.material_usage">{{ detail.trace.material_usage.id }} · {{ usageStatusLabel(detail.trace.material_usage.status) || detail.trace.material_usage.status || '—' }}</strong><strong v-else>—</strong></div>
          </div>
        </section>
        <section class="output-detail__panel">
          <h4 class="output-detail__title">生产记录</h4>
          <div class="output-detail__grid">
            <div><span class="output-detail__label">合成策略</span><t-link v-if="linkable" theme="primary" @click="emit('open-strategy', detail.trace.strategy.id)">{{ detail.trace.strategy.id }} {{ detail.trace.strategy.name || '—' }}</t-link><strong v-else>{{ detail.trace.strategy.id }} {{ detail.trace.strategy.name || '—' }}</strong></div>
            <div><span class="output-detail__label">合成模板</span><strong>{{ detail.trace.strategy.template_code }} v{{ detail.trace.strategy.template_version }}</strong></div>
            <div><span class="output-detail__label">合成任务</span><t-link v-if="linkable" theme="primary" @click="emit('open-task', detail.trace.compose_task.id)">{{ detail.trace.compose_task.id }}</t-link><strong v-else>{{ detail.trace.compose_task.id }}</strong></div>
            <div><span class="output-detail__label">任务状态</span><strong>{{ taskStatusLabels[detail.trace.compose_task.status] || detail.trace.compose_task.status || '—' }}</strong></div>
            <div><span class="output-detail__label">任务创建</span><strong>{{ formatDateTime(detail.trace.compose_task.created_at) }}</strong></div>
          </div>
        </section>
      </t-tab-panel>
      <t-tab-panel v-if="tabs.includes('snapshot')" value="snapshot" label="配置快照">
        <section class="output-detail__panel">
          <h4 class="output-detail__title">生成时生效参数</h4>
          <p class="output-detail__hint">成片按任务创建时的参数生成，后续策略修改不会改变该成片。</p>
          <div v-if="taskFields.length" class="output-detail__grid output-detail__snapshot-grid">
            <div v-for="field in taskFields" :key="field.key"><span class="output-detail__label">{{ field.label }}</span><strong>{{ field.text }}</strong></div>
          </div>
          <p v-else class="output-detail__empty">当前没有可安全展示的模板参数。</p>
        </section>
        <section v-if="fields.length && !sameConfig" class="output-detail__panel">
          <h4 class="output-detail__title">策略当前参数</h4>
          <p class="output-detail__hint">仅供对比，不影响已生成的成片。</p>
          <div class="output-detail__grid output-detail__snapshot-grid">
            <div v-for="field in strategyFields" :key="field.key"><span class="output-detail__label">{{ field.label }}</span><strong>{{ field.text }}</strong></div>
          </div>
        </section>
      </t-tab-panel>
    </t-tabs>
  </div>
</template>

<style scoped>
.output-detail__panel { margin-top: 16px; padding: 16px; border: 1px solid var(--wt-border); border-radius: var(--wt-radius-md); background: var(--wt-bg-card); }
.output-detail__title { margin: 0 0 16px; font-size: 15px; font-weight: 600; color: var(--wt-text-primary); }
.output-detail__video { display: block; width: 100%; max-height: 300px; aspect-ratio: 16 / 9; border-radius: var(--wt-radius-sm); background: #000; object-fit: contain; }
.output-detail__preview-empty { min-height: 220px; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 12px; border: 1px dashed var(--wt-border); border-radius: var(--wt-radius-sm); background: var(--wt-bg-page); color: var(--wt-text-secondary); font-size: 13px; }
.output-detail__grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px 24px; font-size: 14px; line-height: 1.5; }
.output-detail__grid > div { display: flex; flex-direction: column; gap: 4px; min-width: 0; }
.output-detail__wide { grid-column: 1 / -1; }
.output-detail__grid strong { color: var(--wt-text-primary); font-weight: 500; overflow-wrap: anywhere; }
.output-detail__label { color: var(--wt-text-tertiary); font-size: 12px; }
.output-detail__hash { display: flex; flex-wrap: wrap; gap: 4px 8px; align-items: baseline; margin-top: 16px; padding-top: 16px; border-top: 1px dashed var(--wt-border); font-size: 12px; }
.output-detail__hash span:last-child { word-break: break-all; color: var(--wt-text-primary); }
.output-detail__hint { margin: 0; color: var(--wt-text-secondary); font-size: 12px; line-height: 1.5; }
.output-detail__snapshot-grid { margin-top: 16px; }
.output-detail__empty { margin: 16px 0 0; color: var(--wt-text-secondary); font-size: 13px; }
.output-detail__panel--preview { padding: 0; overflow: hidden; }
.output-detail__panel--preview .output-detail__video { max-height: 360px; border-radius: 0; }
.output-detail__sources { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); }
.output-detail__sources > div { display: flex; flex-direction: column; align-items: flex-start; gap: 6px; min-width: 0; padding: 0 16px; border-left: 1px solid var(--wt-border); }
.output-detail__sources > div:first-child { border-left: 0; padding-left: 0; }
.output-detail__sources > div:last-child { padding-right: 0; }
.output-detail__sources strong { max-width: 100%; font-size: 14px; font-weight: 500; color: var(--wt-text-primary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.output-detail__source-sub { max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 12px; color: var(--wt-text-tertiary); }
.output-detail__sources :deep(.t-link) { margin-top: auto; font-size: 12px; }
.output-detail__file-link { align-self: flex-start; max-width: 100%; font-weight: 600; overflow-wrap: anywhere; }
.wt-primary-link { color: var(--wt-primary); text-decoration: none; }
.wt-primary-link:hover, .wt-primary-link:focus-visible { text-decoration: underline; }
@media (max-width: 700px) { .output-detail__grid { grid-template-columns: 1fr; } .output-detail__sources { grid-template-columns: 1fr; gap: 16px; } .output-detail__sources > div { padding: 12px 0 0; border-left: 0; border-top: 1px solid var(--wt-border); } .output-detail__sources > div:first-child { border-top: 0; padding-top: 0; } }
</style>
