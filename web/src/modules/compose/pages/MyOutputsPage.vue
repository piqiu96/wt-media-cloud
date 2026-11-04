<script setup>
import { computed,onMounted,onUnmounted,ref } from 'vue'
import { useRoute,useRouter } from 'vue-router'
import BusinessSelect from '../../../shared/ui/fields/BusinessSelect.vue'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import { feedback } from '../../../shared/ui/feedback/index.js'
import ResourceCard from '../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../shared/ui/resource/ResourcePageHeader.vue'
import { opColumnWidth } from '../../../shared/ui/resource/opColumnWidth.js'
import { formatDateTime } from '../../../shared/utils/datetime.js'
import { publicObjectReadUrl } from '../../../shared/utils/objectReadUrl.js'
import { useAutoRefresh } from '../../../shared/utils/useAutoRefresh.js'
import { createComposeClient } from '../../../shared/api/compose.js'
import { createMaterialsClient } from '../../../shared/api/materials.js'
import { createFileTransferClient } from '../../../shared/api/fileTransfer.js'
import { createSessionClient } from '../../../shared/api/session.js'
import { createdRangeParams } from '../createdRange.js'
import { missingOperatorOption,operatorIdFromOption } from '../operatorFilter.js'
import { OUTPUT_PLATFORMS } from '../strategySettings.js'
import { canDownloadOutput,outputDownloadState,outputVideoState } from '../outputManagement.js'
import { downloadedFileName } from '../../transfer/downloadFacts.js'
import { isDesktopRuntime,openSavedFile,savedFileStates } from '../../transfer/desktopBridge.js'
import { useDownloadCentre } from '../../transfer/downloadCentre.js'
import { createDownloadFailureMessage } from '../../transfer/downloadErrors.js'
import ComposeOutputTable from '../components/ComposeOutputTable.vue'
import ComposeOutputMaterialDetail from '../components/ComposeOutputMaterialDetail.vue'
import ComposeOutputDetail from '../components/ComposeOutputDetail.vue'
import ComposeTaskCreateDialog from '../components/ComposeTaskCreateDialog.vue'
import ComposeStrategySummary from '../components/ComposeStrategySummary.vue'
import MaterialDetailDrawer from '../../materials/MaterialDetailDrawer.vue'
const client=createComposeClient(),materialsClient=createMaterialsClient(),transferClient=createFileTransferClient(),sessionClient=createSessionClient(),downloadCentre=useDownloadCentre(),route=useRoute(),router=useRouter()
const activeView=ref('batches'),rows=ref([]),total=ref(0),loading=ref(false),error=ref(''),advancedFiltersVisible=ref(false)
const pages=ref({batches:{current:1,pageSize:20},outputs:{current:1,pageSize:20}}),pagination=computed(()=>pages.value[activeView.value])
const filters=ref({search:'',team_id:'',category_id:'',operator:'',platform:'',video_status:'',created_range:[]})
const currentUser=ref(null),teams=ref([]),categories=ref([]),enabledCategories=ref([]),operatorOptions=ref([]),operatorsLoading=ref(false)
const selectedRowKeys=ref([]),knownRows=ref({}),batchDownloading=ref(false),batchResult=ref(''),downloadingID=ref(null)
const downloadTasks=ref([]),localFiles=ref({}),downloadKnown=ref(false),detailVisible=ref(false),detail=ref(null),previewUrl=ref(''),previewLoading=ref(false),detailLoading=ref(false)
const materialGroup=ref(null),materialGroupVisible=ref(false),materialDrawer=ref({visible:false,row:null,loading:false}),strategyDrawer=ref({visible:false,row:null,loading:false}),createVisible=ref(false),createSource=ref(null)
let operatorRequest=0,loadSequence=0,downloadPoller=null,detailSequence=0
const isAdmin=computed(()=>currentUser.value?.role==='admin'),isSenior=computed(()=>currentUser.value?.role==='senior_operator'),canFilterOperators=computed(()=>isAdmin.value||isSenior.value)
const operatorFilterOptions=computed(()=>operatorOptions.value.map(option=>({value:`op:${option.id}`,label:option.display_name})))
const categoryFilterOptions=computed(()=>{const own=new Set(currentUser.value?.category_ids||[]);return enabledCategories.value.filter(category=>isAdmin.value||isSenior.value||own.has(category.id)).map(row=>({value:row.id,label:row.name}))})
const selectedRows=computed(()=>selectedRowKeys.value.map(id=>knownRows.value[id]).filter(Boolean))
const publishingReason='发布能力尚未交付，暂不能创建发布任务。'
const videoOptions=[{value:'ready',label:'可用'},{value:'failed',label:'失败'},{value:'unknown',label:'未核验'}]
const columns = computed(() => [
 {colKey:'material_id',title:'素材 ID',width:90},
 {colKey:'material',title:'素材 / 作者',width:310},
 {colKey:'category',title:'类目',width:110},
 {colKey:'strategies',title:'使用策略',width:200},
 {colKey:'output_count',title:'实际成片',width:100},
 {colKey:'latest_created_at',title:'最近生成时间',width:155},
 {colKey:'op',title:'操作',width:opColumnWidth(['详情']),fixed:'right'},
])
const tableScroll = computed(()=>({x:`${columns.value.reduce((sum,col)=>sum+(col.width||0),0)}px`}))
function categoryName(id){return categories.value.find(row=>row.id===id)?.name||'—'}
function teamName(id){return teams.value.find(row=>row.id===id)?.name||'—'}
function downloadFor(row){if(!downloadKnown.value)return{key:'unknown'};const name=downloadedFileName(downloadTasks.value,row.id);return outputDownloadState(downloadTasks.value,row.id,name?localFiles.value[name]:null,isDesktopRuntime())}
function videoFor(row){return outputVideoState(row,downloadFor(row))}
function canDownload(row){return canDownloadOutput(row,downloadFor(row))}
async function refreshDownloadFacts(){try{const data=await transferClient.listTasks();const tasks=(Array.isArray(data)?data:data.items||[]).filter(task=>task.asset_type==='composite_output');const names=[...new Set(tasks.map(task=>downloadedFileName(tasks,task.asset_id)).filter(Boolean))];const files=await savedFileStates(names);downloadTasks.value=tasks;localFiles.value=files;downloadKnown.value=true}catch{downloadKnown.value=false}}
async function loadOperators() {
 const request=++operatorRequest
 operatorOptions.value=[]
 operatorsLoading.value=false
 if(!canFilterOperators.value)return
 if(isAdmin.value&&!filters.value.team_id){filters.value.operator='';return}
 operatorsLoading.value=true
 try{
  const data=await client.listComposeOperators({team_id:isAdmin.value?Number(filters.value.team_id):undefined})
  if(request!==operatorRequest)return
  operatorOptions.value=data||[]
  if(missingOperatorOption(filters.value.operator,operatorFilterOptions.value))filters.value.operator=''
 }finally{if(request===operatorRequest)operatorsLoading.value=false}
}

