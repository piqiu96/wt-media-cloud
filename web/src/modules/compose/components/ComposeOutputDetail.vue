<script setup>
import ResourceSection from '../../../shared/ui/resource/ResourceSection.vue'
import ResourceDescriptions from '../../../shared/ui/resource/ResourceDescriptions.vue'
import { computed } from 'vue'
import { formatDateTime } from '../../../shared/utils/datetime.js'
import { formatBytes,formatDurationMs } from '../../../shared/utils/units.js'
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'
import { settingFields,platformText } from '../strategySettings.js'
import { outputTitle,outputVideoState } from '../outputManagement.js'
const props=defineProps({detail:Object,previewUrl:{type:String,default:''},previewLoading:Boolean,templates:{type:Array,default:()=>[]},linkable:Boolean,management:Boolean,title:String,triggerType:String,taskName:String,downloadState:{type:Object,default:()=>({})},tabs:Array,tab:String})
const emit=defineEmits(['update:tab','retry-preview','open-material','open-strategy','open-task','open-batch','open-folder','open-file'])
const task=computed(()=>props.detail?.trace?.compose_task||{})
const video=computed(()=>outputVideoState(props.detail?.output,props.downloadState))
const fields=computed(()=>{
 const trace=props.detail?.trace
 const template=props.templates.find(row=>row.code===task.value.template_code&&row.version===task.value.template_version)
 const schema=trace?.strategy?.template_schema||template?.schema
 return settingFields(schema).map(field=>{const value=task.value.config_snapshot?.[field.key];let text='—';if(field.key==='guide_video_url')text=value?'已配置（固定）':'—';else if(typeof value==='boolean')text=value?'开启':'关闭';else if(typeof value==='number')text=field.key.endsWith('_seconds')?`${value} 秒`:String(value);else if(typeof value==='string'&&!value.includes('://'))text=value;return {...field,text}})
})
const metadata=computed(()=>{const output=props.detail?.output,media=output?.media||{};return [formatDurationMs(media.duration_ms),media.width&&media.height?`${media.width} × ${media.height}`:'—',output?.file_size_bytes>0?formatBytes(output.file_size_bytes):'—']})
const publishing=computed(()=>props.detail?.trace?.publishing)
const platformRows=computed(()=>publishing.value?.platforms||[])
const publicationLabel=value=>({unarranged:'未安排',scheduled:'已安排',published:'已发布'})[value]||'—'
</script>
<template>
 <div v-if="detail" class="output-detail">
  <ResourceSection title="基本信息"><ResourceDescriptions>
   <t-descriptions-item label="成片 ID"><strong>{{ detail.output.id }}</strong></t-descriptions-item><t-descriptions-item label="视频状态"><span :title="video.reason"><ResourceStatusBadge :tone="video.tone" :label="video.label" /></span></t-descriptions-item>
   <t-descriptions-item label="成片名称" :span="2"><strong :title="!detail.output.name?'历史名称未记录':''">{{ outputTitle(detail.output) }}</strong></t-descriptions-item>
   <t-descriptions-item label="负责人"><strong :title="!detail.output.operator_name?'历史负责人无法可靠确定':''">{{ detail.output.operator_name || '—' }}</strong></t-descriptions-item><t-descriptions-item label="适用平台">{{ platformText(detail.output.platforms || task.platforms || []) }}</t-descriptions-item><t-descriptions-item label="生成时间"><strong>{{ formatDateTime(detail.output.created_at) }}</strong></t-descriptions-item>
  </ResourceDescriptions></ResourceSection>
  <ResourceSection title="视频信息"><div class="output-detail__file">
   <span class="output-detail__label">视频文件</span>
   <t-link v-if="downloadState.key==='downloaded' && detail.output.file_name" theme="primary" @click="emit('open-file')">{{ detail.output.file_name }}</t-link>
   <a v-else-if="previewUrl && detail.output.file_name" :href="previewUrl" target="_blank" rel="noopener noreferrer" class="output-detail__file-link">{{ detail.output.file_name }}</a>
   <strong v-else :title="video.reason">{{ detail.output.file_name || '文件名未记录' }}</strong>
   <div class="output-detail__metadata"><span>时长 {{ metadata[0] }}</span><span>分辨率 {{ metadata[1] }}</span><span>文件大小 {{ metadata[2] }}</span></div>
   <p v-if="!previewUrl && downloadState.key!=='downloaded'" class="output-detail__hint">{{ video.reason || '视频地址尚未核验' }} <t-link theme="primary" :disabled="previewLoading" @click="emit('retry-preview')">重新核验</t-link></p>
  </div></ResourceSection>
  <ResourceSection title="制作来源"><ResourceDescriptions>
   <t-descriptions-item label="来源素材" :span="2"><t-link v-if="linkable && detail.trace?.material?.id" theme="primary" @click="emit('open-material',detail.trace.material.id)">{{ detail.trace.material.title || `素材 ${detail.trace.material.id}` }}</t-link><strong v-else>{{ detail.trace?.material?.title || '—' }}</strong></t-descriptions-item>
   <t-descriptions-item label="合成策略"><t-link v-if="linkable && detail.trace?.strategy?.id" theme="primary" @click="emit('open-strategy',detail.trace.strategy.id)">{{ detail.trace.strategy.name || detail.trace.strategy.id }}</t-link><strong v-else>{{ detail.trace?.strategy?.name || '—' }}</strong></t-descriptions-item>
   <t-descriptions-item label="合成任务"><t-link v-if="linkable && task.id" theme="primary" @click="emit('open-task',task.id)">{{ task.id }}</t-link><strong v-else>{{ task.id || '—' }}</strong></t-descriptions-item>
   <t-descriptions-item label="任务批次"><t-link v-if="linkable && task.batch_id" theme="primary" @click="emit('open-batch',task.batch_id)">{{ task.batch_id }}</t-link><strong v-else>{{ task.batch_id || '—' }}</strong></t-descriptions-item>
   <t-descriptions-item label="成片设置"><strong>{{ task.setting_name || detail.output.setting_name || '—' }}</strong></t-descriptions-item>
   <t-descriptions-item label="模板版本"><strong>{{ task.template_code ? `${task.template_code} v${task.template_version}` : '—' }}</strong></t-descriptions-item>
  </ResourceDescriptions><p class="output-detail__hint output-detail__snapshot-hint">创建时实际配置，后续策略修改不会改变该成片。</p><ResourceDescriptions v-if="fields.length"><t-descriptions-item v-for="field in fields" :key="field.key" :label="field.label">{{ field.text }}</t-descriptions-item></ResourceDescriptions><p v-else class="output-detail__hint">创建时参数未完整记录。</p></ResourceSection>
  <ResourceSection v-if="publishing?.available && platformRows.length" title="发布情况"><p v-if="!publishing?.available" class="output-detail__hint">{{ publishing?.reason || '发布能力尚未交付，暂不能查询发布安排。' }}</p><div v-if="platformRows.length" class="output-detail__publications"><div class="output-detail__publication"><span>适用平台</span><span>发布情况</span><span>关联任务</span></div><div v-for="row in platformRows" :key="row.platform" class="output-detail__publication"><strong>{{ platformText([row.platform]) }}</strong><span>{{ publishing?.available ? publicationLabel(row.status) : '—' }}</span><span>{{ publishing?.available && row.tasks?.length ? row.tasks.map(item=>item.id).join('、') : '—' }}</span></div></div></ResourceSection>
 </div>
