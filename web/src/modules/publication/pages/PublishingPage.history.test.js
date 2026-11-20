import { createSSRApp } from 'vue';
import { renderToString } from 'vue/server-renderer';
import { beforeEach, expect, it, vi } from 'vitest';

const services = vi.hoisted(() => ({
  publication: { get: vi.fn(), attempt: vi.fn(), list: vi.fn(), edit: vi.fn(), create: vi.fn() },
  compose: { getComposeOutput: vi.fn() },
  accounts: { get: vi.fn() },
}));
vi.mock('../../../shared/api/publication.js', () => ({ createPublicationClient: () => services.publication }));
vi.mock('../../../shared/api/compose.js', () => ({ createComposeClient: () => services.compose }));
vi.mock('../../../shared/api/mediaAccounts.js', () => ({ createMediaAccountClient: () => services.accounts }));
vi.mock('../../../shared/api/files.js', () => ({ createFilesClient: () => ({}) }));
vi.mock('../../../shared/ui/feedback/index.js', () => ({ feedback: { toast: { success: vi.fn() }, confirmWarning: vi.fn(async () => true) } }));
vi.mock('vue-router', () => ({ useRoute: () => ({ query: {} }), useRouter: () => ({ push: vi.fn() }), onBeforeRouteLeave: vi.fn() }));
import PublishingPage from './PublishingPage.vue';

const task = id => ({
  id, revision: '1', status: 'pending', request_key: `request-${id}`,
  category_id: '1', media_account_id: '9', composite_output_id: '10', platform: 'baidu',
  content: { description: `标题-${id}`, cover_file_id: '11', declaration: '无需声明', confirmed: true },
  resources: {},
});
const history = [{ id: 'attempt-A', attempt_no: 1, input_snapshot: { content: { description: '仅属于任务A的历史快照' } } }];
let state;
beforeEach(async () => {
  vi.clearAllMocks();
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
  state.openCreate();
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
  expect(state.needsExecutionRefresh.value).toBe(false);
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
