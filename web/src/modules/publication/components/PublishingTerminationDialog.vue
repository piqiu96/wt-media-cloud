<script setup>
import { ref, watch } from 'vue';
import ResourceOverlay from '../../../shared/ui/resource/ResourceOverlay.vue';
import ResourceRequiredLabel from '../../../shared/ui/resource/ResourceRequiredLabel.vue';
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue';
const props = defineProps({ visible: Boolean, task: Object, action: String, busy: Boolean, error: String, submitted: Boolean });
const emit = defineEmits(['update:visible', 'submit']);
const reason = ref(''), basis = ref(''), sceneClosed = ref(false), noPublishableWork = ref(false), validation = ref('');
watch(() => props.visible, visible => {
  if (visible) { reason.value = ''; basis.value = ''; sceneClosed.value = false; noPublishableWork.value = false; validation.value = ''; }
});
function submit() {
  if (props.busy || props.submitted) return;
  validation.value = '';
  if (!reason.value.trim()) { validation.value = '请填写' + (props.action === 'abandon' ? '放弃' : '终止') + '原因。'; return; }
  const input = { expected_revision: props.task.revision };
  if (props.action === 'cancel') input.reason = reason.value.trim();
  if (props.action === 'abandon') {
    if (!reason.value.trim() || basis.value.trim().length < 10 || !sceneClosed.value || !noPublishableWork.value) {
      validation.value = '请填写原因和实际核实依据（至少 10 字），并确认原现场结束、没有可发布稿件。';
      return;
    }
    input.attempt_id = props.task.current_attempt_id;
    input.evidence = { platform_uid: props.task.platform_account_uid, reason: reason.value.trim(), basis: basis.value.trim(), scene_closed: sceneClosed.value, no_publishable_work: noPublishableWork.value };
  }
  emit('submit', input);
}
</script>
<template>
  <ResourceOverlay form-actions :visible="visible" :title="action === 'abandon' ? '放弃发布任务' : '终止发布任务'" :width="640" :busy="busy" @update:visible="emit('update:visible', $event)">
    <FeedbackAlert v-if="error || validation" theme="error" :message="error || validation" />
    <div class="publication-termination">
      <p>任务 {{ task?.id }} · {{ task?.resources?.account_name || '原发布账号' }}</p>
      <template v-if="action === 'abandon'">
        <p>请先核实原尝试 {{ task?.current_attempt_id }}。放弃后旧任务永久结束并释放成片占用，不能恢复执行；已有投稿或仍不确定时不能放弃。</p>
        <label><ResourceRequiredLabel>放弃原因</ResourceRequiredLabel><t-input v-model="reason" aria-label="放弃原因" :maxlength="200" :disabled="busy || submitted" /></label>
        <label><ResourceRequiredLabel>核实依据</ResourceRequiredLabel><t-textarea v-model="basis" aria-label="放弃核实依据" :maxlength="2000" :autosize="{ minRows: 4, maxRows: 8 }" :disabled="busy || submitted" /></label>
        <t-checkbox v-model="sceneClosed" :disabled="busy || submitted">已核实原执行进程与浏览器现场安全结束</t-checkbox>
        <t-checkbox v-model="noPublishableWork" :disabled="busy || submitted">已核实原账号没有可继续发布的稿件或公开作品</t-checkbox>
      </template>
      <template v-else><p>仅未领取且安全的任务可终止。终止后保留成片占用，原任务不恢复；这不会撤回平台投稿。</p><label><ResourceRequiredLabel>终止原因</ResourceRequiredLabel><t-input v-model="reason" aria-label="终止原因" :maxlength="200" :disabled="busy || submitted" /></label></template>
    </div>
    <template #footer>
      <t-button theme="default" variant="outline" :disabled="busy" @click="emit('update:visible', false)">{{ submitted ? '返回查询结果' : '取消' }}</t-button>
      <t-button theme="danger" :loading="busy" :disabled="submitted" @click="submit">{{ action === 'abandon' ? '确认放弃' : '确认终止' }}</t-button>
    </template>
  </ResourceOverlay>
</template>
<style scoped>
.publication-termination { display: flex; flex-direction: column; gap: 16px; }
.publication-termination label { display: flex; flex-direction: column; gap: 8px; font-size: 13px; font-weight: 500; line-height: 20px; }
.publication-termination p { margin: 0; line-height: 1.6; color: var(--wt-text-secondary); }
</style>
