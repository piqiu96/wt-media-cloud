<script setup>
import { computed, ref, watch } from 'vue'
import ResourceOverlay from '../../../shared/ui/resource/ResourceOverlay.vue'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import { createComposeClient } from '../../../shared/api/compose.js'
import { createMaterialsClient } from '../../../shared/api/materials.js'
import { publicObjectReadUrl } from '../../../shared/utils/objectReadUrl.js'
import { relatedTarget } from '../relatedTarget.js'
import ComposeTaskDetailDrawer from './ComposeTaskDetailDrawer.vue'
import ComposeTaskBatchDetail from './ComposeTaskBatchDetail.vue'
import ComposeStrategySummary from './ComposeStrategySummary.vue'
import ComposeOutputDetail from './ComposeOutputDetail.vue'
import MaterialDetailDrawer from '../../materials/MaterialDetailDrawer.vue'
// Each association owns its instance and draft-free state. Returning to an ancestor
// closes its descendants rather than changing an object hidden under another layer.
const props=defineProps({visible:Boolean,kind:String,entityId:Number,canRetry:Boolean,ancestors:{type:Array,default:()=>[]}})
const emit=defineEmits(['update:visible','back'])
const client=createComposeClient(),materials=createMaterialsClient()
const child=ref(null),row=ref(null),loading=ref(false),error=ref(''),templates=ref([]),teams=ref([]),categories=ref([]),videoUrl=ref('')
const current=computed(()=>({kind:props.kind,id:props.entityId,key:`${props.kind}-${props.entityId}`}))
const path=computed(()=>[...props.ancestors,current.value])
const title=computed(()=>({strategy:'策略详情',output:'成片详情'})[props.kind]||'详情')
let sequence=0
function open(kind,id){const target=relatedTarget(path.value,kind,id);if(!target)return;if(target.back)back(target.back);else child.value=target}
function back(key){if(key===current.value.key)child.value=null;else emit('back',key)}
function close(){child.value=null;emit('update:visible',false)}
async function load(){
 const request=++sequence
 if(!props.visible||['task','batch'].includes(props.kind))return
 loading.value=true;error.value='';row.value=null;videoUrl.value=''
 try{
  if(props.kind==='strategy'){
   const [detail,definitions,teamRows,categoryRows]=await Promise.all([client.getStrategy(props.entityId),client.listTemplates(),client.listTeams(),client.listCategories()])
   if(request!==sequence)return
   row.value=detail;templates.value=definitions||[];teams.value=teamRows||[];categories.value=categoryRows||[]
  }else if(props.kind==='material'){
   const [detail,categoryRows]=await Promise.all([materials.get(props.entityId),client.listCategories()])
   if(request!==sequence)return
   row.value=detail;categories.value=categoryRows||[]
  }else if(props.kind==='output'){
   const detail=await client.getComposeOutput(props.entityId)
   if(request!==sequence)return
   row.value=detail
   const link=await client.getComposeOutputVideoUrl(props.entityId).catch(()=>null)
   if(request===sequence)videoUrl.value=publicObjectReadUrl(link?.url)
  }
 }catch(e){if(request===sequence)error.value=e?.message||'读取详情失败'}finally{if(request===sequence)loading.value=false}
}
watch(()=>[props.visible,props.kind,props.entityId],()=>{++sequence;child.value=null;if(props.visible)load()},{immediate:true})
</script>
<template>
 <ComposeTaskDetailDrawer v-if="kind==='task'" :visible="visible" :task-id="entityId" :can-retry="canRetry" @update:visible="close" @material="open('material',$event)" @strategy="open('strategy',$event)" @output="open('output',$event)" @batch="open('batch',$event)" />
 <ComposeTaskBatchDetail v-else-if="kind==='batch'" :visible="visible" :batch-id="entityId" :can-retry="canRetry" @update:visible="close" @task="open('task',$event.id)" />
 <MaterialDetailDrawer v-else-if="kind==='material'" :visible="visible" :material="row" :loading="loading" :error="error" @retry="load" :categories="categories" mode="readonly" :show-page-link="false" @update:visible="close" />
 <ResourceOverlay v-else kind="drawer" :visible="visible" :title="title" :width="kind==='strategy'?960:800" :footer="false" @close="close">
  <FeedbackAlert v-if="error" theme="error" :message="error" /><t-button v-if="error" variant="text" @click="load">重试</t-button>
  <t-loading :loading="loading">
   <ComposeStrategySummary v-if="kind==='strategy'&&row" :strategy="row" :templates="templates" :team="teams.find(item=>item.id===row.team_id)?.name" :category="categories.find(item=>item.id===row.category_id)?.name" />
   <ComposeOutputDetail v-if="kind==='output'&&row" :detail="row" :preview-url="videoUrl" management linkable @open-material="open('material',$event)" @open-strategy="open('strategy',$event)" @open-task="open('task',$event)" @open-batch="open('batch',$event)" />
  </t-loading>
 </ResourceOverlay>
 <ComposeRelatedDetail v-if="child" :key="child.key" :visible="visible" :kind="child.kind" :entity-id="child.id" :ancestors="path" :can-retry="canRetry" @update:visible="child=null" @back="back" />
</template>
