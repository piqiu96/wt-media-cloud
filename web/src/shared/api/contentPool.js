import { createApiClient } from './http.js'

export function createContentPoolClient({ base = '/api/v1', fetch = globalThis.fetch } = {}) {
  const api = createApiClient({ base, fetchImpl: fetch })
  return {
    list(params) { return api.get('/content-pool', params) },
    get(id) { return api.get(`/content-pool/${id}`) },
    setStatus(id, status, reason = '', auditNote = '') { return api.post(`/content-pool/${id}/status`, { status, reason, audit_note: auditNote }) },
    batchSetStatus(ids, status, reason = '', auditNote = '') { return api.post('/content-pool/batch/status', { ids, status, reason, audit_note: auditNote }) },
    materialize(id) { return api.post(`/content-pool/${id}/materialize`, {}) },
    batchMaterialize(ids) { return api.post('/content-pool/batch/materialize', { ids }) },
  }
}
