import { describe, expect, it } from 'vitest'
import { aggregateWorkEnv } from './apps/desktop/features/local-agent/work-env-status.js'

// 快照用 normalizeLocalAgentStatus 的产物（camelCase）——createLocalAgentStatusPage
// 读的正是这个形状，不是 Agent 上报的原始 snake_case。
function healthySnapshot(overrides = {}) {
  return {
    nodeId: 'agent-node_abc',
    agentId: 'local-agent-dev',
    status: 'idle',
    bitbrowserStatus: 'normal',
    mainUserId: 'main-1',
    operatingSystem: 'darwin',
    cpuArchitecture: 'arm64',
    agentVersion: '1.0.0',
    currentTaskId: null,
    currentTaskStatus: null,
    currentTaskProgress: null,
    pendingResultCount: 0,
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
    expect(env.blockers).toContain('本机执行服务未运行，请先启动后再检测。')
  })

  it('surfaces the bitbrowser reason when it is not reachable', () => {
    const env = aggregateWorkEnv({ snapshot: healthySnapshot({ bitbrowserStatus: 'unreachable' }), cloudUser, binding, localDevice })
    expect(env.workable).toBe(false)
    expect(env.blockers).toContain('未检测到可用的比特浏览器，请先启动比特浏览器后再检测。')
  })

  it('surfaces the missing-node reason when no node is connected', () => {
    const env = aggregateWorkEnv({ snapshot: healthySnapshot({ nodeId: '' }), cloudUser, binding, localDevice })
    expect(env.workable).toBe(false)
    expect(env.blockers).toContain('当前会话尚未连接本机执行节点；请查看个人信息中的设备绑定状态。')
  })

  it('keeps 需检查 when there is no logged-in user', () => {
    const env = aggregateWorkEnv({ snapshot: healthySnapshot(), cloudUser: null, binding, localDevice })
    expect(env.workable).toBe(false)
    expect(env.blockers).toContain('请先登录运营平台')
  })
})
