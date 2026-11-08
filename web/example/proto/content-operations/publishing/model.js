/* 原型的纯业务计算：平台分组、账号匹配和排期；不连接业务服务。 */
window.PIPublishModel = (() => {
  const clone = value => JSON.parse(JSON.stringify(value));
  const platforms = ['B站','百度'];
  const now = '2026-10-30T14:00';
  const eligible = (db, platform, channel='browser') => db.accounts.filter(a=>a.platform===platform&&a.enabled&&(channel!=='api'||a.channels?.includes('api')));
  const post = (db, platform) => ({guide:true,guideText:'这期你最想深入了解哪一部分？欢迎在评论区交流。',init:true,accountIds:eligible(db,platform).filter(a=>platform!=='B站'||a.quality==='普通').map(a=>a.id),perWork:platform==='B站'?3:1,actions:['like','comment'],comments:['这次的思路讲得很清楚。','想看更多新手入门的细节。','这部分很实用，期待下一期。']});
  const group = (db, platform, assetIds, defaults={}) => ({platform,enabled:true,useCommon:true,assetIds:[...assetIds],accountIds:eligible(db,platform).map(a=>a.id),mode:defaults.mode||'auto',channel:defaults.channel||'browser',modeInherited:true,channelInherited:true,paused:false,distribution:'one',matching:'quality',rules:[{quality:'优质',daily:1,start:'10:00',end:'20:00'},{quality:'普通',daily:2,start:'10:00',end:'20:00'}],publish:{prefix:'',description:'本期整理了实用思路与操作细节，欢迎交流。',tags:'',partition:platform==='B站'?'单机游戏':'游戏'},post:post(db,platform),versions:{}});
  const ids = (plan,g) => g.useCommon?plan.assetIds:g.assetIds;
  const enabled = plan => plan.groups.filter(g=>g.enabled!==false);
  const locked = task => ['published','preparing','waiting','submitted'].includes(task.status);
  const schedule = (db,plan) => {
    const rows=[],errors=[],groups=[];
    for(const g of enabled(plan)) {
      const contents=ids(plan,g).map(id=>db.assets.find(a=>a.id===id)).filter(Boolean);
      const accounts=eligible(db,g.platform,g.channel).filter(a=>g.accountIds.includes(a.id));
      const existing=plan.editId?db.tasks.filter(t=>t.planId===plan.editId&&(t.platform||db.accounts.find(a=>a.id===t.accountId)?.platform)===g.platform&&locked(t)):[];
      const occupied=new Map();
      existing.forEach(t=>{const key=t.accountId+':'+t.scheduled.slice(0,10);occupied.set(key,(occupied.get(key)||0)+1);});
      const slotFor=account=>{
        const rule=g.rules.find(r=>r.quality===account?.quality)||g.rules[0],capacity=Math.max(1,Number(rule.daily)||1);
        let day=0,date,key,count;
        do {
          const d=new Date((plan.start||'2026-10-30')+'T00:00:00Z');d.setUTCDate(d.getUTCDate()+day);date=d.toISOString().slice(0,10);
          key=account?.id+':'+date;count=occupied.get(key)||0;if(count<capacity)break;day++;
        }while(day<3650);
        return {rule,capacity,day,date,key,count};
      };
      const groupRows=[];
      if(!contents.length)errors.push(g.platform+'：请选择发布内容。');
      if(!accounts.length)errors.push(g.platform+'：请选择可用的'+(g.channel==='api'?' API ':'')+'发布账号。');
      if(g.rules.some(r=>r.daily<1||r.daily>20||!r.start||!r.end||r.end<=r.start))errors.push(g.platform+'：请设置有效的每日条数与发布时段。');
      const assign=(asset,account)=>{
        if(existing.some(t=>(t.sourceAssetId||t.assetId)===asset.id&&(g.distribution!=='all'||t.accountId===account?.id)))return;
        const {rule,capacity,date,key,count}=slotFor(account);
        const mins=value=>{const [h,m]=(value||'10:00').split(':').map(Number);return h*60+m;};
        const first=mins(rule.start),last=mins(rule.end),offset=Math.max(0,accounts.indexOf(account))*3;
        const minute=Math.min(last-1,Math.round(first+(last-first)*(count%capacity)/capacity)+offset);
        const time=date+'T'+String(Math.floor(minute/60)).padStart(2,'0')+':'+String(minute%60).padStart(2,'0');
        const version=db.assets.find(a=>a.id===Number(g.versions?.[asset.id]))||asset;
        groupRows.push({platform:g.platform,sourceAssetId:asset.id,assetId:version.id,title:(g.publish.prefix||'')+asset.title,accountId:account?.id||null,time,mode:g.mode,channel:g.channel,description:g.publish.description,tags:g.publish.tags,partition:g.publish.partition,post:clone(g.post)});
        if(account)occupied.set(key,count+1);
      };
      for(const asset of contents) {
        if(g.distribution==='all')accounts.forEach(a=>assign(asset,a));
        else {
          let candidates=g.matching==='quality'?accounts.filter(a=>a.quality===asset.quality):accounts;
          if(!candidates.length)candidates=accounts;
          const account=[...candidates].sort((a,b)=>{const left=slotFor(a),right=slotFor(b);return left.day-right.day||left.count/left.capacity-right.count/right.capacity;})[0];
          assign(asset,account);
        }
      }
      groupRows.sort((a,b)=>a.time.localeCompare(b.time)||a.sourceAssetId-b.sourceAssetId);
      rows.push(...groupRows);
      const end=groupRows.map(r=>r.time.slice(0,10)).sort().at(-1)||plan.start;
      groups.push({platform:g.platform,contents:contents.length,accounts:accounts.length,tasks:groupRows.length,locked:existing.length,days:Math.max(1,Math.round((new Date(end)-new Date(plan.start||'2026-10-30'))/86400000)+1),end});
    }
    return {rows,errors,groups,contentCount:new Set(enabled(plan).flatMap(g=>ids(plan,g))).size};
  };
  return {clone,platforms,now,eligible,post,group,ids,enabled,locked,schedule};
})();
