<script setup>
import { computed, ref, watch } from 'vue'
import ResourceOverlay from '../../../shared/ui/resource/ResourceOverlay.vue'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import ComposeOutputTable from './ComposeOutputTable.vue'
import { createComposeClient } from '../../../shared/api/compose.js'
const props=defineProps({visible:Boolean,material:Object,videoState:Function,canDownload:Function,downloadingId:Number,categoryName:Function,busy:Boolean,publishingReason:String,result:String})
const emit=defineEmits(['update:visible','detail','download','downloads','remake','batch'])
const client=createComposeClient(),rows=ref([]),total=ref(0),page=ref(1),pageSize=ref(10),loading=ref(false),error=ref(''),selectedKeys=ref([]),knownRows=ref({})
const selectedRows=computed(()=>selectedKeys.value.map(id=>knownRows.value[id]).filter(Boolean))
let sequence=0
async function load(){const current=++sequence;loading.value=true;error.value='';try{const data=await client.listComposeOutputs({material_id:props.material.material_id,page:page.value,page_size:pageSize.value});if(current!==sequence)return;rows.value=data.items||[];total.value=data.total||0;for(const row of rows.value)knownRows.value[row.id]=row}catch(e){if(current===sequence)error.value=e?.message||'读取素材成片失败'}finally{if(current===sequence)loading.value=false}}
watch(()=>[props.visible,props.material?.material_id],([visible])=>{if(visible){page.value=1;selectedKeys.value=[];knownRows.value={};load()}else sequence++})
function paginate(data){page.value=data.current;pageSize.value=data.pageSize;load()}
</script>
<template>
 <ResourceOverlay kind="drawer" :visible="visible" title="素材成片详情" :width="1120" :footer="false" @close="emit('update:visible',false)">
  <div class="output-material"><div><h3>{{ material?.material_title || '来源素材未记录' }}</h3><p>素材 ID {{ material?.material_id }} · {{ categoryName?.(material?.category_id) || '—' }} · {{ total }} 个实际成片</p><p>该素材在不同策略、任务批次下生成的成片。</p></div>
   <FeedbackAlert v-if="error" theme="error" :message="error" />
   <FeedbackAlert v-if="result" theme="info" :message="result" />
   <div v-if="selectedKeys.length" class="wt-selection-toolbar"><span>已选 {{ selectedKeys.length }} 项</span><t-link theme="primary" @click="selectedKeys=[]">清空选择</t-link><t-button theme="primary" :loading="busy" :disabled="!selectedRows.some(canDownload)" @click="emit('downloads',selectedRows)">下载</t-button><span :title="publishingReason"><t-button variant="outline" disabled>去发布</t-button></span><span class="output-material__reason">{{ publishingReason }}</span></div>
   <t-loading :loading="loading"><ComposeOutputTable :rows="rows" v-model:selected-keys="selectedKeys" material-mode :video-state="videoState" :can-download="canDownload" :downloading-id="downloadingId" :category-name="categoryName" @detail="emit('detail',$event)" @download="emit('download',$event)" @remake="emit('remake',$event)" @batch="emit('batch',$event)" /></t-loading>
   <div class="pagination-bar"><t-pagination :current="page" :page-size="pageSize" :total="total" :page-size-options="[10,20,50]" @change="paginate" /></div>
  </div>
 </ResourceOverlay>
</template>
<style scoped>
.output-material{display:grid;gap:16px;min-width:0;}.output-material>*{min-width:0;}.output-material h3{margin:0 0 8px;font-size:16px;}.output-material p{margin:5px 0;color:var(--wt-text-secondary);font-size:13px;}.output-material__reason{font-size:12px;color:var(--wt-text-tertiary);}
</style>
