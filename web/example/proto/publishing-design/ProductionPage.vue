<script setup>
import {ref,onMounted,onBeforeUnmount,watch} from 'vue';
import ResourceNotice from '../../../src/shared/ui/resource/ResourceNotice.vue';
import {attachProduction,detachProduction} from './production/loader.js';
const props=defineProps({page:String,context:Object});const host=ref(null);
const names={materials:'素材库',mine:'我的素材',strategies:'合成策略','production-tasks':'合成任务',outputs:'成片管理'};
function render(){if(host.value)attachProduction(props.context,host.value,props.page);}
function protectUnload(event){if(window.CP?.dialogs.some(d=>d.dirty||d.minePending?.())){event.preventDefault();event.returnValue='';}}
onMounted(()=>{render();window.addEventListener('beforeunload',protectUnload);});watch(()=>props.page,render,{flush:'post'});onBeforeUnmount(()=>{window.removeEventListener('beforeunload',protectUnload);detachProduction(props.context);});
</script>
<template><div class="production-page"><div class="production-context"><span>内容生产 / {{names[page]}}</span></div><ResourceNotice tone="info">交互原型：合成、下载与发布均为本地演示。</ResourceNotice><div ref="host" class="production-scope production-content"></div><div id="production-toast" class="production-toast" role="status" aria-live="polite"></div></div></template>
<style scoped>.production-context{height:56px;margin-bottom:24px;display:flex;justify-content:space-between;align-items:center;color:var(--wt-text-secondary);gap:16px}.production-context label{font-size:12px}.production-context select{padding:6px;border:1px solid var(--td-component-border);border-radius:4px;background:var(--td-bg-color-container)}.production-content{margin-top:20px;min-width:0}.production-toast:not(:empty){position:fixed;bottom:28px;left:50%;transform:translateX(-50%);background:#263449;color:white;padding:12px 20px;border-radius:6px;z-index:12000}.production-page{min-width:0}@media(max-width:700px){.production-context{height:auto;flex-wrap:wrap}}</style>
