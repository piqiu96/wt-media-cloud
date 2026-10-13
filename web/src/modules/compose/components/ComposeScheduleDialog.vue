<script setup>
import { ref, watch } from 'vue'
import { feedback } from '../../../shared/ui/feedback/index.js'
import { createComposeClient } from '../../../shared/api/compose.js'
import { scheduleFormFor, schedulePayload } from '../scheduleForm.js'

const props = defineProps({ strategy: { type: Object, default: null } })
const emit = defineEmits(['close', 'saved'])
const client = createComposeClient()
const form = ref(scheduleFormFor())
const errors = ref({})
const saving = ref(false)
const requestError = ref('')

watch(() => props.strategy, (value) => {
  form.value = scheduleFormFor(value)
  errors.value = {}
  requestError.value = ''
})

async function save() {
  const result = schedulePayload(form.value)
  errors.value = result.errors
  if (Object.keys(result.errors).length) return
  saving.value = true
  requestError.value = ''
  try {
    const stored = await client.setStrategySchedule(props.strategy.id, result.payload)
    emit('saved', stored)
    feedback.toast.success('自动执行计划已保存')
  } catch (error) {
    requestError.value = error?.message || '计划保存失败'
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <t-dialog :visible="Boolean(strategy)" :header="`自动执行计划 · ${strategy?.name || ''}`" width="600px" :footer="false" @close="emit('close')">
    <div class="schedule-editor">
      <p class="schedule-editor__context">候选素材来自策略创建人的“我的素材”，限当前团队与游戏；已由该策略生产的素材不会再次生产。</p>
      <t-form label-width="116px">
        <t-form-item label="执行方式" required-mark>
          <div class="schedule-editor__field">
            <t-select v-model="form.mode" :status="errors.mode ? 'error' : undefined">
              <t-option value="manual" label="手动执行" />
              <t-option value="interval" label="每 N 分钟" />
              <t-option value="daily" label="每天固定时间" />
            </t-select>
            <p v-if="errors.mode" class="schedule-editor__error">{{ errors.mode }}</p>
          </div>
        </t-form-item>
        <t-form-item v-if="form.mode === 'interval'" label="执行间隔" required-mark>
          <div class="schedule-editor__field"><t-input-number v-model="form.interval_minutes" :min="1" :max="1440" suffix="分钟" :status="errors.interval_minutes ? 'error' : undefined" /><p v-if="errors.interval_minutes" class="schedule-editor__error">{{ errors.interval_minutes }}</p></div>
        </t-form-item>
        <t-form-item v-if="form.mode === 'daily'" label="每日时间" required-mark>
          <div class="schedule-editor__field"><t-input v-model="form.daily_time" placeholder="09:30" :status="errors.daily_time ? 'error' : undefined" /><p v-if="errors.daily_time" class="schedule-editor__error">{{ errors.daily_time }}</p></div>
        </t-form-item>
        <t-form-item label="时区" required-mark>
          <div class="schedule-editor__field"><t-input v-model="form.timezone" placeholder="Asia/Shanghai" :status="errors.timezone ? 'error' : undefined" /><p v-if="errors.timezone" class="schedule-editor__error">{{ errors.timezone }}</p></div>
        </t-form-item>
        <t-form-item label="单次上限" required-mark>
          <div class="schedule-editor__field"><t-input-number v-model="form.per_run_limit" :min="1" :max="100" suffix="条" :status="errors.per_run_limit ? 'error' : undefined" /><p v-if="errors.per_run_limit" class="schedule-editor__error">{{ errors.per_run_limit }}</p></div>
        </t-form-item>
        <t-form-item label="每日上限" required-mark>
          <div class="schedule-editor__field"><t-input-number v-model="form.daily_limit" :min="1" :max="1000" suffix="条" :status="errors.daily_limit ? 'error' : undefined" /><p v-if="errors.daily_limit" class="schedule-editor__error">{{ errors.daily_limit }}</p></div>
        </t-form-item>
        <t-form-item v-if="form.mode !== 'manual'" label="自动执行"><t-switch v-model="form.enabled" /></t-form-item>
      </t-form>
      <p class="schedule-editor__note">启用或修改计划后，将从下一个计划时间开始；保存本身不会创建合成任务。</p>
      <p v-if="requestError" class="schedule-editor__error" role="alert">{{ requestError }}</p>
      <div class="schedule-editor__actions"><t-button variant="outline" class="wt-secondary-button" @click="emit('close')">取消</t-button><t-button theme="primary" :loading="saving" @click="save">保存计划</t-button></div>
    </div>
  </t-dialog>
</template>

<style scoped>
.schedule-editor__context, .schedule-editor__note { margin: 0 0 16px; color: var(--wt-text-tertiary); font-size: 12px; line-height: 1.6; }
.schedule-editor__note { margin: 8px 0 0; }
.schedule-editor__field { display: flex; flex-direction: column; gap: 4px; width: 100%; }
.schedule-editor__error { margin: 0; color: var(--td-error-color, #d54941); font-size: 12px; }
.schedule-editor__actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 18px; }
</style>
