import { describe, expect, it } from 'vitest'

import {
  SYSTEM_MANAGEMENT_PATHS,
  canUseDesktopEnv,
  canViewSystemPath,
  isAdminRole,
  isSystemManagementPath,
} from './roles.js'

describe('system management visibility', () => {
  // 用户 2026-10-27：用户管理仅管理员；运营团队 / 类目管理 高级运营 + 管理员。
  it('gives users to admins only and teams/categories to seniors as well', () => {
    expect(canViewSystemPath('admin', '/users')).toBe(true)
    expect(canViewSystemPath('senior_operator', '/users')).toBe(false)
    expect(canViewSystemPath('operator', '/users')).toBe(false)

    for (const path of ['/operation-teams', '/categories']) {
      expect(canViewSystemPath('admin', path), path).toBe(true)
      expect(canViewSystemPath('senior_operator', path), path).toBe(true)
      expect(canViewSystemPath('operator', path), path).toBe(false)
    }
  })

  it('knows exactly the three management paths', () => {
    expect(SYSTEM_MANAGEMENT_PATHS).toEqual(['/users', '/operation-teams', '/categories'])
    expect(isSystemManagementPath('/users')).toBe(true)
    expect(isSystemManagementPath('/content-pool')).toBe(false)
    // 未登录（角色为空）一律不可见，而不是默认放行。
    expect(canViewSystemPath(undefined, '/users')).toBe(false)
  })

  it('keeps writes with the admin even where seniors can read', () => {
    expect(isAdminRole({ role: 'admin' })).toBe(true)
    expect(isAdminRole({ role: 'senior_operator' })).toBe(false)
    expect(isAdminRole(null)).toBe(false)
  })
})

describe('desktop environment visibility', () => {
  // 用户 2026-10-27：桌面环境 运营 + 高级运营；管理员不可见。
  it('covers operators and seniors but not admins', () => {
    expect(canUseDesktopEnv({ role: 'operator' })).toBe(true)
    expect(canUseDesktopEnv({ role: 'senior_operator' })).toBe(true)
    expect(canUseDesktopEnv({ role: 'admin' })).toBe(false)
    expect(canUseDesktopEnv(null)).toBe(false)
  })
})
