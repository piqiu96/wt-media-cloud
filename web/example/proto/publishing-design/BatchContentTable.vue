<script setup>
import { computed } from 'vue';
import ResourceRowActions from '../../../src/shared/ui/resource/ResourceRowActions.vue';
import { contentIssues, contentState, approvalIssue } from './batchContentActions.js';
import '../../../src/shared/styles/resource-data-table.css';
const props=defineProps({rows:{type:Array,required:true},db:{type:Object,required:true},selectable:Boolean,selected:{type:Array,default:()=>[]},pagination:{type:Object,default:undefined}});
const emit=defineEmits(['edit','preview','select-change','page-change']);
const columns=computed(()=>[
  ...(props.selectable?[{colKey:'row-select',type:'multiple',width:48,fixed:'left'}]:[]),
  {colKey:'content',title:'发布内容',width:300},
  {colKey:'account',title:'发布账号',width:140},
  {colKey:'asset',title:'发布成片',width:200},
  {colKey:'configuration',title:'配置状态',width:220},
  {colKey:'operation',title:'操作',width:80,fixed:'right'}
]);
const asset=row=>props.db.assets.find(a=>a.id===row.assetId);
const account=row=>props.db.accounts.find(a=>a.id===row.accountId);
const title=row=>row.platform==='百家号'?row.description:row.title;
const problems=row=>contentIssues(row,props.db).sort((a,b)=>(a===approvalIssue?1:0)-(b===approvalIssue?1:0)).map(message=>({[approvalIssue]:'内容与封面待确认','请按内容确认创作声明':'未选择创作声明','请填写标题':'标题未填写','请填写作品描述':'作品描述未填写'}[message]||message));
const state=row=>contentState(row,props.db);
const hasSuggestion=row=>Object.values(row.ai?.suggestions||{}).some(s=>s.status==='待采用'&&!s.error&&!s.expired);
</script>
<template>
  <t-table class="resource-data-table resource-embedded-table is-summary is-flush batch-content-table" row-key="candidateId" :data="rows" :columns="columns" table-layout="fixed" hover
    :selected-row-keys="selected" :reserve-selected-row-on-paginate="true" :pagination="pagination"
    @select-change="keys=>emit('select-change',keys)" @page-change="value=>emit('page-change',value)">
    <template #content="{row}">
      <div class="content-identity"><button class="content-cover" type="button" :aria-label="`预览${title(row)||asset(row)?.title}封面`" @click="emit('preview',{kind:'cover',row})"><img :src="row.cover||asset(row)?.cover" alt="发布封面" /></button>
        <div><span class="content-title" :title="title(row)">{{title(row)||'待填写发布文案'}}</span><span v-if="row.platform!=='百家号'&&row.description" class="content-summary">{{row.description}}</span>
          <div class="content-meta"><small>{{row.category}}</small><t-tag v-if="hasSuggestion(row)" size="small" theme="primary" variant="light">AI 建议</t-tag></div>
        </div>
      </div>
    </template>
    <template #account="{row}"><span class="content-title content-account">{{account(row)?.name||'账号不可用'}}</span></template>
    <template #asset="{row}"><span class="content-title content-asset">{{asset(row)?.title||'成片不可用'}}</span><small>V-{{row.assetId}} · {{asset(row)?.duration||'时长未知'}}</small></template>
    <template #configuration="{row}"><t-tag :theme="state(row)==='已就绪'?'success':state(row)==='待完善'?'warning':'default'" variant="light">{{state(row)}}</t-tag><small v-if="problems(row).length" class="content-problem">{{problems(row)[0]}}</small>
      <details v-if="problems(row).length>1" class="content-problems"><summary>另 {{problems(row).length-1}} 项</summary><ul><li v-for="problem in problems(row).slice(1)" :key="problem">{{problem}}</li></ul></details>
    </template>
    <template #operation="{row}"><ResourceRowActions><t-button class="is-primary" variant="text" theme="primary" size="small" @click="emit('edit',row)">编辑</t-button></ResourceRowActions></template>
    <template #empty><div class="content-empty"><slot name="empty">暂无内容</slot></div></template>
  </t-table>
</template>
<style scoped>
.content-identity{display:flex;align-items:center;gap:12px;min-width:0}.content-identity>div{min-width:0}.content-cover{width:64px;height:48px;flex:none;padding:0;border:1px solid var(--wt-color-border);border-radius:4px;overflow:hidden;background:var(--wt-color-bg-subtle);cursor:zoom-in}.content-cover img{width:100%;height:100%;object-fit:contain}.content-title,.content-problem{display:-webkit-box;-webkit-box-orient:vertical;-webkit-line-clamp:2;overflow:hidden;overflow-wrap:anywhere}.content-title{font-weight:500}.content-account,.content-asset{font-weight:400}.content-asset{color:var(--wt-text-secondary)}.content-summary{display:-webkit-box;-webkit-box-orient:vertical;-webkit-line-clamp:1;overflow:hidden;color:var(--wt-color-text-secondary);font-size:12px;line-height:20px;margin-top:4px}.content-meta{display:flex;align-items:center;flex-wrap:wrap;gap:6px;margin-top:4px}.content-meta small{margin:0!important}.content-problems{font-size:12px;color:var(--wt-color-text-secondary);white-space:normal;margin-top:2px}.content-problems summary{cursor:pointer;color:var(--wt-color-primary);width:fit-content}.content-problems ul{margin:6px 0 0;padding-left:16px;line-height:20px}.content-empty{display:grid;justify-items:center;gap:8px;padding:24px}
</style>
