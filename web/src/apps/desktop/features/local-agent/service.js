// Real Tauri invoke-based Local Agent service (M1-R5).
// Replaces the M0 mock service. Tokens and credentials are never held in Vue.

export const LOCAL_AGENT_COMMANDS = Object.freeze({
  status: "local_agent_status",
  health: "local_agent_health",
  start: "local_agent_start",
  stop: "local_agent_stop",
  configureToken: "local_agent_configure_token",
  tokenFacts: "local_agent_token_facts",
  bitAccountBind: "local_agent_bit_account_bind",
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
  agent_id: "local-agent-dev",
  status: "stopped",
  bitbrowser_status: "unknown",
  main_user_id_masked: "",
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
    agent_id: String(source.agent_id ?? DEFAULT_STATUS.agent_id),
    status: String(source.status ?? DEFAULT_STATUS.status),
    bitbrowser_status: String(source.bitbrowser_status ?? DEFAULT_STATUS.bitbrowser_status),
    main_user_id_masked: String(source.main_user_id_masked ?? DEFAULT_STATUS.main_user_id_masked),
    operating_system: String(source.operating_system ?? DEFAULT_STATUS.operating_system),
    cpu_architecture: String(source.cpu_architecture ?? DEFAULT_STATUS.cpu_architecture),
    agent_version: String(source.agent_version ?? DEFAULT_STATUS.agent_version),
    current_task_id: source.current_task_id ?? null,
    current_task_progress: source.current_task_progress ?? null,
    current_task_status: source.current_task_status ?? null,
    pending_result_count: Number(source.pending_result_count ?? 0),
  };
}

// The camelCase view of a status snapshot. `normalizeLocalAgentStatus` keeps the
// Agent's wire spelling (snake_case) — init.js and PersonalInfoPage read that —
// while the status page and the work-env aggregate read camelCase. This is the
// one place that spells the mapping; the store and the aggregate both go
// through it, so a renamed field breaks here once, not per consumer.
export function localAgentStateFromStatus(value) {
  const source = value && typeof value === "object" ? value : {};
  return {
    agentId: String(source.agent_id ?? DEFAULT_STATUS.agent_id),
    status: String(source.status ?? DEFAULT_STATUS.status),
    bitbrowserStatus: String(source.bitbrowser_status ?? DEFAULT_STATUS.bitbrowser_status),
    mainUserIdMasked: String(source.main_user_id_masked ?? DEFAULT_STATUS.main_user_id_masked),
    operatingSystem: String(source.operating_system ?? DEFAULT_STATUS.operating_system),
    cpuArchitecture: String(source.cpu_architecture ?? DEFAULT_STATUS.cpu_architecture),
    agentVersion: String(source.agent_version ?? DEFAULT_STATUS.agent_version),
    currentTaskId: source.current_task_id ?? null,
    currentTaskProgress: source.current_task_progress ?? null,
    currentTaskStatus: source.current_task_status ?? null,
    pendingResultCount: Number(source.pending_result_count ?? 0),
  };
}

// Real Tauri invoke-based service.
// Falls back to mock when Tauri is unavailable (Vite dev mode).
export function createLocalAgentService({ invoke }) {
  if (typeof invoke !== "function") {
    return createMockLocalAgentService();
  }

  return {
    // `reuseScan` asks the Agent to reuse a recent BitBrowser scan instead of
    // paying for a fresh one. Only background polling sets it: the account id in
    // the answer is what the bind flow and the runtime report act on, so a
    // caller that shows one has to get a live scan.
    async status({ reuseScan = false } = {}) {
      return normalizeLocalAgentStatus(
        await invoke(LOCAL_AGENT_COMMANDS.status, { reuseScan })
      );
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
    // 用当前 Desktop 会话向 Cloud 换一张 Agent Token，并交给本机 Agent。
    // 只回事实（有没有、何时过期），Token 本身不进页面。
    async configureToken({ cloudBaseUrl, sessionToken }) {
      return invoke(LOCAL_AGENT_COMMANDS.configureToken, {
        args: {
          cloud_base_url: cloudBaseUrl,
          session_token: sessionToken,
        },
      });
    },
    async tokenFacts() {
      return invoke(LOCAL_AGENT_COMMANDS.tokenFacts);
    },
    // 让 Agent 实读 BitBrowser 主账号并请 Cloud 绑定；`rebind` 只在用户明确
    // 确认换绑时为真。失败时 reject 的是 Rust 的 `GuardRefusal`，按 `code` 分支。
    async bitAccountBind({ rebind = false } = {}) {
      return invoke(LOCAL_AGENT_COMMANDS.bitAccountBind, { rebind });
    },
    async accountCheck({ taskId, bitProfileId, platform, expectedPlatformAccountId = "" }) {
      return invoke(LOCAL_AGENT_COMMANDS.accountCheck, {
        args: {
          task_id: taskId,
          bit_profile_id: bitProfileId,
          platform,
          expected_platform_account_id: expectedPlatformAccountId,
        },
      });
    },
    async cookieRead({ taskId, bitProfileId }) {
      return invoke(LOCAL_AGENT_COMMANDS.cookieRead, {
        args: {
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
    async status({ reuseScan = false } = {}) {
      void reuseScan;
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
    async configureToken() {
      return { configured: true, expires_at: null };
    },
    async tokenFacts() {
      return { configured: true, expires_at: null };
    },
    async bitAccountBind() {
      return { bound: true };
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
      return { main_user_id: "main-user-mock", observed_at: new Date().toISOString(), profiles: [] };
    },
    async profileGroups() {
      return { data: { groups: [{ id: "group-mock", name: "模拟分组" }] } };
    },
    async profileOpen(bitProfileId) {
      return { bit_profile_id: bitProfileId, status: "opened", snapshot: { main_user_id: "main-user-mock", profiles: [] } };
    },
    async profileClose(bitProfileId) {
      return { bit_profile_id: bitProfileId, status: "closed", snapshot: { main_user_id: "main-user-mock", profiles: [] } };
    },
    async profileCreate(profile) {
      return {
        bit_profile_id: "profile-created-mock",
        snapshot: {
          main_user_id: "main-user-mock",
          profiles: [{
            bit_profile_id: "profile-created-mock",
            main_user_id: "main-user-mock",
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
          main_user_id: "main-user-mock",
          profiles,
        },
      };
    },
  };
}
