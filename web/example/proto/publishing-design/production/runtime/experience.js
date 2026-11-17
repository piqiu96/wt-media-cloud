/* 在既有生产能力上补齐体验；新增关联只用于本地原型，不代表后端字段。 */
(() => {
  const C=CP,O=OPS,{copy,table}=C.prod;
  if(!C.db.productionRevision){
    C.db.usages={陈小雨:(C.db.mine||[]).map(materialId=>({materialId,status:'active',download:'notDownloaded',added:'10-31 09:00'})),林予安:[{materialId:104,status:'active',download:'downloaded',added:'10-30 10:00'}]};
    if(!C.db.usages.陈小雨.some(u=>u.materialId===105))C.db.usages.陈小雨.push({materialId:105,status:'removed',download:'missing',added:'10-29 10:00'});
    C.db.strategies.forEach(s=>Object.assign(s,{executionMode:s.auto||s.schedule.includes('每天')?'scheduled':'manual',scheduleMode:'daily',dailyTime:'09:00',interval:60,timeZone:'Asia/Shanghai',singleLimit:3,dailyLimit:12,team:'内容运营一组',templateVersion:'v1',rules:[{action:'match',logic:'all',conditions:[{field:'likes',operator:'gte',value:3000}]}]}));
    C.db.tasks.forEach(t=>{t.snapshot={strategyName:C.get('strategies',t.strategyId)?.name,template:C.get('strategies',t.strategyId)?.template,...copy(t.version)};t.finished=t.status==='success'?'10-30 14:20':'—';t.error=t.status==='failed'?'合成中断，未生成成片。可在检查素材后重试。':'';});
    C.db.runs=[];C.db.productionRevision=4;C.save();
  }
  C.canManage=()=>O.actor==='陈小雨';
  C.usages=(owner=O.actor)=>C.db.usages[owner]||=[];
  C.usage=(id,owner=O.actor)=>C.usages(owner).find(x=>x.materialId===id);
  C.mine=()=>C.db.materials.filter(m=>C.usage(m.id)?.status==='active');
  C.join=(ids,owner=O.actor)=>{ids.forEach(id=>{const u=C.usage(id,owner);if(u)u.status='active';else C.usages(owner).push({materialId:id,status:'active',download:'notDownloaded',added:'11-01 09:00'});});C.save();};
  C.downloadText=s=>({notDownloaded:'未下载',downloading:'下载中',downloaded:'已下载',failed:'下载失败',missing:'文件缺失',unverified:'待校验'})[s]||'未下载';
  C.batchKey=t=>t.runKey?'run:'+t.runKey:t.batchId||'single:'+t.id;
  C.realBatchId=t=>t&&t.batchId&&!String(t.batchId).startsWith('历史单条-')?t.batchId:t?.runKey||null;
  C.batchRows=key=>C.db.tasks.filter(t=>C.batchKey(t)===key);
  C.time=()=>new Date().toLocaleString('sv-SE',{timeZone:'Asia/Shanghai'}).slice(5,16);
  C.submitTasks=({strategy,ids,vs,owner,trigger='人工创建',name,runKey,followup,replicationId,requestKey})=>{
    const existing=requestKey&&C.db.tasks.filter(t=>t.requestKey===requestKey);if(existing?.length){if(existing.some(t=>t.creator!==O.actor))throw Error('无权读取该次提交');return existing.sort((a,b)=>a.id-b.id).map(t=>t.id);}
    if(!C.access(strategy)||!strategy.enabled||!C.prod.valid(vs,strategy.template))throw Error('策略或参数不可用');
    if(trigger==='人工创建'&&!ProductionModel.validOverrides(strategy.versions,vs,C.prod.editable[strategy.template]||[]))throw Error('固定配置不允许修改');
    if(!ids.length||new Set(ids).size!==ids.length||ids.some(id=>{const m=C.get('materials',id);return !m||m.category!==strategy.category||!ProductionModel.canPrepare(m);})||(strategy.operator&&strategy.operator!==owner))throw Error('素材或负责人范围已变化，请重新确认');
    const batchId=C.id(),created=C.fullTime(),made=[];
    ids.forEach(materialId=>vs.forEach(v=>{for(let n=1;n<=v.qty;n++){
      const t={id:C.id(),materialId,strategyId:strategy.id,name:v.name+(v.qty>1?' · 第'+n+'份':''),batch:name||strategy.name+' · '+created,batchId,runKey,requestKey,creator:O.actor,status:'pending',owner,sourceQuantity:v.qty,copyIndex:n,version:{...copy(v),qty:1},snapshot:{strategyName:strategy.name,template:strategy.template,templateVersion:strategy.templateVersion,guideUrl:strategy.guideUrl||strategy.intro||'',...copy(v)},created,trigger,attempt:1,finished:'—',followup:followup||'',replicationId:replicationId||null,events:[{text:'任务已创建',time:created}]};C.db.tasks.unshift(t);made.push(t.id);
    }}));C.save();return made;
  };
  C.filterProduction=type=>{
    const applied=C.read(),f=C.ui.filterDrafts[C.route]||={...applied};
    const strategy=C.select('strategy','合成策略',f.strategy,C.db.strategies.filter(C.access).map(s=>[s.id,s.name])),owner=C.canManage()?C.select('operator','负责人',f.operator,O.peopleOptions()):'',category=C.select('category','类目',f.category,O.categories),dates=C.prod.dateRange(type==='outputs'?'生成日期':'创建日期',f.from,f.to),trigger=C.select('trigger','创建方式',f.trigger,['人工创建','策略执行','策略自动']);
    const primary=type==='strategies'?category+C.select('status','状态',f.status,[['enabled','启用'],['disabled','停用']])+owner:type==='tasks'?C.select('status','任务状态',f.status,[['pending','排队中'],['running','运行中'],['success','已完成'],['failed','失败'],['cancelled','已取消']])+strategy+owner:category+C.select('platform','适用平台',f.platform,O.platforms)+C.select('availability','发布情况',f.availability,[['available','未安排'],['reserved','已安排'],['published','已发布']]);
    const more=type==='strategies'?C.select('template','合成模板',f.template,Object.keys(C.prod.schemas))+C.select('executionMode','执行方式',f.executionMode,[['manual','手动执行'],['scheduled','定时执行']]):type==='tasks'?category+trigger+dates:strategy+owner+trigger+C.select('download','下载状态',f.download,[['downloaded','已下载'],['notDownloaded','未下载']])+dates;
    const keys=type==='strategies'?['template','executionMode']:type==='tasks'?['category','trigger',['from','to']]:['strategy',...(C.canManage()?['operator']:[]),'trigger','download',['from','to']],count=C.prod.filterCount(applied,keys);
    return `<form class="ex-filters ex-filters-stacked" data-list-filter>${C.field('综合搜索','q',f.q,{placeholder:type==='strategies'?'策略名称 / ID':'名称 / 素材 ID / 批次 ID',className:'ex-filter-search'})}${primary}<div class="ex-actions"><button class="ex-button primary" type="submit">查询</button>${C.btn('重置',()=>{C.ui.filters[C.route]=null;delete C.ui.filterDrafts[C.route];C.ui.page=1;C.ui.selected.clear();C.render();})}</div>${C.prod.advanced(more,count,C.ui.filterOpen[C.route]??count>0)}</form>`;
  };
  // 无年份的 MM-DD HH:mm 仅用于既有 2026 年离线样本；未知日期不参加范围筛选。
  C.match=(x,type)=>{const f=C.read(),t=type==='tasks'?x:C.get('tasks',x.taskId),s=type==='strategies'?x:C.get('strategies',t?.strategyId),m=C.get('materials',x.materialId),rawDate=x.created||x.date||'',date=/^\d{4}-\d{2}-\d{2}/.test(rawDate)?rawDate.slice(0,10):/^\d{2}-\d{2} \d{2}:\d{2}$/.test(rawDate)?'2026-'+rawDate.slice(0,5):'';return (!(f.from||f.to)||!!date)&&(!f.availability||x.platforms?.some(p=>(!f.platform||f.platform===p)&&(O.publication(x.id,p).key===({available:'unarranged',reserved:'reserved',published:'published'})[f.availability])))&&(!f.template||s?.template===f.template)&&(!f.executionMode||s?.executionMode===f.executionMode)&&(C.canManage()||(x.owner||s?.operator)===O.actor)&&(!f.q||`${x.id} ${x.materialId||''} ${x.name} ${x.batchId||t?.batchId||''} ${x.batch||''} ${m?.title||''}`.includes(f.q))&&(!f.category||(m?.category||s?.category)===f.category)&&(!f.strategy||s?.id===+f.strategy)&&(!f.operator||(x.owner||s?.operator)===f.operator)&&(!f.status||(type==='strategies'?(x.enabled?'enabled':'disabled'):x.status)===f.status)&&(!f.platform||x.platforms?.includes(f.platform))&&(!f.trigger||t?.trigger===f.trigger)&&(!f.download||(C.outputLocalText?.(x)==='已下载'?'downloaded':'notDownloaded')===f.download)&&(!f.from||date>=f.from)&&(!f.to||date<=f.to);};
  C.notice='';C.noticeHTML=()=>C.notice&&C.noticeRoute===C.route?`<div class="ops-result" role="status">${C.notice}</div>`:'';
})();
