import { readFileSync } from 'node:fs'
import { describe, expect, it, vi } from 'vitest'

function deferred() { let resolve; const promise = new Promise(r => { resolve = r }); return { promise, resolve } }
for (const page of ['ComposeTasksPage', 'MyOutputsPage']) {
  const source = readFileSync(new URL(`./${page}.vue`, import.meta.url), 'utf8')
  function context() {
    const filters = { value: { team_id: '1', operator: 'op:1' } }
    const operatorOptions = { value: [] }, operatorsLoading = { value: false }
    const client = { listComposeOperators: vi.fn() }
    const command = source.slice(source.indexOf('async function loadOperators()'), source.indexOf('\nfunction onTeamFilterChange()'))
    const run = new Function('filters', 'operatorOptions', 'operatorsLoading', 'client', 'canFilterOperators', 'isAdmin', 'missingOperatorOption', 'operatorFilterOptions', `let operatorRequest = 0; ${command}; return loadOperators`)(filters, operatorOptions, operatorsLoading, client, { value: true }, { value: true }, () => false, { value: [] })
    return { run, filters, operatorOptions, operatorsLoading, client }
  }
  describe(`${page} team/member linkage`, () => {
    it('ignores an older team response, including its loading completion', async () => {
      const c = context(), first = deferred(), second = deferred()
      c.client.listComposeOperators.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise)
      const a = c.run(); c.filters.value.team_id = '2'; const b = c.run()
      first.resolve([{ id: 1 }]); await a
      expect(c.operatorOptions.value).toEqual([])
      expect(c.operatorsLoading.value).toBe(true)
      second.resolve([{ id: 2 }]); await b
      expect(c.operatorOptions.value).toEqual([{ id: 2 }])
      expect(c.operatorsLoading.value).toBe(false)
    })
    it('clearing the team clears members and invalidates an in-flight response', async () => {
      const c = context(), first = deferred()
      c.client.listComposeOperators.mockReturnValueOnce(first.promise)
      const a = c.run(); c.filters.value.team_id = ''; await c.run()
      first.resolve([{ id: 1 }]); await a
      expect(c.operatorOptions.value).toEqual([])
      expect(c.filters.value.operator).toBe('')
      expect(c.operatorsLoading.value).toBe(false)
    })
  })
}

describe('ComposeStrategyPage member choices', () => {
  it('removes old candidates immediately while loading a new team/category', async () => {
    const source = readFileSync(new URL('./ComposeStrategyPage.vue', import.meta.url), 'utf8')
    const command = source.slice(source.indexOf('async function loadOperatorOptions()'), source.indexOf('\nwatch(() => [visible.value, form.value.team_id'))
    const form = { value: { team_id: 2, category_id: 'game-2', operator_id: '' } }
    const operatorOptions = { value: [{ id: 1 }] }, operatorsLoading = { value: false }, pending = deferred()
    const run = new Function('form', 'operatorOptions', 'operatorsLoading', 'operatorError', 'client', 'isAdmin', 'currentUser', `let operatorSequence = 0; ${command}; return loadOperatorOptions`)(form, operatorOptions, operatorsLoading, { value: '' }, { listComposeOperators: () => pending.promise }, { value: true }, { value: null })
    const done = run()
    expect(operatorOptions.value).toEqual([])
    expect(operatorsLoading.value).toBe(true)
    pending.resolve([{ id: 2 }]); await done
    expect(operatorOptions.value).toEqual([{ id: 2 }])
  })
})
