import { createSSRApp } from 'vue';
import { renderToString } from 'vue/server-renderer';
import { beforeEach, expect, it, vi } from 'vitest';

const services = vi.hoisted(() => ({
  session: { me: vi.fn() },
  draft: { get: vi.fn(), save: vi.fn(), clear: vi.fn(), remember: vi.fn(), persistent: false },
  chooseDraft: vi.fn(),
  images: { getImageAsset: vi.fn() },
  publication: { copy: vi.fn(), findByRequest: vi.fn(), get: vi.fn(), attempt: vi.fn(), list: vi.fn(), edit: vi.fn(), create: vi.fn(), cancel: vi.fn(), abandon: vi.fn() },
  compose: { getComposeOutput: vi.fn() },
  accounts: { get: vi.fn() },
  human: vi.fn(),
}));
vi.mock('../../../shared/api/publicationRun.js',()=>({handlePublicationHuman:services.human}));
vi.mock('../../../shared/api/image.js', () => ({ createImageClient: () => services.images }));
vi.mock('../../../shared/api/session.js', () => ({ createSessionClient: () => services.session }));
vi.mock('../../../shared/api/publicationDraft.js', () => ({ createPublicationDraftClient: () => services.draft }));
vi.mock('../draftRecovery.js', async importOriginal => ({ ...(await importOriginal()), choosePublicationDraft: services.chooseDraft }));
vi.mock('../../../shared/api/publication.js', () => ({ createPublicationClient: () => services.publication }));
vi.mock('../../../shared/api/compose.js', () => ({ createComposeClient: () => services.compose }));
vi.mock('../../../shared/api/mediaAccounts.js', () => ({ createMediaAccountClient: () => services.accounts }));
vi.mock('../../../shared/api/files.js', () => ({ createFilesClient: () => ({}) }));
vi.mock('../../../shared/ui/feedback/index.js', () => ({ feedback: { toast: { success: vi.fn() }, confirmWarning: vi.fn(async () => true) } }));
vi.mock('vue-router', () => ({ useRoute: () => ({ query: {} }), useRouter: () => ({ push: vi.fn() }), onBeforeRouteLeave: vi.fn() }));
import PublishingPage from './PublishingPage.vue';
import { feedback } from '../../../shared/ui/feedback/index.js';

const task = id => ({
  id, revision: '1', status: 'pending', can_cancel: true, request_key: `request-${id}`,
  category_id: '1', media_account_id: '9', composite_output_id: '10', platform: 'baidu',
  content: { description: `标题-${id}`, cover_file_id: '11', portrait_cover_file_id:'12', declaration: '无需声明', confirmed: true },
  resources: {},
});
const history = [{ id: 'attempt-A', attempt_no: 1, input_snapshot: { content: { description: '仅属于任务A的历史快照' } } }];
let state;
beforeEach(async () => {
  vi.clearAllMocks();
  services.session.me.mockResolvedValue({ id: '9' });
  services.draft.get.mockResolvedValue({ payload: null, revision: '0', saved_at: null, schema_version: 1 });
  services.draft.save.mockResolvedValue({ revision: '1' });
  services.draft.clear.mockResolvedValue({ revision: '2' });
  services.publication.get.mockImplementation(async id => task(id));
  services.publication.attempt.mockImplementation(async id => id === 'A' ? history : []);
  services.publication.list.mockResolvedValue({ items: [], total: 0, summary: {} });
  services.publication.edit.mockImplementation(async (id, input) => ({ ...task(id), content: input.content, revision: '2' }));
  services.publication.create.mockImplementation(async input => ({ ...input, id: 'C', revision: '1', status: 'pending' }));
  services.accounts.get.mockResolvedValue({ id: '9' });
  services.compose.getComposeOutput.mockResolvedValue({ output: { id: '10' } });
  // Mount the real page's setup without its visual shell. Navigation and save
  // handlers run unchanged; only external APIs and feedback are substituted.
  await renderToString(createSSRApp({ setup(props, context) { state = PublishingPage.setup(props, context); return () => null; } }));
});

