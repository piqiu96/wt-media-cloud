import { createSSRApp, reactive, ref } from 'vue';
import { renderToString } from 'vue/server-renderer';
import { beforeEach, expect, it, vi } from 'vitest';
import { feedback } from '../../../shared/ui/feedback/index.js';
import BatchResourceSetup from './BatchResourceSetup.vue';

vi.mock('../../../shared/ui/feedback/index.js', () => ({ feedback: { confirmWarning: vi.fn(async () => true) } }));
vi.mock('../../../shared/api/publication.js', () => ({
  createPublicationBatchClient: () => ({ resources: vi.fn(async () => ({})) }),
  createPublicationClient: () => ({}),
}));
vi.mock('../usePublicationPlatforms.js', () => ({ usePublicationPlatforms: () => ({
  options: ref([{ value: 'baidu', label: '百家号' }, { value: 'bilibili', label: 'B站' }]), loading: ref(false), error: ref(''),
}) }));

beforeEach(() => vi.clearAllMocks());
async function setup(extra = {}) {
  let state;
  const props = reactive({ form: { platform: 'baidu', targets: [{ category_id: '1', target_count: 1 }],
    config: { accounts: [{ media_account_id: '7', limit: 1 }], output_ids: ['8'], manual_pairs: [], candidates: [{ row_key: 'row' }] } },
    catalog: {}, categories: [], actor: {}, recipes: { row: { title: '原封面' } }, coverFiles: { row: '9' }, ...extra });
  await renderToString(createSSRApp({ setup(_props, context) { state = BatchResourceSetup.setup(props, context); return () => null; } }));
  return { state, props };
}
it('preserves saved batch platform, candidates and cover inputs without asking to clear them', async () => {
  const { state, props } = await setup({ platformLocked: true });
  const before = JSON.stringify(props);
  await state.changePlatform('bilibili');
  expect(JSON.stringify(props)).toBe(before);
  expect(feedback.confirmWarning).not.toHaveBeenCalled();
  expect(state.platformFixed.value).toBe(true);
});
it.each(['creation', 'copy'])('allows confirmed platform selection for %s', async () => {
  const { state, props } = await setup({ platformLocked: false });
  await state.changePlatform('bilibili');
  expect(props.form.platform).toBe('bilibili');
  expect(props.form.config).toEqual({ accounts: [], output_ids: [], manual_pairs: [], candidates: [] });
  expect(props.form.targets).toEqual([{ category_id: '1', target_count: 1 }]);
  expect(props.recipes).toEqual({});
  expect(props.coverFiles).toEqual({});
  expect(feedback.confirmWarning).toHaveBeenCalledOnce();
});
it('rechecks the platform lock if editing state changes during confirmation', async () => {
  const { state, props } = await setup({ platformLocked: false });
  feedback.confirmWarning.mockImplementationOnce(async () => { props.platformLocked = true; return true; });
  await state.changePlatform('bilibili');
  expect(props.form.platform).toBe('baidu');
  expect(props.form.config.candidates).toHaveLength(1);
});
