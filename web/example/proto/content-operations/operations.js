/* 运营共用语义与演示迁移；不改变既有存储键或删除历史记录。 */
(() => {
  const O=OPS,C=CP,P=PI,E=C.e;
  O.people=[{name:'陈小雨',role:'高级运营'},{name:'林予安',role:'运营'},{name:'周予宁',role:'管理员'}];
  O.categories=['剪辑教程','拍摄技巧'];
  O.role=()=>O.people.find(p=>p.name===O.actor)?.role||'运营';
  O.peopleOptions=()=>O.people.map(p=>[p.name,p.name+' · '+p.role]);
  O.person=name=>E(name)+'<small>'+E(O.people.find(p=>p.name===name)?.role||'运营')+'</small>';
  C.canManage=()=>O.role()!=='运营';
  const migration=6;
  if((C.db.operationsRevision||0)<migration){
    C.db.materials.forEach((m,i)=>Object.assign(m,{author:m.author||['剪辑实验室','镜头练习生','内容笔记'][i%3],published:m.published||'2026-10-'+String(21+i).padStart(2,'0')+' 10:30',views:m.views||18900+i*7321,favorites:m.favorites||680+i*127,created:m.created||'2026-11-01 09:00',cloudState:m.cloudState||(['ready','notSaved','preparing','failed','expired','ready','notSaved','ready'][i%8])}));
    C.db.replications ||= [];
    C.db.operationsRevision=migration;C.save();
  }
  if(!P.db.categoryRevision6){P.db.assets.forEach((a,i)=>{if(!O.categories.includes(a.category))a.category=O.categories[i%2];});P.db.plans.forEach(p=>{if(!O.categories.includes(p.category))p.category=O.categories[0];});P.db.categoryRevision6=true;P.save();}
  P.db.accounts.forEach((a,i)=>{a.tags ||= [a.quality==='优质'?'主推账号':'日常账号',i%2?'教程':'知识'];});
  P.db.replyDrafts ||= {};P.ui.replyDraft=P.db.replyDrafts;
  P.save();
  O.tabs=(items,active,select,label='视图切换')=>`<div class="ops-tabs" role="tablist" aria-label="${E(label)}">${items.map(([id,name])=>C.btn(E(name),()=>select(id),'text '+(active===id?'is-active':''),false,`role="tab" aria-selected="${active===id}"`)).join('')}</div>`;
  O.cloudLabels={notSaved:'未转存',preparing:'转存中',ready:'已转存',failed:'转存失败',expired:'来源失效'};
  O.cloud=m=>C.tag(O.cloudLabels[m.cloudState]||'未转存',m.cloudState==='ready'?'success':['failed','expired'].includes(m.cloudState)?'danger':m.cloudState==='preparing'?'info':'');
  O.prepareMaterial=id=>{const m=C.get('materials',id);if(m&&!['ready','preparing','expired'].includes(m.cloudState)){m.cloudState='preparing';C.save();}};
  const originalJoin=C.join;
  C.join=(ids,owner)=>{originalJoin(ids,owner);ids.forEach(O.prepareMaterial);C.save();};
  O.materialStats=id=>{
    const outputs=C.db.outputs.filter(o=>o.materialId===id),mine=outputs.filter(o=>o.owner===O.actor),pub=P.db.tasks.filter(t=>t.status==='published'&&(P.asset(t.assetId)?.materialId===id||outputs.some(o=>o.id===t.assetId)));
    return {outputs,mine,people:new Set(outputs.map(o=>o.owner)).size,pub,minePub:pub.filter(t=>outputs.some(o=>o.id===t.assetId&&o.owner===O.actor))};
  };
  O.usedLabel=id=>{const s=O.materialStats(id);return s.minePub.length?C.tag('已发布','success'):s.mine.length?C.tag('已制作','info'):C.tag('未制作');};
  O.sortHead=(label,key)=>{const sort=C.ui.sorts?.[C.route];return C.btn(E(label)+(sort?.key===key?(sort.desc?' ↓':' ↑'):' ↕'),()=>{C.ui.sorts||={};C.ui.sorts[C.route]={key,desc:sort?.key===key?!sort.desc:true};C.ui.page=1;P.ui.page=1;C.ui.selected.clear();C.render();},'text ops-sort');};
  O.sorted=(rows,value=(x,key)=>x[key])=>{const sort=C.ui.sorts?.[C.route];if(!sort)return rows;return [...rows].sort((a,b)=>{const x=value(a,sort.key),y=value(b,sort.key);return (typeof x==='number'&&typeof y==='number'?x-y:String(x??'').localeCompare(String(y??''),'zh-CN'))*(sort.desc?-1:1);});};
  O.reference=(text,url)=>url&&/^https?:\/\//.test(url)?`<a href="${E(url)}" target="_blank" rel="noopener noreferrer">${E(text)} ↗</a>`:`<span class="ops-muted">${E(text)}未取得</span>`;
  O.preview=(title,description='当前为演示素材，未加载真实视频。')=>`<div class="ops-media-preview"><div class="ops-poster" aria-label="${E(title)}的演示封面">${C.icon('Video')}</div><div><strong>${E(title)}</strong><p>${E(description)}</p></div></div>`;
  O.capability=(platform,action='publish')=>action==='publish'&&platform==='百度'?['browser','api']:['browser'];
  O.launchPlan=(opts={})=>{O.closeAll();const draft=P.newPlanDraft();draft.replicationId=opts.replicationId||null;if(opts.assetIds?.length){draft.assetIds=[...opts.assetIds];draft.groups.forEach(g=>{g.assetIds=[...opts.assetIds];g.enabled=(!opts.platform||g.platform===opts.platform)&&opts.assetIds.some(id=>O.availability(id,g.platform).ok);});}P.ui.planPlatform=draft.groups.find(g=>g.enabled)?.platform||'B站';P.nav('publish/plan/new');};
  // Exact account/time collisions are shown to the operator; no silent rescheduling.
  O.scheduleConflicts=(rows,opts={})=>{
    const existing=P.db.tasks.filter(t=>!['cancelled','draft'].includes(t.status)&&t.id!==opts.excludeTask&&(!opts.editPlan||t.planId!==opts.editPlan||PIPublishModel.locked(t))&&(!opts.replicationId||t.replicationId!==opts.replicationId));
    const result=[];rows.forEach((r,index)=>{const time=r.time||r.scheduled;if(!r.accountId||!time)return;const task=existing.find(t=>+t.accountId===+r.accountId&&t.scheduled===time),other=rows.findIndex((x,j)=>j!==index&&+x.accountId===+r.accountId&&(x.time||x.scheduled)===time);if(task||other>=0)result.push({index,accountId:+r.accountId,time,task,other});});return result;
  };
  O.conflictText=c=>`${P.account(c.accountId)?.name||c.accountId} · ${P.date(c.time)} · ${c.task?'与发布任务 '+c.task.id+' 冲突':'与本次第 '+(c.other+1)+' 条安排冲突'}`;
  O.accountSummary=ids=>{const accounts=ids.map(P.account).filter(Boolean),tags=[...new Set(accounts.flatMap(a=>a.tags||[]))];return `${accounts.length} 个账号 · ${tags.slice(0,4).join(' / ')}${tags.length>4?'等标签':''}`;};
})();
