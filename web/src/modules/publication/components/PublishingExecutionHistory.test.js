import { createSSRApp } from 'vue';
import { renderToString } from 'vue/server-renderer';
import { describe, expect, it } from 'vitest';
import PublishingExecutionHistory from './PublishingExecutionHistory.vue';
import PublishingAttemptDetail from './PublishingAttemptDetail.vue';
const at = '2026-11-18T07:41:00+08:00';
const attempts = [
  { id: '1', attempt_no: 1, stage: 'finished', status: 'interrupted', result_status: 'uncertain', started_at: at, heartbeat_at: at, resolved_at: at,
    input_snapshot: { content: { description: '第一版标题', topics: '#旧话题', declaration: '无需声明' } },
    error_message: '原始反馈长文', resolution_note: JSON.stringify({ conclusion: 'failed', evidence: { reason: '确认未投稿', basis: '核对原账号及现场' } }), steps: [] },
  { id: '8', attempt_no: 8, stage: 'finished', status: 'completed', result_status: 'reviewing', started_at: at, heartbeat_at: at, finished_at: at,
    input_snapshot: { content: { description: '最终投稿标题' } }, steps: [{ seq: 1, at, report: { stage: 'preparing' } }],
    evidence: { status: 'reviewing', platform_work_id: 'work-123', evidence: { receipt_id: 'work-123', scene_closed: true } } },
];
const render = (history = attempts) => renderToString(createSSRApp(PublishingExecutionHistory, { task: { status: 'reviewing', authorized_at: at }, attempts: history }));
describe('execution history presentation', () => {
  it('lists latest attempts first with an explicit detail entry and no inline history expansion', async () => {
    const html = await render();
    expect(html.indexOf('第 8 次')).toBeLessThan(html.indexOf('第 1 次'));
    expect(html).toContain('平台审核中');
    expect(html.match(/查看执行详情/g)).toHaveLength(2);
    expect(html).not.toContain('<details');
    expect(html).not.toContain('最终投稿标题');
    expect(attempts[0].id).toBe('1');
  });
  it('shows the selected frozen input, recorded stages, receipt and human verification in the detail', async () => {
    const html = (await Promise.all(attempts.map(attempt => renderToString(createSSRApp(PublishingAttemptDetail, { task: {}, attempt }))))).join('');
    for (const text of ['第一版标题', '最终投稿标题', '准备资源', '平台回执', 'work-123', '核实结论', '已核实未发布', '确认未投稿', '核对原账号及现场']) expect(html).toContain(text);
    expect(html).not.toContain('&quot;conclusion&quot;');
    expect(html).not.toContain('反馈代码');
    expect(html).not.toContain('技术信息');
    expect(html).not.toContain('undefined');
  });
  it('retains task-level feedback even when execution never produced an attempt', async () => {
    const html = await renderToString(createSSRApp(PublishingExecutionHistory, { task: { status: 'failed', failure_message: '成片资源无法读取' }, attempts: [] }));
    expect(html).toContain('任务反馈');
    expect(html).toContain('成片资源无法读取');
    expect(html).not.toContain('<details');
  });
  it('renders empty and legacy records without inventing a successful result', async () => {
    expect(await render([])).toContain('暂无已保存的执行记录');
    const html = await render([{ id: '2', attempt_no: 2, stage: 'finished', status: 'interrupted', resolution_note: '旧版核实记录', steps: null }]);
    expect(html).toContain('结果待确认');
    const detail = await renderToString(createSSRApp(PublishingAttemptDetail, { task: {}, attempt: { ...attempts[0], resolution_note: '旧版核实记录' } }));
    expect(detail).toContain('旧版核实记录');
    expect(html).not.toContain('已发布');
  });
});

it('never substitutes edited task content or cover name for an older attempt', async () => {
  const html = await renderToString(createSSRApp(PublishingAttemptDetail, {
    task: { content: { description: '后来修改的标题', cover_file_id: '22' }, resources: { cover_name: '新封面.jpg' } },
    attempt: { ...attempts[0], input_snapshot: { content: { description: '当时的标题', cover_file_id: '11', topics: '#当时的话题' } } },
  }));
  expect(html).toContain('当时的标题');
  expect(html).toContain('文件 ID：11');
  expect(html).not.toContain('后来修改的标题');
  expect(html).not.toContain('新封面.jpg');
});
it('explains a missing snapshot without falling back to current content', async () => {
  const html = await renderToString(createSSRApp(PublishingAttemptDetail, { task: { content: { description: '当前标题' } }, attempt: { id: '99', attempt_no: 9, status: 'completed' } }));
  expect(html).toContain('该次执行未保存发布内容快照');
  expect(html).toContain('结果待确认');
  expect(html).not.toContain('当前标题');
});
it('distinguishes a never-run pending task from a legacy task missing its history', async () => {
  const pending = await renderToString(createSSRApp(PublishingExecutionHistory, {task:{status:'pending'},attempts:[]}));
  const published = await renderToString(createSSRApp(PublishingExecutionHistory, {task:{status:'published'},attempts:[]}));
  expect(pending).toContain('任务尚未执行');
  expect(published).toContain('暂无已保存的执行记录');
  expect(published).not.toContain('任务尚未执行');
});
it('preserves historical prototype cover and platform fields in the shared detail', async () => {
  const html = await renderToString(createSSRApp(PublishingAttemptDetail, { task:{}, attempt:{attempt_no:1,input_snapshot:{content:{title:'当时的标题',description:'当时的简介',cover:'data:image/png;base64,AAA',visibility:'仅自己可见',partition:'游戏区'}}} }));
  for (const value of ['当时的标题','当时的简介','data:image/png;base64,AAA','仅自己可见','游戏区']) expect(html).toContain(value);
});
