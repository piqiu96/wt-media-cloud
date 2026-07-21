import { createApiClient } from './http.js'

export function createSessionClient({ base = '/api/v1', fetch = globalThis.fetch } = {}) {
  const api = createApiClient({ base, fetchImpl: fetch })
  return {
    async login(username, password) {
      return api.post('/auth/login', { username, password })
    },
    async me() {
      return api.get('/auth/me')
    },
    async logout() {
      return api.post('/auth/logout')
    },
  }
}
