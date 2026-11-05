import { readFileSync } from 'node:fs'
import { describe, expect, it, vi } from 'vitest'

const source = readFileSync(new URL('./DiscoveryStrategiesPage.vue', import.meta.url), 'utf8')
function setup(manage = true) {
  const state = { canManage: { value: manage }, statusPendingID: { value: null }, error: { value: '' } }
  let confirmation
  const hide = vi.fn(), load = vi.fn(), setStrategyStatus = vi.fn().mockResolvedValue({})
  const DialogPlugin = { confirm: vi.fn(options => { confirmation = options; return { hide } }) }
  const feedback = { toast: { success: vi.fn(), error: vi.fn() } }
  const command = source.match(/(?:async )?function toggle\([^]*?\n}/)[0]
  const toggle = new Function('canManage', 'statusPendingID', 'error', 'client', 'load', 'DialogPlugin', 'feedback', `${command}; return toggle`)(state.canManage, state.statusPendingID, state.error, { setStrategyStatus }, load, DialogPlugin, feedback)
  return { ...state, toggle, confirmation: () => confirmation, setStrategyStatus, load, hide }
}

describe('discovery list status control', () => {
  it('waits for confirmation and sends only the requested status', async () => {
    const ctx = setup(), row = { id: 8, name: '关键词策略', status: 'enabled' }
    ctx.toggle(row, false)
    expect(ctx.setStrategyStatus).not.toHaveBeenCalled()
    expect(row.status).toBe('enabled')
    await ctx.confirmation().onConfirm()
    expect(ctx.setStrategyStatus).toHaveBeenCalledWith(8, 'disabled')
    expect(ctx.load).toHaveBeenCalledOnce()
    expect(ctx.statusPendingID.value).toBeNull()
    expect(ctx.hide).toHaveBeenCalledOnce()
  })

  it('leaves the displayed status unchanged on failure and allows a later retry', async () => {
    const ctx = setup(), row = { id: 8, status: 'disabled' }
    ctx.setStrategyStatus.mockRejectedValue(new Error('无权限'))
    ctx.toggle(row, true)
    await ctx.confirmation().onConfirm()
    expect(row.status).toBe('disabled')
    expect(ctx.error.value).toBe('无权限')
    expect(ctx.load).not.toHaveBeenCalled()
    expect(ctx.statusPendingID.value).toBeNull()
  })

  it('blocks readonly roles and duplicate submissions', async () => {
    const readonly = setup(false)
    readonly.toggle({ id: 8, status: 'disabled' }, true)
    expect(readonly.confirmation()).toBeUndefined()
    const ctx = setup(), row = { id: 8, status: 'disabled' }
    ctx.toggle(row, true)
    let finish
    ctx.setStrategyStatus.mockReturnValue(new Promise(resolve => { finish = resolve }))
    const pending = ctx.confirmation().onConfirm()
    await ctx.confirmation().onConfirm()
    expect(ctx.setStrategyStatus).toHaveBeenCalledOnce()
    expect(ctx.statusPendingID.value).toBe(8)
    finish({})
    await pending
    expect(ctx.statusPendingID.value).toBeNull()
  })
})
