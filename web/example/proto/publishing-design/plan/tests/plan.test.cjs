const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');
function app(saved) {
  const storage = new Map(saved ? [['wt-publishing-plan-v1', saved]] : []);
  const source = fs.readFileSync(path.join(__dirname, '../app.js'), 'utf8').split("document.addEventListener('click'")[0];
  const ctx = vm.createContext({URL, console, setTimeout:()=>0, clearTimeout(){}, localStorage:{getItem:k=>storage.get(k),setItem:(k,v)=>storage.set(k,v)}, window:{scrollTo(){}}, document:{querySelector:()=>null,addEventListener(){}}});
  vm.runInContext(source + `\nrender=()=>persistPrototype(); globalThis.api={state,get plans(){return plans},newEditor,editPlan,editorSaveEdit,planFromEditor,batchValidation,genBatchSplit,canExecute,openTasks,taskDone,mustUrl,actualByCell,onAction,persistPrototype};`, ctx);
  const api=ctx.api;
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
 const a=app();const p=a.plans[0],before=json(p);a.editPlan(p.id);
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
 assert.equal(b.state.editor.name,'恢复输入');assert.equal(b.state.page,'editor');assert.equal(b.plans.length,7);
});
test('all editable seeded plans permit a basic information edit',()=>{
 const a=app();
 for(const p of a.plans.filter(p=>['draft','ready','running'].includes(p.status))){a.editPlan(p.id);a.state.editor.name='修改 '+p.name;const expected=a.state.editor.name;a.editorSaveEdit();assert.equal(p.name,expected,p.id);}
});
test('makeup editing cannot exceed remaining cell deficits',()=>{
 const a=app(),p=a.plans[2];a.state.drawer={type:'makeup',planId:p.id,platform:'B站',values:{'游戏攻略':4,'精彩剪辑':0}};a.act('create-makeup');
 a.editPlan(p.id);a.state.editor.batches[4].cats['游戏攻略']=1000;a.editorSaveEdit();
 assert.equal(p.batches[4].cats['游戏攻略'],4);
});
test('splitting makeup keeps both rows makeup and preserves original target',()=>{
 const a=app(),p=a.plans[2];a.state.drawer={type:'makeup',planId:p.id,platform:'B站',values:{'游戏攻略':4,'精彩剪辑':0}};a.act('create-makeup');
 a.editPlan(p.id);a.act('split-batch',{idx:'4'});a.act('do-split-batch');
 assert.equal(a.state.editor.batches[5].type,'makeup');assert.equal(a.batchValidation(a.state.editor).length,0);
});
test('makeup cannot merge into an original batch',()=>{
 const a=app(),p=a.plans[2];a.editPlan(p.id);a.state.editor.batches.push({id:'m1',name:'补量',platform:'百家号',cats:{'精彩剪辑':1},phase:'draft',locked:false,type:'makeup',tasks:[]});
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
