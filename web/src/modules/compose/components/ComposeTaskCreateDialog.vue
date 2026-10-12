<script setup>
import { computed, ref, watch } from 'vue'
import { createComposeClient } from '../../../shared/api/compose.js'
import { schemaFields } from '../schemaFields.js'

// CHG-20261013-082：从「我的素材」勾选或行内「加入合成」打开的批量创建对话框。
// 职责只有三件：选一个可用策略、按模板 schema 渲染可覆盖参数（全部可选，留空即
// 用策略里的值）、为每条素材生成幂等键。素材范围由调用方决定，这里不改选。
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
const strategyId = ref(null)
const overrides = ref({})
// request_key 在对话框打开那一刻逐素材生成，并在对话框存续期间保持不变：
// 保存的双击与网络重试复用同一批 key，服务端按 uq_compose_tasks_request_key
// 幂等返回既有任务，不会造出第二批。关了再开才算一次新请求。
const requestKeys = ref({})

const strategyOptions = computed(() => strategies.value
  .filter((item) => item.status === 'active')
  .map((item) => ({ value: item.id, label: `${item.name} · ${item.game_id}` })))

const selectedStrategy = computed(() => strategies.value.find((item) => item.id === strategyId.value) || null)

const fields = computed(() => {
  const strategy = selectedStrategy.value
  if (!strategy) return []
  const template = templates.value.find((item) => item.id === strategy.compose_template_id && item.version === strategy.compose_template_version)
  return schemaFields(template?.schema)
})

function generateKeys(materials) {
  const keys = {}
  for (const material of materials) keys[material.id] = crypto.randomUUID()
  return keys
}

watch(() => props.visible, (visible) => {
  if (!visible) return
  error.value = ''
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
  if (!strategyId.value) {
    error.value = '请选择合成策略'
    return
  }
  saving.value = true
  error.value = ''
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
    error.value = e?.message || '创建合成任务失败'
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
    width="640px"
    :confirm-btn="{ content: '创建合成任务', theme: 'primary', loading: saving }"
    :cancel-btn="{ content: '取消', class: 'wt-secondary-button', variant: 'outline' }"
    :close-on-overlay-click="false"
    @confirm="submit"
    @close="close"
  >
    <t-loading :loading="loading" :show-overlay="true">
      <div class="compose-create">
        <t-alert v-if="error" theme="error" :message="error" style="margin-bottom: 12px" />
        <p v-if="materials.length === 0" class="compose-create__empty">没有选择素材，请回到列表勾选后重试。</p>
        <template v-else>
          <section class="compose-create__section">
            <h4 class="compose-create__title">① 合成策略</h4>
            <t-select v-model="strategyId" :options="strategyOptions" clearable placeholder="选择要使用的合成策略" />
            <p class="compose-create__hint">只有启用中的策略可选；参数留空即使用策略里保存的值。</p>
          </section>
          <section v-if="fields.length > 0" class="compose-create__section">
            <h4 class="compose-create__title">② 覆盖参数（可选）</h4>
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
          <section class="compose-create__section">
            <h4 class="compose-create__title">③ 所选素材（{{ materials.length }}）</h4>
            <ul class="compose-create__materials">
              <li v-for="material in materials" :key="material.id">
                <span class="compose-create__material-id">{{ material.id }}</span>
                <span class="compose-create__material-title">{{ material.title || '未命名素材' }}</span>
              </li>
            </ul>
          </section>
        </template>
      </div>
    </t-loading>
  </t-dialog>
</template>

<style scoped>
.compose-create__empty { color: var(--wt-text-secondary); }
.compose-create__section { margin-bottom: 16px; }
.compose-create__title { margin: 0 0 8px; font-size: 13px; font-weight: 600; color: var(--wt-text-primary); }
.compose-create__hint { margin: 8px 0 0; font-size: 12px; color: var(--wt-text-tertiary); }
.compose-create__params { display: flex; flex-wrap: wrap; gap: 12px; }
.compose-create__field { display: flex; align-items: center; gap: 8px; color: var(--wt-text-secondary); font-size: 13px; }
.compose-create__materials { margin: 0; padding: 0; list-style: none; max-height: 180px; overflow: auto; }
.compose-create__materials li { display: flex; gap: 10px; padding: 4px 0; font-size: 13px; }
.compose-create__material-id { color: var(--wt-text-tertiary); min-width: 48px; }
.compose-create__material-title { color: var(--wt-text-primary); overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
</style>