it('does not show task A attempts after editing and saving task B from the list', async () => {
  await state.openDetail(task('A'));
  expect(state.attempts.value).toEqual(history);
  await state.back();
  expect(state.view.value).toBe('list');
  await state.edit(task('B'));
  expect(state.view.value).toBe('form');
  await state.submit();
  expect(services.publication.edit).toHaveBeenCalledWith('B', expect.objectContaining({ expected_revision: '1' }));
  expect(state.formError.value).toBe('');
  expect(state.view.value).toBe('detail');
  expect(state.detail.value.id).toBe('B');
  expect(state.attempts.value).toEqual([]);
});

it('keeps a newly created task history empty after returning from another task detail', async () => {
  await state.openDetail(task('A'));
  expect(state.attempts.value).toEqual(history);
  await state.back();
  await state.openCreate();
  state.form.value = { ...state.form.value, ...task('C') };
  await state.submit();
  expect(services.publication.create).toHaveBeenCalledOnce();
  expect(state.formError.value).toBe('');
  expect(state.view.value).toBe('detail');
  expect(state.detail.value.id).toBe('C');
  expect(state.attempts.value).toEqual([]);
  await state.back();
  await state.openDetail(task('A'));
  expect(state.attempts.value).toEqual(history);
});

it('refreshes the executing task and its history without leaving its detail', async () => {
  await state.openDetail(task('A'));
  state.detail.value.status = 'running';
  services.publication.get.mockResolvedValue({ ...task('A'), status: 'reviewing' });
  services.publication.attempt.mockResolvedValue([{ id: 'accepted-attempt', status: 'reviewing' }]);
  await state.refreshExecution();
  expect(state.view.value).toBe('detail');
  expect(state.detail.value.status).toBe('reviewing');
  expect(state.attempts.value[0].id).toBe('accepted-attempt');
  expect(state.needsExecutionRefresh.value).toBe(true);
  expect(state.executionRefreshInterval.value).toBe(30000);
});

it('does not overwrite a new task detail with an earlier background response', async () => {
  await state.openDetail(task('A'));
  state.detail.value.status = 'running';
  let finish;
  services.publication.get.mockImplementation(id => id === 'A'
    ? new Promise(resolve => { finish = resolve; }) : Promise.resolve(task(id)));
  const pending = state.refreshExecution();
  await state.openDetail(task('B'));
  finish({ ...task('A'), status: 'reviewing' });
  await pending;
  expect(state.detail.value.id).toBe('B');
  expect(state.attempts.value).toEqual([]);
});

it('skips overlapping refreshes and leaves editing and result verification alone', async () => {
  await state.openDetail(task('A'));
  state.detail.value.status = 'running';
  services.publication.get.mockClear();
  state.busy.value = true;
  await state.refreshExecution();
  state.busy.value = false;
  state.recoveryVisible.value = true;
  await state.refreshExecution();
  state.recoveryVisible.value = false;
  state.view.value = 'form';
  await state.refreshExecution();
  expect(services.publication.get).not.toHaveBeenCalled();
  state.view.value = 'detail';
  let finish;
  services.publication.get.mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  const pending = state.refreshExecution();
  await state.refreshExecution();
  expect(services.publication.get).toHaveBeenCalledOnce();
  finish({ ...task('A'), status: 'failed' });
  await pending;
});


it('closing draft recovery preserves the previous input and current page', async () => {
  services.draft.get.mockResolvedValue({ payload: { form: task('D') }, revision: '1' });
  services.chooseDraft.mockResolvedValue(null);
  await state.openCreate();
  expect(state.view.value).toBe('list');
  expect(services.draft.clear).not.toHaveBeenCalled();
});

