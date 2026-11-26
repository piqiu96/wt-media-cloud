import { computed, effectScope, reactive, ref } from 'vue';
import { expect, it, vi } from 'vitest';
import { usePublicationGeneration } from './usePublicationGeneration.js';
function fixture(generate, getIdentity = async () => '9') {
 const scope=effectScope(),context=ref({rowKey:'row',platform:'baidu',categoryId:'1',outputId:'42',identityKey:'operator-9'}),values=reactive({description:'人工标题',cover_title:'短标题',topics:''}),adopt=vi.fn((field,value)=>{values[field]=value});
 const state=scope.run(()=>usePublicationGeneration({context,values:computed(()=>({...values})),fields:computed(()=>['description','cover_title','topics']),disabled:ref(false),generate,adopt,getIdentity}));
 state.visible.value=true;state.selected.value=['description','topics'];
 return {scope,context,values,adopt,state};
}
function result(input){return {content:{description:'模型建议',topics:'#实战'},field_revisions:input.field_revisions}}
it('generates separate Bilibili fields and rejects stale or invalid tag adoption',async()=>{
 const scope=effectScope(),context=ref({rowKey:'bili',platform:'bilibili',categoryId:'1',outputId:'141'});
 const values=reactive({title:'人工标题',description:'独立简介',tags:'游戏',cover_title:''});
 const calls=[],adopt=vi.fn();let invalid=false;
 const state=scope.run(()=>usePublicationGeneration({context,values:computed(()=>({...values})),fields:computed(()=>['title','description','tags']),disabled:ref(false),
  generate:async input=>{calls.push(input);return {content:{title:'生成标题',description:'新简介',tags:invalid?'重复\n重复':'游戏\n画中画'},field_revisions:input.field_revisions}},adopt,getIdentity:async()=> '9'}));
 state.visible.value=true;state.selected.value=['title','description','tags'];await state.start();
 expect(calls[0].content).toEqual({title:'人工标题',description:'独立简介',tags:'游戏',cover_title:''});
 expect(adopt).not.toHaveBeenCalled();
 values.tags='手动修改';values.tags='游戏';
 expect(state.canAdopt('tags')).toBe(false);
 expect(state.canAdopt('title')).toBe(true);
 await state.apply('title');expect(adopt).toHaveBeenCalledWith('title','生成标题');
 invalid=true;await state.start();expect(state.error.value).toContain('不重复');expect(state.canAdopt('tags')).toBe(false);scope.stop();
});
it('only exposes a complete suggestion for separate adoption without changing input',async()=>{
 const {state,adopt,values,scope}=fixture(async input=>result(input));await state.start();
 expect(values.description).toBe('人工标题');expect(adopt).not.toHaveBeenCalled();expect(state.canAdopt('description')).toBe(true);
 await state.apply('description');expect(values.description).toBe('模型建议');expect(adopt).toHaveBeenCalledTimes(1);expect(state.canAdopt('description')).toBe(false);expect(state.canAdopt('topics')).toBe(true);scope.stop();
});
it('an edited field stays stale even after reverting while untouched topics remain adoptable',async()=>{
 let finish;const {state,values,adopt,scope}=fixture(input=>new Promise(resolve=>{finish=()=>resolve(result(input))}));const pending=state.start();await Promise.resolve();
 values.description='新手稿';values.description='人工标题';finish();await pending;
 expect(state.canAdopt('description')).toBe(false);expect(state.canAdopt('topics')).toBe(true);state.apply('description');expect(adopt).not.toHaveBeenCalled();scope.stop();
});
it('a source or identity change cancels the request and rejects a late same-source response',async()=>{
 let finish,signal;const {state,context,adopt,scope}=fixture((input,options)=>{signal=options.signal;return new Promise(resolve=>{finish=()=>resolve(result(input))})});
 const pending=state.start();await Promise.resolve();context.value={...context.value,outputId:'43'};context.value={...context.value,outputId:'42'};expect(signal.aborted).toBe(true);finish();await pending;
 expect(state.suggestion.value).toBe(null);state.apply('topics');expect(adopt).not.toHaveBeenCalled();scope.stop();
});
it('repeated generation supersedes earlier field suggestions and malformed output never becomes adoptable',async()=>{
 let calls=0;const {state,scope}=fixture(async input=>++calls===1?result(input):{content:{description:'缺项'},field_revisions:input.field_revisions});
 await state.start();expect(state.canAdopt('description')).toBe(true);await state.start();expect(state.canAdopt('description')).toBe(false);expect(state.error.value).toBeTruthy();scope.stop();
});
it('closing cancels the generation and discards responses after unmount',async()=>{
 let finish,signal;const {state,scope}=fixture((input,options)=>{signal=options.signal;return new Promise(resolve=>{finish=()=>resolve(result(input))})});const pending=state.start();await Promise.resolve();scope.stop();expect(signal.aborted).toBe(true);finish();await pending;expect(state.suggestion.value).toBe(null);
});

it('changing login identity and generating only topics invalidates all old-user suggestions', async () => {
 let user='A';const {state,adopt,scope}=fixture(async input=>({content:Object.fromEntries(input.fields.map(field=>[field,user+field])),field_revisions:input.field_revisions}),async()=>user);
 await state.start();user='B';state.selected.value=['topics'];await state.start();await state.apply('description');expect(adopt).not.toHaveBeenCalled();expect(state.canAdopt('description')).toBe(false);scope.stop();
});
it('adoption checks the exact suggestion clicked before asynchronous identity verification',async()=>{
 let resolveIdentity,delay=false,n=0;const {state,adopt,scope}=fixture(async input=>({content:{description:'suggestion-'+(++n),topics:'#topic'},field_revisions:input.field_revisions}),()=>delay?new Promise(resolve=>{resolveIdentity=resolve}):Promise.resolve('A'));
 await state.start();delay=true;const pending=state.apply('description');expect(adopt).not.toHaveBeenCalled();delay=false;await state.start();resolveIdentity('A');await pending;expect(adopt).not.toHaveBeenCalled();scope.stop();
});
