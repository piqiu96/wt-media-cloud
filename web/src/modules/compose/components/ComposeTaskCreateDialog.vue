<script setup>
import { computed, ref, watch } from 'vue'
import { DialogPlugin } from 'tdesign-vue-next'
import ResourceSection from '../../../shared/ui/resource/ResourceSection.vue'
import ResourceForm from '../../../shared/ui/resource/ResourceForm.vue'
import ResourceRowActions from '../../../shared/ui/resource/ResourceRowActions.vue'
import BusinessSelect from '../../../shared/ui/fields/BusinessSelect.vue'
import ResourceOverlay from '../../../shared/ui/resource/ResourceOverlay.vue'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import { createComposeClient } from '../../../shared/api/compose.js'
import { createMaterialsClient } from '../../../shared/api/materials.js'
import { createSessionClient } from '../../../shared/api/session.js'
import { enabledCategories } from '../../../shared/utils/categories.js'
import { settingFields, strategySettings } from '../strategySettings.js'
import { groupOverridePayload, validateTaskGroup } from '../taskGroupOverrides.js'
import { outputRemakeBody, outputRemakeSeed, submitOutputRemake, remakeTaskName } from '../outputRemake.js'
import ComposeStrategyPicker from './ComposeStrategyPicker.vue'
import ComposeTaskSettings from './ComposeTaskSettings.vue'
import ComposeTaskMaterialSelector from './ComposeTaskMaterialSelector.vue'
const props = defineProps({ visible: Boolean, sourceOutput: Object })
const emit = defineEmits(['update:visible', 'created'])
const client = createComposeClient(), session = createSessionClient(), materialsClient = createMaterialsClient()
const currentUser = ref(null), categories = ref([]), teams = ref([]), strategyDetail = ref(null), templates = ref([]), operators = ref([])
const taskName = ref(''), teamID = ref(null), categoryID = ref(null), operatorID = ref(null), strategyID = ref(null)
const loading = ref(false), operatorsLoading = ref(false), saving = ref(false), error = ref(''), nameError = ref(''), strategyError = ref(''), selected = ref([]), overrides = ref({}), remakeConfirmed = ref(false), materialVisible = ref(false), failures = ref([])
const submitted = ref({signature:'',key:''})
let operatorSequence = 0
const isAdmin = computed(() => currentUser.value?.role === 'admin')
const canAssign = computed(() => ['admin','senior_operator'].includes(currentUser.value?.role))
const categoryOptions = computed(() => enabledCategories(categories.value))
const isSource = computed(() => Boolean(props.sourceOutput?.output?.id))
const sourceSeed = computed(() => outputRemakeSeed(props.sourceOutput, templates.value))
const selectedStrategy = computed(() => isSource.value ? sourceSeed.value.strategy : strategyDetail.value)
const selectedTemplate = computed(() => isSource.value ? sourceSeed.value.template : templates.value.find((row) => row.id === selectedStrategy.value?.compose_template_id && row.version === selectedStrategy.value?.compose_template_version))

