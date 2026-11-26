import { createSSRApp, reactive } from 'vue';
import { renderToString } from 'vue/server-renderer';
import { expect, it, vi } from 'vitest';
vi.mock('../../../shared/ui/feedback/index.js', () => ({ feedback: { confirmWarning: vi.fn(async () => true) } }));
import PlanWorkbench from './PlanWorkbench.vue';

async function setup(form, facts = {}) {
  let state;
  const props = reactive({ form, categories: [{ id: '1', name: '游戏' }, { id: '2', name: '其他游戏' }], disabled: false, batchFacts: facts });
  await renderToString(createSSRApp({ setup(_props, context) { state = PlanWorkbench.setup(props, context); return () => null; } }));
  return { state, form: props.form };
}
it('keeps platform/category targets separate when generating and splitting batches', async () => {
  const { state, form } = await setup({ goals: [{ platform: 'baidu', category_id: '1', target_count: 4 }], batches: [] });
  await state.platformScope(['baidu', 'bilibili']);
  expect(form.goals).toEqual([{ platform: 'baidu', category_id: '1', target_count: 4 }, { platform: 'bilibili', category_id: '1', target_count: 1 }]);
  state.distribute();
  expect(form.batches.map(b => [b.platform, b.targets[0].target_count])).toEqual([['baidu', 4], ['bilibili', 1]]);
  expect(state.balanced.value).toBe(true);
  expect(state.goalColumns.value).toEqual([{ category_id: '1' }]);
  expect(state.platformTotal('baidu')).toBe(4);
  expect(state.platformTotal('bilibili')).toBe(1);
  expect(state.categoryTotal('1')).toBe(5);
});
it('uses the selected Bilibili platform when categories and empty batches are added', async () => {
  const { state, form } = await setup({ goals: [], batches: [] });
  await state.platformScope(['bilibili']);
  await state.scope(['1', '2']);
  expect(form.goals.map(g => g.platform)).toEqual(['bilibili', 'bilibili']);
  state.add();
  expect(form.batches[0].platform).toBe('bilibili');
  expect(state.balanced.value).toBe(false);
});
it('cannot remove a platform containing a fixed batch', async () => {
  const { state, form } = await setup({ goals: [{ platform: 'bilibili', category_id: '1', target_count: 1 }],
    batches: [{ row_key: 'b', batch_id: '9', platform: 'bilibili', targets: [{ category_id: '1', target_count: 1 }] }] }, { '9': { fixed: true } });
  await state.platformScope(['baidu']);
  expect(form.goals[0].platform).toBe('bilibili');
  expect(form.batches).toHaveLength(1);
  expect(state.error.value).toContain('不可移除');
});
