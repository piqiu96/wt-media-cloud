// Shared by the development document reader and legacy HTML navigation.
// Only floating main-branch references are redirected; fixed revisions stay remote.
export function localPreviewLink(href, base) {
  if (!href || href.startsWith('#')) return href
  let url
  try { url = new URL(href, base) } catch { return href }
  // Approved scripts still emit these old relative links. Resolve them here
  // instead of changing their frozen rendering/interaction implementations.
  if (['127.0.0.1', 'localhost'].includes(url.hostname)) {
    const old = url.pathname.match(/^\/example\/proto\/(content-operations|content-mining)\/design\/(.+)\.md$/)
    if (old) {
      const names = old[1] === 'content-mining' ? { analysis: 'content-mining' } : { README: 'content-operations', publishing: 'publishing', works: 'works', interaction: 'interaction', 'content-production': 'content-production' }
      if (names[old[2]]) return `/__docs/workspace/docs/design/${names[old[2]]}.md${url.search}${url.hash}`
    }
  }
  if (url.hostname !== 'github.com' || url.protocol !== 'https:') return url.href
  const match = url.pathname.match(/^\/piqiu96\/(wt-media-workspace|wt-media-cloud)\/(blob|tree)\/main\/(.*)$/)
  if (!match) return url.href
  const [, repo, kind, path] = match
  const suffix = url.search + url.hash
  if (repo === 'wt-media-workspace' && path.startsWith('docs/')) {
    return `/__docs/workspace/${path}${kind === 'tree' ? '/README.md' : ''}${suffix}`
  }
  if (repo === 'wt-media-cloud' && (path === 'web/example' || path.startsWith('web/example/'))) {
    if (path.endsWith('.md') || kind === 'tree') return `/__docs/cloud/${path}${kind === 'tree' ? '/README.md' : ''}${suffix}`
    return `/${path.slice(4)}${suffix}`
  }
  return url.href
}
