/* 发布计划 V1 高保真交互原型 · 全部示例数据仅用于 UX 评审 */
'use strict';
const $ = s => document.querySelector(s);
const clone = x => JSON.parse(JSON.stringify(x));
const escapeHtml = s => String(s ?? '').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const cn = n => (Number(n)||0).toLocaleString('zh-CN');
const E = escapeHtml;
const $icon = (n,size=17) => {
 const paths={
  'layers':'<rect x="3" y="4" width="18" height="5" rx="1.5"/><rect x="5" y="10" width="14" height="4" rx="1"/><rect x="7" y="15" width="10" height="5" rx="1"/>',
  'grid':'<rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/>',
  'film':'<rect x="3" y="3" width="18" height="18" rx="2"/><path d="M7 3v18M17 3v18M3 8h4M17 8h4M3 16h4M17 16h4"/>',
  'send':'<path d="m22 2-7 20-4-9-9-4Z"/><path d="M22 2 11 13"/>',
  'users':'<circle cx="9" cy="8" r="3"/><path d="M3 20v-2a6 6 0 0 1 12 0v2M17 11a3 3 0 0 0 0-6M21 20v-2a6 6 0 0 0-4-5.6"/>',
  'chart':'<path d="M3 3v18h18M7 15l4-5 4 3 5-7"/>',
  'settings':'<circle cx="12" cy="12" r="3"/><path d="M12 2v3M12 19v3M4.9 4.9 7 7M17 17l2.1 2.1M2 12h3M19 12h3M4.9 19.1 7 17M17 7l2.1-2.1"/>',
  'chevron-right':'<path d="m9 18 6-6-6-6"/>','chevron-left':'<path d="m15 18-6-6 6-6"/>','chevron-down':'<path d="m6 9 6 6 6-6"/>',
  'plus':'<path d="M12 5v14M5 12h14"/>','search':'<circle cx="11" cy="11" r="7"/><path d="m20 20-4-4"/>','refresh':'<path d="M3 12a9 9 0 0 1 16-5l2 2M21 3v6h-6M21 12a9 9 0 0 1-16 5l-2-2M3 21v-6h6"/>',
  'check':'<path d="m5 12 4 4L19 6"/>','check-circle':'<circle cx="12" cy="12" r="9"/><path d="m7 12 3.5 3.5L17 9"/>','alert':'<circle cx="12" cy="12" r="9"/><path d="M12 7v6M12 17h.01"/>',
  'clock':'<circle cx="12" cy="12" r="9"/><path d="M12 7v5l4 2"/>','play':'<circle cx="12" cy="12" r="9"/><path d="m10 8 6 4-6 4Z"/>',
  'file':'<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8Z"/><path d="M14 2v6h6M8 13h8M8 17h8"/>',
  'copy':'<rect x="8" y="8" width="13" height="13" rx="2"/><path d="M16 8V5a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h3"/>',
  'trash':'<path d="M4 7h16M10 3h4M6 7l1 14h10l1-14M10 11v6M14 11v6"/>',
  'edit':'<path d="m16 4 4 4-12 12H4v-4Z"/><path d="m14 6 4 4"/>','lock':'<rect x="5" y="10" width="14" height="11" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3"/>',
  'eye':'<path d="M2 12s4-7 10-7 10 7 10 7-4 7-10 7S2 12 2 12Z"/><circle cx="12" cy="12" r="3"/>',
  'calendar':'<rect x="3" y="5" width="18" height="16" rx="2"/><path d="M7 2v6M17 2v6M3 10h18"/>','arrow-left':'<path d="m12 19-7-7 7-7M5 12h14"/>','arrow-right':'<path d="m12 5 7 7-7 7M19 12H5"/>',
  'close':'<path d="M18 6 6 18M6 6l12 12"/>','more':'<circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/>',
  'flag':'<path d="M5 22V3M5 4c6-3 8 3 15 0v12c-7 3-9-3-15 0"/>','bookmark':'<path d="M5 4a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2v18l-7-4-7 4Z"/>',
  'move-up':'<path d="m7 14 5-5 5 5"/>','move-down':'<path d="m7 10 5 5 5-5"/>','sparkles':'<path d="m12 3 1.8 5.2L19 10l-5.2 1.8L12 17l-1.8-5.2L5 10l5.2-1.8ZM19 18l.7 2.3L22 21l-2.3.7L19 24l-.7-2.3L16 21l2.3-.7Z"/>',
  'info':'<circle cx="12" cy="12" r="9"/><path d="M12 11v6M12 7h.01"/>','download':'<path d="M12 3v12m-4-4 4 4 4-4M4 17v4h16v-4"/>','upload':'<path d="M12 17V5m-4 4 4-4 4 4M4 17v4h16v-4"/>',
  'activity':'<path d="M2 12h5l3-7 4 14 3-7h5"/>','workflow':'<rect x="2" y="3" width="7" height="6" rx="1"/><rect x="15" y="15" width="7" height="6" rx="1"/><path d="M5.5 9v7h9.5M15 16l-3-3m3 3-3 3"/>',
  'bolt':'<path d="m13 2-9 12h7l-1 8 10-13h-7Z"/>','list':'<path d="M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01"/>',
  'filter':'<path d="M4 5h16L14 12v7l-4-2v-5Z"/>','help':'<circle cx="12" cy="12" r="9"/><path d="M9 9a3 3 0 1 1 5.2 2c-1 1-2.2 1.4-2.2 3M12 17h.01"/>',
  'split':'<path d="M12 3v6M5 21v-5l7-7 7 7v5M3 16h4M17 16h4"/>','shield':'<path d="M12 2 3 6v6c0 6 4 9 9 10 5-1 9-4 9-10V6Z"/><path d="m8 12 3 3 5-6"/>'
 };
 return `<svg width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${paths[n]||paths.info}</svg>`;
};
const CATS=['游戏攻略','精彩剪辑','游戏资讯','游戏评测','赛事复盘','新手教学'];
const PLATFORM_DATA=[
 {name:'B站',accounts:8,videos:160,categories:5,capacity:60,cell:{'游戏攻略':35,'精彩剪辑':30,'游戏资讯':18,'游戏评测':10,'赛事复盘':0,'新手教学':12}},
 {name:'百家号',accounts:12,videos:210,categories:5,capacity:90,cell:{'游戏攻略':55,'精彩剪辑':35,'游戏资讯':40,'游戏评测':25,'赛事复盘':0,'新手教学':30}},
 {name:'小红书',accounts:10,videos:130,categories:4,capacity:50,cell:{'游戏攻略':25,'精彩剪辑':30,'游戏资讯':0,'游戏评测':12,'赛事复盘':0,'新手教学':15}}
];
const pd=n=>PLATFORM_DATA.find(p=>p.name===n);
const b=(id,name,platform,cats,phase='draft',success=0,locked=false,extra={})=>({id,name,platform,cats,phase,success,locked,type:'base',tasks:[],...extra});
const goalsA={'B站':{'游戏攻略':30,'精彩剪辑':20},'百家号':{'游戏攻略':25,'精彩剪辑':25}};
const seedPlans=[
 {id:'PL-202611-005',name:'游戏新品推广 · 第2周计划',note:'聚焦新品上线，通过 B站与百家号进行分批投放。',dateStart:'2026-11-10',dateEnd:'2026-11-23',owner:'运营负责人',status:'running',result:'',target:100,platforms:['B站','百家号'],categories:['游戏攻略','精彩剪辑'],goals:clone(goalsA),updated:'2026-11-12 10:36',created:'2026-11-08 09:20',logs:[['11-12 10:36','批次 02 出现 3 项人工待办','系统'],['11-11 14:20','执行批次 02 · 百家号','运营负责人'],['11-10 18:15','批次 01 已结束，成功 28 / 30','系统'],['11-08 09:20','计划确认创建','运营负责人']],batches:[
  b('BT-101','批次 01 · B站首发','B站',{'游戏攻略':20,'精彩剪辑':10},'ended',28,true,{executed:true}),
  b('BT-102','批次 02 · 百家号','百家号',{'游戏攻略':25,'精彩剪辑':15},'running',17,true,{executed:true,tasks:[
   {id:'PT-1023',title:'新英雄上手教学｜入门操作',account:'百家号 · 运营01',type:'manual',status:'pending',description:'页面内容已填写，等待人工检查并提交'},
   {id:'PT-1024',title:'赛季版本改动分析',account:'百家号 · 运营03',type:'verify',status:'pending',description:'平台结果不明确，请核实最终发布状态'},
   {id:'PT-1025',title:'新手常见操作误区',account:'百家号 · 运营05',type:'failed',status:'pending',description:'上传过程中出现网络异常，等待处理'}
  ]}),
  b('BT-103','批次 03 · B站补充','B站',{'游戏攻略':10,'精彩剪辑':10}),
  b('BT-104','批次 04 · 百家号补充','百家号',{'精彩剪辑':10})
 ]},
 {id:'PL-202611-004',name:'版本内容矩阵 · 周度发布',note:'先完成核心平台首批投放，再按顺序推进其他批次。',dateStart:'2026-11-09',dateEnd:'2026-11-22',owner:'运营负责人',status:'running',result:'',target:100,platforms:['B站','百家号'],categories:['游戏攻略','精彩剪辑'],goals:{'B站':{'游戏攻略':30,'精彩剪辑':30},'百家号':{'游戏攻略':25,'精彩剪辑':15}},updated:'2026-11-12 09:10',created:'2026-11-06 11:03',logs:[['11-11 16:40','批次 01 已结束，等待下一批人工确认','系统'],['11-09 09:00','计划开始执行','运营负责人']],batches:[
  b('BT-201','批次 01 · B站','B站',{'游戏攻略':20,'精彩剪辑':10},'ended',28,true,{executed:true}),
  b('BT-202','批次 02 · 百家号','百家号',{'游戏攻略':25,'精彩剪辑':15},'ready',0,true,{configured:true,accountCount:12}),
  b('BT-203','批次 03 · B站','B站',{'游戏攻略':10,'精彩剪辑':20})
 ]},
 {id:'PL-202611-003',name:'多平台周末集中投放',note:'出现目标缺口后优先核实失败结果，再决定补量。',dateStart:'2026-11-01',dateEnd:'2026-11-15',owner:'运营负责人',status:'running',result:'',target:100,platforms:['B站','百家号'],categories:['游戏攻略','精彩剪辑'],goals:clone(goalsA),updated:'2026-11-11 17:58',created:'2026-10-29 08:30',logs:[['11-11 17:58','所有原始批次已结束，待处理缺口 17 条','系统'],['11-10 10:15','批次 04 结束，成功 20 / 25','系统']],batches:[
  b('BT-301','批次 01 · B站攻略','B站',{'游戏攻略':30},'ended',26,true,{executed:true,successCats:{'游戏攻略':26}}),
  b('BT-302','批次 02 · B站剪辑','B站',{'精彩剪辑':20},'ended',16,true,{executed:true,successCats:{'精彩剪辑':16}}),
  b('BT-303','批次 03 · 百家号攻略','百家号',{'游戏攻略':25},'ended',21,true,{executed:true,successCats:{'游戏攻略':21}}),
  b('BT-304','批次 04 · 百家号剪辑','百家号',{'精彩剪辑':25},'ended',20,true,{executed:true,successCats:{'精彩剪辑':20}})
 ]},
 {id:'PL-202611-002',name:'平台内容拓展 · 第二阶段',note:'等待配置首个批次。',dateStart:'2026-11-15',dateEnd:'2026-11-30',owner:'运营负责人',status:'ready',result:'',target:120,platforms:['B站','百家号','小红书'],categories:['游戏攻略','精彩剪辑'],goals:{'B站':{'游戏攻略':20,'精彩剪辑':20},'百家号':{'游戏攻略':20,'精彩剪辑':20},'小红书':{'游戏攻略':20,'精彩剪辑':20}},updated:'2026-11-11 11:25',created:'2026-11-11 11:25',logs:[['11-11 11:25','计划确认创建，生成 3 个发布批次','运营负责人']],batches:[
  b('BT-401','批次 01 · B站','B站',{'游戏攻略':20,'精彩剪辑':20}),
  b('BT-402','批次 02 · 百家号','百家号',{'游戏攻略':20,'精彩剪辑':20}),
  b('BT-403','批次 03 · 小红书','小红书',{'游戏攻略':20,'精彩剪辑':20})
 ]},
 {id:'PL-202611-001',name:'长视频运营提升计划（草稿）',note:'计划资源尚未确认。',dateStart:'2026-11-18',dateEnd:'2026-11-30',owner:'运营负责人',status:'draft',result:'',target:80,platforms:['B站','百家号'],categories:['游戏攻略','精彩剪辑'],goals:{'B站':{'游戏攻略':20,'精彩剪辑':20},'百家号':{'游戏攻略':20,'精彩剪辑':20}},updated:'2026-11-10 18:02',created:'2026-11-10 18:02',logs:[['11-10 18:02','保存计划草稿','运营负责人']],batches:[
  b('BT-501','批次 01 · B站','B站',{'游戏攻略':20,'精彩剪辑':20}),
  b('BT-502','批次 02 · 百家号','百家号',{'游戏攻略':20,'精彩剪辑':20})
 ]},
 {id:'PL-202610-018',name:'S12 赛季增长计划',note:'计划执行完成。',dateStart:'2026-10-20',dateEnd:'2026-11-06',owner:'运营负责人',status:'ended',result:'achieved',target:100,platforms:['B站','百家号'],categories:['游戏攻略','精彩剪辑'],goals:clone(goalsA),updated:'2026-11-06 19:12',created:'2026-10-18 08:10',logs:[['11-06 19:12','目标达成，计划自动结束','系统']],batches:[
  b('BT-601','批次 01 · B站','B站',{'游戏攻略':30,'精彩剪辑':20},'ended',50,true,{executed:true}),
  b('BT-602','批次 02 · 百家号','百家号',{'游戏攻略':25,'精彩剪辑':25},'ended',50,true,{executed:true})
 ]},
 {id:'PL-202610-015',name:'国庆后运营测试计划',note:'需求调整，运营主动取消。',dateStart:'2026-10-09',dateEnd:'2026-10-22',owner:'运营负责人',status:'ended',result:'cancelled',target:60,platforms:['B站'],categories:['游戏攻略','精彩剪辑'],goals:{'B站':{'游戏攻略':30,'精彩剪辑':30}},updated:'2026-10-15 13:10',created:'2026-10-08 09:00',logs:[['10-15 13:10','运营主动取消计划，保留已发布结果','运营负责人']],batches:[
  b('BT-701','批次 01 · B站','B站',{'游戏攻略':30},'ended',24,true,{executed:true}),
  b('BT-702','批次 02 · B站','B站',{'精彩剪辑':30},'cancelled',0,false)
 ]}
];
const STORAGE_KEY='wt-publishing-plan-v1';
let storageIssue='';
function loadPrototype(){
 try {const raw=localStorage.getItem(STORAGE_KEY);if(!raw)return null;const data=JSON.parse(raw);
 const validPlan=p=>p&&typeof p.id==='string'&&typeof p.name==='string'&&Array.isArray(p.platforms)&&p.platforms.every(pd)&&Array.isArray(p.categories)&&p.categories.every(c=>CATS.includes(c))&&p.goals&&Array.isArray(p.logs)&&typeof p.dateStart==='string'&&typeof p.dateEnd==='string'&&Array.isArray(p.batches)&&p.batches.every(b=>b&&pd(b.platform)&&b.cats&&Array.isArray(b.tasks));
 if(data.version!==1||!Array.isArray(data.plans)||!data.plans.every(validPlan)||new Set(data.plans.map(p=>p.id)).size!==data.plans.length)throw Error('invalid');
 if(data.ui?.editor&&!validPlan({...data.ui.editor,id:'editor',logs:[]}))throw Error('invalid editor');return data;
 }catch{storageIssue='无法读取上次原型数据，当前使用演示初始数据；本次不会覆盖原存档。';return null;}
}
const savedPrototype=loadPrototype();
let plans=savedPrototype?.plans||clone(seedPlans);
const state={page:'list',planId:null,editor:null,activeStep:1,statusFilter:'all',search:'',needOnly:false,advanced:false,modal:null,drawer:null,platformExpand:null,taskFilter:'all',reco:'rule',toast:null};
Object.assign(state,{pageNo:1,pageSize:8,filterDraft:{search:'',status:'all',needOnly:false,platform:'',category:''},filters:{search:'',status:'all',needOnly:false,platform:'',category:''},recoveryEditor:null},savedPrototype?.ui||{});
state.modal=null;state.drawer=null;state.toast=null;
if(state.page==='editor'&&!state.editor)state.page='list';
if(state.page==='detail'&&!plans.some(p=>p.id===state.planId))state.page='list';
let identitySequence=Number(savedPrototype?.sequence)||Date.now();
const uniqueId=prefix=>prefix+'-'+(++identitySequence);
function persistPrototype(){
 if(storageIssue.startsWith('无法读取'))return false;
 try{localStorage.setItem(STORAGE_KEY,JSON.stringify({version:1,plans,sequence:identitySequence,ui:{page:state.page,planId:state.planId,editor:state.editor,recoveryEditor:state.recoveryEditor,pageNo:state.pageNo,pageSize:state.pageSize,filters:state.filters,filterDraft:state.filterDraft,advanced:state.advanced,reco:state.reco}}));storageIssue='';return true;}
 catch{storageIssue='本地保存失败，请保留当前页面并释放浏览器存储空间后重试。';return false;}
}
let toastTimer=null;
const sumVals=o=>Object.values(o||{}).reduce((s,n)=>s+(Number(n)||0),0);
const sumMatrix=m=>Object.values(m||{}).reduce((s,o)=>s+sumVals(o),0);
const sumBatch=(b)=>sumVals(b.cats);
const successOf=p=>p.batches.reduce((s,b)=>s+(b.success||0),0);
const targetBatches=p=>p.batches.filter(b=>b.type!=='makeup').reduce((s,b)=>s+sumBatch(b),0);
const endedCount=p=>p.batches.filter(b=>['ended','cancelled'].includes(b.phase)).length;
const openTasks=p=>p.batches.flatMap(b=>(b.tasks||[]).filter(t=>['pending','retrying'].includes(t.status)).map(t=>({...t,batchId:b.id,batchName:b.name})));
const currentBatch=p=>p.batches.find(b=>['running','stopping'].includes(b.phase));
const allEnded=p=>p.batches.length>0 && p.batches.every(b=>['ended','cancelled'].includes(b.phase));
const firstNotEnded=p=>p.batches.find(b=>!['ended','cancelled'].includes(b.phase));
const nameStatus=s=>({draft:'草稿',ready:'待执行',running:'进行中',stopping:'停止中',ended:'已结束'}[s]||s);
const badgeStatus=s=>({draft:'gray',ready:'blue',running:'blue',stopping:'orange',ended:'green'}[s]||'gray');
const resultText=r=>({achieved:'目标达成',unachieved:'未达成结束',cancelled:'已取消'}[r]||'');
const nameBatchStatus=s=>({draft:'待配置',ready:'待执行',running:'执行中',ended:'已结束',cancelled:'已取消',stopping:'停止中'}[s]||s);
const badgeBatchStatus=s=>({draft:'gray',ready:'blue',running:'blue',ended:'green',cancelled:'gray',stopping:'orange'}[s]||'gray');
const displayStatus=(s,typ='')=>`<span class="badge ${typ||badgeStatus(s)}"><span class="dot"></span>${E(nameStatus(s))}</span>`;
const bBadge=b=>`<span class="badge ${badgeBatchStatus(b.phase)}">${E(nameBatchStatus(b.phase))}</span>`;
const fmtTargets=b=>Object.entries(b.cats||{}).filter(([,v])=>v>0).map(([k,v])=>`${k} ${v}`).join(' / ')||'未分配';
const getPlan=id=>plans.find(p=>p.id===id);
const logPlan=(p,msg,who='运营负责人')=>{p.logs.unshift([new Date().toLocaleDateString('zh-CN',{month:'2-digit',day:'2-digit'}).replace('/','-')+' '+new Date().toTimeString().slice(0,5),msg,who]);p.updated=new Date().toLocaleString('sv-SE').slice(0,16)};
const currentNote=p=>{
 if(p.status==='draft')return {txt:'计划待完善',type:'neutral',reason:'继续完善目标与批次拆分'};
 if(p.status==='stopping')return {txt:'停止处理中',type:'alert',reason:'等待执行中任务结果收敛'};
 if(p.status==='ended')return {txt:resultText(p.result)||'已结束',type:p.result==='achieved'?'complete':'neutral',reason:'计划已关闭'};
 const manual=openTasks(p).filter(t=>t.status==='pending');
 if(manual.length)return {txt:`待人工处理 · ${manual.length}项`,type:'alert',reason:`${currentBatch(p)?.name||'当前批次'}需要介入`};
 if(allEnded(p)){
  const gap=Math.max(0,p.target-successOf(p));
  return gap>0?{txt:`目标缺口 ${gap} 条`,type:'alert',reason:'待确认补量或结束'}:{txt:'目标已达成',type:'complete',reason:'已具备结束条件'};
 }
 const next=firstNotEnded(p);
 if(next?.phase==='ready')return {txt:`${next.name} · 待确认执行`,type:'wait',reason:'人工确认后才能执行'};
 if(next?.phase==='draft')return {txt:`${next.name} · 待配置`,type:'wait',reason:'完成批次资源和内容配置'};
 if(next?.phase==='running')return {txt:`${next.name} · 执行中`,type:'active',reason:'暂无人工待办'};
 return {txt:'等待推进',type:'neutral',reason:''};
};
const needsHuman=p=>['alert','wait'].includes(currentNote(p).type)&&p.status!=='stopping';
let lastFocus=null,lastLayer=null,modalFocus=null;
function focusKey(el){if(!el||!el.matches?.('button,input,select,textarea,a,[tabindex]'))return null;return {id:el.id,tag:el.tagName,attrs:Object.fromEntries([...el.attributes].filter(a=>a.name.startsWith('data-')).map(a=>[a.name,a.value]))};}
function findFocus(key){if(!key)return null;if(key.id)return document.getElementById(key.id);return [...document.querySelectorAll(key.tag)].find(el=>Object.entries(key.attrs).every(([k,v])=>el.getAttribute(k)===v));}
function render(){
 persistPrototype();
 const active=document.activeElement,key=focusKey(active),position=active?.selectionStart;
 const layerKey=state.modal?'modal:'+state.modal.type:state.drawer?'drawer:'+state.drawer.type:null;
 const scrolls=[...document.querySelectorAll('.drawer-body,.modal-body,.table-wrap')].map(el=>[el.scrollTop,el.scrollLeft]);
 if(layerKey&&!lastLayer)lastFocus=key;
 if(layerKey?.startsWith('modal:')&&!lastLayer?.startsWith('modal:'))modalFocus=key;
 document.getElementById('app').innerHTML=renderLayout();
 document.querySelectorAll('.btn').forEach(b=>{b.classList.add('ex-button');if(!b.getAttribute('type'))b.type='button';if(b.classList.contains('icon-btn'))b.setAttribute('aria-label',b.getAttribute('aria-label')||b.title||'关闭');});
 document.querySelectorAll('label').forEach((label,i)=>{if(label.htmlFor)return;const control=label.parentElement.querySelector('input:not([type=checkbox]),textarea,select');if(control){control.id||='plan-field-'+i;label.htmlFor=control.id;}});
 const drawer=document.querySelector('dialog.plan-drawer'),modal=document.querySelector('dialog.plan-modal');
 for(const [dialog,label] of [[drawer,'.drawer-title'],[modal,'.modal-head h3']])if(dialog){const heading=dialog.querySelector(label);if(heading){heading.id='title-'+dialog.className;dialog.setAttribute('aria-labelledby',heading.id);}dialog.showModal();dialog.addEventListener('cancel',event=>{event.preventDefault();if(document.querySelector(':popover-open'))return;if(state.modal)state.modal=null;else state.drawer=null;render();});}
 drawer?.classList.toggle('under-modal',!!modal);
 if(state.toast&&(modal||drawer)){const toast=document.querySelector('.toast');if(toast)(modal||drawer).append(toast);}
 document.querySelectorAll('a[data-act]').forEach(el=>{el.tabIndex=0;el.setAttribute('role','button');});
 document.querySelectorAll('.tagcheck').forEach(el=>{el.tabIndex=0;el.setAttribute('role','checkbox');el.setAttribute('aria-checked',String(el.classList.contains('selected')));el.setAttribute('aria-disabled',String(!!el.querySelector('input:disabled')));});
 document.querySelectorAll('.editor-step').forEach(el=>{el.tabIndex=0;el.setAttribute('role','button');});
 window.WTControls?.enhance(document.getElementById('app'));
 document.querySelectorAll('.drawer-body,.modal-body,.table-wrap').forEach((el,i)=>{if(scrolls[i]){el.scrollTop=scrolls[i][0];el.scrollLeft=scrolls[i][1];}});
 const focus=findFocus(key);
 if(layerKey===lastLayer&&focus){focus.focus({preventScroll:true});if(typeof position==='number'&&focus.setSelectionRange&&['text','search','textarea'].includes(focus.type))focus.setSelectionRange(position,position);}
 else if(layerKey?.startsWith('drawer:')&&lastLayer?.startsWith('modal:'))findFocus(modalFocus)?.focus({preventScroll:true});
 else if(!layerKey&&lastLayer)findFocus(lastFocus)?.focus({preventScroll:true});
 lastLayer=layerKey;
}

