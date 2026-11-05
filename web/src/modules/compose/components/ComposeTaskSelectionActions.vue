<script setup>
import { computed, ref } from 'vue'
import { DialogPlugin } from 'tdesign-vue-next'
import { createComposeClient } from '../../../shared/api/compose.js'
import ResourceSelectionToolbar from '../../../shared/ui/resource/ResourceSelectionToolbar.vue'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import { bulkResultMessage, selectionSkipSummary, taskActionEligible } from '../batchActions.js'
const props = defineProps({ tasks: {type:Array,default:()=>[]}, canRetry:Boolean })
const emit=defineEmits(['clear','changed','busy'])
const client=createComposeClient(),busy=ref(false),result=ref(null),error=ref('')
const retryCount=computed(()=>props.tasks.filter(row=>taskActionEligible(row,'retry',props.canRetry)).length)
const cancelCount=computed(()=>props.tasks.filter(row=>taskActionEligible(row,'cancel',props.canRetry)).length)
function command(action) {
  const tasks=[...props.tasks]
  const count=action==='retry'?retryCount.value:cancelCount.value
  if(!count || busy.value || tasks.length>200) return
  const skips=selectionSkipSummary(tasks,action,props.canRetry).map(item=>`${item.reason} ${item.count} 条`).join('；')
  const dialog=DialogPlugin.confirm({header:action==='retry'?'重试所选任务':'取消所选任务',body:`针对明确所选的 ${tasks.length} 条任务，当前可${action==='retry'?'重试':'取消'} ${count} 条。${skips?`跳过：${skips}。`:''}${action==='cancel'?'运行任务先受理停止请求，确认停止后才会变为已取消。':'使用原任务及原配置，不创建新批次。'}提交时逐项重新核验。`,confirmBtn:{content:action==='retry'?'重试':'取消任务',theme:action==='retry'?'primary':'danger'},onConfirm:async()=>{
    if(busy.value)return
    busy.value=true;emit('busy',true);error.value=''
    try {result.value=await(action==='retry'?client.batchRetryComposeTasks({task_ids:tasks.map(row=>row.id)}):client.batchCancelComposeTasks({task_ids:tasks.map(row=>row.id)}));result.value.action=action;emit('changed');emit('clear');dialog.hide()}
    catch(e){error.value=e?.message || '批量操作失败';dialog.hide()}
    finally{busy.value=false;emit('busy',false)}
  }})
}
</script>
<template>
  <div v-if="tasks.length || result || error" class="task-selection-actions">
    <ResourceSelectionToolbar :count="tasks.length" unit="条任务" :hint="tasks.length>200 ? '每次最多操作 200 条' : ''" @clear="!busy && emit('clear')"><t-button v-if="retryCount" size="small" variant="outline" class="wt-secondary-button" :disabled="tasks.length>200 || busy" @click="command('retry')">重试（{{ retryCount }}）</t-button><t-button v-if="cancelCount" size="small" variant="outline" class="wt-secondary-button wt-danger-button" :disabled="tasks.length>200 || busy" @click="command('cancel')">取消（{{ cancelCount }}）</t-button></ResourceSelectionToolbar>
    <FeedbackAlert v-if="error" theme="error" :message="error" />
    <FeedbackAlert v-if="result" :theme="result.failed || result.skipped ? 'warning':'success'" :message="bulkResultMessage(result,result.action)" />
    <div v-if="result?.items?.some(row=>row.status!=='accepted')" class="task-selection-actions__results"><p v-for="row in result.items.filter(row=>row.status!=='accepted')" :key="row.task_id">任务 {{ row.task_id }} · {{ row.reason }}</p></div>
  </div>
</template>
<style scoped>
.task-selection-actions { display:grid; gap:12px; margin:12px 0; }
.task-selection-actions__bar { margin-bottom:0; }
.task-selection-actions__results { max-height:160px; overflow:auto; color:var(--wt-text-secondary); font-size:13px; }
</style>
