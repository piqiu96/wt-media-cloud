import { createApiClient, setSessionToken } from './http.js'

export function createSessionClient({ base = '/api/v1', fetch = globalThis.fetch } = {}) {
  const api = createApiClient({ base, fetchImpl: fetch })
  return {
    async login(username, password, options = {}) {
      const data = await api.post('/auth/login', {
        username,
        password,
        replace_existing: !!options.replaceExisting,
      })
      // Packaged Desktop login returns { user, token }; store the token for the
      // X-Session-Token header. Cloud Web returns the user directly (cookie auth).
      if (data && data.token) {
        setSessionToken(data.token)
        return data.user
      }
      return data
    },
    async me() {
      return api.get('/auth/me')
    },
    async logout() {
      setSessionToken('')
      return api.post('/auth/logout')
    },
  }
}
