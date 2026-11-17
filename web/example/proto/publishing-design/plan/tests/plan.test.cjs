const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');
function app(saved) {
  const storage = new Map(saved ? [['wt-publishing-plan-v1', saved]] : []);
  const source = fs.readFileSync(path.join(__dirname, '../app.js'), 'utf8').split('// Plan event bindings')[0].split('// Plan state engine: independently persisted, mounted only within the shared shell.')[1];
  const ctx = vm.createContext({URL, console, setTimeout:()=>0, clearTimeout(){}, localStorage:{getItem:k=>storage.get(k),setItem:(k,v)=>storage.set(k,v)}, window:{scrollTo(){}}, document:{querySelector:()=>null,addEventListener(){}}});
  vm.runInContext("const adapters={}; const actor=()=>({id:'chen',name:'陈小雨'}); const ownerIssue=()=>''; const disposed=false;" + source + `\nrender=()=>persistPrototype(); globalThis.api={state,get plans(){return plans},newEditor,editPlan,editorSaveEdit,planFromEditor,batchValidation,genBatchSplit,canExecute,openTasks,currentNote,detailView,taskDone,mustUrl,actualByCell,onAction,persistPrototype};`, ctx);
  const api=ctx.api;
 const rawNewEditor=api.newEditor;
 api.newEditor=fresh=>{rawNewEditor(fresh);if(!fresh)api.evaluate("Object.assign(state.editor,{name:'测试计划',dateStart:'2026-11-16',dateEnd:'2026-11-30',target:100,platforms:['B站','百家号'],categories:['游戏攻略','精彩剪辑'],goals:clone(goalsA),batches:[b('TEMP-1','首批','B站',{'游戏攻略':30,'精彩剪辑':20}),b('TEMP-2','次批','百家号',{'游戏攻略':25,'精彩剪辑':25})]})")};
 api.blankNewEditor=rawNewEditor;
  api.act=(act,data={})=>api.onAction(act,{dataset:data});
  api.storage=storage;
  api.evaluate=expr=>vm.runInContext(expr,ctx,{timeout:100});
  return api;
}
const json=x=>JSON.parse(JSON.stringify(x));
test('matrix and batch quantities reject negative, fractional and hidden counts',()=>{
 const a=app();a.newEditor();const e=a.state.editor;
 assert.equal(a.batchValidation(e).length,0);
 e.goals['B站']['游戏攻略']=-1;
 assert.ok(a.batchValidation(e).some(x=>x.includes('非负整数')));
});
test('split then merge preserves every platform/category allocation',()=>{
 const a=app();a.newEditor();const before=json(a.state.editor.batches[0].cats);
 a.act('split-batch',{idx:'0'});a.act('do-split-batch');
 a.act('merge-batch',{idx:'1'});
 assert.deepEqual(json(a.state.editor.batches[0].cats),before);
 assert.equal(a.batchValidation(a.state.editor).length,0);
});
test('saving edits cannot change locked rows or their position and goals',()=>{
 const a=app();const p=a.plans[0];p.status='ready';const before=json(p);a.editPlan(p.id);
 a.state.editor.batches[1].name='不应保存';a.editorSaveEdit();
 assert.deepEqual(json(p),before);
});
test('copy clears aggregate results and all inherited execution metadata',()=>{
 const a=app();a.newEditor(a.plans[2]);
 for(const b of a.state.editor.batches){assert.equal(b.success,0);assert.equal(b.locked,false);assert.equal(b.successCats,undefined);assert.equal(b.phase,'draft');}
});
test('execution is serial and requires all preceding task outcomes to settle',()=>{
 const a=app(),p=a.plans[1],b=p.batches[1];assert.equal(a.canExecute(p,b),true);
 p.batches[0].tasks=[{id:'x',type:'verify',status:'pending'}];
 assert.equal(a.canExecute(p,b),false);
});
test('retrying task remains unresolved and prevents safe completion',()=>{
 const a=app(),p=a.plans[0];p.batches[1].tasks[2].status='retrying';
 assert.equal(a.openTasks(p).length,3);
});
test('cancelling a running batch without task detail waits for reconciliation',()=>{
 const a=app(),p=a.plans[1];p.batches[1].phase='running';
 a.state.modal={type:'cancel-plan',planId:p.id,reason:'停止本地演示'};a.act('do-cancel-plan');
 assert.equal(p.status,'stopping');assert.equal(p.batches[1].phase,'stopping');
});
test('cancel preserves unknown task and records reason',()=>{
 const a=app(),p=a.plans[0];a.state.modal={type:'cancel-plan',planId:p.id,reason:'需求变更'};a.act('do-cancel-plan');
 assert.equal(p.status,'stopping');assert.equal(p.batches[1].tasks[1].status,'pending');
 assert.ok(p.logs.some(x=>x[1].includes('需求变更')));
});
test('successful result requires a public HTTPS URL and cannot count twice',()=>{
 const a=app(),p=a.plans[0],b=p.batches[1],t=b.tasks[1],m={planId:p.id,batchId:b.id,taskId:t.id};
 assert.equal(a.mustUrl('http://example.com/a'),false);assert.equal(a.mustUrl('https://127.0.0.1/a'),false);
 assert.equal(a.mustUrl('https://www.bilibili.com/video/BV1'),true);
 a.taskDone(m,'success',{publishUrl:'javascript:alert(1)'});assert.equal(b.success,17);
 a.taskDone(m,'success',{publishUrl:'https://example.com/review-demo'});a.taskDone(m,'success',{publishUrl:'https://example.com/review-demo'});
 assert.equal(b.success,18);
});
test('makeup cannot bypass stale eligibility or increase original target',()=>{
 const a=app(),p=a.plans[2],target=p.target;
 a.state.drawer={type:'makeup',planId:p.id,platform:'B站',values:{'游戏攻略':5}};a.act('create-makeup');
 assert.equal(p.batches.length,4);
 a.state.drawer={type:'makeup',planId:p.id,platform:'B站',values:{'游戏攻略':4,'精彩剪辑':0}};a.act('create-makeup');
 assert.equal(p.batches.length,5);assert.equal(p.target,target);
 a.state.drawer={type:'makeup',planId:p.id,platform:'百家号',values:{}};a.act('create-makeup');
 assert.equal(p.batches.length,5);
});
test('new plan identities do not repeat after deletion',()=>{
 const a=app();a.newEditor();const p=a.planFromEditor('draft');a.plans.unshift(p);a.plans.splice(0,1);
 assert.notEqual(a.planFromEditor('draft').id,p.id);
});
test('saved prototype restores plans and unfinished editor',()=>{
 const a=app();a.newEditor();a.state.editor.name='恢复输入';a.persistPrototype();
 const b=app(a.storage.get('wt-publishing-plan-v1'));
 assert.equal(b.state.recoveryEditor.name,'恢复输入');assert.equal(b.state.page,'list');assert.equal(b.state.editor,null);assert.equal(b.plans.length,7);
});
test('all editable seeded plans permit a basic information edit',()=>{
 const a=app();
 for(const p of a.plans.filter(p=>['draft','ready'].includes(p.status))){a.editPlan(p.id);a.state.editor.name='修改 '+p.name;const expected=a.state.editor.name;a.editorSaveEdit();assert.equal(p.name,expected,p.id);}
});
test('makeup creation cannot exceed remaining cell deficits',()=>{
 const a=app(),p=a.plans[2],before=p.batches.length;a.state.drawer={type:'makeup',planId:p.id,platform:'B站',values:{'游戏攻略':1000,'精彩剪辑':0}};a.act('create-makeup');
 assert.equal(p.batches.length,before);
});
test('a plan with makeup remains locked against plan editing',()=>{
 const a=app(),p=a.plans[2];a.state.drawer={type:'makeup',planId:p.id,platform:'B站',values:{'游戏攻略':4,'精彩剪辑':0}};a.act('create-makeup');
 const before=json(p);a.editPlan(p.id);assert.equal(a.state.editor,null);assert.deepEqual(json(p),before);
});
test('makeup cannot merge into an original batch',()=>{
 const a=app(),p=a.plans[2];p.status='ready';a.editPlan(p.id);a.state.editor.batches.push({id:'m1',name:'补量',platform:'百家号',cats:{'精彩剪辑':1},phase:'draft',locked:false,type:'makeup',tasks:[]});
 // Explicitly unlock the adjacent base row to exercise the type guard.
 a.state.editor.batches[3].locked=false;a.state.editor.batches[3].phase='draft';
 a.act('merge-batch',{idx:'4'});assert.equal(a.state.editor.batches.length,5);
});
test('oversized recommendation is rejected before any quantity loop',()=>{
 const a=app();a.newEditor();a.state.editor.goals['B站']['游戏攻略']=1000000000;
 assert.doesNotThrow(()=>a.evaluate('api.genBatchSplit(api.state.editor)'));
 const before=json(a.state.editor.batches);a.act('recommend');
 assert.deepEqual(json(a.state.editor.batches),before);assert.equal(a.state.modal.type,'validation');
});
test('corrupt storage stays intact and does not silently overwrite user archive',()=>{
 const a=app('{invalid-json');a.newEditor();assert.equal(a.persistPrototype(),false);
 assert.equal(a.storage.get('wt-publishing-plan-v1'),'{invalid-json');assert.equal(a.plans.length,7);
});

