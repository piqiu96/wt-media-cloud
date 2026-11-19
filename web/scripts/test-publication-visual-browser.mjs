// Deterministic UI regression. All API calls are intercepted; no real task is created or executed.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';
import { chromium } from 'playwright';
import { spawn } from 'node:child_process';
const root = fileURLToPath(new URL('..', import.meta.url));
const evidence = process.env.WT_MEDIA_BROWSER_EVIDENCE || '/tmp/wt-media-publication-visual';
await fs.mkdir(evidence, { recursive: true });
const server = await createServer({ root, configFile: path.join(root, 'vite.config.js'), server: { host: '127.0.0.1', port: 0, strictPort: false, open: false } });
await server.listen();
const base = `http://127.0.0.1:${server.httpServer.address().port}`;
const browser = await chromium.launch({ headless: true, executablePath: process.env.CHROMIUM_EXECUTABLE });
const resourcePath = '/api/v1/file-resources/' + 'a'.repeat(64);
const account = { id: '9', platform: 'baidu', platform_name: '走查账号', platform_account_id: '123456789', user_name: '运营用户', enabled: true, identification_status: 'identified', browser_profile_id: '8', platform_login_status: 'normal', category_ids: ['1'], team_id: '1' };
const output = { id: '130', name: '真实接口字段的成片摘要 · 横版', category_id: '1', team_id: '1', platforms: ['baidu'], video: { status: 'ready' }, publish_status: 'unpublished' };
const cover = { id: '12', name: '走查封面', resource_path: resourcePath, deleted_at: null };
const task = { id: '3', revision: '2', request_key: 'visual-only', platform: 'baidu', category_id: '1', media_account_id: '9', composite_output_id: '130', platform_account_uid: '123456789', status: 'pending', attempt_count: 0, current_attempt_id: null, authorized_at: null, created_at: '2026-11-18T06:00:00Z', not_before: null, content: { description: '走查内容，核对公共布局与返回后输入保留。', cover_file_id: '12', topics: '#公共规范', declaration: '无需声明', confirmed: true }, resources: { account_name: account.platform_name, account_owner: '运营用户', output_name: output.name, cover_name: cover.name, cover_resource_path: resourcePath } };
const checks = [], errors = [], writes = [];
const check = (name, value) => { assert(value, name); checks.push(name); };
const style = (locator, properties) => locator.evaluate((node, keys) => Object.fromEntries(keys.map(key => [key, getComputedStyle(node)[key]])), properties);
try {
 for (const width of [1440, 390]) {
  task.status = 'pending'; task.revision = '2'; task.revision = '2';
  const context = await browser.newContext({ viewport: { width, height: 1000 } });
  let videoRequests = 0;
  const page = await context.newPage(); page.on('pageerror', error => errors.push(error.message));
  await page.route('https://video.visual.invalid/**', route => route.fulfill({ status: 404, body: 'fixture media unavailable' }));
  await page.route('**/api/v1/**', async route => {
   const request = route.request(), url = new URL(request.url()), p = url.pathname;
   let data;
   if (p.includes('/file-resources/')) return route.fulfill({ contentType: 'image/svg+xml', body: '<svg xmlns="http://www.w3.org/2000/svg" width="640" height="360"><rect width="640" height="360" fill="#dbeafe"/><text x="230" y="190" font-size="32" fill="#2563eb">封面预览</text></svg>' });
   if (request.method() !== 'GET') { writes.push({ path: p, body: request.postDataJSON() }); return route.fulfill({ json: { errcode: 18001, message: '视觉走查不写入真实任务', data: null } }); }
   if (p === '/api/v1/auth/me') data = { id: '1', username: 'operator01', nickname: '走查用户', role: 'operator', category_ids: ['1'] };
   else if (p.endsWith('/categories')) data = [{ id: '1', name: '游戏', enabled: true }];
   else if (p.endsWith('/publication')) { check(`${width}: default page size`, url.searchParams.get('page_size') === '10'); data = { items: [task], total: 1, execution_enabled: true, summary: { all: 1, awaiting_human: 0, running: 0, uncertain: 0 } }; }
   else if (p.endsWith('/attempts')) data = [];
   else if (p === '/api/v1/publication/3') data = task;
   else if (p === '/api/v1/media-accounts/9') data = account;
   else if (p.endsWith('/media-accounts')) data = { items: [account], total: 1 };
   else if (p.endsWith('/video-url')) {
    videoRequests++;
    if (videoRequests === 1) return route.fulfill({ status: 503, json: { errcode: 10001, message: '测试视频暂不可读', data: null } });
    data = { url: 'https://video.visual.invalid/preview.mp4' };
   }
   else if (p === '/api/v1/compose-outputs/130') data = { output };
   else if (p.endsWith('/compose-outputs')) data = { items: [output], total: 1 };
   else if (p.endsWith('/files')) data = { items: [cover], total: 1 };
   else data = { items: [], total: 0 };
   await route.fulfill({ json: { errcode: 0, message: 'ok', data } });
  });
  await page.goto(base + '/publish');
  await page.getByRole('heading', { name: '发布任务', exact: true }).waitFor();
  if (width < 600) { await page.locator('.topbar .t-button').first().click(); await page.waitForFunction(() => document.querySelector('.app-aside').getBoundingClientRect().width < 70); }
  await page.locator('.publication-content strong').waitFor();
  const h = await style(page.locator('.resource-page-header h1'), ['fontSize', 'fontWeight']);
  check(`${width}: list heading`, h.fontSize === '24px' && h.fontWeight === '500');
  const th = await style(page.locator('.t-table__header th').first(), ['fontSize', 'backgroundColor']);
  check(`${width}: table typography`, th.fontSize === '13px');
  check(`${width}: no viewport overflow`, await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
  await page.screenshot({ path: path.join(evidence, `${width}-list.png`), fullPage: true });
  await page.getByRole('button', { name: '快速发布', exact: true }).click();
  await page.getByRole('heading', { name: '快速发布', exact: true }).waitFor();
  const footerBoundary = async () => page.locator('.resource-detail-page__footer').evaluate(node => {
    const footer = node.getBoundingClientRect(), aside = document.querySelector('.app-aside').getBoundingClientRect();
    return { left: footer.left, sidebarRight: aside.right, bottom: footer.bottom, viewportHeight: innerHeight };
  });
  const expanded = await footerBoundary();
  check(`${width}: page footer never covers sidebar`, expanded.left >= expanded.sidebarRight - 1);
  if (width > 600) {
    await page.locator('.topbar .t-button').first().click();
    await page.waitForFunction(() => document.querySelector('.app-aside').getBoundingClientRect().width < 70);
    await page.waitForFunction(() => Math.abs(document.querySelector('.resource-detail-page__footer').getBoundingClientRect().left - document.querySelector('.app-aside').getBoundingClientRect().right) < 1);
    checks.push(`${width}: footer follows collapsed sidebar`);
    await page.locator('.topbar .t-button').first().click();
    await page.waitForFunction(() => document.querySelector('.app-aside').getBoundingClientRect().width > 240);
    await page.waitForFunction(() => Math.abs(document.querySelector('.resource-detail-page__footer').getBoundingClientRect().left - document.querySelector('.app-aside').getBoundingClientRect().right) < 1);
    checks.push(`${width}: footer follows expanded sidebar`);
  }
  await page.getByRole('button', { name: '创建任务', exact: true }).click();
  await page.getByText('请选择发布类目、账号和成片', { exact: true }).waitFor();
  await page.screenshot({ path: path.join(evidence, `${width}-quick-publish.png`), fullPage: true });
  check(`${width}: empty creation stays in form`, await page.getByRole('heading', { name: '快速发布', exact: true }).isVisible());
  await page.getByRole('button', { name: '返回', exact: true }).click();
  await page.getByRole('heading', { name: '发布任务', exact: true }).waitFor();
  await page.getByRole('button', { name: '高级筛选', exact: false }).click();
  await page.locator('#publication-advanced').waitFor();
  await page.getByRole('button', { name: '详情', exact: true }).click();
  await page.getByRole('heading', { name: '发布任务详情', exact: true }).waitFor();
  await page.locator('.resource-descriptions').first().waitFor();
  check(`${width}: detail only returns to list`, await page.locator('.resource-detail-page__footer button').allTextContents().then(labels => labels.length === 1 && labels[0].trim() === '返回'));
  check(`${width}: no invented refresh action`, await page.getByRole('button', { name: '刷新进展', exact: true }).count() === 0);
  check(`${width}: shared detail fields`, await page.locator('.resource-descriptions').count() === 3);
  if (width < 600) check(`${width}: compact fields preserve readable value width`, await page.locator('.resource-descriptions .t-descriptions__content').first().evaluate(node => node.getBoundingClientRect().width >= 160));
  await page.locator('.content-area').evaluate(node => { node.scrollTop = 0; });
  await page.screenshot({ path: path.join(evidence, `${width}-detail.png`), fullPage: true });
  await page.getByRole('button', { name: '预览最终发布封面', exact: true }).click();
  const preview = page.getByRole('dialog', { name: '发布封面预览', exact: true });
  await preview.waitFor(); await preview.getByRole('button', { name: '关闭发布封面预览', exact: true }).click(); await preview.waitFor({ state: 'hidden' });
  await page.getByRole('button', { name: '返回', exact: true }).click();
  const execute = page.getByRole('button', { name: '执行', exact: true });
  check(`${width}: execution is a list text action`, await execute.evaluate(node => node.classList.contains('t-button--variant-text')));
  check(`${width}: list actions use 32px controls`, await execute.evaluate(node => getComputedStyle(node).height === '32px'));
  task.revision = '3';
  await execute.click();
  await page.getByText('任务内容或状态已变化，请核对更新后的列表再操作', { exact: true }).waitFor();
  check(`${width}: stale execution cannot authorize unseen content`, writes.length === 0 && await page.locator('.resource-feedback-dialog').count() === 0);
  await execute.click();
  const executionConfirm = page.locator('.resource-feedback-dialog'); await executionConfirm.waitFor();
  await executionConfirm.getByRole('button', { name: '取消', exact: true }).click(); await executionConfirm.waitFor({ state: 'hidden' });
  check(`${width}: cancelled execution performs no write`, writes.length === 0);
  await page.getByRole('button', { name: '编辑', exact: true }).click();
  await page.getByRole('heading', { name: '编辑发布内容', exact: true }).waitFor();
  const description = page.locator('[aria-label="发布标题"] textarea');
  await description.fill('返回资源选择弹窗后应保留这段输入');
  await page.getByRole('button', { name: '预览成片', exact: true }).click();
  const videoDialog = page.getByRole('dialog', { name: '成片预览', exact: true }); await videoDialog.waitFor();
  await videoDialog.getByText('测试视频暂不可读', { exact: true }).waitFor();
  check(`${width}: video error is recoverable`, await videoDialog.getByRole('button', { name: '重试预览', exact: true }).isVisible());
  await videoDialog.getByRole('button', { name: '重试预览', exact: true }).click();
  await videoDialog.getByText('视频暂时无法读取，请重试预览', { exact: true }).waitFor();
  check(`${width}: media load failure can renew preview URL`, videoRequests === 2 && await videoDialog.getByRole('button', { name: '重试预览', exact: true }).isVisible());
  await videoDialog.getByRole('button', { name: '关闭成片预览', exact: true }).click(); await videoDialog.waitFor({ state: 'hidden' });
  check(`${width}: video preview preserves input`, await description.inputValue() === '返回资源选择弹窗后应保留这段输入');
  const label = await style(page.locator('.quick-block label').first(), ['fontSize', 'fontWeight']);
  check(`${width}: form label`, label.fontSize === '13px' && label.fontWeight === '500');
  check(`${width}: shared date picker`, await page.locator('.resource-date-time').count() === 1 && await page.locator('input[type="datetime-local"]').count() === 0);
  const backStyle = await style(page.locator('.resource-detail-page__back'), ['backgroundColor']);
  check(`${width}: solid secondary action`, backStyle.backgroundColor !== 'rgba(0, 0, 0, 0)' && backStyle.backgroundColor !== 'rgb(255, 255, 255)');
  await page.getByRole('button', { name: '更换图片', exact: true }).click();
  const picker = page.getByRole('dialog', { name: '选择发布封面', exact: true }); await picker.waitFor();
  const rowAction = await style(picker.getByRole('button', { name: '选择', exact: true }), ['height', 'backgroundColor']);
  check(`${width}: embedded row action`, rowAction.height === '32px' && rowAction.backgroundColor === 'rgba(0, 0, 0, 0)');
  await page.screenshot({ path: path.join(evidence, `${width}-picker.png`), fullPage: true });
  await picker.getByRole('button', { name: '关闭选择发布封面', exact: true }).click(); await picker.waitFor({ state: 'hidden' });
  check(`${width}: picker preserves input`, await description.inputValue() === '返回资源选择弹窗后应保留这段输入');
  await page.locator('.content-area').evaluate(node => { node.scrollTop = node.scrollHeight; });
  const footerTop = (await page.locator('.resource-detail-page__footer').boundingBox()).y;
  const lastField = await page.getByRole('checkbox', { name: '已核对当前文案、封面和声明，确认内容可用', exact: true }).boundingBox();
  check(`${width}: footer leaves last field readable`, lastField.y + lastField.height <= footerTop);
  await page.locator('.content-area').evaluate(node => { node.scrollTop = 0; });
  await page.screenshot({ path: path.join(evidence, `${width}-editor.png`), fullPage: true });
  check(`${width}: editor no viewport overflow`, await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
  await page.getByRole('button', { name: '返回', exact: true }).click();
  const confirm = page.locator('.resource-feedback-dialog').filter({ hasText: '当前修改尚未保存' }); await confirm.waitFor();
  await confirm.getByRole('button', { name: '离开', exact: true }).click();
  await page.getByRole('heading', { name: '发布任务', exact: true }).waitFor();
  check(`${width}: list context retained`, await page.locator('#publication-advanced').isVisible());
  task.status = 'failed';
  await page.getByRole('button', { name: '查询', exact: true }).click();
  const retry = page.getByRole('button', { name: '重试', exact: true });
  await retry.waitFor();
  task.revision = '4';
  await retry.click();
  await page.getByText('任务内容或状态已变化，请核对更新后的列表再操作', { exact: true }).waitFor();
  check(`${width}: stale retry requires rereading task`, writes.length === 0 && await page.locator('.resource-feedback-dialog').count() === 0);
  await retry.click();
  const retryConfirm = page.locator('.resource-feedback-dialog'); await retryConfirm.waitFor();
  await retryConfirm.getByRole('button', { name: '取消', exact: true }).click(); await retryConfirm.waitFor({ state: 'hidden' });
  check(`${width}: retry uses list and cancellation performs no write`, writes.length === 0);
  task.status = 'uncertain';
  await page.getByRole('button', { name: '查询', exact: true }).click();
  await page.getByRole('button', { name: '核实结果', exact: true }).click();
  const recovery = page.getByRole('dialog', { name: '核实发布结果', exact: true }); await recovery.waitFor();
  await recovery.getByRole('button', { name: '保存核实结果', exact: true }).click();
  await recovery.getByText('请选择核实结论，填写原因及实际核实依据（至少 10 字）。', { exact: true }).waitFor();
  check(`${width}: verification preserves required validation`, writes.length === 0);
  const cancelStyle = await style(recovery.getByRole('button', { name: '取消', exact: true }), ['backgroundColor']);
  check(`${width}: verification uses scoped solid actions`, cancelStyle.backgroundColor !== 'rgba(0, 0, 0, 0)');
  await page.screenshot({ path: path.join(evidence, `${width}-verification.png`), fullPage: true });
  await recovery.getByRole('button', { name: '取消', exact: true }).click(); await recovery.waitFor({ state: 'hidden' });
  await page.goto(base + '/files');
  await page.locator('.resource-filter-bar').waitFor();
  const sample = () => page.locator('.resource-filter-bar .t-button').evaluateAll(nodes => nodes.map(node => { const s = getComputedStyle(node); return [s.height, s.fontSize, s.backgroundColor, s.borderColor]; }));
  const after = await sample();
  const sheets = ['resource-form-actions.css', 'resource-list.css', 'resource-data-table.css'];
  await page.evaluate(names => { for (const node of document.querySelectorAll('style[data-vite-dev-id]')) if (names.some(name => node.dataset.viteDevId.endsWith('/' + name))) node.sheet.disabled = true; }, sheets);
  assert.deepEqual(await sample(), after, 'unmigrated files page must retain existing buttons'); checks.push(`${width}: existing files page isolated`);
  await context.close();
 }
 assert.deepEqual(errors, []); assert.deepEqual(writes, []);
 await fs.writeFile(path.join(evidence, 'checks.json'), JSON.stringify({ base, fixtureOnly: true, checks, pageErrors: errors, writes }, null, 2));
 console.log(JSON.stringify({ checks: checks.length, evidence, pageErrors: errors, writes }, null, 2));
} finally { await browser.close(); await server.close(); }

if (process.env.WT_MEDIA_CHECK_PROTOTYPE === '1') {
 await fs.mkdir(path.join(evidence, 'prototype'), { recursive: true });
 const prototype = await createServer({ root, configFile: path.join(root, 'vite.config.example.js'), server: { host: '127.0.0.1', port: 18768, strictPort: true, open: false } });
 try {
  await prototype.listen();
  const child = spawn(process.execPath, ['--test', 'example/proto/publishing-design/tests/commonVisual.browser.cjs', 'example/proto/publishing-design/tests/taskPages.browser.cjs'], { cwd: root, stdio: 'inherit', env: { ...process.env, EXAMPLE_URL: 'http://127.0.0.1:18768', SCREENSHOT_DIR: path.join(evidence, 'prototype') } });
  const code = await new Promise(resolve => child.on('exit', resolve));
  assert.equal(code, 0, 'existing common and task prototype regressions');
 } finally { await prototype.close(); }
}
