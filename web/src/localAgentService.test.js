import { describe, expect, it, vi } from 'vitest'

import { createLocalAgentService, LOCAL_AGENT_COMMANDS } from './apps/desktop/features/local-agent/service.js'

describe('local agent desktop service', () => {
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
})
