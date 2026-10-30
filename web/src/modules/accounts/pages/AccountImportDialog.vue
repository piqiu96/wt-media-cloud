<script setup>
import {computed,ref,watch} from 'vue'
import {createMediaAccountImportClient} from '../../../shared/api/mediaAccountImports.js'
import {createProfileBindingClient} from '../../../shared/api/profileBindings.js'
import {feedback} from '../../../shared/ui/feedback/index.js'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import {platformLabel} from '../../../shared/utils/platforms.js'
import {runImportItem} from './accountImports.js'
const props=defineProps({visible:Boolean,initialId:{type:String,default:''},historyOnly:Boolean,categories:{type:Array,default:()=>[]},getLocal:{type:Function,required:true}})
const emit=defineEmits(['update:visible','changed','view-account'])
const api=createMediaAccountImportClient(),profiles=createProfileBindingClient()
const fileInput=ref(null)
const screen=ref('input'),batch=ref(null),history=ref([]),groups=ref([]),preview=ref([]),raw=ref(''),platform=ref('bilibili'),groupId=ref(''),categoryType=ref(''),categoryIds=ref([]),tags=ref([]),tagText=ref(''),remark=ref(''),error=ref(''),busy=ref(false),keepRunning=ref(false),requestKey=ref(''),tab=ref('all'),reuseItem=ref(null),candidateId=ref(''),candidates=ref([]),pauseClose=ref(false)
const labels={pending:'待处理',running:'执行中',succeeded:'已接入',failed:'未完成',skipped:'已跳过',uncertain:'待确认'}
const countLabels={total:'总计',succeeded:'成功',failed:'未完成',skipped:'跳过',running:'执行中',pending:'待处理',uncertain:'待确认'}
const rows=()=>raw.value.split(/\r?\n/).map((s,n)=>({row_number:n+1,input_cookie:s.trim()})).filter(i=>i.input_cookie)
const validRows=computed(()=>preview.value.filter(i=>i.valid))
const content=computed(()=>batch.value?.content_json)
const blocked=computed(()=>batch.value?.counts?.uncertain>0||batch.value?.counts?.running>0)
const continueCount=computed(()=>blocked.value?0:batch.value?.counts?.pending||0)
const shownItems=computed(()=>(content.value?.items||[]).filter(i=>tab.value==='all'||tab.value==='done'&&i.status==='succeeded'||tab.value==='attention'&&!['succeeded','skipped'].includes(i.status)))
const title=computed(()=>screen.value==='input'?'批量接入账号':screen.value==='preview'?'确认接入内容':screen.value==='history'?'接入记录':busy.value?'正在接入账号':'批量接入结果')
const progress=computed(()=>batch.value?Math.round((batch.value.counts.succeeded+batch.value.counts.failed+batch.value.counts.skipped)/batch.value.counts.total*100):0)
const resultTitle=computed(()=>busy.value?'正在接入账号':blocked.value||batch.value?.counts.pending?'本轮接入已暂停':batch.value?.counts.failed?'接入完成，部分账号需处理':'本轮接入完成')
async function readFile(event){const f=event.target.files?.[0];if(!f)return;if(f.size>2*1024*1024){error.value='文件超过 2 MB，请拆分后再接入';return}raw.value=await f.text();event.target.value=''}
const columns=[{colKey:'row_number',title:'原行号',width:68},{colKey:'account',title:'账号',width:150},{colKey:'window',title:'浏览器窗口',width:132},{colKey:'status',title:'接入状态',width:100},{colKey:'message',title:'结果说明',minWidth:180},{colKey:'op',title:'操作',width:225,fixed:'right'}]
watch(()=>props.visible,async open=>{
 if(!open)return
 error.value='';raw.value='';preview.value=[];requestKey.value='';batch.value=null;pauseClose.value=false;keepRunning.value=false;screen.value=props.historyOnly?'history':'input'
 try {history.value=await api.list();if(props.initialId){await openBatch(props.initialId);return}if(!props.historyOnly){const local=await props.getLocal();const result=await local.profileGroups();groups.value=result?.data?.groups||result?.groups||[];groupId.value=groups.value[0]?.id||''}}
 catch(e){error.value=e.message||'读取接入信息失败，请重试'}
})
function addTag(){const v=tagText.value.trim();if(v&&!tags.value.includes(v))tags.value.push(v);tagText.value=''}
function publicBatch(v){const {execution,...safe}=v;return safe}
function update(v){batch.value=publicBatch(v)}
async function showHistory(){if(raw.value.trim()&&!await feedback.confirmWarning({title:'放弃本次输入？',message:'尚未确认的接入内容不会保存。',confirmText:'放弃输入'}))return;raw.value='';try{history.value=await api.list();screen.value='history'}catch(e){fail(e)}}
async function refresh(){try{if(batch.value)update(await api.get(batch.value.id));else history.value=await api.list();error.value=''}catch(e){fail(e)}}
function fail(e){error.value=e.message||'本次操作未完成，请刷新后继续';feedback.toast.error(error.value)}
async function openBatch(id){error.value='';update(await api.get(id));screen.value='results';tab.value='all'}
async function validate(){
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
 catch(e){fail(e);await refresh()}
 finally{busy.value=false;keepRunning.value=false;emit('changed');if(pauseClose.value)emit('update:visible',false)}
}
async function action(i,action,extra={}){
 if(busy.value)return;busy.value=true;error.value=''
 try {
 if(action==='reconcile')await execute(i.row_number,true)
 else update(await api.action(batch.value.id,i.row_number,{expected_revision:batch.value.revision,action,...extra}))
 }catch(e){fail(e);await refresh()}finally{busy.value=false;emit('changed')}
 if(action==='retry'&&content.value?.items.find(x=>x.row_number===i.row_number)?.status==='pending')await runPending(i.row_number)
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
 if(busy.value){keepRunning.value=false;pauseClose.value=true;return}
 if(raw.value.trim()&&screen.value!=='results'&&!await feedback.confirmWarning({title:'放弃本次输入？',message:'尚未确认的接入内容不会保存。',confirmText:'放弃输入'}))return
 emit('update:visible',false)
}
</script>
<template>
<component :is="screen==='input'?'t-drawer':'t-dialog'" :visible="visible" :header="title" width="960px" top="32px" :close-btn="!busy" :close-on-overlay-click="false" :close-on-esc-keydown="false" @close="close">
 <div class="import-body">
 <ol v-if="screen==='input'||screen==='preview'" class="import-steps"><li :class="{active:screen==='input'}">1　输入资料</li><li :class="{active:screen==='preview'}">2　确认接入</li><li>3　结果处理</li></ol>
 <FeedbackAlert v-if="error" :message="error" theme="error" />
 <template v-if="screen==='input'">
  <p class="intro">每行一个 Cookie，先校验内容，再逐个接入。已有账号不会被重复创建。</p>
  <t-form label-align="top">
   <div class="form-grid">
    <t-form-item label="平台"><t-select v-model="platform"><t-option value="bilibili" label="B站"/><t-option value="baidu" label="百度"/></t-select></t-form-item>
    <t-form-item label="新窗口分组"><t-select v-model="groupId" placeholder="选择已有浏览器分组"><t-option v-for="g in groups" :key="g.id" :value="g.id" :label="g.name"/></t-select></t-form-item>
    <t-form-item label="一级分类"><t-select v-model="categoryType" clearable placeholder="选择一级分类" @change="categoryIds=[]"><t-option value="game" label="游戏"/><t-option value="ai" label="AI"/><t-option value="anime" label="二次元"/></t-select></t-form-item>
    <t-form-item label="二级分类"><t-select v-model="categoryIds" multiple clearable :disabled="!categoryType" placeholder="选择二级分类（可多选）"><t-option v-for="c in categories.filter(c=>c.type===categoryType)" :key="c.id" :value="c.id" :label="c.name"/></t-select></t-form-item>
   </div>
   <t-form-item label="标签"><div class="tag-input"><t-tag v-for="t in tags" :key="t" closable @close="tags=tags.filter(x=>x!==t)">{{ t }}</t-tag><t-input v-model="tagText" placeholder="输入标签，回车添加" @enter="addTag"/></div></t-form-item>
   <t-form-item label="备注"><t-input v-model="remark" :maxlength="500" placeholder="本批账号的运营备注（选填）"/></t-form-item>
   <div class="import-file"><span>每行一个账号，同一批次使用同一平台。</span><t-button variant="outline" size="small" @click="fileInput.click()">选择 TXT 文件</t-button><input ref="fileInput" type="file" accept=".txt,text/plain" hidden @change="readFile"/></div>
   <t-form-item label="Cookie 列表"><t-textarea v-model="raw" :autosize="{minRows:7,maxRows:12}" placeholder="每行粘贴一个 Cookie；支持 Cookie 请求头或单行 JSON 数组"/></t-form-item>
   <p class="hint">每批最多 100 行、合计 2 MB。优先创建新窗口，无法新建时可按行选择空闲窗口。</p>
  </t-form>
 </template>
 <template v-else-if="screen==='preview'">
  <div class="preview-summary"><strong>{{ platformLabel(platform) }} · {{ validRows.length }} 行可接入</strong><span>{{ preview.length-validRows.length }} 行无效，不会接入</span></div>
  <p class="hint">确认前不会创建窗口或账号。行号对应原始输入，后续可按行继续处理。</p>
  <t-table :data="preview" row-key="row_number" :columns="[{colKey:'row_number',title:'原行号',width:90},{colKey:'account',title:'待接入资料'},{colKey:'plan',title:'窗口安排'},{colKey:'message',title:'校验结果'}]" :max-height="330" size="small"><template #account="{row}">输入第 {{ row.row_number }} 行</template><template #plan="{row}">{{ row.valid?'优先新建独立窗口':'不接入' }}</template><template #message="{row}"><span :class="{'error-text':!row.valid}">{{ row.message }}</span></template></t-table>
 </template>
 <template v-else-if="screen==='history'">
  <p class="hint">最近 20 次接入，可随时查看和继续处理。</p><t-empty v-if="!history.length" description="暂无接入记录"/>
  <button v-for="h in history" :key="h.id" class="history-row" @click="openBatch(h.id)"><span>{{ platformLabel(h.platform) }} · 接入 {{ h.counts.total }} 个账号<small>{{ new Date(h.created_at).toLocaleString() }}</small></span><span>成功 {{ h.counts.succeeded }} / 待处理 {{ h.counts.pending+h.counts.failed+h.counts.uncertain+h.counts.running }}　查看结果 →</span></button>
 </template>
 <template v-else-if="batch">
  <div class="result-title"><t-icon :name="busy?'time':batch.counts.failed||batch.counts.uncertain?'info-circle':'check-circle'" :class="{attention:batch.counts.failed||batch.counts.uncertain}" size="32px"/><h2>{{ resultTitle }}</h2><p>{{ platformLabel(batch.platform) }} · {{ content.remark || '账号批量接入' }}</p><span>{{ busy?'当前项完成后可暂停':'成功项已保存，可继续处理未完成项' }}</span></div>
  <div class="counts"><div v-for="(label,key) in countLabels" :key="key" :class="key"><span>{{ label }}</span><strong>{{ batch.counts[key] }}</strong></div></div>
  <t-progress v-if="busy" :percentage="progress" :label="false" style="margin-bottom:16px"/>
  <FeedbackAlert v-if="blocked&&!busy" message="有一项结果尚未确认，请先按行核实，再继续接入。" theme="warning"/>
  <t-tabs v-model="tab" class="result-tabs"><t-tab-panel value="all" label="全部"/><t-tab-panel value="attention" label="需要继续处理"/><t-tab-panel value="done" label="成功接入"/></t-tabs>
  <t-table :data="shownItems" :columns="columns" row-key="row_number" :max-height="360" size="small">
   <template #account="{row}"><div>{{ row.name||'待识别账号' }}<small v-if="row.platform_account_id">UID：{{ row.platform_account_id }}</small></div></template>
   <template #window="{row}"><span>{{ row.window_name||(row.browser_profile_id?'窗口 #'+row.browser_profile_id:'待准备窗口') }}</span></template>
   <template #status="{row}"><t-tag variant="light" :theme="row.status==='succeeded'?'success':row.status==='failed'?'danger':row.status==='uncertain'?'warning':'default'">{{ row.step==='manual'&&row.status==='failed'?'需人工登录':labels[row.status] }}</t-tag></template>
   <template #message="{row}"><span :class="{'error-text':row.status==='failed'}">{{ row.message||'等待接入' }}</span></template>
   <template #op="{row}"><t-space size="small" break-line>
    <t-button v-if="row.bit_profile_id&&row.status!=='succeeded'" size="small" variant="text" :disabled="busy" @click="openWindow(row)">打开窗口</t-button>
    <t-button v-if="['running','uncertain'].includes(row.status)||row.step==='manual'&&row.status==='failed'" size="small" variant="text" :disabled="busy" @click="action(row,'reconcile')">{{ row.step==='manual'?'检查并同步':'核实结果' }}</t-button>
    <t-button v-if="row.status==='failed'&&row.creation_unavailable" size="small" variant="text" :disabled="busy||blocked" @click="chooseReuse(row)">选择空闲窗口</t-button>
    <t-button v-if="row.status==='failed'&&row.step!=='manual'&&!row.existing_account_id" size="small" variant="text" :disabled="busy||blocked" @click="action(row,'retry')">{{ row.step==='blocked'?'使用新窗口':'重试' }}</t-button>
    <t-button v-if="row.existing_account_id||row.account_id" size="small" variant="text" :disabled="busy" @click="emit('view-account',row.existing_account_id||row.account_id)">{{ row.existing_account_id?'查看已有账号':'查看账号' }}</t-button>
    <t-button v-if="['failed','pending'].includes(row.status)" size="small" variant="text" :disabled="busy" @click="action(row,'skip')">跳过</t-button>
    <t-button v-if="row.status==='skipped'" size="small" variant="text" :disabled="busy" @click="action(row,'restore')">恢复</t-button>
   </t-space></template>
  </t-table>
 </template>
 </div>
 <template #footer>
 <div class="footer">
  <t-button variant="outline" :disabled="busy" @click="showHistory">接入记录</t-button><span class="spacer"/>
  <t-button variant="outline" @click="close">{{ busy?'暂停并返回':'关闭' }}</t-button>
  <t-button v-if="screen==='input'" theme="primary" :loading="busy" @click="validate">校验并预览</t-button>
  <template v-if="screen==='preview'"><t-button variant="outline" :disabled="busy" @click="screen='input'">返回修改</t-button><t-button theme="primary" :loading="busy" :disabled="!validRows.length" @click="createBatch">确认接入 {{ validRows.length }} 个账号</t-button></template>
  <template v-if="screen==='results'"><t-button variant="outline" :disabled="busy" @click="refresh">刷新结果</t-button><t-button theme="primary" :disabled="busy||!continueCount" @click="runPending()">继续接入（{{ continueCount }}）</t-button></template>
 </div>
 </template>
