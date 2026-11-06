import { describe, expect, it } from 'vitest'
import { taskStrategyFilters, taskStrategyContext } from './taskCreationContext.js'
const strategy = { id: 10, category_id: 3, team_id: 2, operator_id: 8, operator_name: '运营甲', status: 'enabled' }
const admin = { id: 1, role: 'admin' }
describe('strategy-first task creation', () => {
  it('filters by category within the existing role scope, without derived ownership', () => {
    expect(taskStrategyFilters(3, admin)).toEqual({ category_id: 3, status: 'enabled' })
    expect(taskStrategyFilters(3, { id: 5, role: 'senior_operator', team_id: 2 })).toEqual({ category_id: 3, status: 'enabled', team_id: 2 })
    expect(taskStrategyFilters(3, { id: 8, role: 'operator', team_id: 2 })).toEqual({ category_id: 3, status: 'enabled', team_id: 2, operator_id: 8 })
  })
  it('inherits the strategy owner rather than the person creating the task', () => {
    expect(taskStrategyContext(strategy, 10, 3, admin)).toEqual({ team_id: 2, operator_id: 8 })
  })
  it('rejects cleared, stale, disabled, incomplete or out-of-scope strategy details', () => {
    expect(taskStrategyContext(null, 10, 3, admin)).toBeNull()
    for (const patch of [{ id: 11 }, { category_id: 4 }, { status: 'disabled' }, { operator_id: null }, { team_id: null }]) {
      expect(taskStrategyContext({ ...strategy, ...patch }, 10, 3, admin)).toBeNull()
    }
    expect(taskStrategyContext(strategy, 10, 3, { id: 5, role: 'senior_operator', team_id: 9 })).toBeNull()
    expect(taskStrategyContext(strategy, 10, 3, { id: 9, role: 'operator', team_id: 2 })).toBeNull()
    expect(taskStrategyContext(strategy, 10, 3, null)).toBeNull()
  })
})
