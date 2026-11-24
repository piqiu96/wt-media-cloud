import assert from 'node:assert/strict'
import { createServer } from 'vite'
import { chromium } from 'playwright'
import { mkdtemp } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('../../', import.meta.url))
const desktop = process.argv.includes('--desktop')
const output = await mkdtemp(path.join(tmpdir(), 'wt-image-filter-'))
const server = await createServer({
  root, configFile: path.join(root, desktop ? 'vite.config.desktop.js' : 'vite.config.js'), cacheDir: path.join(output, '.vite'),
  server: { host: '127.0.0.1', port: 0, strictPort: false, open: false },
})
let browser
try {
  await server.listen()
  const base = server.resolvedUrls.local[0]
  browser = await chromium.launch({ channel: 'chrome', headless: true })
  for (const width of [1024, 1280, 1440, 1600]) {
    const page = await browser.newPage({ viewport: { width, height: 1000 } })
    page.setDefaultTimeout(10000)
    const errors = [], requests = []
    page.on('pageerror', error => errors.push(error.message))
    await page.route('**/api/v1/**', async route => {
      const req = route.request(), url = new URL(req.url()), name = url.pathname.replace('/api/v1', '')
      assert.equal(req.method(), 'GET', 'verification must remain read-only')
      let data = { items: [], total: 0 }
      if (name === '/auth/me') data = { id: 3, username: 'operator01', role: 'operator', category_ids: [1] }
      else if (name === '/categories') data = [{ id: 1, name: '游戏' }]
      else if (name === '/files') data = { items: [], total: 0, upload_max_bytes: 1000000 }
      else if (name === '/image-asset') requests.push(url.searchParams.toString())
      await route.fulfill({ json: { errcode: 0, message: 'ok', data } })
    })
    if (desktop) await page.addInitScript(({ base }) => {
      window.__TAURI_INTERNALS__ = { invoke: async command => {
        if (command === 'get_public_config') return { cloud_base_url: base, environment: 'dev' }
        if (command === 'plugin:app|version') return '0.1.0'
        if (command === 'local_agent_status') return { state: 'ready' }
        return null
      }, transformCallback: () => 1 }
    }, { base })
    await page.goto(`${base}image-library`)
    await page.getByRole('heading', { name: '图片库', exact: true }).waitFor()
    await page.getByText('还没有图片', { exact: true }).waitFor()
    const bar = page.locator('.image-library-page .resource-filter-bar')
    const toggle = bar.getByRole('button', { name: /高级筛选/ })
    const layout = await bar.evaluate(node => {
      const box = el => { const r = el.getBoundingClientRect(); return { x:r.x, y:r.y, bottom:r.bottom, height:r.height } }
      return {
        fields: [...node.querySelectorAll(':scope > .wt-filter-field')].map(box),
        actions: box(node.querySelector('.wt-filter-actions')),
        toggle: box(node.querySelector('.resource-advanced-toggle')),
        columns: getComputedStyle(node).gridTemplateColumns.split(' ').length,
        overflow: document.documentElement.scrollWidth > innerWidth,
      }
    })
    assert.equal(layout.overflow, false, `page overflow at ${width}`)
    assert.ok(Math.abs(layout.fields.at(-1).bottom - layout.actions.bottom) <= 1, `actions baseline ${width}: ${JSON.stringify(layout)}`)
    assert.ok(Math.abs(layout.fields[0].x - layout.toggle.x) <= 1, `advanced left alignment ${width}`)
    assert.ok(Math.abs(layout.toggle.y - layout.fields.at(-1).bottom - 20) <= 1, `advanced row gap ${width}: ${JSON.stringify(layout)}`)
    if (width >= 1440) assert.equal(layout.columns, 6, `wide layout ${width}`)
    const before = requests.length
    await toggle.click()
    const advanced = page.locator('#image-library-advanced-filters')
    await advanced.waitFor()
    assert.equal(requests.length, before, 'expanding must not query')
    assert.equal(await advanced.evaluate(node => Math.round(node.getBoundingClientRect().top - node.previousElementSibling.getBoundingClientRect().bottom)), 16)
    await advanced.locator('.t-select').click()
    await page.locator('.t-select-option').filter({ hasText: /^上传图片$/ }).click()
    assert.equal(requests.length, before, 'draft must not query')
    assert.equal(await toggle.getByText(/（.*）/).count(), 0, 'draft must not count as applied')
    const queried = page.waitForResponse(response => new URL(response.url()).pathname.endsWith('/image-asset') && new URL(response.url()).searchParams.get('source') === 'upload')
    await bar.getByRole('button', { name: '查询', exact: true }).click()
    await queried
    await toggle.getByText(/（(?:已生效 )?1(?: 项)?）/).waitFor()
    assert.ok(requests.at(-1).includes('source=upload'))
    await toggle.click()
    await advanced.waitFor({ state: 'detached' })
    assert.match(await toggle.innerText(), /（(?:已生效 )?1(?: 项)?）/)
    const resetQuery = page.waitForResponse(response => new URL(response.url()).pathname.endsWith('/image-asset') && !new URL(response.url()).searchParams.has('source'))
    await bar.getByRole('button', { name: '重置', exact: true }).click()
    await resetQuery
    await toggle.getByText(/（.*）/).waitFor({ state: 'detached' })
    assert.ok(!requests.at(-1).includes('source=upload'))
    await page.mouse.move(0, 0)
    await page.screenshot({ path: path.join(output, `${width}-collapsed.png`), fullPage: true, animations: 'disabled' })
    await toggle.focus()
    await page.keyboard.press('Enter')
    await advanced.waitFor()
    assert.equal(await toggle.getAttribute('aria-expanded'), 'true')
    assert.deepEqual(errors, [])
    await page.mouse.move(0, 0)
    await page.screenshot({ path: path.join(output, `${width}-expanded.png`), fullPage: true, animations: 'disabled' })
    console.log(`PASS ${desktop ? 'Desktop' : 'Web'} ${width}: ${layout.columns} columns, aligned actions, 20/16px gaps; expand/query/count/reset/keyboard`)
    await page.close()
  }
  console.log('Screenshots:', output)
} finally {
  await browser?.close()
  await server.close()
}
