export function isDesktop() {
  return typeof window !== 'undefined' && (
    window.__TAURI_INTERNALS__ !== undefined ||
    window.location?.port === '5174'
  )
}

export function canUseDesktop(user) {
  return user?.role === 'operator'
}
