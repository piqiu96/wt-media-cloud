import { createApiClient } from './http.js'

/**
 * 下载中心的 API client。
 *
 * 单独一个文件而不是并进 `materials.js`：`file_transfer_task` 是执行器中立的，
 * 日后还要承载 `compose_input_prepare` 这类非素材任务，把它挂进素材的 client 会把
 * 平台级关注点写成素材功能的一部分。
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
  }
}
