import { createApiClient } from './http.js'

/**
 * 素材库与我的素材的 API client。形状照 `contentPool.js`：工厂函数 + 显式注入 fetch，
 * 线键保持 snake_case，方法名用 camelCase。
 *
 * 两条容易写错的地方，写在调用点旁边：
 *
 * 1. `list` 的形参就是白名单：**只有 `search` 会被转发**。服务端的团队范围与游戏范围由
 *    会话决定（`ListMaterials` 只读 `c.Query("search")`），页面给 `team_id`/`game_id`
 *    这类筛选项就是在假装它筛得动 —— 传进去不会报错，只会被安静地丢掉然后回全量。
 *    解构而不是透传，是为了让这条边界在调用点就成立，而不是靠注释维持。游戏过滤
 *    因此只能在客户端做。
 *
 * 2. `addUsage` 的 200/201 之别在客户端**结构上不可见**：`parseResponse` 两种情况都
 *    返回 `body.data`（见 shared/api/http.js）。差别只在服务端有意义（是不是新建/恢复），
 *    所以它由服务端决定并写进响应状态码，前端不据此分支。
 */
export function createMaterialsClient({ base = '/api/v1', fetch = globalThis.fetch } = {}) {
  const api = createApiClient({ base, fetchImpl: fetch })
  return {
    list({ search } = {}) { return api.get('/materials', { search }) },
    get(id) { return api.get(`/materials/${id}`) },
    addUsage(id) { return api.post(`/materials/${id}/usages`, {}) },
    removeUsage(usageId) { return api.delete(`/material-usages/${usageId}`) },
    listMyMaterials() { return api.get('/my-materials') },
    // 202 恒定：此刻还没有任何字节被下载，任务在节点领取前一直是 pending。
    // 下一次诚实的进度来源是 GET /file-transfer-tasks，不是这个响应。
    createDownload(id) { return api.post(`/materials/${id}/downloads`, {}) },
    // 云端视频地址的唯一入口：列表与素材 body 都不携带（详情抽屉专用）。
    getVideoUrl(id) { return api.get(`/materials/${id}/video-url`) },
  }
}
