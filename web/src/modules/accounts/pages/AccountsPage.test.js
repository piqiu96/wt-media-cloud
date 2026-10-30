import { readFileSync } from 'node:fs'
import { describe, expect, it, vi } from 'vitest'

const source = readFileSync(new URL('./AccountsPage.vue', import.meta.url), 'utf8')

describe('account status after creating an unchecked ledger', () => {
  const body = source.slice(source.indexOf('function accountStatusKey('), source.indexOf('\nfunction accountStatusText('))
  const status = new Function(`${body}; return accountStatusKey`)()
  it('treats an unknown login result as pending, never as a failed check', () => {
    expect(status({ platform_login_status: 'unknown', identification_status: 'unidentified' })).toBe('pending')
  })
  it('distinguishes duplicate ledger records from a different account logged into the window', () => {
    expect(status({ identification_status: 'duplicate', duplicate_of_account_id: '10', platform_login_status: 'account_mismatch' })).toBe('duplicate')
    expect(status({ identification_status: 'identified', platform_login_status: 'account_mismatch' })).toBe('account_mismatch')
  })
  it('continues to show actual check failures and environment errors', () => {
    expect(status({ platform_login_status: 'unknown', last_check_status: 'failed' })).toBe('check_failed')
    expect(status({ platform_login_status: 'environment_error' })).toBe('check_failed')
  })
})

describe('media account resource page', () => {
  it('uses shared resource layout and preserves account operations', () => {
    expect(source).toContain('ResourcePageHeader')
    expect(source).toContain('ResourceCard')
    expect(source).toContain('ResourceStatGrid')
    expect(source).toContain('ResourceStatusBadge')
    expect(source).toContain('wt-resource-table')
    expect(source).toContain('resourceAccountStats')
    expect(source).toContain('@click="openCreate"')
    expect(source).toContain('@click="checkFromRow(row)"')
    expect(source).toContain('toggleEnabled(account, target)')
  })

  it('shows the routine account operations directly without a separate cookie column', () => {
    expect(source).toContain('<t-dropdown')
    expect(source).not.toContain('{ colKey: "cookie", title: "Cookie"')
    expect(source).toContain('openCookieDialog(account)')
    expect(source).toContain('@click="openEdit(row)"')
  })

  it('uses the browser-window resource table structure', () => {
    expect(source).toContain('class="filter-bar"')
    expect(source).toContain('class="table-scroll-wrap"')
    expect(source).toContain(":scroll=\"{ x: 'max-content' }\"")
    expect(source).toContain('class="pagination-bar"')
  })

  it('keeps account filter labels and a compact operation column', () => {
    expect(source).toContain('>搜索</span>')
    expect(source).toContain('>登录状态</span>')
    expect(source).toContain('width: opColumnWidth(')
  })

  // 2026-10-27：列表要能看到账号归谁用，高级运营/管理员按使用人收窄，普通运营不显示该筛选。
  it('shows the account owner and narrows the list by owner for seniors and admins', () => {
    expect(source).toContain('{ colKey: "user_name", title: "使用人", width: 100 }')
    expect(source).toContain("row.user_name || '--'")
    expect(source).toContain('const canFilterByOwner = computed(')
    expect(source).toContain('function defaultOwnerFilter()')
    expect(source).toContain('>使用人</span>')
    expect(source).toContain(':total="pagination.total"')
    expect(source).toContain('accountClient.listPage(params)')
  })

  it('keeps media-account category selections within one code-owned type', () => {
    expect(source).toContain('同一个社媒账号的多个类目必须属于同一一级类型')
    expect(source).toContain('<CategoryFields v-model="createForm.categoryIds"')
    expect(source).toContain('<CategoryFields v-model="editCategoryIds"')
  })

  // 2026-10-27：本地操作按账号归属：高级运营只能动本人账号，管理员一律只读。
  it('gates local account work by ownership on desktop', () => {
    expect(source).toContain('function canOperateLocalAccount(account)')
    expect(source).toContain('return isOwnAccount(account)')
    expect(source).toContain('管理员不执行本地操作')
    expect(source).toContain('仅账号本人可执行本地操作')
    expect(source).toContain('(isDesktop && !canOperateLocalAccount(row))')
    expect(source).toContain('已跳过 ${nonOwnedCount} 条非本人账号')
  })
})

