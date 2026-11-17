<script setup>
import { ref, onMounted, onBeforeUnmount, getCurrentInstance, createVNode, render, reactive, watch } from 'vue'
import { mountPlan } from './plan/app.js'
import * as routes from './routes.js'
import { chooseDraft } from './draftRecovery.js'
import { currentPrototypeUser, prototypePeople } from './prototypeSession.js'
import PlanList from './plan/PlanList.vue'
import PlanDetail from './plan/PlanDetail.vue'
import PlanBasicForm from './plan/PlanBasicForm.vue'
import PlanGoalScope from './plan/PlanGoalScope.vue'
import { planViews, savePlanViews, planInventory, importLegacyPlans } from './planBridge.js'
import { feedback } from '../../../src/shared/ui/feedback/index.js'
import { batchCommand } from './batchApi.js'
import planCSS from './plan/style.css?inline'
import commonCSS from '../../common/example.css?inline'
const props=defineProps({db:Object,persist:Function})
const emit=defineEmits(['view-change','navigate'])
const host=ref(null),context=getCurrentInstance().appContext
let controller,style
function mount(component,element,props){
 const update=props=>{const node=createVNode(component,props);node.appContext=context;render(node,element)};update(props);const unmount=()=>render(null,element);unmount.update=update;return unmount
}
function mountBasics(element, props){
 const options=p=>{const editor=reactive(p.editor);return {...p,editor,onChange:changes=>{Object.assign(editor,changes);p.onChange(changes)}}};
 const unmount=mount(PlanBasicForm,element,options(props)),update=unmount.update;
 unmount.update=props=>update(options(props));return unmount;
}
onMounted(async()=>{
 await import('../../common/controls.js')
 if(!host.value)return
 style=document.createElement('style');style.dataset.planStyles=''
 style.textContent='@scope (.plan-scope) {'+[commonCSS,planCSS].join('\n').replace(/:root|(?<![\w-])body(?![\w-])/g,':scope')+'}'
 document.head.append(style)
 controller=mountPlan(host.value,{
  routes,chooseDraft,loadPlans:()=>planViews(props.db),savePlans:views=>savePlanViews(props.db,views),inventory:()=>planInventory(props.db),importLegacy:legacy=>importLegacyPlans(props.db,legacy),action:async(id,action,value)=>{if(action==='continue'&&!await feedback.confirmWarning({title:'确认推进计划？',message:'仅推进下一批，当前任务结果与资源占用保持不变。',confirmText:'确认继续'}))return;if(!await props.persist())throw Error('当前改动尚未保存');Object.assign(props.db,await batchCommand(null,'planOperation',{planId:id,action,value}));},actor:currentPrototypeUser,members:prototypePeople,
  mountList:(el,props)=>mount(PlanList,el,props),mountDetail:(el,props)=>mount(PlanDetail,el,props),
  mountBasics,
  mountScope:(el,props)=>mount(PlanGoalScope,el,props),
  confirmScope:message=>feedback.confirmWarning({title:'调整发布范围？',message,confirmText:'确认调整'}),
  onView:(page,mode)=>emit('view-change',page,mode),navigate:href=>emit('navigate',href),
 })
})
watch(()=>JSON.stringify([props.db.tasks.map(t=>[t.id,t.status,t.version]),props.db.batches.map(b=>[b.id,b.phase,b.version]),props.db.plans.map(p=>[p.id,p.control,p.position])]),()=>controller?.refresh())
onBeforeUnmount(()=>{controller?.dispose();style?.remove()})
defineExpose({confirmLeave:()=>controller?.confirmLeave()??Promise.resolve(true),readRoute:()=>controller?.readRoute(),currentURL:()=>controller?.currentURL()??routes.operationURL('plans')})
</script>
<template><div ref="host" class="plan-scope"></div></template>