test('detail projection preserves goal totals and never changes the underlying plan',()=>{
 const a=app(),p=a.plans[0],before=JSON.stringify(p);
 const view=a.evaluate('detailView(plans[0])');
 assert.equal(view.actual,45);assert.equal(view.gap,55);assert.equal(view.rate,45);
 assert.equal(view.cells.reduce((sum,c)=>sum+c.actual,0),view.actual);
 assert.equal(view.cells.reduce((sum,c)=>sum+c.target,0),p.target);
 assert.equal(JSON.stringify(p),before);
 assert.deepEqual(json(view.footer.map(x=>x.key)),[]);
 assert.deepEqual(json(view.notice.actions.map(x=>x.key)),['view-batch','workbench']);
});
test('detail footer reflects existing serial execution, deficit and ended eligibility',()=>{
 const a=app();
 assert.deepEqual(json(a.evaluate('detailView(plans[1]).footer.map(a=>a.key)')),['execute']);
 assert.deepEqual(json(a.evaluate('detailView(plans[2]).footer.map(a=>a.key)')),['end-plan','makeup']);
 assert.deepEqual(json(a.evaluate('detailView(plans[5]).footer.map(a=>a.key)')),[]);
 a.plans[1].batches[0].phase='running';
 assert.equal(a.evaluate('detailView(plans[1]).footer.some(a=>a.key==="execute")'),false);
});

