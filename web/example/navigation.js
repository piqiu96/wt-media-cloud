import { localPreviewLink } from './preview-links.js'

document.addEventListener('click', event => {
  const anchor = event.target.closest?.('a[href]')
  if (!anchor || event.defaultPrevented || anchor.hasAttribute('download')) return
  const original = anchor.href
  const mapped = localPreviewLink(original)
  if (mapped.startsWith('/')) anchor.href = mapped
  else if (new URL(original).origin === location.origin && new URL(original).pathname.endsWith('.md')) {
    const url = new URL(original)
    if (url.pathname.startsWith('/example/')) anchor.href = `/__docs/cloud/web${url.pathname}${url.search}${url.hash}`
  }
}, true)
