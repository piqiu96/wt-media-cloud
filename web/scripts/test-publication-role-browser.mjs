// Exercises the real Cloud and Desktop Vue entries with controlled APIs and native bridge.
// Does not connect to an Agent, create a real task, or submit to a platform.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';
import { chromium } from 'playwright';

const root = fileURLToPath(new URL('..', import.meta.url));
const evidence = process.env.WT_MEDIA_BROWSER_EVIDENCE || '/tmp/wt-publication-role-review';
await fs.mkdir(evidence, { recursive: true });
const browser = await chromium.launch({ headless: true, channel: 'chrome' });
const checks = [];
const output = { id: '130', category_id: '1', team_id: '1', name: '走查成片', material_title: '走查成片', platforms: ['baidu'], video: { status: 'ready' }, publish_status: 'unpublished' };
const tasks = ['pending', 'failed', 'awaiting_human', 'uncertain', 'reviewing', 'published', 'pending', 'failed', 'awaiting_human', 'uncertain', 'running'].map((status, i) => ({
  id: String(i + 1), operator_id: i < 6 ? '1' : '9', revision: '1', status, platform: 'baidu', category_id: '1', media_account_id: '9', composite_output_id: '130',
  current_attempt_id: status === 'pending' ? null : '7', authorized_at: status === 'pending' ? null : '2026-11-23T01:00:00Z', can_cancel: ['pending','running'].includes(status), created_at: '2026-11-23T01:00:00Z',
  content: { description: `走查任务${i + 1}`, cover_file_id: '12', portrait_cover_file_id: '13', topics: '', declaration: '无需声明', confirmed: true },
  resources: { account_name: '走查账号', output_name: '走查成片' },
}));
try {
  for (const app of ['cloud', 'desktop']) {
    const server = await createServer({ root, configFile: path.join(root, app === 'cloud' ? 'vite.config.js' : 'vite.config.desktop.js'), cacheDir: path.join(evidence, `.vite-${app}`), server: { host: '127.0.0.1', port: 0, strictPort: false, open: false } });
    await server.listen();
    const base = `http://127.0.0.1:${server.httpServer.address().port}`;
    try {
      for (const role of ['admin', 'operator', 'senior_operator']) {
        const context = await browser.newContext({ viewport: { width: 1440, height: 1100 } });
        const page = await context.newPage(), errors = [], calls = [], writes = [], nativeCalls = [];
        await context.exposeFunction('__recordNativeCall', command => nativeCalls.push(command));
        page.on('pageerror', error => errors.push(error.message));
        if (app === 'desktop') await context.addInitScript(({ base }) => {
          window.__nativeCalls = [];
          window.__TAURI_INTERNALS__ = {
            invoke: async (command, args) => {
              window.__nativeCalls.push(command);
              await window.__recordNativeCall(command);
              if (command === 'get_public_config') return { cloud_base_url: base };
              if (command === 'local_publication_draft_get') return { payload: null, revision: '0', saved_at: null, schema_version: 1 };
              if (command === 'local_agent_token_facts') return { configured: true };
              if (command === 'local_saved_file_states') return {};
              return null;
            },
            transformCallback: () => 1,
            unregisterCallback: () => {},
            metadata: { currentWindow: { label: 'main' }, currentWebview: { label: 'main' } },
          };
        }, { base });
        await page.route('**/api/v1/**', async route => {
          const request = route.request(), url = new URL(request.url()), p = url.pathname;
          calls.push(p);
          if (request.method() !== 'GET') {
            writes.push(p);
            return route.fulfill({ status: 400, json: { errcode: 18001, message: '隔离走查不写入业务', data: null } });
          }
          let data;
          if (p === '/api/v1/auth/me') data = { id: '1', username: 'review', nickname: '走查用户', role, category_ids: ['1'], team_id: '1' };
          else if (p.endsWith('/categories')) data = [{ id: '1', name: '游戏', enabled: true }];
          else if (p === '/api/v1/publication') data = { items: tasks, total: tasks.length, summary: { all: tasks.length } };
          else if (/\/publication\/\d+$/.test(p)) data = tasks.find(task => p.endsWith('/' + task.id));
          else if (p.endsWith('/attempt')) data = [];
          else if (p === '/api/v1/compose-output-materials') data = { items: [{ material_id: '20', material_title: '走查素材', category_id: '1', output_count: 1 }], total: 1 };
          else if (p === '/api/v1/compose-outputs/130') data = { output };
          else if (p === '/api/v1/compose-outputs' || p === '/api/v1/publication/output') data = { items: [output], total: 1 };
          else if (p === '/api/v1/file-transfer-tasks') data = { items: [], total: 0 };
          else if (['/api/v1/compose-operators', '/api/v1/operation-teams', '/api/v1/compose-templates'].includes(p)) data = [];
          else data = { items: [], total: 0 };
          await route.fulfill({ json: { errcode: 0, message: 'ok', data } });
        });
        try {
          await page.goto(base + '/publish');
          await page.getByText('走查任务1', { exact: true }).waitFor();
          if (role === 'admin') {
            for (const name of ['快速发布', '执行', '重试', '去处理', '复制']) assert.equal(await page.getByRole('button', { name, exact: true }).count(), 0, `${app}: admin ${name}`);
            for (const name of ['编辑', '终止', '放弃', '核实结果', '补录结果']) assert(await page.getByRole('button', { name, exact: true }).count() > 0, `${app}: preserve ${name}`);
            await page.getByRole('button', { name: '详情', exact: true }).first().click();
            await page.getByRole('heading', { name: '发布任务详情', exact: true }).waitFor();
            await page.goto(base + '/publish?output_id=130');
            await page.getByText('走查任务1', { exact: true }).waitFor();
            assert.equal(await page.getByRole('heading', { name: '快速发布', exact: true }).count(), 0);
            assert(!calls.includes('/api/v1/compose-outputs/130'), 'admin deep link must not initialize publication preparation');
            await page.screenshot({ path: path.join(evidence, `${app}-admin-publication.png`), fullPage: true });
            await page.goto(base + '/finished-media');
            await page.getByText('走查素材', { exact: true }).waitFor();
            assert.equal(await page.getByRole('button', { name: '去发布', exact: true }).count(), 0);
            await page.getByRole('button', { name: '详情', exact: true }).first().click();
            await page.getByRole('dialog', { name: '素材成片详情', exact: true }).waitFor();
            await page.getByText('走查成片', { exact: true }).waitFor();
            assert.equal(await page.getByRole('button', { name: '去发布', exact: true }).count(), 0, 'admin material drawer publication hidden');
            await page.screenshot({ path: path.join(evidence, `${app}-admin-output.png`), fullPage: true });
            if (app === 'desktop') {
              assert(!nativeCalls.some(command => /publication|profile_open|local_agent_start/.test(command)), JSON.stringify(nativeCalls));
            }
            checks.push(`${app}: admin Cloud management retained, publication entries/deep link/Agent/draft blocked`);
          } else {
            for (const name of ['快速发布', '执行', '重试', '去处理', '复制']) assert(await page.getByRole('button', { name, exact: true }).count() > 0, `${app}: ${role} ${name}`);
            for (const [id, names] of [['7',['执行']], ['8',['重试','复制']], ['9',['去处理']], ['10',['核实结果']], ['11',['终止']]]) {
              const row=page.getByRole('row').filter({hasText:`走查任务${id}`});
              for (const name of names) {
                const button=row.getByRole('button',{name,exact:true});
                assert.equal(await button.isEnabled(),false,`${app}: ${role} member ${id} ${name}`);
                assert.match(await button.getAttribute('title'), /任务所属运营/);
              }
            }
            assert.equal(await page.getByRole('row').filter({hasText:'走查任务1'}).filter({hasNotText:'走查任务10'}).filter({hasNotText:'走查任务11'}).getByRole('button',{name:'执行',exact:true}).isEnabled(),true);
            const ownHuman=page.getByRole('row').filter({hasText:'走查任务3'}).getByRole('button',{name:'去处理',exact:true});
            assert.equal(await ownHuman.isEnabled(),app==='desktop');
            checks.push(`${app}: ${role} own execution retained; member execution/retry/copy/scene/uncertain verification/running stop disabled with reasons`);
            await page.screenshot({path:path.join(evidence,`${app}-${role}-ownership.png`),fullPage:true});
            await page.getByRole('button', { name: '快速发布', exact: true }).click();
            await page.getByRole('heading', { name: '快速发布', exact: true }).waitFor();
            if (app === 'desktop') assert(nativeCalls.includes('local_publication_draft_get'));
            await page.goto(base + '/publish?output_id=130');
            await page.getByRole('heading', { name: '快速发布', exact: true }).waitFor();
            assert(calls.includes('/api/v1/compose-outputs/130'), 'operator source link loads the selected output');
            checks.push(`${app}: ${role} can prepare publication and open source output using expected draft storage`);
          }
          assert.deepEqual(errors, []);
          assert.deepEqual(writes, []);
        } catch (error) {
          await page.screenshot({ path: path.join(evidence, `${app}-${role}-failure.png`), fullPage: true }).catch(() => {});
          console.error(JSON.stringify({ app, role, errors, nativeCalls, recentCalls: calls.slice(-12) }));
          throw error;
        } finally { await context.close(); }
      }
    } finally { await server.close(); }
  }
  await fs.writeFile(path.join(evidence, 'checks.json'), JSON.stringify({ fixtureOnly: true, checks }, null, 2));
  console.log(JSON.stringify({ checks, fixtureOnly: true, evidence }));
} finally { await browser.close(); }
