<script setup>
import { ref, nextTick } from 'vue'
import { ImageIcon, FolderIcon, SettingIcon, VideoIcon, SendIcon, LayersIcon, CalendarIcon, TemplateIcon, MenuIcon, CloseIcon } from 'tdesign-icons-vue-next'
import { useMediaQuery } from '../../../src/shared/composables/useMediaQuery.js'
import { navigationGroups, operationURL } from './routes.js'
import { prototypePeople } from './prototypeSession.js'
const props=defineProps({page:String,actor:String})
const emit=defineEmits(['navigate','actor-change'])
const narrow=useMediaQuery('(max-width: 680px)'),side=ref(null),trigger=ref(null)
const icons={image:ImageIcon,folder:FolderIcon,settings:SettingIcon,video:VideoIcon,send:SendIcon,list:LayersIcon,calendar:CalendarIcon,template:TemplateIcon}
function close(){if(side.value?.open){side.value.close();nextTick(()=>trigger.value?.focus())}}
function go(page){close();emit('navigate',page)}
function handleActor(event){const value=event.target.value;event.target.value=props.actor;close();emit('actor-change',value)}
</script>
<template>
 <button v-if="narrow" ref="trigger" class="operations-menu-toggle" type="button" aria-label="展开菜单" @click="side.showModal()"><MenuIcon/></button>
 <component :is="narrow?'dialog':'aside'" ref="side" class="operations-nav" aria-label="内容运营导航" @cancel.prevent="close" @click="event=>{if(narrow && event.target===side)close()}">
  <div class="operations-nav-body">
   <div class="operations-brand-row"><a :href="operationURL('materials')" class="operations-brand" @click.prevent="go('materials')"><img src="/example/proto/publishing-design/logo.png" alt=""/><span><strong>起飞</strong><small>CONTENT OPERATIONS</small></span></a><button v-if="narrow" type="button" class="operations-menu-close" aria-label="关闭菜单" @click="close"><CloseIcon/></button></div>
   <nav aria-label="功能菜单"><section v-for="group in navigationGroups" :key="group.label"><h2>{{group.label}}</h2><a v-for="item in group.pages" :key="item.id" :href="operationURL(item.id)" class="operations-nav-link" :class="{active:page===item.id}" :aria-current="page===item.id?'page':undefined" @click.prevent="go(item.id)"><component :is="icons[item.icon]"/><span>{{item.label}}</span></a></section></nav>
   <label class="operations-actor"><span>当前使用者 · 原型演示</span><select :value="actor" aria-label="当前使用者" @change="handleActor"><option v-for="person in prototypePeople" :key="person.id" :value="person.name">{{person.name}} · {{person.role}}</option></select></label>
  </div>
 </component>
</template>
<style scoped>
.operations-nav{position:fixed;inset:0 auto 0 0;z-index:100;width:216px;height:100dvh;box-sizing:border-box;margin:0;padding:0;border:0;border-right:1px solid var(--wt-color-border);background:var(--wt-color-bg-container);color:var(--wt-color-text-primary);font:14px/1.6 var(--wt-font-family)}.operations-nav-body{display:flex;flex-direction:column;height:100%;overflow:auto;padding:24px 12px 16px;box-sizing:border-box}.operations-brand-row{display:flex;align-items:center;justify-content:space-between}.operations-brand{display:flex;align-items:center;gap:10px;padding:0 10px 24px;color:inherit;text-decoration:none}.operations-brand img{width:36px;height:36px;object-fit:contain}.operations-brand strong{font-size:24px;font-weight:500;letter-spacing:2px}.operations-brand small{display:block;margin-top:2px;font-size:8px;letter-spacing:.3px;color:var(--wt-text-secondary)}.operations-nav nav section+section{margin-top:24px}.operations-nav h2{margin:0 12px 8px;font-size:12px;line-height:20px;font-weight:400;color:var(--wt-text-secondary)}.operations-nav-link{display:flex;align-items:center;gap:12px;min-height:40px;box-sizing:border-box;margin-top:4px;padding:8px 12px;border-radius:4px;font-size:14px;font-weight:400;color:inherit;text-decoration:none}.operations-nav-link svg{width:20px;height:20px;flex:none;color:var(--wt-text-secondary)}.operations-nav-link:hover{background:var(--td-bg-color-container-hover)}.operations-nav-link.active{background:var(--td-brand-color-light);color:var(--td-brand-color);font-weight:500}.operations-nav-link.active svg{color:inherit}.operations-nav-link:focus-visible,.operations-brand:focus-visible,.operations-menu-toggle:focus-visible{outline:2px solid var(--td-brand-color);outline-offset:2px}.operations-actor{display:flex;flex-direction:column;gap:6px;margin-top:auto;padding:24px 8px 0;color:var(--wt-text-secondary);font-size:11px}.operations-actor select{padding:6px 4px;width:100%;border:1px solid var(--td-component-border);border-radius:4px;background:var(--td-bg-color-container);font:12px var(--wt-font-family);color:var(--wt-text-primary)}.operations-menu-toggle,.operations-menu-close{display:inline-flex;align-items:center;justify-content:center;width:32px;height:32px;border:0;border-radius:4px;background:var(--td-bg-color-container);color:var(--wt-text-primary);cursor:pointer}.operations-menu-toggle{position:absolute;top:12px;left:12px;z-index:95}.operations-menu-toggle svg,.operations-menu-close svg{width:20px;height:20px}.operations-nav::backdrop{background:rgba(0,0,0,.35)}
@media(min-width:681px) and (max-width:900px){.operations-nav{width:184px}.operations-nav-body{padding-inline:8px}.operations-brand{gap:6px;padding-inline:8px}.operations-brand small{font-size:7px}.operations-brand strong{font-size:22px}}
@media(max-width:680px){.operations-nav{width:240px;max-height:100dvh;max-width:85vw}.operations-nav:not([open]){display:none}.operations-brand{padding-bottom:20px}.operations-menu-close{align-self:flex-start}}
</style>
