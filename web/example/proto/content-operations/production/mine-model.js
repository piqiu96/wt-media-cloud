/* 我的素材视频状态投影；设备文件、准备与下载事实保持独立。 */
(() => {
  const labels={pending:'待准备',preparing:'准备中',available:'可用',downloading:'下载中',failed:'失败',unknown:'—'};
  const resources=(material,usage={},device='current',now=Date.now())=>{
    const effective={...material,...usage.videoDemo?.[device],...usage.preparedDemo},file=usage.files?.[device];
    let remote=false;try{remote=effective.cloudFileExists===true&&effective.cloudUrlVerified===true&&/^https?:$/.test(new URL(effective.cloudUrl).protocol)&&(!effective.cloudUrlExpiresAt||Date.parse(effective.cloudUrlExpiresAt)>now);}catch{}
    const local=file?.exists===true&&file?.verified===true;
    return {effective,remote,local,cloudUrl:remote?effective.cloudUrl:'',cloudDemo:effective.cloudDemo===true||effective.cloudUrl==='https://media.example.test/prototype-only.mp4',file:local?file:null};
  };
  const state=(material,usage={},device='current',now=Date.now())=>{
    const {effective:m,remote,local}=resources(material,usage,device,now),file=usage.files?.[device],transfer=usage.transfers?.[device],active=usage.status==='active';
    const result=(key,reason='')=>({key,label:labels[key],reason,remote,local,canCompose:remote,download:active&&!local&&!['preparing','downloading','unknown'].includes(key),tone:key==='available'?'success':key==='failed'?'danger':['preparing','downloading'].includes(key)?'info':'',progress:transfer?.progress});
    if(local)return result('available');
    if(['preparing','downloading'].includes(transfer?.phase))return result(transfer.phase);
    if(transfer?.phase==='failed')return result('failed',transfer.reason||'本次下载未完成，请重试');
    if(remote)return result('available',file?.exists===false?'本机文件已丢失，可重新下载':'');
    if(m.cloudState==='preparing')return result('preparing');
    if(m.cloudState==='failed')return result('failed',m.cloudError||'视频获取失败，可重试');
    if(m.cloudFileExists===true)return result('unknown','视频访问地址需重新核验');
    if(m.sourceState==='expired'||m.cloudState==='expired')return {...result('failed','原作品已失效，未确认有可用视频'),download:false};
    if(m.cloudState==='notSaved'||m.cloudFileExists===false)return result('pending');
    return result('unknown','视频可用性尚未核验，下载前需确认');
  };
  window.MineVideo={labels,state,resources};
})();
