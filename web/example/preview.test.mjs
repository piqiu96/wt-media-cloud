import assert from 'node:assert/strict'
import { test } from 'node:test'
import { mkdtemp, mkdir, writeFile, symlink, rm } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import path from 'node:path'
import http from 'node:http'
import { createDocumentationMiddleware, documentHTML } from './preview.mjs'
import { localPreviewLink } from './preview-links.js'

test('floating references use local documents; immutable and unrelated references remain remote', () => {
  assert.equal(localPreviewLink('https://github.com/piqiu96/wt-media-workspace/blob/main/docs/design/publishing.md#目标'), '/__docs/workspace/docs/design/publishing.md#%E7%9B%AE%E6%A0%87')
  const pinned = 'https://github.com/piqiu96/wt-media-workspace/blob/abc123/docs/README.md'
  assert.equal(localPreviewLink(pinned), pinned)
  const feature = 'https://github.com/piqiu96/wt-media-workspace/blob/chg/CHG-20261109-127/docs/README.md'
  assert.equal(localPreviewLink(feature), feature)
  assert.equal(localPreviewLink('http://127.0.0.1:8768/example/proto/content-operations/design/publishing.md'), '/__docs/workspace/docs/design/publishing.md')
  assert.equal(localPreviewLink('https://github.com/piqiu96/wt-media-cloud/blob/main/web/example/proto/social-accounts/index.html?scene=accounts#list'), '/example/proto/social-accounts/index.html?scene=accounts#list')
})

test('document links, anchors and images resolve; raw HTML and executable links cannot execute', () => {
  const html = documentHTML('# 业务规则\n\n[规范](../standards/frontend/README.md#规则)\n\n![示意](diagram.png)\n\n<script>alert(1)</script>\n\n[x](javascript:alert)\n\n# 业务规则', 'workspace', 'docs/design/publishing.md')
  assert.match(html, /id="业务规则"/); assert.match(html, /id="业务规则-1"/)
  assert.match(html, /href="\/__docs\/workspace\/docs\/standards\/frontend\/README.md#/)
  assert.match(html, /src="\/__docs\/workspace\/docs\/design\/diagram.png"/)
  assert.ok(!html.includes('<script>')); assert.ok(!html.includes('href="javascript:'))
})

test('HTTP reader reflects saved edits, blocks traversal/symlinks/non-documents and never proxies APIs', async t => {
  const root = await mkdtemp(path.join(tmpdir(), 'wt-example-docs-'))
  t.after(() => rm(root, { recursive: true, force: true }))
  await mkdir(path.join(root, 'docs/design'), { recursive: true })
  const file = path.join(root, 'docs/design/page.md')
  await writeFile(file, '# 本地初稿')
  await writeFile(path.join(root, 'secret.md'), 'must not leak')
  await symlink(path.join(root, 'secret.md'), path.join(root, 'docs/design/outside.md'))
  const middleware = createDocumentationMiddleware({ workspaceRoot: root, cloudRoot: root })
  const server = http.createServer((req, res) => middleware(req, res, () => { res.statusCode = 404; res.end() }))
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve))
  t.after(() => new Promise(resolve => server.close(resolve)))
  const base = `http://127.0.0.1:${server.address().port}`
  for (const entry of ['/', '/index.html']) {
    const redirect = await fetch(base + entry, { redirect: 'manual' })
    assert.equal(redirect.status, 302); assert.equal(redirect.headers.get('location'), '/example/')
  }
  let response = await fetch(base + '/__docs/workspace/docs/design/page.md')
  assert.equal(response.status, 200); assert.match(await response.text(), /本地初稿/)
  assert.equal(response.headers.get('cache-control'), 'no-store')
  await writeFile(file, '# 未提交的新内容')
  assert.match(await (await fetch(base + '/__docs/workspace/docs/design/page.md')).text(), /未提交的新内容/)
  for (const suffix of ['docs/design/outside.md', 'docs/design/..%2F..%2Fsecret.md', 'config/credentials.md', 'docs/design/run.js']) {
    assert.equal((await fetch(base + '/__docs/workspace/' + suffix)).status, 404)
  }
  assert.equal((await fetch(base + '/api/files', { method: 'POST' })).status, 403)
  assert.equal((await fetch(base + '/__docs/workspace/docs/design/page.md', { method: 'PUT' })).status, 405)
})

test('missing Workspace is explicit rather than silently serving a remote revision', async t => {
  const middleware = createDocumentationMiddleware({ cloudRoot: tmpdir() })
  const server = http.createServer((req, res) => middleware(req, res, () => res.end()))
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve))
  t.after(() => new Promise(resolve => server.close(resolve)))
  const response = await fetch(`http://127.0.0.1:${server.address().port}/__docs/workspace/docs/design/README.md`)
  assert.equal(response.status, 503)
  assert.match(await response.text(), /未挂载/)
})
