import { createApiClient } from './http.js'

// 当前账号绑定的 BitBrowser 主账号。Cloud 只回掩码与绑定时间：完整主账号留在
// Agent 与 Cloud 之间。绑定与换绑不在这里 —— 那要由 Desktop 请本机 Agent 实读
// 后提交，页面只决定是否确认换绑（`init.js` 的 `bindBitBrowser`）。
export function createBitAccountClient({ base = '/api/v1', fetch = globalThis.fetch } = {}) {
  const api = createApiClient({ base, fetchImpl: fetch })
  return {
    get() {
      return api.get('/local-agent/bit-account')
    },
  }
}
