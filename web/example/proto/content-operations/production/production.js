/* 内容生产交互原型：只操作本地虚构数据，不调用业务API。 */
(() => {
  'use strict';
  const C = window.CP = {};
  const KEY = 'wt-operations-production-v3';
  // 读取时兼容旧平台名称，保留已有演示数据及平台分配关系。
  try { C.db = JSON.parse(localStorage.getItem(KEY)?.replaceAll('\u767e\u5bb6\u53f7','百度') || 'null'); } catch {}
  if (C.db?.version !== 1) C.db = CPSeed();
  C.ui = {page:1,size:8,selected:new Set(),filters:{},filterDrafts:{},filterOpen:{},group:false,collapsed:new Set()};
  C.handlers = new Map(); C.serial = 0; C.dialogs = [];
  C.$ = (s,r=document) => r.querySelector(s);
  C.$$ = (s,r=document) => [...r.querySelectorAll(s)];
  C.e = v => String(v??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  C.icon = n => PROTO_ICONS[n] || PROTO_ICONS.Browse;
  C.get = (collection,id) => C.db[collection].find(x=>x.id===Number(id));
  C.save = () => {try {localStorage.setItem(KEY,JSON.stringify(C.db));} catch {}};
  C.id = () => ++C.db.seq;
  C.action = fn => { const id='c'+String(++C.serial); C.handlers.set(id,fn); return id; };
  C.btn = (label,fn,kind='',disabled=false,attrs='') => `<button type="button" class="${kind.includes('text')?'ex-text-button':'ex-button'} ${kind.replace('text','')}" data-action="${C.action(fn)}" ${disabled?'disabled':''} ${attrs}>${label}</button>`;
  C.row = (label,fn,primary=false) => C.btn(label,fn,'text'+(primary?' ex-row-primary':''));
  C.link = (label,fn) => `<button type="button" class="cp-link" data-action="${C.action(fn)}">${C.e(label)}</button>`;
  C.tag = (s,tone='') => `<span class="ex-tag ${tone}">${C.e(s)}</span>`;
  C.status = s => { const [label,tone]=({pending:['排队中',''],running:['运行中','info'],success:['已完成','success'],failed:['失败','danger'],cancelled:['已取消','']})[s]||[s,'']; return C.tag(label,tone); };
  C.head = (title,desc,actions='') => `<div class="ex-page-head"><div><h1>${title}</h1><p>${desc}</p></div><div class="cp-actions">${actions}</div></div>`;
  C.field = (label,name,value='',options={}) => {
    const id='cp-field-'+(++C.serial),bind=`data-field="${C.e(name)}" ${options.disabled?'disabled':''}`;
    const input=options.options?`<select id="${id}" ${bind}>${options.options.map(x=>{const [v,t]=Array.isArray(x)?x:[x,x];return `<option value="${C.e(v)}" ${String(value)===String(v)?'selected':''}>${C.e(t)}</option>`;}).join('')}</select>`:options.area?`<textarea id="${id}" ${bind} placeholder="${C.e(options.placeholder||'')}">${C.e(value)}</textarea>`:`<input id="${id}" ${bind} type="${options.type||'text'}" value="${C.e(value)}" placeholder="${C.e(options.placeholder||'')}" ${options.required?'required':''} ${options.type==='number'?'min="0"':''}>`;
    return `<div class="ex-field ${options.full?'full':''} ${options.required?'required':''} ${options.className||''}"><div class="production-field-label"><label for="${id}">${label}</label>${options.hint?C.help(label,options.hint):''}${options.action?C.btn(options.action.label,options.action.run,'text production-field-action'):''}</div>${input}${options.help?`<span class="ex-help">${options.help}</span>`:''}</div>`;
  };
  C.section = (title,body) => `<section class="ex-section"><h3 class="ex-section-title detail">${title}</h3>${body}</section>`;
  C.facts = rows => `<div class="ex-description-container"><dl class="ex-description-list" data-density="dense">${rows.map(([k,v,full])=>`<div class="ex-desc-item ${full?'full':''}"><dt>${k}</dt><dd>${v}</dd></div>`).join('')}</dl></div>`;
  C.coverPlaceholder = (extra='') => `<span class="cp-cover cp-cover-default ${extra}" role="img" aria-label="默认视频封面"><svg viewBox="0 0 32 32" fill="none" aria-hidden="true"><rect x="4" y="6" width="24" height="20" rx="3" stroke="currentColor" stroke-width="1.5"/><path d="m13 11 8 5-8 5V11Z" fill="currentColor"/></svg></span>`;
  C.media = (title,sub='') => `<div class="cp-media">${C.coverPlaceholder()}<div class="cp-name"><strong>${C.e(title)}</strong><small>${C.e(sub)}</small></div></div>`;
  C.empty = text => `<div class="ex-empty"><h3>${text}</h3><p>调整筛选条件后重试。</p></div>`;
  C.toast = s => {const e=C.$('#production-toast');e.textContent=s;clearTimeout(C.timer);C.timer=setTimeout(()=>e.textContent='',4500);};
  C.nav = r => {if(location.hash.slice(1)!==r)location.hash=r;else C.render();};
  C.enhance = root => {
    root.querySelectorAll('input[type="checkbox"]').forEach(input=>input.classList.add('ex-check'));
    root.querySelectorAll('[data-all]').forEach(input=>{const ids=input.dataset.all.split(',').filter(Boolean).map(Number);input.indeterminate=ids.some(id=>C.ui.selected.has(id))&&!ids.every(id=>C.ui.selected.has(id));});
    root.querySelectorAll('td:last-child').forEach(td=>{if([...td.children].some(x=>x.matches('.ex-text-button'))&&!td.querySelector(':scope > .ex-row-actions')){const row=document.createElement('div');row.className='ex-row-actions';row.append(...td.childNodes);td.append(row);}});
    WTControls.enhance(root);
    window.LibraryDateRange?.mount(root);C.enhanceHelp?.(root);
  };
  C.open = opts => {
    const d=document.createElement('dialog'),trigger=document.activeElement;
    d.className='ex-overlay '+(opts.kind||'')+(opts.form?' production-form':'');d.setAttribute('aria-label',opts.title);d.dirty=false;d.discardMessage=opts.discardMessage;
    d.paint=()=>{
      // Rebuilding the body must not reset the user's position in a long form.
      const previous=d.querySelector(':scope > .ex-overlay-body'),top=previous?.scrollTop||0,left=previous?.scrollLeft||0;
      WTControls.dismiss();const footer=opts.footer?.();d.innerHTML=`<header class="ex-overlay-header"><h2>${C.e(opts.title)}</h2>${C.btn('×',()=>C.close(d),'ex-close',false,`aria-label="关闭${C.e(opts.title)}"`)}</header><div class="ex-overlay-body cp-editor">${opts.body()}</div>${footer?`<footer class="ex-overlay-footer">${footer}</footer>`:''}`;C.enhance(d);opts.wire?.(d);
      const body=d.querySelector(':scope > .ex-overlay-body');body.scrollTop=top;body.scrollLeft=left;
    };
    d.addEventListener('cancel',e=>{e.preventDefault();C.close(d);});
    const change=e=>{if(opts.form)d.dirty=true;const n=e.target.dataset.field;if(n)opts.change?.(n,e.target.value,e.type,d);};
    d.addEventListener('input',change);d.addEventListener('change',change);
    d.addEventListener('close',()=>{C.dialogs=C.dialogs.filter(x=>x!==d);d.remove();C.dialogs.forEach((x,i)=>x.classList.toggle('ex-underlay',i<C.dialogs.length-1));document.body.style.overflow=C.dialogs.length?'hidden':'';if(trigger?.isConnected)trigger.focus();});
    C.dialogs.push(d);C.dialogs.forEach((x,i)=>x.classList.toggle('ex-underlay',i<C.dialogs.length-1));document.body.append(d);document.body.style.overflow='hidden';d.paint();d.showModal();return d;
  };
  C.close = (d=C.dialogs.at(-1),force=false) => {
    if(!d)return;
    if(d.dirty&&!force){let confirm;confirm=C.open({title:'放弃未保存的修改？',body:()=>'<p>'+C.e(d.discardMessage||'本次输入还没有保存。')+'</p>',footer:()=>C.btn('继续编辑',()=>C.close(confirm,true))+C.btn('放弃修改',()=>{C.close(confirm,true);C.close(d,true);},'primary')});return;}
    WTControls.dismiss();d.close();d.remove();C.dialogs=C.dialogs.filter(x=>x!==d);
    C.dialogs.forEach((x,i)=>x.classList.toggle('ex-underlay',i<C.dialogs.length-1));
    document.body.style.overflow=C.dialogs.length?'hidden':'';
  };
  C.read = () => C.ui.filters[C.route] ||= {q:'',strategy:'',status:'',platform:'',operator:'',trigger:'',download:''};
  C.select = (name,label,value,values) => C.field(label,name,value,{options:[['','全部'],...values],className:'ex-filter-select'});
  C.filter = type => {
    const f=C.read();
    return `<form class="ex-filters" data-list-filter>${C.field('综合搜索','q',f.q,{placeholder:type==='strategies'?'策略名称':'名称 / 素材 / ID',className:'ex-filter-search'})}${type!=='strategies'?C.select('strategy','合成策略',f.strategy,C.db.strategies.map(s=>[s.id,s.name])):C.select('status','状态',f.status,[['enabled','启用'],['disabled','停用']])}${type==='tasks'?C.select('status','任务状态',f.status,[['pending','排队中'],['running','运行中'],['success','已完成'],['failed','失败'],['cancelled','已取消']]):''}${type==='outputs'?C.select('platform','预备用途',f.platform,C.db.platforms):''}${C.select('operator','负责人',f.operator,['陈小雨','林予安'])}<button type="submit" class="ex-button primary">查询</button>${C.btn('重置',()=>{C.ui.filters[C.route]=null;C.ui.page=1;C.ui.selected.clear();C.render();})}</form>`;
  };
  C.filtered = (rows,type) => {
    const f=C.read();return rows.filter(x=>{
      const m=C.get('materials',x.materialId),s=C.get('strategies',x.strategyId||C.get('tasks',x.taskId)?.strategyId);
      return (!f.q||`${x.id} ${x.name} ${x.batch||''} ${m?.title||''}`.includes(f.q))&&(!f.strategy||s?.id===Number(f.strategy))&&(!f.status||(type==='strategies'?(x.enabled?'enabled':'disabled'):x.status)===f.status)&&(!f.platform||x.platform===f.platform)&&(!f.operator||(x.owner||s?.operator||x.operator)===f.operator);
    });
  };
  C.paged = rows => {const max=Math.max(1,Math.ceil(rows.length/C.ui.size));C.ui.page=Math.min(C.ui.page,max);return rows.slice((C.ui.page-1)*C.ui.size,C.ui.page*C.ui.size);};
  C.pagination = total => {const max=Math.max(1,Math.ceil(total/C.ui.size));return `<div class="ex-pagination"><span>共 ${total} 条</span><div class="cp-actions"><label for="cp-size" class="cp-muted">每页</label><select id="cp-size" data-size aria-label="每页条数">${[8,20,50].map(v=>`<option ${v===C.ui.size?'selected':''}>${v}</option>`).join('')}</select>${C.btn('‹',()=>{C.ui.page--;C.render();},'',C.ui.page<=1,'aria-label="上一页"')}<span>${C.ui.page} / ${max}</span>${C.btn('›',()=>{C.ui.page++;C.render();},'',C.ui.page>=max,'aria-label="下一页"')}</div></div>`;};
  C.check = id => `<input type="checkbox" class="ex-check" data-select="${id}" ${C.ui.selected.has(id)?'checked':''} aria-label="选择 ${id}">`;
  C.all = ids => `<input type="checkbox" class="ex-check" data-all="${ids.join(',')}" ${ids.length&&ids.every(id=>C.ui.selected.has(id))?'checked':''} aria-label="选择当前页">`;
  C.selection = actions => C.ui.selected.size?`<div class="ex-selection" role="region" aria-label="批量操作"><span>已选择 <strong>${C.ui.selected.size}</strong> ${C.route==='outputs'?'个成片':'条任务'}</span>${actions}${C.btn('取消选择',()=>{C.ui.selected.clear();C.render();},'text')}</div>`:'';
  C.render = () => window.OPS?.render();
  document.addEventListener('click',e=>{const b=e.target.closest('[data-action]');if(b&&!b.disabled)C.handlers.get(b.dataset.action)?.();});
  document.addEventListener('change',e=>{
    if(e.target.matches('[data-select]')){const id=Number(e.target.dataset.select);e.target.checked?C.ui.selected.add(id):C.ui.selected.delete(id);C.render();}
    if(e.target.matches('[data-all]')){e.target.dataset.all.split(',').filter(Boolean).map(Number).forEach(id=>e.target.checked?C.ui.selected.add(id):C.ui.selected.delete(id));C.render();}
    if(e.target.matches('[data-size]')){C.ui.size=Number(e.target.value);C.ui.page=1;C.ui.selected.clear();C.render();}
  });
  document.addEventListener('input',e=>{if(e.target.closest('[data-list-filter]')&&e.target.dataset.field)(C.ui.filterDrafts[C.route]||={...C.read()})[e.target.dataset.field]=e.target.value;});
  document.addEventListener('change',e=>{if(e.target.closest('[data-list-filter]')&&e.target.dataset.field)(C.ui.filterDrafts[C.route]||={...C.read()})[e.target.dataset.field]=e.target.value;});
  document.addEventListener('toggle',e=>{if(e.target.matches('[data-list-filter] .ex-filter-advanced'))C.ui.filterOpen[C.route]=e.target.open;},true);
  document.addEventListener('submit',e=>{if(!e.target.matches('[data-list-filter]'))return;e.preventDefault();const from=e.target.querySelector('[data-field="from"]')?.value,to=e.target.querySelector('[data-field="to"]')?.value;if(from&&to&&from>to){C.toast('起始日期不能晚于结束日期');return;}C.ui.filters[C.route]=Object.fromEntries([...e.target.querySelectorAll('[data-field]')].map(x=>[x.dataset.field,x.value]));C.ui.filterDrafts[C.route]={...C.read()};C.ui.page=1;C.ui.selected.clear();C.render();});
})();
