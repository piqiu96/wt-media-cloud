import { describe, expect, it, vi } from 'vitest'
import { createFileTransferClient } from './fileTransfer.js'

function ok(data) {
  return { ok: true, status: 200, json: async () => ({ errcode: 0, message: 'success', data, logid: 'test' }) }
}

describe('file transfer api client', () => {
  it('maps the download centre commands to the frozen paths', async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(ok([]))
      .mockResolvedValueOnce(ok({ id: 't-1', status: 'running' }))
      .mockResolvedValueOnce(ok({ id: 't-1', status: 'pending' }))
    const client = createFileTransferClient({ fetch })

    await client.listTasks()
    await client.cancelTask('t-1')
    await client.retryTask('t-1')

    expect(fetch.mock.calls[0][0]).toBe('/api/v1/file-transfer-tasks')
    expect(fetch.mock.calls[0][1].method).toBe('GET')
    expect(fetch.mock.calls[1][0]).toBe('/api/v1/file-transfer-tasks/t-1/cancel')
    expect(fetch.mock.calls[1][1].method).toBe('POST')
    expect(fetch.mock.calls[2][0]).toBe('/api/v1/file-transfer-tasks/t-1/retry')
    expect(fetch.mock.calls[2][1].method).toBe('POST')
  })

  // 取消返回 200 时任务可能仍是 running（执行器才有权终结）。客户端原样把服务端
  // 说的状态交出去，绝不把它改写成 cancelled —— 一个假终态会让界面显示「已取消」
  // 而下载还在继续。
  it('hands back the refreshed task verbatim instead of assuming the cancel landed', async () => {
    const fetch = vi.fn().mockResolvedValue(ok({ id: 't-1', status: 'running' }))
    const client = createFileTransferClient({ fetch })

    await expect(client.cancelTask('t-1')).resolves.toMatchObject({ status: 'running' })
  })
})
