import { describe, expect, it, vi } from 'vitest'
import { createProfileSyncWorkflow } from './profileSyncWorkflow.js'

function fixture(overrides = {}) {
  const state = {}
  const scan = { id: 's1', status: 'ready', profiles: [{ bit_profile_id: 'p1' }], diff: [{ kind: 'added', bit_profile_id: 'p1' }] }
  const deps = { scan: vi.fn().mockResolvedValue({ profiles: [{}] }), submit: vi.fn().mockResolvedValue(scan),
    confirm: vi.fn().mockResolvedValue({ ...scan, status: 'confirmed' }), review: vi.fn().mockResolvedValue(scan),
    saved: vi.fn().mockResolvedValue(undefined), preview: vi.fn(), change: patch => Object.assign(state, patch), ...overrides }
  return { state, deps, flow: createProfileSyncWorkflow(deps), scan }
}
describe('profile sync workflow', () => {
  it('reads, compares and previews directly without a second completion dialog', async () => {
    const { flow, state, deps } = fixture()
    await flow.start()
    expect(state.phase).toBe('preview')
    expect(deps.preview).toHaveBeenCalledWith(expect.objectContaining({ id: 's1' }), expect.anything())
    await flow.confirm(['p1'])
    expect(deps.confirm).toHaveBeenCalledWith('s1', ['p1'])
    expect(state.phase).toBe('result')
  })
  it('does not submit empty local snapshots when the Bit main account cannot be verified', async () => {
    const { flow, state, deps } = fixture({ scan: vi.fn().mockResolvedValue({ profiles: [] }) })
    await flow.start()
    expect(state.phase).toBe('empty')
    expect(deps.submit).not.toHaveBeenCalled()
  })
  it('preserves read failures and never retries automatically', async () => {
    const { flow, state, deps } = fixture({ scan: vi.fn().mockRejectedValue(new Error('比特未启动')) })
    await flow.start()
    expect(state).toMatchObject({ phase: 'read-error', error: '比特未启动' })
    expect(deps.scan).toHaveBeenCalledTimes(1)
    expect(deps.submit).not.toHaveBeenCalled()
  })
  it.each([[23002, 'identity'], [11003, 'forbidden'], [23008, 'unbound'], [20004, 'expired']])('classifies business error %s', async (errcode, phase) => {
    const { flow, state } = fixture({ submit: vi.fn().mockRejectedValue({ errcode, message: '原因' }) })
    await flow.start()
    expect(state.phase).toBe(phase)
  })
  it('blocks duplicate clicks and confirm before preview', async () => {
    let resolve
    const { flow, deps, state } = fixture({ scan: vi.fn(() => new Promise(r => { resolve = r })) })
    await flow.confirm([])
    const pending = flow.start()
    await flow.start()
    expect(deps.scan).toHaveBeenCalledTimes(1)
    resolve({ profiles: [{}] }); await pending
    expect(state.phase).toBe('preview')
    expect(deps.confirm).not.toHaveBeenCalled()
  })
  it('never resubmits an uncertain confirmation until explicitly verified', async () => {
    const { flow, state, deps, scan } = fixture({ confirm: vi.fn().mockRejectedValue(new TypeError('网络断开')) })
    await flow.start(); await flow.confirm(['p1']); await flow.confirm(['p1'])
    expect(state.phase).toBe('uncertain')
    expect(deps.confirm).toHaveBeenCalledTimes(1)
    deps.review.mockResolvedValue({ ...scan, status: 'confirmed' })
    await flow.verify()
    expect(state.phase).toBe('result')
    expect(deps.saved).toHaveBeenCalledTimes(1)
    expect(deps.confirm).toHaveBeenCalledTimes(1)
  })
  it('allows an explicit retry only for the same pending snapshot after verification', async () => {
    const { flow, state, deps } = fixture({ confirm: vi.fn().mockRejectedValue(new Error('timeout')) })
    await flow.start(); await flow.confirm(['p1']); await flow.verify()
    expect(state.phase).toBe('apply-error')
    deps.confirm.mockResolvedValue({ id: 's1', status: 'confirmed', profiles: [] })
    await flow.confirm(['p1'])
    expect(state.phase).toBe('result')
  })
  it('rejects a replaced snapshot and keeps unknown results unknown when review fails', async () => {
    const { flow, state, deps } = fixture({ confirm: vi.fn().mockRejectedValue(new Error('timeout')) })
    await flow.start(); await flow.confirm([])
    deps.review.mockRejectedValue(new Error('offline')); await flow.verify()
    expect(state.phase).toBe('uncertain')
    deps.review.mockResolvedValue({ id: 's2', status: 'ready' }); await flow.verify()
    expect(state.phase).toBe('expired')
    await flow.confirm([]); expect(deps.confirm).toHaveBeenCalledTimes(1)
  })
  it('keeps a saved receipt when list reload fails, and reloads without confirming again', async () => {
    const { flow, state, deps } = fixture({ saved: vi.fn().mockRejectedValue(new Error('列表加载失败')) })
    await flow.start(); await flow.confirm([])
    expect(state.phase).toBe('reload-error')
    deps.saved.mockResolvedValue(undefined); await flow.reload()
    expect(state.phase).toBe('result')
    expect(deps.confirm).toHaveBeenCalledTimes(1)
  })
  it('retains explicit refusal for safe retry without treating malformed responses as refusal', async () => {
    const { flow, state, deps } = fixture({ confirm: vi.fn().mockRejectedValue({ errcode: 20003, message: '资源已改变' }) })
    await flow.start(); await flow.confirm([])
    expect(state.phase).toBe('apply-error')
    deps.confirm.mockRejectedValue({ errcode: 10001, message: 'Invalid response' }); await flow.confirm([])
    expect(state.phase).toBe('uncertain')
  })
})
