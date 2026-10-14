<script setup>
import { computed, ref, watch } from 'vue'
import { createComposeClient } from '../../../shared/api/compose.js'
import { schemaFields } from '../schemaFields.js'
import { feedback } from '../../../shared/ui/feedback/index.js'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import FormError from '../../../shared/ui/feedback/FormError.vue'
import MaterialCover from '../../materials/components/MaterialCover.vue'

// 素材范围由调用方决定；每条素材使用独立幂等键，参数留空时使用策略值。
const props = defineProps({
  visible: { type: Boolean, default: false },
  materials: { type: Array, default: () => [] },
})
const emit = defineEmits(['update:visible', 'created'])

const client = createComposeClient()
const strategies = ref([])
const templates = ref([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const strategyError = ref('')
const strategyId = ref(null)
const overrides = ref({})
// 对话框存续期间复用同一批 request_key，关闭后重新打开才生成新的一批。
const requestKeys = ref({})

const strategyOptions = computed(() => strategies.value
  .filter((item) => item.status === 'active')
  .map((item) => ({ value: item.id, label: `${item.name} · ${item.game_id}` })))

const selectedStrategy = computed(() => strategies.value.find((item) => item.id === strategyId.value) || null)

const fields = computed(() => {
  const strategy = selectedStrategy.value
  if (!strategy) return []
  const template = templates.value.find((item) => item.id === strategy.compose_template_id && item.version === strategy.compose_template_version)
  return schemaFields(template?.schema).filter((field) => field.key !== 'guide_video_url')
})

function generateKeys(materials) {
  const keys = {}
  for (const material of materials) keys[material.id] = crypto.randomUUID()
  return keys
}

watch(() => props.visible, (visible) => {
  if (!visible) return
  error.value = ''
  strategyError.value = ''
  strategyId.value = null
  overrides.value = {}
  requestKeys.value = generateKeys(props.materials)
  load()
})

async function load() {
  loading.value = true
  try {
    const [strategyRows, templateRows] = await Promise.all([client.listStrategies(), client.listTemplates()])
    strategies.value = Array.isArray(strategyRows) ? strategyRows : []
    templates.value = Array.isArray(templateRows) ? templateRows : []
  } catch (e) {
    error.value = e?.message || '读取合成策略失败'
  } finally {
    loading.value = false
  }
}

// 只把填了的字段放进 override：留空意味着「用策略里的值」，把空串发上去反而
// 会被 schema 当成一个显式的空值拒绝。
const overridePayload = computed(() => {
  const payload = {}
  for (const field of fields.value) {
    const raw = overrides.value[field.key]
    if (raw === undefined || raw === null || raw === '') continue
    payload[field.key] = field.type === 'integer' || field.type === 'number' ? Number(raw) : raw
  }
  return payload
})

async function submit() {
  if (saving.value || !props.materials.length) return
  if (!strategyId.value) {
    strategyError.value = '请选择合成策略'
    return
  }
  saving.value = true
  strategyError.value = ''
  try {
    const body = {
      strategy_id: strategyId.value,
      items: props.materials.map((material) => ({ material_id: material.id, request_key: requestKeys.value[material.id] })),
    }
    if (Object.keys(overridePayload.value).length > 0) body.config_override = overridePayload.value
    const result = await client.createComposeTasks(body)
    emit('created', result || { created: [], failed: [] })
    emit('update:visible', false)
  } catch (e) {
    feedback.toast.error(e?.message || '创建合成任务失败')
  } finally {
    saving.value = false
  }
}

function close() {
  emit('update:visible', false)
}
</script>

<template>
  <t-dialog
    :visible="visible"
    header="创建合成任务"
    width="760px"
    :confirm-btn="{ content: '创建合成任务', theme: 'primary', loading: saving }"
    :cancel-btn="{ content: '取消', class: 'wt-secondary-button', variant: 'outline' }"
    :close-on-overlay-click="false"
    @confirm="submit"
    @close="close"
  >
    <t-loading :loading="loading" :show-overlay="true">
      <div class="compose-create">
        <FeedbackAlert v-if="error" theme="error" :message="error" class="compose-create__load-error" />
        <p v-if="materials.length === 0" class="compose-create__empty">没有选择素材，请回到列表勾选后重试。</p>
        <template v-else>
          <section class="compose-create__section">
            <div class="compose-create__heading"><h4 class="compose-create__title"><span>1</span> 已选素材</h4><strong>{{ materials.length }} 条</strong></div>
            <p class="compose-create__hint">每条素材独立生成一条合成任务。</p>
            <ul class="compose-create__materials">
              <li v-for="material in materials" :key="material.id">
                <MaterialCover :url="material.cover_url" />
                <span class="compose-create__material-text"><strong>{{ material.title || '未命名素材' }}</strong><small>素材 ID {{ material.id }}</small></span>
              </li>
            </ul>
          </section>
          <section class="compose-create__section">
            <h4 class="compose-create__title"><span>2</span> 合成策略</h4>
            <t-select v-model="strategyId" :options="strategyOptions" clearable placeholder="选择要使用的合成策略" :status="strategyError ? 'error' : undefined" aria-describedby="compose-create-strategy-error" @change="strategyError = ''" />
            <FormError id="compose-create-strategy-error" :message="strategyError" />
            <p class="compose-create__hint">只有启用中的策略可选；参数留空即使用策略里保存的值。</p>
          </section>
          <section v-if="fields.length > 0" class="compose-create__section">
            <h4 class="compose-create__title"><span>3</span> 覆盖参数（可选）</h4>
            <div class="compose-create__params">
              <label v-for="field in fields" :key="field.key" class="compose-create__field">
                <span>{{ field.label }}</span>
                <t-input-number
                  v-if="field.type === 'integer' || field.type === 'number'"
                  v-model="overrides[field.key]"
                  :min="field.minimum"
                  theme="column"
                  :placeholder="field.required ? '必填（策略值）' : '策略值'"
                  style="width: 200px"
                />
                <t-input v-else v-model="overrides[field.key]" clearable :placeholder="field.required ? '必填（策略值）' : '策略值'" style="width: 200px" />
              </label>
            </div>
          </section>
        </template>
      </div>
    </t-loading>
  </t-dialog>
</template>

<style scoped>
.compose-create__empty { color: var(--wt-text-secondary); }
.compose-create__load-error { margin-bottom: 12px; }
.compose-create__section { margin-bottom: 16px; padding: 16px; border: 1px solid var(--wt-border); border-radius: 10px; }
.compose-create__heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.compose-create__heading strong { color: var(--wt-text-secondary); font-size: 12px; }
.compose-create__title { display: flex; align-items: center; gap: 8px; margin: 0 0 8px; font-size: 14px; font-weight: 600; color: var(--wt-text-primary); }
.compose-create__title span { display: inline-flex; align-items: center; justify-content: center; width: 20px; height: 20px; border-radius: 6px; background: var(--wt-primary); color: #fff; font-size: 12px; }
.compose-create__hint { margin: 8px 0 0; font-size: 12px; color: var(--wt-text-tertiary); }
.compose-create__params { display: flex; flex-wrap: wrap; gap: 12px; }
.compose-create__field { display: flex; align-items: center; gap: 8px; color: var(--wt-text-secondary); font-size: 13px; }
.compose-create__materials { margin: 12px 0 0; padding: 0; list-style: none; max-height: 176px; overflow: auto; }
.compose-create__materials li { display: flex; align-items: center; gap: 10px; padding: 7px 0; border-top: 1px solid var(--wt-border); font-size: 13px; }
.compose-create__material-text { display: flex; flex-direction: column; min-width: 0; gap: 3px; }
.compose-create__material-text strong { color: var(--wt-text-primary); font-weight: 500; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
.compose-create__material-text small { color: var(--wt-text-tertiary); font-size: 12px; }
</style>
