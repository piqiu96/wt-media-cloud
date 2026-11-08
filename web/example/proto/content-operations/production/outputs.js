/* 成片文件与原任务批次关联；下载只演示状态，不传输真实视频。 */
(() => {
  const C=CP,P=PI,O=OPS,E=C.e,{table}=C.prod;
  C.db.outputs.forEach(o=>Object.values(o.fileUsage||{}).forEach(u=>Object.values(u.transfers||{}).forEach(t=>{if(t.phase==='downloading'){t.phase='failed';t.reason='上次下载演示已中断，请重试';}})));
  const device=()=>O.mineDevice||'current';
  const usage=o=>{o.fileUsage||={};return o.fileUsage[O.actor]||={status:'active',files:{},transfers:{}};};
  const video=o=>MineVideo.state(o,usage(o),device());
  const localText=o=>{
    const u=usage(o),f=u.files?.[device()],t=u.transfers?.[device()];
    return f?.exists&&f.verified?'已下载':t?.phase==='downloading'?'下载中':t?.phase==='failed'?'下载失败':f?.exists===false?'文件缺失':o.downloaded?'待校验（历史标记）':'未下载';
  };
  const key=o=>{const t=C.get('tasks',o.taskId);return t?C.batchKey(t):'unlinked-output:'+o.id;};
  const refresh=()=>{C.save();C.render();C.dialogs.forEach(d=>d.paint());};
  C.outputVideo=video;C.outputLocalText=localText;
  C.outputStatus=o=>{
    const v=video(o),reason=(v.reason||'').replaceAll('云端','').replaceAll('本机','当前设备');
    return `<span class="production-video-status">${C.tag(v.label,v.tone)}${reason?C.help('视频状态',reason):''}</span>${v.key==='downloading'&&Number.isFinite(v.progress)?'<small>'+E(v.progress)+'%</small>':''}`;
  };
  C.publish=(ids,platform)=>{
    const items=[...new Set(ids)].map(id=>C.get('outputs',id));
    if(!items.length||items.some(o=>!C.access(o))){C.toast('请先选择有权限的成片');return;}
    const common=O.platforms.filter(p=>items.every(o=>O.availability(o.id,p).ok));
    const launch=p=>{O.sync();O.closeAll();P.beginPublish({assetIds:items.map(o=>o.id),platform:p});};
    if(platform&&common.includes(platform)){launch(platform);return;}
    if(!common.length){C.toast('所选成片没有共同可发布的平台，请按平台拆分；已选内容保留');return;}
    if(common.length===1){launch(common[0]);return;}
    let selected=common[0],d;d=C.open({title:'选择发布平台',body:()=>C.field('发布平台','platform',selected,{options:common})+'<p class="ops-muted">下一步带入所选成片配置发布，此处不会提交发布。</p>',change:(k,v)=>selected=v,footer:()=>C.btn('取消',()=>C.close(d,true))+C.btn('下一步',()=>launch(selected),'primary')});
  };
  const downloadJobs=[];
  C.download=ids=>{
    const existing=downloadJobs.find(j=>j.owner===O.actor&&j.dev===device()&&j.entries.some(x=>x.result==='pending'&&ids.includes(x.id)));
    if(existing){existing.open();C.toast('已打开进行中的下载');return existing.dialog;}

    const owner=O.actor,dev=device(),entries=[...new Set(ids)].map(id=>{
      const o=C.get('outputs',id);if(!C.access(o))return {id,result:'skipped',reason:'不存在或无权限'};
      const u=usage(o),v=video(o);if(v.local)return {id,result:'skipped',reason:'当前设备已有文件'};
      if(u.transfers?.[dev]?.phase==='downloading')return {id,result:'skipped',reason:'已有下载进行中'};
      if(!v.remote)return {id,result:'failed',reason:'视频可用性尚未确认，请先查看详情',retryable:false};
      u.transfers||={};u.transfers[dev]={phase:'downloading',progress:0};return {id,result:'pending',reason:'下载中',retryable:true};
    });
    let d;const pending=()=>entries.some(x=>x.result==='pending');
    const advance=fail=>{entries.filter(x=>x.result==='pending').forEach(x=>{
      const o=C.get('outputs',x.id),u=o.fileUsage[owner];u.transfers[dev]={phase:fail?'failed':'completed',progress:fail?0:100,reason:fail?'演示：下载连接中断':''};x.result=fail?'failed':'success';x.reason=fail?'演示下载失败':'演示下载完成';
      if(!fail){u.files||={};u.files[dev]={exists:true,verified:true,demo:true,path:'示例下载/成片-'+o.id+'.mp4'};}
    });refresh();};
    const job={owner,dev,entries,dialog:null};downloadJobs.push(job);
    job.open=()=>{if(job.dialog?.isConnected)return job.dialog;d=C.open({title:'下载进度与结果',kind:'mine-download-dialog',body:()=>`<p>${E(owner)} · ${dev==='current'?'当前设备':'另一台设备'} · 仅状态演示，不传输真实视频</p>`+table(['成片','结果','说明'],entries.map(x=>`<tr><td>${x.id}</td><td>${({pending:'下载中',success:'已下载',failed:'失败',skipped:'跳过'})[x.result]}</td><td>${E(x.reason)}</td></tr>`).join(''))+(pending()?`<details class="ops-demo" open><summary>离线下载演示</summary>${C.btn('模拟完成',()=>advance(false))}${C.btn('模拟失败',()=>advance(true))}</details>`:''),footer:()=>!pending()&&entries.some(x=>x.result==='failed'&&x.retryable)?C.btn('重试',()=>{C.close(d,true);C.download(entries.filter(x=>x.result==='failed'&&x.retryable).map(x=>x.id));},'primary'):''});job.dialog=d;return d;};job.open();refresh();return d;
  };
  const outputFilename=o=>o.metadata?.filename||o.metadata?.fileName||o.filename||O.outputFilename(o);
  C.openOutputFile=id=>{
    const o=C.get('outputs',id);if(!C.guard(o))return;
    const r=MineVideo.resources(o,usage(o),device());
    if(!r.remote&&!r.local){C.toast('视频文件不可访问，请核对视频状态');return;}
    C.open({title:r.cloudDemo||r.file?.demo?'视频文件 · 原型演示':'视频文件',body:()=>C.facts([['视频文件',E(outputFilename(o)),true],...(r.remote&&!r.cloudDemo?[['打开视频',O.reference(outputFilename(o),r.cloudUrl),true]]:[])])+(r.cloudDemo||r.file?.demo?'<p class="ops-note">当前为示例文件，未接入实际视频。</p>':'<p class="ops-muted">当前原型未接入本机文件打开能力。</p>')});
  };
  C.outputDetail=id=>{
    const o=C.get('outputs',id);if(!C.guard(o))return;const m=C.get('materials',o.materialId),t=C.get('tasks',o.taskId);let d;
    d=C.open({title:'成片详情',kind:'ex-drawer complex',body:()=>{
      const meta=o.metadata||{},r=MineVideo.resources(o,usage(o),device()),filename=outputFilename(o);
      const file=r.remote&&!r.cloudDemo?O.reference(filename,r.cloudUrl):r.remote||r.local?C.link(filename,()=>C.openOutputFile(id)):E(filename);
      return C.section('基本信息',C.facts([['成片 ID',id],['视频状态',C.outputStatus(o)],['成片名称',E(O.outputTitle(o)),true],['负责人',E(o.owner)],['生成时间',E(o.date||'未记录')]]))+
        C.section('视频信息',C.facts([['视频文件',file,true]])+`<dl class="production-video-metadata"><div><dt>时长</dt><dd>${E(meta.duration||'—')}</dd></div><div><dt>分辨率</dt><dd>${E(meta.resolution||'—')}</dd></div><div><dt>文件大小</dt><dd>${E(meta.size||'—')}</dd></div></dl>`)+
        C.section('制作来源',C.facts([['来源素材',m?C.link(m.title,()=>C.materialDetail(m.id)):'来源不可见',true],['来源策略',E(t?.snapshot?.strategyName||'未记录'),true],['成片设置',E(t?.version?.name||o.name||'未记录')],['合成任务',t?C.link(t.id,()=>C.taskDetail(t.id)):'未记录'],['任务批次',C.realBatchId(t)?C.link(C.realBatchId(t),()=>C.batchDetail(C.batchKey(t))):'—']]))+
        C.section('发布情况 '+C.help('发布情况','未安排：尚无有效发布任务；已安排：已有任务，进度和异常见关联任务；已发布：已确认发布成功。视频状态单独判断。'),table(['适用平台','发布情况','关联任务'],o.platforms.map(p=>{
          const a=O.publication(id,p);
          return `<tr><td>${E(p)}</td><td>${C.tag(a.text,a.key==='published'?'success':a.key==='reserved'?'info':'')}${a.tasks?.some(t=>t.status==='failed')?'<small class="production-unavailable">发布失败</small>':''}</td><td>${a.tasks?.length?a.tasks.map(t=>C.link(t.id,()=>P.taskDetail(t.id))).join(' / '):'—'}</td></tr>`;
        }).join('')))+
        `<details class="ops-demo"><summary>文件状态演示</summary><p>仅模拟视频状态，不产生实际文件。</p>${C.btn('模拟可用',()=>{Object.assign(o,{cloudFileExists:true,cloudUrlVerified:true,cloudUrl:'https://media.example.test/prototype-only.mp4',cloudDemo:true});refresh();})}${C.btn('模拟文件丢失',()=>{usage(o).files[device()]={exists:false,verified:true};refresh();})}${C.btn('模拟文件失效',()=>{Object.assign(o,{cloudFileExists:false,cloudUrlVerified:false,cloudState:'expired'});refresh();})}</details>`;
    },footer:()=>video(o).remote&&!video(o).local?C.btn('下载',()=>C.download([id])):''});return d;
  };
  C.outputTable=(rows,selection=false,chosen=null)=>'<div class="production-output-table">'+table([...(selection?[chosen?'选择':C.all(rows.map(o=>o.id))]:[]),'成片 ID','成片名称 / 生成时间','来源素材','类目','负责人','适用平台','视频状态','操作'],rows.map(o=>{
    const m=C.get('materials',o.materialId),v=video(o),t=C.get('tasks',o.taskId);return `<tr>${selection?`<td>${chosen?`<input type="checkbox" data-output-pick="${o.id}" aria-label="选择成片 ${o.id}" ${chosen.has(o.id)?'checked':''}>`:C.check(o.id)}</td>`:''}<td>${o.id}</td><td><strong>${E(O.outputTitle(o))}</strong><small>${E(t?.snapshot?.strategyName||'来源策略未记录')}</small><small class="production-output-date">${E(o.date||'生成时间未记录')}</small></td><td>${m?C.link(m.title,()=>C.materialDetail(m.id)):'来源不可见'}</td><td>${E(m?.category||'—')}</td><td>${E(o.owner||'未记录')}</td><td>${E(o.platforms.join(' / '))}</td><td>${C.outputStatus(o)}</td><td>${C.row('详情',()=>C.outputDetail(o.id))}${v.remote&&!v.local?C.btn(v.key==='failed'?'重试下载':'下载',()=>C.download([o.id]),'text',v.key==='downloading'):''}${canRecreate(o)?C.row('再次制作',()=>C.createFrom(o.taskId)):''}</td></tr>`;
  }).join(''))+'</div>';
  C.outputBatch=(batchKey,matchedIds)=>{
    const rows=()=>C.db.outputs.filter(o=>key(o)===batchKey&&C.access(o)&&(!matchedIds||matchedIds.includes(o.id)));if(!rows().length){C.toast('尚无可查看成片或无权限');return;}
    let chosen=new Set(),page=1,d;
    d=C.open({title:'本次任务产出',kind:'ex-drawer complex',body:()=>`<p>本批已实际生成 ${rows().length} 个成片；尚未生成的任务不展示占位文件。</p>`+C.outputTable(rows().slice((page-1)*8,page*8),true,chosen)+`<div class="ex-pagination">${C.btn('上一页',()=>{page--;d.paint();},'',page===1)} ${page}/${Math.max(1,Math.ceil(rows().length/8))} ${C.btn('下一页',()=>{page++;d.paint();},'',page*8>=rows().length)}</div>`,wire:el=>el.querySelectorAll('[data-output-pick]').forEach(x=>x.addEventListener('change',()=>{x.checked?chosen.add(+x.dataset.outputPick):chosen.delete(+x.dataset.outputPick);d.paint();})),footer:()=>chosen.size?`<span>已选 ${chosen.size} 个</span>`+C.btn('下载',()=>C.download([...chosen]))+C.btn('去发布',()=>C.publish([...chosen])):''});return d;
  };
  const canRecreate=o=>{const t=C.get('tasks',o.taskId),s=t&&C.get('strategies',t.strategyId),m=C.get('materials',o.materialId);return !!(t&&s?.enabled&&C.access(s)&&m&&ProductionModel.canPrepare(m));};
  C.materialOutputTable=(rows,chosen)=>'<div class="production-material-file-table">'+table([`<input type="checkbox" data-output-page aria-label="选择本页成片" ${rows.length&&rows.every(o=>chosen.has(o.id))?'checked':''}>`,'成片 / 生成时间','来源策略 / 任务批次','成片设置','适用平台','视频状态','操作'],rows.map(o=>{
    const t=C.get('tasks',o.taskId),v=video(o),strategy=t?.snapshot?.strategyName||C.get('strategies',t?.strategyId)?.name;
    return `<tr><td><input type="checkbox" data-output-pick="${o.id}" aria-label="选择成片 ${o.id}" ${chosen.has(o.id)?'checked':''}></td><td><strong>${E(O.outputTitle(o))}</strong><small>${O.library.idCell(o.id,'成片 ID')}</small><small>${E(o.date||'生成时间未记录')}</small></td><td>${E(strategy||'未记录')}<small>${t?C.link(C.realBatchId(t)||'任务 '+t.id,()=>C.batchDetail(C.batchKey(t))):'任务关联未记录'}</small></td><td>${E(t?.version?.name||o.name||'未记录')}</td><td>${E(o.platforms.join(' / '))}</td><td>${C.outputStatus(o)}</td><td>${C.row('详情',()=>C.outputDetail(o.id))}${v.remote&&!v.local?C.btn('下载',()=>C.download([o.id]),'text',v.key==='downloading'):''}${canRecreate(o)?C.row('再次制作',()=>C.createFrom(o.taskId)):''}</td></tr>`;
  }).join(''))+'</div>';
  C.outputMaterial=(materialId,matchedIds)=>{
    const all=()=>C.db.outputs.filter(o=>o.materialId===materialId&&C.access(o)).sort((a,b)=>String(b.date||'').localeCompare(String(a.date||''))||b.id-a.id),rows=()=>all().filter(o=>!matchedIds||matchedIds.includes(o.id));
    if(!rows().length){C.toast('没有可查看的成片');return;}
    let chosen=new Set(),page=1,d;
    d=C.open({title:'素材成片详情',kind:'ex-drawer complex production-material-outputs',body:()=>{
      const m=C.get('materials',materialId),items=rows(),strategies=new Set(items.map(o=>C.get('tasks',o.taskId)?.strategyId).filter(Boolean)),batches=new Set(items.map(o=>C.get('tasks',o.taskId)).filter(Boolean).map(C.batchKey));
      return `<div class="production-material-heading">${m?O.library.cover(m):''}<div><h3>${E(m?.title||'来源素材不可见')}</h3><p>素材 ID ${E(materialId)} · ${E(m?.category||'类目未记录')}</p></div></div><div class="production-output-summary"><span><strong>${items.length}</strong> 个成片</span><span><strong>${strategies.size}</strong> 个策略</span><span><strong>${batches.size}</strong> 个任务批次</span></div>${matchedIds&&items.length<all().length?`<div class="production-inline-note">当前筛选匹配 ${items.length} / ${all().length} 个成片。${C.btn('查看全部',()=>{matchedIds=null;chosen.clear();page=1;d.paint();},'text')}</div>`:''}<section class="ex-panel ex-list-pattern">`+C.materialOutputTable(items.slice((page-1)*8,page*8),chosen)+C.prod.pager(items.length,page,8,p=>{page=p;d.paint();})+`</section>`;
    },wire:el=>{el.querySelectorAll('[data-output-pick]').forEach(x=>x.addEventListener('change',()=>{x.checked?chosen.add(+x.dataset.outputPick):chosen.delete(+x.dataset.outputPick);d.paint();}));const current=rows().slice((page-1)*8,page*8),all=el.querySelector('[data-output-page]');if(all){all.indeterminate=current.some(o=>chosen.has(o.id))&&!current.every(o=>chosen.has(o.id));all.addEventListener('change',()=>{current.forEach(o=>all.checked?chosen.add(o.id):chosen.delete(o.id));d.paint();});}},footer:()=>chosen.size?`<span class="production-footer-note">已选 ${chosen.size} 个成片</span>`+C.btn('下载',()=>C.download([...chosen]))+C.btn('去发布',()=>C.publish([...chosen]),'primary'):''});return d;
  };
  let grouped=true;
  C.outputPage=()=>{
    const rows=C.db.outputs.filter(o=>C.match(o,'outputs')).sort((a,b)=>b.id-a.id),groups=ProductionModel.groupOutputs(rows),visible=C.paged(grouped?groups:rows);
    return C.head('成片管理','按素材查看产出，或选择成片下载、发布。')+`<section class="ex-panel ex-list-pattern production-panel">${O.tabs([['batch','成片批次'],['files','成片列表']],grouped?'batch':'files',v=>{grouped=v==='batch';C.ui.selected.clear();C.ui.page=1;C.render();})}${downloadJobs.some(j=>j.owner===O.actor&&j.dev===device()&&j.entries.some(x=>x.result==='pending'))?'<div class="ops-viewbar">'+C.btn('下载进度',()=>downloadJobs.find(j=>j.owner===O.actor&&j.dev===device()&&j.entries.some(x=>x.result==='pending'))?.open())+'</div>':''}${C.filterProduction('outputs')}${grouped?`<div class="production-list-caption">按素材 ID 汇总，查看该素材在不同策略、任务批次下的产出。</div>`+table(['素材 ID','素材 / 作者','类目','使用策略','实际成片','最近生成','操作'],visible.map(group=>{
      const m=C.get('materials',group.materialId),items=group.outputs,strategies=[...new Set(items.map(o=>{const t=C.get('tasks',o.taskId);return t?.snapshot?.strategyName||C.get('strategies',t?.strategyId)?.name;}).filter(Boolean))];
      return `<tr><td>${O.library.idCell(group.materialId,'素材 ID')}</td><td>${m?`<div class="lib-material">${O.library.cover(m)}<div><strong>${E(m.title)}</strong><small>${E(m.author||'—')}</small></div></div>`:'来源素材不可见'}</td><td>${E(m?.category||'—')}</td><td>${strategies.length?strategies.length+' 个策略 '+C.help('来源策略',strategies.join('、')):'未记录'}</td><td>${items.length} 个</td><td>${E(items[0].date||'未记录')}</td><td>${C.row('详情',()=>C.outputMaterial(group.materialId,items.map(o=>o.id)))}</td></tr>`;
    }).join('')):C.selection(C.btn('下载',()=>C.download([...C.ui.selected]))+C.btn('去发布',()=>C.publish([...C.ui.selected])))+C.outputTable(visible,true)}${C.pagination(grouped?groups.length:rows.length)}</section><details class="ops-demo"><summary>设备状态演示</summary><p>当前：${device()==='current'?'当前设备':'另一台设备'}。历史下载标记不代表当前设备存在文件。</p>${C.btn('切换演示设备',()=>{O.mineDevice=device()==='current'?'other':'current';C.ui.selected.clear();C.render();})}</details>`;
  };
})();
