import { describe, expect, it, vi } from 'vitest'
import {
  aggregateWorkEnv,
  bitAccountDiffers,
  loadWorkEnvInputs,
  maskBitAccountId,
} from './apps/desktop/features/local-agent/work-env-status.js'
import { normalizeLocalAgentStatus } from './apps/desktop/features/local-agent/service.js'

// 快照夹具用 Agent `/api/v1/status` 的真实线格式（snake_case，2026-10-02 实测
// 8765 应答摘录）。aggregateWorkEnv 入口负责 snake→camel 适配——此前夹具直接
// 手写 camelCase，绕过了真实边界，字段改名/形状漂移测不出来（CHG-074 走查
// 面板误报的根因）。
function healthySnapshot(overrides = {}) {
  return {
    node_id: 'agent-node_abc',
    agent_id: 'local-agent-dev',
    status: 'idle',
    bitbrowser_status: 'normal',
    main_user_id: 'main-1',
    operating_system: 'macos',
    cpu_architecture: 'arm64',
    agent_version: '0.2.2',
    current_task_id: null,
    current_task_status: null,
    current_task_progress: null,
    pending_result_count: 0,
    ...overrides,
  }
}

const cloudUser = { id: 'u-1', username: 'op' }
const binding = { bound: true, device_id: 'dev-1', device_name: '运营电脑A' }
const localDevice = { device_id: 'dev-1' }

describe('aggregateWorkEnv', () => {
  it('reports workable when binding matches and the local environment is ready', () => {
    const env = aggregateWorkEnv({ snapshot: healthySnapshot(), cloudUser, binding, localDevice })
    expect(env.workable).toBe(true)
    expect(env.mismatched).toBe(false)
    expect(env.blockers).toEqual([])
  })

  it('reports not workable with the binding blocker when no device is bound', () => {
    const env = aggregateWorkEnv({ snapshot: healthySnapshot(), cloudUser, binding: { bound: false }, localDevice })
    expect(env.workable).toBe(false)
    expect(env.blockers[0]).toContain('尚未绑定运营电脑')
  })

  it('flags a device mismatch as its own blocker', () => {
    const env = aggregateWorkEnv({ snapshot: healthySnapshot(), cloudUser, binding, localDevice: { device_id: 'dev-2' } })
    expect(env.workable).toBe(false)
    expect(env.mismatched).toBe(true)
    expect(env.blockers[0]).toContain('不一致')
  })

  it('surfaces the local-agent reason when the agent is stopped', () => {
    const env = aggregateWorkEnv({ snapshot: healthySnapshot({ status: 'stopped' }), cloudUser, binding, localDevice })
    expect(env.workable).toBe(false)
    expect(env.blockers).toContain('本机服务未启动，请重启应用后再检查。')
    expect(env.page.serviceText).toBe('未运行')
  })

  it('surfaces the bitbrowser reason when it is not reachable', () => {
    const env = aggregateWorkEnv({ snapshot: healthySnapshot({ bitbrowser_status: 'unreachable' }), cloudUser, binding, localDevice })
    expect(env.workable).toBe(false)
    expect(env.blockers).toContain('未检测到可用的比特浏览器，请先启动比特浏览器后再检测。')
  })

  it('surfaces the missing-node reason when no node is connected', () => {
    const env = aggregateWorkEnv({ snapshot: healthySnapshot({ node_id: '' }), cloudUser, binding, localDevice })
    expect(env.workable).toBe(false)
    expect(env.blockers).toContain('本机服务尚未连接云端，请到「个人信息」页检查设备绑定。')
  })

  it('keeps 需检查 when there is no logged-in user', () => {
    const env = aggregateWorkEnv({ snapshot: healthySnapshot(), cloudUser: null, binding, localDevice })
    expect(env.workable).toBe(false)
    expect(env.blockers).toContain('请先登录运营平台')
  })
})

