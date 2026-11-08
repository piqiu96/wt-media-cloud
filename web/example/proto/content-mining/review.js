'use strict';
const reviewScenes=window.MINING_SCENARIOS;
let activeId=new URLSearchParams(location.search).get('scene')||'P01';
if(!reviewScenes.some(s=>s.id===activeId))activeId='P01';
const frame=document.querySelector('#product-frame');
const controls={P07:[['query-retry','重试失败目标']],P08:[['import-failure','查看入池失败']],P11:[['review-again','继续审核跳过项']],P12:[['action-success','模拟本次重试成功']],P14:[['batch-processing','查看处理中'],['batch-success','查看全部成功'],['batch-partial','查看部分失败']],P16:[['empty','首次无资源'],['no-match','筛选无结果'],['load-error','首次读取失败']],S04:[['schedule-manual','手动执行'],['schedule-daily','每天固定时间'],['schedule-interval','按分钟间隔']],S07:[['save-failure','模拟保存失败'],['save-success','允许下次保存成功']],T02:[['task-pending','待执行'],['task-running','执行中']],T04:[['task-failed','整体失败'],['task-partial','部分成功']],T05:[['retry-failure','模拟受理失败'],['retry-success','允许下次重试受理']],T07:[['snapshot-retry','恢复当次快照']],G02:[['role-operator','普通运营'],['role-senior','高级运营'],['role-admin','管理员']],G03:[['narrow','560px 产品容器'],['wide','恢复自适应']],G04:[['refresh-success','恢复列表读取']]};
function send(action,payload={}){frame.contentWindow?.postMessage({kind:'mining-review',action,...payload},'*')}
function paintBook(){
 const s=reviewScenes.find(s=>s.id===activeId);
 document.querySelector('#scene-number').textContent=s.id;document.querySelector('#scene-title').textContent=s.title;
 document.querySelector('#scene-position').textContent=`${reviewScenes.indexOf(s)+1} / ${reviewScenes.length}`;
 document.querySelector('#scene-caption').textContent=s.caption;document.querySelector('#scene-review').textContent=s.review;document.querySelector('#scene-check').textContent=s.check;
 document.querySelectorAll('[data-scene-id]').forEach(b=>b.setAttribute('aria-current',b.dataset.sceneId===s.id?'page':'false'));
 document.querySelector('#demo-controls').innerHTML=(controls[s.id]||[]).map(([a,t])=>`<button class="ex-button" data-control="${a}">${t}</button>`).join('');
 document.title=`${s.id} ${s.title} · 内容挖掘设计评审`;
}
function panel(name,open){const el=document.querySelector(name==='catalog'?'#catalog':'#review-notes');el.hidden=open===undefined?!el.hidden:!open;document.querySelector(`[data-book="${name}"]`).setAttribute('aria-expanded',String(!el.hidden));}
function choose(id){activeId=id;history.replaceState(null,'',`?scene=${id}`);paintBook();frame.style.width='100%';frame.style.minWidth='';send('scene',{id});panel('catalog',false);panel('notes',false);}
document.querySelector('#scene-nav').innerHTML=[...new Set(reviewScenes.map(s=>s.group))].map(g=>`<div class="group-label">${g}</div>${reviewScenes.filter(s=>s.group===g).map(s=>`<button class="catalog-item" data-scene-id="${s.id}"><span>${s.id}</span>${s.title}</button>`).join('')}`).join('');
document.addEventListener('click',e=>{
 const item=e.target.closest('[data-scene-id]');if(item)choose(item.dataset.sceneId);
 const ctl=e.target.closest('[data-control]');if(ctl){if(ctl.dataset.control==='narrow'){frame.style.width='560px';frame.style.minWidth='560px'}else if(ctl.dataset.control==='wide'){frame.style.width='100%';frame.style.minWidth=''}else send('control',{control:ctl.dataset.control});panel('notes',false)}
 const b=e.target.closest('[data-book]');if(!b)return;const action=b.dataset.book;
 if(action==='catalog'||action==='notes'){panel(action);return}
 if(action==='focus'){document.body.classList.toggle('focus');document.querySelector('#return-review').hidden=!document.body.classList.contains('focus');panel('catalog',false);panel('notes',false);return}
 const i=reviewScenes.findIndex(s=>s.id===activeId);choose(reviewScenes[(i+(action==='next'?1:-1)+reviewScenes.length)%reviewScenes.length].id);
});
document.addEventListener('keydown',e=>{if(e.key==='Escape'){panel('catalog',false);panel('notes',false)}});
window.addEventListener('message',e=>{if(e.source!==frame.contentWindow)return;if(e.data?.kind==='mining-ready')send('scene',{id:activeId});if(e.data?.kind==='mining-batch-processing')panel('notes',true)});
frame.src=`product.html?scene=${activeId}`;paintBook();
