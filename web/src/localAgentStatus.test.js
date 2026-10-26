import { describe, expect, it } from 'vitest'

import { createLocalAgentStatusPage } from './apps/desktop/features/local-agent/local-agent-status.js'

const snapshot = {
  agentId: 'local-agent-dev',
  status: 'idle',
  bitbrowserStatus: 'normal',
  mainUserIdMasked: 'main****1234',
  operatingSystem: 'macos',
  cpuArchitecture: 'arm64',
  agentVersion: '0.3.0',
}

describe('local agent status page', () => {
  it('is ready only when a credential is held as well as the local environment', () => {
    const page = createLocalAgentStatusPage(snapshot, {
      cloudUser: { username: 'operator01' },
      tokenConfigured: true,
    })

    expect(page.ready).toBe(true)
    expect(page.trustText).toBe('可执行本机浏览器操作')
    expect(page.trustReason).toContain('扫描窗口、配置代理和检查账号')
    expect(page.credentialText).toBe('已配置')
  })

  it('names the missing credential instead of claiming the machine can work', () => {
    const page = createLocalAgentStatusPage(snapshot, {
      cloudUser: { username: 'operator01' },
      tokenConfigured: false,
    })

    expect(page.ready).toBe(false)
    expect(page.trustText).toBe('本机执行凭证未配置')
    expect(page.trustReason).toContain('执行凭证')
    expect(page.credentialText).toBe('未配置')
  })

  it('asks for a BitBrowser login when no masked account was read', () => {
    const page = createLocalAgentStatusPage({ ...snapshot, mainUserIdMasked: '' }, {
      cloudUser: { username: 'operator01' },
      tokenConfigured: true,
    })

    expect(page.ready).toBe(false)
    expect(page.trustReason).toContain('比特浏览器')
    expect(page.bitbrowserText).toBe('可用')
  })
})
