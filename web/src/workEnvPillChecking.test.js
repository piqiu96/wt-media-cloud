import { createSSRApp } from 'vue'
import { renderToString } from 'vue/server-renderer'
import { beforeEach, expect, it, vi } from 'vitest'
import WorkEnvPill from './apps/desktop/features/local-agent/WorkEnvPill.vue'
import { loadWorkEnvInputs } from './apps/desktop/features/local-agent/work-env-status.js'

vi.mock('./apps/desktop/features/local-agent/work-env-status.js', async original => ({
  ...await original(), loadWorkEnvInputs: vi.fn(),
}))
beforeEach(() => vi.clearAllMocks())
const inputs = snapshot => ({ snapshot, cloudUser: { id: '3', username: 'operator' },
  binding: { bound: true, main_user_id_masked: '1234****5678' }, tokenFacts: { configured: true }, errors: [] })
async function state() {
  let value
  await renderToString(createSSRApp({ setup(props, context) {
    value = WorkEnvPill.setup(props, context)
    return () => null
  } }))
  return value
}
it('shows a neutral check before any environment result exists', async () => {
  const html = await renderToString(createSSRApp(WorkEnvPill))
  expect(html).toContain('正在检查工作环境')
  expect(html).not.toContain('工作环境需检查')
})
it('does not report an unchecked environment as ready while a manual check is pending', async () => {
  const s = await state()
  let resolve
  loadWorkEnvInputs.mockImplementationOnce(() => new Promise(done => { resolve = done }))
  const pending = s.manualCheck()
  expect(s.checking.value).toBe(true)
  expect(s.env.value.workable).toBe(false)
  resolve(inputs({ status: 'idle', bitbrowser_status: 'normal', main_user_id_masked: '1234****5678' }))
  await pending
  expect(s.checking.value).toBe(false)
  expect(s.env.value.workable).toBe(true)
})
it('still reports a real missing service after the check has finished', async () => {
  const s = await state()
  loadWorkEnvInputs.mockResolvedValueOnce({ ...inputs(null), errors: [new Error('connection refused')] })
  await s.manualCheck()
  expect(s.checking.value).toBe(false)
  expect(s.env.value.workable).toBe(false)
  expect(s.env.value.blockers).toContain('本机服务未启动，请重启应用后再检查。')
})
