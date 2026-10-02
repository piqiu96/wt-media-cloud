import { describe, expect, it } from 'vitest'
import { aggregateWorkEnv } from './apps/desktop/features/local-agent/work-env-status.js'
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
