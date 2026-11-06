<script setup>
import { computed, ref, watch } from 'vue'
import ResourceSelectionToolbar from '../../../shared/ui/resource/ResourceSelectionToolbar.vue'
import ResourceOverlay from '../../../shared/ui/resource/ResourceOverlay.vue'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import { materialOutputQuery } from '../outputScope.js'
import ComposeOutputTable from './ComposeOutputTable.vue'
import { createComposeClient } from '../../../shared/api/compose.js'
const props=defineProps({visible:Boolean,material:Object,scope:{type:Object,default:()=>({})},videoState:Function,canDownload:Function,downloadReason:Function,downloadingId:Number,categoryName:Function,busy:Boolean,publishingReason:String,result:String})
const emit=defineEmits(['update:visible','detail','download','downloads','remake','batch'])
const client=createComposeClient(),rows=ref([]),total=ref(0),page=ref(1),pageSize=ref(10),loading=ref(false),error=ref(''),selectedKeys=ref([]),knownRows=ref({})
const allForMaterial=ref(false)
const selectedRows=computed(()=>selectedKeys.value.map(id=>knownRows.value[id]).filter(Boolean))
let sequence=0
async function load(){const current=++sequence;loading.value=true;error.value='';try{const data=await client.listComposeOutputs(materialOutputQuery(allForMaterial.value ? {} : props.scope, props.material.material_id, page.value, pageSize.value));if(current!==sequence)return;rows.value=data.items||[];total.value=data.total||0;for(const row of rows.value)knownRows.value[row.id]=row}catch(e){if(current===sequence)error.value=e?.message||'读取素材成片失败'}finally{if(current===sequence)loading.value=false}}
watch(()=>[props.visible,props.material?.material_id],([visible])=>{if(visible){allForMaterial.value=false;page.value=1;selectedKeys.value=[];knownRows.value={};load()}else sequence++})
function viewAll(){allForMaterial.value=true;page.value=1;selectedKeys.value=[];knownRows.value={};load()}
function paginate(data){page.value=data.current;pageSize.value=data.pageSize;load()}
</script>
<template>
 <ResourceOverlay kind="drawer" :visible="visible" title="素材成片详情" :width="960" :footer="false" @close="emit('update:visible',false)">
  <div class="output-material wt-resource-page resource-visual-approved production-page"><div><h3>{{ material?.material_title || '来源素材未记录' }}</h3><p>素材 ID {{ material?.material_id }} · {{ categoryName?.(material?.category_id) || '—' }} · {{ total }} 个实际成片</p></div>
   <p v-if="!allForMaterial && Object.values(scope).some(value => value !== '' && value != null)">沿用列表筛选 <t-link theme="primary" @click="viewAll">查看该素材全部成片</t-link></p>
   <FeedbackAlert v-if="error" theme="error" :message="error" />
   <FeedbackAlert v-if="result" theme="info" :message="result" />
   <ResourceSelectionToolbar :count="selectedKeys.length" unit="个成片" @clear="selectedKeys=[]"><t-button variant="outline" :loading="busy" :disabled="!selectedRows.some(canDownload)" :title="selectedRows.map(downloadReason).filter(Boolean).join('；')" @click="emit('downloads',selectedRows)">下载（{{ selectedKeys.length }}）</t-button><span :title="publishingReason"><t-button variant="outline" disabled>去发布</t-button></span></ResourceSelectionToolbar>
   <t-loading :loading="loading"><ComposeOutputTable :rows="rows" v-model:selected-keys="selectedKeys" material-mode :video-state="videoState" :can-download="canDownload" :download-reason="downloadReason" :downloading-id="downloadingId" :category-name="categoryName" @detail="emit('detail',$event)" @download="emit('download',$event)" @remake="emit('remake',$event)" @batch="emit('batch',$event)" /></t-loading>
   <div class="pagination-bar"><t-pagination :show-jumper="false" :current="page" :page-size="pageSize" :total="total" :page-size-options="[10,20,50]" @change="paginate" /></div>
  </div>
 </ResourceOverlay>
</template>
<style scoped>
.output-material{display:grid;gap:16px;min-width:0;}.output-material>*{min-width:0;}.output-material h3{margin:0 0 8px;font-size:16px;}.output-material p{margin:5px 0;color:var(--wt-text-secondary);font-size:13px;}.output-material__reason{font-size:12px;color:var(--wt-text-tertiary);}
</style>
