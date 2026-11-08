/* 虚构演示数据；不包含真实账户、凭据、平台结果或服务连接。 */
window.PISeed = function () {
  const names = ['新版本入门：三分钟理清配装思路','活动奖励怎么拿？这份路线先收藏','新手容易忽略的三个操作细节','实战复盘：这次转点为什么成功','角色养成顺序与资源分配建议','本周更新重点，一次讲清楚','地图路线对比：不同位置怎么选','每日任务速通，少走这些弯路'];
  const categories = ['三角洲行动','火影忍者','洛克王国'];
  const assets = Array.from({length:24},(_,i)=>({id:4101+i,title:names[i%8]+(i>=8?' · '+(Math.floor(i/8)+1):''),category:categories[i%3],quality:i<8?'优质':'普通',duration:['01:36','02:18','00:58','03:12'][i%4],kind:i%3,date:'10-29 16:'+String(10+i).padStart(2,'0')}));
  const accounts = [
    {id:101,name:'林间游戏研究所',platform:'B站',quality:'优质',window:'B站 · 上海01',channel:'browser',enabled:true},
    {id:102,name:'阿澈的战术笔记',platform:'B站',quality:'优质',window:'B站 · 上海02',channel:'browser',enabled:true},
    {id:103,name:'像素观察室',platform:'B站',quality:'普通',window:'B站 · 杭州03',channel:'browser',enabled:true},
    {id:104,name:'长风游戏日记',platform:'B站',quality:'普通',window:'B站 · 杭州04',channel:'browser',enabled:true},
    {id:105,name:'清晨游玩指南',platform:'B站',quality:'普通',window:'B站 · 上海05',channel:'browser',enabled:true},
    {id:106,name:'战术地图册',platform:'B站',quality:'普通',window:'B站 · 上海06',channel:'browser',enabled:true},
    {id:107,name:'游戏攻略档案',platform:'百度',quality:'优质',window:'百度 · 上海07',channel:'api',enabled:true},
    {id:108,name:'小熊的日常攻略',platform:'百度',quality:'普通',window:'百度 · 杭州08',channel:'api',enabled:true},
    {id:109,name:'山海游玩记录',platform:'B站',quality:'普通',window:'B站 · 上海09',channel:'browser',enabled:false,reason:'需重新登录'}
  ];
  accounts.forEach(a=>a.channels=a.channel==='api'?['browser','api']:['browser']);
  const model=PIPublishModel;
  const resources={assets,accounts};
  assets.forEach(a=>{a.format='16:9';a.variant='通用版';});
  assets.push({...assets[0],id:4141,sourceAssetId:4101,title:assets[0].title+' · 竖版',format:'9:16',variant:'竖版适配'});
  const planSpecs = [
    ['三角洲 · 优质内容四日计划','三角洲行动','auto','browser','running',24,8],
    ['版本更新 · 人工连续发布','火影忍者','assisted','browser','running',12,3],
    ['每日攻略 · 百度发布','洛克王国','auto','api','paused',16,6],
    ['周末活动内容储备','三角洲行动','auto','browser','draft',8,0],
    ['新手入门 · 双账号分发','火影忍者','assisted','browser','ready',10,0],
    ['十月活动攻略收尾','洛克王国','auto','browser','completed',18,18],
    ['战术笔记 · 第二轮发布','三角洲行动','assisted','browser','paused',14,4],
    ['常规内容 · 每日更新','火影忍者','auto','api','completed',20,20]
  ];
  const plans=planSpecs.map((spec,i)=>{
    const assetIds=assets.filter(a=>!a.sourceAssetId).slice(0,i===0?12:spec[5]).map(a=>a.id);
    const platform=spec[3]==='api'?'百度':'B站',groups=[model.group(resources,platform,assetIds,{mode:spec[2],channel:spec[3]})];
    if(i===0){groups[0].mode='assisted';const other=model.group(resources,'百度',assetIds,{mode:'auto',channel:'api'});other.useCommon=false;other.assetIds=[...assetIds.slice(0,4),...assetIds.slice(-4)];groups.push(other);}
    return {id:2101+i,name:i===0?'本周攻略 · 多平台发布计划':spec[0],category:spec[1],assetIds,groups,status:spec[4],seedDone:i===0?1:spec[6],defaultMode:spec[2],defaultChannel:'browser',start:'2026-10-30',date:'10-29 17:'+String(10+i).padStart(2,'0')};
  });
  let taskId=6101;
  const tasks=plans.flatMap(plan=>{
    let index=0;
    return model.schedule(resources,plan).rows.map(row=>{
      const i=index++,status=i<plan.seedDone?'published':plan.status==='running'&&i===plan.seedDone?(row.mode==='assisted'?'waiting':'preparing'):'ready',id=taskId++;
      return {...row,id,planId:plan.id,scheduled:row.time,created:'10-29 18:10',status,phase:status==='waiting'?3:status==='preparing'?1:0,published:status==='published'?'10-30 10:08':null,url:status==='published'?'https://demo.invalid/works/'+id:'',guideStatus:i<Math.max(0,plan.seedDone-1)?'done':'pending',initStatus:i<Math.max(0,plan.seedDone-2)?'done':'pending',reason:''};
    });
  });
  ['ready','preparing','waiting','failed','submitted','published','ready','ready'].forEach((status,i)=>{const asset=assets[i],account=accounts[i%8],id=taskId++;tasks.push({id,assetId:asset.id,title:asset.title,accountId:account.id,planId:null,channel:account.channel,mode:i%2?'auto':'assisted',status,phase:status==='waiting'?3:status==='preparing'?1:0,scheduled:'2026-10-30T14:'+String(i*5).padStart(2,'0'),created:'10-30 09:'+String(10+i),published:status==='published'?'10-30 10:08':null,url:status==='published'?'https://demo.invalid/works/'+id:'',description:'本期整理了实用思路与操作细节，欢迎交流。',tags:'游戏攻略,新手入门',partition:'单机游戏',post:model.post(resources,account.platform),platform:account.platform,sourceAssetId:asset.id,guideStatus:'pending',initStatus:'pending',reason:status==='failed'?'账号登录已失效，请完成登录后继续。':''});});
  const interactions = Array.from({length:8},(_,i)=>{
    const publication=tasks.filter(t=>t.status==='published'&&t.platform==='B站')[Math.floor(i/2)],internal=i%2===0,platform=internal?accounts.find(a=>a.id===publication.accountId).platform:i===3?'百度':'B站';
    const ids=internal?publication.post.accountIds.filter(id=>id!==publication.accountId).slice(0,publication.post.perWork):i===3?[107,108]:[101,102,103,104,105,106];
    const status=['completed','partial','running','ready','attention','completed','paused','ready'][i];
    if(internal)publication.initStatus=status==='completed'?'done':status==='attention'||status==='partial'?'attention':'pending';
    return {id:8101+i,title:internal?publication.title:assets[i].title,target:{title:internal?publication.title:assets[i].title,platform,author:internal?accounts.find(a=>a.id===publication.accountId).name:'外部作者 · 演示',url:internal?publication.url:'https://demo.invalid/works/external-'+i,kind:internal?'internal':'external',assetId:internal?publication.assetId:undefined},source:internal?'发布计划':'手工发起',planId:internal?publication.planId:null,publicationId:internal?publication.id:null,actions:internal?[...publication.post.actions]:['like','comment'],status,date:'10-30 '+String(9+Math.floor(i/2)).padStart(2,'0')+':'+(i%2?'30':'10'),rows:ids.map((id,j)=>({accountId:id,comment:['这次讲得很清楚，感谢整理。','这个思路可以，期待下一期实战。','想了解更多新手配装的选择。'][j%3],status:i===0||i===5?'success':i===1?(j<4?'success':j===4?'failed':'uncertain'):i===2?(j<2?'success':j===2?'running':'pending'):i===4?(j===0?'uncertain':j===1?'failed':'pending'):'pending',reason:i===1&&j===4?'需要重新登录':i===4&&j===0?'已提交，结果待核实':''}))};
  });
  const messages=['这个配装思路适合刚入门的玩家吗？','第二个路线的具体入口在哪里？能不能再讲一下。','新活动奖励明天还能领取吗？','讲得很清楚，已经收藏了！','这个操作在移动端也可以使用吗？','上一期的内容在哪里可以找到？','想看一份资源不多的新手培养路线。','更新以后这套搭配还适用吗？'];
  const replies=messages.map((text,i)=>({id:9101+i,user:['半杯乌龙','青山慢行','小岛玩家','今天也想通关','纸飞机','南风','夏日回声','游戏小白'][i],text,assetId:assets[i].id,accountId:accounts[i%8].id,title:assets[i].title,status:i===3||i===6?'handled':'pending',time:'10-30 '+String(10+Math.floor(i/4)).padStart(2,'0')+':'+String(12+i*3).padStart(2,'0'),thread:[{who:'user',text,time:'今天 10:'+String(12+i*3).padStart(2,'0')},...(i===3?[{who:'mine',text:'谢谢支持！有想看的内容也欢迎告诉我们。',time:'今天 10:28'}]:[])]}));
  return {version:2,assets,accounts,plans,tasks,interactions,replies,templates:[{id:1,name:'攻略讨论 · 通用',text:'这次的思路讲得很清楚。\n想看更多新手入门的细节。\n这部分很实用，期待下一期。'},{id:2,name:'作者引导 · 问题收集',text:'你最想深入了解哪一部分？欢迎在评论区告诉我。'},{id:3,name:'回复 · 新手提问',text:'可以先从视频里第一种方案开始，再根据手上的资源逐步调整。'}],seq:10000};
};
