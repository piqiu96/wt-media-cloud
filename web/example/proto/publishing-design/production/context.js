import {prototypeSession,prototypePeople} from '../prototypeSession.js';
// Content-production context only. Publishing state belongs to the new workbench.
export function createProductionContext(C, services) {
 const O=window.OPS={route:'materials',target:null,services};
 Object.defineProperty(O,'actor',{get:()=>prototypeSession.actor,set:name=>{prototypeSession.actor=name;}});
 O.platforms=['B站','百度','小红书'];O.sourcePlatforms=['抖音'];
 O.people=prototypePeople;O.categories=['剪辑教程','拍摄技巧'];
 O.role=()=>O.people.find(p=>p.name===O.actor)?.role||'运营';O.peopleOptions=()=>O.people.map(p=>[p.name,p.name+' · '+p.role]);
 O.person=name=>C.e(name)+'<small>'+C.e(O.people.find(p=>p.name===name)?.role||'运营')+'</small>';
 O.version=()=>({name:'通用版',qty:1,platforms:['B站','百度'],intro:'不使用',guide:'不使用',outro:'不使用',start:'00:00',end:'01:30'});
 C.db.platforms=[...O.platforms];
 if(!C.db.suiteVersion){
   C.db.strategies.forEach(s=>{s.versions=[O.version()];s.autoOwner=s.operator;s.auto=false;});
   C.db.tasks.forEach(t=>{t.owner='陈小雨';t.version={...O.version(),name:t.name||'通用版',platforms:t.platform?[t.platform]:['B站','百度']};t.batchId=t.batch||'历史单条-'+t.id;});
   C.db.outputs.forEach(o=>{o.owner='陈小雨';o.platforms=C.get('tasks',o.taskId)?.version.platforms||['B站'];o.assignees=Object.fromEntries(o.platforms.map(p=>[p,'陈小雨']));});
   C.db.mine=[101,102,103];C.db.suiteVersion=1;
 }
 if((C.db.operationsRevision||0)<6){
   C.db.materials.forEach((m,i)=>Object.assign(m,{author:m.author||['剪辑实验室','镜头练习生','内容笔记'][i%3],published:m.published||'2026-10-'+String(21+i).padStart(2,'0')+' 10:30',views:m.views||18900+i*7321,favorites:m.favorites||680+i*127,created:m.created||'2026-11-01 09:00',cloudState:m.cloudState||(['ready','notSaved','preparing','failed','expired','ready','notSaved','ready'][i%8])}));
   C.db.operationsRevision=6;
 }
 for(const a of C.db.accounts||[])if(!O.platforms.includes(a.platform))a.enabled=false;
 C.save();
 O.outputTitle=o=>[C.get('materials',o.materialId)?.title||'素材',C.get('tasks',o.taskId)?.version?.name||o.name].filter(Boolean).join(' · ');
 O.outputName=O.outputTitle;O.outputFilename=o=>[O.outputTitle(o),String(o.id)].join(' · ').replace(/[\\/:*?"<>|]/g,'_')+'.mp4';
 O.sync=()=>O.services.sync(C.db,O.actor);
 O.publication=(id,platform)=>O.services.publication(id,platform);
 O.availability=(id,platform)=>{
   const o=C.get('outputs',id),p=O.publication(id,platform);
   if(p.key==='unknown')return {ok:false,text:'发布记录暂未加载'};
   if(p.task)return {ok:false,text:p.text,task:p.task};
   if(!o||!o.platforms.includes(platform)||o.assignees?.[platform]!==O.actor)return {ok:false,text:'不在当前用户可发布范围'};
   return {ok:true,text:'未安排'};
 };
 O.confirmLeave=()=>new Promise(resolve=>{
   let answered=false,d;
   const finish=leave=>{answered=true;C.close(d,true);resolve(leave);};
   d=C.open({title:'离开当前页面？',body:()=>'<p>尚未保存的修改将丢失。</p>',footer:()=>C.btn('继续编辑',()=>finish(false))+C.btn('离开',()=>finish(true),'primary')});
   d.addEventListener('close',()=>{if(!answered)resolve(false);},{once:true});
 });
 O.closeAll=()=>C.dialogs.slice().reverse().forEach(d=>C.close(d,true));
 O.go=r=>O.services.navigate(r==='tasks'?'production-tasks':r);C.nav=O.go;
 O.tabs=(items,active,select,label='视图切换')=>`<div class="ops-tabs" role="tablist" aria-label="${C.e(label)}">${items.map(([id,name])=>C.btn(C.e(name),()=>select(id),'text '+(active===id?'is-active':''),false,`role="tab" aria-selected="${active===id}"`)).join('')}</div>`;
 O.cloudLabels={notSaved:'未转存',preparing:'转存中',ready:'已转存',failed:'转存失败',expired:'来源失效'};
 O.cloud=m=>C.tag(O.cloudLabels[m.cloudState]||'未转存',m.cloudState==='ready'?'success':['failed','expired'].includes(m.cloudState)?'danger':m.cloudState==='preparing'?'info':'');
 O.prepareMaterial=id=>{const m=C.get('materials',id);if(m&&!['ready','preparing','expired'].includes(m.cloudState))m.cloudState='preparing';};
 O.materialStats=id=>{const outputs=C.db.outputs.filter(o=>o.materialId===id),mine=outputs.filter(o=>o.owner===O.actor);return {outputs,mine,people:new Set(outputs.map(o=>o.owner)).size,pub:[],minePub:[]};};
 O.usedLabel=id=>O.materialStats(id).mine.length?C.tag('已制作','info'):C.tag('未制作');
 O.sortHead=(label,key)=>{const sort=C.ui.sorts?.[C.route];return C.btn(C.e(label)+(sort?.key===key?(sort.desc?' ↓':' ↑'):' ↕'),()=>{C.ui.sorts||={};C.ui.sorts[C.route]={key,desc:sort?.key===key?!sort.desc:true};C.ui.page=1;C.ui.selected.clear();C.render();},'text ops-sort');};
 O.sorted=(rows,value=(x,key)=>x[key])=>{const sort=C.ui.sorts?.[C.route];return sort?[...rows].sort((a,b)=>{const x=value(a,sort.key),y=value(b,sort.key);return (typeof x==='number'&&typeof y==='number'?x-y:String(x??'').localeCompare(String(y??''),'zh-CN'))*(sort.desc?-1:1);}):rows;};
 O.reference=(text,url)=>url&&/^https?:\/\//.test(url)?`<a href="${C.e(url)}" target="_blank" rel="noopener noreferrer">${C.e(text)} ↗</a>`:`<span class="ops-muted">${C.e(text)}未取得</span>`;
 O.preview=(title,description='当前为演示素材，未加载真实视频。')=>`<div class="ops-media-preview"><div class="ops-poster">${C.icon('Video')}</div><div><strong>${C.e(title)}</strong><p>${C.e(description)}</p></div></div>`;
 O.render=()=>{
   if(!O.target?.isConnected)return;
   const pages={materials:()=>C.materialPage(false),mine:()=>C.materialPage(true),strategies:C.strategyPage,tasks:C.taskPage,outputs:C.outputPage};
   O.target.innerHTML=pages[C.route]?.()||'';O.target.dataset.route=C.route;
   O.target.querySelectorAll('a[href^="#"]').forEach(a=>a.addEventListener('click',e=>{e.preventDefault();O.go(a.hash.slice(1));}));
   C.enhance(O.target);window.OPSControls.mount(O.target,id=>C.switchProps(id));
 };
 return O;
}