it('saves creation input before posting and clears only after a confirmed task', async () => {
  await state.openCreate();
  state.form.value = { ...state.form.value, ...task('C') };
  await state.submit();
  expect(services.draft.save.mock.invocationCallOrder[0]).toBeLessThan(services.publication.create.mock.invocationCallOrder[0]);
  expect(services.draft.clear.mock.invocationCallOrder[0]).toBeGreaterThan(services.publication.create.mock.invocationCallOrder[0]);
});

it('preserves uncertain creation and queries the original key before any repost', async () => {
  await state.openCreate();
  state.form.value = { ...state.form.value, ...task('C') };
  services.publication.create.mockRejectedValueOnce(new Error('断网'));
  await state.submit();
  expect(state.pendingRequest.value.request_key).toBe('request-C');
  expect(services.draft.clear).not.toHaveBeenCalled();
  services.publication.findByRequest.mockResolvedValue(task('C'));
  await state.submit();
  expect(services.publication.findByRequest).toHaveBeenCalledWith('request-C');
  expect(services.publication.create).toHaveBeenCalledTimes(1);
  expect(state.view.value).toBe('detail');
});

it('keeps the created task visible when clearing local input fails', async () => {
  await state.openCreate();
  state.form.value = { ...state.form.value, ...task('C') };
  services.draft.clear.mockRejectedValueOnce(new Error('本机保存冲突'));
  await state.submit();
  expect(state.view.value).toBe('detail');
  expect(state.detail.value.id).toBe('C');
  expect(state.error.value).toContain('任务已创建');
  expect(state.pendingRequest.value).toBeNull();
});

it('restores human text when a selected resource is unavailable without silently replacing it', async () => {
  const old = task('D');
  services.draft.get.mockResolvedValue({ payload: { form: old, schedule: '', image_asset_id: '' }, revision: '2' });
  services.chooseDraft.mockResolvedValue('continue');
  services.accounts.get.mockRejectedValueOnce(new Error('无权限'));
  services.compose.getComposeOutput.mockRejectedValueOnce(new Error('已删除'));
  await state.openCreate();
  expect(state.form.value.content.description).toBe(old.content.description);
  expect(state.form.value.media_account_id).toBe('9');
  expect(state.form.value.composite_output_id).toBe('10');
  expect(state.form.value.content.confirmed).toBe(false);
  expect(state.account.value).toBeNull();
  expect(state.output.value).toBeNull();
  await state.submit();
  expect(services.publication.create).not.toHaveBeenCalled();
});

it('reselecting the same image preserves confirmation', async () => {
  await state.openCreate();
  state.form.value.content.cover_file_id = '11';
  state.form.value.content.confirmed = true;
  state.picker.value = 'image';
  state.picked({ id: '11',width:1280,height:720 });
  expect(state.form.value.content.confirmed).toBe(true);
  state.picker.value = 'image';
  state.picked({ id: '12',width:1280,height:720 });
  expect(state.form.value.content.confirmed).toBe(false);
});


it('reselects an unavailable original account after it becomes available without clearing unchanged confirmation', async () => {
  await state.openCreate();
  state.form.value.media_account_id = '9';
  state.form.value.content.confirmed = true;
  state.account.value = null;
  state.draftResourceIssue.value.account = '原账号不可用';
  state.picker.value = 'account';
  state.picked({ id: '9', platform: 'baidu' });
  expect(state.account.value.id).toBe('9');
  expect(state.draftResourceIssue.value.account).toBeFalsy();
  expect(state.form.value.content.confirmed).toBe(true);
});

it('does not apply a previous new-draft resource error to editing an existing task', async () => {
  state.draftResourceIssue.value = { account: '上一份草稿的错误' };
  await state.edit(task('B'));
  await state.submit();
  expect(services.publication.edit).toHaveBeenCalledOnce();
  expect(state.view.value).toBe('detail');
});