test('saving a new plan draft replaces the single buffer without adding a plan',()=>{
 const a=app(),count=a.plans.length;a.newEditor();a.state.editor.name='单份新建草稿';a.act('save-draft');
 assert.equal(a.plans.length,count);assert.equal(a.state.recoveryEditor.name,'单份新建草稿');assert.ok(a.state.recoveryEditor.savedAt);
 a.newEditor();a.state.editor.name='替换草稿';a.act('save-draft');assert.equal(a.plans.length,count);assert.equal(a.state.recoveryEditor.name,'替换草稿');
});
test('edit save and discard preserve an unrelated creation draft',()=>{
 const a=app();a.newEditor();a.state.editor.name='新建输入';a.act('save-draft');const draft=json(a.state.recoveryEditor);
 const p=a.plans.find(p=>p.status==='draft'),name=p.name;a.editPlan(p.id);a.state.editor.name='未保存修改';a.act('detail',{id:p.id});
 assert.equal(a.state.modal.type,'leave-editor');assert.equal(p.name,name);a.act('discard-edit');assert.equal(p.name,name);assert.deepEqual(json(a.state.recoveryEditor),draft);
 a.editPlan(p.id);a.state.editor.name='保存的修改';a.editorSaveEdit();assert.equal(p.name,'保存的修改');assert.deepEqual(json(a.state.recoveryEditor),draft);
});
test('reload never offers an existing-plan edit as a new creation draft',()=>{
 const a=app(),draft=json(a.state.recoveryEditor);a.editPlan(a.plans.find(p=>p.status==='ready').id);a.state.editor.name='不应缓存的编辑';a.persistPrototype();
 const b=app(a.storage.get('wt-publishing-plan-v1'));assert.equal(b.state.editor,null);assert.deepEqual(json(b.state.recoveryEditor),draft);assert.equal(b.state.page,'list');
});
test('new-plan draft stays available when storage fails',()=>{
 const a=app();a.newEditor();a.state.editor.name='保留输入';a.storage.set=()=>{throw Error('disk full')};
 // Inject failure at the storage boundary used by persistPrototype.
 a.evaluate("localStorage.setItem=()=>{throw Error('full')}");a.act('save-draft');assert.equal(a.state.editor.name,'保留输入');assert.equal(a.state.page,'editor');
});