function renderLayout(){return `<div class="app"><aside class="side">
 <a class="brand-bar" href="/example/proto/content-operations/"><img class="brand-image" src="/example/proto/content-operations/assets/logo.png" alt=""/><div><div class="brand-title">起飞</div><div class="brand-sub">CONTENT OPERATIONS</div></div></a>
 <nav class="nav-scroll" aria-label="业务导航"><div class="nav-heading">内容生产</div>
 <a class="nav-item" href="/example/proto/content-operations/#materials">${$icon('film')}素材库</a>
 <a class="nav-item" href="/example/proto/content-operations/#tasks">${$icon('layers')}合成任务</a>
 <a class="nav-item" href="/example/proto/content-operations/#outputs">${$icon('film')}成片管理</a>
 <div class="nav-heading">发布管理</div>
 <a class="nav-item" href="/example/?view=publishing#tasks">${$icon('send')}发布任务</a>
 <a class="nav-item" href="/example/?view=publishing#batches">${$icon('layers')}发布批次</a>
 <button class="nav-item active" data-act="list" aria-current="page">${$icon('calendar')}发布计划</button>
 <a class="nav-item" href="/example/?view=publishing#images">${$icon('film')}图片库</a>
 <a class="nav-item" href="/example/?view=publishing#templates">${$icon('grid')}封面模板库</a>
 <div class="nav-heading">作品管理</div><a class="nav-item" href="/example/proto/content-operations/#works">${$icon('bookmark')}作品列表</a>
 </nav><div class="nav-foot"><div class="avatar">运</div><div><div class="nav-user">运营负责人</div><div class="nav-role">本地交互原型</div></div></div>
 </aside><div class="shell"><header class="topbar"><div class="crumb">内容运营 / <button class="btn-link" data-act="list">发布计划</button>${state.page!=='list'?` / <span class="now">${state.page==='editor'?(state.editor?.mode==='create'?'新增计划':'编辑计划'):'计划详情'}</span>`:''}</div><div class="top-actions"><span>V1 · 本地演示数据</span><a href="../../../common/index.html" target="_blank">common 规范</a></div></header>
 <main class="main">${storageIssue?`<div class="hint orange" role="alert">${E(storageIssue)}</div>`:''}${state.page==='list'?renderList():state.page==='editor'?renderEditor():renderDetail()}</main></div></div>${renderDrawer()}${renderModal()}${state.toast?`<div role="status" class="toast ${state.toast.type==='error'?'error':''}">${$icon(state.toast.type==='error'?'alert':'check-circle',18)}${E(state.toast.text)}</div>`:''}`;}

