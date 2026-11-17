import './runtime/mine-model.js';
import { occupied } from '../model.js';
export const publishingPlatform = name => name==='百度'?'百家号':name;
const nextId=rows=>Math.max(0,...rows.map(row=>Number(row.id)||0))+1;
export function syncProductionResources(db,production,actor='陈小雨') {
  const categories=[...new Set(production.materials.map(m=>m.category).filter(Boolean))];
  for(const account of production.accounts||[]) {
    if(!['百度','百家号','B站','小红书'].includes(account.platform))continue;
    if(!db.accounts.some(a=>a.sourceAccountId===account.id))db.accounts.push({...account,id:nextId(db.accounts),sourceAccountId:account.id,platform:publishingPlatform(account.platform),categories:[...categories],quality:'普通',window:'内容生产演示窗口',loginValid:true});
  }
  for(const output of production.outputs) {
    const video=(typeof window==='undefined'?globalThis:window).MineVideo.resources(output,output.fileUsage?.[actor]||{},'current');
    const material=production.materials.find(m=>m.id===output.materialId),task=production.tasks.find(t=>t.id===output.taskId);
    let asset=db.assets.find(a=>a.sourceOutputId===output.id);
    if(!asset){asset={id:nextId(db.assets),sourceOutputId:output.id};db.assets.push(asset);}
    Object.assign(asset,{title:[material?.title||'素材',task?.version?.name||output.name].filter(Boolean).join(' · '),category:material?.category||'—',quality:'普通',materialId:output.materialId,productionTaskId:output.taskId,duration:output.metadata?.duration||'待识别',cover:asset.cover||'',ready:true,fileAvailable:video.remote||video.local,videoUrl:video.remote&&!video.cloudDemo?video.cloudUrl:null,publishingPlatforms:(output.platforms||[]).filter(p=>!output.assignees||output.assignees[p]===actor).map(publishingPlatform),productionOwner:output.owner});
  }
  for(const asset of db.assets.filter(a=>a.sourceOutputId!=null))if(!production.outputs.some(o=>o.id===asset.sourceOutputId))Object.assign(asset,{ready:false,fileAvailable:false,publishingPlatforms:[]});
}
export function productionPublication(db,outputId,platform) {
  if(!Array.isArray(db.tasks))return {key:'unknown',text:'—',tasks:[]};
  const asset=db.assets.find(a=>a.sourceOutputId===outputId);
  const tasks=asset?db.tasks.filter(t=>t.assetId===asset.id&&t.platform===publishingPlatform(platform)&&(t.status==='已发布'||occupied(t))):[];
  const task=tasks.find(t=>t.status==='已发布')||tasks[0];
  return task?{key:task.status==='已发布'?'published':'reserved',text:task.status==='已发布'?'已发布':'已安排',task,tasks}:{key:'unarranged',text:'未安排',tasks:[]};
}