// 面板三行各说各话：一行坏了不歪曲另一行的事实（此前两行共用 page.bound
// 合取开关，比特浏览器坏会把已连上的本机服务显示成未连接）。
describe('work env rows', () => {
  it('keeps the service row connected while bitbrowser is down', () => {
    const env = aggregateWorkEnv({ snapshot: healthySnapshot({ bitbrowser_status: 'unreachable' }), cloudUser, binding, localDevice })
    expect(env.page.serviceText).toBe('已连接，当前无任务')
    expect(env.page.bitbrowserText).toBe('不可达')
  })

  it('shows the service row as not connected to cloud when no node exists, with bitbrowser fine', () => {
    const env = aggregateWorkEnv({ snapshot: healthySnapshot({ node_id: '' }), cloudUser, binding, localDevice })
    expect(env.page.serviceText).toBe('未连接云端')
    expect(env.page.bitbrowserText).toBe('已登录指定账号')
  })

  it('shows the running task on the service row', () => {
    const env = aggregateWorkEnv({ snapshot: healthySnapshot({ status: 'running', current_task_id: 'task-9' }), cloudUser, binding, localDevice })
    expect(env.page.serviceText).toBe('已连接，当前任务 task-9')
  })
})

// 比特账号一致性判定是个人中心入口可见性与胶囊结论的共同来源。32 位 hex 账号
// （形如 2c9b…619f）经掩码比对；下面每组输入都标出「谁更该被信」的走向。
const BOUND_MASK = '2c9b****619f'
const LOCAL_ID = '2c9b000000000000000000000000619f'
const OTHER_ID = 'aaaa111111111111111111111111bbbb'

describe('bitAccountDiffers', () => {
  const bound = { bit_account_bound: true, bit_main_user_id_masked: BOUND_MASK }

  it('is false when the environment account matches the bound one', () => {
    expect(bitAccountDiffers(bound, LOCAL_ID)).toBe(false)
  })

  it('is true when the client switched to another account', () => {
    expect(bitAccountDiffers(bound, OTHER_ID)).toBe(true)
  })

  it('is false when no bit account is bound yet', () => {
    expect(bitAccountDiffers({ bit_account_bound: false, bit_main_user_id_masked: '' }, LOCAL_ID)).toBe(false)
  })

  // 闸门是 bit_account_bound，不是「掩码非空」：只凭一段残留掩码就宣告不一致，
  // 会让未绑定的账号被推去点一次无意义的「比特账号绑定」。
  it('gates on the bound flag rather than on a leftover mask', () => {
    expect(bitAccountDiffers({ bit_account_bound: false, bit_main_user_id_masked: BOUND_MASK }, OTHER_ID)).toBe(false)
  })

  it('is false when the environment reports no account at all', () => {
    expect(bitAccountDiffers(bound, '')).toBe(false)
    expect(bitAccountDiffers(bound, undefined)).toBe(false)
  })

  // 下面两条钉住的是「比对不了时倒向哪边」——不是「一致」。后端掩码在长度 ≤8 时
  // 整体变成 ****（maskDeviceIdentity），或本机账号短到无法按首尾4位比对：两者都
  // 不能证明一致，判定按不一致走（多给一次自助绑定入口，好过静默当作已一致）。
  it('reads a masked-out bound value as differing rather than as matching', () => {
    expect(bitAccountDiffers({ bit_account_bound: true, bit_main_user_id_masked: '****' }, LOCAL_ID)).toBe(true)
  })

  it('reads an environment id too short to split as differing', () => {
    expect(bitAccountDiffers(bound, 'short')).toBe(true)
  })

  // 唯一判定源：同一输入下，个人中心读的函数与胶囊读的聚合字段必须同值。
  it('agrees with the aggregate on the same inputs', () => {
    const cases = [
      bound,
      { bit_account_bound: false, bit_main_user_id_masked: '' },
      { bit_account_bound: false, bit_main_user_id_masked: BOUND_MASK },
      { bit_account_bound: true, bit_main_user_id_masked: '****' },
    ]
    for (const b of cases) {
      for (const local of [LOCAL_ID, OTHER_ID, '', 'short']) {
        const env = aggregateWorkEnv({ snapshot: healthySnapshot({ main_user_id: local }), cloudUser, binding: b, localDevice })
        expect(env.bitAccountDiffers).toBe(bitAccountDiffers(b, local))
      }
    }
  })
})

