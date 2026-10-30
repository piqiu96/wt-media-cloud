<script setup>
import ResourceOverlay from "../../../shared/ui/resource/ResourceOverlay.vue"
import ResourceEmptyState from "../../../shared/ui/resource/ResourceEmptyState.vue"
import ResourceRowActions from "../../../shared/ui/resource/ResourceRowActions.vue"
import { opColumnWidth } from "../../../shared/ui/resource/opColumnWidth.js"
import { importItemActions, importItemPresentation } from "./importItemPresentation.js"
import ResourceForm from "../../../shared/ui/resource/ResourceForm.vue"
import OperationResultSummary from "../../../shared/ui/feedback/OperationResultSummary.vue"
import ResourceStatusBadge from "../../../shared/ui/resource/ResourceStatusBadge.vue"
import CategoryFields from "../../../shared/ui/fields/CategoryFields.vue"
import TagInput from "../../../shared/ui/fields/TagInput.vue"
import {computed,ref,watch} from 'vue'
import {createMediaAccountImportClient} from '../../../shared/api/mediaAccountImports.js'
import {createProfileBindingClient} from '../../../shared/api/profileBindings.js'
import {feedback} from '../../../shared/ui/feedback/index.js'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import {platformLabel} from '../../../shared/utils/platforms.js'
import {runImportItem} from './accountImports.js'
import {importHistoryCounts,importResultSummary} from './importResultSummary.js'
const props=defineProps({visible:Boolean,initialId:{type:String,default:''},historyOnly:Boolean,categories:{type:Array,default:()=>[]},getLocal:{type:Function,required:true}})
const emit=defineEmits(['update:visible','changed','view-account'])
const api=createMediaAccountImportClient(),profiles=createProfileBindingClient()
const fileInput=ref(null),historyLoading=ref(false),openedFromHistory=ref(false)
const screen=ref('input'),batch=ref(null),history=ref([]),groups=ref([]),preview=ref([]),raw=ref(''),platform=ref('bilibili'),groupId=ref(''),categoryType=ref(''),categoryIds=ref([]),tags=ref([]),remark=ref(''),error=ref(''),busy=ref(false),keepRunning=ref(false),requestKey=ref(''),tab=ref('all'),reuseItem=ref(null),candidateId=ref(''),candidates=ref([]),pauseClose=ref(false)
let defaultGroupId = ''
const countLabels={total:'总计',succeeded:'成功',failed:'未完成',skipped:'跳过',running:'执行中',pending:'待处理',uncertain:'待确认'}
const rows=()=>raw.value.split(/\r?\n/).map((s,n)=>({row_number:n+1,input_cookie:s.trim()})).filter(i=>i.input_cookie)
const validRows=computed(()=>preview.value.filter(i=>i.valid))
const content=computed(()=>batch.value?.content_json)
const blocked=computed(()=>batch.value?.counts?.uncertain>0||batch.value?.counts?.running>0)
const continueCount=computed(()=>blocked.value?0:batch.value?.counts?.pending||0)
const shownItems=computed(()=>(content.value?.items||[]).filter(i=>tab.value==='all'||tab.value==='done'&&i.status==='succeeded'||tab.value==='attention'&&!['succeeded','skipped'].includes(i.status)))
const title=computed(()=>screen.value==='input'?'批量接入账号':screen.value==='preview'?'确认接入内容':screen.value==='history'?'接入记录':busy.value?'正在接入账号':'批量接入结果')
const progress=computed(()=>batch.value?Math.round((batch.value.counts.succeeded+batch.value.counts.failed+batch.value.counts.skipped)/batch.value.counts.total*100):0)
const resultPresentation=computed(()=>importResultSummary(batch.value?.counts,busy.value,content.value?.items))
const resultCounts=computed(()=>Object.entries(countLabels).map(([key,label])=>({key,label,value:batch.value?.counts[key]||0,tone:key==='succeeded'?'success':key==='failed'?'danger':key==='uncertain'?'warning':key==='running'?'info':'neutral'})).filter(item=>['total','succeeded'].includes(item.key)||item.value>0))
async function readFile(event){const f=event.target.files?.[0];if(!f)return;if(f.size>2*1024*1024){error.value='文件超过 2 MB，请拆分后再接入';return}raw.value=await f.text();event.target.value=''}
const historyColumns=[{colKey:'created_at',title:'接入时间',width:174},{colKey:'batch',title:'接入批次',minWidth:190},{colKey:'result',title:'接入结果',minWidth:300},{colKey:'op',title:'操作',width:100,fixed:'right'}]
const columns=computed(()=>[{colKey:'row_number',title:'原行号',width:68},{colKey:'account',title:'账号',width:150},{colKey:'window',title:'浏览器窗口',width:132},{colKey:'status',title:'接入状态',width:100},{colKey:'message',title:'结果说明',minWidth:180},{colKey:'op',title:'操作',width:Math.max(100,...shownItems.value.map(item=>opColumnWidth(itemActions(item).map(action=>action.label),{variant:'text',primary:itemActions(item).some(action=>action.primary)}))),fixed:'right'}])
function resetInput() {
 raw.value='';platform.value='bilibili';groupId.value='';defaultGroupId='';categoryType.value='';categoryIds.value=[];tags.value=[];remark.value='';preview.value=[];requestKey.value=''
}
function inputHasChanges() {
 return Boolean(raw.value.trim()||platform.value!=='bilibili'||groupId.value!==defaultGroupId||categoryType.value||categoryIds.value.length||tags.value.length||remark.value.trim())
}
watch(()=>props.visible,async (open,_,onCleanup)=>{
 if(!open)return
 let active=true;onCleanup(()=>{active=false})
 resetInput();openedFromHistory.value=false;error.value='';batch.value=null;groups.value=[];history.value=[];reuseItem.value=null;candidateId.value='';candidates.value=[];pauseClose.value=false;keepRunning.value=false;tab.value='all';screen.value=props.historyOnly?'history':'input';historyLoading.value=true
 try {
  const recent=await api.list();if(!active)return;history.value=recent
  if(props.initialId){const saved=await api.get(props.initialId);if(active){update(saved);screen.value='results'};return}
  if(!props.historyOnly){const local=await props.getLocal();if(!active)return;const result=await local.profileGroups();if(!active)return;groups.value=result?.data?.groups||result?.groups||[];groupId.value=groups.value[0]?.id||'';defaultGroupId=groupId.value}
 } catch(e){if(active)error.value=e.message||'读取接入信息失败，请重试'}
 finally{if(active)historyLoading.value=false}
})
function publicBatch(v){const {execution,...safe}=v;return safe}
function update(v){batch.value=publicBatch(v)}
async function backToHistory(){
 if(busy.value)return
 error.value='';screen.value='history';historyLoading.value=true
 try{history.value=await api.list()}catch(e){fail(e)}finally{historyLoading.value=false}
}
async function reloadBatch(){try{if(batch.value)update(await api.get(batch.value.id))}catch(e){fail(e)}}
function fail(e){error.value=e.message||'本次操作未完成，请重试'}
async function openBatch(id){if(busy.value)return;busy.value=true;error.value='';try{update(await api.get(id));screen.value='results';openedFromHistory.value=true;tab.value='all'}catch(e){fail(e)}finally{busy.value=false}}
async function validate(){
 if(busy.value)return
 error.value='';if(!groupId.value){error.value='请选择浏览器窗口分组';return}busy.value=true
 try {preview.value=await api.preview({platform:platform.value,items:rows()});screen.value='preview';requestKey.value=crypto.randomUUID()}catch(e){fail(e)}finally{busy.value=false}
}
async function createBatch(){
 if(busy.value||!validRows.value.length)return;busy.value=true;error.value=''
 try {const valid=new Set(validRows.value.map(r=>r.row_number));update(await api.create({request_key:requestKey.value,platform:platform.value,group_id:groupId.value,category_ids:categoryIds.value,tags:tags.value,remark:remark.value,items:rows().filter(r=>valid.has(r.row_number))}));raw.value='';screen.value='results'}catch(e){fail(e)}finally{busy.value=false}
 if(batch.value)await runPending()
}
async function execute(row,reconcile=false){
 const local=await props.getLocal()
 await runImportItem({api,local,profiles,batch:batch.value,row,reconcile,onChange:update})
}
async function runPending(onlyRow=null){
 if(busy.value||blocked.value)return;busy.value=true;keepRunning.value=true;error.value=''
 try {while(keepRunning.value){const i=content.value.items.find(i=>i.status==='pending'&&(onlyRow===null||i.row_number===onlyRow));if(!i)break;await execute(i.row_number);if(blocked.value||onlyRow!==null)break}}
 catch(e){fail(e);await reloadBatch()}
 finally{busy.value=false;keepRunning.value=false;emit('changed');if(pauseClose.value)emit('update:visible',false)}
}
async function action(i,action,extra={}){
 if(busy.value)return;busy.value=true;error.value=''
 try {
 if(action==='reconcile')await execute(i.row_number,true)
 else update(await api.action(batch.value.id,i.row_number,{expected_revision:batch.value.revision,action,...extra}))
 }catch(e){fail(e);await reloadBatch()}finally{busy.value=false;emit('changed')}
 if(action==='retry'&&content.value?.items.find(x=>x.row_number===i.row_number)?.status==='pending')await runPending(i.row_number)
}
function itemActions(item){return importItemActions(item,{busy:busy.value,blocked:blocked.value})}
function itemPresentation(item){return importItemPresentation(item,busy.value)}
async function onItemAction(item,key){
 if(busy.value)return
 if(key==='view')emit('view-account',item.existing_account_id||item.account_id)
 else if(key==='open')await openWindow(item)
 else if(key==='reuse')await chooseReuse(item)
 else await action(item,key)
}
async function openWindow(i){try{await(await props.getLocal()).profileOpen(i.bit_profile_id)}catch(e){fail(e)}}
async function chooseReuse(i){
 try {candidates.value=await profiles.listProfiles();candidateId.value='';reuseItem.value=i}catch(e){fail(e)}
}
async function confirmReuse(){
 if(!candidateId.value)return
 const p=candidates.value.find(p=>String(p.id)===candidateId.value)
 if(!await feedback.confirmWarning({title:'使用这个空闲窗口？',message:`将先检查「${p?.bit_name||candidateId.value}」的平台登录情况，确认可用后接入。`,confirmText:'使用并检查'}))return
 const i=reuseItem.value;reuseItem.value=null;await action(i,'reuse',{candidate_profile_id:candidateId.value,confirmed:true});await runPending(i.row_number)
}
async function close(){
 if(busy.value){if(keepRunning.value&&screen.value==='results'){keepRunning.value=false;pauseClose.value=true};return}
 if(['input','preview'].includes(screen.value)&&inputHasChanges()&&!await feedback.confirmWarning({title:'放弃本次输入？',message:'尚未确认的接入内容不会保存。',confirmText:'放弃输入'}))return
 emit('update:visible',false)
}
</script>
<template>
<ResourceOverlay :kind="screen === 'input' ? 'drawer' : 'dialog'" :visible="visible" :title="title" :width="960" :busy="busy" :esc-closable="false" @close="close">
 <div class="import-body">
 <ol v-if="screen==='input'||screen==='preview'" class="import-steps"><li :class="{active:screen==='input'}">1　输入资料</li><li :class="{active:screen==='preview'}">2　确认接入</li><li>3　结果处理</li></ol>
 <FeedbackAlert v-if="error" :message="error" theme="error" />
 <template v-if="screen==='input'">
  <p class="intro">每行一个 Cookie，先校验内容，再逐个接入。已有账号不会被重复创建。</p>
  <ResourceForm>
   <div class="form-grid">
    <t-form-item label="平台"><t-select v-model="platform"><t-option value="bilibili" label="B站"/><t-option value="baidu" label="百度"/></t-select></t-form-item>
    <t-form-item label="新窗口分组"><t-select v-model="groupId" placeholder="选择已有浏览器分组"><t-option v-for="g in groups" :key="g.id" :value="g.id" :label="g.name"/></t-select></t-form-item>

   </div>
   <t-form-item label="业务分类"><CategoryFields v-model="categoryIds" v-model:type="categoryType" :categories="categories" /></t-form-item>
   <t-form-item label="标签"><TagInput v-model="tags" /></t-form-item>
   <t-form-item label="运营备注"><t-input v-model="remark" :maxlength="500" placeholder="账号来源、用途或注意事项（应用于本批账号）"/></t-form-item>
   <div class="import-file"><span>每行一个账号，同一批次使用同一平台。</span><t-button variant="outline" size="small" @click="fileInput.click()">选择 TXT 文件</t-button><input ref="fileInput" type="file" accept=".txt,text/plain" hidden @change="readFile"/></div>
   <t-form-item label="Cookie 列表"><t-textarea v-model="raw" :autosize="{minRows:7,maxRows:12}" placeholder="每行粘贴一个 Cookie；支持 Cookie 请求头或单行 JSON 数组"/></t-form-item>
   <p class="hint">每批最多 100 行、合计 2 MB。优先创建新窗口，无法新建时可按行选择空闲窗口。</p>
  </ResourceForm>
 </template>
 <template v-else-if="screen==='preview'">
  <div class="preview-summary"><strong>{{ platformLabel(platform) }} · {{ validRows.length }} 行可接入</strong><span>{{ preview.length-validRows.length }} 行无效，不会接入</span></div>
  <p class="hint">确认前不会创建窗口或账号。行号对应原始输入，后续可按行继续处理。</p>
  <t-table class="wt-resource-table" :data="preview" row-key="row_number" :columns="[{colKey:'row_number',title:'原行号',width:90},{colKey:'account',title:'待接入资料',width:180},{colKey:'plan',title:'窗口安排',width:190},{colKey:'message',title:'校验结果',minWidth:240}]" :scroll="{x:'700px'}" :max-height="330" size="small"><template #account="{row}">输入第 {{ row.row_number }} 行</template><template #plan="{row}">{{ row.valid?'优先新建独立窗口':'不接入' }}</template><template #message="{row}"><span class="result-message" :class="{'error-text':!row.valid}">{{ row.message }}</span></template></t-table>
 </template>
 <template v-else-if="screen==='history'">
  <p class="history-intro">最近 20 次接入。查看逐项结果，继续处理未完成的账号。</p>
  <t-table v-if="historyLoading||history.length" class="wt-resource-table" :data="history" :columns="historyColumns" row-key="id" :loading="historyLoading" :max-height="420" :scroll="{x:'764px'}" size="small" hover>
   <template #created_at="{row}"><span class="history-time">{{ new Date(row.created_at).toLocaleString('zh-CN',{year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',hour12:false}) }}</span></template>
   <template #batch="{row}"><div class="history-batch"><strong>{{ platformLabel(row.platform) }} · {{ row.counts.total }} 个账号</strong><small v-if="row.content_json?.remark">{{ row.content_json.remark }}</small></div></template>
   <template #result="{row}"><div class="history-counts"><ResourceStatusBadge v-for="item in importHistoryCounts(row.counts)" :key="item.key" :tone="item.tone" :label="`${item.label} ${item.value}`" /></div></template>
   <template #op="{row}"><ResourceRowActions><t-button variant="text" size="small" :disabled="busy" @click="openBatch(row.id)">查看结果</t-button></ResourceRowActions></template>
  </t-table>
  <ResourceEmptyState v-else-if="!error" title="暂无接入记录" description="完成首次批量 CK 接入后，可在这里查看结果。" />
 </template>
 <template v-else-if="batch">
  <t-button v-if="openedFromHistory" class="history-back" theme="primary" variant="text" :disabled="busy" @click="backToHistory">返回接入记录</t-button>
  <OperationResultSummary :title="resultPresentation.title" :tone="resultPresentation.tone" :running="busy" :description="resultPresentation.description" :counts="resultCounts"><span>{{ platformLabel(batch.platform) }} · {{ content.remark || '账号批量接入' }}</span></OperationResultSummary>
  <t-progress v-if="busy" :percentage="progress" :label="false" style="margin-bottom:16px"/>
  <t-tabs v-model="tab" class="result-tabs"><t-tab-panel value="all" label="全部"/><t-tab-panel value="attention" label="需要继续处理"/><t-tab-panel value="done" label="成功接入"/></t-tabs>
  <t-table class="wt-resource-table" :data="shownItems" :columns="columns" row-key="row_number" :max-height="360" :scroll="{x:'max-content'}" size="small">
   <template #account="{row}"><div>{{ row.name||'待识别账号' }}<small v-if="row.platform_account_id">UID：{{ row.platform_account_id }}</small></div></template>
   <template #window="{row}"><span>{{ row.window_name||(row.browser_profile_id?'窗口 #'+row.browser_profile_id:'待准备窗口') }}</span></template>
   <template #status="{row}"><ResourceStatusBadge :tone="itemPresentation(row).tone" :label="itemPresentation(row).label" /></template>
   <template #message="{row}"><span class="result-message" :class="{'error-text':itemPresentation(row).tone==='danger'}">{{ itemPresentation(row).message }}</span></template>
   <template #op="{row}"><ResourceRowActions>
    <t-button v-for="itemAction in itemActions(row)" :key="itemAction.key" size="small" variant="text" :class="{'is-primary':itemAction.primary}" :disabled="itemAction.disabled" @click="onItemAction(row,itemAction.key)">{{ itemAction.label }}</t-button>
   </ResourceRowActions></template>
   <template #empty><ResourceEmptyState :title="tab==='attention'?'没有需要处理的账号':tab==='done'?'还没有成功接入的账号':'暂无接入明细'" /></template>
  </t-table>
 </template>
 </div>
 <template #footer>
 <div class="footer">
  <t-button variant="outline" :disabled="busy&&!keepRunning" @click="close">{{ busy&&pauseClose?'正在暂停…':busy&&keepRunning?'暂停并返回':['input','preview'].includes(screen)?'取消':'关闭' }}</t-button>
  <t-button v-if="screen==='input'" theme="primary" :loading="busy" @click="validate">校验并预览</t-button>
  <template v-if="screen==='preview'"><t-button variant="outline" :disabled="busy" @click="screen='input'">返回修改</t-button><t-button theme="primary" :loading="busy" :disabled="!validRows.length" @click="createBatch">确认接入 {{ validRows.length }} 个账号</t-button></template>
  <template v-if="screen==='results'"><t-button v-if="continueCount" theme="primary" :disabled="busy" @click="runPending()">继续接入（{{ continueCount }}）</t-button></template>
 </div>
 </template>
