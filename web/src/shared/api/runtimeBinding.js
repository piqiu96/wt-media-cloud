import { createApiClient } from './http.js'

export function createRuntimeBindingClient({ base = '/api/v1', fetch = globalThis.fetch } = {}) {
  const api = createApiClient({ base, fetchImpl: fetch })
  return {
    createBindingTicket() {
      return api.post('/local-agent/binding-tickets', {})
    },
  }
}