test('restarting plan creation has no inherited name, targets or batches',()=>{
 const a=app();a.blankNewEditor();assert.equal(a.state.editor.name,'');assert.equal(a.state.editor.target,0);assert.equal(a.state.editor.platforms.length,0);assert.equal(a.state.editor.batches.length,0);
});
test('failed plan edit save retains the editor and original record',()=>{
 const a=app(),p=a.plans.find(p=>p.status==='ready'),original=json(p);a.editPlan(p.id);a.state.editor.name='待保存';a.evaluate("localStorage.setItem=()=>{throw Error('full')}");a.editorSaveEdit();
 assert.deepEqual(json(p),original);assert.equal(a.state.editor.name,'待保存');assert.equal(a.state.page,'editor');
});

test('legacy plan drafts leave formal rows and recover without overwriting current input',()=>{
 const a=app();const draft=a.plans.find(p=>p.status==='draft');
 assert.equal(a.evaluate('listView().rows.some(p=>p.statusLabel==="草稿")'),false);
 assert.equal(a.evaluate('listView().statuses.some(p=>p.value==="draft")'),false);
 assert.equal(a.state.recoveryEditor.name,draft.name);
 a.newEditor();a.state.editor.name='当前续编';a.act('save-draft');a.persistPrototype();
 const b=app(a.storage.get('wt-publishing-plan-v1'));
 assert.equal(b.state.recoveryEditor.name,'当前续编');
 assert.equal(b.evaluate('listView().rows.length'),6);
 b.evaluate('state.recoveryEditor=null; promoteLegacyPlan()');
 assert.equal(b.state.recoveryEditor,null,'consumed legacy draft must not resurface');
});
test('ready list execution explains unavailable resources and repeats checks at confirmation',()=>{
 const a=app(),p=a.plans.find(p=>p.status==='ready'),b=p.batches[0];
 let entry=a.evaluate('listView().rows.find(p=>p.id==="'+p.id+'").actions[0]');
 assert.equal(entry.key,'execute');assert.equal(entry.disabled,true);assert.ok(entry.reason.includes('配置'));
 a.act('execute',{id:p.id});assert.notEqual(a.state.modal?.type,'execute-batch');
 b.phase='ready';b.configured=true;b.locked=true;
 entry=a.evaluate('listView().rows.find(p=>p.id==="'+p.id+'").actions[0]');
 assert.equal(entry.disabled,false);
 a.act('execute',{id:p.id});assert.equal(a.state.modal.type,'execute-batch');
 b.configured=false;a.act('do-execute-batch');assert.equal(p.status,'ready');assert.equal(b.phase,'ready');
});


