// 把 WebView 内的 JS 报错转回 Rust 侧（`log_js_error` → stdout）。
//
// 为什么在前端注册，而不是像原来那样由 Rust 用 `window.eval` 注入：
// 应用 CSP 的 script-src 回落到 default-src 'self'，且不含 'unsafe-eval'，
// `window.eval` 会被 WebView 直接拒绝 —— 那段注入从未生效过，原生端因此
// 一行报错都看不到。前端入口本身就是 'self' 加载的脚本，不受此限制；
// 这样也无需为了日志给 CSP 开 'unsafe-eval'（那是拿安全边界换日志）。
import { invoke } from "@tauri-apps/api/core"

function isTauri() {
  return typeof window !== "undefined" && window.__TAURI_INTERNALS__ !== undefined
}

export function installWebviewErrorReporting({ tauri = isTauri(), invokeImpl = invoke } = {}) {
  if (!tauri) return false

  // 上报失败必须自己吞掉：否则会再次触发 error 监听，形成递归。
  const report = (message, stack) => {
    try {
      void Promise.resolve(
        invokeImpl("log_js_error", { message: String(message), stack: String(stack || "") })
      ).catch(() => {})
    } catch {
      // 桥不可用时静默：诊断能力缺失不应反过来影响业务
    }
  }

  window.addEventListener("error", (event) => {
    report(
      `${event.message || ""} @ ${event.filename || ""}:${event.lineno || 0}`,
      event.error?.stack
    )
  })

  window.addEventListener("unhandledrejection", (event) => {
    const reason = event.reason || {}
    report(
      `unhandled rejection: ${reason?.message ?? String(reason)}`,
      reason?.stack
    )
  })

  // 与原先 Rust 注入版的覆盖范围保持一致：console.error / console.warn 也转发。
  for (const level of ["error", "warn"]) {
    const original = console[level].bind(console)
    console[level] = (...args) => {
      report(`[console.${level}] ${args.map(String).join(" ")}`, "")
      original(...args)
    }
  }

  return true
}
