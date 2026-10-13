import { createApiClient } from './http.js'

/**
 * 下载中心的 API client。
 *
 * `file_transfer_task` 同时承载素材、成片下载和云端准备任务，
 * 因此会话操作独立于素材 API。
 *
 * `cancel` 的响应体是**刷新后的任务，可能仍是 running**：取消只是请求，running 的
 * 执行器才有权做终态迁移。所以调用方拿到 200 之后仍然要按任务自己的 status 渲染，
 * 不能把「取消成功」读成「已取消」。
 */
export function createFileTransferClient({ base = '/api/v1', fetch = globalThis.fetch } = {}) {
  const api = createApiClient({ base, fetchImpl: fetch })
  return {
    listTasks(params) { return api.get('/file-transfer-tasks', params) },
    cancelTask(taskId) { return api.post(`/file-transfer-tasks/${taskId}/cancel`, {}) },
    hideTask(taskId) { return api.post(`/file-transfer-tasks/${taskId}/hide`, {}) },
  }
}
