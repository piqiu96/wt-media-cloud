import { createApiClient } from './http.js'

export function createContentPoolClient({ base = '/api/v1', fetch = globalThis.fetch } = {}) {
  const api = createApiClient({ base, fetchImpl: fetch })
  return {
    list(params) { return api.get('/content-pool', params) },
    get(id) { return api.get(`/content-pool/${id}`) },
    setStatus(id, status, reason = '') { return api.post(`/content-pool/${id}/status`, { status, reason }) },
    batchSetStatus(ids, status, reason = '') { return api.post('/content-pool/batch/status', { ids, status, reason }) },
    materialize(id) { return api.post(`/content-pool/${id}/materialize`, {}) },
  }
}
