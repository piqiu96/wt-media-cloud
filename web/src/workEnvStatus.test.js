import { describe, expect, it, vi } from 'vitest'
import {
  aggregateWorkEnv,
  bitAccountDiffers,
  loadWorkEnvInputs,
} from './apps/desktop/features/local-agent/work-env-status.js'
import { normalizeLocalAgentStatus } from './apps/desktop/features/local-agent/service.js'

// 快照夹具用 Agent `/api/v1/status` 的真实线格式（snake_case）。`main_user_id_masked`
// 是 Tauri 按 Cloud 同一规则掩好的值 —— 页面这一侧根本收不到完整主账号。
function healthySnapshot(overrides = {}) {
  return {
    agent_id: 'local-agent-dev',
    status: 'idle',
    bitbrowser_status: 'normal',
    main_user_id_masked: BOUND_MASK,
    operating_system: 'macos',
    cpu_architecture: 'arm64',
    agent_version: '0.3.0',
    current_task_id: null,
    current_task_status: null,
    current_task_progress: null,
    pending_result_count: 0,
    ...overrides,
  }
}

const BOUND_MASK = '2c9b****619f'
const OTHER_MASK = 'aaaa****bbbb'
const cloudUser = { id: 'u-1', username: 'op' }
const binding = { bound: true, main_user_id_masked: BOUND_MASK }
const tokenFacts = { configured: true, expires_at: 1800000000 }

describe('aggregateWorkEnv', () => {
  it('reports workable when the binding matches and the local environment is ready', () => {
    const env = aggregateWorkEnv({ snapshot: healthySnapshot(), cloudUser, binding, tokenFacts })
    expect(env.workable).toBe(true)
    expect(env.bitAccountDiffers).toBe(false)
    expect(env.blockers).toEqual([])
  })

  it('reports not workable with the binding blocker when no BitBrowser account is bound', () => {
    const env = aggregateWorkEnv({ snapshot: healthySnapshot(), cloudUser, binding: { bound: false }, tokenFacts })
    expect(env.workable).toBe(false)
    expect(env.blockers[0]).toContain('尚未绑定比特浏览器账号')
  })

  it('flags a different BitBrowser account as its own blocker', () => {
    const env = aggregateWorkEnv({ snapshot: healthySnapshot({ main_user_id_masked: OTHER_MASK }), cloudUser, binding, tokenFacts })
    expect(env.workable).toBe(false)
    expect(env.bitAccountDiffers).toBe(true)
    expect(env.blockers[0]).toContain('不一致')
  })

  it('surfaces the local-agent reason when the agent is stopped', () => {
    const env = aggregateWorkEnv({ snapshot: healthySnapshot({ status: 'stopped' }), cloudUser, binding, tokenFacts })
    expect(env.workable).toBe(false)
    expect(env.blockers).toContain('本机服务未启动，请重启应用后再检查。')
    expect(env.page.serviceText).toBe('未运行')
  })

  it('surfaces the bitbrowser reason when it is not reachable', () => {
    const env = aggregateWorkEnv({ snapshot: healthySnapshot({ bitbrowser_status: 'unreachable' }), cloudUser, binding, tokenFacts })
    expect(env.workable).toBe(false)
    expect(env.blockers).toContain('未检测到可用的比特浏览器，请先启动比特浏览器后再检测。')
  })

  it('asks for a BitBrowser login when no account was read', () => {
    const env = aggregateWorkEnv({ snapshot: healthySnapshot({ main_user_id_masked: '' }), cloudUser, binding, tokenFacts })
    expect(env.workable).toBe(false)
    expect(env.blockers).toContain('未读取到比特浏览器账号，请确认比特浏览器已登录后重新检测。')
  })

  it('blocks with the credential reason when the process holds no execution credential', () => {
    const env = aggregateWorkEnv({ snapshot: healthySnapshot(), cloudUser, binding, tokenFacts: { configured: false } })
    expect(env.workable).toBe(false)
    expect(env.blockers).toContain('本机执行凭证未配置，任务无法领取；请重新登录或重启应用后重试。')
    expect(env.page.credentialText).toBe('未配置')
  })

  it('keeps 需检查 when there is no logged-in user', () => {
    const env = aggregateWorkEnv({ snapshot: healthySnapshot(), cloudUser: null, binding, tokenFacts })
    expect(env.workable).toBe(false)
    expect(env.blockers).toContain('请先登录运营平台')
  })
})

