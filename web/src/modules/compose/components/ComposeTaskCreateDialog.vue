<script setup>
import { computed, ref, watch } from 'vue'
import { DialogPlugin } from 'tdesign-vue-next'
import BusinessSelect from '../../../shared/ui/fields/BusinessSelect.vue'
import ResourceOverlay from '../../../shared/ui/resource/ResourceOverlay.vue'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import { createComposeClient } from '../../../shared/api/compose.js'
import { createSessionClient } from '../../../shared/api/session.js'
import { enabledCategories } from '../../../shared/utils/categories.js'
import { settingFields, strategySettings } from '../strategySettings.js'
import { groupOverridePayload, validateTaskGroup } from '../taskGroupOverrides.js'
import ComposeTaskSettings from './ComposeTaskSettings.vue'
import ComposeTaskMaterialSelector from './ComposeTaskMaterialSelector.vue'
const props = defineProps({ visible: Boolean })
const emit = defineEmits(['update:visible', 'created'])
const client = createComposeClient(), session = createSessionClient()
const currentUser = ref(null), categories = ref([]), teams = ref([]), strategies = ref([]), templates = ref([]), operators = ref([])
const taskName = ref(''), teamID = ref(null), categoryID = ref(null), operatorID = ref(null), strategyID = ref(null)
const loading = ref(false), operatorsLoading = ref(false), saving = ref(false), error = ref(''), nameError = ref(''), strategyError = ref(''), selected = ref([]), overrides = ref({}), remakeConfirmed = ref(false), materialVisible = ref(false), failures = ref([])
const submitted = ref({signature:'',key:''})
let operatorSequence = 0
const isAdmin = computed(() => currentUser.value?.role === 'admin')
const canAssign = computed(() => ['admin','senior_operator'].includes(currentUser.value?.role))
const categoryOptions = computed(() => enabledCategories(categories.value))
const selectedStrategy = computed(() => strategies.value.find((row) => row.id === strategyID.value))
const selectedTemplate = computed(() => templates.value.find((row) => row.id === selectedStrategy.value?.compose_template_id && row.version === selectedStrategy.value?.compose_template_version))
const strategyOptions = computed(() => strategies.value.filter((row) => row.status === 'enabled' && row.team_id === teamID.value && row.category_id === categoryID.value && row.operator_id === operatorID.value).map((row) => ({label:row.name,value:row.id})))
const context = computed(() => ({ strategy_id:strategyID.value, operator_id:operatorID.value, team_id:teamID.value, category_id:categoryID.value, operator_name:canAssign.value ? operators.value.find((row) => row.id === operatorID.value)?.display_name : currentUser.value?.display_name || currentUser.value?.name, team_name:teams.value.find((row) => row.id === teamID.value)?.name, category_name:categories.value.find((row) => row.id === categoryID.value)?.name }))
const groups = computed(() => selectedStrategy.value ? strategySettings(selectedStrategy.value) : [])
const estimated = computed(() => selected.value.length * groups.value.reduce((sum,row) => sum + row.copies,0))
const hasPriorWork = computed(() => selected.value.some((row) => Number(row.usage?.output_count || 0) > 0 || Object.entries(row.strategy_history || {}).some(([key,value]) => key.endsWith('_count') && Number(value)>0)))
watch(() => props.visible, async (visible) => {
  if (!visible) return
  taskName.value=''; teamID.value=null; categoryID.value=null; operatorID.value=null; strategyID.value=null; selected.value=[]; overrides.value={}; error.value=''; nameError.value=''; strategyError.value=''; failures.value=[]; remakeConfirmed.value=false; submitted.value={signature:'',key:''}
  loading.value=true
  try {
    const [me, strategyRows, templateRows, categoryRows, teamRows] = await Promise.all([session.me(),client.listStrategies(),client.listTemplates(),client.listCategories(),client.listTeams()])
    currentUser.value=me; strategies.value=Array.isArray(strategyRows)?strategyRows:strategyRows.items || []; templates.value=templateRows || []; categories.value=categoryRows || []; teams.value=teamRows || []
    if (me.role !== 'admin') teamID.value=me.team_id
    if (!['admin','senior_operator'].includes(me.role)) operatorID.value=me.id
  } catch(e) { error.value=e?.message || '新建任务配置读取失败' }
  finally { loading.value=false }
})
async function loadOperators() {
  const sequence=++operatorSequence
  operators.value=[]
  if (!canAssign.value || !teamID.value || !categoryID.value) return
  operatorsLoading.value=true
  try { const rows=await client.listComposeOperators({team_id:teamID.value,category_id:categoryID.value}); if(sequence===operatorSequence) operators.value=rows || [] }
  catch(e) { if(sequence===operatorSequence) error.value=e?.message || '负责人读取失败' }
  finally { if(sequence===operatorSequence) operatorsLoading.value=false }
}
function changeContext(field,value) {
  const apply = () => {
    if(field==='team') { teamID.value=value; categoryID.value=null; operatorID.value=canAssign.value?null:currentUser.value.id; strategyID.value=null }
    if(field==='category') { categoryID.value=value; operatorID.value=canAssign.value?null:currentUser.value.id; strategyID.value=null }
    if(field==='operator') { operatorID.value=value; strategyID.value=null }
    if(field==='strategy') strategyID.value=value
    selected.value=[]; overrides.value={}; remakeConfirmed.value=false; strategyError.value=''; failures.value=[]
    if(['team','category'].includes(field)) loadOperators()
  }
  const previous={team:teamID.value,category:categoryID.value,operator:operatorID.value,strategy:strategyID.value}[field]
  if(previous===value) return
  if(!selected.value.length && !Object.keys(overrides.value).length) { apply(); return }
  const dialog=DialogPlugin.confirm({header:'更换任务配置？',body:'更换后需要重新选择素材并设置本次参数。已加入的负责人素材关系会保留。',confirmBtn:{content:'更换',theme:'primary'},onConfirm:()=>{apply();dialog.hide()}})
}
function acceptSelection(rows) {
  selected.value=Object.values(Object.fromEntries([...selected.value,...rows].map((row)=>[row.id,row])))
  remakeConfirmed.value=false
}
function removeSelected(id) { selected.value=selected.value.filter((row)=>row.id!==id);remakeConfirmed.value=false }
function close() {
  if(saving.value) return
  if(!taskName.value && !selected.value.length && !Object.keys(overrides.value).length) { emit('update:visible',false);return }
  const dialog=DialogPlugin.confirm({header:'放弃未保存的任务？',body:'任务尚未提交，关闭后将丢失当前草稿。已加入的负责人素材关系会保留。',confirmBtn:{content:'放弃',theme:'danger'},onConfirm:()=>{emit('update:visible',false);dialog.hide()}})
}
async function submit() {
  if(saving.value) return
  nameError.value='';strategyError.value='';error.value=''
  const name=taskName.value.trim()
  if(!name) {nameError.value='请输入任务名称';return}
  if(!selectedStrategy.value) {strategyError.value='请选择合成策略';return}
  if(!selected.value.length) {error.value='请选择至少一条素材';return}
  if(hasPriorWork.value && !remakeConfirmed.value) {error.value='请确认本次重新制作';return}
  const fields=settingFields(selectedTemplate.value?.schema)
  for(const group of groups.value) {
    const issues=validateTaskGroup(group,{...group.params,...overrides.value[group.key]},fields)
    if(Object.keys(issues).length) {error.value=`${group.name}：${Object.values(issues)[0]}`;return}
  }
  const body={name,strategy_id:strategyID.value,operator_id:operatorID.value,material_ids:selected.value.map((row)=>row.id),setting_overrides:groupOverridePayload(groups.value,overrides.value,fields),remake_confirmed:remakeConfirmed.value}
  const signature=JSON.stringify(body)
  if(submitted.value.signature!==signature) submitted.value={signature,key:crypto.randomUUID()}
  body.request_key=submitted.value.key
  saving.value=true
  try {
    const result=await client.createComposeTasks(body)
    failures.value=result.failed || []
    emit('created',result)
    if(failures.value.length) error.value=`已创建 ${result.created?.length || 0} 个任务，${failures.value.length} 项未受理`
    else emit('update:visible',false)
  } catch(e) {error.value=e?.message || '创建任务失败'}
  finally {saving.value=false}
}
</script>
<template>
  <ResourceOverlay kind="drawer" :visible="visible" title="新建合成任务" :width="880" :busy="saving" @close="close">
    <t-loading :loading="loading" :show-overlay="true">
      <div class="task-create">
        <FeedbackAlert v-if="error" theme="error" :message="error" />
        <t-form label-width="100px">
          <t-form-item label="任务名称" required-mark><div class="task-create__field"><t-input v-model="taskName" :maxlength="120" placeholder="输入本次任务批次名称" :status="nameError?'error':undefined" /><p v-if="nameError" class="task-create__error">{{ nameError }}</p></div></t-form-item>
          <t-form-item v-if="isAdmin" label="运营团队" required-mark><BusinessSelect kind="team" :value="teamID" clearable @change="changeContext('team',$event)"><t-option v-for="row in teams" :key="row.id" :value="row.id" :label="row.name" /></BusinessSelect></t-form-item>
          <t-form-item v-else label="运营团队"><span>{{ context.team_name || (teamID ? `团队 ${teamID}` : '—') }}</span></t-form-item>
          <t-form-item label="业务分类" required-mark><BusinessSelect kind="category" :value="categoryID" :disabled="!teamID" clearable @change="changeContext('category',$event)"><t-option v-for="row in categoryOptions" :key="row.id" :value="row.id" :label="row.name" /></BusinessSelect></t-form-item>
          <t-form-item label="负责人" required-mark><BusinessSelect v-if="canAssign" kind="member" :value="operatorID" :disabled="!categoryID || operatorsLoading" :loading="operatorsLoading" clearable @change="changeContext('operator',$event)"><t-option v-for="row in operators" :key="row.id" :value="row.id" :label="row.display_name" /></BusinessSelect><span v-else>{{ context.operator_name || '本人' }}</span></t-form-item>
          <t-form-item label="合成策略" required-mark><div class="task-create__field"><t-select :value="strategyID" :options="strategyOptions" :disabled="!operatorID || !categoryID" clearable placeholder="选择负责人可用的策略" :status="strategyError?'error':undefined" @change="changeContext('strategy',$event)" /><p v-if="strategyError" class="task-create__error">{{ strategyError }}</p></div></t-form-item>
        </t-form>
        <section><div class="task-create__heading"><h4>选用素材</h4><t-button variant="outline" class="wt-secondary-button" :disabled="!selectedStrategy" @click="materialVisible=true">选择素材</t-button></div><p v-if="!selected.length" class="task-create__hint">选择负责人工作集或同团队、同类目的素材。</p><div v-else class="task-create__selected"><div v-for="row in selected" :key="row.id"><span>{{ row.title || `素材 ${row.id}` }}</span><t-button size="small" variant="text" @click="removeSelected(row.id)">移除</t-button></div></div></section>
        <section v-if="selectedStrategy"><h4>成片设置</h4><ComposeTaskSettings v-model="overrides" :strategy="selectedStrategy" :template="selectedTemplate" :material-count="selected.length" /></section>
        <div v-if="selected.length" class="task-create__summary"><strong>已选 {{ selected.length }} 条素材，预计创建 {{ estimated }} 个任务、生成 {{ estimated }} 个成片</strong><t-checkbox v-if="hasPriorWork" v-model="remakeConfirmed">所选素材已有产出或任务，确认本次新建任务，保留旧任务和成片。</t-checkbox></div>
        <div v-if="failures.length" class="task-create__failures"><p v-for="(item,index) in failures" :key="item.material_id || index">素材 {{ item.material_id }} · {{ item.message || item.reason || '未受理' }}</p></div>
      </div>
    </t-loading>
    <template #footer><t-button variant="outline" class="wt-secondary-button" :disabled="saving" @click="close">取消</t-button><t-button theme="primary" :loading="saving" :disabled="!selected.length || !selectedStrategy || (hasPriorWork && !remakeConfirmed)" @click="submit">创建任务</t-button></template>
  </ResourceOverlay>
  <ComposeTaskMaterialSelector v-model:visible="materialVisible" :context="context" :selected="selected" @selected="acceptSelection" />
</template>
<style scoped>
.task-create { display:grid; gap:20px; }
.task-create section { border:1px solid var(--wt-border); border-radius:var(--wt-radius-md); padding:16px; }
.task-create h4 { margin:0 0 14px; color:var(--wt-text-primary); font-size:14px; }
.task-create__heading { display:flex; align-items:center; justify-content:space-between; gap:12px; margin-bottom:12px; }
.task-create__heading h4 { margin:0; }
.task-create__field { display:grid; gap:5px; width:100%; }
.task-create__error { margin:0; color:var(--td-error-color); font-size:12px; }
.task-create__hint { color:var(--wt-text-tertiary); font-size:13px; }
.task-create__selected { display:grid; max-height:220px; overflow:auto; }
.task-create__selected > div { display:flex; align-items:center; justify-content:space-between; gap:12px; border-bottom:1px solid var(--wt-border); padding:6px 0; }
.task-create__selected span { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.task-create__summary { display:grid; gap:12px; padding:14px; background:var(--wt-info-bg); border-radius:var(--wt-radius-md); font-size:13px; }
.task-create__failures { max-height:180px; overflow:auto; color:var(--wt-text-secondary); font-size:13px; }
</style>