it('keeps an unreadable cover reference through another explicit save', async () => {
  const old = task('D');
  services.draft.get.mockResolvedValue({ payload: { form: old, image_asset_id: '22' }, revision: '2' });
  services.chooseDraft.mockResolvedValue('continue');
  services.images.getImageAsset.mockRejectedValueOnce(new Error('网络不可用'));
  await state.openCreate();
  state.form.value.content.description = '新的人工稿';
  await state.saveDraft();
  expect(services.draft.save).toHaveBeenCalledWith(expect.objectContaining({ image_asset_id: '22', form: expect.objectContaining({ content: expect.objectContaining({ description: '新的人工稿' }) }) }));
});

it('rechecks cancellation against the original list revision before showing a decision', async () => {
 services.publication.get.mockResolvedValue({...task('A'),status:'running',revision:'2',current_attempt_id:'21'});
 await state.openTermination(task('A'),'cancel');
 expect(state.terminationVisible.value).toBe(false);
 expect(services.publication.cancel).not.toHaveBeenCalled();
 expect(state.error.value).toContain('状态已变化');
});
it('submits cancellation once and refreshes after a lost reply', async () => {
 await state.openTermination(task('A'),'cancel');
 expect(state.terminationVisible.value).toBe(true);
 services.publication.cancel.mockRejectedValue(new Error('响应中断'));
 await state.saveTermination({expected_revision:'1'});
 expect(services.publication.cancel).toHaveBeenCalledTimes(1);
 expect(services.publication.list).toHaveBeenCalled();
 expect(state.terminationError.value).toContain('查询核对');
 expect(state.terminationVisible.value).toBe(true);
});
it('keeps abandonment evidence on failure and sends the frozen original attempt', async () => {
 const failed = {...task('A'),status:'failed',current_attempt_id:'21'};
 services.publication.get.mockResolvedValue(failed);
 await state.openTermination(failed,'abandon');
 const input = {expected_revision:'1',attempt_id:'21',evidence:{basis:'原账号现场核实，没有可继续发布稿件'}};
 services.publication.abandon.mockRejectedValue(new Error('任务已变化'));
 await state.saveTermination(input);
 expect(services.publication.abandon).toHaveBeenCalledWith('A',input);
 expect(state.terminationTask.value.current_attempt_id).toBe('21');
 expect(state.terminationVisible.value).toBe(true);
});

it('uses the real output thumbnail by default and follows an output change without replacing a chosen image', async () => {
 await state.openCreate();
 state.setOutput({id:'10',material_title:'新成片',thumbnail:{file_id:'9007199254740993',resource_path:'/api/v1/file-resources/'+'a'.repeat(64),width:640,height:360}});
 expect(state.form.value.content.cover_file_id).toBe('9007199254740993');
 expect(state.cover.value.origin).toBe('output');
 state.setOutput({id:'12',thumbnail:null});
 expect(state.form.value.content.cover_file_id).toBe('');
 state.cover.value={id:'88',image_asset_id:'3',origin:'image'};state.form.value.content.cover_file_id='88';
 state.setOutput({id:'13',thumbnail:{file_id:'99'}});
 expect(state.form.value.content.cover_file_id).toBe('88');
});
it('restores an output thumbnail only when the current exact output still owns the saved file', async () => {
 services.draft.get.mockResolvedValue({payload:{form:task('D'),cover_origin:'output',cover_output_id:'10'},revision:'1'});
 services.chooseDraft.mockResolvedValue('continue');
 services.compose.getComposeOutput.mockResolvedValue({output:{id:'10',thumbnail:{file_id:'11',resource_path:'/api/v1/file-resources/'+'a'.repeat(64)}}});
 await state.openCreate();
 expect(state.cover.value.origin).toBe('output');expect(state.draftResourceIssue.value.cover).toBeFalsy();
 services.compose.getComposeOutput.mockResolvedValue({output:{id:'10',thumbnail:{file_id:'12'}}});
 await state.openCreate();
 expect(state.form.value.content.cover_file_id).toBe('11');expect(state.draftResourceIssue.value.cover).toContain('原封面不可用');
});