// 面板三行各说各话：一行坏了不歪曲另一行的事实（此前两行共用 page.bound
// 合取开关，比特浏览器坏会把已连上的本机服务显示成未连接）。
describe('work env rows', () => {
  it('keeps the service row connected while bitbrowser is down', () => {
    const env = aggregateWorkEnv({ snapshot: healthySnapshot({ bitbrowser_status: 'unreachable' }), cloudUser, binding, tokenFacts })
    expect(env.page.serviceText).toBe('已连接，当前无任务')
    expect(env.page.bitbrowserText).toBe('不可达')
  })

  it('reports the credential row from the process facts alone', () => {
    const env = aggregateWorkEnv({ snapshot: healthySnapshot({ main_user_id_masked: '' }), cloudUser, binding, tokenFacts })
    expect(env.page.credentialText).toBe('已配置')
    expect(env.page.bitbrowserText).toBe('可用')
  })

  it('shows the running task on the service row', () => {
    const env = aggregateWorkEnv({ snapshot: healthySnapshot({ status: 'running', current_task_id: 'task-9' }), cloudUser, binding, tokenFacts })
    expect(env.page.serviceText).toBe('已连接，当前任务 task-9')
  })
})

// 比特账号一致性判定是个人中心入口可见性与胶囊结论的共同来源。两侧都只有掩码：
// Cloud 回绑定账号的掩码，本机账号由 Tauri 按同一规则掩掉。
describe('bitAccountDiffers', () => {
  const bound = { bound: true, main_user_id_masked: BOUND_MASK }

  it('is false when the environment account matches the bound one', () => {
    expect(bitAccountDiffers(bound, BOUND_MASK)).toBe(false)
  })

  it('is true when the client switched to another account', () => {
    expect(bitAccountDiffers(bound, OTHER_MASK)).toBe(true)
  })

  it('is false when no bit account is bound yet', () => {
    expect(bitAccountDiffers({ bound: false, main_user_id_masked: '' }, BOUND_MASK)).toBe(false)
  })

  // 闸门是 bound，不是「掩码非空」：只凭一段残留掩码就宣告不一致，
  // 会让未绑定的账号被推去点一次无意义的换绑。
  it('gates on the bound flag rather than on a leftover mask', () => {
    expect(bitAccountDiffers({ bound: false, main_user_id_masked: BOUND_MASK }, OTHER_MASK)).toBe(false)
  })

  it('is false when the environment reports no account at all', () => {
    expect(bitAccountDiffers(bound, '')).toBe(false)
    expect(bitAccountDiffers(bound, undefined)).toBe(false)
  })

  // 比对不了时倒向「不一致」而不是「一致」：后端把 8 位以内的账号整体掩成 ****，
  // 这种值不能证明两侧是同一个账号，多给一次自助换绑入口好过静默放行。
  it('reads a masked-out value as differing rather than as matching', () => {
    expect(bitAccountDiffers({ bound: true, main_user_id_masked: '****' }, BOUND_MASK)).toBe(true)
    expect(bitAccountDiffers(bound, '****')).toBe(true)
  })

  // 唯一判定源：同一输入下，个人中心读的函数与胶囊读的聚合字段必须同值。
  it('agrees with the aggregate on the same inputs', () => {
    const cases = [
      bound,
      { bound: false, main_user_id_masked: '' },
      { bound: false, main_user_id_masked: BOUND_MASK },
      { bound: true, main_user_id_masked: '****' },
    ]
    for (const b of cases) {
      for (const local of [BOUND_MASK, OTHER_MASK, '', '****']) {
        const env = aggregateWorkEnv({ snapshot: healthySnapshot({ main_user_id_masked: local }), cloudUser, binding: b, tokenFacts })
        expect(env.bitAccountDiffers).toBe(bitAccountDiffers(b, local))
      }
    }
  })
})

