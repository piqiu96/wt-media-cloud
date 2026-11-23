import { createSSRApp } from 'vue';
import { renderToString } from 'vue/server-renderer';
import { expect, it, vi } from 'vitest';
import Dialog from './PublishingTerminationDialog.vue';
async function setup(action, extra = {}) {
 const emit = vi.fn(); const props = { visible: true, task: { id:'7',revision:'9',current_attempt_id:'21',platform_account_uid:'original-owner' },action,busy:false,submitted:false, ...extra };
 let state;
 await renderToString(createSSRApp({ setup(){ state=Dialog.setup(props,{emit,expose:()=>{}}); return ()=>null; } }));
 return {state,emit,props};
}
it('requires explicit safe-scene and no-work confirmations before abandonment', async () => {
 const {state,emit,props}=await setup('abandon');
 state.reason.value='原资源已无法恢复'; state.basis.value='已检查原账号稿件与原执行进程，没有可继续发布的内容';
 state.submit(); expect(emit).not.toHaveBeenCalled();
 state.sceneClosed.value=true;
 state.submit(); expect(emit).not.toHaveBeenCalled();
 state.noPublishableWork.value=true;
 state.submit();
 expect(emit).toHaveBeenCalledWith('submit',{expected_revision:'9',attempt_id:'21',evidence:{platform_uid:'original-owner',reason:state.reason.value,basis:state.basis.value,scene_closed:true,no_publishable_work:true}});
 props.submitted=true; state.submit(); expect(emit).toHaveBeenCalledTimes(1);
});
it('cancellation sends only the original revision and never claims scene evidence', async () => {
 const {state,emit}=await setup('cancel');
 state.submit(); expect(emit).not.toHaveBeenCalled();
 state.reason.value='停止当前安排'; state.submit(); expect(emit).toHaveBeenCalledWith('submit',{expected_revision:'9',reason:'停止当前安排'});
});

it('management abandonment reuses recorded proof and never manual scene claims',async()=>{
 const {state,emit,props}=await setup('abandon',{management:true,savedEvidence:null});
 state.reason.value='调整资源安排';state.basis.value='依据原尝试中已有的安全结束与未投稿记录';
 state.sceneClosed.value=true;state.noPublishableWork.value=true;
 state.submit();expect(emit).not.toHaveBeenCalled();
 props.savedEvidence={scene_closed:true,no_publishable_work:true};
 state.sceneClosed.value=false;state.noPublishableWork.value=false;
 state.submit();expect(emit).toHaveBeenCalledWith('submit',expect.objectContaining({evidence:expect.objectContaining({scene_closed:true,no_publishable_work:true})}));
});
