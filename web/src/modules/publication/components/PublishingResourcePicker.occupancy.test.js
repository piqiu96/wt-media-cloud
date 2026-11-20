import { createSSRApp } from 'vue';
import { renderToString } from 'vue/server-renderer';
import { beforeEach, expect, it, vi } from 'vitest';
const clients=vi.hoisted(()=>({publication:{outputs:vi.fn()},compose:{listComposeOutputs:vi.fn()}}));
vi.mock('../../../shared/api/publication.js',()=>({createPublicationClient:()=>clients.publication}));
vi.mock('../../../shared/api/compose.js',()=>({createComposeClient:()=>clients.compose}));
import Picker from './PublishingResourcePicker.vue';
let state,emit;
beforeEach(async()=>{
 vi.clearAllMocks();emit=vi.fn();
 await renderToString(createSSRApp({setup(_,context){state=Picker.setup({kind:'output',visible:true,category:'1',account:{team_id:1}}, {...context,emit});return ()=>null;}}));
});
it('uses the publish-owned page and rejects occupied rows without scanning publication pages',async()=>{
 const base={category_id:1,team_id:1,platforms:['baidu'],video:{status:'ready'}};
 const occupied={...base,id:129,occupied:true},available={...base,id:131,occupied:false};
 clients.publication.outputs.mockResolvedValue({items:[occupied,available],total:41});
 await state.load();
 expect(clients.publication.outputs).toHaveBeenCalledWith(expect.objectContaining({platform:'baidu',page:1,page_size:10}));
 expect(clients.compose.listComposeOutputs).not.toHaveBeenCalled();
 expect(state.total.value).toBe(41);
 state.select(occupied);expect(emit).not.toHaveBeenCalled();
 state.select(available);expect(emit).toHaveBeenCalledWith('selected',available);
});
it('clears previously available candidates if occupancy cannot be refreshed',async()=>{
 clients.publication.outputs.mockResolvedValueOnce({items:[{id:131,occupied:false}],total:1}).mockRejectedValueOnce(new Error('无法读取成片占用'));
 await state.load();expect(state.rows.value).toHaveLength(1);
 await state.load();expect(state.error.value).toBe('无法读取成片占用');expect(state.rows.value).toEqual([]);
});
