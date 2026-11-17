/* 发布计划 V1 高保真交互原型 · 全部示例数据仅用于 UX 评审 */
import { ownershipIssue } from './ownership.js'

export function mountPlan(host, adapters) {
 const dom=host, routes=adapters.routes;
 let batchDrag=null;
 let disposed=false,leaveResolve=null;
 const disposers=[],timers=new Set();
 const setTimeout=(fn,ms)=>{const id=globalThis.setTimeout(()=>{timers.delete(id);if(!disposed)fn();},ms);timers.add(id);return id;};
 const clearTimeout=id=>{globalThis.clearTimeout(id);timers.delete(id);};
 const listen=(target,type,handler,options)=>{target.addEventListener(type,handler,options);disposers.push(()=>target.removeEventListener(type,handler,options));};
 const actor=()=>adapters.actor();
 const ownerIssue=e=>ownershipIssue(e,e.mode==='edit'?getPlan(e.sourceId):null,actor(),adapters.members);
 // Plan state engine: independently persisted, mounted only within the shared shell.
const $ = s => dom.querySelector(s);
const clone = x => JSON.parse(JSON.stringify(x));
const escapeHtml = s => String(s ?? '').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const cn = n => (Number(n)||0).toLocaleString('zh-CN');
const E = escapeHtml;
let hintSequence = 0;
const fieldHelp = (label, content) => { const id='plan-field-help-'+(++hintSequence); return `<span class="ex-field-help"><button type="button" aria-label="${E(label)}说明" aria-describedby="${id}"></button><span class="ex-field-help-content" role="tooltip" id="${id}">${E(content)}</span></span>`; };
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
  'grip':'<circle cx="8" cy="5" r="1"/><circle cx="16" cy="5" r="1"/><circle cx="8" cy="12" r="1"/><circle cx="16" cy="12" r="1"/><circle cx="8" cy="19" r="1"/><circle cx="16" cy="19" r="1"/>',
  'move-up':'<path d="m7 14 5-5 5 5"/>','move-down':'<path d="m7 10 5 5 5-5"/>','sparkles':'<path d="m12 3 1.8 5.2L19 10l-5.2 1.8L12 17l-1.8-5.2L5 10l5.2-1.8ZM19 18l.7 2.3L22 21l-2.3.7L19 24l-.7-2.3L16 21l2.3-.7Z"/>',
  'info':'<circle cx="12" cy="12" r="9"/><path d="M12 11v6M12 7h.01"/>','download':'<path d="M12 3v12m-4-4 4 4 4-4M4 17v4h16v-4"/>','upload':'<path d="M12 17V5m-4 4 4-4 4 4M4 17v4h16v-4"/>',
  'activity':'<path d="M2 12h5l3-7 4 14 3-7h5"/>','workflow':'<rect x="2" y="3" width="7" height="6" rx="1"/><rect x="15" y="15" width="7" height="6" rx="1"/><path d="M5.5 9v7h9.5M15 16l-3-3m3 3-3 3"/>',
  'bolt':'<path d="m13 2-9 12h7l-1 8 10-13h-7Z"/>','list':'<path d="M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01"/>',
  'filter':'<path d="M4 5h16L14 12v7l-4-2v-5Z"/>','help':'<circle cx="12" cy="12" r="9"/><path d="M9 9a3 3 0 1 1 5.2 2c-1 1-2.2 1.4-2.2 3M12 17h.01"/>',
  'split':'<path d="M12 3v6M5 21v-5l7-7 7 7v5M3 16h4M17 16h4"/>','shield':'<path d="M12 2 3 6v6c0 6 4 9 9 10 5-1 9-4 9-10V6Z"/><path d="m8 12 3 3 5-6"/>'
 };
 return `<svg width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${paths[n]||paths.info}</svg>`;
};
const CATS=adapters.inventory?.().categories||['游戏攻略','精彩剪辑','游戏资讯','游戏评测','赛事复盘','新手教学'];
const PLATFORM_DATA=[
 {name:'B站',accounts:8,videos:160,categories:5,capacity:60,cell:{'游戏攻略':35,'精彩剪辑':30,'游戏资讯':18,'游戏评测':10,'赛事复盘':0,'新手教学':12}},
 {name:'百家号',accounts:12,videos:210,categories:5,capacity:90,cell:{'游戏攻略':55,'精彩剪辑':35,'游戏资讯':40,'游戏评测':25,'赛事复盘':0,'新手教学':30}},
 {name:'小红书',accounts:10,videos:130,categories:4,capacity:50,cell:{'游戏攻略':25,'精彩剪辑':30,'游戏资讯':0,'游戏评测':12,'赛事复盘':0,'新手教学':15}}
];
if(adapters.inventory)PLATFORM_DATA.splice(0,PLATFORM_DATA.length,...adapters.inventory().platforms);
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
 {id:'PL-202611-001',name:'长视频运营提升计划（草稿）',note:'计划资源尚未确认。',dateStart:'2026-11-18',dateEnd:'2026-11-30',owner:actor().name,ownerId:actor().id,createdBy:actor().id,status:'draft',result:'',target:80,platforms:['B站','百家号'],categories:['游戏攻略','精彩剪辑'],goals:{'B站':{'游戏攻略':20,'精彩剪辑':20},'百家号':{'游戏攻略':20,'精彩剪辑':20}},updated:'2026-11-10 18:02',created:'2026-11-10 18:02',logs:[['11-10 18:02','保存计划草稿','运营负责人']],batches:[
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
 if(adapters.loadPlans&&data.version===1)return data;
 const validPlan=p=>p&&typeof p.id==='string'&&typeof p.name==='string'&&Array.isArray(p.platforms)&&p.platforms.every(pd)&&Array.isArray(p.categories)&&p.categories.every(c=>CATS.includes(c))&&p.goals&&Array.isArray(p.logs)&&typeof p.dateStart==='string'&&typeof p.dateEnd==='string'&&Array.isArray(p.batches)&&p.batches.every(b=>b&&pd(b.platform)&&b.cats&&Array.isArray(b.tasks));
 if(adapters.loadPlans&&data.version===1&&!data.plans)return data;
 if(data.version!==1||!Array.isArray(data.plans)||!data.plans.every(validPlan)||new Set(data.plans.map(p=>p.id)).size!==data.plans.length)throw Error('invalid');
 if(data.ui?.editor&&!validPlan({...data.ui.editor,id:'editor',logs:[]}))throw Error('invalid editor');return data;
 }catch{storageIssue='无法读取上次原型数据，当前使用演示初始数据；本次不会覆盖原存档。';return null;}
}
const savedPrototype=loadPrototype();
if(adapters.importLegacy)adapters.importLegacy(savedPrototype);
let plans=adapters.loadPlans?adapters.loadPlans():savedPrototype?.plans||clone(seedPlans);
const state={page:'list',planId:null,editor:null,statusFilter:'all',search:'',advanced:false,modal:null,drawer:null,platformExpand:null,taskFilter:'all',reco:'rule',toast:null};
Object.assign(state,{pageNo:1,pageSize:10,filterDraft:{search:'',status:'all',platform:'',category:''},filters:{search:'',status:'all',platform:'',category:''},recoveryEditor:null,migratedPlanDraftIds:[]},savedPrototype?.ui||{});
// Retire the removed filter from older saved list preferences.
for(const key of ['filters','filterDraft'])if(state[key])delete state[key].needOnly;
if (![10,20,50].includes(state.pageSize)) state.pageSize=10;
state.modal=null;state.drawer=null;state.toast=null;
if(adapters.loadPlans&&!state.recoveryEditor){const draft=savedPrototype?.plans?.find(p=>p.status==='draft');if(draft)state.recoveryEditor={...clone(draft),mode:'create',sourceId:'',savedAt:draft.updated||null};}
if(state.recoveryEditor?.mode !== 'create'||!hasCreationInput(state.recoveryEditor)) state.recoveryEditor=null;
if(state.editor){if(state.editor.mode==='create'&&hasCreationInput(state.editor)) state.recoveryEditor={...state.editor,savedAt:state.editor.savedAt||null};state.editor=null;state.page='list';}
// Historical list drafts remain archived; consume each through the single creation buffer once.
function promoteLegacyPlan(){
 if(state.recoveryEditor)return;
 const legacy=plans.find(p=>p.status==='draft'&&!state.migratedPlanDraftIds.includes(p.id));
 if(!legacy)return;
 state.recoveryEditor={...clone(legacy),mode:'create',sourceId:'',savedAt:legacy.updated||null};
 state.migratedPlanDraftIds.push(legacy.id);
}
promoteLegacyPlan();
for(const key of ['filters','filterDraft'])if(state[key]?.status==='draft')state[key].status='all';
if(plans.find(p=>p.id===state.planId)?.status==='draft')state.page='list';
let editorBaseline='';
function hasCreationInput(e){return !!e&&(!!e.name?.trim()||!!e.note?.trim()||e.platforms?.length||e.categories?.length||e.batches?.length||e.target>0||!!e.dateStart||!!e.dateEnd||!!(e.ownerId&&e.createdBy&&e.ownerId!==e.createdBy));}
function restoreCreation(){const e=clone(state.recoveryEditor),user=actor();e.owner=user.name;e.ownerId=user.id;return e;}
function rememberCreation(){if(state.editor?.mode==='create'&&hasCreationInput(state.editor))state.recoveryEditor={...clone(state.editor),savedAt:new Date().toISOString()};}
if(state.page==='editor'&&!state.editor)state.page='list';
if(state.page==='detail'&&!plans.some(p=>p.id===state.planId))state.page='list';
let identitySequence=Number(savedPrototype?.sequence)||Date.now();
const uniqueId=prefix=>prefix+'-'+(++identitySequence);
function persistPrototype(){
 if(storageIssue.startsWith('无法读取'))return false;
 try{adapters.savePlans?.(plans);if(adapters.loadPlans)plans=adapters.loadPlans();localStorage.setItem(STORAGE_KEY,JSON.stringify({version:1,...(!adapters.loadPlans?{plans}:{}),sequence:identitySequence,ui:{listScroll,detailScrolls:state.detailScrolls||{},page:state.page==='editor'&&state.editor?.mode==='edit'?'list':state.page,planId:state.planId,editor:state.editor?.mode==='create'&&hasCreationInput(state.editor)?state.editor:null,recoveryEditor:state.recoveryEditor,migratedPlanDraftIds:state.migratedPlanDraftIds,pageNo:state.pageNo,pageSize:state.pageSize,filters:state.filters,filterDraft:state.filterDraft,advanced:state.advanced,reco:state.reco}}));storageIssue='';return true;}
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
const resultText=r=>({achieved:'目标达成',unachieved:'未达成结束',pending:'待最终核实',cancelled:'已取消'}[r]||'');
const nameBatchStatus=s=>({draft:'待配置',ready:'待执行',running:'执行中',ended:'已结束',cancelled:'已取消',stopping:'停止中'}[s]||s);
const badgeBatchStatus=s=>({draft:'gray',ready:'blue',running:'blue',ended:'green',cancelled:'gray',stopping:'orange'}[s]||'gray');
const displayStatus=(s,typ='')=>`<span class="badge ${typ||badgeStatus(s)}"><span class="dot"></span>${E(nameStatus(s))}</span>`;
const bBadge=b=>`<span class="badge ${badgeBatchStatus(b.phase)}">${E(nameBatchStatus(b.phase))}</span>`;
const fmtTargets=b=>Object.entries(b.cats||{}).filter(([,v])=>v>0).map(([k,v])=>`${k} ${v}`).join(' / ')||'未分配';
const getPlan=id=>plans.find(p=>String(p.id)===String(id));
const logPlan=(p,msg,who=actor().name)=>{p.logs.unshift([new Date().toLocaleDateString('zh-CN',{month:'2-digit',day:'2-digit'}).replace('/','-')+' '+new Date().toTimeString().slice(0,5),msg,who]);p.updated=new Date().toLocaleString('sv-SE').slice(0,16)};
const currentNote=p=>{
 if(p.legacyReadOnly)return {txt:'旧版执行记录',type:'neutral',reason:''};
 if(p.status==='draft')return {txt:'计划待完善',type:'neutral',reason:'继续完善目标与批次拆分'};
 if(p.status==='stopping')return {txt:'停止处理中',type:'alert',reason:'等待执行中任务结果收敛'};
 if(p.status==='ended')return {txt:resultText(p.result)||'已结束',type:p.result==='achieved'?'complete':'neutral',reason:'计划已关闭'};
 const manual=openTasks(p).filter(t=>t.status==='pending');
 if(manual.length){const batches=[...new Set(manual.map(t=>t.batchId))];return {txt:`待人工处理 · ${manual.length}项`,type:'alert',reason:batches.length>1?`${batches.length} 个批次有待处理任务`:`${manual[0].batchName}需要介入`};}
 if(allEnded(p)){
  const gap=Math.max(0,p.target-successOf(p));
  return gap>0?{txt:`目标缺口 ${gap} 条`,type:'alert',reason:'待确认补量或结束'}:{txt:'目标已达成',type:'complete',reason:'已具备结束条件'};
 }
 const next=firstNotEnded(p);
 if(p.control==='等待继续确认')return {txt:'失败率超过 10%',type:'alert',reason:'等待人工确认'};
 if(p.control==='等待跳过确认')return {txt:'等待确认跳过',type:'alert',reason:''};
 if(p.control==='等待配置')return {txt:'下一批待配置',type:'wait',reason:''};
 if(p.control==='已暂停')return {txt:'已暂停',type:'wait',reason:''};
 if(next?.phase==='ready')return {txt:`${next.name} · 待确认执行`,type:'wait',reason:'人工确认后才能执行'};
 if(next?.phase==='draft')return {txt:`${next.name} · 待配置`,type:'wait',reason:'完成批次资源和内容配置'};
 if(next?.phase==='running')return {txt:`${next.name} · 执行中`,type:'active',reason:'暂无人工待办'};
 return {txt:'等待推进',type:'neutral',reason:''};
};
let lastFocus=null,lastLayer=null,modalFocus=null,unmountDetail=null,unmountList=null,unmountBasics=null,unmountScope=null,mountedEditor=null,listScroll=savedPrototype?.ui?.listScroll||0,routeInitialised=false,editorFooterObserver=null;
function syncDetailURL(){
 if(!routeInitialised||leaveResolve||routes.readOperationRoute(new URL(location.href)).page!=='plans')return;
 const target=routes.operationURL('plans',{editor:state.page==='editor',planId:state.page==='detail'?state.planId:state.editor?.mode==='edit'?state.editor.sourceId:null});
 if(location.pathname!==target)history.pushState(null,'',target);
}
function focusKey(el){if(!el||!el.matches?.('button,input,select,textarea,a,[tabindex]'))return null;return {id:el.id,tag:el.tagName,attrs:Object.fromEntries([...el.attributes].filter(a=>a.name.startsWith('data-')).map(a=>[a.name,a.value]))};}
function findFocus(key){if(!key)return null;if(key.id)return dom.querySelector('#'+CSS.escape(key.id));return [...dom.querySelectorAll(key.tag)].find(el=>Object.entries(key.attrs).every(([k,v])=>el.getAttribute(k)===v));}
function render(){
 if(disposed)return;
 clearBatchDrag();
 syncDetailURL();
 persistPrototype();
 const active=host.contains(document.activeElement)?document.activeElement:null,key=focusKey(active),position=active?.selectionStart;
 const layerKey=state.modal?'modal:'+state.modal.type:state.drawer?'drawer:'+state.drawer.type:null;
 const scrolls=[...dom.querySelectorAll('.drawer-body,.modal-body,.table-wrap')].map(el=>[el.scrollTop,el.scrollLeft]);
 if(layerKey&&!lastLayer)lastFocus=key;
 if(layerKey?.startsWith('modal:')&&!lastLayer?.startsWith('modal:'))modalFocus=key;
 // Retain mounted editor fields through dependent-table updates. Rebuilding a
 // Select loses its open popup, focus and the document's scroll position.
 const preserveEditor=state.page==='editor'&&state.editor===mountedEditor;
 const basics=preserveEditor?dom.querySelector('#plan-basics'):null;
 const scope=preserveEditor?dom.querySelector('#plan-goal-scope'):null;
 const windowScroll=[window.scrollX,window.scrollY];
 unmountDetail?.();unmountDetail=null;unmountList?.();unmountList=null;
 if(!preserveEditor){unmountBasics?.();unmountBasics=null;unmountScope?.();unmountScope=null;}
 editorFooterObserver?.disconnect();
 host.innerHTML=renderLayout();
 if(basics)dom.querySelector('#plan-basics')?.replaceWith(basics);
 if(scope)dom.querySelector('#plan-goal-scope')?.replaceWith(scope);
 const editorFooter=dom.querySelector('.editor-footer');
 if(editorFooter){editorFooterObserver=new ResizeObserver(()=>host.style.setProperty('--editor-footer-height',editorFooter.getBoundingClientRect().height+'px'));editorFooterObserver.observe(editorFooter);}

 if(state.page==='editor'){
  const basicProps={editor:state.editor,original:state.editor.mode==='edit'?getPlan(state.editor.sourceId):null,actor:actor(),members:adapters.members,locked:isGoalLocked(state.editor),onChange:changes=>{Object.assign(state.editor,changes);persistPrototype();}};
  if(unmountBasics)unmountBasics.update(basicProps);else unmountBasics=adapters.mountBasics(dom.querySelector('#plan-basics'),basicProps);
  const scopeProps={platforms:state.editor.platforms,categories:state.editor.categories,platformOptions:PLATFORM_DATA.map(p=>p.name),categoryOptions:CATS,locked:isGoalLocked(state.editor),onChange:changeGoalScope};
  if(unmountScope)unmountScope.update(scopeProps);else unmountScope=adapters.mountScope(dom.querySelector('#plan-goal-scope'),scopeProps);
 }
 mountedEditor=state.page==='editor'?state.editor:null;
 adapters.onView?.(state.page,state.editor?.mode);
 if(state.page==='list')unmountList=adapters.mountList(dom.querySelector('#plan-list'),{view:listView(),onFilterChange:(key,value)=>{if(key==='advanced'){state.advanced=value;render();dom.querySelector('#plan-list .resource-advanced-toggle button')?.focus({preventScroll:true});}else state.filterDraft[key]=value;},onQuery:()=>{state.filters=clone(state.filterDraft);state.pageNo=1;render();},onPageChange:page=>{state.pageNo=page.current;state.pageSize=page.pageSize;persistPrototype();}});
 if(state.page==='detail'){const p=getPlan(state.planId);unmountDetail=adapters.mountDetail(dom.querySelector('#plan-detail'),{plan:p,view:p?detailView(p):null,onBack:()=>onAction('list',{dataset:{}})});}
 dom.querySelectorAll('.btn').forEach(b=>{b.classList.add('ex-button');if(!b.getAttribute('type'))b.type='button';if(b.classList.contains('icon-btn'))b.setAttribute('aria-label',b.getAttribute('aria-label')||b.title||'关闭');});
 dom.querySelectorAll('label').forEach((label,i)=>{if(label.htmlFor||label.closest('#plan-list,#plan-basics,#plan-goal-scope'))return;const control=label.parentElement.querySelector('input:not([type=checkbox]),textarea,select');if(control){control.id||='plan-field-'+i;label.htmlFor=control.id;}});
 const drawer=dom.querySelector('dialog.plan-drawer'),modal=dom.querySelector('dialog.plan-modal');
 for(const [dialog,label] of [[drawer,'.drawer-title'],[modal,'.modal-head h3']])if(dialog){const heading=dialog.querySelector(label);if(heading){heading.id='title-'+dialog.className;dialog.setAttribute('aria-labelledby',heading.id);}dialog.showModal();dialog.addEventListener('cancel',event=>{event.preventDefault();if(dom.querySelector(':popover-open'))return;if(state.modal){leaveResolve?.(false);leaveResolve=null;state.modal=null;}else state.drawer=null;render();});}
 drawer?.classList.toggle('under-modal',!!modal);
 if(state.toast&&(modal||drawer)){const toast=dom.querySelector('.toast');if(toast)(modal||drawer).append(toast);}
 dom.querySelectorAll('a[data-act]').forEach(el=>{el.tabIndex=0;el.setAttribute('role','button');});
 dom.querySelectorAll('.tagcheck').forEach(el=>{el.tabIndex=0;el.setAttribute('role','checkbox');el.setAttribute('aria-checked',String(el.classList.contains('selected')));el.setAttribute('aria-disabled',String(!!el.querySelector('input:disabled')));});
 window.WTControls?.enhance(host);
 dom.querySelectorAll('.drawer-body,.modal-body,.table-wrap').forEach((el,i)=>{if(scrolls[i]){el.scrollTop=scrolls[i][0];el.scrollLeft=scrolls[i][1];}});
 if(preserveEditor)window.scrollTo(...windowScroll);
 const focus=active?.isConnected?active:findFocus(key);
 if(layerKey===lastLayer&&focus){focus.focus({preventScroll:true});if(typeof position==='number'&&focus.setSelectionRange&&['text','search','textarea'].includes(focus.type))focus.setSelectionRange(position,position);}
 else if(layerKey?.startsWith('drawer:')&&lastLayer?.startsWith('modal:'))findFocus(modalFocus)?.focus({preventScroll:true});
 else if(!layerKey&&lastLayer)findFocus(lastFocus)?.focus({preventScroll:true});
 lastLayer=layerKey;
}

function renderLayout(){return `<div class="plan-content ${state.page==='editor'?'is-editor resource-table-scope':''}"><div class="breadcrumbs">内容运营 / <button class="btn-link" data-act="list">发布计划</button>${state.page!=='list'?` / ${state.page==='editor'?(state.editor?.mode==='create'?'新建计划':'编辑计划'):'计划详情'}`:''}</div>${storageIssue?`<div class="hint orange" role="alert">${E(storageIssue)}</div>`:''}${state.page==='list'?renderList():state.page==='editor'?renderEditor():renderDetail()}</div>${renderDrawer()}${renderModal()}${state.toast?`<div role="status" class="toast ${state.toast.type==='error'?'error':''}">${$icon(state.toast.type==='error'?'alert':'check-circle',18)}${E(state.toast.text)}</div>`:''}`;}

function listView(){const f=state.filters;
 const filtered=plans.filter(p=>p.status!=='draft'&&(f.status==='all'||p.status===f.status)&&(!f.search||[p.name,p.id,p.owner].some(s=>s.toLowerCase().includes(f.search.toLowerCase())))&&(!f.platform||p.platforms.includes(f.platform))&&(!f.category||p.categories.includes(f.category)));
 const pages=Math.max(1,Math.ceil(filtered.length/state.pageSize));state.pageNo=Math.min(pages,Math.max(1,state.pageNo));
 return {draft:clone(state.filterDraft),advanced:state.advanced,appliedCount:[f.platform,f.category].filter(Boolean).length,pageNo:state.pageNo,pageSize:state.pageSize,
  statuses:[['all','全部'],['ready','待执行'],['running','进行中'],['stopping','停止中'],['ended','已结束']].map(([value,label])=>({value,label})),
  platforms:[{value:'',label:'全部'},...PLATFORM_DATA.map(p=>({value:p.name,label:p.name}))],categories:[{value:'',label:'全部'},...CATS.map(c=>({value:c,label:c}))],
  rows:filtered.map(p=>({id:p.id,name:p.name,statusLabel:nameStatus(p.status),statusTheme:['ready','running'].includes(p.status)?'primary':p.status==='stopping'?'warning':'default',result:p.status==='ended'?resultText(p.result):'',note:currentNote(p),success:successOf(p),target:p.target,rate:Math.round(successOf(p)/Math.max(1,p.target)*100),ended:endedCount(p),batches:p.batches.length,dateStart:p.dateStart,dateEnd:p.dateEnd,updated:p.updated,
   actions:p.legacyReadOnly?[{key:'detail',label:'详情'}]:[...(p.status==='ready'?[{key:'execute',label:'执行',batch:firstNotEnded(p)?.id,disabled:!!executionIssue(p,firstNotEnded(p)),reason:executionIssue(p,firstNotEnded(p))}]:[]),{key:'detail',label:'详情'},...(p.status==='ready'?[{key:'edit',label:'编辑'}]:[]),...(['ready','running'].includes(p.status)?[{key:'cancel',label:'终止',danger:true}]:[]),...(p.status==='draft'?[{key:'delete',label:'删除',danger:true}]:[]),...(p.status==='ended'?[{key:'copy',label:'复制'}]:[])]}))};
}
function renderList(){return '<div id="plan-list"></div>';}

function newEditor(fromPlan=null){const fresh=fromPlan?clone(fromPlan):null;
 state.page='editor';state.editor={mode:'create',sourceId:fresh?.id||'',name:fresh?fresh.name+'（副本）':'',note:fresh?fresh.note:'',dateStart:'',dateEnd:'',owner:actor().name,ownerId:actor().id,createdBy:actor().id,status:'draft',target:fresh?.target||0,platforms:fresh?clone(fresh.platforms):[],categories:fresh?clone(fresh.categories):[],goals:fresh?clone(fresh.goals):{},batches:fresh?clone(fresh.batches).filter(b=>b.type!=='makeup').map((item,i)=>b('TEMP-'+(i+1),item.name,item.platform,clone(item.cats))):[],editedBatches:false};state.platformExpand=null;state.modal=null;state.drawer=null;render();window.scrollTo(0,0);}
function editPlan(id){const p=getPlan(id);if(!p||!['draft','ready'].includes(p.status)){if(p)detailPlan(id);return;}
 state.page='editor';state.editor={...clone(p),mode:'edit',sourceId:id,editedBatches:false};editorBaseline=JSON.stringify(state.editor);state.platformExpand=null;state.modal=null;state.drawer=null;render();window.scrollTo(0,0);}
const isGoalLocked=e=>e.mode==='edit'&&(e.status==='running'||e.batches.some(b=>b.locked));
const batchEditable=(e,b)=>e.status!=='ended'&&e.status!=='stopping'&&!b.locked&&!['running','ended','cancelled','stopping'].includes(b.phase);
function canReorderBatch(e,from,to){
 if(!e||!Number.isInteger(from)||!Number.isInteger(to)||from===to||Math.min(from,to)<0||Math.max(from,to)>=e.batches.length)return false;
 // Locked or executed rows keep their positions; moves stay inside one editable segment.
 return e.batches.slice(Math.min(from,to),Math.max(from,to)+1).every(batch=>batchEditable(e,batch));
}
function reorderBatch(e,from,to){
 if(!canReorderBatch(e,from,to))return false;
 const [batch]=e.batches.splice(from,1);e.batches.splice(to,0,batch);e.editedBatches=true;
 return true;
}

const numBatches=e=>e.batches.filter(b=>b.type!=='makeup');
const perCellActual=(e,p,c)=>numBatches(e).reduce((s,b)=>s+(b.platform===p?(b.cats[c]||0):0),0);
const batchValidation=e=>{
 const issues=[];
 const quantities=[e.target,...Object.values(e.goals||{}).flatMap(Object.values),...e.batches.flatMap(b=>Object.values(b.cats))];
 if(quantities.some(n=>!Number.isSafeInteger(n)||n<0)||!Number.isSafeInteger(e.target))issues.push('目标与批次数量必须为非负整数');
 if(ownerIssue(e))issues.push(ownerIssue(e));
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
 const section=(num,title,desc,body,aux='')=>`<section class="surface ex-panel section" id="step-0${num}"><div class="section-hd"><div class="section-hd-left"><span class="sect-num">0${num}</span><div><h2 class="sect-title">${title}${desc?fieldHelp(title,desc):''}</h2></div></div>${aux}</div><div class="section-body">${body}</div></section>`;
 return `<div class="page-head ex-page-head"><h1 class="page-h1">${e.mode==='create'?'新建发布计划':'编辑发布计划'}</h1></div>
 ${section(1,'基础信息','',`<div id="plan-basics"></div>`)}
 ${section(2,'资源盘点','资源盘点用于容量预估，不锁定账号或成片；同一账号的额度在各类目间共享。',renderInventory(e),`<button class="btn sm" data-act="refresh-inventory">${$icon('refresh',14)}重新盘点</button>`)}
 ${section(3,'目标设定','按平台与类目设置目标。容量受平台周期额度、类目兼容性和账号共享额度约束；批次拆解只分解目标。',renderGoalEditor(e,lockGoal),lockGoal?`<span class="badge orange">${$icon('lock',12)} 目标配置已锁定</span>`:'')}
 ${section(4,'批次拆解','规则 / AI 推荐生成初始批次，运营可以调整未锁定批次。',renderBatchEditor(e))}
 <div class="editor-footer"><div class="foot-note">${issues.length?`${$icon('alert',14)} 尚有 ${issues.length} 项配置需要确认`:`${$icon('check-circle',14)} 目标与批次分配已匹配`}</div><div class="buttons"><button class="btn" data-act="${e.mode==='edit'?'detail':'list'}" ${e.mode==='edit'?`data-id="${E(e.sourceId)}"`:''}>返回</button>${e.mode==='create'?`<button class="btn secondary" data-act="save-draft">保存草稿</button><button class="btn primary" data-act="confirm-plan">创建计划</button>`:`<button class="btn primary" data-act="save-edit">保存</button>`}</div></div>`;
}
function renderInventory(e){return `<div class="stat-grid">
 ${[['可用成片',PLATFORM_DATA.reduce((n,p)=>n+p.videos,0),'条',''],['可用账号',PLATFORM_DATA.reduce((n,p)=>n+p.accounts,0),'个',''],['覆盖平台',PLATFORM_DATA.length,'个','支持多平台选择'],['覆盖类目',CATS.length,'类','按账号与成片归类'],['预计发布容量',PLATFORM_DATA.reduce((n,p)=>n+p.capacity,0),'条','']].map(([a,n,u,d])=>`<div class="stat-card"><div class="stat-label">${a}</div><div class="stat-number">${n}<span class="unit">${u}</span></div><div class="stat-assist">${d}</div></div>`).join('')}</div>
 <div class="section-split"><div class="subheading">平台资源分布 <span style="font-size:12px;font-weight:500;color:var(--wt-color-text-assist)"></span></div><span class="meta-info">盘点时间 ${E(state.inventoryAt||new Date().toLocaleString('sv-SE').slice(0,16))}</span></div>
 <div class="table-wrap resource-data-table resource-embedded-table is-editable"><table class="table resource-table"><thead><tr><th>平台</th><th class="resource-table-number">可用成片</th><th class="resource-table-number">可用账号</th><th class="resource-table-number">覆盖类目</th><th class="resource-table-number">预计可发布量</th><th>操作</th></tr></thead><tbody>${PLATFORM_DATA.map(p=>`<tr><td><b>${E(p.name)}</b></td><td class="resource-table-number">${p.videos} 条</td><td class="resource-table-number">${p.accounts} 个</td><td class="resource-table-number">${p.categories} 类</td><td class="resource-table-number"><b>${p.capacity}</b> 条 <span class="badge-inline">周期容量</span></td><td><button class="btn-link" data-act="toggle-platform" data-platform="${E(p.name)}">${state.platformExpand===p.name?'收起类目':'查看类目'} ${$icon(state.platformExpand===p.name?'chevron-down':'chevron-right',12)}</button></td></tr>${state.platformExpand===p.name?`<tr><td colspan="6" style="padding:0 14px"><div class="resource-details"><div style="font-size:12px;font-weight:750;margin-bottom:10px">${p.name} · 平台类目容量（账号额度共享，不能直接相加）</div><div style="display:grid;grid-template-columns:repeat(3,1fr);gap:10px">${CATS.filter(c=>p.cell[c]>0).map(c=>`<div style="padding:10px 12px;background:var(--wt-color-text-inverse);border:1px solid var(--wt-color-border);border-radius:7px"><div class="subtext">${c}</div><div style="font-size:14px;font-weight:760;margin-top:5px">预计可发 ${p.cell[c]} 条</div></div>`).join('')}</div></div></td></tr>`:''}`).join('')}</tbody></table></div>
`;}
async function changeGoalScope(key,values) {
 const e=state.editor;if(!e||isGoalLocked(e)||!['platforms','categories'].includes(key))return;
 const allowed=key==='platforms'?PLATFORM_DATA.map(p=>p.name):CATS;
 const next=[...new Set(values)].filter(value=>allowed.includes(value));
 const removed=e[key].filter(value=>!next.includes(value));
 const impact=removed.some(value=>key==='platforms'?sumVals(e.goals[value])>0:e.platforms.some(p=>(e.goals[p]?.[value]||0)>0));
 if(impact && !(await adapters.confirmScope('移除范围内的目标与批次分配将被清除，其他已配置内容保留。')))return;
 if(state.editor!==e || isGoalLocked(e))return;
 e[key]=next;
 const goals={};for(const p of e.platforms){goals[p]={};for(const c of e.categories)goals[p][c]=e.goals[p]?.[c]||0;}e.goals=goals;
 e.batches=e.batches.filter(batch=>e.platforms.includes(batch.platform)).map(batch=>({...batch,cats:Object.fromEntries(Object.entries(batch.cats).filter(([category])=>e.categories.includes(category)))})).filter(batch=>sumBatch(batch)>0);
 e.editedBatches=e.batches.length>0;render();
}
function renderGoalEditor(e,locked){const matrixSum=sumMatrix(e.goals);
 const sumForP=p=>sumVals(e.goals[p]);
 return `${locked?`<div class="plan-locked m-bottom16">${$icon('lock',14)} 计划已发生资源锁定或执行。平台、类目与总目标保持不变；仍可调整尚未锁定批次的拆分和顺序，但不能破坏既定目标。</div>`:''}
 <div id="plan-goal-scope"></div><div style="display:grid;grid-template-columns:1fr;gap:18px">
 <div class="form-row"><div class="form-field" style="width:210px"><label class="field-label">计划发布总目标<span class="req">*</span></label><input class="input" type="number" min="1" data-editor="target" value="${e.target}" ${locked?'disabled':''}/></div>
 <div class="form-field"><label class="field-label">目标矩阵${fieldHelp('目标矩阵','只设定一个计划总目标，平台和类目的目标由下方矩阵自动汇总。编辑任意单元格时，其他单元格数量不会自动被修改。')}</label><button class="btn secondary" data-act="generate-matrix" ${locked?'disabled':''}>${$icon('grid',14)}生成目标分配表</button></div></div></div>
 <div class="section-split" style="margin:26px 0 12px"><div class="subheading">平台 × 类目目标分配矩阵</div><span class="meta-info">单元格仅展示对应组合的目标数量</span></div>
 ${e.platforms.length&&e.categories.length?`<div class="table-wrap resource-data-table resource-embedded-table is-editable"><table class="table matrix"><thead><tr><th style="min-width:120px">平台 / 类目</th>${e.categories.map(c=>`<th style="min-width:120px">${E(c)}</th>`).join('')}<th class="resource-table-number">平台合计</th><th class="resource-table-number">平台容量</th></tr></thead><tbody>${e.platforms.map(p=>`<tr><td><b>${E(p)}</b></td>${e.categories.map(c=>{let cap=pd(p)?.cell[c]||0;return `<td class="resource-table-number">${cap===0?`<span class="subtext" title="当前平台不支持此类目">—</span>`:locked?`<span class="read-only-cell">${e.goals[p]?.[c]||0} ${$icon('lock',11)}</span>`:`<input class="input" type="number" min="0" data-matrix="1" data-platform="${E(p)}" data-category="${E(c)}" value="${e.goals[p]?.[c]||0}" title="预计上限 ${cap} 条"/>`}<div class="subline" style="margin-top:3px">${cap?'可发≤'+cap:'不可发布'}</div></td>`}).join('')}<td><b>${sumForP(p)}</b></td><td><span class="badge ${sumForP(p)>pd(p).capacity?'red':'gray'}">≤ ${pd(p).capacity}</span></td></tr>`).join('')}<tr class="matrix-total"><td>类目合计</td>${e.categories.map(c=>`<td>${e.platforms.reduce((s,p)=>s+(e.goals[p]?.[c]||0),0)}</td>`).join('')}<td><b>${matrixSum}</b></td><td>目标 ${e.target}</td></tr></tbody></table>
 <div class="matrix-foot"><div class="${matrixSum===e.target?'valid':'invalid'}">${$icon(matrixSum===e.target?'check-circle':'alert',15)} ${matrixSum===e.target?'目标分配平衡':matrixSum<e.target?`仍有 ${e.target-matrixSum} 条未分配`:`超出总目标 ${matrixSum-e.target} 条`}</div><span class="subtext">平台目标和类目目标由矩阵自动汇总</span></div></div>`:`<div class="empty" style="border:1px dashed var(--line)">请先选择至少一个平台和一个类目</div>`}
`;
}
function renderBatchEditor(e){const edited=e.editedBatches;const issues=batchValidation(e);const locked=e.batches.filter(b=>b.locked).length;
 const headers=e.categories.map(c=>`<th class="resource-table-number" style="min-width:85px">${E(c)}</th>`).join('');
 return `
 ${locked?`<div class="plan-locked m-bottom16">${$icon('lock',14)} 当前有 ${locked} 个批次已锁定资源。为避免破坏账号、成片映射，已锁定行不可调整；未锁定批次允许按剩余目标进行拆分、合并、数量调整与排序。</div>`:''}
 <div class="section-split"><div class="subheading">批次规划表 <span class="meta-info">共 ${e.batches.length} 批 · 严格串行</span></div><div class="resource-form-actions" style="display:flex;gap:8px"><button class="btn secondary sm" data-act="recommend" ${locked?'disabled title="已有资源锁定，不可整体重新推荐"':''}>${$icon('sparkles',13)}AI 推荐</button><button class="btn sm" data-act="add-batch">${$icon('plus',13)}新增批次</button></div></div>
 <div class="batches-table table-wrap resource-data-table resource-embedded-table is-editable"><table class="table ex-table"><thead><tr><th style="min-width:85px">顺序</th><th style="min-width:174px">批次名称</th><th style="min-width:120px">平台</th>${headers}<th class="resource-table-number">批次合计</th><th>资源状态</th><th style="min-width:160px">操作</th></tr></thead><tbody>${e.batches.map((b,i)=>{const can=batchEditable(e,b);const prev=e.batches[i-1];const samePrev=prev&&!prev.locked&&can&&prev.platform===b.platform&&prev.type===b.type;
  return `<tr data-plan-batch-row="${E(b.id)}"><td><div class="batch-order"><button type="button" class="plan-batch-drag-handle" data-plan-batch-handle="${E(b.id)}" draggable="${can}" aria-label="${E(can?'调整'+b.name+'顺序':b.name+'顺序不可调整')}" title="${can?'拖拽排序，也可使用上下方向键':b.locked?'已锁定，无法拖拽':'当前批次不可调整顺序'}" ${can?'':'disabled'}>${$icon('grip',16)}</button><span>${String(i+1).padStart(2,'0')}</span></div></td>
   <td>${can?`<input class="input batch-name-input" value="${E(b.name)}" data-batch-name="${i}"/>`:`<div style="font-weight:680">${E(b.name)}</div>`}</td>
   <td>${can?`<select class="select small batch-platform" data-batch-platform="${i}">${e.platforms.map(p=>`<option value="${p}" ${p===b.platform?'selected':''}>${p}</option>`).join('')}</select>`:`<b>${E(b.platform)}</b>`}</td>
   ${e.categories.map(c=>`<td class="resource-table-number">${(pd(b.platform)?.cell[c]||0)===0?`<span class="subtext">—</span>`:can?`<input type="number" class="input batch-count" data-batch-cell="1" data-idx="${i}" data-category="${E(c)}" min="0" value="${b.cats[c]||0}"/>`:`<b>${b.cats[c]||0}</b>`}</td>`).join('')}
   <td class="resource-table-number"><b>${sumBatch(b)}</b> 条${b.type==='makeup'?'<div class="subtext">补量</div>':''}</td><td>${b.locked?`<span class="badge purple">${$icon('lock',11)}已锁定</span>`:`<span class="badge gray">未锁定</span>`}</td>
   <td><div class="inline-actions ex-row-actions">${can?`<button data-act="split-batch" data-idx="${i}">拆分</button>${samePrev?`<button data-act="merge-batch" data-idx="${i}">合并上批</button>`:''}<button class="red" data-act="remove-batch" data-idx="${i}">删除</button>`:`<span class="subtext">${$icon('lock',12)} 不可调整</span>`}</div></td></tr>`}).join('')}</tbody></table>
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
function detailPlan(id){const p=getPlan(id);if(!p)return;if(state.page==='list')listScroll=window.scrollY||0;state.page='detail';state.planId=id;state.editor=null;state.modal=null;state.drawer=null;render();window.scrollTo(0,0);}
function actualByCell(p,pl,cat){return p.batches.reduce((s,b)=>{
 if(b.platform!==pl||b.success<=0)return s;
 if(b.successCats)return s+(b.successCats[cat]||0);
 const num=b.cats[cat]||0;const total=sumBatch(b);if(!total)return s;
 const cats=Object.keys(b.cats).filter(c=>b.cats[c]>0);let remaining=b.success;
 const vals={};for(let i=0;i<cats.length;i++){let k=cats[i];let n=i===cats.length-1?remaining:Math.min(b.cats[k],Math.floor(b.success*b.cats[k]/total));remaining-=n;vals[k]=n;}
 return s+(vals[cat]||0);
 },0)}
// Projection only: metrics and action eligibility remain owned by this model.
function detailView(p) {
 const actual=successOf(p),rate=Math.round(actual/Math.max(1,p.target)*100),gap=Math.max(0,p.target-actual);
 const current=currentBatch(p),pending=openTasks(p).filter(t=>t.status==='pending'),next=firstNotEnded(p);
 const action=(key,label,batch,extra={})=>({key,label,batch,...extra});
 let notice=null;
 if(p.status==='running'){
  if(pending.length)notice={tone:'warning',title:`有待处理任务 · ${pending.length} 项待办`,actions:[action('view-batch','查看待处理批次',pending[0].batchId),action('workbench','进入处理工作区',pending[0].batchId)]};
  else if(allEnded(p)&&gap>0)notice={tone:'warning',title:`全部批次已结束，计划目标尚差 ${gap} 条`,text:`目标 ${p.target} 条，实际发布成功 ${actual} 条。请核实最终结果后，在页底决定补量或确认结束。`};
  else if(next?.phase==='ready')notice={tone:'info',title:`${next.name} · 待人工确认执行`,text:'当前批次资源与内容配置完毕，请检查目标和资源后在页底确认执行；不会自动启动。',actions:[action('view-batch','查看批次',next.id)]};
  else if(next?.phase==='draft')notice={tone:'info',title:`${next.name} · 待配置`,text:'请先前往对应批次完成账号、成片与发布内容配置，再人工确认执行。',actions:[action('view-batch','查看批次并配置',next.id)]};
 }
 if(p.status==='ready')notice={tone:'info',title:'计划已确认，等待首批次执行',text:'批次按列表顺序串行推进，执行前需要人工确认。当前无发布成功记录。',actions:[action('view-batch','查看首批次',p.batches[0]?.id)]};
 if(p.status==='stopping')notice={tone:'warning',title:'终止指令已生效，正在等待当前任务收敛',text:'未启动批次已被阻止执行。请核实可能已经在平台提交的任务；待结果收敛后自动结束计划。',actions:openTasks(p).length?[action('workbench','处理待核实任务',current?.id)]:[]};
 const footer=[];
 if(p.status==='running'&&allEnded(p)&&gap>0){footer.push(action('end-plan','确认结束'));footer.push(action('makeup','创建补量批次',null,{primary:true}));}
 else if(['ready','running'].includes(p.status)&&!executionIssue(p,next))footer.push(action('execute','执行',next.id,{primary:true}));
 if(adapters.loadPlans){
  footer.length=0;
  const active=p.status==='ready'?next:p.batches[p.position],following=p.batches[p.position+1];
  notice=null;
  if(p.control==='等待继续确认'||p.control==='等待跳过确认'){
   const skip=p.control==='等待跳过确认';notice={tone:'warning',title:skip?'当前批次没有可计算的发布结果':'当前批次失败率超过 10%',actions:[action('workbench','去处理',active?.id)]};footer.push(action('continue-plan',skip?'确认跳过当前批次':'确认进入下一批',null,{primary:true}));
  }else if(p.control==='等待配置')notice={tone:'info',title:'下一批待配置',actions:[action('view-batch','去配置',following?.id)]};
  else if(p.control==='已暂停'){notice={tone:'info',title:active?.type==='makeup'?'补量批次待执行':'计划已暂停',actions:active?[action('view-batch',active.phase==='draft'?'去配置':'查看批次',active.id)]:[]};if(active?.phase==='ready'||active?.phase==='ended')footer.push(action('resume-plan',active?.type==='makeup'?'执行':'恢复计划',null,{primary:true}));}
  else if(p.status==='ready'){notice={tone:'info',title:'等待启动计划',actions:active?[action('view-batch',active.phase==='draft'?'配置首批':'查看首批',active.id)]:[]};if(active?.phase==='ready'&&!executionIssue(p,active))footer.push(action('execute','启动计划',active.id,{primary:true}));}
  else if(p.control==='执行中'||(p.status==='running'&&p.control==='规划中')){const destination=pending[0]?.batchId||active?.id;notice={tone:pending.length?'warning':'info',title:pending.length?'有待处理任务':p.control==='执行中'?'按批次顺序自动发布':'批次处理中',actions:destination?[action('workbench',pending.length?'查看待处理批次':'查看当前批次',destination)]:[]};}
  if(['执行中','等待配置'].includes(p.control))footer.push(action('pause-plan','暂停计划'));
  if(p.control==='执行已结束'&&p.status!=='ended'){
   notice={tone:p.canSettle?'warning':'info',title:p.canSettle?`本轮执行已结束，目标缺口 ${gap} 条`:'本轮执行已结束，等待结果核实',actions:pending[0]?[action('workbench','去处理',pending[0].batchId)]:[]};
   if(p.canSettle&&gap>0){footer.push(action('end-plan','确认结束'));footer.push(action('makeup','创建补量批次',null,{primary:true}));}
  }
  if(p.status==='stopping')notice={tone:'warning',title:'计划已终止，等待在途任务核实',actions:pending[0]?[action('workbench','去处理',pending[0].batchId)]:[]};
  if(p.legacyReadOnly){notice={tone:'info',title:'旧版执行记录'};footer.length=0;}
 }
 const tone=s=>({running:'info',ready:'info',stopping:'warning',ended:'neutral',cancelled:'neutral'}[s]||'neutral');
 return {actual,rate,gap,notice,footer,statusLabel:nameStatus(p.status),statusTone:p.status==='ended'&&p.result==='achieved'?'success':tone(p.status),resultLabel:resultText(p.result),originalBatches:p.batches.filter(b=>b.type!=='makeup').length,hasMakeup:p.batches.some(b=>b.type==='makeup'),sumTargets:sumMatrix(p.goals),
  stats:[{key:'target',label:'计划目标',value:p.target,unit:'条'},{key:'success',label:'发布成功',value:actual,unit:'条',tone:'success'},{key:'gap',label:'目标缺口',value:gap,unit:'条',description:'待核实结果不计入已发布'},{key:'batches',label:'批次进度',value:`${endedCount(p)} / ${p.batches.length}`,unit:'批',description:'已结束 / 全部批次'}],
  cells:p.platforms.flatMap(platform=>p.categories.filter(c=>(p.goals[platform]?.[c]||0)>0).map(category=>{const target=p.goals[platform][category],actual=actualByCell(p,platform,category);return {platform,category,target,actual,gap:Math.max(0,target-actual),rate:Math.round(actual/Math.max(1,target)*100)};})),
  batches:p.batches.map(batch=>{const count=(batch.tasks||[]).filter(t=>['pending','retrying'].includes(t.status)).length,executable=canExecute(p,batch);return {batch,count,executable,target:sumBatch(batch),targets:fmtTargets(batch),statusLabel:nameBatchStatus(batch.phase),statusTone:batch.phase==='ended'?'success':tone(batch.phase),pendingLabel:count?`待处理 ${count}`:batch.phase==='ready'&&executable?'待确认执行':batch.phase==='draft'?'待配置':batch.phase==='draft'?'待配置':'—'};})};
}
function renderDetail(){return '<div id="plan-detail"></div>';}
function executionIssue(p,b){
 if(!p||!b)return '暂无可执行批次';
 if(!['ready','running'].includes(p.status))return '当前计划状态不可执行';
 if(openTasks(p).length)return '请先处理当前批次待办';
 if(!canExecute(p,b))return '请等待前序批次结束';
 if(b.phase!=='ready'||!b.configured)return '请先完成批次配置';
 if(!b.locked)return '请先锁定批次资源';
 return '';
}
function canExecute(p,b){if(!p||!b||openTasks(p).length)return false;if(p.status!=='running'&&p.status!=='ready')return false;const i=p.batches.indexOf(b);if(i<0)return false;if(p.batches.some(x=>['running','stopping'].includes(x.phase)))return false;return i===0||p.batches.slice(0,i).every(x=>['ended','cancelled'].includes(x.phase));}
function renderDrawer(){if(!state.drawer)return '';const d=state.drawer,p=getPlan(d.planId);if(!p)return '';const b=p.batches.find(x=>x.id===d.batchId);
 if(d.type==='workbench'&&b){const tasks=b.tasks||[],filtered=tasks.filter(t=>['pending','retrying'].includes(t.status)&&(state.taskFilter==='all'||t.type===state.taskFilter));return `<dialog class="plan-drawer"><div class="drawer"><div class="drawer-hd"><div><div class="drawer-title">批次异常工作区</div><div class="subtext m-top12">${E(b.name)} · ${E(b.platform)} · ${tasks.filter(t=>['pending','retrying'].includes(t.status)).length} 项待办</div></div><button class="btn icon-btn" data-act="close-drawer">${$icon('close',17)}</button></div>
 <div class="drawer-body"><div class="hint orange m-bottom16">${$icon('alert',17)}<div>计划继续保持原主状态。人工待办的结果以单条任务为准，处理后自动更新批次和计划进度。不要直接将整批标记为成功。</div></div>
 <div class="filter-seg" style="margin-bottom:18px">${[['all','全部待办'],['manual','待人工提交'],['verify','待核实'],['failed','发布失败']].map(([k,l])=>`<button class="seg-btn ${state.taskFilter===k?'active':''}" data-act="task-filter" data-value="${k}">${l}</button>`).join('')}</div>
 ${filtered.length?filtered.map(t=>`<div class="task-card"><div style="max-width:62%"><div class="task-title">${E(t.id)} · ${E(t.title)}</div><div class="task-desc">${E(t.account)}<br>${E(t.description)}</div><span class="badge ${t.type==='failed'?'red':'orange'}" style="margin-top:8px">${t.status==='retrying'?'重试中，等待结果':t.type==='manual'?'等待人工提交':t.type==='verify'?'发布结果待核实':'发布失败'}</span></div>
 <div class="task-actions"><button class="btn sm ${t.type==='failed'?'':'primary'}" ${t.status==='retrying'?'disabled':''} data-act="task-action" data-id="${p.id}" data-batch="${b.id}" data-task="${t.id}">${t.status==='retrying'?'重试中':t.type==='manual'?'去确认':t.type==='verify'?'核实结果':'去处理'}</button></div></div>`).join(''):`<div class="empty" style="padding:30px">${$icon('check-circle',28)}<p>当前筛选下没有待处理任务</p></div>`}
 </div><div class="drawer-foot"><button class="btn" data-act="close-drawer">关闭</button><button class="btn secondary" data-act="view-batch" data-id="${p.id}" data-batch="${b.id}">查看批次详情</button></div></div></dialog>`;}
 if(d.type==='batch'&&b){const idx=p.batches.indexOf(b),can=canExecute(p,b),pending=(b.tasks||[]).filter(x=>['pending','retrying'].includes(x.status)).length;return `<dialog class="plan-drawer"><div class="drawer"><div class="drawer-hd"><div><div class="drawer-title">${E(b.name)}</div><div class="subtext m-top12">${E(b.id)} · ${E(b.platform)} · 顺序 ${String(idx+1).padStart(2,'0')}</div></div><button class="btn icon-btn" data-act="close-drawer">${$icon('close',17)}</button></div>
 <div class="drawer-body"><div class="detail-meta m-bottom16">${bBadge(b)}${b.locked?'<span class="badge purple">资源已锁定</span>':'<span class="badge gray">资源未锁定</span>'}${b.type==='makeup'?'<span class="badge purple">补量批次</span>':''}</div>
 <div class="modal-kv"><div><div class="label">批次平台</div><div class="value">${E(b.platform)}</div></div><div><div class="label">目标数量</div><div class="value">${sumBatch(b)} 条</div></div><div><div class="label">发布成功</div><div class="value">${b.success} 条</div></div><div><div class="label">执行顺序</div><div class="value">第 ${idx+1} 批</div></div></div>
 <div class="section-split" style="margin-top:24px"><div class="subheading">类目目标</div></div><div style="border:1px solid var(--line);border-radius:8px">${Object.entries(b.cats).filter(([,n])=>n>0).map(([c,n])=>`<div style="padding:13px 16px;border-bottom:1px solid var(--line);display:flex;justify-content:space-between"><span>${E(c)}</span><b>${n} 条</b></div>`).join('')}</div>
 <div class="section-split" style="margin-top:25px"><div class="subheading">资源配置和执行约束</div></div>
 ${b.locked?`<div class="hint green">${$icon('lock',17)}<div>该批次资源已锁定，账号与成片映射不可重新调整。${b.phase==='ready'?'内容配置已完成，等待人工执行。':'执行历史与发布结果保持不可变。'}</div></div>`:`<div class="hint">${$icon('info',17)}<div>该批次尚未完成资源匹配。请进入「发布批次」完成资源配置和内容配置，锁定后不允许重新映射。</div></div>`}
 ${pending?`<div class="hint orange m-top16">${$icon('alert',15)}${pending} 个任务需要运营介入处理</div>`:''}
 <a class="btn-link" href="/example/operations/publishing/batches">进入发布批次模块</a>
 </div><div class="drawer-foot"><button class="btn" data-act="close-drawer">关闭</button>${pending?`<button class="btn primary" data-act="workbench" data-id="${p.id}" data-batch="${b.id}">去处理 ${pending} 项</button>`:''}${b.phase==='ready'&&can?`<button class="btn primary" data-act="execute" data-id="${p.id}" data-batch="${b.id}">${$icon('play',15)}执行</button>`:''}</div></div></dialog>`;}
 if(d.type==='makeup'){const gaps=[];for(const pl of p.platforms)for(const c of p.categories){const n=(p.goals[pl]?.[c]||0)-actualByCell(p,pl,c);if(n>0)gaps.push({pl,c,n})}
 const availablePlatforms=[...new Set(gaps.map(x=>x.pl))];const selected=d.platform||availablePlatforms[0];d.platform=selected;const cells=gaps.filter(x=>x.pl===selected);
 return `<dialog class="plan-drawer"><div class="drawer"><div class="drawer-hd"><div><div class="drawer-title">创建补量批次</div><div class="subtext m-top12">计划目标固定 · 仅针对未达成的平台 × 类目目标新增尝试</div></div><button class="btn icon-btn" data-act="close-drawer">${$icon('close',17)}</button></div>
 <div class="drawer-body"><div class="hint orange m-bottom16">${$icon('info',16)}<div>原计划目标仍为 ${p.target} 条，已发布 ${successOf(p)} 条。补量属于额外发布尝试，不覆盖原始批次规划或已发布记录。</div></div>
 <div class="form-field m-bottom16"><label class="field-label">补量平台（单选，每批仅一个平台）</label><select class="select" data-makeup="platform">${availablePlatforms.map(n=>`<option value="${n}" ${n===selected?'selected':''}>${n}</option>`).join('')}</select></div>
 <div class="form-field m-bottom16"><label class="field-label">批次名称</label><input class="input" data-makeup="name" value="${E(d.name||`补量批次 · ${selected}`)}"/></div>
 <div class="subheading m-bottom16">目标缺口明细</div><div class="table-wrap resource-data-table resource-embedded-table is-editable is-flush"><table class="table ex-table resource-numeric-columns"><thead><tr><th>类目</th><th class="resource-table-number">目标缺口</th><th class="resource-table-number">本次补量数量</th></tr></thead><tbody>${cells.map(x=>`<tr><td>${E(x.c)}</td><td>${x.n} 条</td><td><input class="input" style="width:96px" type="number" min="0" max="${x.n}" data-makeup-cell="${E(x.c)}" value="${d.values?.[x.c]??x.n}"/></td></tr>`).join('')}</tbody></table></div>
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
 const e=state.editor;title=state.editor?.mode==='edit'?'确认发布计划':'确认创建发布计划';content=`<p class="modal-text" style="margin-top:0">确认后保存计划及批次规划记录。此时仅创建目标和批次，不锁定账号或成片，也不会自动执行发布。</p><div class="modal-kv"><div><div class="label">计划名称</div><div class="value">${E(e.name)}</div></div><div><div class="label">计划目标</div><div class="value">${e.target} 条</div></div><div><div class="label">平台 / 类目</div><div class="value">${e.platforms.length} 个 / ${e.categories.length} 类</div></div><div><div class="label">规划批次</div><div class="value">${e.batches.length} 批</div></div></div><div class="hint green m-top16">${$icon('check-circle',16)}目标矩阵、资源容量与全部批次合计校验通过。</div>`;footer=cancelBtn+okBtn('确认创建',state.editor?.mode==='edit'?'do-confirm-draft':'do-confirm-plan');
 }else if(m.type==='cancel-plan'){
 title='终止发布计划';const running=p.batches.filter(x=>x.phase==='running').length;const other=p.batches.filter(x=>['draft','ready'].includes(x.phase)).length;
 content=`<p class="modal-text" style="margin-top:0">将终止「${E(p.name)}」及其全部未结束批次，取消尚未启动的任务；已提交的任务仍需核实结果，已有发布结果与执行记录保留。</p><div class="modal-kv"><div><div class="label">当前执行中批次</div><div class="value">${running} 批</div></div><div><div class="label">未执行批次</div><div class="value">${other} 批</div></div><div><div class="label">已成功发布</div><div class="value">${successOf(p)} 条（保留）</div></div><div><div class="label">未完成任务</div><div class="value">${openTasks(p).length} 个待人工处理</div></div></div><div class="hint orange m-top16">${$icon('alert',17)}若当前任务仍在上传或提交，须等待最终结果收敛后再进入已结束；否则可以一次性同步终止完成。</div><div class="form-field m-top16"><label class="field-label">终止原因<span class="req">*</span></label><textarea rows="3" class="input" data-modal-field="reason" placeholder="请填写终止计划的原因">${E(m.reason||'')}</textarea></div>`;
 footer=cancelBtn+dangerBtn('确认终止计划','do-cancel-plan');
 }else if(m.type==='delete-plan'){
 title='删除计划草稿';content=`<p class="modal-text">确认删除草稿「${E(p.name)}」？删除后不可恢复。已确认或已执行计划不提供删除操作。</p>`;footer=cancelBtn+dangerBtn('确认删除','do-delete-plan');
 }else if(m.type==='execute-batch'){
 title=adapters.loadPlans?'确认启动发布计划':'确认执行发布批次';const prev=p.batches[p.batches.indexOf(b)-1];content=`<p class="modal-text" style="margin-top:0">即将启动「${E(b.name)}」。执行后该批次内容配置不可再修改，原资源映射保持锁定。</p><div class="modal-kv"><div><div class="label">发布平台</div><div class="value">${E(b.platform)}</div></div><div><div class="label">发布目标</div><div class="value">${sumBatch(b)} 条</div></div><div><div class="label">匹配账号</div><div class="value">${b.accountCount||pd(b.platform).accounts} 个</div></div><div><div class="label">发布任务</div><div class="value">${sumBatch(b)} 个</div></div></div><div class="subheading m-top16 m-bottom16">执行前校验</div><ul class="modal-steps">${[
 '前序批次已结束'+(prev?`（${prev.success}/${sumBatch(prev)} 条）`:''),
 '当前无其他批次执行中',
 '目标和批次分配已通过结构校验',
 '账号、成片和执行环境将在批次启动前核验'
 ].map(s=>`<li>${$icon('check-circle',16)} ${E(s)}</li>`).join('')}</ul><div class="hint m-top16">${$icon('info',16)}此确认属于批次执行动作，不会创建新的计划状态；计划首次执行时自动进入「进行中」。</div>`;
 if(adapters.loadPlans)content=`<p class="modal-text">启动「${E(p.name)}」，后续批次按 10% 失败率阈值自动衔接。</p>`;
 footer=cancelBtn+okBtn(adapters.loadPlans?'确认启动':'确认执行','do-execute-batch');wide=true;
 }else if(m.type==='split-batch'){
 const editor=state.editor,bt=editor.batches[m.index];title='拆分发布批次';const sum=sumBatch(bt),sumA=sumVals(m.values||{}),sumB=sum-sumA;
 content=`<div class="modal-text">原批次「${E(bt.name)}」目标 ${sum} 条。调整 A 的数量，B 自动取剩余量；每个平台 × 类目数量保持不变。</div><div class="table-wrap resource-data-table resource-embedded-table is-editable is-flush m-top16"><table class="table ex-table resource-numeric-columns"><thead><tr><th>类目</th><th class="resource-table-number">原数量</th><th class="resource-table-number">批次 A</th><th class="resource-table-number">批次 B</th></tr></thead><tbody>${Object.entries(bt.cats).filter(([,v])=>v>0).map(([c,n])=>`<tr><td>${E(c)}</td><td>${n}</td><td><input class="input" type="number" min="0" max="${n}" style="width:80px" data-split-cell="${E(c)}" value="${m.values[c]??0}"/></td><td>${n-(m.values[c]??0)}</td></tr>`).join('')}<tr class="matrix-total"><td>合计</td><td>${sum}</td><td>${sumA}</td><td>${sumB}</td></tr></tbody></table></div><div class="hint m-top16">${$icon('info',16)}拆分后新增一个未锁定批次，插入原批次的下一位置；后续顺序依次顺延。</div>`;
 footer=cancelBtn+okBtn('确认拆分','do-split-batch');wide=true;
 }else if(m.type==='delete-batch'){
 const bt=state.editor.batches[m.index];title='删除批次规划';content=`<p class="modal-text">确认删除「${E(bt.name)}」？该批次的目标数量将回到待分配区。必须重新分配数量才能确认或保存正式计划。</p>`;footer=cancelBtn+dangerBtn('确认删除','do-delete-batch');
 }else if(m.type==='overwrite-reco'){
 title='覆盖现有批次规划？';content=`<p class="modal-text">重新推荐会覆盖当前未锁定批次的人工调整。建议先核对平台 × 类目目标矩阵；已锁定资源的批次不允许整体重新推荐。</p>`;footer=cancelBtn+okBtn('确认重新推荐','do-recommend');
 }else if(m.type==='end-plan'){
 const g=Math.max(0,p.target-successOf(p));title='确认结束发布计划';content=`<p class="modal-text" style="margin-top:0">所有批次已经结束，但计划目标尚未完全达成。确认后计划将进入「已结束 · 未达成结束」，不再补量或执行。</p><div class="modal-kv"><div><div class="label">计划目标</div><div class="value">${p.target} 条</div></div><div><div class="label">最终发布成功</div><div class="value">${successOf(p)} 条</div></div><div><div class="label">目标缺口</div><div class="value">${g} 条</div></div><div><div class="label">结束结果</div><div class="value">未达成结束</div></div></div><div class="form-field m-top16"><label class="field-label">未达成原因 / 结果说明<span class="req">*</span></label><textarea rows="3" class="input" data-modal-field="reason" placeholder="请说明放弃补量的原因或最终执行结论">${E(m.reason||'')}</textarea></div>`;footer=cancelBtn+okBtn('确认结束','do-end-plan');
 }else if(m.type==='task-verify'){
 title='核实发布结果';content=`<p class="modal-text" style="margin-top:0">${E(m.taskTitle)} · 请根据平台实际状态确认最终结果。</p><div class="form-field"><label class="field-label">核实结论<span class="req">*</span></label><select class="select" data-modal-field="result"><option value="success" ${m.result!=='fail'?'selected':''}>确认已发布成功</option><option value="fail" ${m.result==='fail'?'selected':''}>确认发布失败</option></select></div><div class="form-field m-top16"><label class="field-label">成功发布链接<span class="req">*</span> <span class="subtext">（确认成功时必填）</span></label><input class="input" data-modal-field="url" value="${E(m.url||'')}" placeholder="https://平台作品链接"/></div><div class="form-field m-top16"><label class="field-label">核实说明</label><textarea rows="2" class="input" data-modal-field="comment" placeholder="补充核实依据">${E(m.comment||'')}</textarea></div>`;footer=cancelBtn+okBtn('保存核实结果','do-task-verify');
 }else if(m.type==='task-manual'){
 title=p.status==='stopping'?'核实人工任务结果':'人工确认发布';content=`<p class="modal-text" style="margin-top:0">${E(m.taskTitle)}。${p.status==='stopping'?'计划已请求终止，请勿新增提交。若已经提交请核实结果；尚未提交可确认放弃。':'请先在平台完成最终提交，再回填结果。不能仅凭页面已填写完成就认定发布成功。'}</p><div class="form-field"><label class="field-label">处理结果</label><select class="select" data-modal-field="result"><option value="success" ${m.result!=='pending'?'selected':''}>已发布成功，可提供链接</option><option value="pending" ${m.result==='pending'?'selected':''}>已提交，暂待核实</option></select></div><div class="form-field m-top16"><label class="field-label">发布成功链接（成功时必填）</label><input class="input" data-modal-field="url" value="${E(m.url||'')}" placeholder="https://平台作品链接"/></div>`;footer=cancelBtn+(p.status==='stopping'?`<button class="btn danger" data-act="do-task-abandon">确认尚未提交并放弃</button>`:'')+okBtn('确认结果','do-task-manual');
 }else if(m.type==='task-failed'){
 title='处理发布失败';content=`<p class="modal-text" style="margin-top:0">${E(m.taskTitle)} · 当前操作只作用于该发布任务，不直接改变计划主状态。</p><div class="hint orange">${$icon('alert',16)}系统检测到上传异常。可在确认资源可用后重试，或决定放弃本次发布。</div><div class="form-field m-top16"><label class="field-label">处理说明</label><textarea rows="2" class="input" data-modal-field="comment">${E(m.comment||'')}</textarea></div>`;footer=cancelBtn+`<button class="btn danger" data-act="do-task-abandon">确认放弃</button>`+(p.status==='stopping'?'':okBtn('重试发布','do-task-retry'));
 }else if(m.type==='leave-editor'){
 title='放弃未保存的修改？';content='<p class="modal-text">本次计划修改尚未保存，返回后将丢失。</p>';footer='<button class="btn secondary" data-act="close-modal">继续编辑</button>'+okBtn('放弃修改','discard-edit');
 }else if(m.type==='info'){
 title=m.title||'提示';content=`<p class="modal-text">${E(m.text)}</p>`;footer=okBtn('知道了','close-modal');
 }
 return `<dialog class="plan-modal"><div class="modal ${wide?'wide':''}"><div class="modal-head"><h3>${title}</h3><button class="btn icon-btn ghost" data-act="close-modal">${$icon('close',19)}</button></div><div class="modal-body">${content}</div><div class="modal-footer">${footer}</div></div></dialog>`;
}
function setToast(text,type='success'){if(type!=='error'&&!persistPrototype()){text='当前输入仍保留在页面，但本地保存失败，请勿关闭页面';type='error';}state.toast={text,type};render();if(toastTimer)clearTimeout(toastTimer);toastTimer=setTimeout(()=>{state.toast=null;render();},3300)}
function openModal(type,data={}){state.modal={type,...data};render();}
function runValidation(e){const problems=batchValidation(e);if(problems.length){openModal('validation',{issues:problems});return false;}return true;}
function planFromEditor(status){const e=state.editor;const id=e.mode==='edit'?e.sourceId:uniqueId('PL');
 return {id,name:e.name||'未命名发布计划',note:e.note,dateStart:e.dateStart,dateEnd:e.dateEnd,owner:e.owner,ownerId:e.ownerId,createdBy:e.createdBy||actor().id,status,result:'',target:e.target,platforms:clone(e.platforms),categories:clone(e.categories),goals:clone(e.goals),updated:new Date().toLocaleString('sv-SE').slice(0,16),created:new Date().toLocaleString('sv-SE').slice(0,16),logs:[[ new Date().toLocaleString('sv-SE').slice(0,16),status==='draft'?'保存计划草稿':'确认创建发布计划',actor().name]],batches:clone(e.batches).map((b,i)=>({...b,id:('BT-'+id.split('-').pop())+'-'+String(i+1).padStart(2,'0'),phase:'draft',success:0,locked:false,configured:false,executed:false,tasks:[],type:'base'}))};
}
function editorSaveDraft(){const e=state.editor;if(!e||e.mode!=='create')return;state.recoveryEditor={...clone(e),savedAt:new Date().toISOString()};if(!persistPrototype()){setToast('草稿保存失败，当前输入已保留','error');return;}state.modal=null;state.editor=null;state.page='list';setToast('草稿已保存');window.scrollTo(0,0);}
async function requestNewPlan(source=null){
 promoteLegacyPlan();
 if(state.recoveryEditor){
  const choice=await adapters.chooseDraft({label:'发布计划',savedAt:state.recoveryEditor.savedAt});
  if(!choice)return;
  if(choice==='continue'){state.editor=restoreCreation();state.page='editor';state.modal=null;render();window.scrollTo(0,0);return;}
  const previous=state.recoveryEditor;state.recoveryEditor=null;
  if(!persistPrototype()){state.recoveryEditor=previous;setToast('无法重新开始，原草稿已保留','error');return;}
 }
 newEditor(source);
}
function editorConfirm(){const e=state.editor;if(!e)return;if(!runValidation(e))return;openModal('confirm-plan');}
function commitPlan(){if(!state.editor||!runValidation(state.editor))return;const buffer=clone(state.editor),draft=state.recoveryEditor;const fresh=planFromEditor('ready');plans.unshift(fresh);state.recoveryEditor=null;state.modal=null;state.editor=null;state.page='detail';state.planId=fresh.id;if(!persistPrototype()){plans.shift();state.editor=buffer;state.recoveryEditor=draft;state.page='editor';setToast('创建失败，当前输入已保留','error');return;}setToast('发布计划已创建，批次规划已生成；尚未执行发布');window.scrollTo(0,0);}
function editorSaveEdit(){const e=state.editor,p=e&&getPlan(e.sourceId);if(!e||!p||!['draft','ready'].includes(p.status))return;if(ownerIssue(e))return openModal('validation',{issues:[ownerIssue(e)]});if(p.status!=='draft'&&!runValidation(e))return;
 const lockedOriginal=p.batches.filter(x=>x.locked||['running','ended','cancelled'].includes(x.phase));
 const changedLocked=lockedOriginal.some(x=>{const candidate=e.batches.find(b=>b.id===x.id);return !candidate||JSON.stringify(candidate)!==JSON.stringify(x)||e.batches.indexOf(candidate)!==p.batches.indexOf(x);});
 if(changedLocked)return openModal('validation',{issues:['已锁定或已执行批次不允许更换平台、类目数量或执行状态。请刷新并重试。']});
 if((p.status==='running'||p.batches.some(b=>b.locked))&&(e.target!==p.target||JSON.stringify(e.goals)!==JSON.stringify(p.goals)||JSON.stringify(e.platforms)!==JSON.stringify(p.platforms)||JSON.stringify(e.categories)!==JSON.stringify(p.categories)||e.dateStart!==p.dateStart||e.dateEnd!==p.dateEnd))return openModal('validation',{issues:['计划已开始执行，总目标与平台 × 类目目标矩阵不可修改。']});
 const original=clone(p);
 Object.assign(p,{name:e.name,note:e.note,owner:e.owner,ownerId:e.ownerId,dateStart:e.dateStart,dateEnd:e.dateEnd,target:e.target,platforms:clone(e.platforms),categories:clone(e.categories),goals:clone(e.goals),batches:clone(e.batches)});
 logPlan(p,'修改计划与未锁定批次配置');if(!persistPrototype()){Object.assign(p,original);setToast('保存失败，当前修改已保留','error');return;}state.editor=null;state.page='detail';state.planId=p.id;setToast('修改已保存，已锁定批次未被改变');window.scrollTo(0,0);
}
function commitDraft(){const e=state.editor,p=getPlan(e.sourceId);if(!e||!p||p.status!=='draft'||!runValidation(e))return;Object.assign(p,{name:e.name,note:e.note,dateStart:e.dateStart,dateEnd:e.dateEnd,owner:e.owner,ownerId:e.ownerId,target:e.target,platforms:clone(e.platforms),categories:clone(e.categories),goals:clone(e.goals),batches:clone(e.batches),status:'ready'});logPlan(p,'人工确认计划，批次规划已生成');state.editor=null;state.modal=null;state.page='detail';state.planId=p.id;setToast('计划已确认，进入待执行；批次不会自动启动');window.scrollTo(0,0);}
function findTask(m){const p=getPlan(m.planId),b=p?.batches.find(x=>x.id===m.batchId),t=b?.tasks.find(x=>x.id===m.taskId);return {p,b,t};}
function mustUrl(url){try{const u=new URL(url),h=u.hostname.toLowerCase();return u.protocol==='https:'&&!u.username&&!u.password&&h.includes('.')&&!/^\d+(\.\d+){3}$/.test(h)&&!h.endsWith('.local')&&!h.endsWith('.localhost')&&h!=='localhost'}catch{return false}}

