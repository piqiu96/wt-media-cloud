/**
 * Unified HTTP client for WT Media Cloud API.
 *
 * All Cloud API responses follow:
 *   { "errcode": 0, "message": "success", "data": ..., "logid": "..." }
 *
 * On error (errcode !== 0 or HTTP error):
 *   Throws an ApiError with errcode, message, logid, and optional details.
 */

export class ApiError extends Error {
  constructor({ errcode = 99999, message = '未知错误', logid = '', type = '', details = null, retryable = false }) {
    super(message)
    this.name = 'ApiError'
    this.errcode = errcode
    this.logid = logid
    this.type = type
    this.details = details
    this.retryable = retryable
  }
}

async function parseResponse(response) {
  let body
  try {
    body = await response.json()
  } catch {
    throw new ApiError({ errcode: 10001, message: '服务器返回格式错误' })
  }

  // Only the unified format is accepted: { errcode, message, data, logid, error? }
  if (!body || typeof body !== 'object' || !('errcode' in body)) {
    throw new ApiError({ errcode: 10001, message: '服务器响应格式错误' })
  }
  if (body.errcode === 0) {
    return body.data
  }
  const apiErr = body.error || {}
  throw new ApiError({
    errcode: body.errcode,
    message: body.message || '请求失败',
    logid: body.logid || '',
    type: apiErr.type || '',
    details: apiErr.details || null,
    retryable: !!apiErr.retryable,
  })
}

// Session token for the packaged Desktop WebView. The Desktop page runs on
// http://tauri.localhost, cross-site to the configured Cloud API, so the WebView does
// not round-trip the HttpOnly session cookie; the token is carried in the
// X-Session-Token header instead. Cloud Web keeps the cookie path (token empty).
//
// The Desktop app uses history-mode routing, so any full page load resets module
// state; persist the token in localStorage on Desktop and restore it on load so
// navigations/reloads do not silently log the operator out.
const SESSION_TOKEN_KEY = 'wt_media_desktop_session_token'
function isDesktopApp() {
  return typeof window !== 'undefined' && window.__WT_MEDIA_APP__ === 'desktop'
}
let sessionToken = ''
export function setSessionToken(token) {
  sessionToken = token
  if (isDesktopApp()) {
    try {
      if (token) localStorage.setItem(SESSION_TOKEN_KEY, token)
      else localStorage.removeItem(SESSION_TOKEN_KEY)
    } catch {
      // localStorage unavailable; token stays in-memory for this load.
    }
  }
}
export function getSessionToken() {
  return sessionToken
}
if (isDesktopApp()) {
  try {
    sessionToken = localStorage.getItem(SESSION_TOKEN_KEY) || ''
  } catch {
    sessionToken = ''
  }
}

let desktopCloudOrigin = ''

export function setDesktopCloudOrigin(origin) {
  desktopCloudOrigin = typeof origin === 'string' ? origin.trim().replace(/\/$/, '') : ''
}

function defaultApiBase(base) {
  if (base !== '/api/v1') return base
  const win = typeof window !== 'undefined' ? window : null
  if (win?.__WT_MEDIA_APP__ !== 'desktop') return base
  if (win.location?.port === '5174') return base
  if (!desktopCloudOrigin) throw new Error('Cloud 地址尚未就绪，请重新启动客户端或检查本机配置')
  return `${desktopCloudOrigin}/api/v1`
}

export function createApiClient({ base = '/api/v1', fetchImpl = globalThis.fetch } = {}) {
  async function request(path, { method = 'GET', body, params } = {}) {
    const apiBase = defaultApiBase(base)
    let url = `${apiBase}${path}`
    if (params) {
      const q = new URLSearchParams()
      for (const [k, v] of Object.entries(params)) {
        if (v !== undefined && v !== null && v !== '') q.set(k, v)
      }
      const suffix = q.toString()
      if (suffix) url += `?${suffix}`
    }

    const options = {
      method,
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
    }
    if (sessionToken) options.headers['X-Session-Token'] = sessionToken
    if (body !== undefined) options.body = JSON.stringify(body)
    if (method === 'GET' || method === 'DELETE') delete options.headers['Content-Type']

    const response = await fetchImpl(url, options)
    if (response.status === 204) {
      return null
    }

    // 401 → redirect to login (skip for login endpoint itself). Clear any
    // stale Desktop token so it does not persist and loop on reload.
    //
    // This must come before parseResponse: the backend answers an unauthenticated
    // request with HTTP 401 *and* a structured body whose errcode is non-zero
    // (e.g. 11001), which parseResponse throws on first — a redirect placed after
    // it could never run.
    //
    // Never navigate while already on /login. The login page probes the session on
    // mount and 401s when logged out; navigating back to /login reloads the page,
    // which probes again — the login page would never stay put and its form could
    // never be submitted.
    const win = typeof window === 'undefined' ? null : window
    const onLoginPage = win?.location?.pathname === '/login'
    if (response.status === 401 && !path.startsWith('/auth/login') && !onLoginPage) {
      setSessionToken('')
      const returnUrl = encodeURIComponent(win.location.pathname + win.location.search)
      win.location.href = `/login?redirect=${returnUrl}`
    }

    const result = await parseResponse(response)

    // HTTP error without structured body
    if (!response.ok && result === null) {
      throw new ApiError({ errcode: 50000, message: `HTTP ${response.status}` })
    }

    return result
  }

  return {
    get(path, params) { return request(path, { params }) },
    post(path, body) { return request(path, { method: 'POST', body }) },
    put(path, body) { return request(path, { method: 'PUT', body }) },
    patch(path, body) { return request(path, { method: 'PATCH', body }) },
    delete(path) { return request(path, { method: 'DELETE' }) },
  }
}
