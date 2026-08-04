// Desktop 本地 Agent 服务初始化
// 分离自 main.ts，避免模块间循环引用

import { createLocalAgentService, createMockLocalAgentService } from "./service.js"
import { createLocalAgentStore } from "./store.js"
import { createLocalAgentStatusPage } from "./local-agent-status.js"
import { createSessionClient } from "../../../../shared/api/session.js"
import { createProfileBindingClient } from "../../../../shared/api/profileBindings.js"
import { createRuntimeBindingClient } from "../../../../shared/api/runtimeBinding.js"
// Static import: a dynamic import() of a node_modules chunk does not resolve in
// the packaged Tauri WebView ("Module name ... does not resolve to a valid file").
import { invoke } from "@tauri-apps/api/core"

function createHttpLocalAgentService() {
  const BASE = "http://127.0.0.1:8765"

  async function fetchJson(path) {
    try {
      const resp = await fetch(`${BASE}${path}`)
      if (!resp.ok) return null
      return resp.json()
    } catch {
      return null
    }
  }

  const DEFAULT = { node_id: "", agent_id: "local-agent-dev", status: "stopped", bitbrowser_status: "unknown", main_user_id: "", operating_system: "", cpu_architecture: "", agent_version: "", current_task_id: null, current_task_progress: null, current_task_status: null, pending_result_count: 0 }

  return {
    async status() {
      let body = await fetchJson("/api/v1/status")
      const src = body?.data ?? body
      if (src) {
        return {
          node_id: String(src.node_id ?? DEFAULT.node_id),
          agent_id: String(src.agent_id ?? DEFAULT.agent_id),
          status: String(src.status ?? DEFAULT.status),
          bitbrowser_status: String(src.bitbrowser_status ?? DEFAULT.bitbrowser_status),
          main_user_id: String(src.main_user_id ?? DEFAULT.main_user_id),
          operating_system: String(src.operating_system ?? DEFAULT.operating_system),
          cpu_architecture: String(src.cpu_architecture ?? DEFAULT.cpu_architecture),
          agent_version: String(src.agent_version ?? DEFAULT.agent_version),
          current_task_id: src.current_task_id ?? null,
          current_task_progress: src.current_task_progress ?? null,
          current_task_status: src.current_task_status ?? null,
          pending_result_count: Number(src.pending_result_count ?? 0),
        }
      }
      return { ...DEFAULT }
    },
    async health() { const body = await fetchJson("/healthz"); return body?.status ?? "unreachable" },
    async start() { return "start_requested" },
    async stop() { return "stop_requested" },
    async taskStatus(_taskId) { return { ...DEFAULT, current_task_id: _taskId } },
    async bind() { return { id: "node-http", agent_id: "local-agent-dev", user_id: "", status: "bound" } },
    async bindSession() { throw new Error("请在Desktop应用内绑定当前电脑") },
    async refreshRuntime() { throw new Error("请在Desktop应用内刷新本机可信状态") },
    async profileScan() { throw new Error("请在Desktop应用内读取BitBrowser身份") },
  }
}

function isTauri() {
  return typeof window !== "undefined" && window.__TAURI_INTERNALS__ !== undefined
}

function cloudBaseUrl() {
  if (typeof window === "undefined") return "http://127.0.0.1:18080"
  const origin = window.location?.origin || "http://127.0.0.1:18080"
  // Packaged Desktop runs on http://tauri.localhost, which is NOT the Cloud
  // API host; the local Cloud server is always the API base.
  return origin.startsWith("http://127.0.0.1:18080") ? origin : "http://127.0.0.1:18080"
}

async function createDesktopStatusContext() {
  const tauri = isTauri()
  let service

  if (tauri) {
    service = createLocalAgentService({ invoke })
  } else {
    service = createHttpLocalAgentService()
  }

  const store = createLocalAgentStore({ service })
  await store.refresh()
  const snapshot = store.snapshot()
  let cloudUser = null
  try {
    cloudUser = await createSessionClient().me()
  } catch {
    cloudUser = null
  }
  return { service, store, snapshot, cloudUser, tauri }
}

export async function createDesktopStatusPreview() {
  const { snapshot, cloudUser, tauri } = await createDesktopStatusContext()
  console.info(`wt-media-desktop local agent: ${snapshot.status}`)
  return createLocalAgentStatusPage(snapshot, { cloudUser, canBindTrustedNode: tauri })
}

export async function bindTrustedLocalAgent() {
  const { service, cloudUser, tauri } = await createDesktopStatusContext()
  if (!tauri) {
    throw new Error("请在Desktop应用内刷新本机可信状态")
  }
  if (!cloudUser) {
    throw new Error("请先登录运营平台")
  }
  const localSnapshot = await service.profileScan()
  const mainUserId = localSnapshot?.main_user_id || ""
  if (!mainUserId) {
    throw new Error("未读取到比特浏览器账号，请确认比特浏览器已登录后重新检测本机环境。")
  }
  try {
    await createProfileBindingClient().confirmMainIdentityDirect(mainUserId)
  } catch (error) {
    if (error?.errcode === 23002) {
      throw new Error("当前比特浏览器登录账号与系统绑定账号不一致，已阻止本机浏览器相关操作。请切换回已绑定的比特浏览器账号后重新检测；如果系统绑定错误，请联系管理员解除绑定后重新绑定。")
    }
    throw error
  }
  const status = await service.status()
  if (status.node_id) {
    try {
      await service.refreshRuntime({ cloudBaseUrl: cloudBaseUrl() })
      return createDesktopStatusPreview()
    } catch (error) {
      console.warn("刷新本机可信状态失败，将重新确认并绑定当前Desktop执行凭证", error)
    }
  }
  const ticket = await createRuntimeBindingClient().createBindingTicket()
  await service.bindSession({
    bindingTicket: ticket.binding_token,
    cloudBaseUrl: cloudBaseUrl(),
  })
  return createDesktopStatusPreview()
}
