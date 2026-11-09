/* 同一计划按平台执行；准备批次遵循演示时间与账号串行。 */
(() => {
  'use strict';
  const P=window.PI,M=PIPublishModel,E=P.e;
  const unfinished=t=>!['published','submitted','cancelled'].includes(t.status);
  P.startQueue=(ids,platform,mode,planId=null,override=null)=>{
    ids=[...ids].sort((a,b)=>P.task(a).scheduled.localeCompare(P.task(b).scheduled));
    P.ui.workAllIds=ids;P.ui.workIds=ids.filter(id=>P.platformOf(P.task(id))===platform);P.ui.workPlatform=platform;
    P.ui.workMode=mode;P.ui.workPlan=planId;P.ui.workOverride=override;P.ui.workId=null;P.ui.workPaused=false;P.ui.skippedWorkIds=new Set();
    P.nav(mode==='assisted'?'publish/workbench':'publish/auto');
  };
  P.openPlanWork=(plan,platform)=>{
    if(!platform)return P.choosePlanExecution(plan);
    const g=plan.groups.find(g=>g.platform===platform),ids=P.planTasks(plan).filter(t=>t.status!=='cancelled').sort((a,b)=>a.scheduled.localeCompare(b.scheduled)).map(t=>t.id);
    P.startQueue(ids,platform,g.mode,plan.id);
  };
  P.openTaskWork=(ids,mode)=>{
    ids=ids.filter(id=>P.task(id)&&unfinished(P.task(id)));
    if(!ids.length){P.toast('请选择尚未完成的发布任务');return;}
    const platforms=[...new Set(ids.map(id=>P.platformOf(P.task(id))))];
    if(platforms.length===1)return P.startQueue(ids,platforms[0],mode,null,mode);
    let d;d=P.open({title:'按平台进入发布',kind:'wide',body:`<p class="pi-meta pi-bottom-gap">已选任务按平台分别执行，账号和发布页面保持对应。</p><div class="pi-platform-cards">${platforms.map(platform=>`<article class="pi-platform-card"><h3>${platform}</h3><p>${ids.filter(id=>P.platformOf(P.task(id))===platform).length} 条任务</p>${P.btn('进入'+platform+'发布',()=>{P.close(d,true);P.startQueue(ids,platform,mode,null,mode);},'primary')}</article>`).join('')}</div>`});
  };
  P.switchWorkPlatform=platform=>{
    const plan=P.plan(P.ui.workPlan);
    if(plan)P.openPlanWork(plan,platform);
    else P.startQueue(P.ui.workAllIds,platform,P.ui.workOverride||P.ui.workMode,null,P.ui.workOverride);
  };
  const workRows=()=>{
    if(!Array.isArray(P.ui.workIds)){
      const mode=P.ui.route==='publish/auto'?'auto':'assisted';
      const plan=P.db.plans.find(p=>p.groups.some(g=>g.mode===mode)&&P.planTasks(p).some(unfinished));
      const group=plan?.groups.find(g=>g.mode===mode);
      if(!group)return [];
      const ids=P.planTasks(plan).sort((a,b)=>a.scheduled.localeCompare(b.scheduled)).map(t=>t.id);
      P.ui.workAllIds=ids;P.ui.workIds=ids.filter(id=>P.platformOf(P.task(id))===group.platform);P.ui.workPlan=plan.id;P.ui.workPlatform=group.platform;P.ui.workMode=mode;P.ui.skippedWorkIds=new Set();
    }
    return P.ui.workIds.map(P.task).filter(Boolean);
  };
  P.workSlots=rows=>{
    const slots=[],accounts=new Set(),time=P.ui.demoNow||M.now;
    for(const t of rows){
      if(!unfinished(t)||P.ui.skippedWorkIds?.has(t.id))continue;
      if(accounts.has(t.accountId))continue;
      accounts.add(t.accountId);
      if(t.scheduled>time&&!['preparing','waiting'].includes(t.status))continue;
      slots.push(t);if(slots.length>=5)break;
    }
    return slots;
  };
  P.prepareQueue=()=>{
    const slots=P.workSlots(workRows());
    slots.filter(t=>['ready','failed'].includes(t.status)&&!P.isTaskPaused(t)).forEach(t=>{t.status='preparing';t.phase=0;t.reason='';t.mode=P.ui.workMode;});P.save();P.render();
  };
  P.isTaskPaused=t=>{const plan=P.plan(t.planId);return plan?.status==='paused'||plan?.groups.find(g=>g.platform===P.platformOf(t))?.paused||(P.ui.workIds?.includes(t.id)&&P.ui.workPaused);};
  P.workbench=()=>{
    const rows=workRows();if(P.ui.workMode==='auto'&&!P.ui.workPaused){P.workSlots(rows).filter(t=>t.status==='ready'&&!P.isTaskPaused(t)).forEach(t=>{t.status='preparing';t.phase=0;});P.save();}const slots=P.workSlots(rows),pending=rows.filter(unfinished),future=pending.filter(t=>t.scheduled>(P.ui.demoNow||M.now)),plan=P.plan(P.ui.workPlan),g=plan?.groups.find(g=>g.platform===P.ui.workPlatform);
    const t=slots.find(t=>t.id===P.ui.workId)||slots[0]||future[0]||rows.at(-1);P.ui.workId=t?.id;
    if(!t)return P.head('发布执行','按平台处理当前发布队列。')+`<section class="ex-panel">${P.empty('没有待处理内容','返回发布任务选择需要执行的内容。',P.btn('返回发布任务',()=>P.nav('publish/tasks')))}</section>`;
    const assisted=P.ui.workMode==='assisted',paused=P.ui.workPaused||P.isTaskPaused(t),phase=t.status==='published'?4:t.status==='waiting'?3:t.phase||0;
    const steps=t.channel==='api'?['准备账号与请求','上传素材','生成发布参数','提交与取得结果']:['打开发布入口','上传视频','填写内容','提交与取得结果'];
    const platforms=plan?plan.groups.map(g=>g.platform):[...new Set(P.ui.workAllIds.map(id=>P.platformOf(P.task(id))))];
    let primary='';
    if(!slots.length&&future.length)primary=`<h3>等待下一个发布时段</h3><p>下一条安排在 ${P.date(future[0].scheduled)}。各平台按自己的节奏执行；下方演示控制可推进时间。</p>`;
    else if(t.status==='ready'||t.status==='failed')primary=`<h3>${t.status==='failed'?'当前账号需要处理':'准备当前发布内容'}</h3><p>${t.status==='failed'?E(t.reason):'只准备当前平台、已到发布时段的内容。同一账号的任务依次执行。'}</p>${P.btn(t.status==='failed'?'重新准备':'准备当前批次',P.prepareQueue,'primary',paused)}`;
    else if(t.status==='preparing')primary=`<h3>${t.channel==='api'?'正在准备 API 发布参数':'系统正在准备平台页面'}</h3><p>当前处于${steps[Math.min(phase,3)]}阶段。使用演示控制推进。</p>${P.btn(t.channel==='api'?'查看发布参数':'预览窗口',()=>P.platformWindow(t))}`;
    else if(t.status==='waiting')primary=`<h3>${t.checkPending?'已在平台提交，等待检查结果':'内容已准备，请到窗口微调并发布'}</h3><p>可先准备多个窗口，逐个发布后回到这里检查结果。成功后补入下一条。</p>${P.btn(t.channel==='api'?'检查并提交':'打开窗口',()=>P.platformWindow(t),t.checkPending?'':'primary',paused)} ${P.btn('已发布，检查结果',()=>{if(!t.checkPending){P.toast('请先在演示窗口完成提交');return;}P.completePublication(t);t.checkPending=false;P.ui.workId=null;P.fillNextWork();P.render();P.toast('演示检查成功，已取得作品链接');},t.checkPending?'primary':'',paused)} ${P.btn('稍后处理',()=>{P.ui.skippedWorkIds.add(t.id);t.status='ready';t.phase=0;P.ui.workId=null;P.save();P.render();})}`;
    else primary=`<h3>本平台内容已处理</h3><p>可切换平台继续，或查看本平台发布结果。</p>${P.btn('查看发布结果',()=>{P.ui.filters['publish/tasks']={q:plan?String(plan.id):'',platform:P.ui.workPlatform};P.nav('publish/tasks');},'primary')}`;
    const toggle=()=>{P.ui.workPaused=!paused;if(g){g.paused=!paused;plan.status=plan.groups.every(g=>g.paused)?'paused':'running';}P.save();P.render();};
    return `<a class="pi-back" href="#publish/tasks">‹ 返回发布任务</a>${P.head(P.ui.workPlatform+' · '+(assisted?'人工连续发布':'全自动发布'),plan?E(plan.name):'普通发布 · 按平台执行',P.btn(paused?'继续本平台':'暂停本平台',toggle)+P.btn('离开工作区',()=>{P.ui.workIds=null;P.ui.workPlan=null;P.nav('publish/tasks');}))}<p class="ops-note">${assisted?'人工连续发布：系统准备窗口，运营在平台微调并提交，再回此处检查结果。':'全自动发布：到时进入执行；此原型通过下方演示控制展示过程。'} 暂停本平台只暂停当前队列后续执行；已提交的平台结果仍可查看。离开工作区保留任务进度，再次进入可继续。稍后处理的记录可从发布任务重新进入。</p><div class="pi-work-platforms" aria-label="切换执行平台">${platforms.map(platform=>{const all=P.ui.workAllIds.map(P.task).filter(t=>P.platformOf(t)===platform);return P.btn(platform+' '+all.filter(t=>t.status==='published').length+'/'+all.length,()=>P.switchWorkPlatform(platform),platform===P.ui.workPlatform?'primary':'');}).join('')}</div><section class="ex-panel"><div class="pi-queue-summary"><div>${P.ui.workPlatform} ${rows.length} 条任务 · 已发布 ${rows.filter(t=>t.status==='published').length} 条${paused?' · 已暂停':''}<small class="pi-subline">演示时间 ${P.date(P.ui.demoNow||M.now)} · 另有 ${future.length} 条等待后续时段</small></div><div class="ex-actions"><span class="pi-meta">当前最多准备 ${Math.min(5,new Set(rows.map(t=>t.accountId)).size)} 个${t.channel==='api'?'账号任务':'窗口'}</span>${assisted?P.btn('准备本批',P.prepareQueue,'',paused||!slots.some(t=>['ready','failed'].includes(t.status))):'<span class="pi-meta">按排期自动准备</span>'}</div></div><div class="pi-workbench"><aside class="pi-work-queue"><div class="pi-work-queue-head"><h2>当前准备</h2><span class="pi-meta">${slots.length} 条</span></div>${slots.map(item=>`<button class="pi-work-item ${item.id===t.id?'active':''}" data-action="${P.action(()=>{P.ui.workId=item.id;P.render();})}">${P.cover(P.asset(item.assetId))}<span><strong>${E(P.account(item.accountId).name)}</strong><span class="pi-subline">${E(item.title)}</span>${P.status(item.status,item.status==='ready'?'待准备':null)}</span></button>`).join('')||'<p class="pi-meta">当前没有已到时段的任务。</p>'}</aside><div class="pi-work-content"><div class="pi-work-heading"><h2>${E(t.title)}</h2>${P.status(t.status,t.status==='ready'?'待发布':null)}</div>${P.facts([['发布 ID',t.id],['平台',P.ui.workPlatform],['发布账号',E(P.account(t.accountId).name)],['发布通道',P.channel(t.channel)],['计划时间',P.date(t.scheduled)],['成片版本',E(P.asset(t.assetId).variant||'通用版')]])}<ol class="pi-execution-steps">${steps.map((s,i)=>`<li class="${phase>i?'done':phase===i?'active':''}">${s}</li>`).join('')}</ol><div class="pi-work-primary">${primary}</div>${P.section('发布后动作',`<p class="pi-meta">${P.postText(t.post)}；初始互动仅使用 ${P.ui.workPlatform} 账号，自动关联本条作品的实际链接。</p>`)}<div class="pi-demo-strip"><strong>原型演示控制</strong>${P.btn('推进准备一步',()=>P.advancePublishing(slots,assisted),'',paused||!slots.some(t=>t.status==='preparing'))}${P.btn('模拟发布成功',()=>{P.completePublication(t);P.ui.workId=null;P.fillNextWork();P.render();},'',paused||!slots.includes(t)||!['preparing','waiting'].includes(t.status))}${P.btn('模拟账号异常',()=>{t.status='failed';t.reason=t.channel==='api'?'账号授权已失效，完成授权后可继续。':'账号登录已失效，完成登录后可继续。';P.save();P.render();},'',paused||!slots.includes(t)||!['preparing','waiting'].includes(t.status))}${P.btn('推进到下一发布时间',()=>{P.ui.demoNow=future.map(t=>t.scheduled).sort()[0];P.ui.workId=null;P.render();},'',!future.length)}<span>仅手动推进演示状态，不启动真实窗口或接口。</span></div></div></div></section>`;
  };
  P.fillNextWork=()=>{if(!P.ui.workIds)return;P.workSlots(workRows()).filter(t=>t.status==='ready').forEach(t=>{t.status='preparing';t.phase=0;t.mode=P.ui.workMode;});P.save();};
  P.advancePublishing=(slots,assisted)=>{slots.filter(t=>t.status==='preparing').forEach(t=>{t.phase=Math.min(3,(t.phase||0)+1);if(t.phase===3){if(assisted)t.status='waiting';else P.completePublication(t);}});if(!assisted)P.fillNextWork();P.save();P.render();};
})();
