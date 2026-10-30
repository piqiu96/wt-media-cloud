import { describe, expect, it, vi } from 'vitest'
import { createProfileProxyWorkflow } from './profileProxyWorkflow.js'
function fixture(overrides = {}) {
  const operation = { operation_id: 'op1', action: 'apply', proxy_id: 42, bit_profile_id: 'bit1', agent_required: true, outcome: 'pending', password: 'secret' }
  const deps = { start: vi.fn().mockResolvedValue(operation), current: vi.fn().mockResolvedValue(operation), write: vi.fn().mockResolvedValue({ accepted: true }), finish: vi.fn().mockResolvedValue({}), ...overrides }
  return { deps, operation, flow: createProfileProxyWorkflow(deps) }
}
describe('profile proxy write and recovery', () => {
  it('settles explicit acknowledgement once, without reading again', async () => {
    const { flow, deps } = fixture(); await flow.execute('p1', { action: 'apply', proxy_id: 42 })
    expect(deps.current).not.toHaveBeenCalled()
    expect(deps.write).toHaveBeenCalledTimes(1)
    expect(deps.finish).toHaveBeenCalledWith('p1', expect.objectContaining({ operation_id: 'op1', outcome: 'success' }))
  })
  it('clears reservations only for explicit refusal', async () => {
    const error = { code: 'proxy_operation_rejected', message: '拒绝' }
    const { flow, deps } = fixture({ write: vi.fn().mockRejectedValue(error) })
    await expect(flow.execute('p1', {})).rejects.toEqual(error)
    expect(deps.finish).toHaveBeenCalledWith('p1', expect.objectContaining({ outcome: 'failed' }))
  })
  it.each([new Error('timeout'), { code: 'local_agent_unreachable' }, { code: 'proxy_result_unconfirmed' }])('preserves the reservation for unknown writes', async error => {
    const { flow, deps } = fixture({ write: vi.fn().mockRejectedValue(error) })
    await expect(flow.execute('p1', {})).rejects.toMatchObject({ unconfirmed: true })
    expect(deps.finish).not.toHaveBeenCalled()
  })
  it('a lost settlement acknowledgement never triggers another BitBrowser write', async () => {
    const { flow, deps, operation } = fixture({ finish: vi.fn().mockRejectedValue(new Error('offline')) })
    await expect(flow.execute('p1', {})).rejects.toMatchObject({ unconfirmed: true })
    deps.current.mockResolvedValue({ ...operation, outcome: 'success' }); await flow.recover('p1', { operationId: 'op1' })
    expect(deps.write).toHaveBeenCalledTimes(1)
  })
  it('does not mistake an older terminal receipt for a lost start response', async () => {
    const { flow, deps, operation } = fixture()
    deps.current.mockResolvedValue({ ...operation, outcome: 'success' })
    await expect(flow.recover('p1', { action: 'apply', proxyId: 42 })).rejects.toMatchObject({ unconfirmed: true })
    expect(deps.write).not.toHaveBeenCalled()
  })
  it('rejects another operation receipt before touching the local browser', async () => {
    const { flow, deps } = fixture()
    await expect(flow.recover('p1', { operationId: 'older', action: 'apply', proxyId: 42 })).rejects.toMatchObject({ unconfirmed: true })
    expect(deps.write).not.toHaveBeenCalled()
  })
  it.each([['matches', 'success'], ['different', 'failed']])('settles locally compared %s as %s', async (comparison, outcome) => {
    const { flow, deps } = fixture({ write: vi.fn().mockResolvedValue({ comparison }) })
    expect(await flow.recover('p1')).toEqual({ outcome })
    expect(deps.start).not.toHaveBeenCalled()
    expect(deps.write).toHaveBeenCalledWith(expect.objectContaining({ verify: true }))
    expect(deps.finish).toHaveBeenCalledWith('p1', expect.objectContaining({ operation_id: 'op1', outcome }))
  })
  it('does not settle when credentials are unavailable or masked', async () => {
    const { flow, deps } = fixture({ write: vi.fn().mockResolvedValue({ comparison: 'unverifiable' }) })
    await expect(flow.recover('p1')).rejects.toMatchObject({ unconfirmed: true })
    expect(deps.finish).not.toHaveBeenCalled()
  })
})
