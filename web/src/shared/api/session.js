import { createApiClient } from './http.js'

const api = createApiClient()

export function createSessionClient() {
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
