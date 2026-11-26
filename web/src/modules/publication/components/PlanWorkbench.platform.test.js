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
const mixedPlan = () => ({ goals: ['baidu','bilibili'].map(platform => ({ platform, category_id:'1', target_count:2 })),
  batches: ['baidu','bilibili'].flatMap((platform,index) => [1,2].map(order => ({ row_key:platform+order,
    batch_id:platform+order, platform, group_order:index+1, batch_order:order,
    targets:[{category_id:'1',target_count:1}] }))) });
it('renumbers surviving platform groups and preserves batch order after removal', async () => {
  const { state, form } = await setup(mixedPlan());
  await state.platformScope(['bilibili']);
  expect(form.batches.map(b => [b.row_key,b.group_order,b.batch_order])).toEqual([
    ['bilibili1',1,1],['bilibili2',1,2],
  ]);
});
it('refuses platform removal that would renumber a surviving fixed batch', async () => {
  const initial = mixedPlan();
  const { state, form } = await setup(initial, { bilibili1:{fixed:true} });
  await state.platformScope(['bilibili']);
  expect(form.batches).toHaveLength(4);
  expect(form.goals).toHaveLength(2);
  expect(state.error.value).toContain('固定');
});
it('can remove a later platform without moving an earlier fixed batch', async () => {
  const { state, form } = await setup(mixedPlan(), { baidu1:{fixed:true} });
  await state.platformScope(['baidu']);
  expect(form.batches.map(b => [b.row_key,b.group_order,b.batch_order])).toEqual([
    ['baidu1',1,1],['baidu2',1,2],
  ]);
});
it('renumbers groups after changing a batch to its following platform', async () => {
  const { state, form } = await setup(mixedPlan());
  await state.batchPlatform(form.batches[1], 'bilibili');
  expect(form.batches.map(b => [b.platform,b.group_order,b.batch_order])).toEqual([
    ['baidu',1,1],['bilibili',2,1],['bilibili',2,2],['bilibili',2,3],
  ]);
  expect(form.batches[1].targets).toEqual([]);
});
it('refuses a batch platform change that moves a surviving fixed batch', async () => {
  const { state, form } = await setup(mixedPlan(), { bilibili1:{fixed:true} });
  await state.batchPlatform(form.batches[1], 'bilibili');
  expect(form.batches[1].platform).toBe('baidu');
  expect(form.batches[1].targets).toHaveLength(1);
  expect(state.error.value).toContain('固定');
});
it('refuses a batch platform change that splits one platform into noncontiguous groups', async () => {
  const { state, form } = await setup(mixedPlan());
  await state.batchPlatform(form.batches[0], 'bilibili');
  expect(form.batches[0].platform).toBe('baidu');
  expect(state.error.value).toContain('连续');
});
