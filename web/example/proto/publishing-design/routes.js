export const operationsBase = '/example/operations';
export const navigationGroups = [
  {label:'内容生产',pages:[['materials','素材库','image'],['mine','我的素材','folder'],['strategies','合成策略','settings'],['production-tasks','合成任务','video'],['outputs','成片管理','video']].map(([id,label,icon])=>({id,label,icon}))},
  {label:'发布管理',pages:[['tasks','发布任务','send'],['batches','发布批次','list'],['plans','发布计划','calendar']].map(([id,label,icon])=>({id,label,icon}))},
  {label:'运营工具',pages:[['images','图片库','image'],['templates','封面模板','template']].map(([id,label,icon])=>({id,label,icon}))},
];
const paths={materials:'production/materials',mine:'production/mine',strategies:'production/strategies','production-tasks':'production/tasks',outputs:'production/outputs',tasks:'publishing/tasks',batches:'publishing/batches',plans:'publishing/plans',images:'tools/images',templates:'tools/templates'};
export const productionPages=['materials','mine','strategies','production-tasks','outputs'];
function operationPath(page='materials',{taskId,batchId,templateId,pending=false,planId,editor=false}={}) {
  const path=operationsBase+'/'+(paths[page]||paths.materials);
  if(page==='batches'&&batchId)return path+'/'+encodeURIComponent(batchId)+(pending?'?pending=1':'');
  if(page==='templates'&&templateId)return path+'/'+encodeURIComponent(templateId)+'/edit';
  return taskId&&page==='tasks'?path+'/'+encodeURIComponent(taskId):page==='plans'?(editor?(planId?path+'/'+encodeURIComponent(planId)+'/edit':path+'/new'):planId?path+'/'+encodeURIComponent(planId):path):path;
}
export function operationURL(page='materials', options={}) {
 const path=operationPath(page,options),params=new URLSearchParams(path.split('?')[1]||'');
 if(options.fromPlan)params.set('fromPlan',options.fromPlan);
 if(options.fromBatch)params.set('fromBatch',options.fromBatch);
 return path.split('?')[0]+(params.size?'?'+params:'');
}
export function readOperationRoute(url) {
  const path=url.pathname.replace(/\/$/,'');
  const page=Object.keys(paths).find(key=>path===operationURL(key)||(['tasks','plans','batches','templates'].includes(key)&&path.startsWith(operationURL(key)+'/')))||'materials';
  const tail=path.slice(operationURL(page).length+1);
  return {page,batchId:page==='batches'&&tail?(Number(tail)||decodeURIComponent(tail)):null,templateId:page==='templates'?Number(tail.split('/')[0])||null:null,taskId:page==='tasks'&&/^\d+$/.test(tail)?Number(tail):null,planId:page==='plans'&&tail&&tail!=='new'?decodeURIComponent(tail.split('/')[0]):null,editor:page==='plans'&&(tail==='new'||tail.endsWith('/edit'))};
}
export function legacyOperationURL(url) {
  const [hash,query]=url.hash.slice(1).split('?'),params=new URLSearchParams(query);
  if(url.pathname.startsWith(operationsBase))return null;
  if(url.searchParams.get('view')==='publishing')return operationURL(['tasks','batches','plans','images','templates'].includes(hash)?hash:'tasks',{taskId:params.get('task'),batchId:params.get('batch'),templateId:params.get('edit'),pending:params.get('pending')==='1'});
  if(url.pathname.includes('/publishing-design/plan/'))return operationURL('plans',{planId:params.get('plan'),editor:hash==='editor'});
  if(url.pathname.includes('/content-operations/')) {
    const pages={materials:'materials',mine:'mine',strategies:'strategies',tasks:'production-tasks',outputs:'outputs','publish/tasks':'tasks','publish/plans':'plans','tools/images':'images','tools/templates':'templates'};
    return operationURL(pages[hash]||'materials');
  }
  return null;
}
export function updateOperationURL(page,options={},replace=false) {
  const current=new URL(location.href);
 if(page==='batches'&&options.batchId&&!options.fromPlan&&current.searchParams.get('fromPlan'))options={...options,fromPlan:current.searchParams.get('fromPlan')};
 const next=operationURL(page,options);
  if(location.pathname+location.search+location.hash!==next)history[replace?'replaceState':'pushState'](null,'',next);
}