test('drag reordering preserves batch identities, contents and allocation totals',()=>{
 const a=app();a.newEditor();a.evaluate("state.editor.batches.push(b('TEMP-3','第三批','B站',{}));state.editor.editedBatches=false");
 const e=a.state.editor,before=json(e.batches),goals=json(e.goals);
 assert.equal(a.evaluate('reorderBatch(state.editor,0,2)'),true);
 assert.deepEqual(json(e.batches),[before[1],before[2],before[0]]);
 assert.deepEqual(json(e.goals),goals);assert.equal(e.editedBatches,true);
});
test('locked and executed batches anchor their positions and cannot be crossed',()=>{
 const a=app();a.newEditor();a.evaluate("state.editor.batches.push(b('TEMP-3','第三批','B站',{}),b('TEMP-4','第四批','B站',{}));state.editor.batches[1].locked=true;state.editor.editedBatches=false");
 const e=a.state.editor,locked=json(e.batches[1]),before=json(e.batches);
 for(const [from,to] of [[1,0],[0,1],[0,3],[3,0]])assert.equal(a.evaluate(`reorderBatch(state.editor,${from},${to})`),false);
 assert.deepEqual(json(e.batches),before);assert.equal(e.editedBatches,false);
 assert.equal(a.evaluate('reorderBatch(state.editor,2,3)'),true);assert.deepEqual(json(e.batches[1]),locked);
 a.evaluate("state.editor.batches[1].locked=false;state.editor.batches[1].phase='running'");
 assert.equal(a.evaluate('reorderBatch(state.editor,0,3)'),false);
});
test('reordering rejects invalid, same, newly locked and stopped-plan moves without dirtying',()=>{
 const a=app();a.newEditor();a.state.editor.editedBatches=false;const before=json(a.state.editor.batches);
 for(const [from,to] of [[0,0],[-1,0],[0,9],[0,0.5],[9,0]])assert.equal(a.evaluate(`reorderBatch(state.editor,${from},${to})`),false);
 a.state.editor.batches[0].locked=true;assert.equal(a.evaluate('reorderBatch(state.editor,0,1)'),false);
 a.state.editor.batches[0].locked=false;a.state.editor.status='stopping';assert.equal(a.evaluate('reorderBatch(state.editor,0,1)'),false);
 assert.deepEqual(json(a.state.editor.batches),before);assert.equal(a.state.editor.editedBatches,false);
});
test('a reordered edit stays isolated until save and survives persistence and reload',()=>{
 const a=app(),p=a.plans.find(p=>p.status==='ready'),before=json(p.batches),ids=before.map(b=>b.id);a.editPlan(p.id);
 assert.equal(a.evaluate('reorderBatch(state.editor,0,2)'),true);assert.deepEqual(json(p.batches),before);
 a.editorSaveEdit();assert.deepEqual(json(p.batches.map(b=>b.id)),[ids[1],ids[2],ids[0]]);
 const restored=app(a.storage.get('wt-publishing-plan-v1'));assert.deepEqual(json(restored.plans.find(x=>x.id===p.id).batches),json(p.batches));
});

test('plan attention summary names the batches that actually need attention',()=>{
 const a=app(),p=a.plans[0];
 p.batches[0].tasks=[{id:'earlier',type:'verify',status:'pending'}];
 assert.equal(a.currentNote(p).reason,'2 个批次有待处理任务');
 p.batches[1].tasks=[];
 assert.equal(a.currentNote(p).reason,p.batches[0].name+'需要介入');
});
test('the execution notice opens the batch with outstanding tasks rather than an unrelated current batch',()=>{
 const a=app(),p=a.plans[0];
 p.batches[0].tasks=[{id:'earlier',type:'verify',status:'pending'}];p.batches[1].tasks=[];
 const v=a.detailView(p);
 assert.equal(v.notice.title,'有待处理任务 · 1 项待办');
 assert.equal(v.notice.actions[0].batch,p.batches[0].id);
});
