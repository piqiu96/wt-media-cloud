const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs'),vm=require('node:vm');
const ctx={window:{}};
const path=require('node:path').join(__dirname,'../production/production-model.js');
if(fs.existsSync(path))vm.runInNewContext(fs.readFileSync(path,'utf8'),ctx);
const model=()=>{assert.ok(ctx.window.ProductionModel,'内容生产规则模型尚未提供');return ctx.window.ProductionModel;};
const rule=(action,field,value,logic='all')=>({action,logic,conditions:[{field,operator:'gte',value}]});
test('执行记录按实际尝试分组，不补造阶段或重复完成，兼容旧记录且不修改原数据',()=>{
 const task={attempt:2,events:[{text:'任务已创建',time:'2026-11-04 07:58'},{text:'重试原任务，第 2 次执行',time:'2026-11-04 08:00'},{text:'合成完成，第 2 次执行',time:'11-04 08:00'}]};
 const before=JSON.stringify(task),groups=model().executionHistory(task);
 assert.equal(groups.length,2);assert.equal(groups[0].attempt,2);assert.equal(groups[0].current,true);
 assert.equal(groups[0].events.map(x=>x.text).join(','),'重试原任务,合成完成');
 assert.equal(groups[1].events.length,1);assert.equal(groups[1].current,false);assert.equal(JSON.stringify(task),before);
});
test('没有执行日志时保留未知；明确的尝试编号优先，无记录的最新尝试不沿用旧日志',()=>{
 assert.equal(model().executionHistory({attempt:3})[0].events.length,0);
 const groups=model().executionHistory({attempt:3,events:[{text:'合成失败',attempt:2,time:'10:00'}]});
 assert.equal(groups[0].attempt,3);assert.equal(groups[0].events.length,0);assert.equal(groups[1].attempt,2);
});
test('排除规则不受位置影响；缺失值不会被补成0或其他指标',()=>{
 const M=model(),m={likes:3500};
 assert.equal(M.matches([rule('match','likes',3000),rule('filter','likes',3400)],m).selected,false);
 assert.equal(M.matches([rule('match','favorites',0)],m).selected,false);
 assert.equal(M.matches([rule('match','favorites',0)],{favorites:0}).selected,true);
 assert.equal(M.matches([rule('filter','favorites',1),rule('match','likes',3000)],m).selected,false);
});
test('多平台不乘产量；预览跳过旧任务，限额按素材计算且不改变输入',()=>{
 const M=model(),s={id:1,category:'A',executionMode:'scheduled',singleLimit:3,dailyLimit:4,rules:[rule('match','likes',0)],versions:[{qty:2,platforms:['B','C']},{qty:1}]};
 const materials=[1,2,3,4].map(id=>({id,category:'A',likes:1,cloudState:'ready',published:'2026-11-0'+(5-id)}));
 const before=JSON.stringify(materials);
 const p=M.preview(s,materials,[{strategyId:1,materialId:1}],2);
 assert.equal(p.ids.join(','),'2,3');assert.equal(p.tasks,6);assert.equal(p.remaining,2);
 assert.equal(JSON.stringify(materials),before);
 s.executionMode='manual';assert.equal(M.preview(s,materials,[],100).ids.length,3);
});
test('任务只能覆盖模板开放参数；组数和固定结构不允许更改',()=>{
 const M=model(),base=[{name:'A',qty:1,platforms:['B'],guideUrl:'fixed',insertSeconds:0}];
 assert.equal(M.validOverrides(base,[{...base[0],insertSeconds:8}],['insertSeconds']),true);
 assert.equal(M.validOverrides(base,[{...base[0],qty:2}],['insertSeconds']),false);
 assert.equal(M.validOverrides(base,[{...base[0],guideUrl:'changed'}],['insertSeconds']),false);
 assert.equal(M.validOverrides(base,[...base,...base],['insertSeconds']),false);
});
test('批次结束数包括失败和取消，成功数单独统计',()=>{
 const n=model().counts([{status:'success'},{status:'failed'},{status:'cancelled'},{status:'running'}]);
 assert.equal(n.ended,3);assert.equal(n.success,1);assert.equal(n.active,1);assert.equal(n.total,4);
});
test('距发布天数按真实日期计算；非法发布时间不能命中',()=>{
 const M=model(),r=[{action:'match',logic:'all',conditions:[{field:'recentDays',operator:'lte',value:3}]}];
 const now=Date.parse('2026-11-04T12:00:00+08:00');
 assert.equal(M.matches(r,{published:'2026-11-02 12:00'},now).selected,true);
 assert.equal(M.matches(r,{published:'未知'},now).selected,false);
});
test('成片按素材聚合，跨任务批次合并，保留真实来源且不混入其他素材',()=>{
 const M=model(),rows=[{id:1,materialId:101,taskId:11,date:'2026-11-01 10:00'},{id:2,materialId:101,taskId:22,date:'2026-11-04 10:00'},{id:3,materialId:102,taskId:33,date:'2026-11-03 10:00'},{id:4,taskId:44}];
 const before=JSON.stringify(rows),groups=M.groupOutputs(rows);
 assert.equal(groups.length,2);assert.equal(groups[0].materialId,101);
 assert.equal(groups[0].outputs.map(o=>o.id).join(','),'2,1');assert.equal(groups[0].outputs.map(o=>o.taskId).join(','),'22,11');
 assert.equal(groups[1].materialId,102);assert.equal(JSON.stringify(rows),before);
 assert.equal(M.groupOutputs(rows.filter(o=>o.id===1))[0].outputs.length,1);
});

