/* 虚构演示数据。任务与成片保持一对一，作品/发布草稿分别记录。 */
window.CPSeed = () => {
  const platforms = ['B站','百度','小红书'];
  const materials = ['剪辑效率提升的三个方法','手机拍摄也能做出电影感','短视频开头怎样留住观众','一盏灯拍出干净的人像','旅行镜头如何自然衔接','新人做内容的选题方法','让口播更自然的实用技巧','普通场景的构图练习'].map((title,i)=>({id:101+i,title,category:i%2?'拍摄技巧':'剪辑教程',duration:'01:'+String(20+i*3),likes:2800+i*137}));
  materials.forEach((m,i)=>Object.assign(m,{platform:'抖音',platformWorkId:'DEMO-'+(810001+i),created_by:i===7?'departed-demo-user':i%2?'林予安':'陈小雨',metricsUpdated:'2026-11-03 09:00'}));
  const strategies = ['教程通用引导','B站教程引导','百度知识引导','竖屏引导','小红书经验分享','横屏精简剪辑','教程画中画引导','日常素材自动合成'].map((name,i)=>({id:201+i,name,template:i===5?'视频截取':i===6?'引导画中画':'引导视频插入',category:i===5||i===7?'拍摄技巧':'剪辑教程',operator:'陈小雨',enabled:i!==6,schedule:i===7?'每天 09:00':'手动执行',intro:'',outro:`https://media.example.test/guide/${i===0?'common':i}-outro.mp4`,start:'0',end:'90',runs:i===7?12:3}));
  const accounts = platforms.flatMap((platform,p)=>Array.from({length:p===0?5:platform==='小红书'?3:2},(_,i)=>({id:301+(platform==='小红书'?3:p)*10+i,platform,name:`${platform} · ${i===0?'主账号':'运营号 '+(i+1)}`,enabled:!(p===2&&i===1)})));
  const tasks = Array.from({length:12},(_,i)=>({id:1001+i,materialId:101+(i<4?0:i%8),strategyId:201+i%4,name:i<4?['B站主账号版','百度引导版','小红书引导版','B站运营号 2 版'][i]:'通用成片',batch:i<4?'教程素材 · 10月30日第一批':'',platform:i<4?platforms[i%3]:'',accountId:i===0?301:i===3?302:null,status:i===3?'failed':i===4?'running':i===5?'pending':'success',intro:'',outro:strategies[i%4].outro,start:'0',end:'90',created:'10-30 10:'+String(10+i),trigger:i===11?'策略自动':'人工创建',attempt:1}));
  tasks.forEach((t,i)=>{
    const s=i<4?strategies[[1,2,3,1][i]]:i===11?strategies[7]:materials.find(m=>m.id===t.materialId).category==='拍摄技巧'?strategies[5]:strategies[0];
    t.strategyId=s.id;t.outro=s.template==='视频截取'?'':s.outro;
  });
  const outputs = tasks.filter(t=>t.status==='success').map((t,i)=>({id:2001+i,taskId:t.id,materialId:t.materialId,name:t.name,platform:t.platform,accountId:t.accountId,downloaded:i<3,date:t.created}));
  const uses = [
    {id:4001,outputId:2001,platform:'B站',accountId:301,time:'10-30 10:28',views:'12.8万',increment:'+3.2万',updated:'10-30 12:30',url:'https://www.bilibili.com/video/DEMO4001',growing:true},
    {id:4002,outputId:2001,platform:'小红书',accountId:331,time:'10-30 10:40',views:'8,520',increment:'+1,280',updated:'10-30 12:30',url:'https://www.xiaohongshu.com/explore/DEMO4002',growing:false},
    {id:4003,outputId:2002,platform:'百度',accountId:311,time:'10-30 11:00',views:'3,210',increment:'+420',updated:'10-30 12:30',url:'https://baijiahao.baidu.com/s?id=DEMO4003',growing:false}
  ];
  return {version:1,seq:5000,platforms,materials,strategies,accounts,tasks,outputs,uses,drafts:[]};
};