function renderList(){const f=state.filters,d=state.filterDraft;
 const filtered=plans.filter(p=>(f.status==='all'||p.status===f.status)&&(!f.search||[p.name,p.id,p.owner].some(s=>s.toLowerCase().includes(f.search.toLowerCase())))&&(!f.needOnly||needsHuman(p))&&(!f.platform||p.platforms.includes(f.platform))&&(!f.category||p.categories.includes(f.category)));
 const pages=Math.max(1,Math.ceil(filtered.length/state.pageSize));state.pageNo=Math.min(pages,Math.max(1,state.pageNo));
 const appliedCount=[f.platform,f.category].filter(Boolean).length;
 return `<div class="page-head"><div><h1 class="page-h1">发布计划</h1><div class="page-desc">规划发布目标，组织批次，跟踪目标达成情况</div></div><button class="btn primary" data-act="new">新增发布计划</button></div>
 ${state.recoveryEditor?`<div class="hint m-bottom16">已保留未完成的计划输入。<button class="btn-link" data-act="new">继续填写</button><button class="btn-link muted" data-act="discard-recovery">清除暂存</button></div>`:''}
 <section class="surface ex-panel ex-list-pattern"><form class="ex-filters ex-filters-stacked" id="plan-filters">
 <div class="ex-field"><label for="plan-search">计划名称 / ID</label><input id="plan-search" class="input" data-filter="search" value="${E(d.search)}" placeholder="输入名称或 ID"/></div>
 <div class="ex-field"><label for="plan-status">计划状态</label><select id="plan-status" class="select" data-filter="status">${[['all','全部状态'],['draft','草稿'],['ready','待执行'],['running','进行中'],['stopping','停止中'],['ended','已结束']].map(([v,l])=>`<option value="${v}" ${d.status===v?'selected':''}>${l}</option>`).join('')}</select></div>
 <div class="ex-field"><label>待办范围</label><label class="tickfilter"><input type="checkbox" data-filter="needOnly" ${d.needOnly?'checked':''}/>只看需要我处理</label></div>
 <div class="ex-filter-actions"><button class="btn primary" type="submit">查询</button><button class="btn" type="button" data-act="reset-filters">重置</button></div>
 <div class="plan-advanced-toggle"><button type="button" class="btn-link" data-act="advanced" aria-expanded="${state.advanced}">${state.advanced?'收起':'展开'}高级筛选 ${$icon('chevron-down',12)}</button>${appliedCount?`<span class="subtext">已生效 ${appliedCount} 项</span>`:''}</div>
 <div class="plan-advanced-fields" ${!state.advanced?'hidden':''}><div class="ex-field"><label for="plan-platform">发布平台</label><select id="plan-platform" class="select" data-filter="platform"><option value="">全部平台</option>${PLATFORM_DATA.map(p=>`<option ${d.platform===p.name?'selected':''}>${p.name}</option>`).join('')}</select></div><div class="ex-field"><label for="plan-category">发布类目</label><select id="plan-category" class="select" data-filter="category"><option value="">全部类目</option>${CATS.map(c=>`<option ${d.category===c?'selected':''}>${c}</option>`).join('')}</select></div></div>
 </form><div class="table-wrap"><table class="table ex-table"><thead><tr><th>ID</th><th>计划名称</th><th>状态</th><th>当前进展</th><th>发布进度</th><th>批次进度</th><th>执行周期</th><th>操作</th></tr></thead><tbody>
 ${filtered.slice((state.pageNo-1)*state.pageSize,state.pageNo*state.pageSize).map(p=>{const note=currentNote(p),s=successOf(p),ratio=Math.round(s/Math.max(1,p.target)*100);return `<tr><td class="id-cell">${E(p.id)}</td><td><a class="name" data-act="detail" data-id="${p.id}">${E(p.name)}</a></td><td>${displayStatus(p.status)}${p.status==='ended'?`<div class="subline">${E(resultText(p.result))}</div>`:''}</td><td><button class="progress-action ${note.type}" data-act="focus-progress" data-id="${p.id}">${E(note.txt)}</button><div class="subtext">${E(note.reason)}</div></td><td><div class="progress"><div class="progress-fill ${ratio>=100?'good':''}" style="width:${Math.min(ratio,100)}%"></div></div><span>${s} / ${p.target}</span><div class="subtext">达成率 ${ratio}%</div></td><td>${endedCount(p)} / ${p.batches.length} 批</td><td class="date-cell">${E(p.dateStart)}<br>${E(p.dateEnd)}</td><td><div class="inline-actions"><button data-act="detail" data-id="${p.id}">详情</button>${['draft','ready','running'].includes(p.status)?`<button data-act="edit" data-id="${p.id}">编辑</button>`:''}${['ready','running'].includes(p.status)?`<button class="red" data-act="cancel" data-id="${p.id}">取消</button>`:''}${p.status==='draft'?`<button class="red" data-act="delete" data-id="${p.id}">删除</button>`:''}${p.status==='ended'?`<button data-act="copy" data-id="${p.id}">复制</button>`:''}</div></td></tr>`}).join('')||`<tr><td colspan="8"><div class="empty">暂无符合条件的计划<button class="btn-link" data-act="reset-filters">重置筛选</button></div></td></tr>`}
 </tbody></table></div><div class="footerline"><span>共 ${filtered.length} 条</span><div class="pager"><label>每页 <select data-page-size class="select small" aria-label="每页条数">${[8,16,32].map(n=>`<option ${n===state.pageSize?'selected':''}>${n}</option>`).join('')}</select> 条</label><button class="btn icon-btn" aria-label="上一页" data-act="page" data-dir="-1" ${state.pageNo===1?'disabled':''}>${$icon('chevron-left',16)}</button><span>${state.pageNo} / ${pages}</span><button class="btn icon-btn" aria-label="下一页" data-act="page" data-dir="1" ${state.pageNo===pages?'disabled':''}>${$icon('chevron-right',16)}</button></div></div></section>`;
}

function newEditor(fromPlan=null){const fresh=fromPlan?clone(fromPlan):null;
 state.page='editor';state.editor={mode:'create',sourceId:fresh?.id||'',name:fresh?fresh.name+'（副本）':'新赛季内容发布 · 11月周计划',note:fresh?fresh.note:'围绕重点游戏类目，按照平台容量进行串行分批发布。',dateStart:'2026-11-16',dateEnd:'2026-11-30',owner:'运营负责人',status:'draft',target:fresh?.target||100,platforms:fresh?clone(fresh.platforms):['B站','百家号'],categories:fresh?clone(fresh.categories):['游戏攻略','精彩剪辑'],goals:fresh?clone(fresh.goals):clone(goalsA),batches:fresh?clone(fresh.batches).filter(b=>b.type!=='makeup').map((item,i)=>b('TEMP-'+(i+1),item.name,item.platform,clone(item.cats))): [
 b('TEMP-1','批次 01 · B站首批','B站',{'游戏攻略':15,'精彩剪辑':10}),b('TEMP-2','批次 02 · 百家号首批','百家号',{'游戏攻略':15,'精彩剪辑':10}),b('TEMP-3','批次 03 · B站第二批','B站',{'游戏攻略':15,'精彩剪辑':10}),b('TEMP-4','批次 04 · 百家号第二批','百家号',{'游戏攻略':10,'精彩剪辑':15})
 ],editedBatches:false};state.activeStep=1;state.platformExpand=null;state.modal=null;state.drawer=null;render();window.scrollTo(0,0);}
function editPlan(id){const p=getPlan(id);if(!p||['ended','stopping'].includes(p.status))return;
 state.page='editor';state.editor={...clone(p),mode:'edit',sourceId:id,editedBatches:false};state.activeStep=1;state.platformExpand=null;state.modal=null;state.drawer=null;render();window.scrollTo(0,0);}
const isGoalLocked=e=>e.mode==='edit'&&(e.status==='running'||e.batches.some(b=>b.locked));
const batchEditable=(e,b)=>e.status!=='ended'&&e.status!=='stopping'&&!b.locked&&!['running','ended','cancelled','stopping'].includes(b.phase);
const numBatches=e=>e.batches.filter(b=>b.type!=='makeup');
const perCellActual=(e,p,c)=>numBatches(e).reduce((s,b)=>s+(b.platform===p?(b.cats[c]||0):0),0);
const batchValidation=e=>{
 const issues=[];
 const quantities=[e.target,...Object.values(e.goals||{}).flatMap(Object.values),...e.batches.flatMap(b=>Object.values(b.cats))];
 if(quantities.some(n=>!Number.isSafeInteger(n)||n<0)||!Number.isSafeInteger(e.target))issues.push('目标与批次数量必须为非负整数');
 if(!e.owner.trim())issues.push('请填写计划负责人');
 if(e.batches.some(b=>!b.name.trim()))issues.push('请填写批次名称');
 if(new Set(e.batches.map(b=>b.id)).size!==e.batches.length)issues.push('批次 ID 重复，请重新推荐');
 if(Object.entries(e.goals).some(([pl,cats])=>!e.platforms.includes(pl)||Object.entries(cats).some(([c,n])=>n>0&&!e.categories.includes(c))))issues.push('目标矩阵含未选择的平台或类目');
 if(!e.name.trim())issues.push('请填写计划名称');
 if(!e.dateStart||!e.dateEnd||e.dateStart>e.dateEnd)issues.push('请设置有效的执行周期');
 if(!(e.target>0))issues.push('计划目标必须大于 0');
 if(!e.platforms.length||!e.categories.length)issues.push('至少选择一个平台和一个类目');
 if(sumMatrix(e.goals)!==e.target)issues.push(`目标矩阵已分配 ${sumMatrix(e.goals)} 条，与计划目标 ${e.target} 条不一致`);
 for(const p of e.platforms){let n=sumVals(e.goals[p]);if(n>pd(p).capacity)issues.push(`${p} 目标 ${n} 条，超过本周期平台可发布上限 ${pd(p).capacity}`);
   for(const c of e.categories){const k=e.goals[p]?.[c]||0;if(k>0&&pd(p).cell[c]===0)issues.push(`${p}不支持类目「${c}」`);if(k>pd(p).cell[c])issues.push(`${p} · ${c} 目标 ${k} 条，超过预计容量 ${pd(p).cell[c]}`);
   const n2=perCellActual(e,p,c);if(n2!==k)issues.push(`${p} · ${c}：批次合计 ${n2} 条 / 矩阵目标 ${k} 条`);}}
 if(e.batches.some(b=>!e.platforms.includes(b.platform)))issues.push('存在不在目标平台范围内的批次');
 if(targetBatches(e)!==e.target)issues.push(`原始批次总目标 ${targetBatches(e)} 条，与计划目标 ${e.target} 条不一致`);
 if(e.batches.some(b=>Object.entries(b.cats).some(([c,n])=>Number(n)>0&&!e.categories.includes(c))))issues.push('部分批次仍包含未选中的类目，请重新分配');
 if(e.batches.some(b=>sumBatch(b)===0))issues.push('存在未分配任何发布数量的批次');
 if(e.batches.length===0)issues.push('请至少配置一个批次');
 for(const pl of e.platforms)for(const c of e.categories){
  const remaining=Math.max(0,(e.goals[pl]?.[c]||0)-actualByCell(e,pl,c));
  const reserved=e.batches.filter(b=>b.type==='makeup'&&b.platform===pl&&!['ended','cancelled'].includes(b.phase)).reduce((n,b)=>n+Math.max(0,(b.cats[c]||0)-actualByCell({...e,batches:[b]},pl,c)),0);
  if(reserved>remaining)issues.push(`${pl} · ${c}：补量分配 ${reserved} 条超过剩余缺口 ${remaining} 条`);
 }
 return [...new Set(issues)];
};
function renderEditor(){const e=state.editor;if(!e)return '';
 const lockGoal=isGoalLocked(e);const ratio=sumMatrix(e.goals);const issues=batchValidation(e);
 const section=(num,title,desc,body,aux='')=>`<section class="surface ex-panel section" id="step-0${num}"><div class="section-hd"><div class="section-hd-left"><span class="sect-num">0${num}</span><div><div class="sect-title">${title}</div><div class="sect-desc">${desc}</div></div></div>${aux}</div><div class="section-body">${body}</div></section>`;
 return `<button class="crumb-back" data-act="${e.mode==='create'?'list':'detail'}" ${e.mode==='edit'?`data-id="${E(e.sourceId)}"`:''}>${$icon('arrow-left',15)}返回${e.mode==='create'?'计划列表':'计划详情'}</button>
 <div class="page-head"><div><h1 class="page-h1">${e.mode==='create'?'新增发布计划':'编辑发布计划'}</h1><div class="page-desc">先盘点资源，再设定平台 × 类目目标，最后完成批次拆解。${e.mode==='create'?'当前展示一组可编辑的演示数据。':'已执行或锁定的批次不可修改。'}</div></div><div class="header-right">${e.mode==='edit'?displayStatus(e.status):`<span class="badge gray">待确认创建</span>`}</div></div>
 <div class="editor-nav">${[['基础信息','名称与周期'],['资源盘点','资源容量测算'],['目标设定','平台 × 类目矩阵'],['批次拆解','按目标拆分批次']].map(([a,b],i)=>`<div class="editor-step ${state.activeStep===i+1?'active':''}" data-act="jump" data-step="${i+1}"><span class="num">0${i+1}</span><span><strong>${a}</strong><small>${b}</small></span></div>`).join('')}</div>
 ${section(1,'基础信息','定义计划的名称、执行范围与业务说明。',`<div class="form-grid">
 <div class="form-field span2"><label class="field-label">计划名称<span class="req">*</span></label><input class="input" data-editor="name" maxlength="60" value="${E(e.name)}" placeholder="例如：新赛季内容矩阵 · 周度发布"/></div>
 <div class="form-field"><label class="field-label">计划负责人</label><input class="input" data-editor="owner" value="${E(e.owner)}"/></div>
 <div class="form-field"><label class="field-label">开始日期<span class="req">*</span></label><input class="input" type="date" data-editor="dateStart" value="${E(e.dateStart)}" ${isGoalLocked(e)?'disabled title="执行周期已锁定"':''}/></div>
 <div class="form-field"><label class="field-label">结束日期<span class="req">*</span></label><input class="input" type="date" data-editor="dateEnd" value="${E(e.dateEnd)}" ${isGoalLocked(e)?'disabled title="执行周期已锁定"':''}/></div>
 <div class="form-field"><label class="field-label">周期说明</label><div class="hint" style="padding:8px 11px;align-items:center">${$icon('info',14)} 账号容量按该周期进行测算</div></div>
 <div class="form-field wide"><label class="field-label">计划备注</label><textarea class="input" rows="2" data-editor="note" placeholder="记录发布目标、活动背景、投放重点等">${E(e.note)}</textarea></div></div>`)}
 ${section(2,'资源盘点','先了解可用资源，再决定选择哪些平台、类目以及目标数量。',renderInventory(e),`<button class="btn sm" data-act="refresh-inventory">${$icon('refresh',14)}重新盘点</button>`)}
 ${section(3,'目标设定','先选择平台与类目，再通过目标矩阵调整各组合的发布数量。',renderGoalEditor(e,lockGoal),lockGoal?`<span class="badge orange">${$icon('lock',12)} 目标配置已锁定</span>`:'')}
 ${section(4,'批次拆解','规则 / AI 推荐生成初始批次，运营可以调整未锁定批次。',renderBatchEditor(e))}
 <div class="editor-footer"><div class="foot-note">${issues.length?`${$icon('alert',14)} 尚有 ${issues.length} 项配置需要确认`:`${$icon('check-circle',14)} 目标与批次分配已匹配`}<div style="margin-top:3px">计划创建后，仅在批次内匹配具体账号与成片</div></div><div class="buttons"><button class="btn" data-act="${e.mode==='edit'?'detail':'list'}" ${e.mode==='edit'?`data-id="${E(e.sourceId)}"`:''}>返回</button>${e.mode==='create'?`<button class="btn secondary" data-act="save-draft">保存草稿</button><button class="btn primary" data-act="confirm-plan">确认创建</button>`:e.status==='draft'?`<button class="btn secondary" data-act="save-edit">保存草稿</button><button class="btn primary" data-act="confirm-draft">确认计划</button>`:`<button class="btn primary" data-act="save-edit">${$icon('check',15)}保存修改</button>`}</div></div>`;
}
function renderInventory(e){return `<div class="stat-grid">
 ${[['可用成片',PLATFORM_DATA.reduce((n,p)=>n+p.videos,0),'条','演示资源汇总'],['可用账号',PLATFORM_DATA.reduce((n,p)=>n+p.accounts,0),'个','演示周期可用'],['覆盖平台',PLATFORM_DATA.length,'个','支持多平台选择'],['覆盖类目',CATS.length,'类','按账号与成片归类'],['预计发布容量',PLATFORM_DATA.reduce((n,p)=>n+p.capacity,0),'条','演示周期额度合计']].map(([a,n,u,d])=>`<div class="stat-card"><div class="stat-label">${a}</div><div class="stat-number">${n}<span class="unit">${u}</span></div><div class="stat-assist">${d}</div></div>`).join('')}</div>
 <div class="section-split"><div class="subheading">平台资源分布 <span style="font-size:12px;font-weight:500;color:var(--wt-color-text-assist)">点击“查看类目”展开资源细节</span></div><span class="meta-info">盘点时间 ${E(state.inventoryAt||new Date().toLocaleString('sv-SE').slice(0,16))} · 演示资源</span></div>
 <div style="border:1px solid var(--line);border-radius:9px;overflow:hidden"><table class="table resource-table"><thead><tr><th>平台</th><th>可用成片</th><th>可用账号</th><th>覆盖类目</th><th>预计可发布量</th><th>操作</th></tr></thead><tbody>${PLATFORM_DATA.map(p=>`<tr><td><b>${E(p.name)}</b></td><td>${p.videos} 条</td><td>${p.accounts} 个</td><td>${p.categories} 类</td><td><b>${p.capacity}</b> 条 <span class="badge-inline">周期容量</span></td><td><button class="btn-link" data-act="toggle-platform" data-platform="${E(p.name)}">${state.platformExpand===p.name?'收起类目':'查看类目'} ${$icon(state.platformExpand===p.name?'chevron-down':'chevron-right',12)}</button></td></tr>${state.platformExpand===p.name?`<tr><td colspan="6" style="padding:0 14px"><div class="resource-details"><div style="font-size:12px;font-weight:750;margin-bottom:10px">${p.name} · 平台类目容量（账号额度共享，不能直接相加）</div><div style="display:grid;grid-template-columns:repeat(3,1fr);gap:10px">${CATS.filter(c=>p.cell[c]>0).map(c=>`<div style="padding:10px 12px;background:var(--wt-color-text-inverse);border:1px solid var(--wt-color-border);border-radius:7px"><div class="subtext">${c}</div><div style="font-size:14px;font-weight:760;margin-top:5px">预计可发 ${p.cell[c]} 条</div></div>`).join('')}</div></div></td></tr>`:''}`).join('')}</tbody></table></div>
 <div class="hint m-top16">${$icon('info',17)}<div>当前为演示资源，未连接真实资源服务。资源盘点仅用于决策与容量预估，不在计划阶段锁定账号或成片。同一账号支持多个类目时，额度全局共享；具体资源映射在批次配置时确定，锁定后不能改变映射。</div></div>`;}
