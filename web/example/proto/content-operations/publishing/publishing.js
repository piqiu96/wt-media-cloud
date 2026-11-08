/* 发布任务与平台执行队列。 */
(() => {
  'use strict';
  const P=window.PI,E=P.e,M=PIPublishModel,postText=P.postText,defaultPost=P.defaultPost,planTasks=P.planTasks;
  const rowActions=buttons=>`<div class="ex-row-actions">${buttons.join('')}</div>`;
  P.taskList=()=>{
    const all=[...P.db.tasks].filter(t=>!P.ui.createdTaskIds||P.ui.createdTaskIds.includes(t.id)).sort((a,b)=>({failed:0,submitted:1,waiting:2,preparing:3,ready:4,published:5,cancelled:6}[a.status]??7)-({failed:0,submitted:1,waiting:2,preparing:3,ready:4,published:5,cancelled:6}[b.status]??7)||String(a.scheduled).localeCompare(String(b.scheduled))).map(t=>({...t,platform:P.account(t.accountId)?.platform,accountName:P.account(t.accountId)?.name})),rows=P.filterRows(all),visible=P.pageRows(rows);
    const selection=P.selection('条发布任务',P.btn('连续发布',()=>P.openTaskWork([...P.ui.selected],'assisted'))+P.btn('自动发布',()=>P.openTaskWork([...P.ui.selected],'auto')));
    return P.head('发布任务','查看每条发布任务的安排、进度和结果。',P.btn(P.icon('Add')+'新建发布任务',()=>P.beginPublish(),'primary'))+P.tabs('publish','tasks')+(P.ui.createdTaskIds?'<div class="ops-note">已定位本次创建的 '+P.ui.createdTaskIds.length+' 条发布任务。'+P.btn('查看全部',()=>{P.ui.createdTaskIds=null;P.ui.page=1;P.render();},'text')+'</div>':'')+`<section class="ex-panel">${P.filterPanel('内容／账号／计划 ID',{mode:true})}${P.stateChips([['','全部'],['ready','待发布'],['preparing','准备中'],['waiting','待人工提交'],['failed','未完成'],['submitted','待补链接'],['published','已发布'],['cancelled','已取消']])}${selection}<div class="ex-table-scroll"><table class="ex-table pi-table"><thead><tr><th class="pi-selection-col">${P.checkAll(visible.filter(t=>['ready','failed','waiting','preparing'].includes(t.status)).map(t=>t.id))}</th>${['发布 ID','发布内容','发布账号','来源','方式／通道','状态 / 计划时间','操作'].map(v=>`<th>${v}</th>`).join('')}</tr></thead><tbody>${visible.map(t=>{
      const a=[];
      if(t.status==='submitted')a.push(P.rowBtn('补链接',()=>P.backfill(P.task(t.id)),true));
      else if(t.status==='waiting')a.push(P.rowBtn('打开窗口',()=>P.platformWindow(P.task(t.id)),true));
      else if(!['cancelled','published'].includes(t.status))a.push(P.rowBtn(t.status==='failed'?'处理异常':t.scheduled>M.now?'查看排期':'进入执行',()=>P.openTaskWork([t.id],t.mode),true));
      a.push(P.rowBtn('详情',()=>P.taskDetail(t.id)));
      if(t.status==='ready')a.push(P.rowBtn(t.planId?'调整安排':'编辑',()=>{if(t.planId){P.newPlanDraft(P.plan(t.planId));P.nav('publish/plan/new');}else P.openCompose(P.task(t.id));}));

      return `<tr class="ex-resource-row"><td class="pi-selection-col">${['ready','failed','waiting','preparing'].includes(t.status)?P.rowCheck(t.id):''}</td><td class="pi-id">${t.id}</td><td>${P.media(P.asset(t.assetId),t.title)}</td><td>${E(P.account(t.accountId).name)}<small>${P.account(t.accountId).platform}</small></td><td>${P.taskSource(t)}</td><td>${P.mode(t.mode)}<small>${P.channel(t.channel)}</small></td><td>${P.status(t.status,t.status==='ready'?'待发布':null)}${t.status==='failed'?`<p class="pi-state-note">需要重新登录</p>`:''}<small>${P.date(t.scheduled)}</small></td><td>${rowActions(a)}</td></tr>`;
    }).join('')||`<tr><td colspan="8" class="pi-empty-cell">${P.empty('没有匹配的发布任务','调整筛选条件，或选择内容开始发布。',P.btn('清除筛选',()=>{P.ui.filters[P.ui.route]={};P.render();}))}</td></tr>`}</tbody></table></div>${P.pagination(rows.length)}</section>`;
  };
  P.openCompose=(task=null)=>{
    P.cd={platform:task?P.platformOf(task):'B站',id:task?.id||null,title:task?.title||P.db.assets[0].title,assetIds:task?[task.assetId]:[4101],accountIds:task?[task.accountId]:[101],mode:task?.mode||'auto',channel:task?.channel||'browser',description:task?.description||'本期整理了实用思路与操作细节，欢迎交流。',tags:task?.tags||'游戏攻略,新手入门',partition:task?.partition||'单机游戏',distribution:'sequence',post:JSON.parse(JSON.stringify(task?.post||defaultPost(task?P.platformOf(task):'B站'))),error:''};let d;
    const makeRows=()=>{const c=P.cd;return (c.distribution==='all'||c.assetIds.length===1)?c.assetIds.flatMap(assetId=>c.accountIds.map(accountId=>({assetId,accountId}))):c.assetIds.map((assetId,i)=>({assetId,accountId:c.accountIds[i%c.accountIds.length]}));};
    const save=start=>{const c=P.cd;if(!c.assetIds.length||!c.accountIds.length||!c.title.trim()){c.error='请选择内容、账号并填写标题。';d.paint();return;}if(c.accountIds.some(id=>P.account(id).platform!==c.platform)||c.channel==='api'&&c.accountIds.some(id=>!P.account(id).channels.includes('api'))){c.error='已选账号包含未接入 API 的账号，请调整账号或改用浏览器模拟。';d.paint();return;}const postError=P.postError(c.post,c.platform,c.accountIds);if(postError){c.error=postError;d.paint();return;}const ids=[];
      makeRows().forEach((r,i)=>{const old=i===0&&c.id?P.task(c.id):null,id=old?.id||P.id(),t={...(old||{}),...r,id,platform:c.platform,sourceAssetId:r.assetId,planId:old?.planId||null,title:c.assetIds.length===1||i===0?c.title:P.asset(r.assetId).title,mode:c.mode,channel:c.channel,description:c.description,tags:c.tags,partition:c.partition,status:'ready',phase:0,post:JSON.parse(JSON.stringify(c.post)),guideStatus:'pending',initStatus:'pending',created:'10-30 14:00',scheduled:'2026-10-30T14:00',url:''};if(old)P.db.tasks.splice(P.db.tasks.indexOf(old),1,t);else P.db.tasks.unshift(t);ids.push(id);});
      P.save();P.close(d,true);P.nav('publish/tasks');if(start)P.openTaskWork(ids,c.mode);else P.toast('已保存演示发布任务');
    };
    d=P.open({title:task?'编辑发布任务':'普通发布',kind:'ex-drawer complex',form:true,body:()=>{
      const c=P.cd;return `${c.error?`<p class="pi-form-error" role="alert">${E(c.error)}</p>`:''}<div class="pi-publish-edit"><aside><div class="pi-video-preview">${P.icon('Video')}<span>成片封面预览</span></div><p class="pi-summary-note">${E(P.asset(c.assetIds[0])?.title||'尚未选择成片')}</p><p class="pi-meta">演示素材，不加载真实视频。</p></aside><div>${P.section('内容与账号',`${P.field('发布平台','cd.platform',c.platform,{options:M.platforms,redraw:true})}<div class="pi-resource-select pi-gap-top"><div><strong>${c.assetIds.length} 条成片</strong><p>使用成片标题与封面作为初始值</p></div>${P.btn('选择成片',()=>P.pick('assets',c.assetIds,ids=>{c.assetIds=ids;}))}</div><div class="pi-resource-select"><div><strong>${c.accountIds.length} 个发布账号</strong><p>${c.accountIds.map(id=>P.account(id).name).join('、')}</p></div>${P.btn('选择账号',()=>P.pick('accounts',c.accountIds,ids=>{c.accountIds=ids;},{platform:c.platform,channel:c.channel,title:'选择'+c.platform+'发布账号'}))}</div>${c.assetIds.length>1?P.field('内容分配','cd.distribution',c.distribution,{options:[['sequence','成片依次分配给所选账号'],['all','每条成片发到所有所选账号']],redraw:true}):''}`,true)}${P.section('发布内容',P.field(c.assetIds.length>1?'首条标题（其他内容使用各自标题）':'标题','cd.title',c.title,{required:true})+P.field('简介','cd.description',c.description,{area:true})+`<div class="ex-form-container"><div class="ex-form-grid">${P.field('标签','cd.tags',c.tags)}${P.field('分区／分类','cd.partition',c.partition,{options:c.platform==='B站'?['单机游戏','网络游戏','游戏攻略']:['游戏','资讯','生活']})}</div></div>`,true)}</div></div>${P.section('执行方式',P.radios('如何完成发布','cd.mode',c.mode,[['auto','全自动','自动填写并最终提交。'],['assisted','人工连续','系统准备，人工微调后提交。']])+P.field('发布通道','cd.channel',c.channel,{options:[['browser','浏览器模拟'],...(M.eligible(P.db,c.platform,'api').length?[['api','API发布']]:[])],redraw:true}),true)}${P.section('发布后动作',P.postEditor('cd.post',c.post,c.platform),true)}`;
    },footer:()=>`<span class="pi-meta">将生成 ${makeRows().length} 条发布安排 · 仅演示</span>`+P.btn('取消',()=>P.close(d))+P.btn('保存待发布',()=>save(false))+P.btn('开始演示',()=>save(true),'primary')});
    d.addEventListener('close',()=>{if(P.ui.route==='publish/compose')P.nav('publish/tasks');});return d;
  };
  P.taskDetail=id=>{
    const t=P.task(id);let d;d=P.open({title:'发布任务详情',kind:'ex-drawer',body:()=>P.section('发布信息',P.facts([['发布 ID',t.id],['状态',P.status(t.status,t.status==='ready'?'待发布':null)],['标题',E(t.title),true],['发布账号',E(P.account(t.accountId).name)],['平台',P.account(t.accountId).platform],['执行方式',P.mode(t.mode)],['发布通道',P.channel(t.channel)],['关联成片',P.link(P.asset(t.assetId).title,()=>P.detailAsset(t.assetId)),true],['任务来源',P.taskSource(t),true],['计划时间',P.date(t.scheduled)],['实际发布时间',t.published||'尚未发布'],['作品链接',t.url?OPS.reference('打开作品',t.url):'尚未取得',true]]))+P.section('发布内容',P.facts([['简介',E(t.description),true],['标签',E(t.tags)],['分类',E(t.partition)]]))+P.section('发布后动作',P.facts([['动作配置',postText(t.post),true],['引导评论',t.post?.guide?P.status(t.guideStatus||'pending'): '未开启'],['初始互动',t.post?.init?P.status(t.initStatus||'pending'):'未开启'],['引导文案',E(t.post?.guideText||'—'),true]]))+(t.status==='published'?`<div class="pi-demo-strip"><strong>演示控制</strong>${P.btn('推进发布后动作',()=>{P.advancePost(t);d.paint();P.render();})}<span>手动展示引导评论与初始互动的衔接。</span></div>`:'')+(t.reason?P.section('需要处理',`<p>${E(t.reason)}</p>`):''),footer:()=>['ready','failed'].includes(t.status)?P.btn('取消发布安排',()=>{t.status='cancelled';P.save();P.close(d,true);P.render();P.toast('已取消发布安排');}):''});return d;
  };
  P.backfill=t=>{
    P.bf={url:t.url||'https://demo.invalid/works/'+t.id,time:M.now,error:''};let d;
    d=P.open({title:'补充发布链接',kind:'medium',form:true,body:()=>`${P.facts([['发布账号',E(P.account(t.accountId).name)],['发布 ID',t.id],['标题',E(t.title),true]])}<div style="margin-top:20px">${P.field('发布链接','bf.url',P.bf.url,{required:true})}${P.field('发布时间','bf.time',P.bf.time,{type:'datetime-local',required:true})}${P.bf.error?`<p class="ex-error">${P.bf.error}</p>`:''}</div>`,footer:()=>P.btn('取消',()=>P.close(d))+P.btn('保存',()=>{try{const url=new URL(P.bf.url);if(!['http:','https:'].includes(url.protocol))throw Error();}catch{P.bf.error='请填写完整的 http 或 https 作品链接。';d.paint();return;}P.completePublication(t,P.bf.url);P.close(d,true);P.render();P.toast('演示链接已补充，后续动作已关联');},'primary')});
  };
  P.platformWindow=t=>{
    P.platformDraft={title:t.title,description:t.description||'',tags:t.tags||'',error:''};let d;
    d=P.open({title:t.channel==='api'?'API提交预览 · 演示':'比特窗口预览 · 演示',kind:'wide',form:true,body:()=>`<div class="pi-platform-demo"><span>${E(P.account(t.accountId).window)} · ${P.account(t.accountId).platform}</span><span>本地模拟页面，不连接平台</span></div><div class="pi-publish-edit" style="margin-top:24px"><aside><div class="pi-video-preview">${P.icon('Video')}<span>已准备演示视频</span></div></aside><div>${P.field('标题','platformDraft.title',P.platformDraft.title,{required:true})}${P.field('简介','platformDraft.description',P.platformDraft.description,{area:true})}${P.field('标签','platformDraft.tags',P.platformDraft.tags)}${P.platformDraft.error?`<p class="ex-error">${P.platformDraft.error}</p>`:''}</div></div>`,footer:()=>P.btn('返回工作区',()=>P.close(d))+P.btn('模拟平台提交',()=>{if(!P.platformDraft.title.trim()){P.platformDraft.error='标题不能为空。';d.paint();return;}t.title=P.platformDraft.title;t.description=P.platformDraft.description;t.tags=P.platformDraft.tags;t.checkPending=true;P.save();P.close(d,true);P.render();P.toast('已记录平台提交，请回工作区检查发布结果');},'primary',P.isTaskPaused(t)||(t.status!=='waiting'&&t.status!=='preparing'))});
  };
  P.ensureInitialInteraction=t=>{

    if(t.post?.init&&!P.db.interactions.some(it=>it.publicationId===t.id)){
      const pool=t.post.accountIds.filter(id=>id!==t.accountId&&P.account(id)?.platform===P.account(t.accountId).platform),offset=pool.length?t.assetId%pool.length:0,accounts=[...pool.slice(offset),...pool.slice(0,offset)].slice(0,t.post.perWork);
      P.db.interactions.unshift({id:P.id(),publicationId:t.id,replicationId:t.replicationId||null,planId:t.planId,title:t.title,target:{title:t.title,platform:P.account(t.accountId).platform,author:P.account(t.accountId).name,url:t.url,kind:'internal',assetId:t.assetId},source:t.planId?'发布计划':'发布后动作',actions:[...t.post.actions],status:t.initStatus==='done'?'completed':accounts.length?'ready':'attention',date:P.date(M.now),rows:accounts.map((id,i)=>({accountId:id,comment:t.post.comments[i%Math.max(1,t.post.comments.length)]||'',status:t.initStatus==='done'?'success':'pending',reason:''}))});
    }
    P.save();
  };
  P.completePublication=(t,url)=>{
    if(t.status==='published')return;
    t.status='published';t.phase=4;t.published=P.bf?.time&&url?P.date(P.bf.time):P.date(P.ui.demoNow||M.now);t.url=url||'https://demo.invalid/works/'+t.id;t.reason='';
    P.ensureInitialInteraction(t);
    const p=P.plan(t.planId);if(p){const rows=planTasks(p);if(rows.every(t=>t.status==='published'))p.status='completed';}
    P.save();
  };
  P.advancePost=t=>{
    P.ensureInitialInteraction(t);
    if(t.post?.guide&&t.guideStatus!=='done'){t.guideStatus='done';P.toast('演示：作者引导评论已完成');}
    else if(t.post?.init&&t.initStatus!=='done'){const it=P.db.interactions.find(it=>it.publicationId===t.id);if(!it?.rows.length){t.initStatus='attention';P.toast('演示：没有匹配平台的互动账号，请调整配置');P.save();return;}it.rows.forEach(r=>r.status='success');it.status='completed';t.initStatus='done';P.toast('演示：初始互动已完成，可在主动互动查看');}
    else P.toast('本作品的已配置后续动作均已完成');P.save();
  };
  P.renderPublishing=()=>{
    if(P.ui.route.startsWith('publish/plan/'))return P.planWizard();
    if(['publish/workbench','publish/auto'].includes(P.ui.route)){const mode=P.ui.route==='publish/auto'?'auto':'assisted';if(P.ui.workMode!==mode){P.ui.workIds=null;P.ui.workPlan=null;P.ui.workId=null;P.ui.workPaused=false;}return P.workbench();}
    if(['publish/tasks','publish/compose'].includes(P.ui.route))return P.taskList();
    return P.planList();
  };
  P.wirePublishing=()=>{if(P.ui.route==='publish/compose'&&!P.dialogs.length)P.openCompose();};
  document.addEventListener('change',e=>{
    const el=e.target;
    if(el.dataset.postAction){const path=el.dataset.postAction.split('.');let post=P;path.forEach(k=>post=post[k]);post.actions=el.checked?[...new Set([...post.actions,el.value])]:post.actions.filter(a=>a!==el.value);}
  });
  document.addEventListener('input',e=>{const el=e.target;if(el.dataset.postComments){let post=P;el.dataset.postComments.split('.').forEach(k=>post=post[k]);post.comments=el.value.split('\n').map(s=>s.trim()).filter(Boolean);}});
})();