it('keeps output cover provenance through an offline draft save so it can recover later', async () => {
 const payload={form:task('D'),cover_origin:'output',cover_output_id:'10'};
 services.draft.get.mockResolvedValue({payload,revision:'1'});services.chooseDraft.mockResolvedValue('continue');
 services.compose.getComposeOutput.mockRejectedValue(new Error('offline'));
 await state.openCreate();
 await state.saveDraft();
 const saved=services.draft.save.mock.calls.at(-1)[0];
 expect(saved.cover_origin).toBe('output');expect(saved.cover_output_id).toBe('10');expect(saved.form.content.cover_file_id).toBe('11');
 services.draft.get.mockResolvedValue({payload:saved,revision:'2'});
 services.compose.getComposeOutput.mockResolvedValue({output:{id:'10',thumbnail:{file_id:'11',resource_path:'/api/v1/file-resources/'+'a'.repeat(64)}}});
 await state.openCreate();
 expect(state.cover.value.origin).toBe('output');expect(state.draftResourceIssue.value.cover).toBeFalsy();
});

it('preserves a rendered cover and confirmation when reselecting the same output', async()=>{
 await state.openCreate();const o={id:'71',thumbnail:{file_id:'81'}};state.setOutput(o);
 state.coverConfig.value={method:'template',background_mode:'business',background_source:'output'};
 state.cover.value={id:'91',origin:'template'};state.form.value.content.cover_file_id='91';state.form.value.content.cover_render={title:'实际标题'};state.form.value.content.confirmed=true;
 state.setOutput({...o,thumbnail:{...o.thumbnail}});
 expect(state.form.value.content.cover_file_id).toBe('91');expect(state.form.value.content.confirmed).toBe(true);
 state.setOutput({...o,thumbnail:{file_id:'82'}});expect(state.form.value.content.cover_file_id).toBe('');expect(state.form.value.content.confirmed).toBe(false);
});
it('keeps the frozen generated cover through draft restore without rereading the current template',async()=>{
 const ref={template_id:'8',published_revision:2,title:'实际标题',background_image_asset_id:'0',background_output_id:'10',render_input_hash:'a'.repeat(64)};
 const draft=task('D');draft.content.cover_render=ref;
 services.images.getCover=vi.fn().mockResolvedValue({file:{id:'11',resource_path:'/actual'},snapshot:{...ref,document:{backgroundMode:'business'}},render_input_hash:ref.render_input_hash});
 services.draft.get.mockResolvedValue({payload:{form:draft,cover_origin:'template'},revision:'1'});services.chooseDraft.mockResolvedValue('continue');
 await state.openCreate();expect(state.cover.value.origin).toBe('template');expect(state.draftResourceIssue.value.cover).toBeFalsy();
 await state.saveDraft();expect(services.draft.save.mock.calls.at(-1)[0].form.content.cover_render).toEqual(ref);
 expect(state.coverConfig.value.method).toBe('template');
});

it('shows the running stop request without claiming final cancellation', async () => {
 const running = {...task('A'), status:'running',current_attempt_id:'21',can_cancel:true};
 services.publication.get.mockResolvedValue(running);
 await state.openTermination(running,'cancel');
 expect(state.terminationVisible.value).toBe(true);
 services.publication.cancel.mockResolvedValue({...running,cancel_requested_at:'2026-11-21T00:00:00Z',can_cancel:false});
 await state.saveTermination({expected_revision:'1',reason:'停止当前安排'});
 expect(feedback.toast.success).toHaveBeenCalledWith('已请求安全停止，等待执行端确认；请查看任务结果');
 expect(state.terminationVisible.value).toBe(false);
});
it('hides running termination after submission permission or a previous stop request', async () => {
 expect(state.cancellable({...task('A'),status:'running',can_cancel:false})).toBe(false);
 expect(state.cancellable({...task('A'),status:'running',can_cancel:true,cancel_requested_at:'2026-11-21'})).toBe(false);
});