function renderGoalEditor(e,locked){const matrixSum=sumMatrix(e.goals);
 const selectPlatforms=`<div class="form-field"><label class="field-label">发布平台（多选）<span class="req">*</span></label><div class="tagcheck-group">${PLATFORM_DATA.map(p=>`<label class="tagcheck ${e.platforms.includes(p.name)?'selected':''}" data-act="toggle-choice" data-kind="platform" data-value="${p.name}" ${locked?'style="opacity:.65;cursor:not-allowed"':''}><span class="check-square">✓</span>${p.name}<input type="checkbox" ${e.platforms.includes(p.name)?'checked':''} ${locked?'disabled':''}/></label>`).join('')}</div></div>`;
 const selectCats=`<div class="form-field"><label class="field-label">发布类目（多选）<span class="req">*</span></label><div class="tagcheck-group">${CATS.map(c=>`<label class="tagcheck ${e.categories.includes(c)?'selected':''}" data-act="toggle-choice" data-kind="category" data-value="${c}" ${locked?'style="opacity:.65;cursor:not-allowed"':''}><span class="check-square">✓</span>${c}<input type="checkbox" ${e.categories.includes(c)?'checked':''} ${locked?'disabled':''}/></label>`).join('')}</div></div>`;
 const sumForP=p=>sumVals(e.goals[p]);
 return `${locked?`<div class="plan-locked m-bottom16">${$icon('lock',14)} 计划已发生资源锁定或执行。平台、类目与总目标保持不变；仍可调整尚未锁定批次的拆分和顺序，但不能破坏既定目标。</div>`:''}
 <div style="display:grid;grid-template-columns:1fr;gap:18px">${selectPlatforms}${selectCats}
 <div class="form-row"><div class="form-field" style="width:210px"><label class="field-label">计划发布总目标<span class="req">*</span></label><input class="input" type="number" min="1" data-editor="target" value="${e.target}" ${locked?'disabled':''}/></div>
 <div class="form-field"><label class="field-label">目标矩阵</label><button class="btn secondary" data-act="generate-matrix" ${locked?'disabled':''}>${$icon('grid',14)}生成目标分配表</button></div><div class="field-help" style="align-self:flex-end;max-width:420px">只设定一个计划总目标，平台和类目的目标由下方矩阵自动汇总。编辑任意单元格时，其他单元格数量不会自动被修改。</div></div></div>
 <div class="section-split" style="margin:26px 0 12px"><div class="subheading">平台 × 类目目标分配矩阵</div><span class="meta-info">单元格仅展示对应组合的目标数量</span></div>
 ${e.platforms.length&&e.categories.length?`<div class="table-wrap" style="border:1px solid var(--line);border-radius:9px"><table class="table matrix"><thead><tr><th style="min-width:120px">平台 / 类目</th>${e.categories.map(c=>`<th style="min-width:120px">${E(c)}</th>`).join('')}<th>平台合计</th><th>平台容量</th></tr></thead><tbody>${e.platforms.map(p=>`<tr><td><b>${E(p)}</b></td>${e.categories.map(c=>{let cap=pd(p)?.cell[c]||0;return `<td>${cap===0?`<span class="subtext" title="当前平台不支持此类目">—</span>`:locked?`<span class="read-only-cell">${e.goals[p]?.[c]||0} ${$icon('lock',11)}</span>`:`<input class="input" type="number" min="0" data-matrix="1" data-platform="${E(p)}" data-category="${E(c)}" value="${e.goals[p]?.[c]||0}" title="预计上限 ${cap} 条"/>`}<div class="subline" style="margin-top:3px">${cap?'可发≤'+cap:'不可发布'}</div></td>`}).join('')}<td><b>${sumForP(p)}</b></td><td><span class="badge ${sumForP(p)>pd(p).capacity?'red':'gray'}">≤ ${pd(p).capacity}</span></td></tr>`).join('')}<tr class="matrix-total"><td>类目合计</td>${e.categories.map(c=>`<td>${e.platforms.reduce((s,p)=>s+(e.goals[p]?.[c]||0),0)}</td>`).join('')}<td><b>${matrixSum}</b></td><td>目标 ${e.target}</td></tr></tbody></table>
 <div class="matrix-foot"><div class="${matrixSum===e.target?'valid':'invalid'}">${$icon(matrixSum===e.target?'check-circle':'alert',15)} ${matrixSum===e.target?'目标分配平衡':matrixSum<e.target?`仍有 ${e.target-matrixSum} 条未分配`:`超出总目标 ${matrixSum-e.target} 条`}</div><span class="subtext">平台目标和类目目标由矩阵自动汇总</span></div></div>`:`<div class="empty" style="border:1px dashed var(--line)">请先选择至少一个平台和一个类目</div>`}
 <div class="hint m-top16">${$icon('info',17)}<div>资源容量校验包含：平台周期额度、类目兼容性与全局账号额度。03 的目标矩阵作为计划唯一目标依据；04 批次拆分只能分解目标，不能重新定义目标。</div></div>`;
}
function renderBatchEditor(e){const edited=e.editedBatches;const issues=batchValidation(e);const locked=e.batches.filter(b=>b.locked).length;
 const headers=e.categories.map(c=>`<th style="min-width:85px">${E(c)}</th>`).join('');
 return `<div class="reco-panel"><div class="subheading">推荐拆分方式</div><div class="tabpair"><button class="${state.reco==='rule'?'active':''}" data-act="change-reco" data-reco="rule">${$icon('grid',13)} 规则推荐</button><button class="${state.reco==='ai'?'active':''}" data-act="change-reco" data-reco="ai">${$icon('sparkles',13)} AI 推荐</button></div><span class="meta-info">${state.reco==='ai'?'AI 根据运营策略生成建议（原型模拟，不调用模型）':'按照平台额度、目标规模与类目优先级计算'}</span><span class="spacer"></span><button class="btn secondary sm" data-act="recommend" ${locked?'disabled title="已有资源锁定，不可整体重新推荐"':''}>${$icon('refresh',13)}重新推荐</button></div>
 ${locked?`<div class="plan-locked m-bottom16">${$icon('lock',14)} 当前有 ${locked} 个批次已锁定资源。为避免破坏账号、成片映射，已锁定行不可调整；未锁定批次允许按剩余目标进行拆分、合并、数量调整与排序。</div>`:''}
 <div class="section-split"><div class="subheading">批次规划表 <span class="meta-info">共 ${e.batches.length} 批 · 严格串行</span></div><button class="btn sm" data-act="add-batch">${$icon('plus',13)}新增批次</button></div>
 <div class="batches-table table-wrap" style="border:1px solid var(--line);border-radius:9px"><table class="table ex-table"><thead><tr><th style="min-width:85px">顺序</th><th style="min-width:174px">批次名称</th><th style="min-width:120px">平台</th>${headers}<th>批次合计</th><th>资源状态</th><th style="min-width:160px">操作</th></tr></thead><tbody>${e.batches.map((b,i)=>{const can=batchEditable(e,b);const prev=e.batches[i-1];const samePrev=prev&&!prev.locked&&can&&prev.platform===b.platform&&prev.type===b.type;
  return `<tr><td><div class="batch-order"><span>${String(i+1).padStart(2,'0')}</span><button title="上移" data-act="move-batch" data-idx="${i}" data-dir="-1" ${!can||i===0||!batchEditable(e,e.batches[i-1])?'disabled':''}>${$icon('move-up',12)}</button><button title="下移" data-act="move-batch" data-idx="${i}" data-dir="1" ${!can||i===e.batches.length-1||!batchEditable(e,e.batches[i+1])?'disabled':''}>${$icon('move-down',12)}</button></div></td>
   <td>${can?`<input class="input batch-name-input" value="${E(b.name)}" data-batch-name="${i}"/>`:`<div style="font-weight:680">${E(b.name)}</div>`}</td>
   <td>${can?`<select class="select small batch-platform" data-batch-platform="${i}">${e.platforms.map(p=>`<option value="${p}" ${p===b.platform?'selected':''}>${p}</option>`).join('')}</select>`:`<b>${E(b.platform)}</b>`}</td>
   ${e.categories.map(c=>`<td>${(pd(b.platform)?.cell[c]||0)===0?`<span class="subtext">—</span>`:can?`<input type="number" class="input batch-count" data-batch-cell="1" data-idx="${i}" data-category="${E(c)}" min="0" value="${b.cats[c]||0}"/>`:`<b>${b.cats[c]||0}</b>`}</td>`).join('')}
   <td><b>${sumBatch(b)}</b> 条${b.type==='makeup'?'<div class="subtext">补量</div>':''}</td><td>${b.locked?`<span class="badge purple">${$icon('lock',11)}已锁定</span>`:`<span class="badge gray">未锁定</span>`}</td>
   <td><div class="inline-actions">${can?`<button data-act="split-batch" data-idx="${i}">拆分</button>${samePrev?`<button data-act="merge-batch" data-idx="${i}">合并上批</button>`:''}<button class="red" data-act="remove-batch" data-idx="${i}">删除</button>`:`<span class="subtext">${$icon('lock',12)} 不可调整</span>`}</div></td></tr>`}).join('')}</tbody></table>
 <div class="batch-summary"><div class="${issues.length?'invalid':'valid'}">${$icon(issues.length?'alert':'check-circle',15)} ${issues.length?`${issues.length} 项配置未通过校验`:'所有目标分配校验通过'}</div><span class="subtext">原始批次合计 ${targetBatches(e)} / 目标 ${e.target} 条</span></div></div>
 ${issues.length?`<div class="hint ${issues.length?'orange':''} m-top16">${$icon('alert',16)}<div><b>待解决的问题（最多显示 4 项）</b><div style="margin-top:4px">${issues.slice(0,4).map(x=>`• ${E(x)}`).join('<br>')}${issues.length>4?`<br>• 另有 ${issues.length-4} 项……`:''}</div></div></div>`:`<div class="hint green m-top16">${$icon('check-circle',16)}<div>批次目标与矩阵一致。确认计划时仅创建批次规划；批次内具体账号、成片、内容配置将在执行前完成。</div></div>`}`;
}
function allocateGoals(e){if(!e.platforms.length||!e.categories.length||e.target<=0)return;
 const keys=[];for(const p of e.platforms)for(const c of e.categories)if(pd(p).cell[c]>0)keys.push([p,c]);if(!keys.length)return;
 e.goals={};for(const p of e.platforms)e.goals[p]={};
 if(e.target===100&&e.platforms.length===2&&e.platforms.includes('B站')&&e.platforms.includes('百家号')&&e.categories.length===2&&e.categories.includes('游戏攻略')&&e.categories.includes('精彩剪辑')){e.goals=clone(goalsA);return;}
 let remaining=e.target;const capacities={};for(const p of e.platforms)capacities[p]=pd(p).capacity;
 let j=0,avoid=0;while(remaining>0&&avoid<e.target*keys.length+10){const [p,c]=keys[j%keys.length],n=e.goals[p][c]||0;if(n<pd(p).cell[c]&&sumVals(e.goals[p])<capacities[p]){e.goals[p][c]=n+1;remaining--;avoid=0}else avoid++;j++;if(avoid>keys.length*2)break;}
}
function recommendationIssues(e){
 const issues=[];
 if(!e.platforms.length||!e.categories.length)issues.push('请先选择平台和类目');
 if(!Number.isSafeInteger(e.target)||e.target<1||sumMatrix(e.goals)!==e.target)issues.push('请先完成有效的目标矩阵分配');
 for(const pl of e.platforms){const resource=pd(pl);if(!resource){issues.push('存在不支持的平台');continue;}
  if(sumVals(e.goals[pl])>resource.capacity)issues.push(`${pl}目标超过平台周期容量`);
  for(const [c,n] of Object.entries(e.goals[pl]||{}))if(!Number.isSafeInteger(n)||n<0||n>(resource.cell[c]||0)||!e.categories.includes(c))issues.push(`${pl} · ${c}数量超出有效资源容量`);
 }
 return [...new Set(issues)];
}
function genBatchSplit(e,ai=false){if(recommendationIssues(e).length)return [];let packs=[];const all={};for(const p of e.platforms){const values=e.goals[p]||{};const total=sumVals(values);if(total<=0)continue;const max=ai?30:25;const times=Math.ceil(total/max);let remain=clone(values);
 for(let t=0;t<times;t++){const size=Math.min(max,sumVals(remain));const piece={};let left=size;
 // 按类目轮询切分，避免单个批次独占某一类目
 while(left>0){let changed=false;for(const c of e.categories){if(remain[c]>0&&left>0){piece[c]=(piece[c]||0)+1;remain[c]-=1;left--;changed=true;}}if(!changed)break;}
 (all[p]??=[]).push(b('TEMP-'+(packs.length+1),'',p,piece));}}
 const longest=Math.max(0,...Object.values(all).map(a=>a.length));for(let i=0;i<longest;i++)for(const p of e.platforms)if(all[p]?.[i])packs.push(all[p][i]);
 return packs.map((item,i)=>({...item,id:'TEMP-'+(i+1),name:`批次 ${String(i+1).padStart(2,'0')} · ${item.platform}`}));}