</template>
<style scoped>
.output-detail { display:grid;gap:28px;min-width:0; }
.output-detail :deep(.resource-section) { margin-top:0; }
.output-detail strong { font-weight:400;overflow-wrap:anywhere; }
.output-detail__file { display:grid;grid-template-columns:112px minmax(0,1fr);align-items:start;gap:8px 0;font-size:14px;line-height:24px; }
.output-detail__label { color:var(--wt-text-tertiary); }
.output-detail__file-link { color:var(--wt-primary);text-decoration:none;overflow-wrap:anywhere; }
.output-detail__file-link:hover { text-decoration:underline; }
.output-detail__metadata { grid-column:2;display:flex;flex-wrap:wrap;gap:8px 24px;font-size:13px;color:var(--wt-text-secondary); }
.output-detail__hint { font-size:13px;line-height:1.6;color:var(--wt-text-secondary);margin:8px 0 0; }
.output-detail__file .output-detail__hint { grid-column:2; }
.output-detail__snapshot-hint { margin:20px 0 8px;padding-top:16px;border-top:1px solid var(--wt-border); }
.output-detail__publication { display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px;padding:12px 0;border-bottom:1px solid var(--wt-border);font-size:13px; }
.output-detail__publication:first-child { color:var(--wt-text-tertiary); }
</style>
