import { describe, expect, it, vi } from 'vitest'

import { createLocalAgentService, LOCAL_AGENT_COMMANDS } from './apps/desktop/features/local-agent/service.js'
import { startDesktopLocalAgent } from './apps/desktop/features/local-agent/init.js'

describe('local agent desktop service', () => {
  it('starts the Agent through Tauri only when Desktop is available', async () => {
    const invoke = vi.fn(async (command) => {
      if (command === LOCAL_AGENT_COMMANDS.tokenFacts) return { configured: false, expires_at: null }
      return 'sidecar_started'
    })

    await expect(startDesktopLocalAgent({ tauri: true, invokeImpl: invoke })).resolves.toBe(true)
    await expect(startDesktopLocalAgent({ tauri: false, invokeImpl: invoke })).resolves.toBe(false)
    expect(invoke).toHaveBeenNthCalledWith(1, LOCAL_AGENT_COMMANDS.start)
    // 启动后补一次凭证：先只读问 Agent 手里有没有，未登录时不会发换取请求。
    await vi.waitFor(() => expect(invoke).toHaveBeenCalledWith(LOCAL_AGENT_COMMANDS.tokenFacts))
    expect(invoke).not.toHaveBeenCalledWith(LOCAL_AGENT_COMMANDS.configureToken, expect.anything())
  })

  it('proxies profile scan through Tauri invoke', async () => {
    const snapshot = { main_user_id: 'main-user-1', profiles: [{ bit_profile_id: 'p1' }] }
    const invoke = vi.fn(async (command) => {
      if (command === LOCAL_AGENT_COMMANDS.profileScan) return snapshot
      return {}
    })
    const service = createLocalAgentService({ invoke })

    await expect(service.profileScan()).resolves.toEqual(snapshot)
    expect(invoke).toHaveBeenCalledWith('local_agent_profile_scan')
  })

  it('proxies profile restore through Tauri invoke', async () => {
    const profiles = [{ bit_profile_id: 'p1', name: '窗口一' }]
    const result = { restored_count: 1, profiles: [{ bit_profile_id: 'p1', status: 'verified' }] }
    const invoke = vi.fn(async (command, args) => {
      if (command === LOCAL_AGENT_COMMANDS.profileRestore) {
        expect(args).toEqual({ profiles })
        return result
      }
      return {}
    })
    const service = createLocalAgentService({ invoke })

    await expect(service.profileRestore(profiles)).resolves.toEqual(result)
    expect(invoke).toHaveBeenCalledWith('local_agent_profile_restore', { profiles })
  })

  it('proxies account check through Tauri invoke without exposing agent credentials', async () => {
    const result = { platform_account_id: '123', login_status: 'normal' }
    const invoke = vi.fn(async (command, args) => {
      if (command === LOCAL_AGENT_COMMANDS.accountCheck) {
        expect(args).toEqual({
          args: {
            operation_id: 'task-1',
            bit_profile_id: 'bit-profile-1',
            platform: 'bilibili',
            expected_platform_account_id: '123',
          },
        })
        return result
      }
      return {}
    })
    const service = createLocalAgentService({ invoke })

    await expect(service.accountCheck({
      operationId: 'task-1',
      bitProfileId: 'bit-profile-1',
      platform: 'bilibili',
      expectedPlatformAccountId: '123',
    })).resolves.toEqual(result)
    expect(invoke).toHaveBeenCalledWith('local_agent_account_check', expect.any(Object))
  })

  it('extracts dynamic proxy addresses through the Desktop Agent bridge', async () => {
    const result = { proxy_protocol: 'socks5', host: '127.0.0.1', port: 1080, username: '', password: '' }
    const invoke = vi.fn(async (command, args) => {
      if (command === LOCAL_AGENT_COMMANDS.proxyExtract) {
        expect(args).toEqual({
          args: {
            extract_url: 'https://provider.example/extract',
            proxy_protocol: 'socks5',
          },
        })
        return result
      }
      return {}
    })
    const service = createLocalAgentService({ invoke })

    await expect(service.proxyExtract({
      extractUrl: 'https://provider.example/extract',
      proxyProtocol: 'socks5',
    })).resolves.toEqual(result)
    expect(invoke).toHaveBeenCalledWith('local_agent_proxy_extract', expect.any(Object))
  })

  it('proxies account row window operations through Tauri invoke', async () => {
    const invoke = vi.fn(async (command, args) => ({ command, args }))
    const service = createLocalAgentService({ invoke })

    await service.profileOpen('bit-profile-1')
    await service.profileClose('bit-profile-1')

    expect(invoke).toHaveBeenNthCalledWith(1, 'local_agent_profile_open', {
      args: { bit_profile_id: 'bit-profile-1' },
    })
    expect(invoke).toHaveBeenNthCalledWith(2, 'local_agent_profile_close', {
      args: { bit_profile_id: 'bit-profile-1' },
    })
  })

  it('creates profiles without sending optional bit sequence', async () => {
    const invoke = vi.fn(async () => ({ bit_profile_id: 'profile-1' }))
    const service = createLocalAgentService({ invoke })

    await service.profileCreate({ name: '窗口', group_id: 'group-1', group_name: '测试组', remark: '备注' })

    expect(invoke).toHaveBeenCalledWith('local_agent_profile_create', {
      args: {
        name: '窗口',
        group_id: 'group-1',
        group_name: '测试组',
        remark: '备注',
      },
    })
  })

  it('exchanges a Desktop session for an execution credential and says only whether one is held', async () => {
    const facts = { configured: true, expires_at: 1800000000 }
    const invoke = vi.fn(async (command, args) => {
      if (command === LOCAL_AGENT_COMMANDS.configureToken) {
        expect(args).toEqual({
          args: {
            cloud_base_url: 'https://cloud.example.test',
            session_token: 'session-token-1',
          },
        })
        return facts
      }
      if (command === LOCAL_AGENT_COMMANDS.tokenFacts) return { configured: false, expires_at: null }
      return {}
    })
    const service = createLocalAgentService({ invoke })

    await expect(service.tokenFacts()).resolves.toEqual({ configured: false, expires_at: null })
    await expect(service.configureToken({
      cloudBaseUrl: 'https://cloud.example.test',
      sessionToken: 'session-token-1',
    })).resolves.toEqual(facts)
    expect(JSON.stringify(facts)).not.toContain('session-token-1')
  })

  it('asks the Agent to bind the current BitBrowser account, carrying only the user intent', async () => {
    const invoke = vi.fn(async () => ({ bound: true }))
    const service = createLocalAgentService({ invoke })

    await expect(service.bitAccountBind({ rebind: true })).resolves.toEqual({ bound: true })
    expect(invoke).toHaveBeenCalledWith('local_agent_bit_account_bind', { rebind: true })
  })
})

