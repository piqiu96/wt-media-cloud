import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

import { canRunLocalAgent, canUseDesktop, canUseLocalAgent } from '../../utils.js'
import { desktopRoutes } from './router.ts'

const desktopMain = readFileSync(new URL('./main.ts', import.meta.url), 'utf8')
const loginPage = readFileSync(new URL('../../modules/auth/pages/LoginPage.vue', import.meta.url), 'utf8')
const layout = readFileSync(new URL('../../layout/AppLayout.vue', import.meta.url), 'utf8')
const personalInfo = readFileSync(new URL('../../modules/auth/pages/PersonalInfoPage.vue', import.meta.url), 'utf8')
const routerSource = readFileSync(new URL('./router.ts', import.meta.url), 'utf8')

// 2026-10-28（V3 D08）：浏览器窗口对运营与高级运营开放本机能力；查看其他运营的
// 窗口只读。开户仍是普通运营专属，代理管理不再受本机页面可见性限制。
const LOCAL_ONLY_PATHS = ['browser-windows']

describe('desktop role guard', () => {
  it('lets all three roles into Desktop; windows follow the operator/senior local capability', () => {
    expect(canUseDesktop({ role: 'operator' })).toBe(true)
    expect(canUseDesktop({ role: 'senior_operator' })).toBe(true)
    expect(canUseDesktop({ role: 'admin' })).toBe(true)
    expect(canUseDesktop({ role: 'unknown' })).toBe(false)
    expect(canUseDesktop(null)).toBe(false)

    expect(canUseLocalAgent({ role: 'operator' })).toBe(true)
    expect(canUseLocalAgent({ role: 'senior_operator' })).toBe(true)
    expect(canUseLocalAgent({ role: 'admin' })).toBe(false)
    expect(canUseLocalAgent(null)).toBe(false)

    // 2026-10-27：高级运营要在本机跑自己账号的检查与 Cookie 读取，sidecar 得起来；
    // 管理员仍不参与本机操作。
    expect(canRunLocalAgent({ role: 'operator' })).toBe(true)
    expect(canRunLocalAgent({ role: 'senior_operator' })).toBe(true)
    expect(canRunLocalAgent({ role: 'admin' })).toBe(false)
    expect(canRunLocalAgent(null)).toBe(false)
  })

  it('marks the local window route and redirects roles without the capability away from it', () => {
    const children = desktopRoutes.flatMap((route) => route.children ?? [])
    for (const path of LOCAL_ONLY_PATHS) {
      const route = children.find((child) => child.path === path)
      expect(route, `missing desktop route ${path}`).toBeDefined()
      expect(route.meta?.localOnly, `${path} must be localOnly`).toBe(true)
    }
    // 分母：本地专属路由只有窗口这一条，代理与账号页面不再标 localOnly。
    expect(children.filter((child) => child.meta?.localOnly)).toHaveLength(1)
    expect(children.find((child) => child.path === 'proxies')?.meta?.localOnly).toBeUndefined()

    expect(desktopMain).toContain('to.meta?.localOnly')
    expect(desktopMain).toContain('next("/content-pool")')
    expect(desktopMain).toContain('canUseLocalAgent(user)')
    expect(routerSource).toContain('localOnly: true')
  })

  it('starts the local agent for operators and seniors and stops it for admins', () => {
    expect(desktopMain).toContain('startDesktopLocalAgent')
    expect(desktopMain).toContain('stopDesktopLocalAgent')
    expect(desktopMain).toContain('syncLocalAgentFor')
    expect(desktopMain).toContain('if (canRunLocalAgent(user))')
    // 旧行为是应用启动就无条件拉起 sidecar，和登录角色无关。
    expect(desktopMain).not.toContain('void startDesktopLocalAgent({ tauri: true }).catch')
    expect(loginPage).toContain('stopDesktopLocalAgent')
    expect(loginPage).toContain('if (!canRunLocalAgent(user))')
    expect(loginPage).not.toContain('管理员和高级运营不能登录 Desktop')
  })

  it('keeps local-resource navigation operator-only while the agent pill follows the sidecar roles', () => {
    expect(layout).toContain('const localOnlyPaths = new Set')
    expect(layout).toContain('const localAgentAllowed = computed(() => canUseLocalAgent(currentUser.value))')
    expect(layout).toContain('if (isDesktop() && !localAgentAllowed.value)')
    expect(layout).toContain('const localAgentRunnable = computed(() => canRunLocalAgent(currentUser.value))')
    expect(layout).toContain('v-if="isDesktop() && localAgentRunnable"')
  })

  // 2026-10-27：桌面环境（本地日志 / 本机设置）给运营与高级运营，管理员不可见；
  // 菜单与路由守卫用的是同一个判据。
  it('gives the desktop environment to operators and seniors, not to admins', () => {
    expect(layout).toContain('const desktopEnvAllowed = computed(() => canUseDesktopEnv(currentUser.value))')
    expect(layout).toContain('desktopEnvAllowed.value ? desktopItems : []')
    expect(desktopMain).toContain('if (!canUseDesktopEnv(user))')
  })

  // 2026-10-27：系统管理进桌面端。用户管理仅管理员；运营团队 / 类目管理 高级运营只读。
  // 两端共用同一张路径表，守卫各写一次解析同一个判据。
  it('routes system management by role on both ends', () => {
    expect(layout).toContain('SYSTEM_MANAGEMENT_PATHS')
    expect(layout).toContain('canViewSystemPath(currentUser.value?.role, path)')
    expect(desktopMain).toContain('isSystemManagementPath(to.path) && !canViewSystemPath(user.role, to.path)')

    const cloudMain = readFileSync(new URL('../cloud/main.ts', import.meta.url), 'utf8')
    expect(cloudMain).toContain('isSystemManagementPath(to.path) && !canViewSystemPath(user.role, to.path)')
  })

  it('lets every role into the social accounts page', () => {
    const children = desktopRoutes.flatMap((route) => route.children ?? [])
    const accounts = children.find((child) => child.path === 'accounts')
    expect(accounts, 'missing desktop route accounts').toBeDefined()
    expect(accounts.meta?.localOnly).toBeUndefined()
  })

  it('does not read or render local agent and BitBrowser binding for admins', () => {
    expect(personalInfo).toContain("if (!canRunLocalAgent(currentUser)) return")
    expect(personalInfo).toContain('v-if="localAgentAllowed" class="device-card"')
  })

  it('waits for the local agent before the window page performs its entry scan', () => {
    expect(desktopMain).toContain('if (to.path === "/browser-windows") await agentReady')
    expect(desktopMain).toContain('if (key === syncedAgentUser) return agentSyncPromise')
  })

  it('uses a native form submit for session replacement login', () => {
    expect(loginPage).toContain('<form class="login-form" @submit.prevent="submitLogin">')
    expect(loginPage).toContain('@confirm="submitReplacementLogin"')
    expect(loginPage).toContain('login-button primary')
    expect(loginPage).not.toContain('<t-form @submit="login">')
    expect(loginPage).not.toContain('@click.stop.prevent="login({ replaceExisting: true })"')
  })
})