function taskDone(m,st,extra={}){const {p,b,t}=findTask(m);
 if(!p||!b||!t||!['pending','retrying'].includes(t.status)||p.status==='ended')return;
 if(st==='success'&&(!mustUrl(extra.publishUrl)||b.success>=sumBatch(b))){setToast('请填写有效的 HTTPS 作品链接后再核实成功','error');return;}
 if(st==='retrying'&&p.status==='stopping'){setToast('终止中的计划不可重新发布','error');return;}
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
 if(adapters.loadPlans){
  if(['view-batch','workbench'].includes(act)){const b=p?.batches.find(b=>b.id===el.dataset.batch);if(b)adapters.navigate(routes.operationURL('batches',{batchId:b.id,pending:act==='workbench',fromPlan:p.id}));return false;}
  const operations={'do-execute-batch':'execute','continue-plan':'continue','resume-plan':'resume','pause-plan':'pause','do-cancel-plan':'cancel','do-end-plan':'finish','create-makeup':'makeup'};
  if(operations[act]){
   if(act==='do-cancel-plan'&&!state.modal?.reason?.trim()){setToast('请填写终止原因','error');return false;}
   const draft=state.drawer,pid=id||state.modal?.planId||draft?.planId,plan=getPlan(pid);
   const value=act==='create-makeup'?{platform:draft.platform,name:draft.name||`补量批次 · ${draft.platform}`,cats:Object.fromEntries(plan.categories.map(c=>[c,draft.values[c]===undefined?Math.max(0,(plan.goals[draft.platform]?.[c]||0)-actualByCell(plan,draft.platform,c)):Number(draft.values[c])]))}:state.modal?.reason;
   if(state.actionBusy)return false;state.actionBusy=true;
   adapters.action(pid,operations[act],value).then(()=>{state.modal=null;state.drawer=null;state.actionBusy=false;plans=adapters.loadPlans();setToast('计划已更新');}).catch(error=>{state.actionBusy=false;setToast(error.message,'error')});return false;
  }
 }

 if(state.editor?.mode==='edit'&&['list','detail'].includes(act)&&JSON.stringify(state.editor)!==editorBaseline){openModal('leave-editor',{action:act,id});return;}
 switch(act){
  case 'leave-module':adapters.navigate?.(el.dataset.href);break;
  case 'external-leave':leaveResolve?.(true);leaveResolve=null;break;
  case 'discard-edit':{const next=state.modal;if(next.action==='external-leave'){state.editor=null;state.modal=null;leaveResolve?.(true);leaveResolve=null;return false;}state.editor=null;state.modal=null;onAction(next.action,{dataset:{id:next.id,href:next.href}});break;}

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

  case 'list':rememberCreation();state.page='list';state.modal=null;state.drawer=null;state.editor=null;render();window.scrollTo(0,listScroll);break;
  case 'new':return requestNewPlan();
  case 'discard-recovery':state.recoveryEditor=null;render();break;
  case 'reset-filters':state.filterDraft={search:'',status:'all',platform:'',category:''};state.filters=clone(state.filterDraft);state.pageNo=1;render();break;
  case 'page':state.pageNo=el.dataset.page?Number(el.dataset.page):state.pageNo+Number(el.dataset.dir);render();break;
  case 'detail':detailPlan(id);break;
  case 'focus-progress':detailPlan(id);setTimeout(()=>$('#batch-area')?.scrollIntoView({behavior:'smooth',block:'start'}),30);break;
  case 'edit':editPlan(id);break;
  case 'copy':if(p&&p.status==='ended')return requestNewPlan(p);break;
  case 'filter-status':state.statusFilter=el.dataset.value;render();break;
  case 'advanced':state.advanced=!state.advanced;render();break;
  case 'toggle-platform':state.platformExpand=state.platformExpand===el.dataset.platform?null:el.dataset.platform;render();break;
  case 'refresh-inventory':if(adapters.inventory)PLATFORM_DATA.splice(0,PLATFORM_DATA.length,...adapters.inventory().platforms);state.inventoryAt=new Date().toLocaleString('sv-SE').slice(0,16);setToast('资源盘点已刷新（原型使用示例资源数据）');break;
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
  case 'recommend':{
   const e=state.editor;if(!e||e.batches.some(x=>x.locked))break;const issues=recommendationIssues(e);if(issues.length){openModal('validation',{issues});break;}
   if(e.editedBatches)openModal('overwrite-reco');else{e.batches=genBatchSplit(e,true);render();setToast('AI 推荐演示方案已生成');}
   break;
  }
  case 'do-recommend':{
   const e=state.editor;if(!e||e.batches.some(x=>x.locked))break;const issues=recommendationIssues(e);if(issues.length){openModal('validation',{issues});break;}e.batches=genBatchSplit(e,true);e.editedBatches=false;state.modal=null;setToast('已重新生成拆分方案，人工调整已重置');break;
  }
  case 'add-batch':{
   const e=state.editor;if(!e.platforms.length){setToast('请先在 03 中选择目标平台','error');break;}
   e.batches.push(b('TEMP-'+(++identitySequence),`新增批次 ${String(e.batches.length+1).padStart(2,'0')}`,e.platforms[0],{}));e.editedBatches=true;render();break;
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
   if(!m.reason?.trim()){setToast('请填写终止原因','error');break;}
   const running=p.batches.filter(x=>['running','stopping'].includes(x.phase));
   for(const b of p.batches)if(['draft','ready'].includes(b.phase))b.phase='cancelled';
   p.cancelReason=m.reason.trim();
   if(running.length||openTasks(p).length){p.status='stopping';for(const b of running)b.phase='stopping';logPlan(p,`终止请求：${p.cancelReason}；等待执行器结果收敛（原型）`);}
   else{p.status='ended';p.result='cancelled';logPlan(p,`终止计划：${p.cancelReason}`);}
   state.modal=null;state.drawer=null;state.page='detail';state.planId=p.id;setToast(p.status==='stopping'?'已记录终止请求；仍需确认在途任务与未决结果':'计划已终止，已发布结果保留');break;
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
   const b=el.dataset.batch?p?.batches.find(x=>x.id===el.dataset.batch):p&&firstNotEnded(p);const issue=executionIssue(p,b);if(issue){setToast(issue,'error');break;}
   openModal('execute-batch',{planId:p.id,batchId:b.id});break;
  }
  case 'do-execute-batch':{
   const m=state.modal,p=m&&getPlan(m.planId),b=p?.batches.find(x=>x.id===m.batchId);
   if(executionIssue(p,b)){setToast('执行前校验未通过，请刷新后再试','error');break;}
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
  case 'close-modal':leaveResolve?.(false);leaveResolve=null;state.modal=null;render();break;
 }
}
// 失焦 change 不得重建正在按下的操作按钮；click 完成后统一刷新。
let pointerActionPending=false;
// Plan event bindings
function clearBatchDropMarks(){
 dom.querySelectorAll('.batches-table .is-drop-before,.batches-table .is-drop-after').forEach(row=>row.classList.remove('is-drop-before','is-drop-after'));
}
function clearBatchDrag(){
 batchDrag=null;clearBatchDropMarks();
 dom.querySelectorAll('.batches-table .is-batch-dragging').forEach(row=>row.classList.remove('is-batch-dragging'));
}
function batchDropTarget(event){
 const e=state.editor;
 if(!batchDrag||batchDrag.editor!==e||state.page!=='editor'||state.modal||state.drawer)return null;
 if(batchDrag.order.length!==e.batches.length||batchDrag.order.some((batch,index)=>e.batches[index]!==batch))return null;
 const row=event.target.closest('[data-plan-batch-row]');if(!row)return null;
 const from=e.batches.findIndex(batch=>batch.id===batchDrag.id),to=e.batches.findIndex(batch=>batch.id===row.dataset.planBatchRow);
 return canReorderBatch(e,from,to)?{e,from,to,row}:null;
}
listen(host,'dragstart',event=>{
 const handle=event.target.closest('[data-plan-batch-handle]');if(!handle)return;
 const e=state.editor,batch=e?.batches.find(batch=>batch.id===handle.dataset.planBatchHandle);
 if(handle.disabled||!e||!batch||!batchEditable(e,batch)||state.modal||state.drawer){event.preventDefault();return;}
 clearBatchDrag();batchDrag={editor:e,id:batch.id,order:[...e.batches]};
 event.dataTransfer.effectAllowed='move';event.dataTransfer.setData('text/plain',batch.id);
 const row=handle.closest('[data-plan-batch-row]');event.dataTransfer.setDragImage(row,12,12);row.classList.add('is-batch-dragging');
});
listen(host,'dragover',event=>{
 if(!batchDrag)return;clearBatchDropMarks();const target=batchDropTarget(event);
 if(!target){event.dataTransfer.dropEffect='none';return;}
 event.preventDefault();event.dataTransfer.dropEffect='move';target.row.classList.add(target.from<target.to?'is-drop-after':'is-drop-before');
});
listen(host,'dragleave',event=>{const row=event.target.closest('[data-plan-batch-row]');if(row&&!row.contains(event.relatedTarget))row.classList.remove('is-drop-before','is-drop-after');});
listen(host,'drop',event=>{
 if(!batchDrag)return;event.preventDefault();const target=batchDropTarget(event);clearBatchDrag();
 if(target&&reorderBatch(target.e,target.from,target.to))render();
});
listen(host,'dragend',clearBatchDrag);
listen(host,'keydown',event=>{
 const handle=event.target.closest('[data-plan-batch-handle]');if(!handle||!['ArrowUp','ArrowDown'].includes(event.key))return;
 event.preventDefault();if(handle.disabled||state.modal||state.drawer)return;
 const e=state.editor,from=e?.batches.findIndex(batch=>batch.id===handle.dataset.planBatchHandle);
 if(reorderBatch(e,from,from+(event.key==='ArrowUp'?-1:1)))render();
});
listen(host,'pointerdown',ev=>{pointerActionPending=!!ev.target.closest('button,a,input,select,textarea,[data-act]');},true);
listen(host,'pointerup',()=>setTimeout(()=>{pointerActionPending=false;},0),true);
listen(host,'pointercancel',()=>{pointerActionPending=false;},true);
listen(host,'click',ev=>{const el=ev.target.closest('[data-act]');if(!el||el.disabled)return;if(el.classList.contains('overlay')&&ev.target!==el)return;ev.preventDefault();if(el.closest('.modal')===null&&state.modal&&el.dataset.act!=='close-modal')return;const result=onAction(el.dataset.act,el);if(el.dataset.act!=='jump'&&result!==false)render();});
listen(host,'input',ev=>{const t=ev.target;if(t.dataset.filter){state.filterDraft[t.dataset.filter]=t.value;return;}if(t.dataset.field==='search'){
 const pos=t.selectionStart;state.search=t.value;render();const next=dom.querySelector('[data-field="search"]');if(next){next.focus();next.setSelectionRange(pos,pos);}return;}
 if(t.dataset.editor){const e=state.editor;if(e){e[t.dataset.editor]=t.dataset.editor==='target'?Math.max(0,Math.floor(Number(t.value)||0)):t.value;}return;}
 if(t.dataset.matrix){const e=state.editor;if(e){const p=t.dataset.platform,c=t.dataset.category;e.goals[p]??={};e.goals[p][c]=Math.max(0,Math.floor(Number(t.value)||0));}return;}
 if(t.dataset.batchCell){const e=state.editor;if(e){const bt=e.batches[Number(t.dataset.idx)];bt.cats[t.dataset.category]=Math.max(0,Math.floor(Number(t.value)||0));e.editedBatches=true;}return;}
 if(t.dataset.batchName!==undefined){const e=state.editor;if(e){e.batches[Number(t.dataset.batchName)].name=t.value;e.editedBatches=true;}return;}
 if(t.dataset.modalField){if(state.modal)state.modal[t.dataset.modalField]=t.value;return;}
 if(t.dataset.splitCell){if(state.modal){const bt=state.editor.batches[state.modal.index];const n=bt.cats[t.dataset.splitCell]||0;state.modal.values[t.dataset.splitCell]=Math.max(0,Math.min(n,Math.floor(Number(t.value)||0)));}return;}
 if(t.dataset.makeup==='name'){if(state.drawer)state.drawer.name=t.value;return;}
 if(t.dataset.makeupCell){if(state.drawer){state.drawer.values??={};state.drawer.values[t.dataset.makeupCell]=t.value;}return;}
});
listen(host,'change',ev=>{const t=ev.target;if(t.hasAttribute('data-page-size')){state.pageSize=Number(t.value);state.pageNo=1;render();return;}if(t.dataset.filter){state.filterDraft[t.dataset.filter]=t.value;return;}
 if(t.dataset.batchPlatform!==undefined){const e=state.editor,b=e?.batches[Number(t.dataset.batchPlatform)];if(!b||!batchEditable(e,b))return;b.platform=t.value;b.cats={};e.editedBatches=true;render();return;}
 if(t.dataset.makeup==='platform'){if(state.drawer){state.drawer.platform=t.value;state.drawer.name=`补量批次 · ${t.value}`;state.drawer.values={};render();}return;}
 if(t.dataset.modalField==='result'||t.dataset.splitCell){render();return;}
 if(t.dataset.editor||t.dataset.matrix||t.dataset.batchCell||t.dataset.batchName!==undefined){if(!pointerActionPending)setTimeout(render,0);return;}
});

listen(host,'keydown',ev=>{if(['Enter',' '].includes(ev.key)&&ev.target.matches('a[data-act],.tagcheck')){ev.preventDefault();ev.target.click();}});
listen(host,'submit',ev=>{if(ev.target.id==='plan-filters'){ev.preventDefault();state.filters=clone(state.filterDraft);state.pageNo=1;render();}});
let numericRefreshTimer=null;
function scheduleNumericRefresh(){
 clearTimeout(numericRefreshTimer);
 numericRefreshTimer=setTimeout(()=>{if(batchDrag||pointerActionPending||dom.querySelector(':popover-open')){scheduleNumericRefresh();return;}render();},180);
}
listen(host,'input',ev=>{persistPrototype();const t=ev.target;if(t.dataset.matrix||t.dataset.batchCell||t.dataset.splitCell||t.dataset.editor==='target')scheduleNumericRefresh();});
listen(window,'pagehide',persistPrototype);
listen(window,'beforeunload',event=>{if(state.editor?.mode==='edit'&&JSON.stringify(state.editor)!==editorBaseline){event.preventDefault();event.returnValue='';}});
function readDetailURL(){
 const route=routes.readOperationRoute(new URL(location.href));
 if(route.page!=='plans')return false;
 if(route.planId&&getPlan(route.planId)?.status==='draft'){state.page='list';state.planId=null;state.editor=null;return true;}
 if(route.editor){
  if(!state.editor){
   if(route.planId)editPlan(route.planId);
   else if(state.recoveryEditor){state.editor=restoreCreation();state.page='editor';}
   else newEditor();
  }
  return true;
 }
 rememberCreation();state.page=route.planId?'detail':'list';state.planId=route.planId;state.modal=null;state.drawer=null;state.editor=null;
 return true;
}
function currentURL(){return routes.operationURL('plans',{editor:state.page==='editor',planId:state.page==='detail'?state.planId:state.editor?.mode==='edit'?state.editor.sourceId:null});}
function confirmLeave(){
 if(state.page==='detail'){state.detailScrolls||={};state.detailScrolls[state.planId]=window.scrollY;}
 if(state.editor?.mode==='edit'&&JSON.stringify(state.editor)!==editorBaseline){
  return new Promise(resolve=>{leaveResolve=resolve;openModal('leave-editor',{action:'external-leave'});});
 }
 rememberCreation();persistPrototype();return Promise.resolve(true);
}
async function readRoute(){
 const target=location.pathname+location.search;
 if(target===currentURL())return;
 history.replaceState(null,'',currentURL());
 if(!await confirmLeave())return;
 history.replaceState(null,'',target);state.editor=null;
 readDetailURL();render();window.scrollTo(0,state.page==='list'?listScroll:0);
}
// Native field help follows common hover/focus/click and Escape behavior.
listen(host,'click',event=>{const button=event.target.closest('.ex-field-help > button');dom.querySelectorAll('.ex-field-help.is-open').forEach(el=>{if(el!==button?.parentElement)el.classList.remove('is-open')});if(button){button.parentElement.classList.remove('is-dismissed');button.parentElement.classList.add('is-open')}});
listen(host,'focusin',event=>event.target.closest('.ex-field-help')?.classList.remove('is-dismissed'));
listen(host,'keydown',event=>{if(event.key==='Escape'){const hint=document.activeElement?.closest('.ex-field-help');hint?.classList.remove('is-open');hint?.classList.add('is-dismissed')}});
readDetailURL();routeInitialised=true;render();
 globalThis.requestAnimationFrame?.(()=>window.scrollTo(0,state.page==='detail'?state.detailScrolls?.[state.planId]||0:state.page==='list'?listScroll:0));
return {
 confirmLeave,currentURL,readRoute,
 refresh(){if(adapters.loadPlans)plans=adapters.loadPlans();render();},
 dispose(){
  clearBatchDrag();rememberCreation();persistPrototype();disposed=true;
  leaveResolve?.(false);leaveResolve=null;
  disposers.forEach(off=>off());timers.forEach(id=>globalThis.clearTimeout(id));timers.clear();
  editorFooterObserver?.disconnect();unmountDetail?.();unmountList?.();unmountBasics?.();unmountScope?.();
  host.querySelectorAll('dialog[open]').forEach(dialog=>dialog.close());host.innerHTML='';
 }
};
}
