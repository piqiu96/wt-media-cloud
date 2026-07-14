export function createSessionClient({ fetch = globalThis.fetch } = {}) {
  async function read(response) {
    const payload = await response.json()
    if (!response.ok) {
      const error = new Error(payload?.error?.message || '请求失败')
      error.code = payload?.error?.code || 'request_failed'
      throw error
    }
    return payload.data
  }

  return {
    async login(username, password) {
      const response = await fetch('/api/v1/auth/login', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password }),
      })
      return read(response)
    },

    async me() {
      return read(await fetch('/api/v1/auth/me', { credentials: 'include' }))
    },

    async logout() {
      return read(await fetch('/api/v1/auth/logout', { method: 'POST', credentials: 'include' }))
    },
  }
}
