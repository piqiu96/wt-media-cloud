import { createApiClient } from './http.js'
import { isDesktop } from '../../utils.js'

/**
 * 素材库与我的素材的 API client。形状照 `contentPool.js`：工厂函数 + 显式注入 fetch，
 * 线键保持 snake_case，方法名用 camelCase。
 *
 * 两条容易写错的地方，写在调用点旁边：
 *
 * 1. `list` 的形参就是白名单：服务端读 `search`/`team_id`/`game_ids`/`game_id`/
 *    `joined`/`my_usage`/`team_usage`/`video_status`/`sort_by`/`sort_order`
 *    （`parseMaterialListFilter` + `ListMaterialsFiltered`）。团队范围由服务端决定：
 *    普通运营与高级运营固定本团队，只有管理员能命名目标团队；游戏只是筛选，不是
 *    可见边界。解构而不是透传，是为了让这条边界在调用点就成立，而不是靠注释维持。
 *
 * 2. `addUsage` 的 200/201 之别在客户端**结构上不可见**：`parseResponse` 两种情况都
 *    返回 `body.data`（见 shared/api/http.js）。差别只在服务端有意义（是不是新建/恢复），
 *    所以它由服务端决定并写进响应状态码，前端不据此分支。
 */
export function createMaterialsClient({ base = '/api/v1', fetch = globalThis.fetch } = {}) {
  const api = createApiClient({ base, fetchImpl: fetch })
  return {
    list({ search, team_id, game_ids, game_id, joined, my_usage, team_usage, video_status, sort_by, sort_order } = {}) {
      // `game_ids` 在线上是逗号分隔的一个参数；页面拿数组表达多选，序列化留在这一层，
      // 免得每个调用点各自 join 一次、各自漏掉去重。
      const games = Array.isArray(game_ids) ? game_ids.filter(Boolean).join(',') : game_ids
      return api.get('/materials', { search, team_id, game_ids: games, game_id, joined, my_usage, team_usage, video_status, sort_by, sort_order })
    },
    get(id) { return api.get(`/materials/${id}`) },
    addUsage(id) { return api.post(`/materials/${id}/usages`, {}) },
    removeUsage(usageId) { return api.delete(`/material-usages/${usageId}`) },
    // 恢复走关系自己的端点，不靠「再 add 一次」：那条命令按素材 id 走，说不清它恢复的是
    // 哪一行。两条路径最终落到同一行（uq_material_usages_user_material）。
    restoreUsage(usageId) { return api.post(`/material-usages/${usageId}/restore`, {}) },
    listMyMaterials(params = {}) { return api.get('/my-materials', params) },
    // 202 恒定：此刻还没有任何字节被下载，任务在领取前一直是 pending。
    // 下一次诚实的进度来源是 GET /file-transfer-tasks，不是这个响应。
    async createDownload(id) {
      if (isDesktop()) {
        const { ensureAgentToken } = await import('../../apps/desktop/features/local-agent/init.js')
        await ensureAgentToken()
      }
      return api.post(`/materials/${id}/downloads`, {})
    },
    // 云端视频地址的唯一入口：列表与素材 body 都不携带（详情抽屉专用）。
    getVideoUrl(id) { return api.get(`/materials/${id}/video-url`) },
  }
}
