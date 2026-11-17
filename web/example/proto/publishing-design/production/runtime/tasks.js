(() => {
  const C=CP,O=OPS,E=C.e,{copy,table,versionForm,wireVersions,updateCount,changeVersion,valid}=C.prod;
  C.create=(opts={})=>{
    let owner=O.actor,ids=[...(opts.materialIds||[])],strategyId=opts.strategyId||'',name=opts.name||'',error='',nameError=false,d;const requestKey='manual-'+C.id();let submitting=false;
    if(strategyId&&C.canManage())owner=C.get('strategies',strategyId)?.operator||owner;
    let category=opts.category||C.get('strategies',strategyId)?.category||C.get('materials',ids[0])?.category||'';
    const candidates=()=>C.db.strategies.filter(s=>s.enabled&&s.operator===owner&&category&&s.category===category);
    if(!candidates().some(s=>s.id===+strategyId))strategyId='';
    const strategy=()=>C.get('strategies',strategyId);
    // A task draft always owns a deep copy; editing it must never change saved strategy defaults.
    let defaults=copy(strategy()?.versions||[]),vs=copy(defaults);
    const missing=()=>ids.filter(id=>C.usage(id,owner)?.status!=='active');
    const bad=()=>ids.filter(id=>!C.get('materials',id)||C.get('materials',id).category!==category||!ProductionModel.canPrepare(C.get('materials',id)));
    const count=()=>ids.length*vs.reduce((n,v)=>n+Number(v.qty||0),0);
    const waiting=()=>ids.filter(id=>C.get('materials',id)?.cloudState!=='ready').length*vs.reduce((n,v)=>n+Number(v.qty||0),0);
    const confirmChange=(title,body,commit)=>{let q;q=C.open({title,body:()=>body,footer:()=>C.btn('继续编辑',()=>C.close(q,true))+C.btn('确认更换',()=>{C.close(q,true);commit();},'primary')});};
    const paint=()=>{error='';d.dirty=true;d.paint();};
    const changeScope=(key,value)=>{
      const commit=()=>{if(key==='category')category=value;else owner=value;strategyId='';defaults=[];vs=[];ids=[];paint();};
      d.paint();
      if(strategyId||ids.length)confirmChange(key==='category'?'更换类目？':'更换任务负责人？',`<p>更换后清空已选策略、${ids.length} 条素材及本次成片设置。任务名称保留。</p><p>已经确认加入的个人素材和转存保持不变。</p>`,commit);else commit();
    };
    const update=()=>{
      updateCount(d,vs,ids.length);
      const total=count(),wait=waiting();
      d.querySelectorAll('[data-create-total]').forEach(el=>el.textContent=Number.isFinite(total)?total:'—');
      const states=d.querySelector('[data-create-waiting]');if(states)states.textContent=`${total-wait} 条可排队 · ${wait} 条等待素材转存。`;
      d.querySelectorAll('[data-version-row]').forEach(row=>{const v=vs[+row.dataset.versionRow];row.cells[0].textContent=v.name;row.cells[1].textContent=v.platforms.join(' / ');row.cells[2].textContent=v.qty+' 份';row.cells[3].textContent=v.qty*ids.length+' 个';});
    };
    const configure=index=>{
      const template=strategy().template,base=vs[index],value=copy(base),editable=C.prod.editable[template]||[];let dialog,message='';
      const changed=()=>editable.some(key=>JSON.stringify(value[key])!==JSON.stringify(defaults[index][key]));
      dialog=C.open({title:base.name+' · 模板参数',kind:'production-parameter-dialog',form:true,body:()=>`<div class="production-inline-note">${E(template)} · 调整仅用于本次任务。${C.help('固定参数','灰色参数由策略固定。如需修改，请联系策略负责人。')}</div>${message?`<p class="ops-error" role="alert">${E(message)}</p>`:''}<div class="ex-form-grid">`+(C.prod.schemas[template]||[]).map(([key,label,type])=>C.field(label,key,value[key]??'',{type,disabled:!editable.includes(key),full:type==='url',hint:editable.includes(key)?C.prod.parameterHint(key)||(type==='number'?'请输入非负整数，单位秒。':'使用分:秒格式，如 00:30。'):'由策略固定，本次任务不可修改。'})).join('')+`</div>`,change:(key,v)=>{if(editable.includes(key)){value[key]=['insertSeconds','maxDuration'].includes(key)?Number(v):v;dialog.querySelector('.ex-overlay-footer button').disabled=!changed();}},footer:()=>C.btn('恢复默认',()=>{editable.forEach(k=>value[k]=copy(defaults[index][k]));message='';dialog.dirty=true;dialog.paint();},'',!changed())+C.btn('取消',()=>C.close(dialog))+C.btn('确定',()=>{if(!valid([value],template)){message='参数无效，请检查时间范围和数值；固定参数异常请联系策略负责人。';dialog.paint();return;}vs[index]=copy(value);C.close(dialog,true);d.dirty=true;d.paint();},'primary')});
    };
    const submit=()=>{
      if(submitting)return;
      error=!category?'请选择类目':!name.trim()?'请填写任务名称':!strategy()||!candidates().includes(strategy())?'请选择当前类目和负责人下的启用策略':!ids.length?'请选择制作素材':bad().length?'所选素材类目不符或来源不可用，请调整选择':missing().length?'请重新确认选用素材，确保加入当前负责人的我的素材':!ProductionModel.validOverrides(defaults,vs,C.prod.editable[strategy()?.template]||[])?'固定配置不允许修改，请重新选择策略':!valid(vs,strategy()?.template)?'请检查策略固定值及模板参数':'';
      if(error){nameError=!name.trim();d.paint();if(nameError){const el=d.querySelector('[data-field="name"]');el.setAttribute('aria-invalid','true');el.focus();}return;}
      const make=()=>{if(submitting)return;submitting=true;let made;try{made=C.submitTasks({strategy:strategy(),ids,vs,owner,name,replicationId:opts.replicationId,followup:opts.followup,requestKey});}catch(e){submitting=false;error=e.message;d.paint();return;}C.close(d,true);C.noticeRoute='tasks';C.notice=`已创建 ${made.length} 条任务，${waiting()} 条等待素材转存。${C.link('详情',()=>C.batchDetail(C.batchKey(C.get('tasks',made[0]))))}`;C.nav('tasks');C.toast('任务已创建');};
      const existing=ids.filter(id=>C.db.tasks.some(t=>t.materialId===id&&t.strategyId===+strategyId));
      if(existing.length){let q;q=C.open({title:'这些素材已用同一策略制作',body:()=>`<p>${existing.length} 条素材已有任务。继续将生成新的任务，已有任务与成片保留。</p>`,footer:()=>C.btn('返回检查',()=>C.close(q,true))+C.btn('创建新任务',()=>{C.close(q,true);make();},'primary')});}else make();
    };
    d=C.open({title:opts.recreate?'再次制作':'新增合成任务',kind:'ex-drawer complex production-task-editor',form:true,discardMessage:'本次任务配置尚未保存。已经确认加入的个人素材和转存不会撤销。',body:()=>{
      const s=strategy();return `${error?`<p class="ops-error" role="alert">${E(error)}</p>`:''}${opts.followup?`<p class="ops-note">来源作品：${E(opts.followup)}。</p>`:''}`+
      C.section('01 · 确定制作范围',`<div class="ex-form-grid">${C.field('任务名称','name',name,{required:true,full:true,placeholder:'例如：剪辑教程 · 第一批',hint:'用于识别本次提交的任务批次。'})}${C.field('类目','category',category,{required:true,disabled:!!opts.lockMaterial,options:[['','请选择类目'],...O.categories]})}${C.canManage()?C.field('任务负责人','owner',owner,{options:O.peopleOptions()}):`<div class="ex-field"><label>任务负责人</label><p>${E(owner)}</p></div>`}</div>`)+
      C.section('02 · 选择合成策略',C.field('合成策略','strategyId',strategyId,{disabled:!category,options:[['',category?'请选择合成策略':'请先选择类目'],...candidates().map(s=>[s.id,s.name+' · '+s.template])]})+`<p class="ops-muted">${s?'模板：'+E(s.template)+'。以下带入策略默认值，仅可调整模板开放的参数。':category&&!candidates().length?'当前类目和负责人下没有可用策略，请检查范围或先新增策略。':'只显示当前类目、负责人下已启用的策略。'}</p>`)+
      C.section('03 · 选择制作素材',`<div class="ops-line"><div><strong>已选 ${ids.length} 条素材</strong><p>${category?'类目：'+E(category)+' · 从我的素材或素材库选择':'先选择类目，确定素材范围。'}</p></div>${C.btn(ids.length?'调整素材':'选择素材',()=>C.pickMaterials(ids,v=>{ids=v;paint();},category,{owner,materialId:opts.lockMaterial?opts.materialIds?.[0]:null}),'',!category)}</div>`+(ids.length?table(['素材 ID','素材','本次处理','操作'],ids.map(id=>{const m=C.get('materials',id);return `<tr><td>${id}</td><td>${C.link(m?.title||'素材已失效',()=>C.materialDetail(id))}</td><td>${m?.cloudState==='ready'?'可进入合成队列':bad().includes(id)?'请调整选择':'等待转存完成'}</td><td>${C.btn('取消选择',()=>{ids=ids.filter(x=>x!==id);paint();},'text',!!opts.lockMaterial)}</td></tr>`;}).join('')):'<p class="ops-muted">尚未选择素材。</p>'))+
      C.section('04 · 确认成片配置',s?`<div class="production-inline-note">按以下配置制作 ${ids.length} 条素材，预计生成 ${count()} 个成片。${C.help('预计成片','按素材数 × 每条素材份数计算。适用平台不会增加份数；实际产出以成功任务为准。')}</div>`+table(['成片设置','适用平台','每条素材份数','预计成片','参数','操作'],vs.map((v,i)=>{const adjusted=(C.prod.editable[s.template]||[]).some(k=>JSON.stringify(v[k])!==JSON.stringify(defaults[i][k]));return `<tr data-version-row="${i}"><td>${E(v.name)}</td><td>${E(v.platforms.join(' / '))}</td><td>${v.qty} 份</td><td>${v.qty*ids.length} 个</td><td>${C.tag(adjusted?'已调整':'策略默认',adjusted?'info':'')}</td><td>${C.row('配置',()=>configure(i))}</td></tr>`;}).join(''))+`<div class="production-total"><span>预计成片</span><strong>${count()} <small>个</small></strong><span>${ids.length} 条素材 × 每条 ${vs.reduce((n,v)=>n+v.qty,0)} 份</span></div>`:'<p class="ops-muted">选择策略后显示成片配置。</p>');
    },change:(k,v,type)=>{
      if(k==='name'){name=v;nameError=false;}
      else if(['category','owner'].includes(k)&&type==='change')changeScope(k,v);
      else if(k==='strategyId'&&type==='change'){
        const commit=()=>{strategyId=+v||'';defaults=copy(strategy()?.versions||[]);vs=copy(defaults);paint();};
        if(strategyId&&JSON.stringify(vs)!==JSON.stringify(defaults)){d.paint();confirmChange('更换合成策略？','<p>素材保留，本次修改的成片设置将替换为新策略的默认值。</p>',commit);}else commit();
      }else if(/^v\./.test(k)){const key=k.split('.')[2];if((C.prod.editable[strategy()?.template]||[]).includes(key))changeVersion(vs,k,v);update();};
    },wire:el=>{
      el.querySelectorAll('[data-platform-version]').forEach(x=>x.addEventListener('change',update));
    },footer:()=>`<span class="ex-help">确认后创建 <strong data-create-total>${count()}</strong> 条任务并加入执行队列</span>`+C.btn('取消',()=>C.close(d))+C.btn('创建任务',submit,'primary',!category||!strategy()||!ids.length)});return d;
  };
  C.finish=ids=>{ids.forEach(id=>{const t=C.get('tasks',id);if(!C.access(t)||t.cancelRequested||!['pending','running'].includes(t.status)||C.db.outputs.some(o=>o.taskId===id))return;t.status='success';t.finished=C.fullTime();(t.events||=[]).push({text:'合成完成，第 '+(t.attempt||1)+' 次执行',time:C.fullTime(),attempt:t.attempt||1});C.db.outputs.unshift({id:C.id(),taskId:id,materialId:t.materialId,name:t.name,owner:t.owner,platforms:[...t.version.platforms],assignees:Object.fromEntries(t.version.platforms.map(p=>[p,t.owner])),date:C.fullTime(),downloaded:false,cloudFileExists:true,cloudUrlVerified:true,cloudUrl:'https://media.example.test/prototype-only.mp4',cloudDemo:true});});C.save();O.sync();C.render();C.dialogs.forEach(d=>d.paint());C.toast('演示合成完成，已生成成片');};
  C.retry=ids=>{
    const targets=[...new Set(ids)].map(id=>C.get('tasks',id)),ready=targets.filter(t=>C.access(t)&&t.status==='failed');let d;
    if(!ready.length){C.toast('没有可重试的失败任务');return;}
    d=C.open({title:'重试失败任务？',body:()=>`<p>可重试 ${ready.length} 条，跳过 ${targets.length-ready.length} 条（无权访问或当前非失败状态）。</p><p>沿用原任务、批次和配置；已有成功成片保持不变。</p>`,footer:()=>C.btn('返回',()=>C.close(d,true))+C.btn('重试',()=>{ready.forEach(t=>{if(!C.access(t)||t.status!=='failed')return;t.status='pending';t.attempt=(t.attempt||1)+1;t.error='';(t.events||=[]).push({text:'重新排队',time:C.fullTime(),attempt:t.attempt});});C.save();C.close(d,true);C.render();C.dialogs.forEach(x=>x.paint());C.toast('已将失败任务重新排队');},'primary')});
  };
  C.cancel=ids=>{
    const targets=[...new Set(ids)].map(id=>C.get('tasks',id)),ready=targets.filter(t=>C.access(t)&&['pending','running'].includes(t.status)&&!t.cancelRequested);let d;
    if(!ready.length){C.toast('没有可取消的未完成任务');return;}
    d=C.open({title:'取消未完成任务？',body:()=>`<p>将请求取消 ${ready.length} 条，跳过 ${targets.length-ready.length} 条（已结束、无权限或正在取消）。</p><p>已有成功任务和成片保留。离线原型将模拟执行侧确认后更新状态。</p>`,footer:()=>C.btn('返回',()=>C.close(d,true))+C.btn('取消任务',()=>{ready.forEach(t=>{if(C.access(t)&&['pending','running'].includes(t.status))t.cancelRequested=true;});C.save();C.close(d,true);C.render();C.dialogs.forEach(x=>x.paint());setTimeout(()=>{ready.forEach(t=>{if(t.cancelRequested&&['pending','running'].includes(t.status)){t.status='cancelled';t.finished=C.fullTime();(t.events||=[]).push({text:'已取消',time:C.fullTime(),attempt:t.attempt||1});}delete t.cancelRequested;});C.save();C.render();C.dialogs.forEach(x=>x.paint());},500);},'primary')});
  };
  C.createFrom=id=>{const t=C.get('tasks',id);if(!C.guard(t))return;const s=C.get('strategies',t.strategyId);if(!s?.enabled||!C.access(s)){C.toast('来源策略停用或不可用，请先修正策略');return;}O.closeAll();C.create({strategyId:t.strategyId,materialIds:[t.materialId],name:(t.batch||s.name)+' · 再次制作',recreate:true,lockMaterial:!!t.replicationId,replicationId:t.replicationId});};
  C.taskDetail=id=>{
    const t=C.get('tasks',id);if(!C.guard(t))return;let tab='basic',d;
    d=C.open({title:'合成任务详情',kind:'ex-drawer complex',body:()=>{
      const material=C.get('materials',t.materialId),strategy=C.get('strategies',t.strategyId),output=C.db.outputs.find(o=>o.taskId===id),template=t.snapshot?.template;
      const notice=t.cancelRequested?'<p class="ops-note">取消处理中</p>':t.status==='failed'?'<p class="ops-error">失败原因：'+E(t.error||'未记录')+'</p>':t.status==='pending'&&material?.cloudState==='preparing'?'<p class="ops-note">等待素材准备</p>':'';
      return O.tabs([['basic','任务信息'],['execution','执行记录']],tab,v=>{tab=v;d.paint();})+(tab==='basic'?
        C.section('基本信息',C.facts([['任务 ID',t.id],['任务状态',C.status(t.status)],['任务名称',E(t.batch||'独立任务'),true],['批次 ID',C.realBatchId(t)?C.link(C.realBatchId(t),()=>C.batchDetail(C.batchKey(t))):'—'],['负责人',E(t.owner)],['创建方式',E(t.trigger)],['创建时间',E(t.created)],['结束时间',E(t.finished||'—')]])+notice)+
        C.section('合成配置 '+C.help('合成配置','使用创建任务时保存的参数，后续修改策略不会改变本任务。'),C.facts([['合成模板',template?C.link(template,()=>C.templateDetail(template,t.snapshot?.templateVersion)):'未记录'],['模板版本',E(t.snapshot?.templateVersion||'未记录')],['成片设置',E(t.version?.name||t.name)],['适用平台',E(t.version?.platforms?.join(' / ')||'—')],...(C.prod.schemas[template]||[]).map(([key,label,type])=>[label,E(C.prod.parameterValue(key,t.version?.[key])),type==='url'])]))+
        C.section('关联信息',C.facts([['来源素材',material?C.link(material.title,()=>C.materialDetail(material.id)):'来源不可见',true],['来源策略',strategy?C.link(t.snapshot?.strategyName||strategy.name,()=>C.strategyDetail(strategy.id)):E(t.snapshot?.strategyName||'未记录'),true],['成片',output?C.link(O.outputTitle(output),()=>C.outputDetail(output.id)):'尚未生成',true]])):
        C.prod.executionHistory(t));
    },footer:()=>t.status==='failed'?C.btn('重试',()=>C.retry([id])):['pending','running'].includes(t.status)&&!t.cancelRequested?C.btn('取消',()=>C.cancel([id])):''});return d;
  };
  C.taskTable=(rows,selection)=>'<div class="production-task-table '+(selection?'has-selection':'')+'">'+table([...(selection?[C.all(rows.map(t=>t.id))]:[]),'任务 ID','批次 ID','任务名称 / 成片设置','素材','类目','合成策略','负责人','状态／阶段','创建时间','完成时间','操作'],rows.map(t=>{
    const m=C.get('materials',t.materialId),strategy=C.get('strategies',t.strategyId);
    return `<tr>${selection?`<td>${C.check(t.id)}</td>`:''}<td>${t.id}</td><td>${E(C.realBatchId(t)||'—')}</td><td><strong>${E(t.batch||'独立任务')}</strong><small>${E(t.name)} · ${t.version.platforms.join(' / ')}</small></td><td>${C.link(m?.title||'来源不可见',()=>m&&C.materialDetail(m.id))}</td><td>${E(m?.category||'—')}</td><td>${C.link(strategy?.name||'策略不可见',()=>strategy&&C.strategyDetail(strategy.id))}</td><td>${O.person(t.owner)}</td><td>${C.status(t.status)}${t.cancelRequested?'<small>取消请求处理中</small>':t.status==='failed'?'<small>'+E(t.error||'失败原因未记录')+'</small>':t.status==='pending'&&C.get('materials',t.materialId)?.cloudState==='preparing'?'<small>素材准备中</small>':''}</td><td>${E(t.created)}</td><td>${E(t.finished||'—')}</td><td>${C.row('详情',()=>C.taskDetail(t.id))}${t.status==='failed'?C.row('重试',()=>C.retry([t.id]),true):['pending','running'].includes(t.status)&&!t.cancelRequested?C.row('取消',()=>C.cancel([t.id])):''}</td></tr>`;
  }).join(''))+'</div>';
  C.batchTaskTable=rows=>'<div class="production-batch-task-table">'+table(['任务 / 成片设置','素材','任务状态','创建 / 完成时间','操作'],rows.map(t=>{
    const m=C.get('materials',t.materialId);
    return `<tr><td><strong>${E(t.name)}</strong><small>任务 ID ${t.id} · ${E(t.version.platforms.join(' / '))}</small></td><td>${m?C.link(m.title,()=>C.materialDetail(m.id)):'来源不可见'}</td><td>${C.status(t.status)}${t.cancelRequested?'<small>取消处理中</small>':t.status==='failed'?'<small>'+E(t.error||'失败原因未记录')+'</small>':t.status==='pending'&&m?.cloudState==='preparing'?'<small>素材准备中</small>':''}</td><td><span>${E(t.created||'未记录')}</span><small>完成 ${E(t.finished||'—')}</small></td><td>${C.row('详情',()=>C.taskDetail(t.id))}</td></tr>`;
  }).join(''))+'</div>';
  C.batchDetail=(key,onlyFailed=false)=>{
    const all=()=>C.batchRows(key).filter(C.access);if(!all().length){C.toast('批次不存在或无查看权限');return;}
    let state=onlyFailed?'failed':'',page=1,d;
    d=C.open({title:'任务批次详情',kind:'ex-drawer complex',body:()=>{
      const items=all(),first=items[0],filtered=items.filter(t=>!state||t.status===state),max=Math.max(1,Math.ceil(filtered.length/8));page=Math.min(page,max);
      return C.facts([['批次 ID',E(C.realBatchId(first)||'—')],['任务名称',E(first.batch||'历史独立任务'),true],['负责人',E(first.owner)],['合成策略',E(first.snapshot?.strategyName||C.get('strategies',first.strategyId)?.name||'未记录')],['创建方式',E(first.trigger)],['创建时间',E(first.created)],['素材数',new Set(items.map(t=>t.materialId)).size]])+C.section('全批进度',C.prod.progress(items))+C.section('任务清单',`<section class="ex-panel ex-list-pattern"><div class="production-task-list-tools"><span>共 ${items.length} 条${state?' · 当前 '+filtered.length+' 条':''}</span>${C.field('任务状态','batchStatus',state,{options:[['','全部'],['pending','排队中'],['running','处理中'],['success','已完成'],['failed','失败'],['cancelled','已取消']]})}</div>`+C.batchTaskTable(filtered.slice((page-1)*8,page*8))+C.prod.pager(filtered.length,page,8,p=>{page=p;d.paint();})+'</section>')+`<p class="ops-muted">底部操作作用于全批符合条件的任务，不随清单筛选改变。重试与取消前会再次核对范围。</p><details class="ops-demo"><summary>离线演示控制</summary><p>以下模拟执行结果，不代表云端已完成。</p>${C.btn('模拟完成',()=>C.finish(items.map(t=>t.id)),'',!items.some(t=>['pending','running'].includes(t.status)))}${C.btn('模拟失败',()=>{const t=items.find(t=>['pending','running'].includes(t.status));if(t){t.status='failed';t.error='演示：合成执行失败，请检查后重试';(t.events||=[]).push({text:'合成失败',time:C.fullTime(),attempt:t.attempt||1});C.save();C.render();d.paint();}},'',!items.some(t=>['pending','running'].includes(t.status)))}</details>`;
    },change:(k,v,type)=>{if(type==='change'&&k==='batchStatus'){state=v;page=1;d.paint();}},footer:()=>{
      const items=all();return (items.some(t=>t.status==='failed')?C.btn('重试',()=>C.retry(items.map(t=>t.id))):'')+(items.some(t=>['pending','running'].includes(t.status)&&!t.cancelRequested)?C.btn('取消',()=>C.cancel(items.map(t=>t.id))):'');
    }});return d;
  };
  let grouped=true;
  C.taskPage=()=>{const rows=C.db.tasks.filter(t=>C.match(t,'tasks')).sort((a,b)=>b.id-a.id),keys=[...new Set(rows.map(C.batchKey))],visible=grouped?C.paged(keys):C.paged(rows);return C.head('合成任务','查看合成进度，处理失败任务。',C.btn('新增合成任务',()=>C.create(),'primary'))+C.noticeHTML()+`<section class="ex-panel ex-list-pattern production-panel">${O.tabs([['batch','任务批次'],['all','任务列表']],grouped?'batch':'all',v=>{grouped=v==='batch';C.ui.page=1;C.ui.selected.clear();C.render();})}${C.filterProduction('tasks')}<div class="ops-viewbar"><span class="ops-muted">${grouped?'按提交批次查看整体进度。':'查看单条任务的状态与执行结果。'}</span></div>${grouped?table(['批次 ID','任务名称','素材 / 任务','完成进度','待处理','负责人 / 来源','创建时间','操作'],visible.map(key=>{const a=rows.filter(t=>C.batchKey(t)===key),all=C.batchRows(key).filter(C.access),first=all[0],done=all.filter(t=>t.status==='success').length,failed=all.filter(t=>t.status==='failed').length,active=all.filter(t=>['pending','running'].includes(t.status)).length;return `<tr><td>${E(C.realBatchId(first)||'—')}</td><td><strong>${E(first.batch||(C.get('materials',first.materialId)?.title||'来源不可见'))}</strong><small>${E(C.get('strategies',first.strategyId)?.name||'策略不可见')}</small></td><td>${new Set(all.map(t=>t.materialId)).size} 条素材 / ${all.length} 条任务${a.length!==all.length?`<small>筛选匹配 ${a.length} 条</small>`:''}</td><td>${C.prod.progress(all,true)}<small>${all.filter(t=>['success','failed','cancelled'].includes(t.status)).length} / ${all.length} 已结束 · 成功 ${done}</small></td><td>${failed?C.tag(failed+' 条失败','danger'):C.tag(active?active+' 条进行中':done===all.length?'全部成功':all.every(t=>t.status==='cancelled')?'已取消':done?'部分成功':'已结束',done===all.length?'success':'')}<small>${all.filter(t=>t.status==='cancelled').length} 已取消</small></td><td>${first.owner}<small>${first.trigger}</small></td><td>${first.created}</td><td>${C.row('详情',()=>C.batchDetail(key))}</td></tr>`;}).join('')):C.selection(C.btn('重试',()=>C.retry([...C.ui.selected]))+C.btn('取消',()=>C.cancel([...C.ui.selected])))+C.taskTable(visible,true)}${C.pagination(grouped?keys.length:rows.length)}</section>`;};
})();
