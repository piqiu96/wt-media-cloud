// Real Tauri invoke-based Local Agent service (M1-R5).
// Replaces the M0 mock service. Tokens and credentials are never held in Vue.

export const LOCAL_AGENT_COMMANDS = Object.freeze({
  status: "local_agent_status",
  health: "local_agent_health",
  start: "local_agent_start",
  stop: "local_agent_stop",
  taskStatus: "local_agent_task_status",
  bind: "local_agent_bind",
  bindSession: "local_agent_bind_session",
  refreshRuntime: "local_agent_refresh_runtime",
  accountCheck: "local_agent_account_check",
  cookieRead: "local_agent_cookie_read",
  profileScan: "local_agent_profile_scan",
  profileGroups: "local_agent_profile_groups",
  profileOpen: "local_agent_profile_open",
  profileClose: "local_agent_profile_close",
  profileCreate: "local_agent_profile_create",
  profileRestore: "local_agent_profile_restore",
});

const DEFAULT_STATUS = Object.freeze({
  node_id: "",
  agent_id: "local-agent-dev",
  status: "stopped",
  bitbrowser_status: "unknown",
  main_user_id: "",
  operating_system: "",
  cpu_architecture: "",
  agent_version: "",
  current_task_id: null,
  current_task_progress: null,
  current_task_status: null,
  pending_result_count: 0,
});

function clone(value) {
  return JSON.parse(JSON.stringify(value));
}

export function normalizeLocalAgentStatus(value) {
  const source = value && typeof value === "object" ? value : {};
  return {
    node_id: String(source.node_id ?? DEFAULT_STATUS.node_id),
    agent_id: String(source.agent_id ?? DEFAULT_STATUS.agent_id),
    status: String(source.status ?? DEFAULT_STATUS.status),
    bitbrowser_status: String(source.bitbrowser_status ?? DEFAULT_STATUS.bitbrowser_status),
    main_user_id: String(source.main_user_id ?? DEFAULT_STATUS.main_user_id),
    operating_system: String(source.operating_system ?? DEFAULT_STATUS.operating_system),
    cpu_architecture: String(source.cpu_architecture ?? DEFAULT_STATUS.cpu_architecture),
    agent_version: String(source.agent_version ?? DEFAULT_STATUS.agent_version),
    current_task_id: source.current_task_id ?? null,
    current_task_progress: source.current_task_progress ?? null,
    current_task_status: source.current_task_status ?? null,
    pending_result_count: Number(source.pending_result_count ?? 0),
  };
}

export function normalizeBoundNode(value) {
  const source = value && typeof value === "object" ? value : {};
  return {
    id: String(source.node_id ?? ""),
    agent_id: String(source.agent_id ?? ""),
    user_id: String(source.user_id ?? ""),
    status: String(source.status ?? "unknown"),
  };
}