function detailPlan(id){const p=getPlan(id);if(!p)return;state.page='detail';state.planId=id;state.editor=null;state.modal=null;state.drawer=null;render();window.scrollTo(0,0);}
function actualByCell(p,pl,cat){return p.batches.reduce((s,b)=>{
 if(b.platform!==pl||b.success<=0)return s;
 if(b.successCats)return s+(b.successCats[cat]||0);
 const num=b.cats[cat]||0;const total=sumBatch(b);if(!total)return s;
 const cats=Object.keys(b.cats).filter(c=>b.cats[c]>0);let remaining=b.success;
 const vals={};for(let i=0;i<cats.length;i++){let k=cats[i];let n=i===cats.length-1?remaining:Math.min(b.cats[k],Math.floor(b.success*b.cats[k]/total));remaining-=n;vals[k]=n;}
 return s+(vals[cat]||0);
 },0)}
function renderDetail(){const p=getPlan(state.planId);if(!p)return `<div>计划不存在 <button data-act="list">返回列表</button></div>`;
 const note=currentNote(p),actual=successOf(p),rat=Math.round(actual/Math.max(1,p.target)*100),gap=Math.max(0,p.target-actual),sumTargets=sumMatrix(p.goals);
 const current=currentBatch(p),pending=openTasks(p).filter(t=>t.status==='pending'),next=firstNotEnded(p);
 let banner='';
 if(p.status==='running'){
  if(pending.length)banner=`<div class="attention">${$icon('alert',22)}<div style="flex:1"><div class="att-title">当前批次需要人工处理 · ${pending.length} 项待办</div><div class="att-text">${current?.name||'当前批次'}存在发布确认、结果核实或失败任务。处理动作在对应批次工作区完成，计划保持「进行中」。</div></div><div class="att-actions"><button class="btn sm" data-act="view-batch" data-id="${p.id}" data-batch="${current?.id||''}">查看批次</button><button class="btn primary sm" data-act="workbench" data-id="${p.id}" data-batch="${current?.id||''}">进入处理工作区 ${$icon('arrow-right',14)}</button></div></div>`;
  else if(allEnded(p)&&gap>0)banner=`<div class="attention">${$icon('alert',22)}<div style="flex:1"><div class="att-title">全部批次已结束，计划目标尚差 ${gap} 条</div><div class="att-text">目标 ${p.target} 条，实际发布成功 ${actual} 条。请核实最终结果后，决定补量还是人工确认结束；原计划目标不变。</div></div><div class="att-actions"><button class="btn" data-act="end-plan" data-id="${p.id}">确认结束</button><button class="btn primary" data-act="makeup" data-id="${p.id}">创建补量批次</button></div></div>`;
  else if(next?.phase==='ready')banner=`<div class="attention blue">${$icon('clock',22)}<div style="flex:1"><div class="att-title">${E(next.name)} · 待人工确认执行</div><div class="att-text">前序批次已结束，当前批次资源与内容配置完毕。请检查目标和资源后确认执行；不会自动启动。</div></div><div class="att-actions"><button class="btn sm" data-act="view-batch" data-id="${p.id}" data-batch="${next.id}">查看批次</button><button class="btn primary sm" data-act="execute" data-id="${p.id}" data-batch="${next.id}">${$icon('play',14)}确认执行</button></div></div>`;
  else if(next?.phase==='draft')banner=`<div class="attention blue">${$icon('info',22)}<div style="flex:1"><div class="att-title">${E(next.name)} · 待配置</div><div class="att-text">当前批次尚未完成账号、成片与发布内容配置。请先前往对应批次完成资源匹配，配置完成后再人工确认执行。</div></div><div class="att-actions"><button class="btn primary sm" data-act="view-batch" data-id="${p.id}" data-batch="${next.id}">查看批次并配置</button></div></div>`;
 }
 if(p.status==='ready')banner=`<div class="attention blue">${$icon('clock',22)}<div style="flex:1"><div class="att-title">计划已确认，等待首批次执行</div><div class="att-text">批次按列表顺序串行推进，执行前需要人工确认。当前无发布成功记录。</div></div><div class="att-actions"><button class="btn primary sm" data-act="view-batch" data-id="${p.id}" data-batch="${p.batches[0]?.id||''}">查看首批次</button></div></div>`;
 if(p.status==='stopping')banner=`<div class="attention">${$icon('clock',22)}<div style="flex:1"><div class="att-title">取消指令已生效，正在等待当前任务收敛</div><div class="att-text">未启动批次已被阻止执行。请核实可能已经在平台提交的任务；待结果收敛后自动结束计划。</div></div>${openTasks(p).length?`<div class="att-actions"><button class="btn primary sm" data-act="workbench" data-id="${p.id}" data-batch="${currentBatch(p)?.id||''}">处理待核实任务</button></div>`:''}</div>`;
 if(p.status==='ended')banner=`<div class="attention green">${$icon('check-circle',22)}<div style="flex:1"><div class="att-title">计划已结束 · ${E(resultText(p.result))}</div><div class="att-text">发布结果和批次执行历史均已保留。已结束计划只可查看或复制，不可继续执行或修改。</div></div></div>`;
 return `<button class="crumb-back" data-act="list">${$icon('arrow-left',15)}返回发布计划列表</button>
 <div class="page-head"><div><div class="detail-meta" style="margin-bottom:7px"><span class="subtext">${E(p.id)}</span>${displayStatus(p.status)}${p.status==='ended'?`<span class="badge ${p.result==='achieved'?'green':'gray'}">${E(resultText(p.result))}</span>`:''}</div><h1 class="page-h1">${E(p.name)}</h1><div class="page-desc">${E(p.note||'暂无备注')}</div></div>
 <div class="header-right">${['draft','ready','running'].includes(p.status)?`<button class="btn" data-act="edit" data-id="${p.id}">${$icon('edit',15)}编辑计划</button>`:''}${['ready','running'].includes(p.status)?`<button class="btn danger" data-act="cancel" data-id="${p.id}">${$icon('close',15)}取消计划</button>`:''}${p.status==='ended'?`<button class="btn primary" data-act="copy" data-id="${p.id}">${$icon('copy',15)}复制计划</button>`:''}</div></div>
 ${banner}
 <section class="surface ex-panel section"><div class="section-hd"><div class="section-hd-left"><span class="sect-num">01</span><div><div class="sect-title">计划信息</div><div class="sect-desc">计划基础属性及目标范围</div></div></div></div><div class="section-body"><div class="info-grid">
 ${[['计划编号',p.id],['计划负责人',p.owner],['执行周期',p.dateStart+' ～ '+p.dateEnd],['创建时间',p.created],['发布平台',p.platforms.join('、')],['发布类目',p.categories.join('、')],['规划批次',p.batches.filter(b=>b.type!=='makeup').length+' 个原始批次'],['最近更新',p.updated]].map(([k,v])=>`<div class="info-item"><div class="key">${E(k)}</div><div class="value">${E(v)}</div></div>`).join('')}
 </div></div></section>
 <section class="surface ex-panel section"><div class="section-hd"><div class="section-hd-left"><span class="sect-num">02</span><div><div class="sect-title">目标完成情况</div><div class="sect-desc">仅最终确认「已发布成功」的任务计入达成量</div></div></div><span class="badge white">总体达成率 ${rat}%</span></div><div class="section-body">
 <div class="detail-grid4" style="margin-bottom:18px">${[['计划目标',p.target,'条','平台 × 类目矩阵的目标数量'],['发布成功',actual,'条','只统计已确认成功的结果'],['目标缺口',gap,'条','待核实结果不计入已发布'],['批次进度',endedCount(p)+'/'+p.batches.length,'批','已结束批次数 / 全部批次']].map(([k,v,u,d])=>`<div class="detail-stat"><div class="label">${k}</div><div class="num">${v} <span style="font-size:12px;font-weight:500;color:var(--wt-color-text-assist)">${u}</span></div><div class="minor">${d}</div></div>`).join('')}</div>
 <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:8px"><b style="font-size:12px">总体完成进度</b><span class="meta-info">${actual} / ${p.target} 条</span></div><div class="dual-progress"><div class="done" style="width:${Math.min(100,rat)}%"></div></div>
 <div class="section-split" style="margin-top:22px"><div class="subheading">平台 × 类目完成矩阵</div><span class="meta-info">平台目标与类目目标由原始计划矩阵计算</span></div>
 <div class="table-wrap" style="border:1px solid var(--line);border-radius:9px"><table class="table detail-table"><thead><tr><th>平台</th><th>类目</th><th>目标数</th><th>已发布成功</th><th>目标缺口</th><th style="min-width:200px">完成率</th></tr></thead><tbody>
 ${p.platforms.flatMap(pl=>p.categories.filter(c=>(p.goals[pl]?.[c]||0)>0).map(c=>{const target=p.goals[pl][c]||0;const actual=actualByCell(p,pl,c);const percent=Math.round(actual/Math.max(1,target)*100);return `<tr><td><b>${E(pl)}</b></td><td>${E(c)}</td><td>${target}</td><td style="font-weight:750">${actual}</td><td>${Math.max(0,target-actual)}</td><td><div class="progress" style="width:105px"><div class="progress-fill ${percent>=100?'good':''}" style="width:${Math.min(percent,100)}%"></div></div> ${percent}%</td></tr>`})).join('')}
 <tr class="matrix-total"><td colspan="2">合计</td><td>${sumTargets}</td><td>${actual}</td><td>${gap}</td><td>${rat}%</td></tr></tbody></table></div>
 ${p.batches.some(b=>b.type==='makeup')?`<div class="hint m-top16">${$icon('info',16)}补量批次是额外发布尝试，独立于原始计划目标分配；原计划目标始终保持 ${p.target} 条。</div>`:''}</div></section>
 <section class="surface ex-panel section" id="batch-area"><div class="section-hd"><div class="section-hd-left"><span class="sect-num">03</span><div><div class="sect-title">批次推进</div><div class="sect-desc">按顺序串行执行，下一批必须等待前一批结束并人工确认执行</div></div></div><span class="badge white">${endedCount(p)} / ${p.batches.length} 已结束</span></div>
 <div class="table-wrap"><table class="table detail-table"><thead><tr><th style="min-width:65px">顺序</th><th style="min-width:180px">批次名称</th><th>平台 / 类目</th><th>目标数</th><th>资源配置</th><th>执行状态</th><th>结果</th><th>当前待办</th><th style="min-width:135px">操作</th></tr></thead><tbody>
 ${p.batches.map((b,i)=>{let count=(b.tasks||[]).filter(t=>['pending','retrying'].includes(t.status)).length;const executable=canExecute(p,b);return `<tr><td style="color:var(--wt-color-text-assist);font-weight:700">${String(i+1).padStart(2,'0')}</td><td><div style="font-weight:750">${E(b.name)}</div><div class="subline">${E(b.id)}</div>${b.type==='makeup'?'<div class="subline" style="color:var(--wt-color-text-secondary)">补量批次</div>':''}</td><td><div>${E(b.platform)}</div><div class="subtext">${E(fmtTargets(b))}</div></td><td><b>${sumBatch(b)}</b> 条</td>
 <td>${b.locked?`<span class="badge purple">已锁定</span>`:'<span class="badge gray">未配置</span>'}</td><td>${bBadge(b)}</td><td>${['ended','running','stopping'].includes(b.phase)?`<b>${b.success}</b><span class="subtext"> / ${sumBatch(b)}</span>`:'<span class="subtext">—</span>'}</td>
 <td>${count?`<span class="badge orange">待处理 ${count}</span>`:b.phase==='ready'&&executable?'<span class="badge blue">待确认执行</span>':b.phase==='draft'&&executable?'<span class="subtext">待配置</span>':b.phase==='draft'?'<span class="subtext">等待前序批次</span>':'<span class="subtext">—</span>'}</td>
 <td><div class="inline-actions"><button data-act="view-batch" data-id="${p.id}" data-batch="${b.id}">详情</button>${count?`<button data-act="workbench" data-id="${p.id}" data-batch="${b.id}">去处理</button>`:''}${b.phase==='ready'&&executable?`<button data-act="execute" data-id="${p.id}" data-batch="${b.id}">执行</button>`:''}${b.phase==='draft'&&executable?`<button data-act="view-batch" data-id="${p.id}" data-batch="${b.id}">去配置</button>`:''}</div></td></tr>`}).join('')}</tbody></table></div>
 <div class="batch-summary"><span class="subtext">批次内部账号与成片映射遵循批次资源锁定规则；计划详情只负责推进，不重复实现资源配置。</span>${p.status==='running'&&allEnded(p)&&gap>0?`<div class="inline-actions"><button data-act="end-plan" data-id="${p.id}">确认结束</button><button data-act="makeup" data-id="${p.id}">创建补量批次</button></div>`:''}</div></section>
 <section class="surface ex-panel section"><div class="section-hd"><div class="section-hd-left"><span class="sect-num">04</span><div><div class="sect-title">执行记录</div><div class="sect-desc">记录计划确认、人工介入、批次推进及目标结束结果</div></div></div></div><div class="section-body"><div class="detail-timeline">${(p.logs||[]).map(([date,msg,user])=>`<div class="timeline-row"><div class="timeline-point"></div><div class="timeline-content"><div class="t-title">${E(msg)}</div><div class="t-desc">${E(date)} · ${E(user)}</div></div></div>`).join('')}</div></div></section>`;
}
function canExecute(p,b){if(!p||!b||openTasks(p).length)return false;if(p.status!=='running'&&p.status!=='ready')return false;const i=p.batches.indexOf(b);if(i<0)return false;if(p.batches.some(x=>['running','stopping'].includes(x.phase)))return false;return i===0||p.batches.slice(0,i).every(x=>['ended','cancelled'].includes(x.phase));}
function renderDrawer(){if(!state.drawer)return '';const d=state.drawer,p=getPlan(d.planId);if(!p)return '';const b=p.batches.find(x=>x.id===d.batchId);
 if(d.type==='workbench'&&b){const tasks=b.tasks||[],filtered=tasks.filter(t=>['pending','retrying'].includes(t.status)&&(state.taskFilter==='all'||t.type===state.taskFilter));return `<dialog class="plan-drawer"><div class="drawer"><div class="drawer-hd"><div><div class="drawer-title">批次异常工作区</div><div class="subtext m-top12">${E(b.name)} · ${E(b.platform)} · ${tasks.filter(t=>['pending','retrying'].includes(t.status)).length} 项待办</div></div><button class="btn icon-btn" data-act="close-drawer">${$icon('close',17)}</button></div>
 <div class="drawer-body"><div class="hint orange m-bottom16">${$icon('alert',17)}<div>计划继续保持原主状态。人工待办的结果以单条任务为准，处理后自动更新批次和计划进度。不要直接将整批标记为成功。</div></div>
 <div class="filter-seg" style="margin-bottom:18px">${[['all','全部待办'],['manual','待人工提交'],['verify','待核实'],['failed','发布失败']].map(([k,l])=>`<button class="seg-btn ${state.taskFilter===k?'active':''}" data-act="task-filter" data-value="${k}">${l}</button>`).join('')}</div>
 ${filtered.length?filtered.map(t=>`<div class="task-card"><div style="max-width:62%"><div class="task-title">${E(t.id)} · ${E(t.title)}</div><div class="task-desc">${E(t.account)}<br>${E(t.description)}</div><span class="badge ${t.type==='failed'?'red':'orange'}" style="margin-top:8px">${t.status==='retrying'?'重试中，等待结果':t.type==='manual'?'等待人工提交':t.type==='verify'?'发布结果待核实':'发布失败'}</span></div>
 <div class="task-actions"><button class="btn sm ${t.type==='failed'?'':'primary'}" ${t.status==='retrying'?'disabled':''} data-act="task-action" data-id="${p.id}" data-batch="${b.id}" data-task="${t.id}">${t.status==='retrying'?'重试中':t.type==='manual'?'去确认':t.type==='verify'?'核实结果':'去处理'}</button></div></div>`).join(''):`<div class="empty" style="padding:30px">${$icon('check-circle',28)}<p>当前筛选下没有待处理任务</p></div>`}
 ${demoFeedback(p,b)}<div class="fineprint">本工作区演示与既有发布任务的交互衔接；真实「核实结果」「重试」沿用已定稿的发布任务操作规则。</div></div><div class="drawer-foot"><button class="btn" data-act="close-drawer">关闭</button><button class="btn secondary" data-act="view-batch" data-id="${p.id}" data-batch="${b.id}">查看批次详情</button></div></div></dialog>`;}
 if(d.type==='batch'&&b){const idx=p.batches.indexOf(b),can=canExecute(p,b),pending=(b.tasks||[]).filter(x=>['pending','retrying'].includes(x.status)).length;return `<dialog class="plan-drawer"><div class="drawer"><div class="drawer-hd"><div><div class="drawer-title">${E(b.name)}</div><div class="subtext m-top12">${E(b.id)} · ${E(b.platform)} · 顺序 ${String(idx+1).padStart(2,'0')}</div></div><button class="btn icon-btn" data-act="close-drawer">${$icon('close',17)}</button></div>
 <div class="drawer-body"><div class="detail-meta m-bottom16">${bBadge(b)}${b.locked?'<span class="badge purple">资源已锁定</span>':'<span class="badge gray">资源未锁定</span>'}${b.type==='makeup'?'<span class="badge purple">补量批次</span>':''}</div>
 <div class="modal-kv"><div><div class="label">批次平台</div><div class="value">${E(b.platform)}</div></div><div><div class="label">目标数量</div><div class="value">${sumBatch(b)} 条</div></div><div><div class="label">发布成功</div><div class="value">${b.success} 条</div></div><div><div class="label">执行顺序</div><div class="value">第 ${idx+1} 批</div></div></div>
 <div class="section-split" style="margin-top:24px"><div class="subheading">类目目标</div></div><div style="border:1px solid var(--line);border-radius:8px">${Object.entries(b.cats).filter(([,n])=>n>0).map(([c,n])=>`<div style="padding:13px 16px;border-bottom:1px solid var(--line);display:flex;justify-content:space-between"><span>${E(c)}</span><b>${n} 条</b></div>`).join('')}</div>
 <div class="section-split" style="margin-top:25px"><div class="subheading">资源配置和执行约束</div></div>
 ${b.locked?`<div class="hint green">${$icon('lock',17)}<div>该批次资源已锁定，账号与成片映射不可重新调整。${b.phase==='ready'?'内容配置已完成，等待人工执行。':'执行历史与发布结果保持不可变。'}</div></div>`:`<div class="hint">${$icon('info',17)}<div>该批次尚未完成资源匹配。正式系统应跳转到既有「发布批次」配置页：资源配置 → 内容配置，锁定后不允许重新映射。</div></div>`}
 ${pending?`<div class="hint orange m-top16">${$icon('alert',15)}${pending} 个任务需要运营介入处理</div>`:''}
 ${demoFeedback(p,b)}<a class="btn-link" href="/example/?view=publishing#batches">进入发布批次模块</a><div class="fineprint" style="margin-top:20px">原型演示说明：这里仅展示计划到批次的衔接，未重新制作完整的批次配置系统。下方的「模拟配置完成」只用于测试串行推进。</div>
 </div><div class="drawer-foot"><button class="btn" data-act="close-drawer">关闭</button>${pending?`<button class="btn primary" data-act="workbench" data-id="${p.id}" data-batch="${b.id}">去处理 ${pending} 项</button>`:''}${b.phase==='draft'&&can?`<button class="btn secondary" data-act="demo-config" data-id="${p.id}" data-batch="${b.id}">原型演示：配置完成</button>`:''}${b.phase==='ready'&&can?`<button class="btn primary" data-act="execute" data-id="${p.id}" data-batch="${b.id}">${$icon('play',15)}确认执行</button>`:''}</div></div></dialog>`;}
 if(d.type==='makeup'){const gaps=[];for(const pl of p.platforms)for(const c of p.categories){const n=(p.goals[pl]?.[c]||0)-actualByCell(p,pl,c);if(n>0)gaps.push({pl,c,n})}
 const availablePlatforms=[...new Set(gaps.map(x=>x.pl))];const selected=d.platform||availablePlatforms[0];d.platform=selected;const cells=gaps.filter(x=>x.pl===selected);
 return `<dialog class="plan-drawer"><div class="drawer"><div class="drawer-hd"><div><div class="drawer-title">创建补量批次</div><div class="subtext m-top12">计划目标固定 · 仅针对未达成的平台 × 类目目标新增尝试</div></div><button class="btn icon-btn" data-act="close-drawer">${$icon('close',17)}</button></div>
 <div class="drawer-body"><div class="hint orange m-bottom16">${$icon('info',16)}<div>原计划目标仍为 ${p.target} 条，已发布 ${successOf(p)} 条。补量属于额外发布尝试，不覆盖原始批次规划或已发布记录。</div></div>
 <div class="form-field m-bottom16"><label class="field-label">补量平台（单选，每批仅一个平台）</label><select class="select" data-makeup="platform">${availablePlatforms.map(n=>`<option value="${n}" ${n===selected?'selected':''}>${n}</option>`).join('')}</select></div>
 <div class="form-field m-bottom16"><label class="field-label">批次名称</label><input class="input" data-makeup="name" value="${E(d.name||`补量批次 · ${selected}`)}"/></div>
 <div class="subheading m-bottom16">目标缺口明细</div><div style="border:1px solid var(--line);border-radius:9px;overflow:hidden"><table class="table ex-table"><thead><tr><th>类目</th><th>目标缺口</th><th>本次补量数量</th></tr></thead><tbody>${cells.map(x=>`<tr><td>${E(x.c)}</td><td>${x.n} 条</td><td><input class="input" style="width:96px" type="number" min="0" max="${x.n}" data-makeup-cell="${E(x.c)}" value="${d.values?.[x.c]??x.n}"/></td></tr>`).join('')}</tbody></table></div>
 <div class="fineprint m-top16">补量提交后先生成新的待配置批次，仍需完成资源配置、内容配置，并在前序批次结束后人工确认执行。</div></div><div class="drawer-foot"><button class="btn" data-act="close-drawer">返回</button><button class="btn primary" data-act="create-makeup">确认新增补量批次</button></div></div></dialog>`;}
 return '';
}
function demoFeedback(p,b){if(!['running','stopping'].includes(b.phase))return '';return `<details class="demo-controls"><summary>演示反馈控制</summary><p>仅用于本地流程评审，真实执行器尚未接入。“停止中”是否保留需结合执行器同步停止能力确认。</p>${(b.tasks||[]).filter(t=>t.status==='retrying').map(t=>`<button class="btn" data-act="demo-retry-result" data-id="${p.id}" data-batch="${b.id}" data-task="${t.id}">模拟重试返回 · ${E(t.id)}</button>`).join('')}<button class="btn" data-act="demo-settle" data-id="${p.id}" data-batch="${b.id}" ${openTasks({...p,batches:[b]}).length?'disabled':''}>模拟执行结果已收敛</button><p>未提供作品证据的剩余数量不会计入成功，后续批次不会自动执行。</p></details>`;}
function renderModal(){const m=state.modal;if(!m)return '';const p=m.planId?getPlan(m.planId):null;const b=p&&m.batchId?p.batches.find(x=>x.id===m.batchId):null;
 let title='',content='',footer='',wide=false;
 const dangerBtn=(txt,act)=>`<button class="btn danger solid" data-act="${act}">${txt}</button>`;
 const okBtn=(txt,act)=>`<button class="btn primary" data-act="${act}">${txt}</button>`;
 const cancelBtn=`<button class="btn" data-act="close-modal">返回</button>`;
 if(m.type==='validation'){
 title='配置校验未通过';content=`<div class="hint orange m-bottom16">${$icon('alert',17)}请先解决以下问题，再执行确认操作。</div><ul style="margin:0;padding-left:20px;line-height:2;color:var(--wt-color-text-secondary);font-size:12px">${m.issues.map(s=>`<li>${E(s)}</li>`).join('')}</ul>`;footer=okBtn('返回修改','close-modal');
 }else if(m.type==='confirm-plan'){
 const e=state.editor;title=state.editor?.mode==='edit'?'确认发布计划':'确认创建发布计划';content=`<p class="modal-text" style="margin-top:0">确认后将在本地原型中保存计划及其批次规划记录。此时仅创建目标和批次，不锁定账号或成片，也不会自动执行发布。</p><div class="modal-kv"><div><div class="label">计划名称</div><div class="value">${E(e.name)}</div></div><div><div class="label">计划目标</div><div class="value">${e.target} 条</div></div><div><div class="label">平台 / 类目</div><div class="value">${e.platforms.length} 个 / ${e.categories.length} 类</div></div><div><div class="label">规划批次</div><div class="value">${e.batches.length} 批</div></div></div><div class="hint green m-top16">${$icon('check-circle',16)}目标矩阵、资源容量与全部批次合计校验通过。</div>`;footer=cancelBtn+okBtn('确认创建',state.editor?.mode==='edit'?'do-confirm-draft':'do-confirm-plan');
 }else if(m.type==='cancel-plan'){
 title='取消发布计划';const running=p.batches.filter(x=>x.phase==='running').length;const other=p.batches.filter(x=>['draft','ready'].includes(x.phase)).length;
 content=`<p class="modal-text" style="margin-top:0">将取消「${E(p.name)}」。系统将阻止新的批次启动，停止当前批次后续任务派发；已经完成的发布结果与执行记录保留。</p><div class="modal-kv"><div><div class="label">当前执行中批次</div><div class="value">${running} 批</div></div><div><div class="label">未执行批次</div><div class="value">${other} 批</div></div><div><div class="label">已成功发布</div><div class="value">${successOf(p)} 条（保留）</div></div><div><div class="label">未完成任务</div><div class="value">${openTasks(p).length} 个待人工处理</div></div></div><div class="hint orange m-top16">${$icon('alert',17)}若当前任务仍在上传或提交，须等待最终结果收敛后再进入已结束；否则可以一次性同步取消完成。</div><div class="form-field m-top16"><label class="field-label">取消原因<span class="req">*</span></label><textarea rows="3" class="input" data-modal-field="reason" placeholder="请填写取消计划的原因">${E(m.reason||'')}</textarea></div>`;
 footer=cancelBtn+dangerBtn('确认取消计划','do-cancel-plan');
 }else if(m.type==='delete-plan'){
 title='删除计划草稿';content=`<p class="modal-text">确认删除草稿「${E(p.name)}」？删除后不可恢复。已确认或已执行计划不提供删除操作。</p>`;footer=cancelBtn+dangerBtn('确认删除','do-delete-plan');
 }else if(m.type==='execute-batch'){
 title='确认执行发布批次';const prev=p.batches[p.batches.indexOf(b)-1];content=`<p class="modal-text" style="margin-top:0">即将启动「${E(b.name)}」。执行后该批次内容配置不可再修改，原资源映射保持锁定。</p><div class="modal-kv"><div><div class="label">发布平台</div><div class="value">${E(b.platform)}</div></div><div><div class="label">发布目标</div><div class="value">${sumBatch(b)} 条</div></div><div><div class="label">匹配账号</div><div class="value">${b.accountCount||pd(b.platform).accounts} 个</div></div><div><div class="label">发布任务</div><div class="value">${sumBatch(b)} 个</div></div></div><div class="subheading m-top16 m-bottom16">执行前校验</div><ul class="modal-steps">${[
 '前序批次已结束'+(prev?`（${prev.success}/${sumBatch(prev)} 条）`:''),
 '当前无其他批次执行中',
 '演示资源和内容配置完整',
 '账号、成片及环境：演示数据通过，未连接真实执行器'
 ].map(s=>`<li>${$icon('check-circle',16)} ${E(s)}</li>`).join('')}</ul><div class="hint m-top16">${$icon('info',16)}此确认属于批次执行动作，不会创建新的计划状态；计划首次执行时自动进入「进行中」。</div>`;
 footer=cancelBtn+okBtn('确认执行','do-execute-batch');wide=true;
 }else if(m.type==='split-batch'){
 const editor=state.editor,bt=editor.batches[m.index];title='拆分发布批次';const sum=sumBatch(bt),sumA=sumVals(m.values||{}),sumB=sum-sumA;
 content=`<div class="modal-text">原批次「${E(bt.name)}」目标 ${sum} 条。调整 A 的数量，B 自动取剩余量；每个平台 × 类目数量保持不变。</div><div class="table-wrap m-top16"><table class="table ex-table"><thead><tr><th>类目</th><th>原数量</th><th>批次 A</th><th>批次 B</th></tr></thead><tbody>${Object.entries(bt.cats).filter(([,v])=>v>0).map(([c,n])=>`<tr><td>${E(c)}</td><td>${n}</td><td><input class="input" type="number" min="0" max="${n}" style="width:80px" data-split-cell="${E(c)}" value="${m.values[c]??0}"/></td><td>${n-(m.values[c]??0)}</td></tr>`).join('')}<tr class="matrix-total"><td>合计</td><td>${sum}</td><td>${sumA}</td><td>${sumB}</td></tr></tbody></table></div><div class="hint m-top16">${$icon('info',16)}拆分后新增一个未锁定批次，插入原批次的下一位置；后续顺序依次顺延。</div>`;
 footer=cancelBtn+okBtn('确认拆分','do-split-batch');wide=true;
 }else if(m.type==='delete-batch'){
 const bt=state.editor.batches[m.index];title='删除批次规划';content=`<p class="modal-text">确认删除「${E(bt.name)}」？该批次的目标数量将回到待分配区。必须重新分配数量才能确认或保存正式计划。</p>`;footer=cancelBtn+dangerBtn('确认删除','do-delete-batch');
 }else if(m.type==='overwrite-reco'){
 title='覆盖现有批次规划？';content=`<p class="modal-text">重新推荐会覆盖当前未锁定批次的人工调整。建议先核对平台 × 类目目标矩阵；已锁定资源的批次不允许整体重新推荐。</p>`;footer=cancelBtn+okBtn('确认重新推荐','do-recommend');
 }else if(m.type==='end-plan'){
 const g=Math.max(0,p.target-successOf(p));title='确认结束发布计划';content=`<p class="modal-text" style="margin-top:0">所有批次已经结束，但计划目标尚未完全达成。确认后计划将进入「已结束 · 未达成结束」，不再补量或执行。</p><div class="modal-kv"><div><div class="label">计划目标</div><div class="value">${p.target} 条</div></div><div><div class="label">最终发布成功</div><div class="value">${successOf(p)} 条</div></div><div><div class="label">目标缺口</div><div class="value">${g} 条</div></div><div><div class="label">结束结果</div><div class="value">未达成结束</div></div></div><div class="form-field m-top16"><label class="field-label">未达成原因 / 结果说明<span class="req">*</span></label><textarea rows="3" class="input" data-modal-field="reason" placeholder="请说明放弃补量的原因或最终执行结论">${E(m.reason||'')}</textarea></div>`;footer=cancelBtn+okBtn('确认结束','do-end-plan');
 }else if(m.type==='task-verify'){
 title='核实发布结果';content=`<p class="modal-text" style="margin-top:0">${E(m.taskTitle)} · 请根据平台实际状态确认最终结果。</p><div class="form-field"><label class="field-label">核实结论<span class="req">*</span></label><select class="select" data-modal-field="result"><option value="success" ${m.result!=='fail'?'selected':''}>确认已发布成功</option><option value="fail" ${m.result==='fail'?'selected':''}>确认发布失败</option></select></div><div class="form-field m-top16"><label class="field-label">成功发布链接<span class="req">*</span> <span class="subtext">（确认成功时必填）</span></label><input class="input" data-modal-field="url" value="${E(m.url||'')}" placeholder="https://平台作品链接"/></div><div class="form-field m-top16"><label class="field-label">核实说明（选填）</label><textarea rows="2" class="input" data-modal-field="comment" placeholder="补充核实依据">${E(m.comment||'')}</textarea></div>`;footer=cancelBtn+okBtn('保存核实结果','do-task-verify');
 }else if(m.type==='task-manual'){
 title=p.status==='stopping'?'核实人工任务结果':'人工确认发布';content=`<p class="modal-text" style="margin-top:0">${E(m.taskTitle)}。${p.status==='stopping'?'计划已请求取消，请勿新增提交。若已经提交请核实结果；尚未提交可确认放弃。':'请先在平台完成最终提交，再回填结果。不能仅凭页面已填写完成就认定发布成功。'}</p><div class="form-field"><label class="field-label">处理结果</label><select class="select" data-modal-field="result"><option value="success" ${m.result!=='pending'?'selected':''}>已发布成功，可提供链接</option><option value="pending" ${m.result==='pending'?'selected':''}>已提交，暂待核实</option></select></div><div class="form-field m-top16"><label class="field-label">发布成功链接（成功时必填）</label><input class="input" data-modal-field="url" value="${E(m.url||'')}" placeholder="https://平台作品链接"/></div>`;footer=cancelBtn+(p.status==='stopping'?`<button class="btn danger" data-act="do-task-abandon">确认尚未提交并放弃</button>`:'')+okBtn('确认结果','do-task-manual');
 }else if(m.type==='task-failed'){
 title='处理发布失败';content=`<p class="modal-text" style="margin-top:0">${E(m.taskTitle)} · 当前操作只作用于该发布任务，不直接改变计划主状态。</p><div class="hint orange">${$icon('alert',16)}系统检测到上传异常。可在确认资源可用后重试，或决定放弃本次发布。</div><div class="form-field m-top16"><label class="field-label">处理说明（选填）</label><textarea rows="2" class="input" data-modal-field="comment">${E(m.comment||'')}</textarea></div>`;footer=cancelBtn+`<button class="btn danger" data-act="do-task-abandon">确认放弃</button>`+(p.status==='stopping'?'':okBtn('重试发布','do-task-retry'));
 }else if(m.type==='info'){
 title=m.title||'提示';content=`<p class="modal-text">${E(m.text)}</p>`;footer=okBtn('知道了','close-modal');
 }
 return `<dialog class="plan-modal"><div class="modal ${wide?'wide':''}"><div class="modal-head"><h3>${title}</h3><button class="btn icon-btn ghost" data-act="close-modal">${$icon('close',19)}</button></div><div class="modal-body">${content}</div><div class="modal-footer">${footer}</div></div></dialog>`;
}
function setToast(text,type='success'){if(type!=='error'&&!persistPrototype()){text='当前输入仍保留在页面，但本地保存失败，请勿关闭页面';type='error';}state.toast={text,type};render();if(toastTimer)clearTimeout(toastTimer);toastTimer=setTimeout(()=>{state.toast=null;render();},3300)}
function openModal(type,data={}){state.modal={type,...data};render();}
function runValidation(e){const problems=batchValidation(e);if(problems.length){openModal('validation',{issues:problems});return false;}return true;}
function planFromEditor(status){const e=state.editor;const id=e.mode==='edit'?e.sourceId:uniqueId('PL');
 return {id,name:e.name||'未命名发布计划',note:e.note,dateStart:e.dateStart,dateEnd:e.dateEnd,owner:e.owner||'运营负责人',status,result:'',target:e.target,platforms:clone(e.platforms),categories:clone(e.categories),goals:clone(e.goals),updated:new Date().toLocaleString('sv-SE').slice(0,16),created:new Date().toLocaleString('sv-SE').slice(0,16),logs:[[ new Date().toLocaleString('sv-SE').slice(0,16),status==='draft'?'保存计划草稿':'确认创建发布计划','运营负责人']],batches:clone(e.batches).map((b,i)=>({...b,id:('BT-'+id.split('-').pop())+'-'+String(i+1).padStart(2,'0'),phase:'draft',success:0,locked:false,configured:false,executed:false,tasks:[],type:'base'}))};
}
function editorSaveDraft(){const e=state.editor;if(!e)return;const fresh=planFromEditor('draft');plans.unshift(fresh);state.modal=null;state.editor=null;state.page='list';setToast('发布计划草稿已保存，后续可继续编辑');window.scrollTo(0,0);}
function editorConfirm(){const e=state.editor;if(!e)return;if(!runValidation(e))return;openModal('confirm-plan');}
function commitPlan(){if(!state.editor||!runValidation(state.editor))return;const fresh=planFromEditor('ready');plans.unshift(fresh);state.modal=null;state.editor=null;state.page='detail';state.planId=fresh.id;setToast('发布计划已创建，批次规划已生成；尚未执行发布');window.scrollTo(0,0);}
function editorSaveEdit(){const e=state.editor,p=e&&getPlan(e.sourceId);if(!e||!p||['ended','stopping'].includes(p.status))return;if(p.status!=='draft'&&!runValidation(e))return;
 const lockedOriginal=p.batches.filter(x=>x.locked||['running','ended','cancelled'].includes(x.phase));
 const changedLocked=lockedOriginal.some(x=>{const candidate=e.batches.find(b=>b.id===x.id);return !candidate||JSON.stringify(candidate)!==JSON.stringify(x)||e.batches.indexOf(candidate)!==p.batches.indexOf(x);});
 if(changedLocked)return openModal('validation',{issues:['已锁定或已执行批次不允许更换平台、类目数量或执行状态。请刷新并重试。']});
 if((p.status==='running'||p.batches.some(b=>b.locked))&&(e.target!==p.target||JSON.stringify(e.goals)!==JSON.stringify(p.goals)||JSON.stringify(e.platforms)!==JSON.stringify(p.platforms)||JSON.stringify(e.categories)!==JSON.stringify(p.categories)||e.dateStart!==p.dateStart||e.dateEnd!==p.dateEnd))return openModal('validation',{issues:['计划已开始执行，总目标与平台 × 类目目标矩阵不可修改。']});
 Object.assign(p,{name:e.name,note:e.note,owner:e.owner,dateStart:e.dateStart,dateEnd:e.dateEnd,target:e.target,platforms:clone(e.platforms),categories:clone(e.categories),goals:clone(e.goals),batches:clone(e.batches)});
 logPlan(p,'修改计划与未锁定批次配置');state.editor=null;state.page='detail';state.planId=p.id;setToast('修改已保存，已锁定批次未被改变');window.scrollTo(0,0);
}
function commitDraft(){const e=state.editor,p=getPlan(e.sourceId);if(!e||!p||p.status!=='draft'||!runValidation(e))return;Object.assign(p,{name:e.name,note:e.note,dateStart:e.dateStart,dateEnd:e.dateEnd,owner:e.owner,target:e.target,platforms:clone(e.platforms),categories:clone(e.categories),goals:clone(e.goals),batches:clone(e.batches),status:'ready'});logPlan(p,'人工确认计划，批次规划已生成');state.editor=null;state.modal=null;state.page='detail';state.planId=p.id;setToast('计划已确认，进入待执行；批次不会自动启动');window.scrollTo(0,0);}
function findTask(m){const p=getPlan(m.planId),b=p?.batches.find(x=>x.id===m.batchId),t=b?.tasks.find(x=>x.id===m.taskId);return {p,b,t};}
function mustUrl(url){try{const u=new URL(url),h=u.hostname.toLowerCase();return u.protocol==='https:'&&!u.username&&!u.password&&h.includes('.')&&!/^\d+(\.\d+){3}$/.test(h)&&!h.endsWith('.local')&&!h.endsWith('.localhost')&&h!=='localhost'}catch{return false}}

