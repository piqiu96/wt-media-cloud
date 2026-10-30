const REFUSALS = new Set(['proxy_operation_rejected', 'proxy_assignment_rejected', 'proxy_mutation_input_invalid', 'proxy_input_invalid', 'profile_id_required', 'proxy_action_invalid'])
function uncertain(error, operation) {
  const next = new Error(error?.message || '操作结果待核实，请先读取实际配置。')
  next.unconfirmed = true
  next.operationId = operation?.operation_id || ''
  return next
}
export function createProfileProxyWorkflow({ start, current, write, finish }) {
  const args = operation => ({ action: operation.action, bitProfileId: operation.bit_profile_id, proxyProtocol: operation.protocol, host: operation.host, port: operation.port, username: operation.username, password: operation.password })
  const settle = (profileId, operation, outcome) => finish(profileId, { operation_id: operation.operation_id, action: operation.action, proxy_id: operation.proxy_id, outcome, message: '' })
  return {
    async execute(profileId, input) {
      let operation
      try { operation = await start(profileId, input) }
      catch (error) {
        // A malformed/transport response can hide a reservation; never blind-retry it.
        if (!error?.errcode || error.errcode === 10001 || error.errcode === 99999) throw uncertain(error)
        throw error
      }
      if (!operation.agent_required) return operation
      if (!operation.operation_id) throw uncertain(new Error('服务版本不支持安全结算，请升级后核实操作。'), operation)
      try {
        const result = await write(args(operation))
        if (!result?.accepted) throw new Error('未收到比特明确成功响应')
      } catch (error) {
        if (!REFUSALS.has(error?.code)) throw uncertain(error, operation)
        try { await settle(profileId, operation, 'failed') } catch (settlementError) { throw uncertain(settlementError, operation) }
        throw error
      }
      try { return await settle(profileId, operation, 'success') }
      catch (error) { throw uncertain(error, operation) }
    },
    async recover(profileId, expected = {}) {
      let operation
      try { operation = await current(profileId) } catch (error) { throw uncertain(error) }
      if (expected.operationId && operation.operation_id !== expected.operationId) throw uncertain(new Error('操作记录已变化，请重新查看窗口状态。'))
      if (expected.action && operation.action !== expected.action) throw uncertain(new Error('当前操作与待核实操作不一致，请重新查看窗口状态。'))
      if (expected.proxyId && operation.proxy_id !== expected.proxyId) throw uncertain(new Error('目标代理已变化，请重新查看窗口状态。'))
      // A lost start response has no operation ID. A prior terminal receipt is
      // not proof that this new attempt ran, even if action and proxy match.
      if (!expected.operationId && operation.outcome !== 'pending') throw uncertain(new Error('无法确认本次操作是否已开始，请查看当前代理关系后再处理。'))
      if (operation.outcome === 'success') return { outcome: 'success' }
      if (operation.outcome === 'failed') return { outcome: 'failed' }
      if (!operation.operation_id || operation.outcome !== 'pending') throw uncertain(new Error('没有可核实的操作记录，请联系管理员核查。'))
      let result
      try { result = await write({ ...args(operation), verify: true }) } catch (error) { throw uncertain(error) }
      if (!['matches', 'different'].includes(result?.comparison)) throw uncertain(new Error('比特未提供完整配置，暂时无法核实。预留已保留，请稍后再试。'))
      const outcome = result.comparison === 'matches' ? 'success' : 'failed'
      try { await settle(profileId, operation, outcome) } catch (error) { throw uncertain(error) }
      return { outcome }
    },
  }
}
