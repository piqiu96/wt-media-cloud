<script setup>
import { computed, ref, watch } from 'vue'
import { createComposeClient } from '../../../shared/api/compose.js'
import { createMaterialsClient } from '../../../shared/api/materials.js'
import { schemaFields } from '../schemaFields.js'
import { CANDIDATE_SORTS, addAction, cappedRows, candidateRows, materialStatesById, usageCounts } from '../materialStates.js'
import { buildOverridePayload, prefillOverrides, validateOverrides } from '../taskOverrides.js'
import { feedback } from '../../../shared/ui/feedback/index.js'
import { enabledCategories } from '../../../shared/utils/categories.js'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import FormError from '../../../shared/ui/feedback/FormError.vue'
import MaterialCover from '../../materials/components/MaterialCover.vue'

// 创建合成任务的唯一入口在合成任务页（我的素材里的两处入口已撤），素材因此只能在这里选。
// 候选就是「我的素材」里**使用中**的关系：已放弃的关系不允许新建任务（PRD §5.4.5），过滤
// 落在候选集上，不选完再悄悄丢掉。下载状态不参与过滤 —— 下载原素材是任务自己工作的一部分
// （云端会先准备文件），不是运营动手的前提。每条素材一个独立幂等键。
//
// 「这条素材在这条策略下用过没有」是**按策略**算的（materialStates 的口径），所以合成策略
// 排在第 1 段：没选策略之前这块不给候选，也不去问服务端。一条 (策略, 素材) 只跑一次，所以
// 有过任务记录的行整行只读、连按钮都不给，重跑在合成任务页对那条任务点「重试」。
//
// 参数默认填策略里保存的值（taskOverrides 的口径）：留空不是「没设置」，空着反而让人以为
// 策略值没生效。提交前先本地校验一遍，报错全是中文，不再把执行器的英文原文丢给运营。
const props = defineProps({
  visible: { type: Boolean, default: false },
})
const emit = defineEmits(['update:visible', 'created'])

// 不选中的候选一次最多列这么多行：素材多的时候整墙铺开反而找不到东西，搜索才是找它的方式。
// 已选中的行不受这个上限影响（被截掉就删不掉了）。
const CANDIDATE_ROW_LIMIT = 50