function taskDone(m,st,extra={}){const {p,b,t}=findTask(m);
 if(!p||!b||!t||!['pending','retrying'].includes(t.status)||p.status==='ended')return;
 if(st==='success'&&(!mustUrl(extra.publishUrl)||b.success>=sumBatch(b))){setToast('请填写有效的 HTTPS 作品链接后再核实成功','error');return;}
 if(st==='retrying'&&p.status==='stopping'){setToast('取消中的计划不可重新发布','error');return;}
 if(st==='success'){
  const before=Object.fromEntries(Object.keys(b.cats).map(c=>[c,actualByCell({...p,batches:[b]},b.platform,c)]));
  const cat=t.category||Object.keys(b.cats).find(c=>(before[c]||0)<b.cats[c]);
  if(!cat||!b.cats[cat]||before[cat]>=b.cats[cat])return;
  b.successCats=before;b.successCats[cat]=(before[cat]||0)+1;b.success++;
 }
 Object.assign(t,{status:st,...extra});
 logPlan(p,`任务 ${t.id}：${st==='success'?'核实成功':st==='retrying'?'重试中，等待结果':st==='abandoned'?'确认放弃':'待后续核实'}`);
 state.modal=null;state.drawer={type:'workbench',planId:p.id,batchId:b.id};setToast('任务处理结果已保存；批次结束需等待所有执行结果收敛');
}

