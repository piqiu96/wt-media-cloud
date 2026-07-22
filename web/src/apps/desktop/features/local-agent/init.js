// Desktop 本地 Agent 服务初始化
// 分离自 main.ts，避免模块间循环引用

import { createLocalAgentService, createMockLocalAgentService } from "./service.js"
import { createLocalAgentStore } from "./store.js"
import { createLocalAgentStatusPage } from "./local-agent-status.js"
import { createSessionClient } from "../../../../shared/api/session.js"

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
  }
}

function isTauri() {
  return typeof window !== "undefined" && window.__TAURI_INTERNALS__ !== undefined
}

export async function createDesktopStatusPreview() {
  let service

  if (isTauri()) {
    const { invoke } = await import("@tauri-apps/api/core")
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
  console.info(`wt-media-desktop local agent: ${snapshot.status}`)
  return createLocalAgentStatusPage(snapshot, { cloudUser })
}
