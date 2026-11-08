/* 我的素材局部交互。下载过程由显式演示控件推进，不传输真实文件。 */
(() => {
  const C=CP,O=OPS,L=O.library,E=C.e,M=MineVideo,{table}=C.prod;
  const oldPage=C.materialPage,oldDetail=C.materialDetail;
  const defaults=()=>({q:'',platforms:[],categories:[],usage:'unused',team:'',video:'',from:'',to:'',enteredFrom:'',enteredTo:'',addedFrom:'',addedTo:'',creator:'',sort:'published',desc:true,page:1,size:8,chosen:[],scrollLeft:0});
  const state=()=>{const store=C.ui.filters.mine||={};store.byActor||={};return store.byActor[O.actor]||=defaults();};
  const device=()=>O.mineDevice||'current';
  const video=m=>M.state(m,C.usage(m.id),device());
  const date=v=>{if(!v)return '';const s=String(v);return /^\d{2}-\d{2} /.test(s)?'2026-'+s:s;};
  const val=(m,key)=>key==='added'?date(C.usage(m.id)?.added):key==='outputs'?L.stats(m.id).outputs.length:m[key];
  const rows=()=>{const f=state();return C.db.materials.filter(m=>{
    const u=C.usage(m.id);if(!u||u.status!=='active')return false;
    if(!L.matches(m,{...f,joined:''})||f.video&&video(m).key!==f.video)return false;
    const added=date(u.added).slice(0,10);return (!f.addedFrom||added>=f.addedFrom)&&(!f.addedTo||added&&added<=f.addedTo);
  }).sort((a,b)=>{const x=val(a,f.sort),y=val(b,f.sort),missing=v=>v==null||v==='';if(missing(x)||missing(y))return missing(x)===missing(y)?a.id-b.id:missing(x)?1:-1;return (typeof x==='number'&&typeof y==='number'?x-y:String(x).localeCompare(String(y),'zh-CN'))*(f.desc?-1:1)||b.id-a.id;});};
  const refresh=()=>{C.render();C.dialogs.filter(d=>d.mineDetail).forEach(d=>d.paint());};
  const sort=(label,key)=>{const f=state(),active=f.sort===key,next=active&&f.desc?'升序':'降序';return `<span data-mine-sort="${key}">${C.btn(label+L.sortIcon(active),()=>{f.desc=active?!f.desc:true;f.sort=key;f.page=1;C.render();},'text ex-sort-button'+(active?' is-active':'')+(active&&!f.desc?' is-ascending':''),false,`aria-label="${label}${active?'，当前'+(f.desc?'降序':'升序'):''}，点击按${next}" title="点击按${label}${next}"`)}</span>`;};
  const multi=(label,key,values,chosen)=>L.multi(label,key,values,chosen).replaceAll('data-lib-multi','data-mine-multi');
  const range=(label,key,start,end)=>`<div class="ex-field lib-range ex-filter-range"><label id="mine-range-${key}">${label}</label><div data-library-range="mine-${key}" aria-labelledby="mine-range-${key}"><input type="hidden" data-field="${start}" value="${E(state()[start])}"><input type="hidden" data-field="${end}" value="${E(state()[end])}"><div data-library-range-host></div></div></div>`;
  const status=m=>{const v=video(m);return `<div class="mine-video">${C.tag(v.label,v.tone)}${v.key==='downloading'&&Number.isFinite(v.progress)?`<progress max="100" value="${v.progress}" aria-label="下载进度 ${v.progress}%"></progress><small>${v.progress}%</small>`:''}${v.reason?`<small>${E(v.reason)}</small>`:''}${v.key==='failed'&&v.canCompose?'<small>可用于合成</small>':''}</div>`;};
  const summary=m=>{const s=L.stats(m.id);return s.outputs.length?C.link(`${s.outputs.length} 个成片 · ${s.people} 人使用`,()=>L.usageDetail(m.id)):'—';};
  const transfer=(u,d)=>{u.transfers||={};return u.transfers[d]||={};};
  const batches=[];
  // Reloading the offline prototype interrupts its manual simulator, never a real transfer.
  let interrupted=false;
  Object.values(C.db.usages||{}).flat().forEach(u=>Object.values(u.transfers||{}).forEach(t=>{if(['preparing','downloading'].includes(t.phase)){Object.assign(t,{phase:'failed',reason:'上次下载演示已中断，请重新下载'});interrupted=true;}}));
  if(interrupted)C.save();
  const preparedDemo=u=>{u.preparedDemo={cloudFileExists:true,cloudUrl:'https://media.example.test/prototype-only.mp4',cloudDemo:true,cloudUrlVerified:true,cloudUrlExpiresAt:null,cloudState:'ready'};};
  const download=ids=>{
    const owner=O.actor,dev=device(),entries=[...new Set(ids)].map(id=>{
      const m=C.get('materials',id),u=C.usage(id,owner);if(!m||u?.status!=='active')return {id,result:'failed',reason:'素材已移出或不可访问',retryable:false};
      const v=M.state(m,u,dev);if(v.local)return {id,result:'skipped',reason:'当前电脑已有可用文件'};
      if(['preparing','downloading'].includes(v.key))return {id,result:'skipped',reason:v.key==='preparing'?'视频准备中，请等待完成':'已有下载正在进行'};
      if(v.key==='unknown'||v.key==='failed'&&!v.download)return {id,result:'failed',reason:v.reason,retryable:false};
      Object.assign(transfer(u,dev),{phase:v.remote?'downloading':'preparing',progress:v.remote?0:null,reason:''});
      return {id,result:'running',reason:v.remote?'下载中':'准备中'};
    });
    const batch={owner,dev,entries};batches.push(batch);C.save();refresh();openBatch(batch);return batch;
  };
  const advance=(batch,mode)=>{
    for(const [i,entry] of batch.entries.entries())if(entry.result==='running'){
      const u=C.usage(entry.id,batch.owner);if(!u){Object.assign(entry,{result:'failed',reason:'个人关系不存在',retryable:false});continue;}
      const t=transfer(u,batch.dev);
      if(mode==='progress'){preparedDemo(u);if(t.phase==='preparing')t.phase='downloading';t.progress=50;entry.reason='下载中 50%';continue;}
      if(mode==='failure'||mode==='partial'&&i%2===0){Object.assign(t,{phase:'failed',reason:t.phase==='preparing'?'视频获取失败，请重试':'下载失败：本机空间不足'});Object.assign(entry,{result:'failed',reason:t.reason,retryable:true});}
      else{preparedDemo(u);Object.assign(t,{phase:'completed',progress:100,reason:''});u.files||={};u.files[batch.dev]={exists:true,verified:true,demo:true,path:`示例下载/内容运营/素材-${entry.id}.mp4`};Object.assign(entry,{result:'success',reason:'演示下载完成'});}
    }
    C.save();refresh();batch.dialog?.paint();
  };
  const openBatch=batch=>{
    let d;const pending=()=>batch.entries.some(x=>x.result==='running');
    d=C.open({title:'下载进度与结果',kind:'mine-download-dialog',body:()=>{
      const count=k=>batch.entries.filter(x=>x.result===k).length;
      return `<p class="ops-note">本地原型演示 · ${E(batch.owner)} · ${batch.dev==='current'?'当前电脑':'另一台电脑'}。不会传输真实视频，请使用下方演示控制推进过程。</p><div class="mine-batch-summary" role="status"><strong>${pending()?'正在处理':count('failed')?'处理结束，有失败项':'处理结束'}</strong><div>总数 ${batch.entries.length} · 进行中 ${count('running')} · 成功 ${count('success')} · 失败 ${count('failed')} · 跳过 ${count('skipped')}</div></div><div class="mine-batch-rows">${table(['素材','结果','说明'],batch.entries.map(x=>`<tr><td>${E(C.get('materials',x.id)?.title||x.id)}</td><td>${({running:'进行中',success:'成功',failed:'失败',skipped:'跳过'})[x.result]}</td><td>${E(x.reason)}</td></tr>`).join(''))}</div>${pending()?`<details class="ops-demo" open><summary>演示控制</summary><div class="cp-actions">${C.btn('推进到下载中',()=>advance(batch,'progress'))}${C.btn('模拟全部成功',()=>advance(batch,'success'))}${C.btn('模拟部分失败',()=>advance(batch,'partial'))}${C.btn('模拟全部失败',()=>advance(batch,'failure'))}</div></details>`:''}`;
    },footer:()=>C.btn('关闭',()=>C.close(d),'',pending())+(!pending()&&batch.entries.some(x=>x.result==='failed'&&x.retryable)?C.btn('仅重试失败项',()=>{const ids=batch.entries.filter(x=>x.result==='failed'&&x.retryable).map(x=>x.id);C.close(d,true);download(ids);},'primary'):''),wire:el=>{el.querySelector('.ex-close').disabled=pending();}});
    batch.dialog=d;d.minePending=pending;
  };
  const close=C.close;C.close=(d=C.dialogs.at(-1),force=false)=>{if(d?.minePending?.()&&!force){C.toast('下载演示处理中，请先通过演示控制完成本次过程');return;}return close(d,force);};
  const relation=(id,restore)=>{
    const u=C.usage(id);if(!u)return;
    const apply=()=>{u.status=restore?'active':'removed';if(restore)u.added=new Date().toLocaleString('sv-SE',{timeZone:'Asia/Shanghai'}).slice(0,16);C.save();state().chosen=state().chosen.filter(x=>x!==id);refresh();C.toast(restore?'已恢复到我的素材':'已移出我的素材');};
    if(restore){apply();return;}
    let d;d=C.open({title:'移出我的素材？',body:()=>`<p>${E(C.get('materials',id)?.title)}</p><p>移出后可恢复。已有合成任务、成片、发布记录及已下载文件保留，已受理的下载继续处理。</p>`,footer:()=>C.btn('取消',()=>C.close(d))+C.btn('移出',()=>{C.close(d,true);apply();},'danger')});
  };
  const actions=m=>{const u=C.usage(m.id),v=video(m);return (u.status==='active'&&v.download?C.row('下载',()=>download([m.id]),true):'')+C.row('详情',()=>C.materialDetail(m.id))+(u.status==='active'?C.btn('移出',()=>relation(m.id,false),'text danger'):C.row('恢复',()=>relation(m.id,true)));};
  const select=(ids,on)=>{const f=state(),set=new Set(f.chosen);ids.forEach(id=>on?set.add(id):set.delete(id));f.chosen=[...set];C.render();};
  const reset=()=>{C.ui.filters.mine.byActor[O.actor]=defaults();C.render();};
  const filters=f=>{
    return `<form class="ex-filters ex-filters-stacked" data-mine-filter>
      ${C.field('综合搜索','q',f.q,{placeholder:'素材 ID、平台作品 ID、标题、作者',className:'ex-filter-search'})}
      ${multi('来源平台','platforms',O.sourcePlatforms,f.platforms)}${multi('类目','categories',O.categories,f.categories)}
      ${C.select('video','视频状态',f.video,Object.entries(M.labels).filter(([k])=>k!=='unknown'))}
      ${C.select('usage','我的使用',f.usage,[['unused','未使用'],['made','已合成']])}
      ${C.select('team','团队使用',f.team,[['unused','无人使用'],['used','有人使用']])}
      <div class="ex-actions"><button class="ex-button primary" type="submit">查询</button>${C.btn('重置',reset)}</div>
      ${L.advancedFilters('mine',f,range('加入时间','added','addedFrom','addedTo')+range('发布时间','published','from','to')+range('入库时间','entered','enteredFrom','enteredTo')+(C.canManage()?C.select('creator','操作人',f.creator,O.peopleOptions()):''))}
      <p data-mine-error class="ops-error" role="alert" hidden></p></form>`;
  };
  C.materialPage=mine=>{
    if(!mine)return oldPage(mine);
    const f=state(),all=rows(),max=Math.max(1,Math.ceil(all.length/f.size));f.page=Math.min(f.page,max);const visible=all.slice((f.page-1)*f.size,f.page*f.size),chosen=new Set(f.chosen);
    const heads=[`<input type="checkbox" class="ex-check" data-mine-page aria-label="选择本页素材" ${visible.length&&visible.every(m=>chosen.has(m.id))?'checked':''} ${!visible.length?'disabled':''}>`,'素材 ID','平台','平台作品 ID','素材信息','类目',sort('发布时间','published'),`内容表现<div class="lib-metric-sort">${sort('浏览','views')}${sort('点赞','likes')}${sort('收藏','favorites')}</div>`,'视频状态','我的使用',sort('团队使用','outputs'),sort('加入时间','added'),sort('入库时间','created'),'操作人','操作'];
    const body=visible.map(m=>{const u=C.usage(m.id),s=L.stats(m.id);return `<tr><td><input type="checkbox" class="ex-check" data-mine-row="${m.id}" aria-label="选择素材 ${m.id}" ${chosen.has(m.id)?'checked':''}></td><td>${L.idCell(m.id,'素材 ID')}</td><td>${E(m.platform||'—')}</td><td>${L.idCell(m.platformWorkId??m.platform_work_id,'平台作品 ID')}</td><td><div class="lib-material">${L.cover(m)}<div>${C.link(m.title,()=>C.materialDetail(m.id))}<small>${E(m.author||'—')}</small>${u.status==='removed'?C.tag('已移出'):''}</div></div></td><td>${E(m.category||'—')}</td><td>${E(m.published||'—')}</td><td>${L.performance(m)}</td><td>${status(m)}</td><td>${C.tag(s.mine.length?'已合成':'未使用',s.mine.length?'success':'')}</td><td>${summary(m)}</td><td>${E(date(u.added)||'—')}</td><td>${E(m.created||'—')}</td><td>${E(L.creator(m))}</td><td>${actions(m)}</td></tr>`;}).join('');
    return C.head('我的素材','管理已关注的素材，按需下载，在合成任务中选用。')+`<section class="ex-panel lib-panel mine-panel ex-list-pattern">${filters(f)}${chosen.size?`<div class="ex-selection"><span>已选择 <strong>${chosen.size}</strong> 条（跨页保留）</span>${C.btn('下载',()=>download(f.chosen),'primary',!f.chosen.some(id=>{const m=C.get('materials',id);return m&&C.usage(id)&&video(m).download;}),'title="仅处理可下载素材，准备中、下载中及已有本机文件的素材不重复处理"')}${C.btn('选择全部 '+all.length+' 条匹配结果',()=>select(all.map(m=>m.id),true),'text')}${C.btn('取消选择',()=>{f.chosen=[];C.render();},'text')}</div>`:''}<div class="lib-table mine-table">${table(heads,body||`<tr><td colspan="15"><div class="ex-empty"><h3>${C.usages().length?'没有符合条件的素材':'还没有关注的素材'}</h3><p>${C.usages().length?'调整筛选条件后重试。':'可从素材库加入感兴趣的素材。'}</p>${C.usages().length?C.btn('清除筛选条件',reset):'<a class="ex-button" href="#materials">前往素材库</a>'}</div></td></tr>`).replace('<div class="ex-table-scroll">','<div class="ex-table-scroll" tabindex="0" role="region" aria-label="我的素材列表，可横向滚动">')}</div>${L.pagination(f,all.length,max,'data-mine-size','我的素材')}<details class="ops-status-guide"><summary>状态说明</summary><p>视频统一展示待准备、准备中、可用、下载中、失败。已下载且当前电脑文件可用时仍显示可用，并隐藏下载。我的使用与团队使用只统计成功成片；下载不是合成前置条件。</p><p>本页下载和文件核验为原型演示。历史下载记录未经当前设备核验，不直接显示可用。</p>${C.btn('切换演示设备',()=>{O.mineDevice=device()==='current'?'other':'current';refresh();C.toast('演示设备：'+(device()==='current'?'当前电脑':'另一台电脑'));},'text')}<span>演示设备：${device()==='current'?'当前电脑':'另一台电脑'}</span></details></section>`;
  };
  const setDemo=(id,key)=>{
    const u=C.usage(id),dev=device();delete u.preparedDemo;u.files||={};u.files[dev]={exists:false,verified:true,demo:true};u.videoDemo||={};u.videoDemo[dev]=key==='available'?{cloudFileExists:true,cloudUrl:'https://media.example.test/prototype-only.mp4',cloudDemo:true,cloudUrlVerified:true,cloudUrlExpiresAt:null,cloudState:'ready'}:{cloudFileExists:false,cloudUrlVerified:false,cloudState:key==='pending'?'notSaved':'failed',sourceState:'available'};
    if(key==='missing'){u.files||={};u.files[dev]={exists:false,verified:true};u.videoDemo[dev]={cloudFileExists:true,cloudUrl:'https://media.example.test/prototype-only.mp4',cloudDemo:true,cloudUrlVerified:true,cloudUrlExpiresAt:null,cloudState:'ready'};}
    if(key==='downloaded'){preparedDemo(u);u.files[dev]={exists:true,verified:true,demo:true,path:`示例下载/内容运营/素材-${id}.mp4`};}
    delete u.transfers?.[dev];C.save();refresh();
  };
  C.materialDetail=id=>{
    if(C.route!=='mine')return oldDetail(id);
    const m=C.get('materials',id),u=C.usage(id);if(!m||!u){C.toast('素材不存在或无权访问');return;}
    const d=C.open({title:'我的素材详情',kind:'ex-drawer complex lib-detail mine-detail ex-list-pattern',body:()=>{
      const s=L.stats(id);return `<div class="lib-detail-heading">${L.cover(m,true)}<div><h3>${E(m.title)}</h3><p>${E(m.author||'—')}</p></div></div>`+C.section('基本信息',C.facts([['素材 ID',L.idCell(id,'素材 ID')],['平台',E(m.platform||'—')],['平台作品 ID',L.idCell(m.platformWorkId??m.platform_work_id,'平台作品 ID')],['平台作品链接',L.sourceLink(m)],['类目',E(m.category||'—')],['发布时间',E(m.published||'—')],['加入时间',E(date(u.added)||'—')],['入库时间',E(m.created||'—')],['操作人',E(L.creator(m))]]))+C.section('内容表现',L.performance(m,true))+C.section('使用情况',C.facts([['我的使用',s.mine.length?'已合成':'未使用'],['团队使用',summary(m)]]))+C.section('视频文件',C.facts(L.fileAccessRows(id,status(m)))+`<details class="ops-demo"><summary>状态演示（不传输真实文件）</summary><p>仅改变当前用户、当前演示设备的体验样例，不改写公共素材或真实视频地址。</p><div class="cp-actions">${C.btn('模拟待准备',()=>setDemo(id,'pending'))}${C.btn('模拟视频可用',()=>setDemo(id,'available'))}${C.btn('模拟已下载',()=>setDemo(id,'downloaded'))}${C.btn('模拟准备失败',()=>setDemo(id,'failed'))}${C.btn('模拟本机文件丢失',()=>setDemo(id,'missing'))}</div></details>`);
    }});d.mineDetail=true;return d;
  };
  document.addEventListener('submit',e=>{
    if(!e.target.matches('[data-mine-filter]'))return;e.preventDefault();const form=e.target,next={},f=state();
    const incomplete=[...form.querySelectorAll('[data-library-range]')].some(el=>{const inputs=[...el.querySelectorAll('[data-library-range-host] input')];return inputs.length===2&&Boolean(inputs[0].value)!==Boolean(inputs[1].value);});
    form.querySelectorAll('[data-field]').forEach(el=>next[el.dataset.field]=el.value.trim());
    if(incomplete||[['from','to'],['enteredFrom','enteredTo'],['addedFrom','addedTo']].some(([a,b])=>next[a]&&next[b]&&next[a]>next[b])){const err=form.querySelector('[data-mine-error]');err.hidden=false;err.textContent='请选择完整且有效的日期范围';return;}
    for(const key of ['platforms','categories'])next[key]=[...form.querySelectorAll(`[data-mine-multi="${key}"]:checked`)].map(el=>el.value);
    Object.assign(f,next,{page:1,chosen:[]});C.render();
  });
  document.addEventListener('change',e=>{
    if(C.route!=='mine')return;const el=e.target;
    if(el.matches('[data-mine-row]'))select([+el.dataset.mineRow],el.checked);
    if(el.matches('[data-mine-page]'))select(rows().slice((state().page-1)*state().size,state().page*state().size).map(m=>m.id),el.checked);
    if(el.matches('[data-mine-size]')){state().size=+el.value;state().page=1;C.render();}
    if(el.matches('[data-mine-multi]')){const chosen=[...el.closest('details').querySelectorAll('input:checked')].map(x=>x.value),sum=el.closest('details').querySelector('summary');sum.innerHTML=L.multiSummary(chosen);sum.title=chosen.join('、')||'全部';el.closest('label').classList.toggle('is-selected',el.checked);}
  });
  document.addEventListener('toggle',e=>{if(e.target.isConnected&&C.route==='mine'&&e.target.matches?.('[data-resource-advanced="mine"]'))state().advanced=e.target.open;},true);
  document.addEventListener('scroll',e=>{if(C.route==='mine'&&e.target.matches?.('.mine-table .ex-table-scroll'))state().scrollLeft=e.target.scrollLeft;},true);
  const render=O.render;O.render=()=>{render();if(C.route!=='mine')return;const f=state(),scroll=document.querySelector('.mine-table .ex-table-scroll');if(scroll)scroll.scrollLeft=f.scrollLeft;document.querySelectorAll('[data-mine-sort]').forEach(el=>el.closest('th').setAttribute('aria-sort',el.dataset.mineSort===f.sort?(f.desc?'descending':'ascending'):'none'));const visible=rows().slice((f.page-1)*f.size,f.page*f.size),check=document.querySelector('[data-mine-page]');if(check)check.indeterminate=visible.some(m=>f.chosen.includes(m.id))&&!visible.every(m=>f.chosen.includes(m.id));};
  addEventListener('DOMContentLoaded',()=>O.render());
  O.mine={state,defaults,rows,video,download,advance,relation,setDemo,batches};
})();