// 接线用例：四项输入一次读齐（云端用户、云端绑定、本机状态、凭证事实）。
describe('loadWorkEnvInputs', () => {
  function deps(overrides = {}) {
    return {
      session: { me: vi.fn(async () => ({ id: 'u-1', username: 'op' })) },
      bitAccounts: { get: vi.fn(async () => ({ bound: true, main_user_id_masked: BOUND_MASK })) },
      service: {
        status: vi.fn(async () => healthySnapshot()),
        tokenFacts: vi.fn(async () => tokenFacts),
      },
      ...overrides,
    }
  }

  it('reads all four inputs in one load', async () => {
    const d = deps()
    const inputs = await loadWorkEnvInputs(d)

    expect(d.session.me).toHaveBeenCalledTimes(1)
    expect(d.bitAccounts.get).toHaveBeenCalledTimes(1)
    expect(d.service.status).toHaveBeenCalledTimes(1)
    expect(d.service.tokenFacts).toHaveBeenCalledTimes(1)
    expect(inputs.cloudUser).toEqual({ id: 'u-1', username: 'op' })
    expect(inputs.tokenFacts).toEqual(tokenFacts)
    expect(inputs.snapshot.status).toBe('idle')
    expect(inputs.binding).toEqual({ bound: true, main_user_id_masked: BOUND_MASK })
    expect(inputs.errors).toEqual([])
  })

  // 冷启动竞态：sidecar 还没监听时 local_agent_status 直接抛错，另外三项照样可用。
  it('keeps the other inputs when the local agent is not up yet', async () => {
    const failure = new Error('agent unreachable: connection refused')
    const inputs = await loadWorkEnvInputs(deps({
      service: { status: vi.fn(async () => { throw failure }), tokenFacts: vi.fn(async () => tokenFacts) },
    }))

    expect(inputs.snapshot).toBeNull()
    expect(inputs.cloudUser).toEqual({ id: 'u-1', username: 'op' })
    expect(inputs.tokenFacts).toEqual(tokenFacts)
    expect(inputs.binding).toEqual({ bound: true, main_user_id_masked: BOUND_MASK })
    expect(inputs.errors).toEqual([failure])
  })

  it('treats missing credential facts as absent, not as a failed load', async () => {
    const failure = new Error('no facts')
    const inputs = await loadWorkEnvInputs(deps({
      service: { status: vi.fn(async () => healthySnapshot()), tokenFacts: vi.fn(async () => { throw failure }) },
    }))

    expect(inputs.tokenFacts).toBeNull()
    expect(inputs.binding).toEqual({ bound: true, main_user_id_masked: BOUND_MASK })
    expect(inputs.errors).toEqual([failure])
  })
})

// 接线用例：真实线格式经 normalizeLocalAgentStatus（service 边界）后进聚合，
// 钉死「service 输出形状 = 聚合输入形状」这条链——形状漂移在此变红。
describe('work env wiring from the agent wire shape', () => {
  it('reports a fully green panel from a normalized real-agent snapshot', () => {
    const snapshot = normalizeLocalAgentStatus(healthySnapshot())
    const env = aggregateWorkEnv({ snapshot, cloudUser, binding, tokenFacts })
    expect(env.workable).toBe(true)
    expect(env.blockers).toEqual([])
    expect(env.page.serviceText).toBe('已连接，当前无任务')
    expect(env.page.bitbrowserText).toBe('已登录指定账号')
  })

  it('degrades to 未检测 (not 未知) when the wire snapshot lacks bitbrowser_status', () => {
    const snapshot = normalizeLocalAgentStatus({ status: 'idle' })
    const env = aggregateWorkEnv({ snapshot, cloudUser, binding, tokenFacts })
    expect(env.workable).toBe(false)
    expect(env.page.bitbrowserText).toBe('未检测')
    expect(env.page.bitbrowserStatusText).toBe('未检测')
  })
})
