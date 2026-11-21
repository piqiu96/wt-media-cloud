import { createSSRApp } from 'vue';
import { renderToString } from 'vue/server-renderer';
import { expect, it, vi } from 'vitest';
import Dialog from './PublishingTerminationDialog.vue';
async function setup(action) {
 const emit = vi.fn(); const props = { visible: true, task: { id:'7',revision:'9',current_attempt_id:'21',platform_account_uid:'original-owner' },action,busy:false,submitted:false };
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
