import { describe, expect, it, vi } from 'vitest'

import { createLocalAgentService, LOCAL_AGENT_COMMANDS } from './apps/desktop/features/local-agent/service.js'
import { startDesktopLocalAgent } from './apps/desktop/features/local-agent/init.js'

describe('local agent desktop service', () => {
  it('starts the Agent through Tauri only when Desktop is available', async () => {
    const invoke = vi.fn(async () => 'sidecar_started')

    await expect(startDesktopLocalAgent({ tauri: true, invokeImpl: invoke })).resolves.toBe(true)
    await expect(startDesktopLocalAgent({ tauri: false, invokeImpl: invoke })).resolves.toBe(false)
    expect(invoke).toHaveBeenCalledTimes(1)
    expect(invoke).toHaveBeenCalledWith(LOCAL_AGENT_COMMANDS.start)
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
            cloud_base_url: 'http://127.0.0.1:5176',
            task_id: 'task-1',
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
      cloudBaseUrl: 'http://127.0.0.1:5176',
      taskId: 'task-1',
      bitProfileId: 'bit-profile-1',
      platform: 'bilibili',
      expectedPlatformAccountId: '123',
    })).resolves.toEqual(result)
    expect(invoke).toHaveBeenCalledWith('local_agent_account_check', expect.any(Object))
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

  it('proxies runtime refresh through Tauri invoke without exposing node credentials', async () => {
    const status = { node_id: 'node-1', agent_id: 'local-agent-dev', status: 'idle' }
    const invoke = vi.fn(async (command, args) => {
      if (command === LOCAL_AGENT_COMMANDS.refreshRuntime) {
        expect(args).toEqual({
          args: {
            cloud_base_url: 'http://127.0.0.1:18080',
          },
        })
        return status
      }
      return {}
    })
    const service = createLocalAgentService({ invoke })

    await expect(service.refreshRuntime({
      cloudBaseUrl: 'http://127.0.0.1:18080',
    })).resolves.toEqual(expect.objectContaining({ node_id: 'node-1' }))
    expect(invoke).toHaveBeenCalledWith('local_agent_refresh_runtime', expect.any(Object))
  })
})
