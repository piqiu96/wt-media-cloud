import { describe, expect, it, vi } from 'vitest'
import { createFilesClient } from './files.js'
import { setSessionToken } from './http.js'

describe('operations files', () => {
  it('uploads actual multipart bytes with the Desktop session header', async () => {
    const fetch = vi.fn(async () => new Response(JSON.stringify({ errcode: 0, data: { id: 1 } })))
    const client = createFilesClient({ base: 'https://cloud.example/api/v1', fetch })
    const file = new File(['png bytes'], '原名.png', { type: 'image/png' })
    setSessionToken('desktop-test-session')
    try {
      await client.upload('开头图片', file)
      const [url, options] = fetch.mock.calls[0]
      expect(url).toBe('https://cloud.example/api/v1/files')
      expect(options.headers['X-Session-Token']).toBe('desktop-test-session')
      expect(options.headers['Content-Type']).toBeUndefined()
      expect(options.body.get('name')).toBe('开头图片')
      expect(await options.body.get('file').text()).toBe('png bytes')
    } finally { setSessionToken('') }
  })
  it('builds persistent Cloud resource URLs and refuses unknown paths', () => {
    const client = createFilesClient({ base: 'https://cloud.example/api/v1' })
    const resource_path = '/api/v1/file-resources/' + 'a'.repeat(64)
    expect(client.resourceURL({ resource_path })).toBe('https://cloud.example' + resource_path)
    expect(() => client.resourceURL({ resource_path: 'https://other.example/path' })).toThrow()
  })
})