it('copies reusable text into a new unconfirmed form with no old resources or execution identity',async()=>{
 services.publication.copy.mockResolvedValue({platform:'baidu',category_id:'1',content:{description:'可复用的人工文案',topics:'#话题',declaration:'无需声明'}});
 await state.copyTask({...task('A'),status:'published'});
 expect(services.publication.copy).toHaveBeenCalledWith('A','1');
 expect(state.view.value).toBe('form');expect(state.editing.value).toBe(null);
 expect(state.form.value.request_key).not.toBe('request-A');
 expect(state.form.value.content).toEqual({description:'可复用的人工文案',topics:'#话题',declaration:'无需声明',cover_file_id:'',portrait_cover_file_id:'',confirmed:false});
 expect(state.form.value.media_account_id).toBe('');expect(state.form.value.composite_output_id).toBe('');
 expect(state.account.value).toBe(null);expect(state.output.value).toBe(null);expect(state.cover.value).toBe(null);
 expect(state.pendingRequest.value).toBe(null);expect(state.attempts.value).toEqual([]);
 expect(services.publication.create).not.toHaveBeenCalled();
 state.setOutput({id:'99',material_title:'新成片默认标题'});
 expect(state.form.value.content.description).toBe('可复用的人工文案');
});
it('preserves an existing draft instead of replacing it with copied task text',async()=>{
 services.publication.copy.mockResolvedValue({platform:'baidu',category_id:'1',content:{description:'旧任务文案',topics:'',declaration:'无需声明'}});
 services.draft.get.mockResolvedValue({payload:{form:task('D'),schedule:''},revision:'2'});
 services.chooseDraft.mockResolvedValue('continue');
 await state.copyTask({...task('A'),status:'cancelled'});
 expect(state.form.value.request_key).toBe('request-D');expect(state.form.value.content.description).toBe('标题-D');
 expect(services.draft.clear).not.toHaveBeenCalled();expect(services.publication.create).not.toHaveBeenCalled();
});
it('keeps the list on rejected conditional copy and never initializes new work',async()=>{
 services.publication.copy.mockRejectedValue(new Error('原尝试尚未安全结束'));
 await state.copyTask({...task('A'),status:'failed'});
 expect(state.view.value).toBe('list');expect(state.error.value).toContain('尚未安全结束');
 expect(services.draft.get).not.toHaveBeenCalled();expect(services.publication.create).not.toHaveBeenCalled();
});

it('discards delayed copy content after the signed-in user changes before draft access',async()=>{
 let finish;services.publication.copy.mockImplementation(()=>new Promise(resolve=>{finish=resolve;}));
 const copying=state.copyTask({...task('A'),status:'published'});
 await vi.waitFor(()=>expect(finish).toBeTypeOf('function'));
 services.session.me.mockResolvedValue({id:'different-user'});
 finish({platform:'baidu',category_id:'1',content:{description:'原账号私有文案',topics:'',declaration:''}});
 await copying;
 expect(state.view.value).toBe('list');expect(services.draft.get).not.toHaveBeenCalled();expect(services.draft.clear).not.toHaveBeenCalled();
 expect(state.error.value).toContain('登录账号已变化');
});
it('does not clear an original draft after an account switch during the recovery choice',async()=>{
 services.publication.copy.mockResolvedValue({platform:'baidu',category_id:'1',content:{description:'原账号私有文案',topics:'',declaration:''}});
 services.draft.get.mockResolvedValue({payload:{form:task('D')},revision:'2'});
 services.chooseDraft.mockImplementation(async()=>{services.session.me.mockResolvedValue({id:'different-user'});return 'restart';});
 await state.copyTask({...task('A'),status:'published'});
 expect(services.draft.clear).not.toHaveBeenCalled();expect(state.view.value).toBe('list');
});
it('discards a copy response after the page has been disposed',async()=>{
 let finish;services.publication.copy.mockImplementation(()=>new Promise(resolve=>{finish=resolve;}));
 const copying=state.copyTask({...task('A'),status:'published'});
 await vi.waitFor(()=>expect(finish).toBeTypeOf('function'));
 state.invalidateCreation();
 finish({platform:'baidu',category_id:'1',content:{description:'迟到的文案',topics:'',declaration:''}});
 await copying;
 expect(services.draft.get).not.toHaveBeenCalled();expect(state.view.value).toBe('list');
});

