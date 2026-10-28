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

  // 服务端读 `search` / `team_id` / `category_ids` / `category_id` / `joined` / `my_usage` /
  // `team_usage` / `video_status` / `sort_by` / `sort_order`（parseMaterialListFilter）。
  // 团队范围仍由服务端决定角色边界，`team_id` 只是管理员的目标团队；页面传它不等于
  // 自己获得权限。多选类目在客户端序列化成逗号分隔的一个参数，调用点只交数组。
  it('forwards every server-side filter and joins the multi-category selection', async () => {
    const fetch = vi.fn().mockResolvedValue(ok([]))
    const client = createMaterialsClient({ fetch })

    await client.list({
      search: 'x', team_id: 10, category_ids: ['naruto', 'onepiece'], category_id: 'legacy',
      joined: 'joined', my_usage: 'used', team_usage: 'unused',
      video_status: 'ready', sort_by: 'published_at', sort_order: 'desc',
    })

    expect(fetch.mock.calls[0][0]).toBe('/api/v1/materials?search=x&team_id=10&category_ids=naruto%2Conepiece&category_id=legacy&joined=joined&my_usage=used&team_usage=unused&video_status=ready&sort_by=published_at&sort_order=desc')
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
// 这是它在客户端的唯一入口。返回的是**签名**地址（CHG-20260930-069 任务 26），带
// 查询串且会过期 —— 客户端原样透传，不解析、不缓存。
describe('materials api client video url', () => {
  it('reads the signed cloud video address through the detail link endpoint', async () => {
    const signed = 'http://127.0.0.1:9000/wt-media/materials/7/x.mp4?X-Amz-Signature=stub&X-Amz-Expires=900'
    const fetch = vi.fn().mockResolvedValue(ok({ url: signed }))
    const client = createMaterialsClient({ fetch })

    await expect(client.getVideoUrl(7)).resolves.toEqual({ url: signed })
    expect(fetch.mock.calls[0][0]).toBe('/api/v1/materials/7/video-url')
    expect(fetch.mock.calls[0][1].method).toBe('GET')
  })
})
