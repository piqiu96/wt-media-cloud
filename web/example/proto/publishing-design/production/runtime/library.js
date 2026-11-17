/* 素材库局部优化；共享存储和工作台组件，个人素材及合成选择器沿用原流程。 */
(() => {
  const C=CP,O=OPS,E=C.e,{table}=C.prod;
  const originalPage=C.materialPage,originalDetail=C.materialDetail;
  CPSeed().materials.forEach(seed=>{const m=C.get('materials',seed.id);if(m?.title===seed.title){m.platform=seed.platform;for(const key of ['platformWorkId','created_by','metricsUpdated'])if(!Object.hasOwn(m,key))m[key]=seed[key];}});
  C.save();
  const enabled=()=>O.categories;
  const defaults=()=>({q:'',platforms:[],categories:O.role()==='管理员'?[...enabled()]:[...(O.people.find(p=>p.name===O.actor)?.categories||({'陈小雨':['剪辑教程'],'林予安':['拍摄技巧']}[O.actor]||enabled()))],joined:'',usage:'unused',team:'',from:'',to:'',enteredFrom:'',enteredTo:'',creator:'',sort:'published',desc:true,page:1,size:8,chosen:[],result:null});
  const state=()=>{const store=C.ui.filters.materials||={};store.byActor||={};return store.byActor[O.actor]||=defaults();};
  const number=v=>typeof v==='number'&&Number.isFinite(v)?v.toLocaleString():'—';
  const platform=m=>m.platform||'—',workId=m=>m.platformWorkId??m.platform_work_id??null;
  const creator=m=>m.created_by==null?'—':O.people.find(p=>p.name===m.created_by||p.id!=null&&String(p.id)===String(m.created_by))?.name||'—';
  const personFor=owner=>owner==null?null:O.people.find(p=>p.name===owner||p.id!=null&&String(p.id)===String(owner));
  const stats=id=>{
    // Outputs are successful products, never counts of tasks or publications.
    const outputs=C.db.outputs.filter(o=>o.materialId===id&&(!o.team||o.team==='内容运营一组'));
    const groups=new Map();
    outputs.forEach(o=>{
      const person=personFor(o.owner),hasOwner=o.owner!=null&&String(o.owner).trim()!=='',key=person?'person:'+String(person.id??person.name):hasOwner?'unknown:'+String(o.owner):'unassigned';
      if(!groups.has(key))groups.set(key,{key,person,owner:hasOwner?o.owner:null,count:0});
      groups.get(key).count++;
    });
    const details=[...groups.values()],mine=outputs.filter(o=>o.owner===O.actor||personFor(o.owner)?.name===O.actor);
    return {outputs,mine,details,people:details.filter(g=>g.owner!=null).length,unassigned:groups.get('unassigned')?.count||0};
  };
  const joined=id=>C.usage(id)?.status==='active';
  const date=v=>v?String(v).slice(0,10):'';
  const matches=(m,f)=>{const s=stats(m.id);return (!f.q||`${m.id} ${workId(m)??''} ${m.title} ${m.author||''}`.toLowerCase().includes(f.q.toLowerCase()))&&(!f.platforms.length||f.platforms.includes(m.platform))&&(!f.categories.length||f.categories.includes(m.category))&&(!f.joined||joined(m.id)===(f.joined==='yes'))&&(!f.usage||(s.mine.length?'made':'unused')===f.usage)&&(!f.team||(s.outputs.length?'used':'unused')===f.team)&&(!f.from||date(m.published)>=f.from)&&(!f.to||date(m.published)&&date(m.published)<=f.to)&&(!f.enteredFrom||date(m.created)>=f.enteredFrom)&&(!f.enteredTo||date(m.created)&&date(m.created)<=f.enteredTo)&&(!C.canManage()||!f.creator||m.created_by===f.creator);};
  const value=(m,key)=>key==='outputs'?stats(m.id).outputs.length:m[key];
  const compare=(a,b,f)=>{const x=value(a,f.sort),y=value(b,f.sort),missing=v=>v==null||v==='';if(missing(x)||missing(y))return missing(x)===missing(y)?a.id-b.id:missing(x)?1:-1;return (typeof x==='number'&&typeof y==='number'?x-y:String(x).localeCompare(String(y),'zh-CN'))*(f.desc?-1:1)||a.id-b.id;};
  const rows=()=>{const f=state();return C.db.materials.filter(m=>matches(m,f)).sort((a,b)=>compare(a,b,f));};
  const repaint=()=>{C.render();C.dialogs.filter(d=>d.libraryDetail).forEach(d=>d.paint());};
  const join=ids=>{
    const result={added:[],restored:[],skipped:[],failed:[]};
    [...new Set(ids)].forEach(id=>{const m=C.get('materials',id);if(!m){result.failed.push({id,reason:'素材已不存在或无权访问'});return;}const u=C.usage(id);if(u?.status==='active')result.skipped.push(id);else if(u){u.status='active';u.added=C.time();result.restored.push(id);}else{C.usages().push({materialId:id,status:'active',download:'notDownloaded',added:C.time()});result.added.push(id);}});
    // Do not call C.join: that shared composition helper intentionally starts preparation.
    C.save();state().result=result;state().chosen=result.failed.map(x=>x.id);repaint();C.toast(`加入完成：新增 ${result.added.length}，恢复 ${result.restored.length}，跳过 ${result.skipped.length}，失败 ${result.failed.length}`);return result;
  };
  const copyIcon='<svg viewBox="0 0 20 20" aria-hidden="true" focusable="false"><path d="M7 5V2.8h10.2V13H15M3 6.8h10.2V17H3z" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/></svg>';
  const copy=async(value)=>{const button=document.activeElement;try{await navigator.clipboard.writeText(String(value));if(button?.classList.contains('lib-copy')){button.classList.add('is-copied');button.title='已复制';setTimeout(()=>{button.classList.remove('is-copied');button.title='复制完整 ID';},1600);}C.toast('已复制完整 ID');}catch{C.toast('复制失败，请选择 ID 文本后复制');}};
  const idCell=(value,label)=>value==null||value===''?'—':`<span class="lib-id"><span class="lib-id-value">${E(value)}</span>${C.btn(copyIcon,()=>copy(value),'text lib-copy',false,`aria-label="复制${label} ${E(value)}" title="复制完整 ID"`)}</span>`;
  const summary=id=>{const s=stats(id);return s.outputs.length?`${s.outputs.length} 个成片 · ${s.people} 人使用`:'暂无使用记录';};
  const usageDetail=id=>C.open({title:'使用情况',kind:'lib-usage-dialog ex-list-pattern',body:()=>{
    const s=stats(id),can=C.canManage();
    return C.facts([['成功成片数',s.outputs.length],['使用人数',s.people]])+(s.unassigned?`<p class="ops-muted">另有 ${s.unassigned} 个成片未记录负责人，不计入使用人数。</p>`:'')+(can?`<div class="lib-usage-table" tabindex="0" role="region" aria-label="使用人员明细，可滚动">${table(['使用人','成功成片数'],s.details.map(g=>{
      const valid=g.person&&g.person.active!==false&&g.person.enabled!==false&&!['inactive','deleted'].includes(g.person.status);
      return `<tr><td>${g.owner==null?'未记录负责人':valid?E(g.person.name):`负责人信息不可用<small>关联标识：${E(g.owner)}</small>`}</td><td>${g.count}</td></tr>`;
    }).join('')||'<tr><td colspan="2"><div class="ex-empty"><h3>暂无使用人员明细</h3><p>还没有成功成片。</p></div></td></tr>')}</div>`:'');
  }});
  const multiSummary=selected=>`<span class="lib-multi-values">${selected.length?`<span class="lib-multi-chip">${E(selected[0])}</span>${selected.length>1?`<span class="lib-multi-count">+${selected.length-1}</span>`:''}`:'<span class="lib-multi-placeholder">全部</span>'}</span><svg class="lib-multi-chevron" viewBox="0 0 16 16" aria-hidden="true"><path d="m4 6 4 4 4-4" fill="none" stroke="currentColor" stroke-width="1.5"/></svg>`;
  const multi=(label,key,values,selected)=>`<div class="ex-field lib-multi"><label>${label}</label><details><summary aria-label="${label}多选" title="${E(selected.join('、')||'全部')}">${multiSummary(selected)}</summary><div class="lib-options"><div class="lib-options-caption">可选择多个${label}</div>${values.map(v=>`<label class="${selected.includes(v)?'is-selected':''}"><input type="checkbox" class="ex-check" data-lib-multi="${key}" value="${E(v)}" ${selected.includes(v)?'checked':''}><span>${E(v)}</span><span class="lib-option-check" aria-hidden="true">✓</span></label>`).join('')}</div></details></div>`;
  const sortIcon=active=>`<svg class="ex-sort-icon" viewBox="0 0 20 20" aria-hidden="true">${active?'<path d="M10 3v14m-4-4 4 4 4-4"/>':'<path d="M7 16V4L4 7m3-3 3 3m3-3v12l-3-3m3 3 3-3"/>'}</svg>`;
  const advancedCount=f=>Number(Boolean(f.from||f.to))+Number(Boolean(f.enteredFrom||f.enteredTo))+Number(Boolean(f.addedFrom||f.addedTo))+Number(C.canManage()&&Boolean(f.creator));
  const advancedFilters=(key,f,body)=>{
    const count=advancedCount(f),expanded=f.advanced??Boolean(count);
    return `<details class="ex-filter-advanced" data-resource-advanced="${key}" ${expanded?'open':''}><summary>高级筛选${count?`<span class="ex-filter-applied">（已生效 ${count} 项）</span>`:''}</summary><div class="ex-filter-extra">${body}</div></details>`;
  };
  const pagination=(f,total,max,attribute,label)=>`<footer class="ex-pagination ex-pagination-compact"><span aria-live="polite">共 ${total} 条</span><div class="ex-actions">
    <select ${attribute} aria-label="${label}每页条数">${[8,20,50].map(n=>`<option value="${n}" ${f.size===n?'selected':''}>${n} 条 / 页</option>`).join('')}</select>
    ${C.btn('‹',()=>{f.page--;C.render();},'ex-page-number',f.page<=1,'aria-label="上一页"')}
    <span class="ex-tag info" aria-current="page" aria-label="第 ${f.page} 页，共 ${max} 页">${f.page}</span>
    ${C.btn('›',()=>{f.page++;C.render();},'ex-page-number',f.page>=max,'aria-label="下一页"')}
  </div></footer>`;
  const sortHead=(label,key)=>{
    const f=state(),active=f.sort===key,direction=f.desc?'降序':'升序',next=active&&f.desc?'升序':'降序';
    return `<span data-lib-sort="${key}" aria-label="${label}排序：${active?direction:'未排序'}">${C.btn(label+sortIcon(active),()=>{f.desc=f.sort===key?!f.desc:true;f.sort=key;f.page=1;C.render();},'text ex-sort-button'+(active?' is-active':'')+(active&&!f.desc?' is-ascending':''),false,`aria-label="${label}${active?'，当前'+direction:''}，点击按${next}" title="点击按${label}${next}"`)}</span>`;
  };
  const dateRange=(label,key,start,end)=>`<div class="ex-field lib-range ex-filter-range"><label id="lib-range-${key}">${label}</label><div data-library-range="${key}" aria-labelledby="lib-range-${key}"><input type="hidden" data-field="${key==='published'?'from':'enteredFrom'}" value="${E(start)}"><input type="hidden" data-field="${key==='published'?'to':'enteredTo'}" value="${E(end)}"><div data-library-range-host></div></div></div>`;
  const reset=()=>{const store=C.ui.filters.materials;store.byActor[O.actor]=defaults();C.render();};
  const select=(ids,on)=>{const f=state(),s=new Set(f.chosen);ids.forEach(id=>on?s.add(id):s.delete(id));f.chosen=[...s];C.render();};
  const performance=(m,detail=false)=>{
    const failed=m.metricsStatus==='failed'||Boolean(m.metricsError),hasValues=[m.views,m.likes,m.favorites].some(v=>typeof v==='number'&&Number.isFinite(v));
    return `<div class="${detail?'lib-detail-performance':''}"><div class="lib-metrics">${[['views','浏览'],['likes','点赞'],['favorites','收藏']].map(([key,label])=>`<span>${label}${detail?'量':''}<b>${number(m[key])}</b>${m.metricErrors?.[key]?'<small class="lib-metric-error">采集失败</small>':''}</span>`).join('')}</div><small class="lib-metrics-time">数据更新时间：${E(m.metricsUpdated||'—')}</small>${failed?`<p class="lib-metric-error" role="status">采集失败${hasValues?'，当前保留上次取得的数据':'，暂无可用数据'}。</p>`:''}</div>`;
  };
  const cover=(m,detail=false)=>m.coverUrl&&/^https?:\/\//.test(m.coverUrl)?`<img class="${detail?'lib-detail-cover':''}" src="${E(m.coverUrl)}" alt="${E(m.title)}封面">`:C.coverPlaceholder(detail?'lib-detail-cover':'');
  const sourceLink=m=>O.reference('打开原作品',m.sourceUrl)+`<small class="lib-source-hint">${m.sourceState==='expired'||!m.sourceState&&m.cloudState==='expired'?'来源已失效':m.sourceState==='available'?'来源可用':'来源未核验'}</small>`;
  // Preparation, object existence and verified access remain independent facts.
  // Source validity must never decide cloud-file availability.
  const cloudResource=m=>{
    let valid=false;try{const url=new URL(m.cloudUrl);valid=['http:','https:'].includes(url.protocol);}catch{}
    const expiry=m.cloudUrlExpiresAt?Date.parse(m.cloudUrlExpiresAt):Infinity;
    if(m.cloudFileExists===true&&valid&&m.cloudUrlVerified===true&&expiry>Date.now())return {label:'可用',tone:'success',link:m.cloudDemo===true||m.cloudUrl==='https://media.example.test/prototype-only.mp4'?'':O.reference('打开云端视频',m.cloudUrl)};
    if(m.cloudFileExists===true)return {label:expiry<=Date.now()?'访问链接已过期':'待核验',tone:expiry<=Date.now()?'warning':'',link:''};
    if(m.cloudState==='preparing')return {label:'准备中',tone:'info',link:''};
    if(m.cloudState==='failed')return {label:'准备失败',tone:'danger',link:''};
    if(m.cloudFileExists===false)return {label:m.cloudState==='notSaved'?'未准备':'文件缺失',tone:m.cloudState==='notSaved'?'':'danger',link:''};
    return {label:m.cloudState==='notSaved'?'未准备':'待核验',tone:'',link:''};
  };
  const cloudFile=m=>cloudResource(m).link;
  const fileResources=id=>MineVideo.resources(C.get('materials',id)||{},C.usage(id)||{},O.mineDevice||'current');
  const localPath=(id,file)=>typeof file?.path==='string'&&file.path.trim()?file.path.trim():file?.demo?`示例下载/内容运营/素材-${id}.mp4`:'';
  const openFolder=id=>{
    const r=fileResources(id),path=localPath(id,r.file);
    if(!r.local||!path){C.toast(r.local?'文件位置未记录，无法定位所在文件夹':'当前电脑的文件不存在或尚未核验');return;}
    const split=Math.max(path.lastIndexOf('/'),path.lastIndexOf('\\')),folder=split>=0?path.slice(0,split)||'/':path;
    C.open({title:r.file.demo?'文件位置 · 原型演示':'文件位置',body:()=>C.facts([['所在文件夹',E(folder),true],['视频文件',E(path.slice(split+1)),true]])+`<p class="ops-note">${r.file.demo?'这是示例下载位置，不对应真实文件。':''}当前原型未接入桌面文件定位能力，未打开系统文件夹。</p>`});
  };
  const openCloudDemo=id=>{
    const r=fileResources(id);if(!r.remote||!r.cloudDemo){C.toast('视频资源状态已变化，请重新打开详情');return;}
    C.open({title:'云端视频 · 原型演示',body:()=>C.facts([['素材',E(C.get('materials',id)?.title||id)]])+`<p class="ops-note">这是打开云端视频的交互演示，尚未接入真实媒体文件。接入有效地址后，此入口在新标签页打开视频；本次不会下载或转存文件。</p>`});
  };
  const fileAccessRows=(id,statusHTML)=>{
    const r=fileResources(id),resource=cloudResource(r.effective);
    const cloudAccess=r.remote?(r.cloudDemo?C.btn('打开云端视频',()=>openCloudDemo(id),'text')+'<small class="lib-source-hint">演示入口 · 未接入真实视频</small>':O.reference('打开云端视频',r.cloudUrl)):'';
    const rows=[['视频状态',statusHTML??C.tag(resource.label,resource.tone)],['云端视频',cloudAccess||'—']];
    if(r.local){const path=localPath(id,r.file);rows.push(['本机文件',path?C.btn('打开所在文件夹',()=>openFolder(id),'text')+`<small class="lib-source-hint">${r.file.demo?'演示位置 · ':''}${E(path)}</small>`:'已下载，文件位置未记录',true]);}
    return rows;
  };

  const resultHTML=()=>{const r=state().result;if(!r)return '';return `<div class="ops-result" role="status"><span>新增 ${r.added.length} · 恢复 ${r.restored.length} · 已加入跳过 ${r.skipped.length} · 失败 ${r.failed.length}</span>${r.failed.length?C.btn('重试失败项',()=>join(r.failed.map(x=>x.id)),'text')+r.failed.map(x=>`<span>素材 ${E(x.id)}：${E(x.reason)}</span>`).join(''):''}${C.btn('关闭结果',()=>{state().result=null;C.render();},'text')}</div>`;};
  C.materialPage=mine=>{
    if(mine)return originalPage(mine);
    const f=state(),all=rows(),max=Math.max(1,Math.ceil(all.length/f.size));f.page=Math.min(f.page,max);const visible=all.slice((f.page-1)*f.size,f.page*f.size),chosen=new Set(f.chosen),pageChecked=visible.length&&visible.every(m=>chosen.has(m.id));
    const heads=[`<input type="checkbox" class="ex-check" data-lib-page aria-label="选择本页素材" ${pageChecked?'checked':''} ${visible.length?'':'disabled'}>`,'素材 ID','平台','平台作品 ID','素材信息','类目',sortHead('发布时间','published'),`内容表现 <div class="lib-metric-sort">${sortHead('浏览','views')} ${sortHead('点赞','likes')} ${sortHead('收藏','favorites')}</div>`,'加入状态','我的使用',sortHead('团队使用','outputs'),sortHead('入库时间','created'),'操作人','操作'];
    const body=visible.map(m=>{const s=stats(m.id);return `<tr><td><input type="checkbox" class="ex-check" data-lib-row="${m.id}" aria-label="选择素材 ${m.id}" ${chosen.has(m.id)?'checked':''}></td><td>${idCell(m.id,'素材 ID')}</td><td>${E(platform(m))}</td><td>${idCell(workId(m),'平台作品 ID')}</td><td><div class="lib-material">${cover(m)}<div>${C.link(m.title,()=>C.materialDetail(m.id))}<small>${E(m.author||'—')}</small></div></div></td><td>${E(m.category||'—')}</td><td>${E(m.published||'—')}</td><td>${performance(m)}</td><td>${C.tag(joined(m.id)?'已加入':'未加入',joined(m.id)?'info':'')}</td><td>${C.tag(s.mine.length?'已合成':'未使用',s.mine.length?'success':'')}</td><td>${s.outputs.length?C.link(summary(m.id),()=>usageDetail(m.id)):'—'}</td><td>${E(m.created||'—')}</td><td>${E(creator(m))}</td><td>${joined(m.id)?'':C.row('加入我的素材',()=>join([m.id]),true)}${C.row('详情',()=>C.materialDetail(m.id))}</td></tr>`;}).join('');
    return C.head('素材库','浏览团队公共素材，核对内容表现与使用情况，加入个人工作集。')+resultHTML()+`<section class="ex-panel lib-panel ex-list-pattern"><form class="ex-filters ex-filters-stacked" data-library-filter>${C.field('综合搜索','q',f.q,{placeholder:'素材 ID、平台作品 ID、标题、作者',className:'ex-filter-search'})}${multi('来源平台','platforms',O.sourcePlatforms,f.platforms)}${multi('类目','categories',enabled(),f.categories)}${C.select('joined','加入状态',f.joined,[['yes','已加入'],['no','未加入']])}${C.select('usage','我的使用',f.usage,[['unused','未使用'],['made','已合成']])}${C.select('team','团队使用',f.team,[['unused','无人使用'],['used','有人使用']])}<div class="ex-actions"><button class="ex-button primary" type="submit">查询</button>${C.btn('重置',reset)}</div>${advancedFilters('materials',f,dateRange('发布时间','published',f.from,f.to)+dateRange('入库时间','entered',f.enteredFrom,f.enteredTo)+(C.canManage()?C.select('creator','操作人',f.creator,O.peopleOptions()):''))}<p class="ops-error" data-library-error role="alert" hidden></p></form>`+
      (chosen.size?`<div class="ex-selection"><span>已选择 <strong>${chosen.size}</strong> 条（跨页保留），本页 ${visible.filter(m=>chosen.has(m.id)).length} 条</span>${C.btn('加入我的素材',()=>join(f.chosen),'primary')}${C.btn('选择全部 '+all.length+' 条匹配结果',()=>select(all.map(m=>m.id),true),'text')}${C.btn('取消选择',()=>{f.chosen=[];C.render();},'text')}</div>`:'')+
      `<div class="lib-table">${table(heads,body||`<tr><td colspan="14"><div class="ex-empty"><h3>${C.db.materials.length?'没有符合条件的素材':'素材库暂无素材'}</h3><p>${C.db.materials.length?'可调整筛选条件，或重置恢复默认范围。':'团队素材入库后将在此展示。'}</p>${C.db.materials.length?C.btn('清除筛选条件',()=>{Object.assign(f,defaults(),{categories:[],usage:''});C.render();}):''}</div></td></tr>`).replace('<div class="ex-table-scroll">','<div class="ex-table-scroll" tabindex="0" role="region" aria-label="素材列表，可横向滚动">')}</div>`+
      `${pagination(f,all.length,max,'data-library-size','素材库')}<details class="ops-status-guide"><summary>状态说明</summary><p>加入状态只表示个人工作集关系；我的使用只统计本人负责的成功成片；团队使用统计团队成功成片及使用人数。失败、排队和取消任务均不计入。加入素材不准备视频，不创建任务。</p></details></section>`;
  };
  C.materialDetail=id=>{
    if(C.route!=='materials')return originalDetail(id);
    const m=C.get('materials',id);if(!m){C.toast('素材不存在或无权访问');return;}
    let d;d=C.open({title:'素材详情',kind:'ex-drawer complex lib-detail ex-list-pattern',body:()=>{
      const s=stats(id);return `<div class="lib-detail-heading">${cover(m,true)}<div><h3>${E(m.title)}</h3><p>${E(m.author||'—')}</p></div></div>`+
        C.section('基本信息',C.facts([['素材 ID',idCell(id,'素材 ID')],['平台',E(platform(m))],['平台作品 ID',idCell(workId(m),'平台作品 ID')],['平台作品链接',sourceLink(m)],['类目',E(m.category||'—')],['发布时间',E(m.published||'—')],['入库时间',E(m.created||'—')],['操作人',E(creator(m))]]))+
        C.section('内容表现',performance(m,true))+
        C.section('使用情况',C.facts([['加入状态',joined(id)?'已加入':'未加入'],['我的使用',s.mine.length?'已合成':'未使用'],['团队使用',C.link(summary(id),()=>usageDetail(id)),true]]))+
        C.section('视频文件',C.facts(fileAccessRows(id)));
    }});d.libraryDetail=true;return d;
  };
  document.addEventListener('submit',e=>{if(!e.target.matches('[data-library-filter]'))return;e.preventDefault();const form=e.target,f=state(),next={};const incomplete=[...form.querySelectorAll('[data-library-range]')].find(field=>{const inputs=[...field.querySelectorAll('[data-library-range-host] input')];return inputs.length===2&&Boolean(inputs[0].value)!==Boolean(inputs[1].value);});if(incomplete){const error=form.querySelector('[data-library-error]');error.hidden=false;error.textContent='请选择完整的日期范围';return;}form.querySelectorAll('[data-field]').forEach(el=>next[el.dataset.field]=el.value.trim());for(const key of ['platforms','categories'])next[key]=[...form.querySelectorAll(`[data-lib-multi="${key}"]:checked`)].map(el=>el.value);if(next.from&&next.to&&next.from>next.to||next.enteredFrom&&next.enteredTo&&next.enteredFrom>next.enteredTo){const el=form.querySelector('[data-library-error]');el.hidden=false;el.textContent='开始日期不能晚于结束日期';return;}Object.assign(f,next,{page:1,chosen:[],result:null});C.render();});
  document.addEventListener('change',e=>{if(C.route!=='materials')return;const el=e.target;if(el.matches('[data-lib-row]'))select([+el.dataset.libRow],el.checked);if(el.matches('[data-lib-page]'))select(rows().slice((state().page-1)*state().size,state().page*state().size).map(m=>m.id),el.checked);if(el.matches('[data-library-size]')){state().size=+el.value;state().page=1;C.render();}if(el.matches('[data-lib-multi]')){const checked=[...el.closest('details').querySelectorAll('input:checked')].map(x=>x.value);const summary=el.closest('details').querySelector('summary');summary.innerHTML=multiSummary(checked);summary.title=checked.join('、')||'全部';el.closest('label').classList.toggle('is-selected',el.checked);}});
  document.addEventListener('toggle',e=>{if(e.target.isConnected&&C.route==='materials'&&e.target.matches?.('[data-resource-advanced="materials"]'))state().advanced=e.target.open;},true);
  // Native checkbox indeterminate state is set after each library render only.
  document.addEventListener('scroll',e=>{if(C.route==='materials'&&e.target.matches?.('.lib-table .ex-table-scroll'))state().scrollLeft=e.target.scrollLeft;},true);
  const render=O.render;O.render=()=>{render();window.LibraryDateRange?.mount(document);if(C.route==='materials'){document.querySelectorAll('[data-lib-sort]').forEach(span=>{const th=span.closest('th');if(th)th.setAttribute('aria-sort','none');});const active=document.querySelector(`[data-lib-sort="${state().sort}"]`);active?.closest('th')?.setAttribute('aria-sort',state().desc?'descending':'ascending');const scroll=document.querySelector('.lib-table .ex-table-scroll');if(scroll)scroll.scrollLeft=state().scrollLeft||0;const el=document.querySelector('[data-lib-page]'),visible=rows().slice((state().page-1)*state().size,state().page*state().size);if(el)el.indeterminate=visible.some(m=>state().chosen.includes(m.id))&&!visible.every(m=>state().chosen.includes(m.id));}};
  document.addEventListener('keydown',e=>{if(e.key==='Escape'&&e.target.closest('.lib-multi details[open]')){const panel=e.target.closest('details');panel.open=false;panel.querySelector('summary').focus();e.preventDefault();e.stopPropagation();}},true);
  document.addEventListener('click',e=>{document.querySelectorAll('.lib-multi details[open]').forEach(panel=>{if(!panel.contains(e.target))panel.open=false;});});
  O.library={state,defaults,stats,matches,compare,join,creator,cloudFile,cloudResource,performance,usageDetail,idCell,cover,sourceLink,multiSummary,multi,sortIcon,advancedCount,advancedFilters,pagination,fileResources,fileAccessRows};
})();
