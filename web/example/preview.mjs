import { readFile, realpath, readdir } from 'node:fs/promises'
import path from 'node:path'
import { Marked } from 'marked'
import { localPreviewLink } from './preview-links.js'

const escapeHTML = value => String(value).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c])
const remoteURL = (repo, file) => `https://github.com/piqiu96/wt-media-${repo}/blob/main/${file}`
const types = { '.md': 'text/html; charset=utf-8', '.png': 'image/png', '.jpg': 'image/jpeg', '.jpeg': 'image/jpeg', '.gif': 'image/gif', '.webp': 'image/webp', '.svg': 'image/svg+xml' }
const inside = (root, file) => file === root || file.startsWith(root + path.sep)

export function documentHTML(markdown, repo, file) {
  const counts = new Map()
  const safeLink = href => {
    const mapped = localPreviewLink(href, remoteURL(repo, file))
    return mapped?.startsWith('/') || /^(https?:|mailto:|#)/i.test(mapped || '') ? mapped : '#'
  }
  const parser = new Marked({ renderer: {
    html({ text }) { return escapeHTML(text) },
    link({ href, tokens, title }) {
      return `<a href="${escapeHTML(safeLink(href))}"${title ? ` title="${escapeHTML(title)}"` : ''}>${this.parser.parseInline(tokens)}</a>`
    },
    image({ href, text }) { return `<img src="${escapeHTML(safeLink(href))}" alt="${escapeHTML(text)}">` },
    heading({ tokens, depth, text }) {
      const slug = text.toLowerCase().replace(/[`*]/g, '').replace(/[^\p{L}\p{N}_\-\s]/gu, '').replace(/\s/g, '-')
      const count = counts.get(slug) || 0; counts.set(slug, count + 1)
      return `<h${depth} id="${escapeHTML(slug + (count ? `-${count}` : ''))}">${this.parser.parseInline(tokens)}</h${depth}>`
    },
  } })
  return `<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>${escapeHTML(file)}</title><style>body{margin:0;font:15px/1.75 system-ui;color:#243047;background:#fff}header{padding:12px 24px;background:#f3f6fa;border-bottom:1px solid #e5e8ed}main{max-width:1100px;margin:auto;padding:24px}a{color:#1765d1}pre{overflow:auto;padding:16px;background:#f3f6fa}code{font-size:13px}table{border-collapse:collapse;display:block;overflow:auto}th,td{border:1px solid #dfe4eb;padding:8px 12px}img{max-width:100%}h1,h2,h3,h4{scroll-margin-top:16px}</style><header>本地工作区 · 保存后刷新即可读取未提交修改　<a href="/example/">原型入口</a>　<a href="${escapeHTML(remoteURL(repo, file))}">查看 GitHub 已推送版本</a></header><main>${parser.parse(markdown)}</main></html>`
}

export function createDocumentationMiddleware({ workspaceRoot, cloudRoot }) {
  return async (req, res, next) => {
    const url = new URL(req.url, 'http://localhost')
    if (url.pathname === '/' || url.pathname === '/index.html') {
      res.statusCode = 302; res.setHeader('Location', '/example/'); res.end(); return
    }
    if (url.pathname === '/__example/context') {
      res.setHeader('Content-Type', 'application/json'); res.setHeader('Cache-Control', 'no-store')
      res.end(JSON.stringify({ localWorkspace: Boolean(workspaceRoot) })); return
    }
    if (/^\/api(?:\/|$)/.test(url.pathname)) { res.statusCode = 403; res.end('原型预览仅支持演示操作'); return }
    if (!url.pathname.startsWith('/__docs/')) return next()
    if (!['GET', 'HEAD'].includes(req.method)) { res.statusCode = 405; res.end(); return }
    res.setHeader('Cache-Control', 'no-store')
    res.setHeader('X-Content-Type-Options', 'nosniff')
    res.setHeader('Content-Security-Policy', "default-src 'none'; img-src 'self' https:; style-src 'unsafe-inline'; base-uri 'none'; frame-ancestors 'self'")
    let repo, file
    try {
      const parts = decodeURIComponent(url.pathname.slice('/__docs/'.length)).split('/')
      repo = parts.shift(); file = parts.join('/')
      const root = repo === 'workspace' ? workspaceRoot : repo === 'cloud' ? cloudRoot : null
      const prefix = repo === 'workspace' ? 'docs/' : 'web/example/'
      if (!['workspace', 'cloud'].includes(repo) || !file.startsWith(prefix) || file.includes('\\') || file.split('/').some(p => p === '..' || p.startsWith('.'))) throw new Error('invalid path')
      if (!types[path.extname(file).toLowerCase()]) throw new Error('unsupported file')
      if (!root) {
        res.statusCode = 503; res.setHeader('Content-Type', 'text/html; charset=utf-8')
        res.end(`本地 Workspace 文档未挂载。请从 Workspace 启动预览，或<a href="${escapeHTML(remoteURL(repo, file))}">查看 GitHub 已推送版本</a>。`); return
      }
      const allowedRoot = await realpath(path.join(root, prefix))
      const actual = await realpath(path.join(root, file))
      if (!inside(allowedRoot, actual)) throw new Error('outside documentation root')
      const bytes = await readFile(actual)
      res.setHeader('Content-Type', types[path.extname(file).toLowerCase()])
      res.end(req.method === 'HEAD' ? '' : file.endsWith('.md') ? documentHTML(bytes.toString('utf8'), repo, file) : bytes)
    } catch {
      res.statusCode = 404; res.setHeader('Content-Type', 'text/plain; charset=utf-8')
      res.end('本地文档不存在或不在允许的文档范围内；请检查引用。')
    }
  }
}

export const injectNavigation = html => html.replace('</head>', '<script type="module" src="/example/navigation.js"></script></head>')

export function examplePreviewPlugin({ webRoot, workspaceRoot }) {
  const middleware = createDocumentationMiddleware({ workspaceRoot, cloudRoot: path.dirname(webRoot) })
  return {
    name: 'wt-example-preview',
    configureServer(server) { server.middlewares.use(middleware) },
    configurePreviewServer(server) { server.middlewares.use(middleware) },
    transformIndexHtml: {
      order: 'pre',
      handler(html, context) {
        // Keep legacy global scripts/styles in their own document. The Vue entry is bundled normally.
        return context.path.startsWith('/example/') && !html.includes('/example/main.ts') ? injectNavigation(html) : html
      },
    },
    async generateBundle() {
      const root = path.join(webRoot, 'example')
      const walk = async directory => {
        for (const entry of await readdir(directory, { withFileTypes: true })) {
          if (entry.name === 'tests') continue
          const full = path.join(directory, entry.name)
          if (entry.isDirectory()) { await walk(full); continue }
          const relative = path.relative(root, full).split(path.sep).join('/')
          if (relative === 'index.html' || !/\.(html|css|js|png|svg)$/.test(relative) && !relative.endsWith('LICENSE')) continue
          let source = await readFile(full)
          if (relative.endsWith('.html')) source = injectNavigation(source.toString('utf8'))
          this.emitFile({ type: 'asset', fileName: `example/${relative}`, source })
        }
      }
      await walk(root)
    },
  }
}
