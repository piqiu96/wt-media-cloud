/* 离线原型共用规则：只计算，不产生选用、任务或文件副作用。 */
(() => {
  const metric=(c,m,now)=>{
      const published=/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}/.test(m.published||'')?Date.parse(m.published.replace(' ','T')+':00+08:00'):NaN;
      const raw=c.field==='recentDays'?(Number.isFinite(published)&&published<=now?Math.floor((now-published)/86400000):null):m[c.field];
      if(raw==null||raw===''||!Number.isFinite(Number(raw)))return null;
      return Number(raw);
  };
  const condition=(c,m,now)=>{const value=metric(c,m,now),target=Number(c.value);return value===null?null:c.operator==='gte'?value>=target:c.operator==='lte'?value<=target:value===target;};
  const test=(r,m,now)=>{
    const results=r.conditions.map(c=>condition(c,m,now));
    return r.logic==='any'?(results.includes(true)?true:results.includes(null)?null:false):(results.includes(false)?false:results.includes(null)?null:true);
  };
  const evidence=(r,m,now)=>r.conditions.map(c=>{
    const value=metric(c,m,now),label=({likes:'点赞',favorites:'收藏',recentDays:'距发布天数'})[c.field]||c.field;
    if(value===null)return '缺少'+label+'数据';
    const expected=({gte:'≥',lte:'≤',eq:'='})[c.operator]||'=',actual=condition(c,m,now)?expected:({gte:'<',lte:'>',eq:'≠'})[c.operator]||'≠';
    return r.conditions.length===1?label+' '+value+' '+actual+' '+c.value:label+' '+value+'（要求 '+expected+' '+c.value+'）';
  }).join(r.logic==='any'?'；或 ':'；');
  const matches=(rules,m,now=Date.now())=>{
    for(const r of rules.filter(r=>r.action==='filter')){const result=test(r,m,now);if(result!==false)return {selected:false,reason:(result===null?'无法核对排除规则：':'命中排除规则：')+evidence(r,m,now)};}
    const index=rules.findIndex(r=>r.action==='match'&&test(r,m,now)===true);
    return {selected:index>=0,rule:index,reason:index>=0?evidence(rules[index],m,now):rules.filter(r=>r.action==='match').map((r,i)=>(rules.filter(x=>x.action==='match').length>1?'规则 '+(i+1)+'：':'')+evidence(r,m,now)).join('；')||'未配置选入规则'};
  };
  const canPrepare=m=>m.cloudFileExists===true&&m.cloudUrlVerified===true||m.cloudState==='ready'||!['expired','failed'].includes(m.cloudState);
  const preview=(s,materials,tasks,used=0)=>{
    const remaining=s.executionMode==='scheduled'?Math.max(0,s.dailyLimit-used):null;
    const limit=remaining==null?s.singleLimit:Math.min(s.singleLimit,remaining);
    const reasons={},eligible=[],details=[];
    materials.filter(m=>m.category===s.category&&(!s.team||!m.team||m.team===s.team)).sort((a,b)=>String(b.published||'').localeCompare(String(a.published||''))||b.id-a.id).forEach(m=>{
      const result=matches(s.rules,m);
      const reason=!result.selected?result.reason:tasks.some(t=>t.strategyId===s.id&&t.materialId===m.id)?'本策略已有任务':!canPrepare(m)?(m.cloudState==='expired'?'源视频失效':m.cloudError||'视频准备失败'):eligible.length>=limit?(remaining!==null&&remaining<=eligible.length?'今日素材额度已用完':'达到每次素材上限'):'';
      if(reason)reasons[reason]=(reasons[reason]||0)+1;else eligible.push(m.id);
      details.push({id:m.id,title:m.title||String(m.id),selected:!reason,reason:reason||result.reason});
    });
    return {ids:eligible,tasks:eligible.length*s.versions.reduce((n,v)=>n+Number(v.qty),0),remaining,limit,reasons,details};
  };
  const validOverrides=(base,values,editable)=>base.length===values.length&&base.every((v,i)=>{
    const keys=new Set([...Object.keys(v),...Object.keys(values[i])]);
    return [...keys].every(k=>editable.includes(k)||JSON.stringify(v[k])===JSON.stringify(values[i][k]));
  });
  const counts=rows=>{
    const n={total:rows.length,success:0,failed:0,cancelled:0,pending:0,running:0};
    rows.forEach(t=>n[t.status]=(n[t.status]||0)+1);n.ended=n.success+n.failed+n.cancelled;n.active=n.pending+n.running;return n;
  };
  const groupOutputs=rows=>{
    const groups=new Map();
    rows.forEach(output=>{if(output.materialId==null)return;const group=groups.get(output.materialId)||{materialId:output.materialId,outputs:[]};group.outputs.push(output);groups.set(output.materialId,group);});
    return [...groups.values()].map(group=>({...group,outputs:group.outputs.slice().sort((a,b)=>String(b.date||'').localeCompare(String(a.date||''))||b.id-a.id)})).sort((a,b)=>String(b.outputs[0].date||'').localeCompare(String(a.outputs[0].date||''))||b.outputs[0].id-a.outputs[0].id);
  };
  const executionHistory=task=>{
    const groups=new Map();let attempt=1;
    (task.events||[]).forEach(event=>{
      const parsed=String(event.text||'').match(/第\s*(\d+)\s*次执行/),explicit=Number(event.attempt||parsed?.[1]);
      if(explicit>0)attempt=explicit;
      const group=groups.get(attempt)||{attempt,events:[]};
      group.events.push({...event,text:String(event.text||'').replace(/[，,]\s*第\s*\d+\s*次执行/,'')});groups.set(attempt,group);
    });
    const current=Math.max(Number(task.attempt)||1,attempt);
    if(!groups.has(current))groups.set(current,{attempt:current,events:[]});
    return [...groups.values()].sort((a,b)=>b.attempt-a.attempt).map(group=>({...group,current:group.attempt===current}));
  };
  window.ProductionModel={matches,preview,validOverrides,counts,canPrepare,groupOutputs,executionHistory};
})();
