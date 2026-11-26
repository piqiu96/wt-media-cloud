import { createSSRApp, reactive } from 'vue';
import { renderToString } from 'vue/server-renderer';
import { expect, it, vi } from 'vitest';
vi.mock('../../../shared/api/image.js',()=>({createImageClient:()=>({})}));
vi.mock('../../../shared/api/files.js',()=>({createFilesClient:()=>({})}));
vi.mock('../../../shared/api/publication.js',()=>({createPublicationBatchClient:()=>({generate:async(input,options)=>{
 for(const item of input.items) options.onEvent({event:'item_result',data:{...item,result:{content:{description:''},field_revisions:item.field_revisions}}});
}})}));
import BatchSelectedPreparation from './BatchSelectedPreparation.vue';
it('reviews and explicitly applies empty Bilibili descriptions but rejects empty Baidu titles',async()=>{
 for(const platform of ['bilibili','baidu']) {
  const row={row_key:'r',input_revision:'1',field_revisions:{description:'1'},category_id:'1',media_account_id:'18',composite_output_id:'153',content:{title:'原标题',description:'原简介'}};
  const emit=vi.fn();let state;
  const props=reactive({platform,rows:[row],outputFor:()=>({}),labelFor:()=> '候选'});
  await renderToString(createSSRApp({setup(){state=BatchSelectedPreparation.setup(props,{emit,expose:()=>{}});return()=>null;}}));
  state.chosen.value=['ai'];state.fields.value=['description'];state.overwrite.value=true;await state.prepare();
  const info=state.rowsReviewed.value.r;
  if(platform==='bilibili') {
   expect(info.errors).toEqual([]);expect(info.values.description).toBe('');expect(state.canApply.value).toBe(true);
   expect(row.content.description).toBe('原简介');state.apply(row,info);
   expect(emit).toHaveBeenCalledWith('apply','r',{description:''},[]);
  } else {expect(info.errors.length).toBe(1);expect(state.canApply.value).toBe(false);}
 }
});
