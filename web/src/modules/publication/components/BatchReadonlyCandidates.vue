<script setup>
import { computed, ref, watch } from 'vue';
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue';
import { formatDurationMs } from '../../../shared/utils/units.js';
import { batchAccountName, batchOutputName, batchOutputProblem } from '../useBatchResourceCatalog.js';
import { publicationTitle } from '../platformContent.js';
import { contentIssues } from '../eligibility.js';
import { localContentIssue, presentBatchIssues } from '../batchIssuePresentation.js';

const props=defineProps({batch:{type:Object,required:true},catalog:{type:Object,required:true},categoryName:Function});
const page=ref(1),pageSize=ref(10);
const rows=computed(()=>props.batch.config?.candidates||[]);
watch(()=>props.batch.id,()=>{page.value=1;});
const columns=[
  {colKey:'content',title:'发布内容',width:300},
  {colKey:'account',title:'发布账号',width:140},
  {colKey:'output',title:'发布成片',width:200},
  {colKey:'configuration',title:'配置状态',width:220},
];
const account=row=>props.catalog.accounts.get(row.media_account_id);
const output=row=>props.catalog.outputs.get(row.composite_output_id);
const accountName=row=>account(row)?batchAccountName(account(row)):'账号 ID：'+row.media_account_id;
const outputName=row=>output(row)?batchOutputName(output(row)):'成片 ID：'+row.composite_output_id;
function issues(row){
  const current=[];
  if(!account(row))current.push({message:'账号信息暂不可用'});
  else if(account(row).eligible===false||account(row).enabled===false)current.push({message:account(row).reason||'账号当前不可用'});
  if(!output(row))current.push({message:'成片信息暂不可用'});
  else{
    const message=batchOutputProblem(output(row),row.category_id,props.batch.team_id,props.batch.platform);
    if(message)current.push({field:'composite_output_id',code:output(row).occupied?'output_occupied':undefined,message});
  }
  current.push(...(props.batch.readiness?.issues||[]).filter(issue=>issue.row_key===row.row_key));
  current.push(...contentIssues({...row,platform:props.batch.platform,content:{...row.content,
    cover_file_id:row.content?.cover_file_id==='0'?'':row.content?.cover_file_id,
    portrait_cover_file_id:row.content?.portrait_cover_file_id==='0'?'':row.content?.portrait_cover_file_id,
  }}).map(issue=>localContentIssue(issue,row.row_key)));
  return presentBatchIssues(current,props.batch.platform);
}
function state(row){
  const problems=issues(row);
  if(!problems.length)return '已就绪';
  if(problems.every(issue=>issue.code==='confirmation_required'))return '待确认';
  if(!account(row)||!output(row))return '待核实';
  return '待完善';
}
</script>

<template>
  <div class="resource-data-table resource-embedded-table is-summary batch-readonly-candidates" role="region" aria-label="批次候选配置，可横向滚动" tabindex="0">
    <t-table row-key="row_key" :data="rows" :columns="columns" table-layout="fixed" :scroll="{x:'860px'}"
      :pagination="rows.length>10?{current:page,pageSize,total:rows.length,pageSizeOptions:[10,20,50],showJumper:false}:undefined"
      @page-change="value=>{page=value.current;pageSize=value.pageSize}">
      <template #content="{row}"><span class="batch-readonly-title" :title="publicationTitle(batch.platform,row.content)">{{publicationTitle(batch.platform,row.content)||'待填写发布文案'}}</span><small>{{categoryName(row.category_id)}}</small></template>
      <template #account="{row}"><span class="batch-readonly-text" :title="accountName(row)">{{accountName(row)}}</span></template>
      <template #output="{row}"><span class="batch-readonly-text batch-readonly-output" :title="outputName(row)">{{outputName(row)}}</span><small>成片 ID：{{row.composite_output_id}} · 时长：{{formatDurationMs(output(row)?.media?.duration_ms)}}</small></template>
      <template #configuration="{row}"><ResourceStatusBadge :label="state(row)" :tone="state(row)==='已就绪'?'success':'warning'"/><small v-if="issues(row).length" class="batch-readonly-text">{{issues(row)[0].message}}</small><details v-if="issues(row).length>1" class="batch-readonly-issues"><summary>另 {{issues(row).length-1}} 项</summary><ul><li v-for="(issue,index) in issues(row).slice(1)" :key="index">{{issue.message}}</li></ul></details></template>
    </t-table>
  </div>
</template>

<style scoped>
.batch-readonly-candidates{overflow-x:auto;min-width:0;max-width:100%}.batch-readonly-title,.batch-readonly-text{display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden;overflow-wrap:anywhere;white-space:normal}.batch-readonly-title{font-weight:500}.batch-readonly-candidates small{display:block;margin-top:4px;white-space:normal;font-size:12px;line-height:20px;color:var(--wt-text-tertiary)}.batch-readonly-issues{font-size:12px;line-height:20px;color:var(--wt-text-secondary)}.batch-readonly-issues summary{cursor:pointer;color:var(--wt-primary)}.batch-readonly-issues ul{padding-left:16px;margin:6px 0 0}
</style>