const context = computed(() => ({ strategy_id:strategyID.value, operator_id:operatorID.value, team_id:teamID.value, category_id:categoryID.value, operator_name:canAssign.value ? operators.value.find((row) => row.id === operatorID.value)?.display_name : currentUser.value?.nickname || currentUser.value?.name || currentUser.value?.username, team_name:teams.value.find((row) => row.id === teamID.value)?.name, category_name:categories.value.find((row) => row.id === categoryID.value)?.name }))
const groups = computed(() => selectedStrategy.value ? strategySettings(selectedStrategy.value) : [])
const estimated = computed(() => selected.value.length * groups.value.reduce((sum,row) => sum + row.copies,0))
const hasPriorWork = computed(() => isSource.value || selected.value.some((row) => Number(row.usage?.output_count || 0) > 0 || Object.entries(row.strategy_history || {}).some(([key,value]) => key.endsWith('_count') && Number(value)>0)))
watch(() => props.visible, async (visible) => {
  if (!visible) return
  strategyDetail.value=null; taskName.value=''; teamID.value=null; categoryID.value=null; operatorID.value=null; strategyID.value=null; selected.value=[]; overrides.value={}; error.value=''; nameError.value=''; strategyError.value=''; failures.value=[]; remakeConfirmed.value=false; submitted.value={signature:'',key:''}
  loading.value=true
  try {
    const [me, templateRows, categoryRows, teamRows] = await Promise.all([session.me(),isSource.value && props.sourceOutput?.trace?.strategy?.template_schema ? Promise.resolve([]) : client.listTemplates(),client.listCategories(),client.listTeams()])
    currentUser.value=me; templates.value=templateRows || []; categories.value=categoryRows || []; teams.value=teamRows || []
    if (isSource.value) {
      const output = props.sourceOutput.output
      taskName.value = remakeTaskName(output.name)
      teamID.value=output.team_id; categoryID.value=output.category_id; operatorID.value=output.operator_id; strategyID.value=output.compose_strategy_id
      if(sourceSeed.value.reason) error.value=sourceSeed.value.reason
      else selected.value=[sourceSeed.value.material]
      return
    }
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
    strategyDetail.value=null; selected.value=[]; overrides.value={}; remakeConfirmed.value=false; strategyError.value=''; failures.value=[]
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
  const dialog=DialogPlugin.confirm({header:'放弃未保存的任务？',body:isSource.value ? '任务尚未提交，关闭后将丢失当前草稿。' : '任务尚未提交，关闭后将丢失当前草稿。已加入的负责人素材关系会保留。',confirmBtn:{content:'放弃',theme:'danger'},onConfirm:()=>{emit('update:visible',false);dialog.hide()}})
}
async function submit() {
  if(saving.value) return
  nameError.value='';strategyError.value='';error.value=''
  const name=taskName.value.trim()
  if(!name) {nameError.value='请输入任务名称';return}
  if(isSource.value && sourceSeed.value.reason) {error.value=sourceSeed.value.reason;return}
  if(!selectedStrategy.value) {strategyError.value='请选择合成策略';return}
  if(!selected.value.length) {error.value='请选择至少一条素材';return}
  if(hasPriorWork.value && !remakeConfirmed.value) {error.value='请确认本次重新制作';return}
  const fields=settingFields(selectedTemplate.value?.schema)
  for(const group of groups.value) {
    const issues=validateTaskGroup(group,{...group.params,...overrides.value[group.key]},fields)
    if(Object.keys(issues).length) {error.value=`${group.name}：${Object.values(issues)[0]}`;return}
  }
  const body=isSource.value ? outputRemakeBody(props.sourceOutput,name,sourceSeed.value,overrides.value) : {name,strategy_id:strategyID.value,operator_id:operatorID.value,material_ids:selected.value.map((row)=>row.id),setting_overrides:groupOverridePayload(groups.value,overrides.value,fields),remake_confirmed:remakeConfirmed.value}
  const signature=JSON.stringify(body)
  if(submitted.value.signature!==signature) submitted.value={signature,key:crypto.randomUUID()}
  body.request_key=submitted.value.key
  saving.value=true
  try {
    const result=isSource.value ? await submitOutputRemake(props.sourceOutput,body,{materialsClient,composeClient:client,submission:submitted.value}) : await client.createComposeTasks(body)
    failures.value=result.failed || []
    emit('created',result)
    if(failures.value.length) error.value=`已创建 ${result.created?.length || 0} 个任务，${failures.value.length} 项未受理`
    else emit('update:visible',false)
  } catch(e) {error.value=e?.message || '创建任务失败'}
  finally {saving.value=false}
}
</script>
<template>
  <ResourceOverlay kind="drawer" :visible="visible" :title="isSource ? '再次制作' : '新建合成任务'" :width="880" :busy="saving" @close="close">
    <t-loading :loading="loading" :show-overlay="true">
      <div class="task-create wt-resource-page resource-visual-approved production-page">
        <FeedbackAlert v-if="error" theme="error" :message="error" />
        <ResourceSection title="基本信息" variant="form"><ResourceForm class="resource-form-grid">
          <t-form-item class="resource-form-full" label="任务名称" required-mark><div class="task-create__field"><t-input v-model="taskName" :maxlength="60" placeholder="输入本次任务批次名称" :status="nameError?'error':undefined" /><p v-if="nameError" class="task-create__error">{{ nameError }}</p></div></t-form-item>
          <template v-if="isSource">
            <t-form-item label="运营团队"><span>{{ context.team_name || '—' }}</span></t-form-item>
            <t-form-item label="所属类目"><span>{{ context.category_name || '—' }}</span></t-form-item>
            <t-form-item label="负责人"><span>{{ sourceOutput.output.operator_name || '—' }}</span></t-form-item>
            <t-form-item label="来源策略"><span>{{ sourceOutput.output.strategy_name || sourceOutput.trace?.strategy?.name || '—' }}</span></t-form-item>
            <t-form-item label="原模板版本"><span>{{ sourceOutput.trace?.compose_task?.template_code }} v{{ sourceOutput.trace?.compose_task?.template_version }}</span></t-form-item>
          </template>
          <template v-else>
          <t-form-item v-if="isAdmin" label="运营团队" required-mark><BusinessSelect kind="team" :value="teamID" clearable @change="changeContext('team',$event)"><t-option v-for="row in teams" :key="row.id" :value="row.id" :label="row.name" /></BusinessSelect></t-form-item>
          <t-form-item v-else label="运营团队"><span>{{ context.team_name || (teamID ? `团队 ${teamID}` : '—') }}</span></t-form-item>
          <t-form-item label="所属类目" required-mark><BusinessSelect kind="category" :value="categoryID" :disabled="!teamID" clearable @change="changeContext('category',$event)"><t-option v-for="row in categoryOptions" :key="row.id" :value="row.id" :label="row.name" /></BusinessSelect></t-form-item>
          <t-form-item label="负责人" required-mark><BusinessSelect v-if="canAssign" kind="member" :value="operatorID" :disabled="!categoryID || operatorsLoading" :loading="operatorsLoading" clearable @change="changeContext('operator',$event)"><t-option v-for="row in operators" :key="row.id" :value="row.id" :label="row.display_name" /></BusinessSelect><span v-else>{{ context.operator_name || '本人' }}</span></t-form-item>
          <t-form-item label="合成策略" required-mark><div class="task-create__field"><ComposeStrategyPicker :model-value="strategyID" :filters="{team_id:teamID,category_id:categoryID,operator_id:operatorID,status:'enabled'}" :disabled="!operatorID || !categoryID" placeholder="搜索负责人可用的策略" :status="strategyError?'error':undefined" @update:model-value="changeContext('strategy',$event)" @detail="strategyDetail=$event" /><p v-if="strategyError" class="task-create__error">{{ strategyError }}</p></div></t-form-item>
          </template>
        </ResourceForm></ResourceSection>
        <ResourceSection title="选用素材" variant="form">
          <div class="task-create__heading"><span class="task-create__hint">已选 {{ selected.length }} 条素材</span><t-button v-if="!isSource" variant="outline" class="wt-secondary-button" :disabled="!selectedStrategy" @click="materialVisible=true">选择素材</t-button></div>
          <t-table v-if="selected.length" class="wt-resource-table" :data="selected" :columns="[{colKey:'id',title:'素材 ID',width:80},{colKey:'title',title:'素材名称',width:400,ellipsis:true},...(!isSource ? [{colKey:'op',title:'操作',width:80}] : [])]" row-key="id" size="small" :scroll="{x:'560px'}"><template #op="{row}"><ResourceRowActions><t-button variant="text" @click="removeSelected(row.id)">移除</t-button></ResourceRowActions></template></t-table>
          <p v-else class="task-create__hint">请选择素材</p>
        </ResourceSection>
        <p v-if="isSource" class="task-create__hint">沿用原素材、原模板版本与原成片设置，生成一份新成片。仅可调整模板开放的参数。确认创建时，原素材将加入负责人的“我的素材”，并准备制作所需视频。</p>
        <ResourceSection v-if="selectedStrategy" title="成片设置" variant="form"><ComposeTaskSettings v-model="overrides" :strategy="selectedStrategy" :template="selectedTemplate" :material-count="selected.length" /></ResourceSection>
        <div v-if="selected.length" class="task-create__summary"><strong>已选 {{ selected.length }} 条素材，预计创建 {{ estimated }} 个任务、生成 {{ estimated }} 个成片</strong><t-checkbox v-if="hasPriorWork" v-model="remakeConfirmed">所选素材已有产出或任务，确认本次新建任务，保留旧任务和成片。</t-checkbox></div>
        <div v-if="failures.length" class="task-create__failures"><p v-for="(item,index) in failures" :key="item.material_id || index">素材 {{ item.material_id }} · {{ item.message || item.reason || '未受理' }}</p></div>
      </div>
    </t-loading>
    <template #footer><t-button variant="outline" class="wt-secondary-button" :disabled="saving" @click="close">取消</t-button><t-button theme="primary" :loading="saving" :disabled="!selected.length || !selectedStrategy || (isSource && Boolean(sourceSeed.reason)) || (hasPriorWork && !remakeConfirmed)" @click="submit">创建任务</t-button></template>
  </ResourceOverlay>
  <ComposeTaskMaterialSelector v-if="!isSource" v-model:visible="materialVisible" :context="context" :selected="selected" @selected="acceptSelection" />
</template>
<style scoped>
.task-create {display:grid;gap:28px;min-width:0;}
.task-create > * {min-width:0;}
.task-create .resource-section + .resource-section {margin-top:0;}
.task-create__field{width:100%;display:grid;gap:4px;}
.task-create__error{margin:0;color:var(--wt-danger);font-size:12px;}
.task-create__hint{margin:0;font-size:12px;color:var(--wt-text-tertiary);line-height:1.7;}
.task-create__heading{display:flex;justify-content:space-between;align-items:center;gap:16px;margin-bottom:16px;}
.task-create__summary{padding:14px 16px;background:var(--wt-info-bg);border-left:3px solid var(--wt-primary);font-size:14px;}
.task-create__failures { max-height: 160px; overflow: auto; }
</style>