test('选材原因区分指标缺失和未达阈值，保留命中证据',()=>{
 const M=model(),rules=[rule('match','likes',3000)];
 assert.match(M.matches(rules,{likes:3500}).reason,/点赞 3500.*≥ 3000/);
 assert.match(M.matches(rules,{likes:2500}).reason,/点赞 2500 < 3000/);
 assert.match(M.matches(rules,{}).reason,/缺少点赞数据/);
 assert.match(M.matches([rule('filter','favorites',100),...rules],{likes:3500}).reason,/无法核对排除规则.*缺少收藏数据/);
});
test('选材记录保存当时标题和具体跳过原因；团队边界及限额不混淆',()=>{
 const M=model(),s={id:1,team:'A组',category:'A',executionMode:'manual',singleLimit:1,rules:[rule('match','likes',0)],versions:[{qty:1}]};
 const materials=[{id:1,title:'可用',team:'A组',category:'A',likes:1,cloudState:'ready',published:'2026-11-05 12:00'},
 {id:2,title:'失效',category:'A',likes:1,cloudState:'expired',published:'2026-11-04 12:00'},
 {id:3,title:'排除其他团队',team:'B组',category:'A',likes:1,cloudState:'ready'},
 {id:4,title:'失败',category:'A',likes:1,cloudState:'failed',cloudError:'文件获取超时'},
 {id:5,title:'超额',category:'A',likes:1,cloudState:'ready'}];
 const result=M.preview(s,materials,[]);
 assert.equal(result.ids.join(','),'1');assert.equal(result.details.some(x=>x.id===3),false);
 assert.equal(result.details.find(x=>x.id===2).reason,'源视频失效');
 assert.equal(result.details.find(x=>x.id===4).reason,'文件获取超时');
 assert.equal(result.details.find(x=>x.id===5).reason,'达到每次素材上限');
 materials[0].title='后来改名';assert.equal(result.details.find(x=>x.id===1).title,'可用');
 s.executionMode='scheduled';s.dailyLimit=1;assert.equal(M.preview(s,materials,[],1).details.find(x=>x.id===1).reason,'今日素材额度已用完');
});

test('发布情况与文件状态、使用人分离；失败保持占用，取消释放，成功优先',()=>{
 const C={db:{suiteVersion:1,outputs:[],tasks:[],strategies:[]},get:(kind,id)=>C.db[kind].find(x=>x.id===id)};
 const P={db:{suiteVersion:1,tasks:[],assets:[]},save:()=>{},platformOf:t=>t.platform};
 const context={window:{},CP:C,PI:P,PIPublishModel:{},addEventListener:()=>{}};
 vm.runInNewContext(fs.readFileSync(require('node:path').join(__dirname,'../workspace.js'),'utf8'),context);
 const O=context.window.OPS;
 C.db.outputs.push({id:1,platforms:['B站'],assignees:{'B站':'其他运营'},cloudFileExists:false});
 assert.equal(O.publication(1,'B站').text,'未安排');assert.equal(O.availability(1,'B站').ok,false);
 P.db.tasks.push({id:11,assetId:1,platform:'B站',status:'failed'});
 assert.equal(O.publication(1,'B站').text,'已安排');assert.equal(O.publication(1,'B站').task.status,'failed');
 assert.equal(O.availability(1,'B站').ok,false);
 P.db.tasks[0].status='cancelled';assert.equal(O.publication(1,'B站').text,'未安排');
 P.db.tasks[0].status='draft';assert.equal(O.publication(1,'B站').text,'未安排');
 P.db.tasks.push({id:12,assetId:1,platform:'B站',status:'published'});
 assert.equal(O.publication(1,'B站').text,'已发布');assert.equal(O.publication(1,'百度').text,'未安排');
 P.db.tasks=null;assert.equal(O.publication(1,'B站').key,'unknown');
});
