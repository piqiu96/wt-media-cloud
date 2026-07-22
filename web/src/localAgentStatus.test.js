import { describe, expect, it } from 'vitest'

import { createLocalAgentStatusPage } from './apps/desktop/features/local-agent/local-agent-status.js'

describe('local agent status page', () => {
  it('does not claim cloud trust from a local node id alone', () => {
    const page = createLocalAgentStatusPage({
      nodeId: 'agent-node-1',
      agentId: 'local-agent-dev',
      status: 'idle',
      bitbrowserStatus: 'normal',
      mainUserId: 'main-user-1',
      operatingSystem: 'macos',
      cpuArchitecture: 'arm64',
      agentVersion: '0.2.2',
    }, {
      cloudUser: { username: 'operator01' },
      canBindTrustedNode: true,
    })

    expect(page.trustText).toBe('可执行本机浏览器操作')
    expect(page.trustReason).toContain('扫描窗口、配置代理和检查账号')
    expect(page.canBindTrustedNode).toBe(true)
    expect(page.bindActionText).toBe('刷新本机可信状态')
  })

  it('uses first-bind wording before a local node exists', () => {
    const page = createLocalAgentStatusPage({
      agentId: 'local-agent-dev',
      status: 'idle',
      bitbrowserStatus: 'normal',
      mainUserId: 'main-user-1',
    }, {
      cloudUser: { username: 'operator01' },
      canBindTrustedNode: true,
    })

    expect(page.trustText).toBe('尚未完成可信绑定')
    expect(page.trustReason).toContain('暂不能执行本机浏览器相关操作')
    expect(page.bindActionText).toBe('绑定当前比特浏览器账号')
  })
})
