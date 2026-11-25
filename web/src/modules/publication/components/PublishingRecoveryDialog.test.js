import { createSSRApp } from "vue";
import { renderToString } from "vue/server-renderer";
import { expect, it, vi } from "vitest";
import Dialog from "./PublishingRecoveryDialog.vue";
async function mount(
  task = {
    id: "1",
    current_attempt_id: "7",
    revision: "2",
    status: "uncertain",
    platform_account_uid: "6572476037",
  }, extra = {},
) {
  let state;
  const emit = vi.fn();
  await renderToString(
    createSSRApp({
      setup() {
        state = Dialog.setup(
          { visible: true, task, busy: false, error: "", ...extra },
          { emit, expose: () => {} },
        );
        return () => null;
      },
    }),
  );
  return { state, emit };
}
it("requires actual scene and no-work confirmation instead of asserting them automatically", async () => {
  const { state: s, emit } = await mount();
  s.conclusion.value = "failed";
  s.reason.value = "已核对";
  s.basis.value = "已经检查原账号稿件和原执行现场";
  s.submit();
  expect(emit).not.toHaveBeenCalled();
  s.sceneClosed.value = true;
  s.noPublishableWork.value = true;
  s.submit();
  expect(emit).toHaveBeenCalledWith(
    "submit",
    expect.objectContaining({
      evidence: expect.objectContaining({
        scene_closed: true,
        no_publishable_work: true,
      }),
    }),
  );
});
it("supplements reviewing with the original receipt, distinct work ID and explicit public evidence", async () => {
  const { state: s, emit } = await mount({
    id: "1",
    current_attempt_id: "7",
    revision: "3",
    status: "reviewing",
    platform_account_uid: "6572476037",
    platform_submission_id: "1882371730190912814",
  });
  s.conclusion.value = "published";
  s.reason.value = "已核实公开";
  s.basis.value = "已核对原账号和视频，匿名可访问指定作品";
  s.submissionId.value = "1882371730190912814";
  s.workId.value = "7000395122085014122";
  s.workUrl.value =
    "https://mbd.baidu.com/newspage/data/videolanding?nid=sv_7000395122085014122";
  s.publishedTime.value = "2026-11-20T18:12:53";
  s.sceneClosed.value = true;
  s.workMatched.value = true;
  s.submit();
  expect(emit).not.toHaveBeenCalled();
  s.publiclyAccessible.value = true;
  s.submit();
  expect(emit).toHaveBeenCalledWith(
    "submit",
    expect.objectContaining({
      platform_submission_id: "1882371730190912814",
      platform_work_id: "7000395122085014122",
      published_at: "2026-11-20T10:12:53.000Z",
    }),
  );
});

it('management supplementation needs recorded closure and cannot verify an uncertain member scene',async()=>{
 const task={id:'1',current_attempt_id:'7',revision:'3',status:'reviewing',platform_account_uid:'6572476037'};
 const {state:s,emit}=await mount(task,{management:true,savedEvidence:null});
 s.conclusion.value='published';s.reason.value='补录公开结果';s.basis.value='根据原回执核实作品已公开并核对视频一致';
 s.submissionId.value='18';s.workId.value='70';s.workUrl.value='https://example.com/work/70';s.publishedTime.value='2026-11-20T18:12:53';s.workMatched.value=true;s.publiclyAccessible.value=true;s.sceneClosed.value=true;
 s.submit();expect(emit).not.toHaveBeenCalled();
 const good=await mount(task,{management:true,savedEvidence:{scene_closed:true}});
 for(const key of ['conclusion','reason','basis','submissionId','workId','workUrl','publishedTime','workMatched','publiclyAccessible'])good.state[key].value=s[key].value;
 good.state.submit();expect(good.emit).toHaveBeenCalledWith('submit',expect.objectContaining({evidence:expect.objectContaining({scene_closed:true})}));
 const uncertain=await mount({...task,status:'uncertain'},{management:true,savedEvidence:{scene_closed:true}});
 uncertain.state.submit();expect(uncertain.emit).not.toHaveBeenCalled();
});

it('lets the owner verify a stopped reviewing original with all four conclusions but restricts management to public supplement',async()=>{const task={id:'1',current_attempt_id:'7',revision:'3',status:'reviewing',platform_account_uid:'6572476037'};const owner=await mount(task,{management:false});expect(owner.state.conclusions.value.map(v=>v.value)).toEqual(['published','reviewing','failed','uncertain']);owner.state.conclusion.value='failed';owner.state.reason.value='平台明确拒绝公开';owner.state.basis.value='已在原账号指定回执确认拒发并关闭原场景';owner.state.sceneClosed.value=true;owner.state.noPublishableWork.value=true;owner.state.submit();expect(owner.emit).toHaveBeenCalledWith('submit',expect.objectContaining({conclusion:'failed',evidence:expect.objectContaining({no_publishable_work:true})}));const manager=await mount(task,{management:true,savedEvidence:{scene_closed:true}});expect(manager.state.conclusions.value.map(v=>v.value)).toEqual(['published']);});
