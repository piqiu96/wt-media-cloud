import { cloudResourceURL, createApiClient } from './http.js'

export function createFilesClient({ base = '/api/v1', fetch = globalThis.fetch } = {}) {
  const api = createApiClient({ base, fetchImpl: fetch })
  return {
    list(params) { return api.get('/files', params) },
    upload(name, file) {
      const body = new FormData()
      body.set('name', name)
      body.set('file', file)
      return api.post('/files', body)
    },
    rename(id, name) { return api.patch(`/files/${id}`, { name }) },
    delete(id) { return api.delete(`/files/${id}`) },
    resourceURL(file) {
      if (!/^\/api\/v1\/file-resources\/[a-f0-9]{64}$/.test(file.resource_path)) throw new Error('文件链接无效')
      return cloudResourceURL(file.resource_path, base)
    },
  }
}
