export function isDesktop() {
  return typeof window !== 'undefined' && (
    window.__TAURI_INTERNALS__ !== undefined ||
    window.location?.port === '5174'
  )
}

// 三种角色都能登录 Desktop 并使用 M3–M5 的共用 Cloud 页面。
export function canUseDesktop(user) {
  return ['operator', 'senior_operator', 'admin'].includes(user?.role)
}

// 桌面端本机资源页面：浏览器窗口与代理对运营、高级运营开放（D08）。
// 高级运营对本人窗口有同样本机能力；查看成员时由页面隐藏本机动作。
export function canUseLocalAgent(user) {
  return user?.role === 'operator' || user?.role === 'senior_operator'
}

// 本机 Agent（sidecar）由运营与高级运营拉起；管理员不承担本机操作。
// 高级运营能对哪些账号执行本地动作由页面按账号归属再判一层。
export function canRunLocalAgent(user) {
  return user?.role === 'operator' || user?.role === 'senior_operator'
}
