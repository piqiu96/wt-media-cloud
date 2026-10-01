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
    expect(page.bindActionText).toBe('刷新本机执行状态')
  })

  it('keeps device binding and session connection distinct before a local node exists', () => {
    const page = createLocalAgentStatusPage({
      agentId: 'local-agent-dev',
      status: 'idle',
      bitbrowserStatus: 'normal',
      mainUserId: 'main-user-1',
    }, {
      cloudUser: { username: 'operator01' },
      canBindTrustedNode: true,
    })

    expect(page.trustText).toBe('当前会话待连接')
    expect(page.trustReason).toContain('当前会话尚未连接本机执行节点')
    expect(page.trustReason).toContain('个人信息中的设备绑定状态')
    expect(page.bindActionText).toBe('连接当前会话')
  })
})