</ResourceOverlay>
<ResourceOverlay :visible="!!reuseItem" title="选择空闲窗口" :width="560" :busy="busy" @close="reuseItem=null">
 <p class="intro">当前无法新建窗口。请选择本人已有窗口，系统会先检查占用和登录情况。</p>
 <ResourceForm><t-form-item label="浏览器窗口"><t-select v-model="candidateId" filterable placeholder="选择一个已有窗口"><t-option v-for="p in candidates.filter(p=>p.enabled!==false&&p.local_state==='active')" :key="p.id" :value="String(p.id)" :label="p.bit_name||'窗口 '+p.id"/></t-select></t-form-item></ResourceForm>
 <template #footer><t-button variant="outline" :disabled="busy" @click="reuseItem=null">取消</t-button><t-button theme="primary" :disabled="!candidateId||busy" @click="confirmReuse">使用并检查</t-button></template>
</ResourceOverlay>
</template>
<style scoped>
.import-body { min-width: 0; }
.import-steps { display: flex; flex-wrap: wrap; list-style: none; margin: 0 0 20px; padding: 0; gap: 12px 30px; color: var(--wt-text-tertiary); font-size: 13px; }
.import-steps .active { color: var(--wt-primary); font-weight: 500; }
.intro { margin: 0 0 20px; color: var(--wt-text-secondary); line-height: 1.7; }
.form-grid { display: grid; grid-template-columns: repeat(2,minmax(0,1fr)); gap: 0 24px; }
.form-grid > * { min-width: 0; }
.import-file { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px; margin: 4px 0 12px; font-size: 12px; color: var(--wt-text-tertiary); }
.hint, small { color: var(--wt-text-tertiary); font-size: 12px; line-height: 1.7; }
small { display: block; overflow-wrap: anywhere; }
.footer { display: flex; align-items: center; justify-content:flex-end; gap: 8px; width: 100%; flex-wrap: wrap; }
.preview-summary { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px; margin: 4px 0 14px; }
.preview-summary span { color: var(--wt-text-tertiary); font-size: 12px; }
.result-message { white-space: normal; overflow-wrap: anywhere; line-height: 1.7; }
.error-text { color: var(--wt-danger); }
.history-back { margin:0 0 16px; }
.result-tabs { margin: 16px 0; }
.history-intro { margin:0 0 16px;color:var(--wt-text-tertiary);font-size:13px;line-height:1.7; }
.history-time { white-space:nowrap;font-variant-numeric:tabular-nums; }
.history-batch { overflow-wrap:anywhere;line-height:1.7; }
.history-batch strong { font-weight:500;color:var(--wt-text-primary); }
.history-counts { display:flex;flex-wrap:wrap;gap:6px 8px; }
@media (max-width: 600px) { .form-grid { grid-template-columns: minmax(0,1fr); } }
</style>
