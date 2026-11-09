/* 评审提案专用：内存样例，不读取或写入工作台数据。 */
(() => {
  const $ = id => document.getElementById(id);
  const esc = x => String(x ?? '').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  const button=(label,action,primary=false,disabled=false)=>`<button type="button" class="ex-button${primary?' primary':''}" data-action="${action}" ${disabled?'disabled':''}>${label}</button>`;
  const tag=(label,kind='')=>`<span class="ex-tag ${kind}">${label}</span>`;
  const initialMaterials = [
    {id:101,title:'剪辑效率提升的三个方法',author:'剪辑实验室',category:'剪辑教程',mine:true,owner:'陈小雨',date:'11-02 10:30',views:18900,likes:2800,saves:680,state:'ready',use:'未制作'},
    {id:103,title:'短视频开头怎样留住观众',author:'内容笔记',category:'剪辑教程',mine:true,owner:'陈小雨',date:'11-02 14:20',views:33542,likes:3074,saves:934,state:'moving',use:'未制作'},
    {id:107,title:'让口播更自然的实用技巧',author:'剪辑实验室',category:'剪辑教程',mine:true,owner:'陈小雨',date:'11-01 09:10',views:62826,likes:3622,saves:1442,state:'ready',use:'已制作 · 未发布'},
    {id:109,title:'三种节奏剪辑，怎么选才自然',author:'内容笔记',category:'剪辑教程',mine:false,date:'11-03 08:10',views:42600,likes:3900,saves:1080,state:'remote',use:'未使用'},
    {id:111,title:'转场之后，观众为什么还在看',author:'剪辑实验室',category:'剪辑教程',mine:false,date:'11-02 17:00',views:56200,likes:4200,saves:1280,state:'failed',use:'未使用'},
    {id:113,title:'旅行镜头如何自然衔接',author:'镜头练习生',category:'剪辑教程',mine:false,date:'11-01 12:00',views:48184,likes:3348,saves:1188,state:'expired',use:'已移出'},
    {id:115,title:'用声音连接两个画面',author:'剪辑实验室',category:'剪辑教程',mine:false,date:'11-01 15:40',views:21400,likes:1900,saves:620,state:'ready',use:'未使用'},
    {id:117,title:'教程视频的字幕排版',author:'内容笔记',category:'剪辑教程',mine:false,date:'10-31 11:10',views:36700,likes:2800,saves:910,state:'ready',use:'已发布 · B站'},
    {id:102,title:'手机拍摄也能做出电影感',author:'镜头练习生',category:'拍摄技巧',mine:true,owner:'陈小雨',date:'11-02 10:00',views:26221,likes:2937,saves:807,state:'ready',use:'未制作'},
    {id:106,title:'一盏灯拍出干净的人像',author:'镜头练习生',category:'拍摄技巧',mine:false,date:'11-01 10:00',views:40863,likes:3211,saves:1061,state:'remote',use:'未使用'}
  ];
  const strategies=[{id:'201',name:'教程通用引导',category:'剪辑教程',owner:'陈小雨',template:'引导视频插入',effect:'片头插入 5 秒引导视频，再播放源素材。',versions:[{name:'通用版',qty:1,platform:'B站 / 百度'},{name:'竖屏引导版',qty:1,platform:'抖音 / 小红书'}]},
    {id:'202',name:'教程精简剪辑',category:'剪辑教程',owner:'陈小雨',template:'视频截取',effect:'截取源素材 00:05–01:00。',versions:[{name:'精简版',qty:1,platform:'B站 / 百度'}]},
    {id:'206',name:'拍摄技巧精简版',category:'拍摄技巧',owner:'陈小雨',template:'视频截取',effect:'截取源素材 00:00–01:00。',versions:[{name:'精简版',qty:1,platform:'B站 / 抖音'}]}];
  const parameterSchemas={
    '引导视频插入':[['guideUrl','引导视频地址','url'],['insertSeconds','插入时间（主体第几秒）','number'],['guideDuration','引导时长（秒，0 为完整视频）','number'],['maxDuration','成片时长上限（秒，0 为不限）','number']],
    '视频截取':[['start','截取起点（分:秒）','text'],['end','截取终点（分:秒）','text']]
  };
  strategies.forEach(s=>s.versions.forEach(v=>Object.assign(v,s.template==='视频截取'?{start:s.id==='202'?'00:05':'00:00',end:'01:00'}:{guideUrl:'https://example.com/guide.mp4',insertSeconds:10,guideDuration:5,maxDuration:0})));
  strategies[0].effect='策略默认：在主体第 10 秒插入 5 秒引导视频；本次参数可调整。';
  let materials,form,scene,pick,dirty=false;
  const states={ready:['已转存','success'],moving:['转存中','info'],remote:['未转存',''],failed:['转存失败','danger'],expired:['来源失效','danger']};
  const available=m=>!['failed','expired'].includes(m.state);
  const mine=m=>m.mine&&m.owner===form.owner;
  const selected=()=>materials.filter(m=>form.ids.includes(m.id));
  const strategy=()=>strategies.find(s=>s.id===form.strategy);
  const candidates=()=>strategies.filter(s=>s.category===form.category&&s.owner===form.owner);
  const copies=()=>form.versions.reduce((n,v)=>n+Number(v.qty||0),0);
  const totals=()=>({all:selected().length*copies(),waiting:selected().filter(m=>m.state!=='ready').length*copies(),ready:selected().filter(m=>m.state==='ready').length*copies()});
  const say=t=>{$('design-message').textContent=t;};
  const enhance=el=>window.WTControls?.enhance(el);
  function reset(which){
    document.querySelectorAll('dialog[open]').forEach(d=>d.close());
    scene=which;materials=structuredClone(initialMaterials);dirty=false;
    form={category:which==='blank'?'':'剪辑教程',owner:which==='empty'?'林程':'陈小雨',name:which==='blank'?'':'剪辑教程 · 11月3日第一批',strategy:which==='blank'||which==='empty'?'':'201',ids:which==='blank'||which==='empty'?[]:[101,103,107],versions:which==='blank'||which==='empty'?[]:structuredClone(strategies[0].versions),error:''};
    if(which==='failure')form.error='示例：任务未创建成功。填写内容已保留，可以再次保存。';
    say('');render();document.querySelectorAll('[data-scene]').forEach(b=>b.setAttribute('aria-pressed',String(b.dataset.scene===which)));
    if(which==='picker')openPicker('library');
    if(which==='settings'){$('version-settings').open=true;$('output-naming').open=true;$('output-settings-section').scrollIntoView({block:'start'});}
  }
  function versionFields(v,i){
    return `<article class="design-version"><div class="design-version-heading"><strong>成片设置 ${i+1}</strong><span>作用于本次所选的 ${selected().length} 条素材</span></div><div class="design-grid"><div class="ex-field required"><label for="version-name-${i}">成片设置名称</label><input id="version-name-${i}" required value="${esc(v.name)}" data-version="${i}" data-param="name"><small class="ex-help">标识这一组配置，最终名称会自动带上素材标题和成片 ID。</small></div><div class="ex-field required"><label for="qty-${i}">每条素材份数</label><input id="qty-${i}" type="number" min="1" max="20" step="1" required value="${v.qty}" data-qty="${i}"></div>${parameterSchemas[strategy().template].map(([key,label,type])=>`<div class="ex-field required${type==='url'?' full':''}"><label for="param-${i}-${key}">${label}</label><input id="param-${i}-${key}" type="${type}" required ${type==='number'?'min="0" step="1"':''} ${type==='text'?'pattern="[0-9]{1,3}:[0-5][0-9]"':''} value="${esc(v[key])}" data-version="${i}" data-param="${key}"></div>`).join('')}</div></article>`;
  }
  function namingExamples(){
    const title=selected()[0]?.title||'素材标题',name=form.versions[0]?.name.trim()||'成片设置名称';
    const rows=[{owner:form.owner,id:'8731'},{owner:form.owner==='林程'?'陈小雨':'林程',id:'8732'}];
    return `<p class="design-muted">显示名称：素材标题 · 成片设置名称 · 成片 ID。下载文件名使用相同名称，并加文件扩展名。</p><div class="ex-table-scroll"><table class="ex-table"><thead><tr><th>制作人</th><th>成片 ID</th><th>最终名称示例</th></tr></thead><tbody>${rows.map(r=>`<tr><td>${r.owner}</td><td>${r.id}</td><td>${esc(title)} · ${esc(name)} · ${r.id}</td></tr>`).join('')}</tbody></table></div><p class="design-muted naming-file">下载文件名示例：${esc(title)} · ${esc(name)} · ${rows[0].id}.mp4</p><p class="design-muted">以上 ID 仅为效果示例。实际使用每条成片记录自身的完整 ID；创建任务时还没有实际成片的，待生成后带入。</p>`;
  }
  function outputSettings(s,count,t){
    if(!s)return '<div class="design-empty">选择策略后，展示成片设置及预计产出。</div>';
    return `<p class="design-muted">模板决定字段，策略提供默认值；普通运营可调整本次参数，每组设置作用于本次所选素材。</p><div class="ex-table-scroll"><table class="ex-table" data-version-summary><thead><tr><th>成片设置名称</th><th>适用平台</th><th>每条素材份数</th><th>预计任务</th></tr></thead><tbody>${form.versions.map(v=>`<tr><td>${esc(v.name)}</td><td>${v.platform}</td><td>${v.qty} 份</td><td>${v.qty*count} 条</td></tr>`).join('')}</tbody></table></div><details id="version-settings"><summary>调整本次成片设置</summary><p class="design-muted">合成模板：${s.template}。调整仅对本次任务生效，保留实际参数，不修改原策略。${s.template==='引导视频插入'?'引导地址为示例，不请求真实视频。':''}</p>${form.versions.map(versionFields).join('')}</details><details id="output-naming"><summary>查看成片名称与文件名示例</summary><div data-naming-examples>${namingExamples()}</div></details><div class="design-count"><span>${count} 条素材 × 每条 ${copies()} 份 =</span><strong>${t.all}</strong><b>条合成任务</b></div><p class="design-muted">${t.ready} 条可排队 · ${t.waiting} 条等待源视频转存。合成完成后预计得到 ${t.all} 个成片文件。</p>`;
  }
  const selectField=(label,name,value,options,disabled=false)=>`<div class="ex-field required"><label for="${name}">${label}</label><select id="${name}" data-field="${name}" ${disabled?'disabled':''}>${options.map(([v,t])=>`<option value="${esc(v)}" ${v===value?'selected':''}>${esc(t)}</option>`).join('')}</select></div>`;
  function render(){
    const s=strategy(),t=totals(),count=selected().length;
    $('design-form').innerHTML=`<header><h1>新增合成任务</h1><button class="ex-close" data-action="close" aria-label="关闭新增合成任务">×</button></header><div class="form-content">
      <section class="design-section"><h2><span>01</span>确定制作范围</h2><div class="design-grid">${selectField('类目','category',form.category,[['','请选择类目'],['剪辑教程','剪辑教程'],['拍摄技巧','拍摄技巧']])}${selectField('任务负责人','owner',form.owner,[['陈小雨','陈小雨 · 高级运营'],['林程','林程 · 运营']])}<div class="ex-field full required"><label for="task-name">任务名称</label><input id="task-name" data-field="name" value="${esc(form.name)}" placeholder="例如：剪辑教程 · 11月3日第一批"><span class="ex-help">用于识别本次创建的一批任务。</span>${form.nameError?'<span class="ex-error" role="alert">请填写任务名称。</span>':''}</div></div></section>
      <section class="design-section"><h2><span>02</span>选择合成策略</h2>${selectField('合成策略','strategy',form.strategy,[['',form.category?'请选择合成策略':'请先选择类目'],...candidates().map(s=>[s.id,s.name+' · '+s.template])],!form.category)}
      ${s?`<div class="strategy-summary"><div><strong>${s.name}</strong><p class="design-muted">${s.effect}</p></div><span class="design-muted">${form.category} · ${form.owner}负责</span></div>`:form.category&&!candidates().length?`<div class="ex-notice warning" role="status"><span>当前负责人在「${form.category}」下暂无可用策略。请检查负责人或类目；没有策略时可取消本次创建，到合成策略新增。</span></div>`:'<p class="design-muted">只展示当前类目、负责人下已启用的策略；由你选择本次使用哪一条。</p>'}</section>
      <section class="design-section"><h2><span>03</span>选择制作素材</h2><div class="design-select-line"><div><strong>${count?'已选 '+count+' 条素材':'尚未选择素材'}</strong><p>${form.category?'范围：'+form.category+' · 可从我的素材或素材库选择':'确定类目后，选择本次要制作的素材。'}</p></div>${button(count?'调整素材':'选择素材','pick',false,!form.category)}</div>
      ${count?`<div class="ex-table-scroll"><table class="ex-table"><thead><tr><th>素材 ID</th><th>素材</th><th>源视频</th><th>本次处理</th><th>操作</th></tr></thead><tbody>${selected().map(m=>`<tr><td>${m.id}</td><td>${m.title}<small>${m.author}</small></td><td>${tag(...states[m.state])}</td><td>${m.state==='ready'?'可进入合成队列':'等待转存完成'}</td><td><button class="ex-text-button" data-action="remove-${m.id}" aria-label="取消选择 ${m.title}">取消选择</button></td></tr>`).join('')}</tbody></table></div>`:`<div class="design-empty">${form.category?'选择器内只显示「'+form.category+'」素材。':'先选择类目，策略和素材范围会随之确定。'}</div>`}</section>
      <section class="design-section" id="output-settings-section"><h2><span>04</span>核对本次产出</h2>${outputSettings(s,count,t)}
      ${form.error?`<div class="ex-notice danger" role="alert">${esc(form.error)}</div>`:''}</section></div><footer><div>${t.all?`保存后创建 <strong>${t.all}</strong> 条任务并加入执行队列`:'请选择类目、策略和素材'}<small>素材转存中的任务会等待文件就绪。</small></div>${button('取消','close')}${button('保存','save',true,!form.category||!s||!count)}</footer>`;
    enhance($('design-form'));
  }
  function confirm(title,body,action,onConfirm){
    const d=$('design-confirm');d.innerHTML=`<header class="ex-overlay-header"><h2 id="confirm-heading">${title}</h2><button class="ex-close" aria-label="关闭确认">×</button></header><div class="ex-overlay-body">${body}</div><footer class="ex-overlay-footer">${button('继续编辑','stay')}${button(action,'accept',true)}</footer>`;
    d.querySelector('.ex-close').onclick=()=>d.close();d.querySelector('[data-action=stay]').onclick=()=>d.close();d.querySelector('[data-action=accept]').onclick=()=>{d.close();onConfirm();};d.showModal();
  }
  function closeForm(){
    const finish=()=>{dirty=false;form={category:'',owner:form.owner,name:'',strategy:'',ids:[],versions:[],error:''};render();say('本次任务配置已清空，已确认的个人素材和转存保留。');};
    if(dirty)confirm('放弃本次任务配置？','<p>任务尚未创建。已在素材选择器确认加入的个人素材和转存，不会因为放弃任务而撤销。</p>','放弃配置',finish);else finish();
  }
  function changeScope(key,value){
    const previous=form[key];if(value===previous)return;
    const commit=()=>{form[key]=value;form.strategy='';form.versions=[];form.ids=[];form.error='';dirty=true;render();};
    render();
    if(form.strategy||form.ids.length)confirm(key==='category'?'更换类目？':'更换任务负责人？',`<p>从「${previous}」更换为「${value||'未选择'}」后，将清空本次已选策略、${form.ids.length} 条素材及成片设置。</p><p>任务名称保留，已经加入的个人素材及转存保持不变。</p>`,'更换并清空选择',commit);else commit();
  }
  function updateNumbers(){
    const t=totals(),count=selected().length;
    $('design-form').querySelectorAll('[data-version-summary] tbody tr').forEach((row,i)=>{row.cells[2].textContent=form.versions[i].qty+' 份';row.cells[3].textContent=form.versions[i].qty*count+' 条';});
    $('design-form').querySelector('.design-count').innerHTML=`<span>${count} 条素材 × 每条 ${copies()} 份 =</span><strong>${t.all}</strong><b>条合成任务</b>`;
    $('design-form').querySelector('.design-count + p').textContent=`${t.ready} 条可排队 · ${t.waiting} 条等待源视频转存。合成完成后预计得到 ${t.all} 个成片文件。`;
    $('design-form').querySelector('footer>div').innerHTML=`保存后创建 <strong>${t.all}</strong> 条任务并加入执行队列<small>素材转存中的任务会等待文件就绪。</small>`;
  }
  $('design-form').addEventListener('input',e=>{
    if(e.target.dataset.field==='name'){form.name=e.target.value;form.nameError=false;e.target.removeAttribute('aria-invalid');$('design-form').querySelector('.ex-error')?.remove();dirty=true;}
    if(e.target.dataset.version!==undefined){
      const i=Number(e.target.dataset.version),key=e.target.dataset.param;
      form.versions[i][key]=e.target.type==='number'?Number(e.target.value):e.target.value;dirty=true;
      e.target.setCustomValidity(key==='name'&&!e.target.value.trim()?'请填写成片设置名称':'');
      if(key==='start'||key==='end')$('param-'+i+'-end').setCustomValidity('');
      if(key==='name'){$('design-form').querySelectorAll('[data-version-summary] tbody tr')[i].cells[0].textContent=e.target.value;$('design-form').querySelector('[data-naming-examples]').innerHTML=namingExamples();}
    }
    if(e.target.dataset.qty!==undefined){const n=Number(e.target.value),valid=Number.isInteger(n)&&n>=1&&n<=20;e.target.setAttribute('aria-invalid',String(!valid));e.target.setCustomValidity(valid?'':'请输入 1–20 的整数');dirty=true;if(valid){form.versions[+e.target.dataset.qty].qty=n;updateNumbers();}}
  });
  $('design-form').addEventListener('change',e=>{
    const k=e.target.dataset.field;if(k==='category'||k==='owner')return changeScope(k,e.target.value);
    if(k==='strategy'){
      const id=e.target.value,commit=()=>{form.strategy=id;form.versions=structuredClone(strategy()?.versions||[]);form.error='';dirty=true;render();};
      const changed=JSON.stringify(form.versions)!==JSON.stringify(strategy()?.versions||[]);
      if(form.strategy&&changed){render();confirm('更换合成策略？','<p>已选素材保留，本次调整的成片设置名称、份数和合成参数将替换为新策略的默认设置。</p>','更换策略',commit);}else commit();
    }
  });
  $('design-form').addEventListener('click',e=>{const a=e.target.closest('[data-action]')?.dataset.action;if(!a)return;
    if(a==='pick')openPicker();if(a==='close')closeForm();if(a.startsWith('remove-')){form.ids=form.ids.filter(id=>id!==+a.slice(7));dirty=true;render();}
    if(a==='save'){
      const invalid=$('design-form').querySelector('input:invalid');if(invalid){invalid.closest('details')?.setAttribute('open','');invalid.reportValidity();return;}
      if(strategy()?.template==='视频截取'){
        const seconds=value=>value.split(':').reduce((n,x)=>n*60+Number(x),0);
        const bad=form.versions.findIndex(v=>seconds(v.end)<=seconds(v.start));
        if(bad>=0){$('version-settings').open=true;const input=$('param-'+bad+'-end');input.setCustomValidity('截取终点必须晚于起点');input.reportValidity();return;}
      }
      if(!form.name.trim()){form.nameError=true;render();$('task-name').setAttribute('aria-invalid','true');$('task-name').focus();return;}
      form.error='';const t=totals();const d=$('design-result');
      d.innerHTML=`<header class="ex-overlay-header"><h2 id="result-heading">创建结果预览</h2><button class="ex-close" aria-label="关闭结果预览">×</button></header><div class="ex-overlay-body"><div class="ex-result-summary"><h3>已提交 ${t.all} 条合成任务</h3><p>本次：${esc(form.name)}</p></div><div class="result-totals"><div><strong>${t.ready}</strong><span>排队中</span></div><div><strong>${t.waiting}</strong><span>等待素材</span></div><div><strong>0</strong><span>合成完成</span></div></div><p>任务已创建不代表成片已生成。实际接入后，在合成任务中查看本次批量记录和执行结果。</p><p class="design-muted">这是设计状态示例，未创建真实任务。</p></div>`;d.querySelector('button').onclick=()=>d.close();d.showModal();
    }
  });
  function openPicker(source='mine'){
    pick={source,ids:new Set(form.ids),query:'',use:'',page:1,onlySelected:false,sort:'',dir:-1,detail:null};renderPicker();$('material-picker').showModal();
  }
  function filtered(){let rows=materials.filter(m=>m.category===form.category&&(pick.source==='library'||mine(m))&&(!pick.onlySelected||pick.ids.has(m.id))&&(!pick.query||[m.title,m.author,m.id].join(' ').includes(pick.query))&&(!pick.use||(pick.use==='unused'?!m.use.includes('已制作')&&!m.use.includes('已发布'):m.use.includes('已发布'))));if(pick.sort)rows.sort((a,b)=>(pick.sort==='date'?a.date.localeCompare(b.date):a[pick.sort]-b[pick.sort])*pick.dir);return rows;}
  function renderPicker(){
    const rows=filtered(),pages=Math.max(1,Math.ceil(rows.length/4));pick.page=Math.min(pick.page,pages);const visible=rows.slice((pick.page-1)*4,pick.page*4),valid=visible.filter(available),chosen=materials.filter(m=>pick.ids.has(m.id)),added=chosen.filter(m=>!mine(m)),moving=added.filter(m=>m.state==='remote');
    $('material-picker').innerHTML=`<header class="ex-overlay-header"><h2 id="picker-heading">选择素材</h2><button class="ex-close" data-pick="close" aria-label="关闭选择素材">×</button></header><div class="ex-overlay-body"><div class="picker-topline"><span>类目：<strong>${form.category}</strong></span><span class="design-muted">任务负责人：${form.owner}</span></div><div class="picker-tabs" role="tablist" aria-label="素材来源"><button role="tab" aria-selected="${pick.source==='mine'}" data-pick="mine">我的素材</button><button role="tab" aria-selected="${pick.source==='library'}" data-pick="library">素材库</button></div><div class="picker-filters"><div class="ex-field"><label for="material-query">综合搜索</label><input id="material-query" value="${esc(pick.query)}" placeholder="素材名称、作者、ID"></div><div class="ex-field"><label for="material-use">我的使用</label><select id="material-use"><option value="">全部</option><option value="unused" ${pick.use==='unused'?'selected':''}>未制作／未使用</option><option value="published" ${pick.use==='published'?'selected':''}>已发布</option></select></div>${button('查询','query',true)}${button('重置','reset')}</div><div class="picker-tools"><span>${pick.source==='mine'?'个人工作集':'团队素材库'} · 匹配 ${rows.length} 条</span><div><button class="ex-text-button" data-pick="all" ${rows.some(available)?'':'disabled'}>选择全部匹配可用项（${rows.filter(available).length}）</button><label><input type="checkbox" data-pick="selected" ${pick.onlySelected?'checked':''}> 只看已选</label></div></div><div class="ex-table-scroll"><table class="ex-table picker-table"><thead><tr><th><input type="checkbox" aria-label="选择本页可用素材" data-pick="page" ${valid.length&&valid.every(m=>pick.ids.has(m.id))?'checked':''} ${valid.length?'':'disabled'}></th><th>素材 ID</th><th>素材 / 作者</th><th><button data-sort="date">发布时间 ↕</button></th><th><button data-sort="views">浏览 ↕</button><br><small>赞 / 收藏</small></th><th>我的使用</th><th>源视频</th></tr></thead><tbody>${visible.map(m=>`<tr><td><input type="checkbox" aria-label="选择素材 ${m.id}" data-material="${m.id}" ${pick.ids.has(m.id)?'checked':''} ${available(m)?'':'disabled'}></td><td>${m.id}</td><td><div class="picker-title"><span class="picker-thumb" aria-hidden="true">${m.category==='剪辑教程'?'剪辑':'拍摄'}</span><div><strong>${m.title}</strong><small>${m.author}</small></div></div></td><td>${m.date}</td><td>${m.views.toLocaleString()}<small>${m.likes.toLocaleString()} / ${m.saves.toLocaleString()}</small></td><td>${m.use}<small>${mine(m)?'已加入':'未加入我的素材'}</small></td><td>${tag(...states[m.state])}${available(m)?`<small>${m.state==='remote'?'确认选用后转存':m.state==='moving'?'可选，任务等待转存':''}</small>`:`<small>暂不可制作</small><button class="ex-text-button" data-info="${m.id}">查看原因</button>`}</td></tr>`).join('')||'<tr><td colspan="7"><div class="design-empty">没有匹配素材，请调整筛选条件。</div></td></tr>'}</tbody></table></div>${pick.detail?`<div class="ex-notice warning">${materials.find(m=>m.id===pick.detail).title}：${materials.find(m=>m.id===pick.detail).state==='expired'?'源视频链接已失效，需要补充有效来源。':'源视频转存失败，需要检查来源并重新转存。'}恢复可用后再选择；本页保留其他已选项。</div>`:''}<div class="picker-page"><span>共 ${rows.length} 条 · 每页 4 条（设计示例）</span><div><button class="ex-button" data-pick="prev" ${pick.page===1?'disabled':''}>上一页</button><span>${pick.page} / ${pages}</span><button class="ex-button" data-pick="next" ${pick.page===pages?'disabled':''}>下一页</button></div></div><div class="ex-notice info">点击“使用所选”后，新选素材会加入当前负责人的“我的素材”，未转存视频开始转存。取消本次选择不会加入或转存；之后取消任务不会撤销已确认的选用和转存。</div></div><footer class="ex-overlay-footer"><div class="selection-total" aria-live="polite">已选 <strong>${pick.ids.size}</strong> 条（跨页、跨 Tab 保留）<small>新增个人素材 ${added.length} 条 · 新发起转存 ${moving.length} 条</small></div><button class="ex-button" data-pick="close">取消</button><button class="ex-button primary" data-pick="apply">使用所选</button></footer>`;enhance($('material-picker'));
  }
  $('material-picker').addEventListener('click',e=>{
    const b=e.target.closest('button'),a=b?.dataset.pick,operation=b?.dataset.action;
    if(operation==='query'){pick.query=$('material-query').value.trim();pick.use=$('material-use').value;pick.page=1;renderPicker();}
    if(operation==='reset'){pick.query='';pick.use='';pick.onlySelected=false;pick.page=1;renderPicker();}
    if(a==='mine'||a==='library'){pick.source=a;pick.page=1;pick.detail=null;renderPicker();}
    if(a==='next'||a==='prev'){pick.page+=a==='next'?1:-1;renderPicker();}
    if(a==='close')$('material-picker').close();
    if(a==='all'){filtered().filter(available).forEach(m=>pick.ids.add(m.id));renderPicker();}
    if(a==='apply'){materials.filter(m=>pick.ids.has(m.id)).forEach(m=>{if(!mine(m)){m.mine=true;m.owner=form.owner;if(m.state==='remote')m.state='moving';}});form.ids=[...pick.ids];dirty=true;$('material-picker').close();render();}
    if(b?.dataset.info){pick.detail=+b.dataset.info;renderPicker();}
    if(b?.dataset.sort){pick.dir=pick.sort===b.dataset.sort?-pick.dir:-1;pick.sort=b.dataset.sort;renderPicker();}
  });
  $('material-picker').addEventListener('change',e=>{const el=e.target;if(el.dataset.material){const id=+el.dataset.material;el.checked?pick.ids.add(id):pick.ids.delete(id);renderPicker();}if(el.dataset.pick==='page'){filtered().slice((pick.page-1)*4,pick.page*4).filter(available).forEach(m=>el.checked?pick.ids.add(m.id):pick.ids.delete(m.id));renderPicker();}if(el.dataset.pick==='selected'){pick.onlySelected=el.checked;pick.page=1;renderPicker();}});
  document.querySelectorAll('[data-scene]').forEach(b=>b.onclick=()=>reset(b.dataset.scene));
  const sceneFromHash=()=>reset(location.hash==='#picker'?'picker':location.hash==='#settings'?'settings':'filled');
  addEventListener('hashchange',sceneFromHash);
  sceneFromHash();
})();
