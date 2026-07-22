import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const desktopMain = readFileSync(new URL('./main.ts', import.meta.url), 'utf8')
const loginPage = readFileSync(new URL('../../modules/auth/pages/LoginPage.vue', import.meta.url), 'utf8')
const utils = readFileSync(new URL('../../utils.js', import.meta.url), 'utf8')

describe('desktop role guard', () => {
  it('only allows operator role to use Desktop business pages', () => {
    expect(utils).toContain("user?.role === 'operator'")
    expect(desktopMain).toContain('canUseDesktop')
    expect(desktopMain).toContain('desktop_role_forbidden')
    expect(loginPage).toContain('管理员和高级运营不能登录 Desktop')
  })
})
