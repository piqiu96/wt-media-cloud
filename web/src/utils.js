export function isDesktop() {
  return typeof window !== 'undefined' && window.__TAURI_INTERNALS__ !== undefined
}
