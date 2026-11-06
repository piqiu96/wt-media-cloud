<script setup>
import { ref, watch } from 'vue'
import ResourceOverlay from '../../../shared/ui/resource/ResourceOverlay.vue'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import ComposeTaskDetailContent from './ComposeTaskDetailContent.vue'
import { createComposeClient } from '../../../shared/api/compose.js'
import { useAutoRefresh } from '../../../shared/utils/useAutoRefresh.js'
const props=defineProps({visible:Boolean,taskId:Number,canRetry:Boolean})
const emit=defineEmits(['update:visible','material','strategy','output','batch'])
const client=createComposeClient(),detail=ref(null),attempts=ref([]),templates=ref([]),outputSummary=ref(null),loading=ref(false),error=ref(''),attemptsError=ref(''),tab=ref('overview')
let sequence=0
async function load(quiet=false){
 const request=++sequence
 if(!props.visible || !props.taskId)return
 if(!quiet)loading.value=true
 try{
  const row=await client.getComposeTask(props.taskId)
  if(request!==sequence)return
  detail.value=row;error.value=''
  try { const data=await client.listComposeTaskAttempts(row.id);if(request===sequence){attempts.value=(data||[]).sort((a,b)=>b.attempt-a.attempt);attemptsError.value=''} }catch(e){if(request===sequence)attemptsError.value=e?.message||'执行记录读取失败'}
  if(row.status==='success'&&row.composite_output_id){const data=await client.getComposeOutput(row.composite_output_id).catch(()=>null);if(request===sequence)outputSummary.value=data?.output||null}
 }catch(e){if(request===sequence)error.value=e?.message||'任务读取失败'}finally{if(request===sequence)loading.value=false}
}
watch(()=>[props.visible,props.taskId],()=>{++sequence;if(!props.visible)return;detail.value=null;attempts.value=[];outputSummary.value=null;tab.value='overview';load();client.listTemplates().then(rows=>templates.value=rows||[]).catch(()=>{})},{immediate:true})
useAutoRefresh(()=>load(true),{shouldSkip:()=>!props.visible||!['pending','running'].includes(detail.value?.status)})
</script>
<template>
 <ResourceOverlay kind="drawer" :visible="visible" title="任务详情" :width="960" :footer="false" @close="emit('update:visible',false)">
  <FeedbackAlert v-if="error" theme="error" :message="error" />
  <t-loading :loading="loading"><ComposeTaskDetailContent v-if="detail" v-model:tab="tab" :detail="detail" :templates="templates" :attempts="attempts" :attempts-error="attemptsError" :output-summary="outputSummary" :can-retry="canRetry" @material="emit('material',$event)" @strategy="emit('strategy',$event)" @output="emit('output',$event)" @batch="emit('batch',$event)" /></t-loading>
 </ResourceOverlay>
</template>