// Execute the actual page command with injected boundaries; cover asynchronous
// confirmation/persistence and repeated table events with a stale row object.
function enabledAction({ confirm = async () => true, update = async (_, input) => input } = {}) {
  const accounts = { value: [{ id: '1', enabled: true, platform_nickname: '测试' }] }
  const enabledPending = { value: new Set() }, enabledConfirming = { value: new Set() }
  const feedback = { confirmWarning: vi.fn(confirm), toast: { success: vi.fn(), error: vi.fn() } }
  const accountClient = { update: vi.fn(update) }
  const body = source.slice(source.indexOf('async function toggleEnabled('), source.indexOf('\nfunction rowMoreOptions('))
  const run = new Function('accounts', 'enabledPending', 'enabledConfirming', 'feedback', 'accountClient', 'accountDisplayName', `${body}; return toggleEnabled`)(accounts, enabledPending, enabledConfirming, feedback, accountClient, a => a.platform_nickname)
  return { run, accounts, enabledPending, enabledConfirming, feedback, accountClient }
}

describe('account enabled command', () => {
  it('uses the switch target and current account after a previous save, including stale table row events', async () => {
    const ctx = enabledAction(), oldRow = ctx.accounts.value[0]
    await ctx.run(oldRow, false)
    expect(ctx.accounts.value[0].enabled).toBe(false)
    await ctx.run(oldRow, true)
    expect(ctx.feedback.confirmWarning.mock.lastCall[0].title).toBe('启用账号')
    expect(ctx.accountClient.update.mock.lastCall).toEqual(['1', { enabled: true }])
    expect(ctx.accounts.value[0].enabled).toBe(true)
  })
  it('does not change the row or submit when confirmation is cancelled', async () => {
    const ctx = enabledAction({ confirm: async () => false })
    await ctx.run(ctx.accounts.value[0], false)
    expect(ctx.accounts.value[0].enabled).toBe(true)
    expect(ctx.accountClient.update).not.toHaveBeenCalled()
    expect(ctx.enabledConfirming.value.size).toBe(0)
  })
  it('preserves the original value on failure, reports an error and permits retry', async () => {
    const ctx = enabledAction({ update: async () => { throw new Error('保存失败') } })
    await ctx.run(ctx.accounts.value[0], false)
    expect(ctx.accounts.value[0].enabled).toBe(true)
    expect(ctx.feedback.toast.error).toHaveBeenCalledWith('保存失败')
    expect(ctx.enabledPending.value.size).toBe(0)
    expect(ctx.enabledConfirming.value.size).toBe(0)
    await ctx.run(ctx.accounts.value[0], false)
    expect(ctx.accountClient.update).toHaveBeenCalledTimes(2)
  })
  it('blocks repeated clicks while confirming and while saving', async () => {
    let confirm, save
    const ctx = enabledAction({ confirm: () => new Promise(resolve => { confirm = resolve }), update: () => new Promise(resolve => { save = resolve }) })
    const row = ctx.accounts.value[0], first = ctx.run(row, false)
    await ctx.run(row, false)
    expect(ctx.feedback.confirmWarning).toHaveBeenCalledTimes(1)
    confirm(true); await Promise.resolve(); await Promise.resolve()
    await ctx.run(row, false)
    expect(ctx.accountClient.update).toHaveBeenCalledTimes(1)
    save({ enabled: false }); await first
    expect(ctx.accounts.value[0].enabled).toBe(false)
    expect(ctx.enabledPending.value.size).toBe(0)
  })
})
