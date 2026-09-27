import { describe, expect, it } from 'vitest'
import { createDownloadFailureMessage, taskErrorLabel } from './downloadErrors.js'

describe('download failure messages', () => {
  // 走查里报的第一条：「有些点击下载需要绑定，但不知道去哪里绑定」。原文案只说了
  // 「请确认本机 Agent 已启动并绑定」，没说是**哪一页**——那个按钮在「本地环境 → Agent
  // 状态」上，不在这条提示里，运营就得自己找。判据（node 是否已绑定）不变，变的是这句
  // 话必须把去处说出来。
  it('names the page that fixes a missing local node', () => {
    const message = createDownloadFailureMessage({ type: 'local_transfer_node_unavailable' })
    expect(message).toContain('本地环境')
    expect(message).toContain('Agent 状态')
  })

  // 第二条是「文案改准」。Cloud 只在 `source_url` 为空时抛 `material_unavailable`，
  // 也就是「这个素材没有可下载的来源地址」——一个等多久都不会变的结论。原来的
  // 「云端正在准备」是句假承诺：它让人以为等着就好了。
  it('does not promise a preparation for a material with no source address', () => {
    const message = createDownloadFailureMessage({ type: 'material_unavailable' })
    expect(message).not.toContain('正在准备')
    expect(message).toContain('来源地址')
  })

  // 认不出来就把服务端的话原样交出去：这是这个接口唯一能给出的诊断信息，
  // 换成「操作失败」等于把它扔掉。
  it('passes through the server message for a type it does not know', () => {
    expect(createDownloadFailureMessage({ type: 'mystery', message: '服务端的原话' })).toBe('服务端的原话')
    expect(createDownloadFailureMessage({})).toBe('发起下载失败')
  })

  // 任务行上的码认不出来时**显示这个码本身**：空白会被读成「没出错」，
  // 而一个没见过的码恰恰是最该被看见的东西。
  it('shows an unknown task error code rather than nothing', () => {
    expect(taskErrorLabel('download_integrity_failed')).toBe('文件完整性校验未通过')
    expect(taskErrorLabel('download_disk_insufficient')).toContain('磁盘')
    expect(taskErrorLabel('cancelled_by_user')).toContain('取消')
    expect(taskErrorLabel('some_new_code')).toBe('some_new_code')
    expect(taskErrorLabel('')).toBe('')
  })
})
