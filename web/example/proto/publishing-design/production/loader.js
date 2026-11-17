import commonCSS from '../../../common/example.css?inline';
import workspaceCSS from './workspace.css?inline';
import productionCSS from './runtime/production.css?inline';
import { createProductionContext } from './context.js';
let loading,style;
export function loadProduction(services) {
 if(!loading)loading=(async()=>{
   await import('../../../common/controls.js');await import('./assets/icons.js');
   await import('./assets/ui-controls.entry.js');await import('./assets/library-date-range.entry.js');
   await import('./runtime/data.js');await import('./runtime/production.js');
   const O=createProductionContext(window.CP,services);
   await import('./runtime/production-model.js');await import('./forms.js');await import('./runtime/experience.js');
   const join=window.CP.join;window.CP.join=(ids,owner)=>{join(ids,owner);ids.forEach(O.prepareMaterial);window.CP.save();};
   await import('./runtime/materials.js');await import('./runtime/strategies.js');await import('./runtime/components.js');
   await import('./runtime/strategy-execution.js');await import('./runtime/tasks.js');await import('./runtime/outputs.js');
   await import('./runtime/library.js');await import('./runtime/mine-model.js');await import('./runtime/mine.js');
   style=document.createElement('style');style.dataset.productionStyles='';
   style.textContent='@scope (.production-scope) {'+[commonCSS,workspaceCSS,productionCSS].join('\n').replace(/:root|(?<![\w-])body(?![\w-])/g,':scope')+'}';
   document.head.append(style);style.sheet.disabled=true;
   return O;
 })();
 return loading.then(O=>{O.services=services;return O;});
}
export function attachProduction(O,target,page){
 const C=window.CP,route=page==='production-tasks'?'tasks':page;
 O.listContexts||={};
 if(O.route&&O.target){O.listContexts[O.route]={page:C.ui.page,size:C.ui.size,selected:[...C.ui.selected],scroll:window.scrollY};}
 if(O.route!==route){const saved=O.listContexts[route];C.ui.page=saved?.page||1;C.ui.size=saved?.size||8;C.ui.selected=new Set(saved?.selected||[]);}
 style.sheet.disabled=false;O.target=target;O.route=C.route=route;O.render();
 requestAnimationFrame(()=>window.scrollTo(0,O.listContexts[route]?.scroll||0));
}

export function detachProduction(O){const C=window.CP;O.listContexts||={};if(O.route)O.listContexts[O.route]={page:C.ui.page,size:C.ui.size,selected:[...C.ui.selected],scroll:window.scrollY};O.closeAll();O.target=null;O.route=C.route='';style.sheet.disabled=true;}
