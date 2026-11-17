import {it,expect} from 'vitest';
import {batchCreationDraft, migrateBatchCreationDrafts, promoteBatchDraft} from '../creationDrafts.js';
import {formalBatch,runBatchCommand} from '../batchLifecycle.js';
const legacy=(id,more={})=>({id,name:'旧配置'+id,platform:'B站',targets:[{category:'火影忍者',count:2}],phase:'待资源配置',control:'待配置',candidates:[],...more});
it('legacy drafts are not formal batches; plan skeletons stay outside personal recovery',()=>{
 const d={batches:[legacy(1),legacy(2,{planId:9}),legacy(3)],tasks:[{id:30,batchId:3,status:'待执行'}],plans:[]};
 migrateBatchCreationDrafts(d);
 expect(d.batches.filter(b=>formalBatch(d,b)).map(b=>b.id)).toEqual([3]);
 expect(d.publishDraft).toMatchObject({name:'旧配置1',active:true,batchId:null});
 expect(d.publishDraft.platforms).toEqual(['B站']);
 expect(d.migratedBatchDraftIds).toEqual([1]);expect(d.batches).toHaveLength(3);
 d.publishDraft={active:false};migrateBatchCreationDrafts(d);expect(d.publishDraft.active).toBe(false);
});
it('current creation buffer wins, and a legacy full configuration is recoverable once',()=>{
 const d={batches:[legacy(1,{draft:{step:2,resourcesLocked:true,rows:[{key:'r',title:'人工文案'}],accountOrder:[8]}})],tasks:[],plans:[],publishDraft:{active:true,name:'当前续编'}};
 migrateBatchCreationDrafts(d);expect(d.publishDraft.name).toBe('当前续编');
 d.publishDraft={active:false};promoteBatchDraft(d);
 expect(d.publishDraft).toMatchObject({name:'旧配置1',step:2,resourcesLocked:true,accountOrder:[8],batchId:null});
 expect(d.publishDraft.rows[0].title).toBe('人工文案');
 d.publishDraft={active:false};promoteBatchDraft(d);expect(d.publishDraft.active).toBe(false);
});
it('copy inherits strategy only and cannot create rows or overwrite another creation',()=>{
 const source=legacy(1,{phase:'内容预检通过',rules:{taskIntervalSeconds:12},firstStartedAt:'now',candidates:[{accountId:7}],draft:{rows:[{assetId:9}]}});
 const d={batches:[source],tasks:[{batchId:1,status:'已发布'}],plans:[]};
 const draft=runBatchCommand(d,source,'copy',{name:'副本'});
 expect(d.batches).toHaveLength(1);expect(d.tasks).toHaveLength(1);
 expect(d.publishDraft).toEqual(draft);expect(draft).toMatchObject({name:'副本',batchId:null,planId:null,step:0,rows:[],assets:[],resourcesLocked:false,taskIntervalSeconds:12,sourceBatchId:1});
 expect(draft.firstStartedAt).toBeUndefined();expect(draft.candidates).toBeUndefined();
 expect(()=>runBatchCommand(d,source,'copy',{name:'再复制'})).toThrow(/未完成/);
 expect(d.publishDraft.name).toBe('副本');
 expect(batchCreationDraft(source).targets).toEqual(source.targets);
});
