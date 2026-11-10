/* 统一工作台；所有操作均为浏览器内的体验演示。 */
(() => {
  const C=CP,P=PI,E=C.e,O=window.OPS={actor:'陈小雨',route:'overview'};
  O.platforms=['B站','百度','小红书'];
  O.sourcePlatforms=['抖音'];
  C.db.platforms=[...O.platforms];
  C.db.strategies.forEach(s=>{if(s.id===204&&s.name==='抖音竖屏引导')s.name='竖屏引导';});
  O.version=()=>({name:'通用版',qty:1,platforms:['B站','百度'],intro:'不使用',guide:'不使用',outro:'不使用',start:'00:00',end:'01:30'});
  if(!C.db.suiteVersion){
    C.db.strategies.forEach(s=>{s.versions=[O.version()];s.autoOwner=s.operator;s.auto=false;});
    C.db.tasks.forEach(t=>{t.owner='陈小雨';t.version={...O.version(),name:t.name||'通用版',platforms:t.platform?[t.platform]:['B站','百度']};t.batchId=t.batch||'历史单条-'+t.id;});
    C.db.outputs.forEach(o=>{o.owner='陈小雨';o.platforms=C.get('tasks',o.taskId)?.version.platforms||['B站'];o.assignees=Object.fromEntries(o.platforms.map(p=>[p,'陈小雨']));});
    C.db.mine=[101,102,103];C.db.suiteVersion=1;C.save();
  }
  if(!P.db.suiteVersion){
    ['小红书'].forEach(platform=>{for(let n=0;n<3;n++)P.db.accounts.push({id:511+n,name:platform+'运营'+(n+1),platform,quality:n?'普通':'优质',window:platform+'窗口'+(n+1),channel:'browser',channels:['browser'],enabled:true});});
    const seen=new Set();P.db.tasks=P.db.tasks.filter(t=>{const k=t.assetId+':'+(t.platform||P.account(t.accountId)?.platform);if(seen.has(k))return false;seen.add(k);return true;});
    P.db.plans=P.db.plans.filter(p=>P.db.tasks.some(t=>t.planId===p.id));P.db.suiteVersion=1;P.db.externalWorks=[];P.db.publishDrafts=[];P.save();
  }
  // Keep historical account IDs resolvable, but never offer unsupported accounts for new work.
  for(const accounts of [C.db.accounts,P.db.accounts])for(const account of accounts||[])if(!O.platforms.includes(account.platform)){account.enabled=false;account.reason='当前不支持该发布平台';}
  C.save?.();
  PIPublishModel.platforms=O.platforms;PIPublishModel.now='2026-11-03T09:00';
  O.outputName=o=>O.outputTitle(o);
  O.outputTitle=o=>[C.get('materials',o.materialId)?.title||'素材',C.get('tasks',o.taskId)?.version?.name||o.name].filter(Boolean).join(' · ');
  O.outputFilename=o=>[O.outputTitle(o),String(o.id)].join(' · ').replace(/[\\/:*?"<>|]/g,'_')+'.mp4';
  O.sync=()=>{C.db.outputs.forEach(o=>{const m=C.get('materials',o.materialId);const a={id:o.id,outputId:o.id,materialId:o.materialId,title:O.outputName(o),category:m?.category||'—',quality:'优质',duration:o.metadata?.duration||'待识别',format:o.metadata?.resolution||'待识别',variant:o.name,date:o.date,kind:0};const old=P.asset(o.id);if(old)Object.assign(old,a);else P.db.assets.unshift(a);});P.save();};O.sync();
  O.reservation=(id,platform,exceptId,exceptPlan)=>P.db.tasks.find(t=>t.assetId===Number(id)&&P.platformOf(t)===platform&&t.id!==exceptId&&(!exceptPlan||t.planId!==exceptPlan)&&!['cancelled','draft'].includes(t.status));
  O.availability=(id,platform,exceptId,exceptPlan)=>{const o=C.get('outputs',id),t=O.reservation(id,platform,exceptId,exceptPlan);if(t)return {ok:false,text:t.status==='published'?'已发布':'已安排',task:t};if(o&&(!o.platforms.includes(platform)||o.assignees[platform]!==O.actor))return {ok:false,text:o.platforms.includes(platform)?'由 '+o.assignees[platform]+' 使用':'未分配该平台'};return {ok:true,text:'未安排'};};
  // 发布情况独立于文件可用性及当前使用者，不把权限当作发布状态。
  O.publication=(id,platform)=>{
    if(!Array.isArray(P.db.tasks))return {key:'unknown',text:'—'};
    const tasks=P.db.tasks.filter(t=>t.assetId===Number(id)&&P.platformOf(t)===platform&&!['draft','cancelled'].includes(t.status));
    const task=tasks.find(t=>t.status==='published')||tasks[0];
    return task?{key:task.status==='published'?'published':'reserved',text:task.status==='published'?'已发布':'已安排',task,tasks}:{key:'unarranged',text:'未安排'};
  };
  O.closeAll=()=>{C.dialogs.slice().reverse().forEach(d=>C.close(d,true));P.dialogs.slice().reverse().forEach(d=>P.close(d,true));};
  O.go=r=>{O.closeAll();if(location.hash.slice(1)===r)O.render();else location.hash=r;};
  const groups=[['工作台',[['overview','运营概览','ChartBar']]],['内容生产',[['materials','素材库','Browse'],['mine','我的素材','Folder'],['strategies','合成策略','Setting'],['tasks','合成任务','Video'],['outputs','成片管理','Video']]],['发布管理',[['publish/tasks','发布任务','Send'],['publish/plans','发布计划','Calendar']]],['作品管理',[['works','作品列表','ChartBar'],['replications','复刻任务','Video']]],['互动管理',[['interact/tasks','主动互动','Chat'],['interact/replies','评论管理','Chat']]]];
  O.overview=()=>C.head('运营概览','')+`<section class="ex-panel ops-deferred">${C.icon('ChartBar')}<h2>运营概览暂缓设计</h2><p>先完成素材、生产、发布和作品运营流程。</p><a href="#materials" class="ex-button primary">进入素材库</a></section>`;
  O.render=()=>{
    const r=location.hash.slice(1)||'overview';O.route=r;C.route=r;P.ui.route=r;
    document.body.dataset.route=r;let body;
    if(r.startsWith('publish/'))body=P.renderPublishing();else if(r.startsWith('interact/'))body=P.renderInteraction();else body=({overview:O.overview,materials:()=>C.materialPage(false),mine:()=>C.materialPage(true),strategies:C.strategyPage,tasks:C.taskPage,outputs:C.outputPage,works:O.worksPage,replications:O.replicationsPage})[r]?.()||O.overview();
    document.getElementById('app').innerHTML=`<div class="ex-app ops-shell"><aside class="ex-sidebar"><a class="ops-brand" href="#overview"><img src="assets/logo.png" alt="起飞"><div><strong>起飞</strong><small>CONTENT OPERATIONS</small></div></a><nav aria-label="主导航">${groups.map(([name,items])=>`<section class="ops-nav-group"><h2>${name}</h2>${items.map(([path,label,icon])=>`<a href="${path.startsWith('publish/')?'/example/?view=publishing#'+(path==='publish/plans'?'plans':'tasks'):'#'+path}" ${r===path||(path==='publish/plans'&&r.startsWith('publish/plan/'))?'aria-current="page"':''}>${C.icon(icon)}${label}</a>`).join('')}</section>`).join('')}</nav><div class="ops-nav-foot">体验评审版 · 2026.11.03<br>全部模块共用一套演示数据</div></aside><main class="ex-main"><header class="ops-topbar"><span>内容运营 / ${groups.flatMap(g=>g[1]).find(x=>x[0]===r)?.[1]||'工作详情'}</span><div class="ops-top-actions">${C.btn('规模演示',()=>O.scaleDemo(),'text')}<a href="review.html">本轮修改与评审 ↗</a><label>当前使用者 <select id="ops-actor">${O.people.map(p=>`<option value="${p.name}" ${p.name===O.actor?'selected':''}>${p.name} · ${p.role}</option>`).join('')}</select></label></div></header><div class="ex-content">${body}<footer class="ops-footer"><span>交互原型 · 操作仅保存在当前浏览器 · 无真实平台执行</span><a href="review.html#global-review">补充整体评审 →</a></footer></div></main></div>`;
    C.enhance(document.getElementById('app'));OPSControls.mount(document.getElementById('app'),id=>C.switchProps(id));document.getElementById('ops-actor').addEventListener('change',e=>{O.actor=e.target.value;O.render();});
    if(r.startsWith('publish/'))P.wirePublishing();if(r.startsWith('interact/'))P.wireInteraction();
  };
  addEventListener('hashchange',()=>{C.ui.selected.clear();P.ui.selected.clear();C.ui.page=P.ui.page=1;O.render();window.scrollTo({top:0,left:0});document.querySelector('.ex-content')?.scrollTo({top:0,left:0});});
  addEventListener('DOMContentLoaded',O.render);
})();
