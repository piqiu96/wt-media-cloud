const {test}=require('node:test'),assert=require('node:assert/strict'),vm=require('node:vm'),fs=require('node:fs'),path=require('node:path');
function setup(){
 const C={db:{productionRevision:4,tasks:[],materials:[{id:1,category:'A'}],strategies:[]},prod:{copy:v=>JSON.parse(JSON.stringify(v)),valid:()=>true,editable:{T:['insertSeconds']}},id:(()=>{let n=0;return()=>++n;})(),fullTime:()=> '2026-11-04 12:00',save:()=>{},access:()=>true};
 C.get=(collection,id)=>C.db[collection].find(x=>x.id===Number(id));
 const context={window:{},CP:C,OPS:{actor:'运营A'}};
 vm.runInNewContext(fs.readFileSync(path.join(__dirname,'../production/production-model.js'),'utf8'),context);context.ProductionModel=context.window.ProductionModel;
 vm.runInNewContext(fs.readFileSync(path.join(__dirname,'../production/experience.js'),'utf8'),context);
 return C;
}
test('创建按份数拆任务，同一次提交重放返回原ID，快照与默认值隔离',()=>{
 const C=setup(),strategy={id:1,enabled:true,category:'A',template:'T',name:'S',versions:[{name:'A',qty:2,platforms:['B','C'],insertSeconds:0},{name:'B',qty:1,platforms:['B'],insertSeconds:1}]};
 const args={strategy,ids:[1],vs:strategy.versions,owner:'运营A',requestKey:'once'};
 const first=C.submitTasks(args),second=C.submitTasks(args);
 assert.equal(first.length,3);assert.equal(JSON.stringify(first),JSON.stringify(second));assert.equal(C.db.tasks.length,3);
 strategy.versions[0].insertSeconds=99;assert.equal(C.db.tasks.find(t=>t.name.startsWith('A')).version.insertSeconds,0);
 assert.equal(new Set(C.db.tasks.map(t=>t.batchId)).size,1);
 assert.equal(C.db.tasks.every(t=>t.version.qty===1),true);
 assert.equal(C.db.tasks.filter(t=>t.sourceQuantity===2).map(t=>t.copyIndex).sort().join(','),'1,2');
});
test('新请求允许明确再次制作，固定结构的越权修改被拒绝',()=>{
 const C=setup(),strategy={id:1,enabled:true,category:'A',template:'T',versions:[{name:'A',qty:1,platforms:['B'],insertSeconds:0}]};
 C.submitTasks({strategy,ids:[1],vs:strategy.versions,owner:'运营A',requestKey:'one'});
 C.submitTasks({strategy,ids:[1],vs:strategy.versions,owner:'运营A',requestKey:'two'});
 assert.equal(C.db.tasks.length,2);
 assert.throws(()=>C.submitTasks({strategy,ids:[1],vs:[{...strategy.versions[0],qty:20}],owner:'运营A',requestKey:'bad'}),/固定配置/);
 assert.equal(C.db.tasks.length,2);
});

test('提交前范围重核验失败不产生空批次或任务',()=>{
 const C=setup(),strategy={id:1,enabled:true,category:'B',template:'T',versions:[{name:'A',qty:1,platforms:['B']}]};
 assert.throws(()=>C.submitTasks({strategy,ids:[1],vs:strategy.versions,owner:'运营A',requestKey:'invalid'}),/范围已变化/);
 assert.equal(C.db.tasks.length,0);
});
