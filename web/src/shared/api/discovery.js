import { createApiClient } from './http.js'

export function createDiscoveryClient({ base = '/api/v1', fetch = globalThis.fetch } = {}) {
  const api = createApiClient({ base, fetchImpl: fetch })
  return {
    listStrategies() { return api.get('/discovery-strategies') },
    createStrategy(data) { return api.post('/discovery-strategies', data) },
    updateStrategy(id, data) { return api.put(`/discovery-strategies/${id}`, data) },
    setStrategyStatus(id, status) { return api.post(`/discovery-strategies/${id}/status`, { status }) },
    deleteStrategy(id) { return api.delete(`/discovery-strategies/${id}`) },
    runStrategy(id) { return api.post(`/discovery-strategies/${id}/run`, {}) },
    listTasks(params) { return api.get('/crawl-tasks', params) },
    getTask(id) { return api.get(`/crawl-tasks/${id}`) },
    confirmResults(id, ids) { return api.post(`/crawl-tasks/${id}/confirm`, { ids }) },
    retryFailed(id) { return api.post(`/crawl-tasks/${id}/retry-failed`, {}) },
    search(data) { return api.post('/content-pool/search', data) },
    authorSearch(data) { return api.post('/content-pool/author-search', data) },
    importResults(data) { return api.post('/content-pool/import-results', data) },
    importUrl(data) { return api.post('/content-pool/import-url', data) },
  }
}