</component>
<t-dialog :visible="!!reuseItem" header="选择空闲窗口" width="560px" confirm-btn="使用并检查" @close="reuseItem=null" @confirm="confirmReuse">
 <p class="intro">当前无法新建窗口。请选择本人已有窗口，系统会先检查占用和登录情况。</p>
 <t-select v-model="candidateId" filterable placeholder="选择一个已有窗口"><t-option v-for="p in candidates.filter(p=>p.enabled!==false&&p.local_state==='active')" :key="p.id" :value="String(p.id)" :label="p.bit_name||'窗口 #'+p.id"/></t-select>
</t-dialog>
</template>
<style scoped>
.import-steps{display:flex;list-style:none;padding:0 0 20px;gap:30px;color:var(--td-text-color-placeholder);font-size:13px}.import-steps .active{color:var(--td-brand-color);font-weight:500}.import-file{display:flex;align-items:center;justify-content:space-between;margin:4px 0 12px;font-size:12px;color:var(--td-text-color-secondary)}.result-title{flex-direction:column;gap:8px;text-align:center;padding:8px 0}.result-title h2{font-size:20px;font-weight:600;margin:0}.result-title p{margin:0;font-size:13px;color:var(--td-text-color-secondary)}.result-title .t-icon{color:var(--td-success-color)}.result-title .attention{color:var(--td-warning-color)}:deep(.t-table td){font-size:13px}.import-body{max-height:calc(100vh - 250px);overflow:auto;padding:0 2px}.intro{margin:0 0 20px;color:var(--td-text-color-secondary);line-height:1.7}.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:0 24px}.hint,small{color:var(--td-text-color-secondary);font-size:12px;line-height:1.7}small{display:block}.tag-input{display:flex;align-items:center;gap:8px;flex-wrap:wrap;width:100%}.tag-input .t-input__wrap{min-width:220px;flex:1}.footer{display:flex;align-items:center;gap:8px;border-top:1px solid var(--td-component-border);padding-top:20px;margin-top:20px}.spacer{flex:1}.preview-summary,.result-title{display:flex;align-items:center;justify-content:space-between;margin:4px 0 14px}.result-title span,.preview-summary span{color:var(--td-text-color-secondary);font-size:12px}.counts{display:grid;grid-template-columns:repeat(7,1fr);gap:8px;margin:20px 0;background:var(--td-bg-color-secondarycontainer);border-radius:6px;padding:16px 10px}.counts>div{display:flex;flex-direction:column;gap:8px;padding:0 10px}.counts span{font-size:12px;color:var(--td-text-color-secondary)}.counts strong{font-size:22px}.succeeded strong{color:var(--td-success-color)}.failed strong,.error-text{color:var(--td-error-color)}.uncertain strong{color:var(--td-warning-color)}.result-tabs{margin:16px 0}.history-row{display:flex;justify-content:space-between;align-items:center;width:100%;padding:18px 10px;border:0;border-bottom:1px solid var(--td-component-border);background:transparent;text-align:left;color:var(--td-text-color-primary);cursor:pointer}.history-row:hover{background:var(--td-bg-color-container-hover)}
</style>
