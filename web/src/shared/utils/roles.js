// 系统管理的可见角色，Cloud Web 与 Desktop 共用这一张表。用户管理只有管理员；
// 运营团队与游戏管理对高级运营开放只读、对管理员开放读写。页面里的写按钮由
// `isAdminRole` 控制，这里只管「进不进得去」。
const SYSTEM_PATH_ROLES = {
  '/users': ['admin'],
  '/operation-teams': ['admin', 'senior_operator'],
  '/games': ['admin', 'senior_operator'],
}

export const SYSTEM_MANAGEMENT_PATHS = Object.keys(SYSTEM_PATH_ROLES)

export function isAdminRole(user) {
  return user?.role === 'admin'
}

export function isSystemManagementPath(path) {
  return Object.prototype.hasOwnProperty.call(SYSTEM_PATH_ROLES, path)
}

export function canViewSystemPath(role, path) {
  const allowed = SYSTEM_PATH_ROLES[path]
  return Boolean(allowed && allowed.includes(role))
}

// 桌面环境（本地日志 / 本机设置）是本机排障入口，给运营与高级运营；管理员不承担
// 本机操作，所以既没有菜单入口，也不能留在页面上。
export function canUseDesktopEnv(user) {
  return ['operator', 'senior_operator'].includes(user?.role)
}
