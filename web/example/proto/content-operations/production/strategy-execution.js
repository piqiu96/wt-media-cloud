/* 执行预览与记录；离线演示只在明确确认后生成任务。 */
(() => {
  const C=CP,O=OPS,E=C.e,{table,copy}=C.prod;
  const allowed=s=>s&&C.access(s)&&s.enabled;
  const active=s=>C.db.tasks.find(t=>t.strategyId===s.id&&['pending','running'].includes(t.status));
  const preview=s=>{
    const today=C.fullTime().slice(0,10),used=C.db.runs.filter(r=>r.strategyId===s.id&&r.created?.startsWith(today)).reduce((n,r)=>n+(r.materialCount||0),0);
    return ProductionModel.preview(s,C.db.materials,C.db.tasks,used);
  };
  C.executeStrategy=s=>{
    if(!allowed(s)){C.toast('策略已停用或无权使用');return;}
    if(s.versions.some(v=>v.platforms.some(p=>!O.platforms.includes(p)))){C.toast('成片设置包含暂不支持的平台，请先编辑策略');return;}
    let d,submitting=false,display=preview(s),existing=active(s);
    const fingerprint=JSON.stringify(s);
    const submit=()=>{
      if(submitting)return;
      if(!allowed(s)||fingerprint!==JSON.stringify(s)){C.toast('策略已变更，请关闭后重新预览');return;}
      existing=active(s);const fresh=preview(s);
      if(existing){C.close(d,true);C.executeStrategy(s);return;}
      if(JSON.stringify(fresh)!==JSON.stringify(display)){display=fresh;d.paint();C.toast('选材结果或额度已变化，请重新确认');return;}
      if(!display.ids.length)return;
      if(!C.prod.valid(s.versions,s.template)){C.toast('策略参数不完整，请先检查默认设置');return;}
      submitting=true;d.paint();
      const runKey='run-'+C.id(),snapshot=copy(s);let taskIds;try{taskIds=C.submitTasks({strategy:s,ids:display.ids,vs:s.versions,owner:s.operator,trigger:'策略执行',name:s.name+' · '+C.fullTime(),runKey,requestKey:runKey});}catch(e){submitting=false;C.toast(e.message);d.paint();return;}
      display.ids.forEach(O.prepareMaterial);
      C.db.runs.unshift({runKey,strategyId:s.id,created:C.fullTime(),trigger:'立即执行',actor:O.actor,materialCount:display.ids.length,taskIds,details:copy(display.details),snapshot,reason:'选入 '+display.ids.length+' 条素材，创建 '+taskIds.length+' 条任务'});
      C.save();C.close(d,true);C.render();C.toast('已创建任务，可在执行记录中查看进度');C.batchDetail(C.batchKey(C.get('tasks',taskIds[0])));
    };
    d=C.open({title:existing?'已有任务执行中':'立即执行 · 预览',kind:existing?'production-active-dialog':'ex-drawer complex',body:()=>{
      if(existing){const n=ProductionModel.counts(C.batchRows(C.batchKey(existing)).filter(C.access));return `<p class="production-active-message">「${E(s.name)}」还有 ${n.active} 条任务未结束。</p>`+C.facts([[C.realBatchId(existing)?'批次 ID':'任务 ID',E(C.realBatchId(existing)||existing.id)],['任务状态',C.status(existing.status)],['已结束',n.ended+' / '+n.total+' 条'],['创建时间',E(existing.created||'未记录')]]);}
      return `<div class="production-execution-summary"><strong>本次选入 ${display.ids.length} 条素材，预计生成 ${display.tasks} 个成片</strong>${C.help('预计成片','素材数 × 各组成片设置份数之和。平台不增加份数，实际产出以成功任务为准。')}</div>`+C.facts([['合成策略',E(s.name)],['选材类目',E(s.category)],['每次素材上限',s.singleLimit+' 条'],...(display.remaining===null?[]:[['今日剩余额度',display.remaining+' 条']])])+(!display.ids.length?'<p class="ops-note">没有可执行素材，请查看选材原因。</p>':'')+C.section('成片设置',table(['成片设置','适用平台','每条素材份数','预计成片'],s.versions.map(v=>`<tr><td>${E(v.name)}</td><td>${E(v.platforms.join(' / '))}</td><td>${v.qty} 份</td><td>${v.qty*display.ids.length} 个</td></tr>`).join('')))+C.section('选材结果 '+C.help('选材依据','限定策略类目，优先选择最新发布的素材；先排除，再匹配选入规则，跳过本策略已有任务、视频不可准备或超过额度的素材。'),C.prod.selectionTable(display.details))+(s.executionMode==='scheduled'?'<p class="ops-muted">本次计入今日额度，原定时计划保持不变。</p>':'');
    },footer:()=>existing?C.btn('查看进度',()=>{C.close(d,true);if(C.realBatchId(existing))C.batchDetail(C.batchKey(existing));else C.taskDetail(existing.id);},'primary'):display.ids.length?C.btn('取消',()=>C.close(d,true),'',submitting)+C.btn(submitting?'创建中…':'确认执行',submit,'primary',submitting):''});return d;
  };

  C.runDetail=key=>{
    const r=C.db.runs.find(x=>x.runKey===key),s=r&&C.get('strategies',r.strategyId);if(!r||!C.guard(s))return;
    const tasks=(r.taskIds||[]).map(id=>C.get('tasks',id)).filter(C.access);
    C.open({title:'执行记录详情',kind:'ex-drawer complex',body:()=>C.facts([['策略',E(r.snapshot?.name||s.name)],['执行时间',E(r.created)],['触发方式',E(r.trigger)],['操作人',E(r.actor||'未记录')],['任务批次',tasks[0]?E(tasks[0].batchId||tasks[0].runKey):'—（未创建任务）'],['选入素材',r.materialCount+' 条'],['创建任务',tasks.length+' 条'],['选材说明',E(r.reason||'具体原因未记录'),true]])+C.prod.progress(tasks)+C.section('选材结果',r.details?C.prod.selectionTable(r.details):'<p>历史记录未保存逐条选材原因。</p>')+C.section('执行配置快照',r.snapshot?C.facts([['模板',E(r.snapshot.template)],['版本',E(r.snapshot.templateVersion||'未记录')],['成片设置',E(r.snapshot.versions.map(v=>v.name+' × '+v.qty).join('，')),true]]):'<p>历史配置未记录，不以当前设置补造。</p>'),footer:()=>tasks.length?C.btn('任务',()=>C.batchDetail(C.batchKey(tasks[0])))+(C.db.outputs.some(o=>r.taskIds.includes(o.taskId))?C.btn('成片',()=>C.outputBatch(C.batchKey(tasks[0]))):''):''});
  };
  C.strategyDetail=id=>{
    const s=C.get('strategies',id);if(!C.guard(s))return;let tab='overview',f={trigger:'',from:'',to:''},draft={...f},page=1,d;
    d=C.open({title:'合成策略详情',kind:'ex-drawer complex',body:()=>O.tabs([['overview','策略信息'],['runs','执行记录']],tab,v=>{tab=v;d.paint();})+(tab==='overview'?C.facts([['策略 ID',s.id],['名称',E(s.name)],['模板',C.link(s.template,()=>C.templateDetail(s.template,s.templateVersion))],['版本',E(s.templateVersion||'未记录')],['类目',E(s.category)],['团队',E(s.team)],['负责人',E(s.operator)],['执行方式',s.executionMode==='scheduled'?'定时执行 · 北京时间':'手动执行'],['每次素材上限',s.singleLimit],...(s.executionMode==='scheduled'?[['每日素材上限',s.dailyLimit],['执行计划',s.scheduleMode==='daily'?'每天 '+s.dailyTime:'每 '+s.interval+' 分钟']]:[])])+C.section('默认成片设置',s.versions.map(v=>`<article class="ops-version"><strong>${E(v.name)} × ${v.qty} 份 · ${E(v.platforms.join(' / '))}</strong>${C.facts((C.prod.schemas[s.template]||[]).map(([key,label])=>[label,E(C.prod.parameterValue(key,v[key]))]))}</article>`).join(''))+C.section('选材条件 '+C.help('选材规则','先排除，再按优先级选入。缺少必要指标或已在本策略创建过任务的素材将跳过。'),`<div class="production-rule-summary">`+[...s.rules.filter(r=>r.action==='filter'),...s.rules.filter(r=>r.action==='match')].map((r,i)=>`<div><span>${C.tag(r.action==='filter'?'排除':'选入',r.action==='filter'?'':'info')}</span><p>${r.conditions.map(c=>E(({likes:'点赞数',favorites:'收藏数',recentDays:'距发布天数'})[c.field])+' '+E(({gte:'≥',lte:'≤',eq:'='})[c.operator])+' '+c.value).join(r.logic==='all'?' 且 ':' 或 ')}</p></div>`).join('')+'</div>'):(()=>{
      const rows=C.db.runs.filter(r=>r.strategyId===id&&(!f.trigger||r.trigger===f.trigger)&&(!f.from||/^\d{4}-/.test(r.created)&&r.created.slice(0,10)>=f.from)&&(!f.to||/^\d{4}-/.test(r.created)&&r.created.slice(0,10)<=f.to)).sort((a,b)=>String(b.created).localeCompare(String(a.created)));page=Math.min(page,Math.max(1,Math.ceil(rows.length/8)));
      return `<section class="ex-panel ex-list-pattern production-records"><form class="ex-filters ex-filters-stacked" data-run-query>`+C.select('trigger','触发方式',draft.trigger,[['立即执行','手动点击'],['定时执行','定时触发']])+C.prod.dateRange('执行日期',draft.from,draft.to)+`<div class="ex-actions"><button type="submit" class="ex-button primary">查询</button>${C.btn('重置',()=>{f={trigger:'',from:'',to:''};draft={...f};page=1;d.paint();})}</div></form>`+table(['执行时间','批次 ID','触发 / 操作人','选入素材','创建任务','制作进度','操作'],rows.slice((page-1)*8,page*8).map(r=>{
        const tasks=(r.taskIds||[]).map(id=>C.get('tasks',id)).filter(C.access),n=ProductionModel.counts(tasks);
        return `<tr><td>${E(r.created)}</td><td>${E(tasks[0]?.batchId||tasks[0]?.runKey||'—')}</td><td>${E(r.trigger)}<small>${E(r.actor||'未记录')}</small></td><td>${r.materialCount} 条</td><td>${tasks.length} 条</td><td>${n.total?`已结束 ${n.ended} / ${n.total}<small>成功 ${n.success} · 失败 ${n.failed} · 取消 ${n.cancelled}</small>`:'未创建任务'}</td><td>${C.row('详情',()=>C.runDetail(r.runKey))}</td></tr>`;
      }).join(''))+C.prod.pager(rows.length,page,8,p=>{page=p;d.paint();})+'</section>';
    })()),change:(k,v)=>{draft[k]=v;},wire:el=>el.querySelector('[data-run-query]')?.addEventListener('submit',event=>{event.preventDefault();f={...draft};page=1;d.paint();})});
  };
})();
