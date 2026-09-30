import { describe, expect, it, vi } from 'vitest'
import { createMaterialsClient } from './materials.js'

function ok(data, status = 200) {
  return { ok: true, status, json: async () => ({ errcode: 0, message: 'success', data, logid: 'test' }) }
}

describe('materials api client', () => {
  it('maps the material library and my-material commands to the frozen paths', async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(ok([]))
      .mockResolvedValueOnce(ok({ id: 7, title: '演示素材' }))
      .mockResolvedValueOnce(ok({ id: 3 }, 201))
      .mockResolvedValueOnce({ ok: true, status: 204, json: async () => ({}) })
      .mockResolvedValueOnce(ok([]))
      .mockResolvedValueOnce(ok({ id: 't-1', status: 'pending' }, 202))
    const client = createMaterialsClient({ fetch })

    await client.list({ search: '演示' })
    await client.get(7)
    await client.addUsage(7)
    await client.removeUsage(3)
    await client.listMyMaterials()
    await client.createDownload(7)

    expect(fetch.mock.calls[0][0]).toBe('/api/v1/materials?search=%E6%BC%94%E7%A4%BA')
    expect(fetch.mock.calls[0][1].method).toBe('GET')
    expect(fetch.mock.calls[1][0]).toBe('/api/v1/materials/7')
    expect(fetch.mock.calls[2][0]).toBe('/api/v1/materials/7/usages')
    expect(fetch.mock.calls[2][1].method).toBe('POST')
    expect(fetch.mock.calls[3][0]).toBe('/api/v1/material-usages/3')
    expect(fetch.mock.calls[3][1].method).toBe('DELETE')
    expect(fetch.mock.calls[4][0]).toBe('/api/v1/my-materials')
    expect(fetch.mock.calls[5][0]).toBe('/api/v1/materials/7/downloads')
    expect(fetch.mock.calls[5][1].method).toBe('POST')
  })

  // 服务端只读 `search`（production/handler.go ListMaterials 只取 c.Query("search")）。
  // 把 team_id / game_id / video_status 传进去不会报错，只会被安静地丢掉然后回全量 ——
  // 那正是「看起来筛了其实没筛」。这条钉住客户端不替服务端想象参数。
  it('sends only the search parameter the server actually reads', async () => {
    const fetch = vi.fn().mockResolvedValue(ok([]))
    const client = createMaterialsClient({ fetch })

    await client.list({ search: 'x', team_id: 10, game_id: 'naruto', video_status: 'ready' })

    expect(fetch.mock.calls[0][0]).toBe('/api/v1/materials?search=x')
  })

  // 200（已在我的素材里）与 201（新建或恢复）的响应体完全一样，`parseResponse` 两种
  // 都返回 data。这里钉住「客户端不做分支」，免得日后有人加一个按状态码的判断去猜
  // 服务端的意图 —— 那个判断在 body 上是推不出来的。
  it('resolves addUsage identically for a repeated click and a fresh add', async () => {
    const existing = createMaterialsClient({ fetch: vi.fn().mockResolvedValue(ok({ id: 3 }, 200)) })
    const created = createMaterialsClient({ fetch: vi.fn().mockResolvedValue(ok({ id: 3 }, 201)) })

    await expect(existing.addUsage(7)).resolves.toEqual({ id: 3 })
    await expect(created.addUsage(7)).resolves.toEqual({ id: 3 })
  })

  // 移出返回真 204（api.NoContentEmpty，不是那个带 data:null 的 200 信封），
  // http.js 对 204 直接返回 null —— 调用方必须按 null 处理，不能解引用。
  it('resolves removeUsage to null on the real 204', async () => {
    const fetch = vi.fn().mockResolvedValue({ ok: true, status: 204, json: async () => ({}) })
    const client = createMaterialsClient({ fetch })

    await expect(client.removeUsage(3)).resolves.toBeNull()
  })
})

// 云端视频地址只从详情链接接口读（CHG-20260930-069）：列表与素材 body 都不携带，
// 这是它在客户端的唯一入口。
describe('materials api client video url', () => {
  it('reads the cloud video address through the detail link endpoint', async () => {
    const fetch = vi.fn().mockResolvedValue(ok({ url: 'http://127.0.0.1:9000/wt-media/materials/7/x.mp4' }))
    const client = createMaterialsClient({ fetch })

    await expect(client.getVideoUrl(7)).resolves.toEqual({ url: 'http://127.0.0.1:9000/wt-media/materials/7/x.mp4' })
    expect(fetch.mock.calls[0][0]).toBe('/api/v1/materials/7/video-url')
    expect(fetch.mock.calls[0][1].method).toBe('GET')
  })
})
