// Desktop 本地 Agent 服务初始化
// 分离自 main.ts，避免模块间循环引用

import { createLocalAgentService } from "./service.js"
import { getSessionToken } from "../../../../shared/api/http.js"
// Static import: a dynamic import() of a node_modules chunk does not resolve in
// the packaged Tauri WebView ("Module name ... does not resolve to a valid file").
import { invoke } from "@tauri-apps/api/core"

function isTauri() {
  return typeof window !== "undefined" && window.__TAURI_INTERNALS__ !== undefined
}

export async function startDesktopLocalAgent({ tauri = isTauri(), invokeImpl = invoke } = {}) {
  if (!tauri) return false
  await createLocalAgentService({ invoke: invokeImpl }).start()
  // Agent 重启后内存里没有执行凭证，领不到任务。登录态还在就补一次；
  // 没登录会在里面报「请先登录」，不该影响启动结果。
  void ensureAgentToken({ tauri, invokeImpl }).catch((error) => {
    console.info('本机执行凭证将在登录或下一次执行前获取', error?.message || error)
  })
  return true
}

// 高级运营和管理员登录 Desktop 时，本机 Agent 不能继续以任何旧会话工作。
export async function stopDesktopLocalAgent({ tauri = isTauri(), invokeImpl = invoke } = {}) {
  if (!tauri) return false
  await createLocalAgentService({ invoke: invokeImpl }).stop()
  return true
}

// The command that answers "where is Cloud". One definition for the WebView
// side; the Rust side spells it in `commands/public_config.rs`.
export const PUBLIC_CONFIG_COMMAND = "get_public_config"

// The last answer `get_public_config` managed to give.
//
// At module scope because the answer outlives one call: the token exchange and
// every later press happen at different moments, and the second press must not
// lose the address the first one learned.
//
// It starts empty, and stays empty if the native side never answers. Empty is
// the honest failure value — the callers already report "Cloud地址为空" — and it
// is never a loopback literal. That literal was the old behaviour: it is wrong
// in every packaged build (their origin is `tauri.localhost`, not the Cloud API
// host), and the whole point of asking the native side is that only it knows.
let lastKnownCloudBaseUrl = ""

export async function cloudBaseUrl({ invokeImpl = invoke } = {}) {
  try {
    const config = await invokeImpl(PUBLIC_CONFIG_COMMAND)
    // A successful answer replaces the cache even when it is empty: the native
    // side is authoritative, and "Cloud is not configured" is an answer rather
    // than a failure to get one.
    lastKnownCloudBaseUrl =
      typeof config?.cloud_base_url === "string" ? config.cloud_base_url.trim() : ""
  } catch {
    // Keep the last known good address. Nothing is reported here on purpose:
    // an empty address already surfaces to the user as the existing
    // "Cloud地址为空" error, and a second message would say the same thing twice.
  }
  return lastKnownCloudBaseUrl
}

/**
 * 用当前 Desktop 会话换一张 Agent Token，交给本机 Agent。
 *
 * 只在登录成功、应用启动、Agent 重启时调用；本进程已经持有就不再换。
 * `force` 用于登录：新的 Desktop 会话对应新的 Token，旧 Token 的会话已被替换。
 * 返回的是事实（有没有、何时过期），Token 本身不进页面。
 */
let pendingToken = null
export function ensureAgentToken({ force = false, tauri = isTauri(), invokeImpl = invoke } = {}) {
  if (!tauri) return Promise.resolve(null)
  if (force) return configureAgentToken({ invokeImpl })
  if (!pendingToken) {
    pendingToken = configureAgentToken({ invokeImpl }).finally(() => { pendingToken = null })
  }
  return pendingToken
}

async function configureAgentToken({ invokeImpl }) {
  const service = createLocalAgentService({ invoke: invokeImpl })
  const held = await service.tokenFacts().catch(() => null)
  if (held?.configured) return held
  const sessionToken = getSessionToken()
  if (!sessionToken) {
    throw new Error("请先登录运营平台，才能获取本机执行凭证")
  }
  const address = await cloudBaseUrl({ invokeImpl })
  if (!address) {
    throw new Error("Cloud地址为空，无法获取本机执行凭证")
  }
  return service.configureToken({ cloudBaseUrl: address, sessionToken })
}

/**
 * 让本机 Agent 实读 BitBrowser 主账号并请 Cloud 绑定。
 *
 * `rebind` 只在用户明确确认「换用当前账号」时为真；不一致会被 Cloud 以
 * `BIT_MAIN_USER_MISMATCH` 拒绝，不自动重绑。失败时 reject 的是 Rust 的
 * `GuardRefusal`，页面按 `code` 分支。
 */
export async function bindBitBrowser({ rebind = false, tauri = isTauri(), invokeImpl = invoke } = {}) {
  if (!tauri) throw new Error("请在Desktop应用内绑定比特浏览器账号")
  // 绑定走严格档，sidecar 手里得先有凭证；没有时会给出可读的登录提示。
  await ensureAgentToken({ invokeImpl })
  return createLocalAgentService({ invoke: invokeImpl }).bitAccountBind({ rebind })
}
