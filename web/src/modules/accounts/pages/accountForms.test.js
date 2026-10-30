import { readFileSync } from 'node:fs'
import { describe, expect, it, vi } from 'vitest'

const source = readFileSync(new URL('./AccountsPage.vue', import.meta.url), 'utf8')
function command(name) {
  const match = source.match(new RegExp(`(?:async )?function ${name}\\([^]*?\\n}`))
  if (!match) throw new Error(`Missing page command: ${name}`)
  return match[0]
}
function context(confirm = async () => false) {
  const createForm = { value: { platform: 'bilibili', categoryIds: [], tags: [], remark: '' } }
  const state = { createForm, createCategoryType: { value: '' }, editCategoryType: { value: 'game' }, editRemark: { value: '原备注' }, editCategoryIds: { value: ['1'] }, editTags: { value: ['原标签'] }, showCreate: { value: true }, editVisible: { value: true }, creating: { value: false }, savingEdit: { value: false }, editAccount: { value: { id: '1' } }, createError: { value: '' }, editError: { value: '' }, error: { value: '' }, createdGuide: { value: null }, accounts: { value: [] }, loadAccounts: vi.fn(), validateCategorySelection: () => '', accountClient: { create: vi.fn(), update: vi.fn() }, feedback: { confirmWarning: vi.fn(confirm), toast: { success: vi.fn() } } }
  const declarations = ['accountFormSnapshot', 'closeAccountForm', 'createAccount', 'saveEdit'].map(command).join('\n')
  const commands = new Function(...Object.keys(state), `let closingAccountForm = false; let createSnapshot, editSnapshot; ${declarations}; createSnapshot = accountFormSnapshot('create'); editSnapshot = accountFormSnapshot('edit'); return { closeAccountForm, createAccount, saveEdit }`)(...Object.values(state))
  return { ...state, ...commands }
}
describe('account create/edit form lifecycle', () => {
  for (const kind of ['create', 'edit']) {
    it(`${kind}: leaves changed input intact when discard is cancelled, closes after confirmation`, async () => {
      const c = context(), visible = kind === 'create' ? c.showCreate : c.editVisible
      if (kind === 'create') c.createForm.value.tags = ['新标签']; else c.editRemark.value = '修改后'
      await c.closeAccountForm(kind)
      expect(visible.value).toBe(true)
      expect(c.feedback.confirmWarning).toHaveBeenCalledOnce()
      c.feedback.confirmWarning.mockResolvedValueOnce(true)
      await c.closeAccountForm(kind)
      expect(visible.value).toBe(false)
    })
    it(`${kind}: closes an untouched form without a confirmation`, async () => {
      const c = context(); await c.closeAccountForm(kind)
      expect((kind === 'create' ? c.showCreate : c.editVisible).value).toBe(false)
      expect(c.feedback.confirmWarning).not.toHaveBeenCalled()
    })
    it(`${kind}: blocks duplicate submission and closing until save completes`, async () => {
      const c = context(); let finish
      const api = kind === 'create' ? c.accountClient.create : c.accountClient.update
      api.mockImplementation(() => new Promise(resolve => { finish = resolve }))
      const save = kind === 'create' ? c.createAccount : c.saveEdit
      const first = save(); await save(); await c.closeAccountForm(kind)
      expect(api).toHaveBeenCalledOnce()
      expect((kind === 'create' ? c.showCreate : c.editVisible).value).toBe(true)
      finish({ id: '1' }); await first
      expect((kind === 'create' ? c.showCreate : c.editVisible).value).toBe(false)
    })
    it(`${kind}: keeps input and the form open after a failed save, permitting retry`, async () => {
      const c = context(), api = kind === 'create' ? c.accountClient.create : c.accountClient.update
      api.mockRejectedValueOnce(new Error('暂时无法保存'))
      const save = kind === 'create' ? c.createAccount : c.saveEdit
      await save()
      expect((kind === 'create' ? c.showCreate : c.editVisible).value).toBe(true)
      expect((kind === 'create' ? c.createError : c.editError).value).toBe('暂时无法保存')
      expect(kind === 'create' ? c.createForm.value.remark : c.editRemark.value).toBe(kind === 'create' ? '' : '原备注')
      api.mockResolvedValueOnce({ id: '1' }); await save()
      expect(api).toHaveBeenCalledTimes(2)
    })
  }
})


describe('Cookie editor close lifecycle', () => {
  function cookieContext() {
    const state = { updatingCookie: { value: false }, cookieInput: { value: 'demo-only' }, cookieUnresolved: { value: false }, cookieUpdateVisible: { value: true }, feedback: { confirmWarning: vi.fn(async () => false) } }
    const close = new Function(...Object.keys(state), `${command('closeCookieUpdate')}; return closeCookieUpdate`)(...Object.values(state))
    return { ...state, close }
  }
  it('retains the draft when discard is cancelled and closes after confirmation', async () => {
    const c = cookieContext(); await c.close()
    expect(c.cookieUpdateVisible.value).toBe(true); expect(c.cookieInput.value).toBe('demo-only')
    c.feedback.confirmWarning.mockResolvedValueOnce(true); await c.close()
    expect(c.cookieUpdateVisible.value).toBe(false)
  })
  it('blocks closing while processing and does not discard an unresolved receipt', async () => {
    const c = cookieContext(); c.updatingCookie.value = true; await c.close()
    expect(c.cookieUpdateVisible.value).toBe(true); expect(c.feedback.confirmWarning).not.toHaveBeenCalled()
    c.updatingCookie.value = false; c.cookieUnresolved.value = true; await c.close()
    expect(c.cookieUpdateVisible.value).toBe(false); expect(c.feedback.confirmWarning).not.toHaveBeenCalled()
  })
  it('closes an empty draft without a confirmation', async () => {
    const c = cookieContext(); c.cookieInput.value = ' '; await c.close()
    expect(c.cookieUpdateVisible.value).toBe(false); expect(c.feedback.confirmWarning).not.toHaveBeenCalled()
  })
})