it('handles human verification in the original task and retains retry context on a lost reply',async()=>{
 const waiting={...task('1'),revision:'5',status:'awaiting_human',current_attempt_id:'7'};
 services.publication.get.mockResolvedValue(waiting);
 services.human.mockResolvedValue({attempt_id:'7',status:'awaiting_human'});
 await state.openHumanScene(waiting);
 expect(services.human).toHaveBeenCalledWith({task:waiting,userId:'9'});
 expect(state.humanVisible.value).toBe(true);
 services.human.mockRejectedValueOnce(new Error('继续应答丢失'));
 await state.continueHumanScene();
 expect(state.humanVisible.value).toBe(true);
 expect(state.humanTask.value.revision).toBe('5');
 expect(state.humanError.value).toBe('继续应答丢失');
 services.human.mockResolvedValueOnce({attempt_id:'7',status:'running'});
 await state.continueHumanScene();
 expect(services.human).toHaveBeenLastCalledWith({task:waiting,userId:'9',resume:true});
 expect(state.humanVisible.value).toBe(false);
});
it('never continues an original scene after the signed-in user changes',async()=>{
 const waiting={...task('1'),status:'awaiting_human',current_attempt_id:'7'};
 services.publication.get.mockResolvedValue(waiting);
 services.human.mockResolvedValue({attempt_id:'7',status:'awaiting_human'});
 await state.openHumanScene(waiting);
 services.session.me.mockResolvedValue({id:'10'});
 await state.continueHumanScene();
 expect(services.human).toHaveBeenCalledTimes(1);
 expect(state.humanError.value).toContain('登录身份已变化');
 expect(state.humanVisible.value).toBe(true);
});

it('refreshes reviewing results and pauses while the human supplementation dialog is open',async()=>{
 const reviewing={...task('1'),status:'reviewing',revision:'5',current_attempt_id:'7'};
 services.publication.get.mockResolvedValue(reviewing);
 await state.openDetail(reviewing);
 expect(state.needsExecutionRefresh.value).toBe(true);
 expect(state.executionRefreshInterval.value).toBe(30000);
 services.publication.get.mockClear();
 state.recoveryVisible.value=true;
 await state.refreshExecution();
 expect(services.publication.get).not.toHaveBeenCalled();
 state.recoveryVisible.value=false;
 services.publication.get.mockResolvedValue({...reviewing,status:'published',revision:'6',work_id:'10',last_check_message:'指定作品已经公开'});
 await state.refreshExecution();
 expect(state.detail.value.status).toBe('published');
 expect(state.detail.value.work_id).toBe('10');
 expect(state.needsExecutionRefresh.value).toBe(false);
});

it('keeps the horizontal artwork when only the portrait configuration changes',async()=>{
 await state.openCreate();state.form.value.content.cover_file_id='11';
 state.cover.value={id:'11',origin:'template'};state.form.value.content.portrait_cover_file_id='12';
 state.form.value.content.confirmed=true;
 state.updatePortraitConfig({...state.portraitCoverConfig.value,method:'template',title:'新竖版标题'});
 expect(state.form.value.content.cover_file_id).toBe('11');
 expect(state.form.value.content.portrait_cover_file_id).toBe('');
 expect(state.form.value.content.confirmed).toBe(false);
});
it('refuses a landscape image for the portrait slot without replacing either cover',async()=>{
 await state.openCreate();state.form.value.content.cover_file_id='11';state.form.value.content.portrait_cover_file_id='12';
 state.chooseCoverImage('portrait');state.picked({id:'13',width:1280,height:720});
 expect(state.form.value.content.cover_file_id).toBe('11');expect(state.form.value.content.portrait_cover_file_id).toBe('12');
 expect(state.formError.value).toContain('3:4');
});
