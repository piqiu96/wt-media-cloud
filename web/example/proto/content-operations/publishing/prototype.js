/* 本地原型基础：导航、资源选择、原生Dialog、反馈。无网络与真实执行。 */
(() => {
  'use strict';
  const P = window.PI = {};
  const KEY = 'wt-operations-execution-v3';
  // 读取时兼容旧平台名称，保留已有演示数据及平台分配关系。
  try { P.db = JSON.parse(localStorage.getItem(KEY)?.replaceAll('\u767e\u5bb6\u53f7','百度') || 'null'); } catch { /* file://可能禁用存储。 */ }
  if (P.db?.version !== 2) P.db = PISeed();
  P.ui = {route:'publish/tasks',selected:new Set(),filters:{},page:1,size:8,review:false,replyId:9101,replyFilter:'pending',replyQuery:'',replyDraft:{}};
  P.callbacks = new Map(); P.serial = 0; P.dialogs = [];
  P.$ = (s,r=document) => r.querySelector(s);
  P.$$ = (s,r=document) => [...r.querySelectorAll(s)];
  P.e = value => String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  P.icon = name => PROTO_ICONS[name] || PROTO_ICONS.Browse;
  P.asset = id => P.db.assets.find(a=>a.id===Number(id));
  P.account = id => P.db.accounts.find(a=>a.id===Number(id));
  P.plan = id => P.db.plans.find(a=>a.id===Number(id));
  P.task = id => P.db.tasks.find(a=>a.id===Number(id));
  P.interaction = id => P.db.interactions.find(a=>a.id===Number(id));
  P.save = () => { try { localStorage.setItem(KEY,JSON.stringify(P.db)); } catch { /* 数据仍留在本页内存。 */ } };
  P.id = () => ++P.db.seq;
  P.action = fn => { const id='p'+String(++P.serial); P.callbacks.set(id,fn); return id; };
  P.btn = (label,fn,kind='',disabled=false,attrs='') => `<button type="button" class="${kind.includes('text')?'ex-text-button':'ex-button'} ${kind.replace('text','')}" data-action="${P.action(fn)}" ${disabled?'disabled':''} ${attrs}>${label}</button>`;
  P.rowBtn = (label,fn,primary=false) => P.btn(label,fn,'text'+(primary?' ex-row-primary':''));
  P.link = (label,fn) => `<button type="button" class="pi-inline-link" data-action="${P.action(fn)}">${P.e(label)}</button>`;
  P.tag = (text,tone='') => `<span class="ex-tag ${tone}">${P.e(text)}</span>`;
  P.channel = c => c==='api'?'API':'浏览器';
  P.mode = m => m==='assisted'?'人工连续':'全自动';
  P.date = v => v ? v.replace('2026-','').replace('T',' ') : '—';
  P.statuses = {draft:['草稿',''],ready:['待开始',''],running:['执行中','info'],paused:['已暂停',''],completed:['已完成','success'],partial:['部分完成','warning'],attention:['需处理','warning'],cancelled:['已取消',''],published:['已发布','success'],preparing:['准备中','info'],waiting:['待人工提交','warning'],failed:['未完成','danger'],submitted:['待补链接','warning'],pending:['未处理',''],success:['成功','success'],uncertain:['待确认','warning'],skipped:['已跳过',''],handled:['已处理',''],done:['已完成','success']};
  P.status = (v,label) => P.tag(label||P.statuses[v]?.[0]||v,P.statuses[v]?.[1]||'');
  P.cover = a => `<div class="pi-cover" data-kind="${a?.kind||0}" role="img" aria-label="${P.e(a?.title||'演示作品')}封面"><span>${a?.kind===1?'实战':'攻略'}</span></div>`;
  P.media = (a,title) => `<div class="pi-title-cell">${P.cover(a)}<div><strong>${P.e(title||a?.title||'外部作品')}</strong><small>${P.e(a?.category||'作品内容')} ${a?.duration?' · '+a.duration:''}</small></div></div>`;
  P.field = (label,name,value='',opts={}) => {
    const id='field-'+(++P.serial), extra=`data-bind="${name}" ${opts.redraw?'data-redraw="true"':''}`;
    const control=opts.options?`<select id="${id}" ${extra}>${opts.options.map(o=>{const [v,t]=Array.isArray(o)?o:[o,o];return `<option value="${P.e(v)}" ${String(v)===String(value)?'selected':''}>${P.e(t)}</option>`;}).join('')}</select>`:opts.area?`<textarea id="${id}" ${extra} placeholder="${P.e(opts.placeholder||'')}">${P.e(value)}</textarea>`:`<input id="${id}" ${extra} type="${opts.type||'text'}" value="${P.e(value)}" ${opts.min!==undefined?'min="'+opts.min+'"':''} ${opts.max!==undefined?'max="'+opts.max+'"':''} placeholder="${P.e(opts.placeholder||'')}" ${opts.required?'required':''}>`;
    return `<div class="ex-field ${opts.full?'full':''} ${opts.required?'required':''}"><label for="${id}">${label}</label>${control}${opts.help?`<span class="ex-help">${opts.help}</span>`:''}</div>`;
  };
  P.check = (label,name,value,redraw=false) => `<label class="pi-check-label"><input type="checkbox" class="ex-check" data-bind="${name}" ${value?'checked':''} ${redraw?'data-redraw="true"':''}>${label}</label>`;
  P.radios = (label,name,value,choices) => `<fieldset class="ex-choices pi-mode-grid"><legend>${label}</legend>${choices.map(([v,title,desc])=>`<label class="ex-choice"><input type="radio" name="${name}" data-bind="${name}" data-redraw="true" value="${v}" ${v===value?'checked':''}><span><strong>${title}</strong><small>${desc}</small></span></label>`).join('')}</fieldset>`;
  P.section = (title,body,form=false) => `<section class="ex-section"><h2 class="ex-section-title ${form?'form':'detail'}">${title}</h2>${body}</section>`;
  P.facts = items => `<div class="ex-description-container"><dl class="ex-description-list" data-density="dense">${items.map(([k,v,full])=>`<div class="ex-desc-item ${full?'full':''}"><dt>${k}</dt><dd>${v}</dd></div>`).join('')}</dl></div>`;
  P.notice = (text,tone='') => `<div class="ex-notice ${tone}"><span class="ex-notice-icon">i</span><div>${text}</div></div>`;
  P.empty = (title,desc,action='') => `<div class="ex-empty"><div class="ex-empty-symbol">${P.icon('Search')}</div><h3>${title}</h3><p>${desc}</p>${action}</div>`;
  P.head = (title,desc,actions='') => `<div class="ex-page-head"><div><h1>${title}</h1>${desc?`<p>${desc}</p>`:''}</div><div class="pi-head-actions">${actions}</div></div>`;
  P.tabs = () => '';

  P.toast = (message,tone='info') => {
    const el=P.$('#prototype-toast'); el.textContent=message; el.className='ex-toast '+tone; el.setAttribute('role',tone==='danger'?'alert':'status');
    clearTimeout(P.toastTimer); P.toastTimer=setTimeout(()=>{el.textContent='';},4000);
  };
  P.nav = route => { if(P.ui.route!==route) { P.ui.selected.clear(); P.ui.page=1; P.ui.review=false; P.ui.route=route; location.hash=route; } else P.render(); };
  P.readFilter = () => P.ui.filters[P.ui.route] ||= {q:'',platform:'',status:'',mode:''};
  P.filterRows = rows => { const f=P.readFilter(); return rows.filter(a=>(!f.q||JSON.stringify(a).toLowerCase().includes(f.q.toLowerCase()))&&(!f.platform||(a.platform||P.account(a.accountId)?.platform)===f.platform)&&(!f.mode||a.mode===f.mode)&&(!f.status||a.status===f.status)); };
  P.filterPanel = (placeholder,options={}) => {
    const f=P.readFilter();
    return `<form class="ex-filters" data-filter-form><div class="ex-field ex-filter-search"><label for="pi-list-q">综合搜索</label><input id="pi-list-q" name="q" value="${P.e(f.q)}" placeholder="${placeholder}"></div>${options.platform!==false?`<div class="ex-field ex-filter-select"><label for="pi-list-platform">平台</label><select id="pi-list-platform" name="platform"><option value="">全部平台</option>${PIPublishModel.platforms.map(p=>`<option ${f.platform===p?'selected':''}>${p}</option>`).join('')}</select></div>`:''}${options.mode?`<div class="ex-field ex-filter-select"><label for="pi-list-mode">执行方式</label><select id="pi-list-mode" name="mode"><option value="">全部方式</option><option value="auto" ${f.mode==='auto'?'selected':''}>全自动</option><option value="assisted" ${f.mode==='assisted'?'selected':''}>人工连续</option></select></div>`:''}<button class="ex-button primary" type="submit">查询</button>${P.btn('重置',()=>{P.ui.filters[P.ui.route]={q:'',platform:'',status:'',mode:''};P.ui.page=1;P.ui.selected.clear();P.render();})}</form>`;
  };
  P.stateChips = pairs => { const f=P.readFilter();return `<div class="pi-chips" aria-label="状态筛选">${pairs.map(([s,t])=>`<button class="pi-chip ${f.status===s?'active':''}" aria-pressed="${f.status===s}" data-action="${P.action(()=>{f.status=s;P.ui.page=1;P.ui.selected.clear();P.render();})}">${t}</button>`).join('')}</div>`; };
  P.pageRows = rows => {const max=Math.max(1,Math.ceil(rows.length/P.ui.size));P.ui.page=Math.max(1,Math.min(P.ui.page,max));return rows.slice((P.ui.page-1)*P.ui.size,P.ui.page*P.ui.size);};
  P.pagination = total => {const max=Math.max(1,Math.ceil(total/P.ui.size));return `<div class="ex-pagination"><span>共 ${total} 条</span><div class="ex-actions"><label class="pi-meta" for="pi-page-size">每页</label><div class="ex-field pi-page-size"><select id="pi-page-size" data-page-size aria-label="每页数量">${[8,20,50].map(n=>`<option value="${n}" ${P.ui.size===n?'selected':''}>${n} 条／页</option>`).join('')}</select></div>${P.btn('‹',()=>{P.ui.page--;P.ui.selected.clear();P.render();},'ex-page-number',P.ui.page<=1,'aria-label="上一页"')}<span>${P.ui.page} / ${max}</span>${P.btn('›',()=>{P.ui.page++;P.ui.selected.clear();P.render();},'ex-page-number',P.ui.page>=max,'aria-label="下一页"')}</div></div>`;};
  P.selection = (name,actions) => P.ui.selected.size?`<div class="ex-selection"><span>当前页已选 <strong>${P.ui.selected.size}</strong> ${name}</span>${actions}${P.btn('取消选择',()=>{P.ui.selected.clear();P.render();},'text')}</div>`:'';
  P.rowCheck = id => `<input class="ex-check" type="checkbox" data-row-select="${id}" aria-label="选择 ${id}" ${P.ui.selected.has(id)?'checked':''}>`;
  P.checkAll = ids => `<input class="ex-check" type="checkbox" data-all-select="${ids.join(',')}" aria-label="选择当前页" ${ids.length&&ids.every(id=>P.ui.selected.has(id))?'checked':''}>`;
  P.open = opts => {
    const d=document.createElement('dialog'), trigger=document.activeElement;
    d.className='ex-overlay '+(opts.kind||''); d._opts=opts; d._dirty=false; d.setAttribute('aria-label',opts.title);
    d.paint=()=>{
      WTControls.dismiss();
      const footer=typeof opts.footer==='function'?opts.footer():opts.footer;
      d.innerHTML=`<header class="ex-overlay-header"><h2>${opts.title}</h2>${P.btn('×',()=>P.close(d),'ex-close',false,`aria-label="关闭${opts.title}"`)}</header><div class="ex-overlay-body ${opts.bodyClass||''}">${typeof opts.body==='function'?opts.body():opts.body}</div>${footer?`<footer class="ex-overlay-footer">${footer}</footer>`:''}`;
      WTControls.enhance(d); opts.wire?.(d);
    };
    d.addEventListener('cancel',e=>{e.preventDefault();P.close(d);});
    d.addEventListener('input',()=>{if(opts.form)d._dirty=true;});
    d.addEventListener('change',()=>{if(opts.form)d._dirty=true;});
    d.addEventListener('close',()=>{P.dialogs=P.dialogs.filter(x=>x!==d);d.remove();P.dialogs.forEach((x,i)=>x.classList.toggle('ex-underlay',i<P.dialogs.length-1));document.body.style.overflow=P.dialogs.length?'hidden':'';if(trigger?.isConnected)trigger.focus();});
    P.dialogs.push(d);P.dialogs.forEach((x,i)=>x.classList.toggle('ex-underlay',i<P.dialogs.length-1));document.body.append(d);document.body.style.overflow='hidden';d.paint();d.showModal();return d;
  };
  P.close = (d=P.dialogs.at(-1),force=false) => {
    if(!d)return;
    if(d._dirty&&!force){P.confirm('放弃未保存的修改？','本次修改尚未保存。','放弃修改',()=>P.close(d,true));return;}
    WTControls.dismiss();d.close();P.dialogs=P.dialogs.filter(x=>x!==d);d.remove();P.dialogs.forEach((x,i)=>x.classList.toggle('ex-underlay',i<P.dialogs.length-1));document.body.style.overflow=P.dialogs.length?'hidden':'';
  };
  P.confirm = (title,text,label,fn,danger=false) => {let d;d=P.open({title,body:`<p>${text}</p>`,footer:()=>P.btn('取消',()=>P.close(d,true))+P.btn(label,()=>{P.close(d,true);fn();},danger?'danger':'primary')});};
  P.detailAsset = id => { const a=P.asset(id); P.open({title:'成片详情',kind:'ex-drawer',body:`<div class="pi-asset-details">${P.cover(a)}${P.facts([['成片 ID',a.id],['内容质量',P.e(a.quality)],['成片标题',P.e(a.title),true],['业务分类',P.e(a.category)],['时长',a.duration],['画幅／版本',(a.format||'16:9')+' · '+(a.variant||'通用版')],['来源成片',a.sourceAssetId||'原始生成成片'],['生成时间',a.date]])}</div>${P.section('内容说明','<p class="pi-muted">这是用于发布流程评审的演示成片，未加载真实视频文件。</p>')}`}); };
  P.pick = (type,ids,apply,opts={}) => {
    const source=type==='assets'?P.db.assets:P.db.accounts;
    const allowed=source.filter(a=>(!opts.rootOnly||!a.sourceAssetId)&&(!opts.platform||a.platform===opts.platform)&&(!opts.channel||opts.channel!=='api'||a.channels?.includes('api')));
    const chosen=new Set(ids.filter(id=>allowed.some(a=>a.id===id&&a.enabled!==false)));
    let q='',quality='',platform=opts.platform||'',page=1,d;
    const filtered=()=>allowed.filter(a=>(!q||(a.title||a.name).includes(q))&&(!quality||a.quality===quality)&&(!platform||a.platform===platform));
    d=P.open({title:opts.title||(type==='assets'?'选择成片':'选择账号'),kind:'wide',bodyClass:'pi-picker-body',body:()=>{
      const rows=filtered(),max=Math.max(1,Math.ceil(rows.length/8));page=Math.min(page,max);const visible=rows.slice((page-1)*8,page*8);
      return `<form class="ex-filters" data-picker-filter><div class="ex-field ex-filter-search"><label for="picker-q">搜索</label><input id="picker-q" name="q" value="${P.e(q)}" placeholder="${type==='assets'?'成片标题':'账号昵称'}"></div><div class="ex-field ex-filter-select"><label for="picker-quality">质量分组</label><select id="picker-quality" name="quality"><option value="">全部分组</option>${['优质','普通'].map(v=>`<option ${quality===v?'selected':''}>${v}</option>`).join('')}</select></div>${type==='accounts'?opts.platform?`<div class="ex-field ex-filter-select"><label>平台</label><div class="pi-fixed-field">${opts.platform} · 已限定</div></div>`:`<div class="ex-field ex-filter-select"><label for="picker-platform">平台</label><select id="picker-platform" name="platform"><option value="">全部平台</option>${PIPublishModel.platforms.map(v=>`<option ${v===platform?'selected':''}>${v}</option>`).join('')}</select></div>`:''}<button class="ex-button primary">查询</button></form><div class="pi-picker-scroll"><table class="ex-table pi-picker-table"><thead><tr><th><input class="ex-check" type="checkbox" data-picker-all aria-label="选择当前页可用资源" ${visible.length&&visible.filter(a=>a.enabled!==false).every(a=>chosen.has(a.id))?'checked':''}></th><th>ID</th><th>${type==='assets'?'成片':'社媒账号'}</th><th>质量分组</th><th>${type==='assets'?'分类／画幅':'平台'}</th><th>${type==='assets'?'时长':'可用情况'}</th></tr></thead><tbody>${visible.map(a=>`<tr class="ex-resource-row"><td><input class="ex-check" type="checkbox" data-picker-id="${a.id}" aria-label="选择${P.e(a.title||a.name)}" ${chosen.has(a.id)?'checked':''} ${a.enabled===false?'disabled':''}></td><td>${a.id}</td><td>${type==='assets'?P.media(a):`<strong class="pi-row-name">${P.e(a.name)}</strong><small>${P.e(a.window)}</small>`}</td><td>${P.tag(a.quality)}</td><td>${type==='assets'?a.category+' · '+a.format:a.platform}</td><td>${type==='assets'?a.duration:P.tag(a.enabled?'可使用':a.reason,a.enabled?'success':'warning')}</td></tr>`).join('')||`<tr><td colspan="6">${P.empty('没有匹配的资源','调整搜索或质量分组。')}</td></tr>`}</tbody></table></div><div class="ex-pagination"><span>共 ${rows.length} 条${opts.channel==='api'?' · 仅展示可用 API 账号':''}</span><div class="ex-actions">${P.btn('‹',()=>{page--;d.paint();},'',page<=1,'aria-label="上一页资源"')}<span>${page} / ${max}</span>${P.btn('›',()=>{page++;d.paint();},'',page>=max,'aria-label="下一页资源"')}</div></div>`;
    },footer:()=>`<span class="pi-picker-count">已选择 <strong>${chosen.size}</strong> ${type==='assets'?'条成片':'个账号'}</span>`+P.btn('取消',()=>P.close(d,true))+P.btn('使用所选',()=>{apply([...chosen]);P.close(d,true);if(P.dialogs.length){P.dialogs.at(-1)._dirty=true;P.dialogs.at(-1).paint();}else P.render();},'primary'),wire:el=>{
      P.$('[data-picker-filter]',el).addEventListener('submit',e=>{e.preventDefault();const form=new FormData(e.target);q=form.get('q');quality=form.get('quality');platform=opts.platform||form.get('platform')||'';page=1;d.paint();});
      P.$$('[data-picker-id]',el).forEach(input=>input.addEventListener('change',()=>{input.checked?chosen.add(+input.dataset.pickerId):chosen.delete(+input.dataset.pickerId);d.paint();}));
      P.$('[data-picker-all]',el).addEventListener('change',e=>{filtered().slice((page-1)*8,page*8).filter(a=>a.enabled!==false).forEach(a=>e.target.checked?chosen.add(a.id):chosen.delete(a.id));d.paint();});
    }});return d;
  };
  P.viewWork = task => P.open({title:'作品详情',kind:'ex-drawer',body:P.section('作品信息',P.facts([['作品标题',P.e(task.title),true],['平台',P.e(P.account(task.accountId)?.platform||task.target?.platform||'B站')],['发布账号',P.e(P.account(task.accountId)?.name||task.target?.author||'演示作者')],['作品链接',P.e(task.url||task.target?.url||'待取得链接'),true],['说明','此链接为演示作品地址，不访问外部平台。',true]])),footer:()=>P.btn('对该作品发起互动',()=>{P.close(undefined,true);P.beginInteraction?.({title:task.title,platform:P.account(task.accountId)?.platform||task.target?.platform||'B站',author:P.account(task.accountId)?.name||task.target?.author||'演示作者',url:task.url||task.target?.url,kind:task.target?.kind||'internal',assetId:task.assetId});},'primary')});
  P.sceneLinks=[['publish/plans','发布计划列表'],['publish/plan/new','新增计划 · 公共内容与目标平台'],['publish/plan/rules','计划 · 分平台配置'],['publish/plan/preview','计划 · 多平台发布安排总览'],['publish/tasks','发布任务列表'],['publish/compose','普通发布配置'],['publish/workbench','人工连续发布 · 5窗口'],['publish/auto','全自动执行工作区'],['interact/tasks','主动互动列表'],['interact/create','外部链接批量互动'],['interact/result','互动过程与结果'],['interact/replies','收到评论与连续回复']];
  P.render = () => window.OPS?.render();
  document.addEventListener('click',e=>{const b=e.target.closest('[data-action]');if(b&&!b.disabled){try{P.callbacks.get(b.dataset.action)?.(e);}catch(err){P.toast('演示操作未完成：'+err.message,'danger');console.error(err);}}if(e.target.closest('[data-scene-jump]'))P.ui.review=false;});
  document.addEventListener('input',e=>{const el=e.target;if(!el.dataset.bind)return;const path=el.dataset.bind.split('.');let o=P;for(const key of path.slice(0,-1))o=o[key];o[path.at(-1)]=el.type==='checkbox'?el.checked:el.type==='number'?Number(el.value):el.value;});
  document.addEventListener('change',e=>{
    const el=e.target;
    if(el.dataset.bind){const path=el.dataset.bind.split('.');let o=P;for(const key of path.slice(0,-1))o=o[key];o[path.at(-1)]=el.type==='checkbox'?el.checked:el.type==='number'?Number(el.value):el.value;P.onFieldChange?.(el.dataset.bind);if(el.dataset.redraw){if(P.dialogs.length)P.dialogs.at(-1).paint();else P.render();}}
    if(el.dataset.rowSelect){el.checked?P.ui.selected.add(+el.dataset.rowSelect):P.ui.selected.delete(+el.dataset.rowSelect);P.render();}
    if(el.dataset.allSelect!==undefined){P.ui.selected.clear();if(el.checked)el.dataset.allSelect.split(',').filter(Boolean).forEach(id=>P.ui.selected.add(+id));P.render();}
    if(el.hasAttribute('data-page-size')){P.ui.size=+el.value;P.ui.page=1;P.ui.selected.clear();P.render();}
  });
  document.addEventListener('submit',e=>{if(!e.target.matches('[data-filter-form]'))return;e.preventDefault();const f=new FormData(e.target),old=P.readFilter();P.ui.filters[P.ui.route]={...old,q:f.get('q')||'',platform:f.get('platform')||'',mode:f.get('mode')||''};P.ui.page=1;P.ui.selected.clear();P.render();});
})();
