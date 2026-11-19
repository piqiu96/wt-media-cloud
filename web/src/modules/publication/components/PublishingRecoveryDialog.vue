<script setup>
import { ref, watch } from 'vue';
import ResourceRequiredLabel from '../../../shared/ui/resource/ResourceRequiredLabel.vue';
import ResourceOverlay from '../../../shared/ui/resource/ResourceOverlay.vue';
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue';
const props=defineProps({visible:Boolean,task:Object,busy:Boolean,error:String});
const emit=defineEmits(['update:visible','submit']);
const conclusion=ref(''),reason=ref(''),basis=ref(''),validation=ref('');
watch(()=>props.visible,value=>{if(value){conclusion.value='';reason.value='';basis.value='';validation.value='';}});
function submit(){
 validation.value='';
 if(!conclusion.value||!reason.value.trim()||basis.value.trim().length<10){validation.value='请选择核实结论，填写原因及实际核实依据（至少 10 字）。';return;}
 emit('submit',{attempt_id:props.task.current_attempt_id,expected_revision:props.task.revision,conclusion:conclusion.value,
  evidence:{platform_uid:props.task.platform_account_uid,reason:reason.value.trim(),basis:basis.value.trim(),scene_closed:conclusion.value==='failed',no_publishable_work:conclusion.value==='failed'}});
}
</script>
<template>
 <ResourceOverlay form-actions :visible="visible" title="核实发布结果" :width="640" :busy="busy" @update:visible="emit('update:visible',$event)">
  <FeedbackAlert v-if="error || validation" theme="error" :message="error || validation" />
  <div class="publication-verification">
   <p>任务 {{task?.id}} · 原尝试 {{task?.current_attempt_id}}。请检查原账号稿件及执行证据，核实不会重新投稿。</p>
   <label><ResourceRequiredLabel>核实结论</ResourceRequiredLabel><t-select v-model="conclusion" aria-label="核实结论" :disabled="busy" :options="[{label:'确认未发布',value:'failed'},{label:'仍无法判断',value:'uncertain'}]" /></label>
   <p v-if="conclusion === 'failed'">只有确认无可继续发布的稿件、原执行进程与浏览器已安全结束，才能记录为失败。没有提交标记不能单独作为依据。</p>
   <label><ResourceRequiredLabel>失败原因 / 核实原因</ResourceRequiredLabel><t-input v-model="reason" aria-label="核实原因" :maxlength="200" :disabled="busy" /></label>
   <label><ResourceRequiredLabel>核实依据</ResourceRequiredLabel><t-textarea v-model="basis" aria-label="核实依据" :maxlength="2000" :autosize="{minRows:4,maxRows:8}" :disabled="busy" /></label>
  </div>
  <template #footer><t-button theme="default" variant="outline" :disabled="busy" @click="emit('update:visible',false)">取消</t-button><t-button :loading="busy" @click="submit">保存核实结果</t-button></template>
 </ResourceOverlay>
</template>
<style scoped>
.publication-verification{display:flex;flex-direction:column;gap:16px;}
.publication-verification label{display:flex;flex-direction:column;gap:8px;font-size:13px;font-weight:500;line-height:20px;}
.publication-verification p{margin:0;line-height:1.6;color:var(--wt-text-secondary);}
</style>
