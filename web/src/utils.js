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

// 本机浏览器操作、Profile/Cookie 与 Agent 执行凭证仍只属于普通运营。
export function canUseLocalAgent(user) {
  return user?.role === 'operator'
}
