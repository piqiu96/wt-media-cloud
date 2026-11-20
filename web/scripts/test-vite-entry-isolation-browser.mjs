// Isolated development entries and API fixtures only; never uses a real session.
import assert from 'node:assert/strict';
import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';
import { chromium } from 'playwright';

const root = fileURLToPath(new URL('..', import.meta.url));
const evidence = process.env.WT_MEDIA_BROWSER_EVIDENCE || '/tmp/wt-media-vite-entry-isolation';
await mkdir(evidence, { recursive: true });
const servers = [], contexts = [], results = [];
let browser;
try {
  for (const [entry, configFile, port] of [['cloud', 'vite.config.js', 18873], ['desktop', 'vite.config.desktop.js', 18874]]) {
    const server = await createServer({ root, configFile: path.join(root, configFile), mode: 'cache-regression', server: { host: '127.0.0.1', port, strictPort: true, open: false, hmr: false } });
    servers.push({ entry, server, base: `http://127.0.0.1:${port}` });
    await server.listen();
  }
  assert.notEqual(servers[0].server.config.cacheDir, servers[1].server.config.cacheDir);
  browser = await chromium.launch({ headless: true, executablePath: process.env.CHROMIUM_EXECUTABLE });
  for (const { entry, server, base } of servers) {
    const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } });
    contexts.push(context);
    if (entry === 'desktop') await context.addInitScript(({ base }) => {
      // Native configuration is the only shell capability this page needs.
      window.__TAURI_INTERNALS__ = { invoke: async command => command === 'get_public_config' ? { cloud_base_url: base, cloud_env: 'dev' } : {} };
    }, { base });
    await context.route('**/api/v1/**', async route => {
      assert.equal(route.request().method(), 'GET', 'This regression must never send a business write');
      const url = new URL(route.request().url());
      const data = url.pathname.endsWith('/auth/me') ? { id: '1', role: 'admin', username: 'cache-regression' }
        : url.pathname.endsWith('/categories') ? []
        : { items: [], total: 0, summary: { all: 0 } };
      await route.fulfill({ json: { errcode: 0, message: 'ok', data } });
    });
    const page = await context.newPage(), errors = [];
    page.on('pageerror', error => errors.push(error.message));
    page.on('console', message => { if (/Missing ref owner|withDirectives|reading ['"]slots/.test(message.text())) errors.push(message.text()); });
    for (const phase of ['cold', 'reload']) {
      const runtimes = new Set(), pending = [];
      const inspect = response => {
        if (!response.url().includes('/node_modules/.vite/') || !new URL(response.url()).pathname.endsWith('.js')) return;
        pending.push(response.text().then(source => {
          if (source.includes('@vue/runtime-core/dist/runtime-core.esm-bundler.js')) runtimes.add(new URL(response.url()).pathname);
        }));
      };
      page.on('response', inspect);
      if (phase === 'cold') await page.goto(base + '/publish'); else await page.reload();
      await page.getByRole('heading', { name: '发布任务', exact: true }).waitFor();
      await page.getByRole('button', { name: '快速发布', exact: true }).click();
      await page.getByRole('heading', { name: '快速发布', exact: true }).waitFor();
      await page.waitForLoadState('networkidle');
      await Promise.all(pending);
      page.off('response', inspect);
      assert.equal(runtimes.size, 1, `${entry} ${phase} must load exactly one Vue runtime: ${[...runtimes]}`);
      assert.deepEqual(errors, [], `${entry} ${phase} must render without Vue instance errors`);
      await page.screenshot({ path: path.join(evidence, `${entry}-${phase}.png`), fullPage: true });
      results.push({ entry, phase, cacheDir: server.config.cacheDir, runtimes: [...runtimes], pageErrors: [...errors] });
    }
  }
  await writeFile(path.join(evidence, 'checks.json'), JSON.stringify(results, null, 2));
  console.log(JSON.stringify({ checks: results, fixtureOnly: true, businessWrites: 0 }, null, 2));
} finally {
  await Promise.all(contexts.map(context => context.close()));
  await browser?.close();
  await Promise.all(servers.map(({ server }) => server.close()));
}
