import { describe, expect, it, vi } from 'vitest'
import { createApiClient } from './shared/api/http.js'

describe('createApiClient', () => {
  it('accepts empty 204 responses for delete operations', async () => {
    const fetch = vi.fn(async () => ({ status: 204, ok: true }))
    const client = createApiClient({ fetchImpl: fetch })

    await expect(client.delete('/browser-profiles/profile-1')).resolves.toBeNull()

    expect(fetch).toHaveBeenCalledWith('/api/v1/browser-profiles/profile-1', {
      method: 'DELETE',
      credentials: 'include',
      headers: {},
    })
  })
})