describe('maskBitAccountId', () => {
  it('keeps the first and last four, the rule the backend masks with', () => {
    expect(maskBitAccountId(LOCAL_ID)).toBe(BOUND_MASK)
  })

  it('masks everything when the id is too short to split', () => {
    expect(maskBitAccountId('abc12345')).toBe('****')
    expect(maskBitAccountId('abc123456')).toBe('abc1****3456')
  })

  it('stays empty when there is no id', () => {
    expect(maskBitAccountId('')).toBe('')
    expect(maskBitAccountId('   ')).toBe('')
    expect(maskBitAccountId(undefined)).toBe('')
  })
})

// 接线用例：四项输入一次读齐。此前胶囊的轮询只刷本机状态与云端绑定，
// cloudUser / localDevice 挂载后再不重读——在个人中心改完绑定，胶囊停在旧结论。
describe('loadWorkEnvInputs', () => {
  function deps(overrides = {}) {
    return {
      session: { me: vi.fn(async () => ({ id: 'u-1', username: 'op' })) },
      devices: { get: vi.fn(async () => ({ bound: true, device_id: 'dev-1' })) },
      service: { status: vi.fn(async () => healthySnapshot()) },
      invoke: vi.fn(async () => ({ device_id: 'dev-1' })),
      ...overrides,
    }
  }

  it('reads all four inputs in one load', async () => {
    const d = deps()
    const inputs = await loadWorkEnvInputs(d)

    expect(d.session.me).toHaveBeenCalledTimes(1)
    expect(d.invoke).toHaveBeenCalledWith('local_device_identity')
    expect(d.service.status).toHaveBeenCalledTimes(1)
    expect(d.devices.get).toHaveBeenCalledTimes(1)
    expect(inputs.cloudUser).toEqual({ id: 'u-1', username: 'op' })
    expect(inputs.localDevice).toEqual({ device_id: 'dev-1' })
    expect(inputs.snapshot.status).toBe('idle')
    expect(inputs.binding).toEqual({ bound: true, device_id: 'dev-1' })
    expect(inputs.errors).toEqual([])
  })

  // 冷启动竞态：sidecar 还没监听时 local_agent_status 直接抛错，另外三项照样可用。
  it('keeps the other three inputs when the local agent is not up yet', async () => {
    const failure = new Error('agent unreachable: connection refused')
    const inputs = await loadWorkEnvInputs(deps({ service: { status: vi.fn(async () => { throw failure }) } }))

    expect(inputs.snapshot).toBeNull()
    expect(inputs.cloudUser).toEqual({ id: 'u-1', username: 'op' })
    expect(inputs.localDevice).toEqual({ device_id: 'dev-1' })
    expect(inputs.binding).toEqual({ bound: true, device_id: 'dev-1' })
    expect(inputs.errors).toEqual([failure])
  })

  it('treats a missing device identity as absent, not as a failed load', async () => {
    const inputs = await loadWorkEnvInputs(deps({ invoke: vi.fn(async () => { throw new Error('no identity') }) }))

    expect(inputs.localDevice).toBeNull()
    expect(inputs.binding).toEqual({ bound: true, device_id: 'dev-1' })
    expect(inputs.errors).toHaveLength(1)
  })
})

// 接线用例：真实线格式经 normalizeLocalAgentStatus（service 边界）后进聚合，
// 钉死「service 输出形状 = 聚合输入形状」这条链——形状漂移在此变红。
describe('work env wiring from the agent wire shape', () => {
  it('reports a fully green panel from a normalized real-agent snapshot', () => {
    const snapshot = normalizeLocalAgentStatus(healthySnapshot())
    const env = aggregateWorkEnv({ snapshot, cloudUser, binding, localDevice })
    expect(env.workable).toBe(true)
    expect(env.blockers).toEqual([])
    expect(env.page.serviceText).toBe('已连接，当前无任务')
    expect(env.page.bitbrowserText).toBe('已登录指定账号')
  })

  it('degrades to 未检测 (not 未知) when the wire snapshot lacks bitbrowser_status', () => {
    const snapshot = normalizeLocalAgentStatus({ status: 'idle' })
    const env = aggregateWorkEnv({ snapshot, cloudUser, binding, localDevice })
    expect(env.workable).toBe(false)
    expect(env.page.bitbrowserText).toBe('未检测')
    expect(env.page.bitbrowserStatusText).toBe('未检测')
  })
})