describe('where Cloud is', () => {
  // `init.js` caches the last known good answer at module scope, so these tests
  // need a fresh module registry each time -- sharing one instance would make
  // them depend on the order they run in.
  async function freshCloudBaseUrl() {
    vi.resetModules()
    const init = await import('./apps/desktop/features/local-agent/init.js')
    return init.cloudBaseUrl
  }

  it('asks the native side instead of assuming loopback', async () => {
    const cloudBaseUrl = await freshCloudBaseUrl()
    const invoke = vi.fn(async () => ({ cloud_base_url: 'https://cloud.example.test' }))

    await expect(cloudBaseUrl({ invokeImpl: invoke })).resolves.toBe('https://cloud.example.test')
    // The literal, not a constant imported from the module under test: this is
    // the WebView-to-Rust contract, and pinning it here means renaming the
    // command on one side alone fails the suite.
    expect(invoke).toHaveBeenCalledWith('get_public_config')
  })

  it('answers with nothing rather than a loopback address when there is no answer yet', async () => {
    const cloudBaseUrl = await freshCloudBaseUrl()
    const invoke = vi.fn(async () => {
      throw new Error('not running inside Desktop')
    })

    const url = await cloudBaseUrl({ invokeImpl: invoke })

    // Empty, not a fallback address: an empty Cloud address reaches the
    // existing "Cloud地址为空" error path, while a loopback guess reaches a
    // different machine's idea of where Cloud is -- and is wrong in every
    // packaged build, whose own origin is tauri.localhost.
    expect(url).toBe('')
    expect(url).not.toMatch(/127\.0\.0\.1/)
  })

  it('keeps the last known good address when the native side later fails', async () => {
    const cloudBaseUrl = await freshCloudBaseUrl()
    const ok = vi.fn(async () => ({ cloud_base_url: 'https://cloud.example.test' }))
    await cloudBaseUrl({ invokeImpl: ok })

    const broken = vi.fn(async () => {
      throw new Error('window went away')
    })

    await expect(cloudBaseUrl({ invokeImpl: broken })).resolves.toBe('https://cloud.example.test')
  })
})
