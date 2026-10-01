import { createApiClient } from './http.js'

export function createDeviceBindingClient({ base = '/api/v1', fetch = globalThis.fetch } = {}) {
  const api = createApiClient({ base, fetchImpl: fetch })
  return {
    get() { return api.get('/local-agent/device-binding') },
    unbind() { return api.delete('/local-agent/device-binding') },
  }
}
