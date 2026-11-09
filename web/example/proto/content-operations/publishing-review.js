/* 逐页目录只组织评审，画板直接复用完整初版。 */
(()=>{'use strict';
const $=s=>document.querySelector(s),E=v=>String(v??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const scenes=[
['batches','01 日常发布','发布列表','按本次发布找进度，不需要运营手动建立批次。状态和操作分列。'],
['tasks','01 日常发布','任务明细','六个互斥状态；自动核验仍属执行中。可跨批次找具体成片。'],
['plans','01 日常发布','发布计划列表','保留初版的独立计划入口、平台规则、实际账号和任务数量，避免丢失已有能力。'],
['drafts','01 日常发布','发布草稿','草稿不授权执行、不占用内容；删除草稿与取消有效任务分开。'],
['create','02 新建发布','内容与安排','单平台、多账号、多成片。与成片勾选去发布共用；本页样例已带入成片。'],
['content','02 新建发布','选择发布成片','资源选择使用 960px Dialog；保留已选内容，不可用项解释原因。'],
['accounts','02 新建发布','选择发布账号','只选当前平台的可用账号；切换平台不静默代选账号。'],
['confirm','02 新建发布','逐条检查与确认','内容默认轮流分配账号；单账号可多条。确认即按安排执行。'],
['plan1','03 按计划编排','内容与目标平台','保留三步计划入口。当前建议与直接发布共用执行范围，非新增三层管理。'],
['plan2','03 按计划编排','分平台规则','平台独立配置账号、每日条数和时段；不强制跨天，也不绑定定时产出。'],
['plan3','03 按计划编排','全部安排预览','确认前逐条检查安排；人工修改不被重新分配静默覆盖。'],
['progress','04 执行与协同','发布进度','整体检查、平台进度、任务分布；正常检查自动继续，异常才突出。'],
['plan-progress','04 执行与协同','多平台计划进度','平台各自检查和推进；局部问题不阻断其他平台。汇总数字取实际任务，不平均平台百分比。'],
['windows','04 执行与协同','5 窗口统一微调','当前窗口集合保持稳定；明确取得结果后，由运营准备下一组。非每组另建批次。'],
['window','04 执行与协同','单个窗口核对','平台窗口为视觉示意。原型不证明已打开真实浏览器或已具备最终提交能力。'],
['login','04 执行与协同','账号验证与恢复','登录问题只影响该账号。完成验证后重新检查，不从头重发。'],
['verify','04 执行与协同','提交结果待核对','结果未明时保留现场，不标失败，不开放重发。'],
['detail','05 结果与边界','任务详情与过程','阶段、通道和原因在详情；发布后互动结果独立展示。'],
['result','05 结果与边界','部分失败的收尾','结束不等于全部成功；失败、取消、未决数量均可解释。'],
['cancel','05 结果与边界','取消未执行任务','明确范围与条数，保留已发布结果和执行中任务。'],
['empty','05 结果与边界','空态与首次发布','给出清晰行动入口；不展示无意义的零值统计。'],
['unavailable','05 结果与边界','执行条件不足','文件不可访问、设备离线等硬条件不能通过人工确认跳过。']
];
let key=location.hash.slice(1)||'batches',notes={};if(!scenes.some(s=>s[0]===key))key='batches';try{notes=JSON.parse(localStorage.getItem('wt-publishing-visual-review-notes')||'{}');}catch{}
$('#root').innerHTML=`<div class="book"><aside class="catalog"><h1>发布管理</h1><p>逐页视觉与交互评审<br>基于完整初版 · 待人工评审</p>${[...new Set(scenes.map(s=>s[1]))].map(g=>`<h2>${g}</h2>${scenes.filter(s=>s[1]===g).map(s=>`<a class="scene-link" href="#${s[0]}"><span>${String(scenes.indexOf(s)+1).padStart(2,'0')}</span>${s[2]}</a>`).join('')}`).join('')}<h2>参考</h2><a href="../../common/index.html" target="_blank">common 基础规范 ↗</a><br><a href="design/publishing.md">发布设计工作稿 ↗</a></aside><main class="book-main"><header class="book-head"><div><h1 id="scene-title"></h1><p>完整工作台 · 公共基础组件 · ${scenes.length} 个评审场景</p></div><div class="ex-actions"><button class="ex-button" id="focus-toggle">收起目录</button><button class="ex-button" id="prev">上一页</button><button class="ex-button" id="next">下一页</button><a class="ex-button" id="standalone" target="_blank">独立预览 ↗</a></div></header><div class="canvas-wrap"><iframe title="发布管理完整视觉画板" src="publishing-review-canvas.html?scene=${key}#publish/tasks"></iframe></div><section class="review-note"><strong>本页评审重点</strong><p id="scene-caption"></p><label for="review-comment">本页意见（自动保存在本机）</label><textarea id="review-comment" placeholder="填写本页需要调整的内容"></textarea><div class="review-bottom"><span id="save-state">未填写不代表确认通过</span><div class="ex-actions"><button class="ex-button" id="export">导出意见</button><a href="index.html#publish/tasks" target="_blank">原始工作台 ↗</a></div></div></section></main></div>`;
function book(){const s=scenes.find(s=>s[0]===key)||scenes[0];key=s[0];$('#scene-title').textContent=s[2];$('#scene-caption').textContent=s[3];$('#review-comment').value=notes[key]||'';$('#save-state').textContent=notes[key]?'已保存本页意见':'未填写不代表确认通过';$('#standalone').href='publishing-review-canvas.html?scene='+key+'#publish/tasks';document.querySelectorAll('.scene-link').forEach(a=>{a.removeAttribute('aria-current');if(a.hash==='#'+key)a.setAttribute('aria-current','page');});document.title=s[2]+' · 发布管理视觉评审';const i=scenes.indexOf(s);$('#prev').disabled=i===0;$('#next').disabled=i===scenes.length-1;}
$('#prev').onclick=()=>location.hash=scenes[scenes.findIndex(s=>s[0]===key)-1][0];$('#next').onclick=()=>location.hash=scenes[scenes.findIndex(s=>s[0]===key)+1][0];$('#focus-toggle').onclick=()=>{document.body.classList.toggle('focused');$('#focus-toggle').textContent=document.body.classList.contains('focused')?'展开目录':'收起目录';};
$('#review-comment').addEventListener('input',e=>{notes[key]=e.target.value;try{localStorage.setItem('wt-publishing-visual-review-notes',JSON.stringify(notes));$('#save-state').textContent='本页意见已保存';}catch{$('#save-state').textContent='保存受限，请导出意见';}});
$('#export').onclick=()=>{const u=URL.createObjectURL(new Blob([JSON.stringify({module:'publishing',notes},null,2)],{type:'application/json'})),a=document.createElement('a');a.href=u;a.download='发布管理逐页评审.json';a.click();setTimeout(()=>URL.revokeObjectURL(u),1000);};
addEventListener('hashchange',()=>{key=location.hash.slice(1)||'batches';book();$('iframe').contentWindow.postMessage({type:'publishing-navigate',key},'*');});addEventListener('message',e=>{if(e.source!==$('iframe').contentWindow||e.data?.type!=='publishing-scene'||!scenes.some(s=>s[0]===e.data.key))return;key=e.data.key;history.replaceState(null,'','#'+key);book();});book();
})();