function onTeamFilterChange(){filters.value.operator='';loadOperators().catch(()=>{})}
async function load({resetPage=false,quiet=false}={}) {
 if (resetPage) pagination.value.current = 1
 if (!quiet) loading.value = true
 if(!quiet)error.value=''
 const sequence=++loadSequence
 try{
  const params={page:pagination.value.current,page_size:pagination.value.pageSize,search:filters.value.search.trim(),platform:filters.value.platform,video_status:filters.value.video_status,...createdRangeParams(filters.value.created_range)}
  if(filters.value.category_id)params.category_id=filters.value.category_id
  if(isAdmin.value&&filters.value.team_id)params.team_id=Number(filters.value.team_id)
  const namedOperator=operatorIdFromOption(filters.value.operator)
  if((isAdmin.value||isSenior.value)&&namedOperator)params.operator_id=namedOperator
  const data=await(activeView.value==='batches'?client.listComposeOutputMaterials(params):client.listComposeOutputs(params))
  if(sequence!==loadSequence)return
  rows.value=data.items||[];total.value=data.total||0
  if(activeView.value==='outputs')for(const row of rows.value)knownRows.value[row.id]=row
 }catch(e){if(!quiet&&sequence===loadSequence)error.value=e?.message||'读取成片失败'}finally{if(!quiet&&sequence===loadSequence)loading.value=false}
}
function query(){selectedRowKeys.value=[];load({resetPage:true})}
function reset(){filters.value={search:'',team_id:'',category_id:'',operator:'',platform:'',video_status:'',created_range:[]};query()}
function switchView(value){activeView.value=value;load()}
function paginate(data){pagination.value.current=data.current;pagination.value.pageSize=data.pageSize;load()}
useAutoRefresh(() => load({ quiet: true }), { shouldSkip: () =>detailVisible.value||materialGroupVisible.value||materialDrawer.value.visible||strategyDrawer.value.visible||createVisible.value||selectedRowKeys.value.length>0||batchDownloading.value||downloadingID.value!==null})
function openMaterialGroup(row){materialGroup.value=row;materialGroupVisible.value=true}
async function openMaterial(id){materialDrawer.value={visible:true,row:null,loading:true};try{materialDrawer.value.row=await materialsClient.get(id)}catch(e){feedback.toast.error(e?.message||'读取素材失败');materialDrawer.value.visible=false}finally{materialDrawer.value.loading=false}}
async function openStrategy(id){strategyDrawer.value={visible:true,row:null,loading:true};try{strategyDrawer.value.row=await client.getStrategy(id)}catch(e){feedback.toast.error(e?.message||'读取策略失败');strategyDrawer.value.visible=false}finally{strategyDrawer.value.loading=false}}
function openTask(id){router.push({name:'Compose',query:{task_id:id}})}
function openBatch(id){router.push({name:'Compose',query:{batch_id:id}})}
async function openDetail(row){const sequence=++detailSequence;detailVisible.value=true;detailLoading.value=true;detail.value=null;previewUrl.value='';try{const data=await client.getComposeOutput(row.id);if(sequence!==detailSequence)return;detail.value=data;await refreshDownloadFacts();await retryPreview()}catch(e){feedback.toast.error(e?.message||'打开成片失败');detailVisible.value=false}finally{if(sequence===detailSequence)detailLoading.value=false}}
async function retryPreview(){if(!detail.value?.output?.id||previewLoading.value)return;const id=detail.value.output.id;previewLoading.value=true;try{const link=await client.getComposeOutputVideoUrl(id);if(detail.value?.output?.id===id)previewUrl.value=publicObjectReadUrl(link?.url)}catch{if(detail.value?.output?.id===id)previewUrl.value=''}finally{previewLoading.value=false}}
async function openLocalFile(){const state=downloadFor(detail.value.output);if(state.key!=='downloaded'||!state.fileName)return;try{await openSavedFile(state.fileName)}catch(e){feedback.toast.error(e?.message||'打开本机文件失败');await refreshDownloadFacts()}}
async function download(row){if(downloadingID.value!==null||!canDownload(row))return;downloadingID.value=row.id;try{await client.createOutputDownload(row.id);feedback.toast.info('下载任务已提交，可在下载中心查看进度');downloadCentre.open();await refreshDownloadFacts()}catch(e){feedback.toast.error(createDownloadFailureMessage(e))}finally{downloadingID.value=null}}
async function batchDownload(targets){if(batchDownloading.value)return;batchDownloading.value=true;let submitted=0,skipped=0;const failures=[];try{for(const row of targets){if(!canDownload(row)){skipped++;continue}try{await client.createOutputDownload(row.id);submitted++}catch(e){failures.push(`成片 ${row.id}：${createDownloadFailureMessage(e)}`)}}batchResult.value=`共 ${targets.length} 项：已提交 ${submitted} 个下载任务，跳过 ${skipped} 项，失败 ${failures.length} 项。${failures.join('；')}`;if(submitted)downloadCentre.open();await refreshDownloadFacts()}finally{batchDownloading.value=false}}
async function remake(row){try{const data=await client.getComposeOutput(row.id);if(!data.output?.actions?.remake){feedback.toast.error(data.output?.actions?.remake_reason||'暂不能再次制作');return}createSource.value=data;createVisible.value=true}catch(e){feedback.toast.error(e?.message||'读取原制作配置失败')}}
function onCreated(result){if(result.created?.length){feedback.toast.success(`已创建 ${result.created.length} 个新任务，原成片保留`);if(result.batch?.id)openBatch(result.batch.id)}}
onMounted(async()=>{try{const [me,teamRows,categoryRows,enabledRows]=await Promise.all([sessionClient.me(),client.listTeams(),client.listCategories({scope:'all'}),client.listCategories({scope:'all',enabled:true})]);currentUser.value=me;teams.value=teamRows||[];categories.value=categoryRows||[];enabledCategories.value=enabledRows||[];loadOperators().catch(()=>{})}catch(e){error.value=e?.message||'读取筛选项失败'}await Promise.all([load(),refreshDownloadFacts()]);downloadPoller=setInterval(()=>{if(downloadTasks.value.some(row=>['pending','running'].includes(row.status)))refreshDownloadFacts()},5000);const id=String(route.query.output_id||'');if(/^\d+$/.test(id))openDetail({ id: Number(id) })})
onUnmounted(()=>{loadSequence++;detailSequence++;if(downloadPoller)clearInterval(downloadPoller)})
</script>
<template>
 <t-loading :loading="loading" :show-overlay="true"><div class="wt-resource-page resource-visual-approved my-outputs-page">
  <ResourcePageHeader title="成片管理" description="按素材查看产出，或选择成片下载。" />
  <FeedbackAlert v-if="error" theme="error" :message="error" closable @close="error=''" />
  <ResourceCard class="my-outputs-card">
   <t-tabs :value="activeView" @change="switchView"><t-tab-panel value="batches" label="成片批次" /><t-tab-panel value="outputs" label="成片列表" /></t-tabs>
   <div class="filter-bar">
    <label class="wt-filter-field wt-filter-field--wide"><span class="wt-filter-field__label">综合搜索</span><t-input v-model="filters.search" clearable placeholder="名称、素材 ID、批次 ID" @enter="query" /></label>
    <label v-if="isAdmin" class="wt-filter-field"><span class="wt-filter-field__label">运营团队</span><BusinessSelect kind="team" filter v-model="filters.team_id" clearable @change="onTeamFilterChange"><t-option v-for="team in teams" :key="team.id" :value="team.id" :label="team.name" /></BusinessSelect></label>
    <label class="wt-filter-field"><span class="wt-filter-field__label">类目</span><BusinessSelect kind="category" filter v-model="filters.category_id" :options="categoryFilterOptions" clearable /></label>
    <div class="wt-filter-actions"><t-button theme="primary" @click="query">查询</t-button><t-button variant="outline" class="wt-secondary-button" @click="reset">重置</t-button><t-button variant="text" @click="advancedFiltersVisible=!advancedFiltersVisible">{{ advancedFiltersVisible?'收起高级筛选':'高级筛选' }}</t-button></div>
    <div v-if="advancedFiltersVisible" class="wt-advanced-filters">
     <label v-if="canFilterOperators" class="wt-filter-field"><span class="wt-filter-field__label">负责人</span><BusinessSelect kind="member" filter v-model="filters.operator" :options="operatorFilterOptions" :loading="operatorsLoading" :disabled="operatorsLoading||(isAdmin&&!filters.team_id)" clearable /></label>
     <label class="wt-filter-field"><span class="wt-filter-field__label">适用平台</span><t-select v-model="filters.platform" :options="OUTPUT_PLATFORMS" placeholder="全部" clearable /></label>
     <label class="wt-filter-field"><span class="wt-filter-field__label">视频状态</span><t-select v-model="filters.video_status" :options="videoOptions" placeholder="全部" clearable /></label>
     <label class="wt-filter-field compose-date-filter"><span class="wt-filter-field__label">生成时间</span><t-date-range-picker v-model="filters.created_range" value-type="YYYY-MM-DD" clearable /></label>
     <p class="output-filter-hint">视频状态筛选按文件核验结果，本机下载情况在列表中显示。</p>
    </div>
   </div>
   <FeedbackAlert v-if="batchResult" theme="info" :message="batchResult" closable @close="batchResult=''" />
   <div v-if="activeView==='outputs' && selectedRowKeys.length" class="wt-selection-toolbar"><span>已选 {{ selectedRowKeys.length }} 项</span><t-link theme="primary" @click="selectedRowKeys=[]">清空选择</t-link><t-button theme="primary" :loading="batchDownloading" :disabled="!selectedRows.some(canDownload)" @click="batchDownload(selectedRows)">下载</t-button><span :title="publishingReason"><t-button variant="outline" disabled>去发布</t-button></span><span class="output-filter-hint">{{ publishingReason }}</span></div>
   <template v-if="activeView==='batches'"><p class="output-filter-hint">按素材 ID 汇总不同策略、任务批次下的实际成片。</p><div class="table-scroll-wrap"><t-table class="wt-resource-table" :data="rows" :columns="columns" row-key="material_id" :scroll="tableScroll" size="small" hover empty="暂无成片"><template #material="{row}"><div class="output-related"><strong :title="row.material_title">{{ row.material_title||'—' }}</strong><span>{{ row.author_name||'—' }}</span></div></template><template #category="{row}">{{ categoryName(row.category_id) }}</template><template #strategies="{row}"><div class="output-related"><span v-for="strategy in row.strategies" :key="strategy.id">{{ strategy.name||'—' }}</span></div></template><template #latest_created_at="{row}">{{ formatDateTime(row.latest_created_at) }}</template><template #op="{row}"><t-button size="small" variant="outline" class="wt-secondary-button" @click="openMaterialGroup(row)">详情</t-button></template></t-table></div></template>
   <ComposeOutputTable v-else :rows="rows" v-model:selected-keys="selectedRowKeys" :video-state="videoFor" :can-download="canDownload" :downloading-id="downloadingID" :category-name="categoryName" @detail="openDetail" @download="download" @remake="remake" @material="openMaterial" />
   <div class="pagination-bar"><t-pagination :current="pagination.current" :page-size="pagination.pageSize" :total="total" :page-size-options="[10,20,50]" @change="paginate" /></div>
  </ResourceCard>
  <ComposeOutputMaterialDetail v-model:visible="materialGroupVisible" :material="materialGroup" :video-state="videoFor" :can-download="canDownload" :category-name="categoryName" :downloading-id="downloadingID" :busy="batchDownloading" :publishing-reason="publishingReason" :result="batchResult" @detail="openDetail" @download="download" @downloads="batchDownload" @remake="remake" @batch="openBatch" />
  <t-drawer v-model:visible="detailVisible" header="成片详情" size="min(92vw, 820px)" :close-btn="true" :footer="Boolean(detail?.output && (canDownload(detail.output)||detail.output.actions?.remake))"><t-loading :loading="detailLoading"><ComposeOutputDetail v-if="detail" :detail="detail" :preview-url="previewUrl" :preview-loading="previewLoading" :download-state="downloadFor(detail.output)" management linkable @retry-preview="retryPreview" @open-file="openLocalFile" @open-material="openMaterial" @open-strategy="openStrategy" @open-task="openTask" @open-batch="openBatch" /></t-loading><template #footer><t-space v-if="detail" class="wt-resource-actions"><t-button v-if="canDownload(detail.output)" variant="outline" class="wt-secondary-button" :loading="downloadingID===detail.output.id" @click="download(detail.output)">下载</t-button><t-button v-if="detail.output.actions?.remake" theme="primary" @click="remake(detail.output)">再次制作</t-button></t-space></template></t-drawer>
  <ComposeTaskCreateDialog v-model:visible="createVisible" :source-output="createSource" @created="onCreated" />
  <MaterialDetailDrawer :visible="materialDrawer.visible" :material="materialDrawer.row" :loading="materialDrawer.loading" :categories="categories" mode="readonly" :show-page-link="false" @update:visible="materialDrawer.visible=$event" />
  <t-drawer :visible="strategyDrawer.visible" header="策略详情" size="min(92vw, 880px)" :footer="false" :close-btn="true" @update:visible="strategyDrawer.visible=$event"><t-loading :loading="strategyDrawer.loading"><ComposeStrategySummary v-if="strategyDrawer.row" :strategy="strategyDrawer.row" :team="teamName(strategyDrawer.row.team_id)" :category="categoryName(strategyDrawer.row.category_id)" /></t-loading></t-drawer>
 </div></t-loading>
</template>
<style scoped>
.my-outputs-card{padding:14px 20px 18px;}.wt-advanced-filters>.compose-date-filter{flex:0 0 320px;width:320px;min-width:280px;max-width:100%;}.compose-date-filter :deep(.t-date-range-picker){width:100%;min-width:0;}.output-filter-hint{font-size:12px;line-height:1.6;color:var(--wt-text-tertiary);}.wt-advanced-filters>.output-filter-hint{flex-basis:100%;margin:0;}.output-related{display:flex;flex-direction:column;gap:4px;min-width:0;}.output-related strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-weight:500;}.output-related span{font-size:12px;color:var(--wt-text-secondary);}
</style>