const client = createComposeClient()
const materialsClient = createMaterialsClient()
const candidates = ref([])
const selectedIds = ref([])
const strategies = ref([])
const templates = ref([])
const categories = ref([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const candidatesError = ref('')
const strategyError = ref('')
const strategyId = ref(null)
const categoryFilter = ref(null)
const candidateUsage = ref('unused')
const candidateKeyword = ref('')
const candidateSort = ref('published_desc')
const states = ref([])
const statesReady = ref(false)
const statesError = ref('')
const taskName = ref('')
const overrides = ref({})
const overrideErrors = ref({})
// 抽屉存续期间复用同一批 request_key，关闭后重新打开才生成新的一批。
const requestKeys = ref({})
// 已按哪条策略预填过。策略与模板是并发加载的，字段清单要等模板到了才算得出来，
// 所以预填既可能在选策略时发生，也可能在模板后到时补发生 —— 这个值保证只发生一次。
const prefilledFor = ref(null)

const selected = computed(() => candidates.value.filter((item) => selectedIds.value.includes(item.id)))

// 只列候选里真的出现过的类目：列一个点下去没有任何素材的选项，与「筛了但服务端没筛」
// 是同一种误导（素材库的类目筛选同此口径）。停用类目同样不列 —— 它已经不能再建策略。
// 名字从类目表来，查不到就退回编号。候选本身不缩小，这里只决定筛选项。
const categoryOptions = computed(() => {
  const enabledIds = new Set(enabledCategories(categories.value).map((category) => category.id))
  const used = [...new Set(candidates.value.map((item) => item.category_id).filter(Boolean))]
  return used
    .filter((id) => enabledIds.has(id))
    .map((id) => ({ value: id, label: categories.value.find((category) => category.id === id)?.name || id }))
})

// 类目筛选只缩候选、不动已选：selected 直接取自全量候选，切类目不会把选好的素材弄丢。
const visibleCandidates = computed(() => (categoryFilter.value
  ? candidates.value.filter((item) => item.category_id === categoryFilter.value)
  : candidates.value))

const statesById = computed(() => materialStatesById(states.value))
// 两组各自的条数：分母是类目筛选之后的这批候选，不跟着搜索框抖 —— 会抖的数字不算信息。
const counts = computed(() => usageCounts(visibleCandidates.value, statesById.value))

// 使用情况是**一个字段**（两个取值），不是两个页签：它和「类目」并排，一起回答「给我看哪些」。
const usageOptions = computed(() => [
  { value: 'unused', label: statesReady.value ? `未使用过（${counts.value.unused}）` : '未使用过' },
  { value: 'used', label: statesReady.value ? `已使用过（${counts.value.used}）` : '已使用过' },
])

const rows = computed(() => candidateRows(visibleCandidates.value, statesById.value, candidateUsage.value, candidateKeyword.value, candidateSort.value))
const visibleRows = computed(() => cappedRows(rows.value, selectedIds.value, CANDIDATE_ROW_LIMIT))

// 状态还没读回来时不给按钮：此刻「没用过」和「不知道」长得一样，让人点下去就是在猜。
// 有过任务记录的行返回 null —— 不是画一个禁用的按钮，而是这一行根本没有可做的动作。
function rowAction(row) {
  if (!statesReady.value) return { label: '状态读取中', disabled: true }
  return addAction(row.state, selectedIds.value.includes(row.id))
}

// 与素材详情抽屉同一个量法（MaterialDetailDrawer:128）的互动数据：千分位，空值显示 0 而不是空白。
function countLabel(value) { return Number(value || 0).toLocaleString() }

const strategyOptions = computed(() => strategies.value
  .filter((item) => item.status === 'enabled')
  .map((item) => ({ value: item.id, label: `${item.name} · ${item.category_id}` })))

const selectedStrategy = computed(() => strategies.value.find((item) => item.id === strategyId.value) || null)
const strategyName = computed(() => selectedStrategy.value?.name || '尚未选择')
const strategyOperatorName = computed(() => selectedStrategy.value?.operator_name || '未记录')
const strategyConfig = computed(() => selectedStrategy.value?.config || {})

const fields = computed(() => {
  const strategy = selectedStrategy.value
  if (!strategy) return []
  const template = templates.value.find((item) => item.id === strategy.compose_template_id && item.version === strategy.compose_template_version)
  return schemaFields(template?.schema).filter((field) => field.key !== 'guide_video_url')
})

// 幂等键在素材**进入选择时**生成、并在抽屉存续期间保持（移出再选回也不换）：保存按钮的
// 双击/重试命中的是服务端唯一键，而不是造出两批任务。
function ensureKeys(ids) {
  const next = { ...requestKeys.value }
  for (const id of ids) if (!next[id]) next[id] = crypto.randomUUID()
  requestKeys.value = next
}

function addMaterial(id) {
  if (!selectedIds.value.includes(id)) selectedIds.value = [...selectedIds.value, id]
}

function removeMaterial(id) {
  selectedIds.value = selectedIds.value.filter((item) => item !== id)
}

watch(selectedIds, (ids) => ensureKeys(ids))
// 使用情况是按策略算的，所以换策略要把状态整份重读；换策略保留已选（仓规：切换不做静默
// 删除），而同一对在别的策略下用过、在这条策略下没用过，那也是这条策略的合法首次。
// 提交时按条拒绝的判据在服务端，与这里的分组是同一个。
watch(strategyId, () => {
  states.value = []
  statesReady.value = false
  statesError.value = ''
  loadStates()
})
// 换策略就整体重填：上一条策略的覆盖值对它没有意义，留着只会变成一次没人想要的覆盖。
watch([strategyId, fields], () => {
  const strategy = selectedStrategy.value
  if (!strategy || fields.value.length === 0) return
  if (prefilledFor.value === strategy.id) return
  prefilledFor.value = strategy.id
  overrides.value = prefillOverrides(fields.value, strategy.config)
  overrideErrors.value = {}
})
watch(() => props.visible, (visible) => {
  if (!visible) return
  error.value = ''
  strategyError.value = ''
  strategyId.value = null
  categoryFilter.value = null
  candidateUsage.value = 'unused'
  candidateKeyword.value = ''
  candidateSort.value = 'published_desc'
  states.value = []
  statesReady.value = false
  statesError.value = ''
  taskName.value = ''
  overrides.value = {}
  overrideErrors.value = {}
  prefilledFor.value = null
  selectedIds.value = []
  requestKeys.value = {}
  load()
})

async function load() {
  loading.value = true
  await Promise.all([loadStrategies(), loadCandidates()])
  loading.value = false
}

async function loadStrategies() {
  try {
    const [strategyRows, templateRows, categoryRows] = await Promise.all([
      client.listStrategies(), client.listTemplates(), client.listCategories().catch(() => []),
    ])
    strategies.value = Array.isArray(strategyRows) ? strategyRows : []
    templates.value = Array.isArray(templateRows) ? templateRows : []
    categories.value = Array.isArray(categoryRows) ? categoryRows : []
  } catch (e) {
    error.value = e?.message || '读取合成策略失败'
  }
}

async function loadCandidates() {
  candidatesError.value = ''
  try {
    const rows = await materialsClient.listMyMaterials()
    candidates.value = (Array.isArray(rows) ? rows : [])
      .filter((usage) => usage.status === 'active' && usage.material)
      // 发布时间与互动数据随行带过来：排序按它们算，运营也在这块挑素材 —— 不必为了看
      // 点赞数再点开素材详情（接口早就带了这几个字段，是这里没接）。
      .map((usage) => ({
        id: usage.material.id,
        title: usage.material.title,
        cover_url: usage.material.cover_url,
        category_id: usage.material.category_id,
        published_at: usage.material.published_at,
        like_count: usage.material.like_count,
        favorite_count: usage.material.favorite_count,
      }))
    // 候选后到：策略可能已经选好了，这时把它这批候选的状态补读一次。
    await loadStates()
  } catch (e) {
    candidatesError.value = e?.message || '读取我的素材失败'
  }
}

// 读失败时整块降级：宁可让人看到「读不到」，也不静默把一切当成未使用过 —— 那正是会
// 诱导重复创建的那个错误答案。
async function loadStates() {
  if (!strategyId.value || candidates.value.length === 0) return
  statesError.value = ''
  statesReady.value = false
  try {
    const rows = await client.listStrategyMaterialStates(strategyId.value, candidates.value.map((item) => item.id))
    states.value = Array.isArray(rows) ? rows : []
    statesReady.value = true
  } catch (e) {
    statesError.value = e?.message || '读取素材使用情况失败'
  }
}

const overridePayload = computed(() => buildOverridePayload(fields.value, strategyConfig.value, overrides.value))

// 提交确认段只复述这一次提交真正会发生什么：几条任务、哪条策略、哪些参数与策略不同。
const overrideSummary = computed(() => fields.value
  .filter((field) => Object.hasOwn(overridePayload.value, field.key))
  .map((field) => `${field.label} ${overridePayload.value[field.key]}`))

// 任务名称留空则按「素材标题摘要 · 策略名称」自动生成；多条素材时服务端在名称后加序号。
const nameSummary = computed(() => {
  const label = taskName.value.trim()
  if (!label) return '未填写，按素材与策略自动生成'
  if (selected.value.length > 1) return `${label}-1 … ${label}-${selected.value.length}（共 ${selected.value.length} 条）`
  return label
})

function close() {
  emit('update:visible', false)
}

async function submit() {
  if (saving.value || selected.value.length === 0) return
  if (!strategyId.value) {
    strategyError.value = '请选择合成策略'
    return
  }
  // 任务负责人必须来自策略；负责人失效/未指派的策略服务端也会拒，先在这里说清原因。
  if (!selectedStrategy.value?.operator_id) {
    strategyError.value = '该策略尚未指定负责人，请先在合成策略页补选后再创建'
    return
  }
  // 本地拦住的就是服务端会拒的那几种情况（值填反、低于下限、必填为空），
  // 拦下来就不发请求，运营看到的是中文，而不是执行器的英文原文。
  overrideErrors.value = validateOverrides(fields.value, strategyConfig.value, overrides.value)
  if (Object.keys(overrideErrors.value).length > 0) {
    feedback.toast.error('可覆盖参数有误，请按提示修改后再提交')
    return
  }
  saving.value = true
  strategyError.value = ''
  try {
    const body = {
      strategy_id: strategyId.value,
      operator_id: selectedStrategy.value.operator_id,
      items: selected.value.map((material) => ({ material_id: material.id, request_key: requestKeys.value[material.id] })),
    }
    if (taskName.value.trim()) body.name = taskName.value.trim()
    if (Object.keys(overridePayload.value).length > 0) body.config_override = overridePayload.value
    const result = await client.createComposeTasks(body)
    emit('created', result || { created: [], failed: [] })
    emit('update:visible', false)
  } catch (e) {
    // 参数类错误在服务端只有「不通过」这一种语义，英文原文（含参数名与取值）由服务端记进
    // 诊断，页面给出可行动的中文提示；其他错误照旧透传。
    if (e?.type === 'compose_task_invalid') {
      feedback.toast.error('任务参数未通过校验，请检查「可覆盖参数」后重试')
    } else {
      feedback.toast.error(e?.message || '创建合成任务失败')
    }
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <t-drawer
    :visible="visible"
    header="创建合成任务"
    size="min(62vw, 880px)"
    :close-btn="true"
    :close-on-overlay-click="false"
    @update:visible="emit('update:visible', $event)"
    @close="close"
  >
    <t-loading :loading="loading" :show-overlay="true">
      <div class="compose-create">
        <FeedbackAlert v-if="error" theme="error" :message="error" class="compose-create__load-error" />
        <FeedbackAlert v-if="candidatesError" theme="error" :message="candidatesError" class="compose-create__load-error">
          <template #operation><t-button class="wt-secondary-button" variant="outline" size="small" @click="loadCandidates">重试</t-button></template>
        </FeedbackAlert>
        <section class="compose-create__section">
          <h4 class="compose-create__title"><span>1</span> 合成策略</h4>
          <t-select v-model="strategyId" :options="strategyOptions" clearable placeholder="选择要使用的合成策略" :status="strategyError ? 'error' : undefined" aria-describedby="compose-create-strategy-error" @change="strategyError = ''" />
          <FormError id="compose-create-strategy-error" :message="strategyError" />
          <p class="compose-create__hint">只有启用中的策略可选；参数留空即使用策略里保存的值。素材是否已经用过，是按策略算的，所以先选它。</p>
        </section>
        <section class="compose-create__section">
          <div class="compose-create__heading"><h4 class="compose-create__title"><span>2</span> 已选素材</h4><strong>{{ selected.length }} 条</strong></div>
          <p v-if="!strategyId" class="compose-create__hint">先在上一步选择合成策略：素材是否已经用过，是按策略算的。</p>
          <template v-else>
            <div class="compose-create__picker-row">
              <t-select
                v-model="categoryFilter"
                class="compose-create__category"
                :options="categoryOptions"
                clearable
                placeholder="全部类目"
              />
              <t-select
                v-model="candidateUsage"
                class="compose-create__usage"
                :options="usageOptions"
                :disabled="!statesReady"
                aria-label="使用情况"
              />
              <t-select
                v-model="candidateSort"
                class="compose-create__sort"
                :options="CANDIDATE_SORTS"
                aria-label="排序"
              />
            </div>
            <t-input
              v-model="candidateKeyword"
              class="compose-create__search"
              clearable
              placeholder="搜索素材标题或素材 ID"
            />
            <FeedbackAlert v-if="statesError" theme="error" :message="statesError" class="compose-create__load-error">
              <template #operation><t-button class="wt-secondary-button" variant="outline" size="small" @click="loadStates">重试</t-button></template>
            </FeedbackAlert>
            <ul v-else class="compose-create__candidates">
              <li v-for="row in visibleRows.rows" :key="row.id">
                <MaterialCover :url="row.cover_url" />
                <span class="compose-create__material-text">
                  <strong>{{ row.title || '未命名素材' }}</strong>
                  <small>素材 {{ row.id }} · 点赞 {{ countLabel(row.like_count) }} · 收藏 {{ countLabel(row.favorite_count) }}<template v-if="statesReady && row.state.success_count > 0"> · 已生成 {{ row.state.success_count }} 版</template></small>
                </span>
                <span v-if="row.tag" class="compose-create__tag" :class="`compose-create__tag--${row.tag.tone}`">{{ row.tag.label }}</span>
                <!-- 有过任务记录的行没有可做的动作，整行只读：这里不画按钮，也不留一个禁用按钮
                     让人以为是权限或状态问题。 -->
                <t-button
                  v-if="rowAction(row)"
                  class="wt-secondary-button compose-create__add"
                  variant="outline"
                  size="small"
                  :disabled="rowAction(row).disabled"
                  @click="addMaterial(row.id)"
                >{{ rowAction(row).label }}</t-button>
              </li>
            </ul>
            <p v-if="!statesError && visibleRows.rows.length === 0" class="compose-create__hint">
              {{ candidateUsage === 'used' ? '这条策略下还没有已使用过的候选素材。' : '当前筛选下没有候选素材，换个条件或清空搜索试试。' }}
            </p>
            <p v-if="!statesError && visibleRows.omitted > 0" class="compose-create__hint">还有 {{ visibleRows.omitted }} 条未列出，用上面的搜索缩小范围。</p>
          </template>
          <p v-if="candidates.length === 0" class="compose-create__hint">「我的素材」里还没有使用中的素材：先在素材库把它加入我的素材，再回来创建任务。</p>
          <template v-else-if="strategyId">
            <p class="compose-create__hint">只列出「我的素材」里使用中的素材；按类目筛选只缩小候选，不会取消已选中的素材。每条素材独立生成一条合成任务，任务名称按「素材标题摘要 · 策略名称」各自生成。</p>
            <p class="compose-create__hint">这条策略下已有任务记录的素材是只读的：一条素材在一条策略下只跑一次，要重跑请到合成任务页对那条任务点「重试」。</p>
          </template>
          <div v-if="selected.length > 0" class="compose-create__selected">
            <h5 class="compose-create__subtitle">已选素材</h5>
            <ul class="compose-create__materials">
              <li v-for="material in selected" :key="material.id">
                <MaterialCover :url="material.cover_url" />
                <span class="compose-create__material-text"><strong>{{ material.title || '未命名素材' }}</strong><small>素材 ID {{ material.id }}</small></span>
                <t-button class="wt-secondary-button compose-create__material-remove" variant="outline" size="small" @click="removeMaterial(material.id)">移除</t-button>
              </li>
            </ul>
          </div>
          <label class="compose-create__field compose-create__name">
            <span>任务名称</span>
            <t-input v-model="taskName" :maxlength="60" clearable placeholder="可不填，默认按素材与策略命名" style="width: 320px" />
          </label>
        </section>
        <section class="compose-create__section">
          <h4 class="compose-create__title"><span>3</span> 可覆盖参数</h4>
          <div v-if="fields.length > 0" class="compose-create__params">
            <div v-for="field in fields" :key="field.key" class="compose-create__param">
              <label class="compose-create__field">
                <span>{{ field.label }}</span>
                <t-input-number
                  v-if="field.type === 'integer' || field.type === 'number'"
                  v-model="overrides[field.key]"
                  :min="field.minimum"
                  theme="column"
                  :status="overrideErrors[field.key] ? 'error' : undefined"
                  :placeholder="field.required ? '必填（策略值）' : '策略值'"
                  style="width: 200px"
                  @change="overrideErrors = {}"
                />
                <t-input
                  v-else
                  v-model="overrides[field.key]"
                  clearable
                  :status="overrideErrors[field.key] ? 'error' : undefined"
                  :placeholder="field.required ? '必填（策略值）' : '策略值'"
                  style="width: 200px"
                  @change="overrideErrors = {}"
                />
              </label>
              <FormError :message="overrideErrors[field.key]" />
            </div>
          </div>
          <p v-else class="compose-create__hint">当前策略没有可覆盖的参数，将完全按策略里保存的值执行。</p>
          <p v-if="fields.length > 0" class="compose-create__hint">下面的值已按策略里保存的参数预填；清空某一项即用回策略值。</p>
        </section>
        <section class="compose-create__section compose-create__confirm">
          <h4 class="compose-create__title"><span>4</span> 提交确认</h4>
          <div class="compose-create__confirm-grid">
            <div><span class="compose-create__confirm-label">将创建任务</span><span>{{ selected.length }} 条</span></div>
            <div><span class="compose-create__confirm-label">合成策略</span><span>{{ strategyName }}</span></div>
            <div><span class="compose-create__confirm-label">负责人</span><span>{{ strategyOperatorName }}</span></div>
            <div><span class="compose-create__confirm-label">任务名称</span><span>{{ nameSummary }}</span></div>
            <div><span class="compose-create__confirm-label">覆盖参数</span><span>{{ overrideSummary.length ? overrideSummary.join('，') : '未覆盖，使用策略里保存的值' }}</span></div>
          </div>
          <p class="compose-create__hint">提交后任务进入队列，可在合成任务页查看执行状态。</p>
        </section>
      </div>
    </t-loading>

    <template #footer>
      <div class="compose-create__footer">
        <t-button class="wt-secondary-button" variant="outline" :disabled="saving" @click="close">取消</t-button>
        <t-button theme="primary" :disabled="selected.length === 0" :loading="saving" @click="submit">创建任务</t-button>
      </div>
    </template>
  </t-drawer>
</template>

<style scoped>
.compose-create__load-error { margin-bottom: 12px; }
.compose-create__section { margin-bottom: 16px; padding: 16px; border: 1px solid var(--wt-border); border-radius: 10px; }
.compose-create__heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.compose-create__heading strong { color: var(--wt-text-secondary); font-size: 12px; }
.compose-create__title { display: flex; align-items: center; gap: 8px; margin: 0 0 8px; font-size: 14px; font-weight: 600; color: var(--wt-text-primary); }
.compose-create__title span { display: inline-flex; align-items: center; justify-content: center; width: 20px; height: 20px; border-radius: 6px; background: var(--wt-primary); color: #fff; font-size: 12px; }
.compose-create__hint { margin: 8px 0 0; font-size: 12px; color: var(--wt-text-tertiary); }
.compose-create__picker-row { display: flex; gap: 8px; margin-top: 10px; }
.compose-create__picker { flex: 1; min-width: 0; }
.compose-create__category { flex: none; width: 160px; }
.compose-create__usage { flex: 1; min-width: 0; }
.compose-create__sort { flex: none; width: 150px; }
.compose-create__search { margin-top: 8px; }
.compose-create__candidates { margin: 10px 0 0; padding: 0; list-style: none; max-height: 224px; overflow: auto; }
.compose-create__candidates li { display: flex; align-items: center; gap: 10px; padding: 7px 0; border-top: 1px solid var(--wt-border); font-size: 13px; }
.compose-create__tag { flex: none; padding: 1px 8px; border-radius: 10px; font-size: 12px; }
.compose-create__tag--warning { background: var(--wt-warning-bg); color: var(--wt-warning); }
.compose-create__tag--danger { background: var(--wt-danger-bg); color: var(--wt-danger); }
.compose-create__tag--muted { background: var(--wt-bg-page); color: var(--wt-text-tertiary); }
.compose-create__add { flex: none; margin-left: auto; }
.compose-create__subtitle { margin: 14px 0 0; font-size: 13px; font-weight: 600; color: var(--wt-text-secondary); }
.compose-create__name { margin-top: 10px; }
.compose-create__param { display: flex; flex-direction: column; gap: 4px; }
.compose-create__params { display: flex; flex-wrap: wrap; gap: 12px; }
.compose-create__field { display: flex; align-items: center; gap: 8px; color: var(--wt-text-secondary); font-size: 13px; }
.compose-create__materials { margin: 8px 0 0; padding: 0; list-style: none; max-height: 176px; overflow: auto; }
.compose-create__materials li { display: flex; align-items: center; gap: 10px; padding: 7px 0; border-top: 1px solid var(--wt-border); font-size: 13px; }
.compose-create__material-text { display: flex; flex-direction: column; min-width: 0; gap: 3px; }
.compose-create__material-text strong { color: var(--wt-text-primary); font-weight: 500; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
.compose-create__material-text small { color: var(--wt-text-tertiary); font-size: 12px; }
.compose-create__material-remove { margin-left: auto; flex: none; }
.compose-create__confirm-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px 20px; font-size: 13px; }
.compose-create__confirm-grid > div { display: flex; flex-direction: column; gap: 4px; min-width: 0; }
.compose-create__confirm-grid > div > span:last-child { color: var(--wt-text-primary); overflow-wrap: anywhere; }
.compose-create__confirm-label { color: var(--wt-text-tertiary); font-size: 12px; }
.compose-create__footer { display: flex; justify-content: flex-end; align-items: center; gap: 8px; }
@media (max-width: 760px) { .compose-create__confirm-grid { grid-template-columns: 1fr; } }
</style>