function onAction(act,el){const id=el.dataset.id,idx=Number(el.dataset.idx),p=id?getPlan(id):null;
 switch(act){
  case 'demo-settle':{
   const bt=p?.batches.find(x=>x.id===el.dataset.batch);
   if(!bt||!['running','stopping'].includes(bt.phase)||openTasks({...p,batches:[bt]}).length){setToast('仍有待办或重试结果未核实，请先处理','error');break;}
   bt.phase='ended';logPlan(p,`演示反馈：${bt.name} 已收敛；未提供成功证据的剩余目标不计成功`,'演示控制');
   if(p.status==='stopping'&&!openTasks(p).length&&!p.batches.some(b=>['running','stopping'].includes(b.phase))){p.status='ended';p.result='cancelled';}
   else if(allEnded(p)&&successOf(p)>=p.target){p.status='ended';p.result='achieved';logPlan(p,'目标达成，计划已结束','演示控制');}
   state.drawer=null;setToast('已接收演示结束反馈；下一批仍需人工确认');break;
  }
  case 'demo-retry-result':{
   const bt=p?.batches.find(x=>x.id===el.dataset.batch),t=bt?.tasks.find(t=>t.id===el.dataset.task);
   if(t?.status==='retrying'){t.status='pending';t.type='verify';t.description='演示重试返回，等待核实最终结果';logPlan(p,`演示反馈：任务 ${t.id} 重试返回，转待核实`,'演示控制');setToast('重试结果待核实，不计入成功');}break;
  }

  case 'list':if(state.editor)state.recoveryEditor=clone(state.editor);state.page='list';state.modal=null;state.drawer=null;state.editor=null;render();window.scrollTo(0,0);break;
  case 'new':if(state.recoveryEditor){state.editor=clone(state.recoveryEditor);state.page='editor';state.recoveryEditor=null;render();}else newEditor();break;
  case 'discard-recovery':state.recoveryEditor=null;render();break;
  case 'reset-filters':state.filterDraft={search:'',status:'all',needOnly:false,platform:'',category:''};state.filters=clone(state.filterDraft);state.pageNo=1;render();break;
  case 'page':state.pageNo+=Number(el.dataset.dir);render();break;
  case 'detail':detailPlan(id);break;
  case 'focus-progress':detailPlan(id);setTimeout(()=>$('#batch-area')?.scrollIntoView({behavior:'smooth',block:'start'}),30);break;
  case 'edit':editPlan(id);break;
  case 'copy':if(p&&p.status==='ended')newEditor(p);break;
  case 'filter-status':state.statusFilter=el.dataset.value;render();break;
  case 'advanced':state.advanced=!state.advanced;render();break;
  case 'jump':state.activeStep=Number(el.dataset.step);document.getElementById('step-0'+state.activeStep)?.scrollIntoView({behavior:'smooth',block:'start'});document.querySelectorAll('.editor-step').forEach((e,i)=>e.classList.toggle('active',i+1===state.activeStep));break;
  case 'toggle-platform':state.platformExpand=state.platformExpand===el.dataset.platform?null:el.dataset.platform;render();break;
  case 'refresh-inventory':state.inventoryAt=new Date().toLocaleString('sv-SE').slice(0,16);setToast('资源盘点已刷新（原型使用示例资源数据）');break;
  case 'toggle-choice':{
   const e=state.editor;if(!e||isGoalLocked(e))break;const val=el.dataset.value,key=el.dataset.kind==='platform'?'platforms':'categories';
   e[key]=e[key].includes(val)?e[key].filter(x=>x!==val):[...e[key],val];
   if(key==='platforms'){const selected={};for(const q of e.platforms)selected[q]=e.goals[q]||{};e.goals=selected;}
   else{for(const plat of e.platforms){const next={};for(const c of e.categories)next[c]=(e.goals[plat]||{})[c]||0;e.goals[plat]=next;}}
   render();break;
  }
  case 'generate-matrix':{
   const e=state.editor;if(isGoalLocked(e))break;allocateGoals(e);render();setToast('已根据当前资源容量生成初始目标矩阵，请人工校核');break;
  }
  case 'change-reco':state.reco=el.dataset.reco;render();break;
  case 'recommend':{
   const e=state.editor;if(!e||e.batches.some(x=>x.locked))break;const issues=recommendationIssues(e);if(issues.length){openModal('validation',{issues});break;}
   if(e.editedBatches)openModal('overwrite-reco');else{e.batches=genBatchSplit(e,state.reco==='ai');render();setToast(state.reco==='ai'?'AI 推荐演示方案已生成':'规则推荐方案已生成');}
   break;
  }
  case 'do-recommend':{
   const e=state.editor;if(!e||e.batches.some(x=>x.locked))break;const issues=recommendationIssues(e);if(issues.length){openModal('validation',{issues});break;}e.batches=genBatchSplit(e,state.reco==='ai');e.editedBatches=false;state.modal=null;setToast('已重新生成拆分方案，人工调整已重置');break;
  }
  case 'add-batch':{
   const e=state.editor;if(!e.platforms.length){setToast('请先在 03 中选择目标平台','error');break;}
   e.batches.push(b('TEMP-'+(++identitySequence),`新增批次 ${String(e.batches.length+1).padStart(2,'0')}`,e.platforms[0],{}));e.editedBatches=true;render();break;
  }
  case 'move-batch':{
   const e=state.editor,i=idx,j=i+Number(el.dataset.dir);if(j<0||j>=e.batches.length||!batchEditable(e,e.batches[i])||!batchEditable(e,e.batches[j]))break;
   [e.batches[i],e.batches[j]]=[e.batches[j],e.batches[i]];e.editedBatches=true;render();break;
  }
  case 'split-batch':{
   const bt=state.editor?.batches[idx];if(!bt||!batchEditable(state.editor,bt))break;
   const values={};Object.entries(bt.cats).forEach(([c,n])=>values[c]=Math.ceil(n/2));openModal('split-batch',{index:idx,values});break;
  }
  case 'do-split-batch':{
   const e=state.editor,m=state.modal,bt=e?.batches[m?.index];if(!bt||!batchEditable(e,bt))break;const sumA=sumVals(m.values),total=sumBatch(bt);
   if(sumA<=0||sumA>=total){setToast('拆分后的两个批次必须都具有非零目标数量','error');break;}
   const before={};const after={};Object.entries(bt.cats).forEach(([c,n])=>{let a=Math.floor(Number(m.values[c])||0);a=Math.max(0,Math.min(n,a));if(a)before[c]=a;if(n-a)after[c]=n-a;});
   bt.cats=before;const second=b('TEMP-'+(++identitySequence),bt.name+' · 拆分',bt.platform,after,'draft',0,false,{type:bt.type});e.batches.splice(m.index+1,0,second);e.editedBatches=true;state.modal=null;setToast('拆分完成，平台 × 类目目标数量保持不变');break;
  }
  case 'merge-batch':{
   const e=state.editor,i=idx,prev=e.batches[i-1],cur=e.batches[i];if(!prev||!cur||prev.platform!==cur.platform||prev.type!==cur.type||!batchEditable(e,prev)||!batchEditable(e,cur))break;
   for(const [c,n] of Object.entries(cur.cats))prev.cats[c]=(prev.cats[c]||0)+n;e.batches.splice(i,1);e.editedBatches=true;setToast('同平台批次已合并，数量守恒');break;
  }
  case 'remove-batch':if(batchEditable(state.editor,state.editor.batches[idx]))openModal('delete-batch',{index:idx});break;
  case 'do-delete-batch':if(!batchEditable(state.editor,state.editor.batches[state.modal.index]))break;state.editor.batches.splice(state.modal.index,1);state.editor.editedBatches=true;state.modal=null;setToast('批次已删除，需重新分配对应目标数量');break;
  case 'save-draft':editorSaveDraft();break;
  case 'confirm-plan':editorConfirm();break;
  case 'confirm-draft':editorConfirm();break;
  case 'do-confirm-draft':commitDraft();break;
  case 'do-confirm-plan':commitPlan();break;
  case 'save-edit':editorSaveEdit();break;
  case 'cancel':if(p&&['ready','running'].includes(p.status))openModal('cancel-plan',{planId:p.id,reason:''});break;
  case 'do-cancel-plan':{
   const m=state.modal,p=getPlan(m?.planId);if(!p||!['ready','running'].includes(p.status))break;
   if(!m.reason?.trim()){setToast('请填写取消原因','error');break;}
   const running=p.batches.filter(x=>['running','stopping'].includes(x.phase));
   for(const b of p.batches)if(['draft','ready'].includes(b.phase))b.phase='cancelled';
   p.cancelReason=m.reason.trim();
   if(running.length||openTasks(p).length){p.status='stopping';for(const b of running)b.phase='stopping';logPlan(p,`取消请求：${p.cancelReason}；等待执行器结果收敛（原型）`);}
   else{p.status='ended';p.result='cancelled';logPlan(p,`取消计划：${p.cancelReason}`);}
   state.modal=null;state.drawer=null;state.page='detail';state.planId=p.id;setToast(p.status==='stopping'?'已记录取消请求；仍需确认在途任务与未决结果':'计划已取消，已发布结果保留');break;
  }
  case 'delete':if(p?.status==='draft')openModal('delete-plan',{planId:p.id});break;
  case 'do-delete-plan':{
   const pid=state.modal.planId;if(getPlan(pid)?.status!=='draft')break;plans=plans.filter(x=>x.id!==pid);state.modal=null;state.page='list';setToast('草稿已删除');break;
  }
  case 'end-plan':if(p&&p.status==='running'&&allEnded(p)&&openTasks(p).length===0&&successOf(p)<p.target)openModal('end-plan',{planId:p.id,reason:''});break;
  case 'do-end-plan':{
   const m=state.modal,p=getPlan(m.planId);if(!m.reason?.trim()){setToast('请填写未达成原因或最终结果说明','error');break;}
   if(p.status!=='running'||successOf(p)>=p.target||!allEnded(p)||openTasks(p).length){setToast('仍有批次或任务未结束，不能确认计划结束','error');break;}
   p.status='ended';p.result='unachieved';logPlan(p,`人工确认未达成结束：${m.reason.trim()}`);state.modal=null;state.page='detail';state.planId=p.id;setToast('计划已结束，结果为未达成结束');break;
  }
  case 'view-batch':state.drawer={type:'batch',planId:id,batchId:el.dataset.batch};state.modal=null;render();break;
  case 'workbench':state.drawer={type:'workbench',planId:id,batchId:el.dataset.batch};state.taskFilter='all';state.modal=null;render();break;
  case 'task-filter':state.taskFilter=el.dataset.value;render();break;
  case 'close-drawer':state.drawer=null;render();break;
  case 'demo-config':{
   const b=p?.batches.find(x=>x.id===el.dataset.batch);
   if(b&&b.phase==='draft'&&canExecute(p,b)){b.locked=true;b.configured=true;b.phase='ready';b.accountCount=pd(b.platform).accounts;logPlan(p,`批次 ${b.id} 完成配置（原型演示）`);state.drawer={type:'batch',planId:p.id,batchId:b.id};setToast('原型模拟：批次资源和内容已配置，现可人工确认执行');}
   break;
  }
  case 'execute':{
   const b=p?.batches.find(x=>x.id===el.dataset.batch);if(!b||b.phase!=='ready'||!b.locked||!b.configured||!canExecute(p,b)){setToast('不满足串行执行或资源锁定前置条件','error');break;}
   openModal('execute-batch',{planId:p.id,batchId:b.id});break;
  }
  case 'do-execute-batch':{
   const m=state.modal,p=getPlan(m.planId),b=p.batches.find(x=>x.id===m.batchId);
   if(!canExecute(p,b)||!b.locked||!b.configured||b.phase!=='ready'){setToast('执行前校验未通过，请刷新后再试','error');break;}
   b.phase='running';b.executed=true;p.status='running';logPlan(p,`人工确认执行 ${b.name}`);state.modal=null;state.drawer=null;state.page='detail';state.planId=p.id;setToast('批次已启动，计划保持进行中；后续批次继续等待');break;
  }
  case 'makeup':{
   if(!p||p.status!=='running'||!allEnded(p)||openTasks(p).length)return;
   state.drawer={type:'makeup',planId:p.id,platform:'',name:'',values:{}};render();break;
  }
  case 'create-makeup':{
   const d=state.drawer,p=d&&getPlan(d.planId),pl=d?.platform,values={};let total=0;if(!p||p.status!=='running'||!allEnded(p)||openTasks(p).length||!p.platforms.includes(pl))return;
   for(const c of p.categories){const max=(p.goals[pl]?.[c]||0)-actualByCell(p,pl,c);if(max<=0)continue;const val=d.values[c]===undefined?max:Number(d.values[c]);if(!Number.isInteger(val)||val<0||val>max){setToast(`${c} 补量数量无效，请按实际目标缺口填写`,'error');return;}if(val>0){values[c]=val;total+=val;}}
   if(total<=0){setToast('请至少填写 1 条补量目标','error');break;}
   const item=b('BT-M'+(++identitySequence),d.name||`补量批次 · ${pl}`,pl,values,'draft',0,false,{type:'makeup'});p.batches.push(item);logPlan(p,`新增补量批次 ${item.id}，目标 ${total} 条（计划总目标不变）`);
   state.drawer=null;setToast(`已生成 ${total} 条补量批次；请先完成资源配置再人工确认执行`);break;
  }
  case 'task-action':{
   const p=getPlan(id),bt=p?.batches.find(x=>x.id===el.dataset.batch),t=bt?.tasks.find(x=>x.id===el.dataset.task);if(!t||t.status!=='pending')break;
   const type=t.type==='manual'?'task-manual':t.type==='verify'?'task-verify':'task-failed';openModal(type,{planId:id,batchId:bt.id,taskId:t.id,taskTitle:t.title,result:'success',url:'',comment:''});break;
  }
  case 'do-task-verify':{
   const m=state.modal;if(m.result!=='fail'&&!mustUrl(m.url)){setToast('确认成功必须填写有效的发布链接','error');break;}
   taskDone(m,m.result==='fail'?'abandoned':'success',{publishUrl:m.url||'',note:m.comment||''});break;
  }
  case 'do-task-manual':{
   const m=state.modal;if(m.result!=='pending'&&!mustUrl(m.url)){setToast('确认发布成功时需填写作品链接','error');break;}
   const {t}=findTask(m);if(m.result==='pending'){t.type='verify';t.description='已由人工提交，等待最终链接或发布状态核实';state.modal=null;setToast('已转为待核实，不计入发布成功');break;}
   taskDone(m,'success',{publishUrl:m.url});break;
  }
  case 'do-task-retry':taskDone(state.modal,'retrying',{note:state.modal.comment});break;
  case 'do-task-abandon':taskDone(state.modal,'abandoned',{note:state.modal.comment||'人工确认放弃'});break;
  case 'close-modal':state.modal=null;render();break;
 }
}
// 失焦 change 不得重建正在按下的操作按钮；click 完成后统一刷新。
let pointerActionPending=false;
document.addEventListener('pointerdown',ev=>{pointerActionPending=!!ev.target.closest('button,a,input,select,textarea,[data-act]');},true);
document.addEventListener('pointerup',()=>setTimeout(()=>{pointerActionPending=false;},0),true);
document.addEventListener('pointercancel',()=>{pointerActionPending=false;},true);
document.addEventListener('click',ev=>{const el=ev.target.closest('[data-act]');if(!el||el.disabled)return;if(el.classList.contains('overlay')&&ev.target!==el)return;ev.preventDefault();if(el.closest('.modal')===null&&state.modal&&el.dataset.act!=='close-modal')return;onAction(el.dataset.act,el);if(el.dataset.act!=='jump')render();});
document.addEventListener('input',ev=>{const t=ev.target;if(t.dataset.filter){state.filterDraft[t.dataset.filter]=t.type==='checkbox'?t.checked:t.value;return;}if(t.dataset.field==='search'){
 const pos=t.selectionStart;state.search=t.value;render();const next=document.querySelector('[data-field="search"]');if(next){next.focus();next.setSelectionRange(pos,pos);}return;}
 if(t.dataset.editor){const e=state.editor;if(e){e[t.dataset.editor]=t.dataset.editor==='target'?Math.max(0,Math.floor(Number(t.value)||0)):t.value;}return;}
 if(t.dataset.matrix){const e=state.editor;if(e){const p=t.dataset.platform,c=t.dataset.category;e.goals[p]??={};e.goals[p][c]=Math.max(0,Math.floor(Number(t.value)||0));}return;}
 if(t.dataset.batchCell){const e=state.editor;if(e){const bt=e.batches[Number(t.dataset.idx)];bt.cats[t.dataset.category]=Math.max(0,Math.floor(Number(t.value)||0));e.editedBatches=true;}return;}
 if(t.dataset.batchName!==undefined){const e=state.editor;if(e){e.batches[Number(t.dataset.batchName)].name=t.value;e.editedBatches=true;}return;}
 if(t.dataset.modalField){if(state.modal)state.modal[t.dataset.modalField]=t.value;return;}
 if(t.dataset.splitCell){if(state.modal){const bt=state.editor.batches[state.modal.index];const n=bt.cats[t.dataset.splitCell]||0;state.modal.values[t.dataset.splitCell]=Math.max(0,Math.min(n,Math.floor(Number(t.value)||0)));}return;}
 if(t.dataset.makeup==='name'){if(state.drawer)state.drawer.name=t.value;return;}
 if(t.dataset.makeupCell){if(state.drawer){state.drawer.values??={};state.drawer.values[t.dataset.makeupCell]=t.value;}return;}
});
document.addEventListener('change',ev=>{const t=ev.target;if(t.hasAttribute('data-page-size')){state.pageSize=Number(t.value);state.pageNo=1;render();return;}if(t.dataset.filter){state.filterDraft[t.dataset.filter]=t.type==='checkbox'?t.checked:t.value;return;}if(t.dataset.field==='needOnly'){state.needOnly=t.checked;render();return;}
 if(t.dataset.batchPlatform!==undefined){const e=state.editor,b=e?.batches[Number(t.dataset.batchPlatform)];if(!b||!batchEditable(e,b))return;b.platform=t.value;b.cats={};e.editedBatches=true;render();return;}
 if(t.dataset.makeup==='platform'){if(state.drawer){state.drawer.platform=t.value;state.drawer.name=`补量批次 · ${t.value}`;state.drawer.values={};render();}return;}
 if(t.dataset.modalField==='result'||t.dataset.splitCell){render();return;}
 if(t.dataset.editor||t.dataset.matrix||t.dataset.batchCell||t.dataset.batchName!==undefined){if(!pointerActionPending)setTimeout(render,0);return;}
});

document.addEventListener('keydown',ev=>{if(['Enter',' '].includes(ev.key)&&ev.target.matches('a[data-act],.tagcheck,.editor-step')){ev.preventDefault();ev.target.click();}});
document.addEventListener('submit',ev=>{if(ev.target.id==='plan-filters'){ev.preventDefault();state.filters=clone(state.filterDraft);state.pageNo=1;render();}});
let numericRefreshTimer=null;
function scheduleNumericRefresh(){
 clearTimeout(numericRefreshTimer);
 numericRefreshTimer=setTimeout(()=>{if(pointerActionPending||document.querySelector(':popover-open')){scheduleNumericRefresh();return;}render();},180);
}
document.addEventListener('input',ev=>{persistPrototype();const t=ev.target;if(t.dataset.matrix||t.dataset.batchCell||t.dataset.splitCell||t.dataset.editor==='target')scheduleNumericRefresh();});
window.addEventListener('pagehide',persistPrototype);
render();
