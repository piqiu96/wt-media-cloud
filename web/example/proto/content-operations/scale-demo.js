/* 可重复打开的本地规模样本，保留原有演示与人工评审存储。 */
(() => {
 const O=OPS,C=CP,P=PI,M=PIPublishModel;
 O.scaleDemo=()=>{
  let d;const loaded=()=>P.db.accounts.filter(a=>a.scaleDemo).length;
  d=C.open({title:'运营规模演示',body:()=>`<p>演示日常批量筛选和处理：新增 ${O.platforms.length*80} 个账号、240 条成片、200 条发布任务及 80 条评论。</p><p class="ops-muted">账号分布在 ${O.platforms.length} 个平台，带有运营分组及内容标签，包含不可用账号、失败任务和待处理评论。所有内容均为本地样本。</p>${loaded()?`<p class="ops-note">已载入 ${loaded()} 个规模账号、${P.db.tasks.filter(t=>t.scaleDemo).length} 条发布任务。可到账号选择器按“运营1组”等标签筛选。</p>`:''}`,footer:()=>C.btn('关闭',()=>C.close(d,true))+C.btn(loaded()?'查看发布任务':'载入规模演示数据',()=>{if(!loaded())load();C.close(d,true);O.go('publish/tasks');},'primary')});
 };
 const load=()=>{
  const groups=O.platforms.map(()=>[]);
  O.platforms.forEach((platform,k)=>{for(let i=0;i<80;i++){const a={id:P.id(),name:platform+' · 演示账号'+String(i+1).padStart(3,'0'),platform,quality:i%4===0?'优质':'普通',window:'演示窗口 '+(k*80+i+1),channel:'browser',channels:platform==='百度'?['browser','api']:['browser'],enabled:i%19!==0,reason:i%19===0?'待登录':'',tags:['运营'+(Math.floor(i/20)+1)+'组',i%2?'教程':'知识','规模演示'],scaleDemo:true};P.db.accounts.push(a);if(a.enabled)groups[k].push(a);}});
  for(let i=0;i<240;i++){
   const material=C.db.materials[i%C.db.materials.length],strategy=C.db.strategies.find(s=>s.category===material.category)||C.db.strategies[0],taskId=C.id(),outputId=C.id(),name='片段'+(i+1)+' · '+(i%2?'引导版':'精简版'),version={...O.version(),name,start:i%2?'00:10':'00:00',end:i%2?'01:40':'01:00',platforms:[...O.platforms]};
   C.db.tasks.push({id:taskId,materialId:material.id,strategyId:strategy.id,name,batch:'规模演示 · 第'+(Math.floor(i/20)+1)+'批',batchId:'scale-'+Math.floor(i/20),status:'success',owner:O.actor,version,snapshot:{strategyName:strategy.name,template:strategy.template,...version},created:'11-03 08:00',finished:'11-03 08:30',trigger:'人工创建',attempt:1,scaleDemo:true});
   C.db.outputs.push({id:outputId,taskId,materialId:material.id,name,owner:O.actor,platforms:[...O.platforms],assignees:Object.fromEntries(O.platforms.map(p=>[p,O.actor])),date:'11-03 08:30',downloaded:false,scaleDemo:true});
   if(i<200){const k=i%groups.length,a=groups[k][Math.floor(i/groups.length)%groups[k].length],status=i<20?'published':i%17===0?'failed':i%13===0?'waiting':'ready',time='2026-11-03T'+String(9+Math.floor(i/60)).padStart(2,'0')+':'+String(i%60).padStart(2,'0'),id=P.id();P.db.tasks.push({id,assetId:outputId,sourceAssetId:outputId,title:material.title+' · '+name,accountId:a.id,platform:a.platform,planId:null,mode:i%2?'auto':'assisted',channel:'browser',scheduled:time,created:'11-03 08:30',status,phase:status==='waiting'?3:0,post:{...P.defaultPost(a.platform),guide:false,init:false},url:status==='published'?'https://demo.invalid/works/'+id:'',published:status==='published'?'11-03 08:50':'',reason:status==='failed'?'账号需重新登录':'',scaleDemo:true});}
  }
  O.sync();
  const publications=P.db.tasks.filter(t=>t.scaleDemo&&t.status==='published');
  for(let i=0;i<80;i++){const t=publications[i%publications.length],text=['请问这一步可以再详细讲一下吗？','这个教程很实用，期待后续。','没有看懂最后一个操作。'][i%3],time='11-03 '+String(9+Math.floor(i/60)).padStart(2,'0')+':'+String(i%60).padStart(2,'0');P.db.replies.push({id:P.id(),accountId:t.accountId,assetId:t.assetId,url:t.url,title:t.title,user:'演示观众'+(i+1),text,time,status:i%5===0?'handled':'pending',sentiment:i%7===0?'negative':'neutral',kind:'comment',thread:[{who:'user',text,time}],scaleDemo:true});}
  C.save();P.save();C.toast('规模演示数据已载入，可按账号标签批量筛选');
 };
})();
