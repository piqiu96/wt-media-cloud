<script setup>
import { computed,onMounted,onUnmounted,ref } from 'vue'
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'
import { opColumnWidth } from '../../../shared/ui/resource/opColumnWidth.js'
import { formatDateTime } from '../../../shared/utils/datetime.js'
import { platformText } from '../strategySettings.js'
import { outputTitle } from '../outputManagement.js'
const props=defineProps({rows:{type:Array,default:()=>[]},selectedKeys:{type:Array,default:()=>[]},materialMode:Boolean,videoState:{type:Function,required:true},canDownload:{type:Function,required:true},downloadReason:{type:Function,default:()=>''},downloadingId:Number,categoryName:{type:Function,default:()=> '—'}})
// Extra viewport space belongs to the name column; checkbox and actions keep their measured widths.
const tableRoot=ref(null),availableWidth=ref(0)
let resizeObserver
onMounted(()=>{const element=tableRoot.value;if(!element)return;availableWidth.value=element.clientWidth;if(typeof ResizeObserver!=='undefined'){resizeObserver=new ResizeObserver(()=>{availableWidth.value=element.clientWidth});resizeObserver.observe(element)}})
onUnmounted(()=>resizeObserver?.disconnect())
const emit=defineEmits(['update:selectedKeys','detail','download','remake','material','batch'])
const columns = computed(() => [
  { colKey: 'row-select', type:'multiple', width:44 },
  { colKey:'id', title:'成片 ID', width:70 },
  { colKey:'output', title:'成片名称 / 生成时间', width:190 },
  { colKey:'source', title:props.materialMode?'来源策略 / 任务批次':'来源素材', width:140 },
  { colKey:'category', title:'类目', width:75 },
  { colKey:'operator', title:'负责人', width:65 },
  { colKey:'platforms', title:'适用平台', width:100 },
  { colKey:'video', title:'视频状态', width:75 },
  { colKey:'op', title:'操作', width:opColumnWidth(['详情','下载','再次制作'],{loading:true}), fixed:'right' },
].map((col,index,all)=>col.colKey==='output'?{...col,width:Math.max(col.width,availableWidth.value-all.reduce((sum,item)=>sum+(item.colKey==='output'?0:item.width),0))}:col))
const tableScroll = computed(()=>({x:`${columns.value.reduce((sum,col)=>sum+(col.width||0),0)}px`}))
</script>
<template>
 <div ref="tableRoot" class="table-scroll-wrap"><t-table class="wt-resource-table" :data="rows" :columns="columns" row-key="id" :selected-row-keys="selectedKeys" reserve-selected-row-on-paginate :scroll="tableScroll" hover size="small" empty="暂无成片" @select-change="emit('update:selectedKeys',$event)">
  <template #output="{row}"><div class="output-table__related"><strong :title="outputTitle(row)">{{ outputTitle(row) }}</strong><span>{{ formatDateTime(row.created_at) }}</span><span v-if="materialMode">{{ row.setting_name || '成片设置未记录' }}</span></div></template>
  <template #source="{row}"><div v-if="materialMode" class="output-table__related"><strong>{{ row.strategy_name || '—' }}</strong><t-link v-if="row.batch_id" theme="primary" @click="emit('batch',row.batch_id)">任务批次 {{ row.batch_id }}</t-link><span v-else>任务批次未记录</span></div><div v-else class="output-table__related"><t-link v-if="row.material_id" theme="primary" :title="row.material_title" @click="emit('material',row.material_id)">{{ row.material_title || `素材 ${row.material_id}` }}</t-link><span>{{ row.material_id }}</span></div></template>
  <template #category="{row}">{{ categoryName(row.category_id) }}</template>
  <template #operator="{row}"><span class="output-table__operator" :title="row.operator_name || '历史负责人无法可靠确定'">{{ row.operator_name || '—' }}</span></template>
  <template #platforms="{row}">{{ row.platforms?.length ? platformText(row.platforms) : '—' }}</template>
  <template #video="{row}"><span :title="videoState(row).reason"><ResourceStatusBadge :label="videoState(row).label" :tone="videoState(row).tone" /></span></template>
  <template #op="{row}"><t-space class="wt-resource-actions" size="small"><t-button size="small" variant="outline" class="wt-secondary-button" @click="emit('detail',row)">详情</t-button><t-button v-if="canDownload(row)||downloadReason(row)" :disabled="!canDownload(row)" :title="downloadReason(row)" size="small" variant="outline" class="wt-secondary-button" :loading="downloadingId===row.id" @click="emit('download',row)">下载</t-button><t-button v-if="row.actions?.remake" size="small" variant="outline" class="wt-secondary-button" @click="emit('remake',row)">再次制作</t-button></t-space></template>
 </t-table></div>
</template>
<style scoped>
.output-table__operator {display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;}
.output-table__related {display:flex;flex-direction:column;gap:3px;min-width:0;}
.output-table__related strong,.output-table__related :deep(.t-link){font-size:13px;font-weight:500;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;max-width:100%;}
.output-table__related span{font-size:12px;color:var(--wt-text-tertiary);}
</style>