// Real Tauri invoke-based service.
// Falls back to mock when Tauri is unavailable (Vite dev mode).
export function createLocalAgentService({ invoke }) {
  if (typeof invoke !== "function") {
    return createMockLocalAgentService();
  }

  return {
    async status() {
      return normalizeLocalAgentStatus(await invoke(LOCAL_AGENT_COMMANDS.status));
    },
    async health() {
      return invoke(LOCAL_AGENT_COMMANDS.health);
    },
    async start() {
      return invoke(LOCAL_AGENT_COMMANDS.start);
    },
    async stop() {
      return invoke(LOCAL_AGENT_COMMANDS.stop);
    },
    async taskStatus(taskId) {
      return normalizeLocalAgentStatus(
        await invoke(LOCAL_AGENT_COMMANDS.taskStatus, { taskId })
      );
    },
    async bind() {
      const result = await invoke(LOCAL_AGENT_COMMANDS.bind);
      return normalizeBoundNode(result);
    },
    async bindSession({ bindingTicket, cloudBaseUrl }) {
      const result = await invoke(LOCAL_AGENT_COMMANDS.bindSession, {
        args: {
          binding_ticket: bindingTicket,
          cloud_base_url: cloudBaseUrl,
        },
      });
      return normalizeBoundNode(result);
    },
    async refreshRuntime({ cloudBaseUrl }) {
      return normalizeLocalAgentStatus(await invoke(LOCAL_AGENT_COMMANDS.refreshRuntime, {
        args: {
          cloud_base_url: cloudBaseUrl,
        },
      }));
    },
    async accountCheck({ cloudBaseUrl, taskId, bitProfileId, platform, expectedPlatformAccountId = "" }) {
      return invoke(LOCAL_AGENT_COMMANDS.accountCheck, {
        args: {
          cloud_base_url: cloudBaseUrl,
          task_id: taskId,
          bit_profile_id: bitProfileId,
          platform,
          expected_platform_account_id: expectedPlatformAccountId,
        },
      });
    },
    async cookieRead({ cloudBaseUrl, taskId, bitProfileId }) {
      return invoke(LOCAL_AGENT_COMMANDS.cookieRead, {
        args: {
          cloud_base_url: cloudBaseUrl,
          task_id: taskId,
          bit_profile_id: bitProfileId,
        },
      });
    },
    async profileScan() {
      return invoke(LOCAL_AGENT_COMMANDS.profileScan);
    },
    async profileGroups() {
      return invoke(LOCAL_AGENT_COMMANDS.profileGroups);
    },
    async profileOpen(bitProfileId) {
      return invoke(LOCAL_AGENT_COMMANDS.profileOpen, { args: { bit_profile_id: bitProfileId } });
    },
    async profileClose(bitProfileId) {
      return invoke(LOCAL_AGENT_COMMANDS.profileClose, { args: { bit_profile_id: bitProfileId } });
    },
    async profileCreate(profile) {
      return invoke(LOCAL_AGENT_COMMANDS.profileCreate, {
        args: {
          name: profile.name,
          group_id: profile.group_id,
          group_name: profile.group_name || "",
          remark: profile.remark || "",
        },
      });
    },
    async profileRestore(profiles) {
      return invoke(LOCAL_AGENT_COMMANDS.profileRestore, { profiles });
    },
  };
}

// Mock service for standalone Vite dev when Tauri is not available.
export function createMockLocalAgentService(initialStatus = DEFAULT_STATUS) {
  let current = normalizeLocalAgentStatus(initialStatus);

  return {
    async status() {
      return clone(current);
    },
    async health() {
      return "ok";
    },
    async start() {
      current = normalizeLocalAgentStatus({ ...current, status: "running" });
      return clone(current);
    },
    async stop() {
      current = normalizeLocalAgentStatus({
        ...current,
        status: "stopped",
        current_task_id: null,
      });
      return clone(current);
    },
    async taskStatus(_taskId) {
      return clone({ ...current, current_task_id: _taskId });
    },
    async bind() {
      return { id: "node-mock", agent_id: current.agent_id, user_id: "user-mock", status: "bound" };
    },
    async bindSession() {
      current = normalizeLocalAgentStatus({ ...current, node_id: "node-mock" });
      return { id: "node-mock", agent_id: current.agent_id, user_id: "user-mock", status: "online" };
    },
    async refreshRuntime() {
      return clone(current);
    },
    async accountCheck() {
      return {
        platform_account_id: "mock-uid",
        name: "",
        avatar_url: "",
        login_status: "normal",
        message: "mock account check",
      };
    },
    async profileScan() {
      return { main_user_id: current.main_user_id || "main-user-mock", profiles: [] };
    },
    async profileGroups() {
      return { data: { groups: [{ id: "group-mock", name: "模拟分组" }] } };
    },
    async profileOpen(bitProfileId) {
      return { bit_profile_id: bitProfileId, status: "opened", snapshot: { main_user_id: current.main_user_id || "main-user-mock", profiles: [] } };
    },
    async profileClose(bitProfileId) {
      return { bit_profile_id: bitProfileId, status: "closed", snapshot: { main_user_id: current.main_user_id || "main-user-mock", profiles: [] } };
    },
    async profileCreate(profile) {
      return {
        bit_profile_id: "profile-created-mock",
        snapshot: {
          main_user_id: current.main_user_id || "main-user-mock",
          profiles: [{
            bit_profile_id: "profile-created-mock",
            main_user_id: current.main_user_id || "main-user-mock",
            profile_user_id: "bit-user-mock",
            name: profile.name,
            group_id: profile.group_id,
            group_name: profile.group_name || "模拟分组",
          }],
        },
      };
    },
    async profileRestore(profiles) {
      return {
        restored_count: profiles.length,
        profiles: profiles.map((profile) => ({
          bit_profile_id: profile.bit_profile_id,
          status: "verified",
        })),
        snapshot: {
          main_user_id: current.main_user_id || "main-user-mock",
          profiles,
        },
      };
    },
  };
}
