/* 内容生产共用组件：排序、模板说明、固定参数和状态汇总。 */
(() => {
  const C=CP,O=OPS,E=C.e;
  C.db.strategies.forEach(s=>{s.versions=(s.versions||[]).map(v=>C.prod.newVersion({...v,guideUrl:v.guideUrl??s.outro??''}));});
  C.fullTime=()=>new Date().toLocaleString('sv-SE',{timeZone:'Asia/Shanghai'}).slice(0,16);
  C.access=x=>!!x&&(O.role()!=='运营'||(x.owner||x.operator)===O.actor);
  C.canManage=()=>O.role()!=='运营';
  C.guard=x=>{if(C.access(x))return true;C.toast('该内容不在当前可查看范围内');return false;};
  // These declarations describe the demo templates only, not runtime permissions.
  C.prod.editable={'视频截取':['start','end'],'引导视频插入':['insertSeconds','maxDuration'],'引导画中画':['insertSeconds','maxDuration']};
  C.help=(label,text)=>`<button type="button" class="production-help" data-help="${E(text)}" aria-label="${E(label)}说明">?</button>`;
  C.enhanceHelp=root=>root.querySelectorAll('[data-help]').forEach(button=>{
    if(button.dataset.helpReady)return;button.dataset.helpReady='1';
    let tip;const hide=()=>{tip?.remove();tip=null;button.removeAttribute('aria-describedby');};
    const show=()=>{if(tip)return;tip=document.createElement('div');tip.className='production-tooltip';tip.id='production-help-'+(++C.serial);tip.setAttribute('role','tooltip');tip.setAttribute('popover','manual');tip.textContent=button.dataset.help;(button.closest('dialog')||document.body).append(tip);button.setAttribute('aria-describedby',tip.id);tip.showPopover();const r=button.getBoundingClientRect(),t=tip.getBoundingClientRect();tip.style.left=Math.max(12,Math.min(r.left,innerWidth-t.width-12))+'px';tip.style.top=(r.bottom+t.height+12<innerHeight?r.bottom+8:Math.max(12,r.top-t.height-8))+'px';};
    button.addEventListener('mouseenter',show);button.addEventListener('focus',show);button.addEventListener('mouseleave',hide);button.addEventListener('blur',hide);button.addEventListener('click',show);button.addEventListener('keydown',e=>{if(e.key==='Escape'){e.preventDefault();e.stopPropagation();hide();}});
  });
  C.prod.dateRange=(label,from='',to='',keys=['from','to'])=>{const id='production-range-'+(++C.serial);return `<div class="ex-field lib-range ex-filter-range"><label id="${id}">${E(label)}</label><div data-library-range="${id}" aria-labelledby="${id}"><input type="hidden" data-field="${keys[0]}" value="${E(from)}"><input type="hidden" data-field="${keys[1]}" value="${E(to)}"><div data-library-range-host></div></div></div>`;};
  C.prod.advanced=(body,count=0,open=false)=>`<details class="ex-filter-advanced" ${open?'open':''}><summary>高级筛选${count?`<span class="ex-filter-applied">已生效 ${count} 项</span>`:''}</summary><div class="ex-filter-extra">${body}</div></details>`;
  C.prod.filterCount=(f,keys)=>keys.reduce((n,k)=>n+(Array.isArray(k)?k.some(key=>!!f[key]):!!f[k]),0);
  C.prod.pager=(total,page,size,go,resize)=>{const max=Math.max(1,Math.ceil(total/size));return `<div class="ex-pagination ex-pagination-compact"><span>共 ${total} 条</span><div class="ex-actions">${resize?`<select data-production-size="${C.action(resize)}" aria-label="每页条数">${[8,20,50].map(n=>`<option value="${n}" ${size===n?'selected':''}>${n} 条 / 页</option>`).join('')}</select>`:'<span>每页 '+size+' 条</span>'}${C.btn('‹',()=>go(page-1),'',page<=1,'aria-label="上一页"')}<span>${page} / ${max}</span>${C.btn('›',()=>go(page+1),'',page>=max,'aria-label="下一页"')}</div></div>`;};
  document.addEventListener('change',event=>{if(event.target.matches('[data-production-size]'))C.handlers.get(event.target.dataset.productionSize)?.(+event.target.value);});
  C.pagination=total=>C.prod.pager(total,C.ui.page,C.ui.size,page=>{C.ui.page=page;C.render();},size=>{C.ui.size=size;C.ui.page=1;C.ui.selected.clear();C.render();});
  C.prod.orderActions=(items,index,{copy=false,remove=true,scope=()=>true}={})=>{
    const d=()=>C.dialogs.at(-1),move=delta=>{const target=index+delta;if(!items[target]||!scope(items[target]))return;[items[index],items[target]]=[items[target],items[index]];d().dirty=true;d().paint();};
    return `<div class="ops-actions ops-order-actions"><button type="button" class="ex-text-button" draggable="true" data-order-handle aria-label="拖动排序">⠿ 拖动排序</button>`+C.btn('上移',()=>move(-1),'text',!items[index-1]||!scope(items[index-1]))+C.btn('下移',()=>move(1),'text',!items[index+1]||!scope(items[index+1]))+(copy?C.btn('复制本组',()=>{items.splice(index+1,0,C.prod.newVersion({...items[index],name:items[index].name+' · 副本'}));d().dirty=true;d().paint();},'text'):'')+(remove?C.btn('移除',()=>{items.splice(index,1);d().dirty=true;d().paint();},'text',items.length===1):'')+'</div>';
  };
  C.prod.wireOrder=(root,items,selector,indexKey,allow=()=>true)=>{
    let source=null;
    root.querySelectorAll(selector).forEach(row=>{
      row.querySelector('[data-order-handle]')?.addEventListener('dragstart',e=>{source=+row.dataset[indexKey];e.dataTransfer.setData('text/plain',String(source));});
      row.addEventListener('dragover',e=>{if(source!==null&&allow(items[source],items[+row.dataset[indexKey]]))e.preventDefault();});
      row.addEventListener('drop',e=>{e.preventDefault();const target=+row.dataset[indexKey];if(source===null||source===target||!allow(items[source],items[target]))return;items.splice(target,0,items.splice(source,1)[0]);source=null;root.dirty=true;root.paint();});
      row.addEventListener('dragend',()=>source=null);
    });
  };
  C.templateDetail=(template,version='v1')=>C.open({title:'合成模板说明',kind:'ex-drawer complex',body:()=>C.facts([['模板',E(template)],['版本',E(version||'未记录')],['定义来源','离线演示模板，不代表服务端 schema']])+C.section('制作过程',template==='视频截取'?'<p>源素材 → 按起止时间截取 → 输出成片。</p>':template==='引导画中画'?'<p>源素材 → 从指定位置叠加引导视频 → 按时长约束输出成片。</p>':'<p>源素材 → 在指定秒数插入引导视频 → 按时长约束输出成片。</p>')+C.prod.table(['参数','类型／含义','任务中可调整'],(C.prod.schemas[template]||[]).map(([k,l,t])=>`<tr><td>${E(l)}<small>${E(k)}</small></td><td>${t==='number'?E(C.prod.parameterHint(k)||'非负整数，单位秒'):t==='url'?'视频访问地址':'分:秒'} </td><td>${(C.prod.editable[template]||[]).includes(k)?'可调整':'固定'}</td></tr>`).join(''))+'<p class="ops-note">策略定义如何制作及默认值；选材规则与执行时间由策略配置。平台标签不增加成片份数。执行不包含发布。</p>'});
  C.prod.selectionTable=details=>'<div class="production-selection-results">'+C.prod.table(['素材','选材结果','依据'],details.map(x=>`<tr><td>${E(x.title||C.get('materials',x.id)?.title||x.id)}</td><td>${C.tag(x.selected?'选入':'跳过',x.selected?'success':'')}</td><td>${E(x.reason||'未记录')}</td></tr>`).join(''))+'</div>';
  C.prod.progress=(tasks,compact=false)=>{
    const n=ProductionModel.counts(tasks),states=[['success','成功'],['failed','失败'],['cancelled','已取消'],['running','处理中'],['pending','排队中']];
    const track=`<div class="production-progress-track" role="progressbar" aria-valuemin="0" aria-valuemax="${n.total||1}" aria-valuenow="${n.ended}" aria-label="已结束 ${n.ended} / ${n.total} 条任务，成功 ${n.success}，失败 ${n.failed}，已取消 ${n.cancelled}，处理中 ${n.running}，排队 ${n.pending}">${states.filter(([key])=>n[key]&&['success','failed','cancelled'].includes(key)).map(([key])=>`<span class="${key}" style="width:${n[key]/n.total*100}%"></span>`).join('')}</div>`;
    if(compact)return track;
    return `<div class="production-progress-summary"><div class="production-progress-heading"><strong>已结束 ${n.ended}<span> / ${n.total} 条任务</span></strong><span>${n.active?'还有 '+n.active+' 条待完成':n.total?'本批任务已结束':'暂无任务'}</span></div>${track}<div class="production-progress-counts">${states.map(([key,label])=>`<div class="${key}"><span>${label}</span><strong>${n[key]}</strong></div>`).join('')}</div></div>`;
  };
  C.prod.executionHistory=task=>{const groups=ProductionModel.executionHistory(task);return `<div class="production-history-head"><div><span class="ops-muted">当前状态</span><h3>第 ${groups[0].attempt} 次执行</h3></div>${C.status(task.status)}</div>${task.status==='failed'?`<p class="ops-error">${E(task.error||'失败原因未记录')}</p>`:''}<div class="production-history">${groups.map(group=>`<details class="production-attempt" ${group.current?'open':''}><summary><span>第 ${group.attempt} 次执行${group.current?' · 最近一次':''}</span><span>${group.events.length} 条记录</span></summary>${group.events.length?`<ol class="production-event-list">${group.events.map(event=>`<li><span class="production-event-dot"></span><div><strong>${E(event.text)}</strong><time>${E(event.time||'时间未记录')}</time></div></li>`).join('')}</ol>`:'<p class="production-history-empty">该次执行没有步骤记录</p>'}</details>`).join('')}</div>`;};
  const oldActorRender=O.render;
  let previous=O.actor;
  O.render=()=>{if(previous!==O.actor){O.closeAll();C.ui.selected.clear();C.ui.page=1;C.ui.filters={};C.ui.filterDrafts={};C.ui.filterOpen={};previous=O.actor;}return oldActorRender();};
})();
