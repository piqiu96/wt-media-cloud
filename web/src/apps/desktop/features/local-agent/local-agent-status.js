const STATUS_LABELS = Object.freeze({
  idle: "空闲",
  running: "运行中",
  starting: "启动中",
  stopped: "已停止",
  stopping: "停止中",
  error: "异常",
  unknown: "未知",
});

const BITBROWSER_LABELS = Object.freeze({
  normal: "可用",
  unreachable: "不可达",
  identity_unverifiable: "身份不可验证",
  unknown: "未检测",
});

function localAgentReady(status) {
  return ["idle", "running"].includes(status);
}

function bitbrowserReady(status) {
  return status === "normal";
}

/**
 * 本机能否执行浏览器操作。
 *
 * 五件事缺一不可：已登录、Agent 在运行、比特浏览器可用、读到了本机主账号、
 * 本进程持有执行凭证。最后一项由 Tauri 的内存状态回答（`tokenFacts`），页面
 * 看不到凭证本身。
 */
export function createLocalAgentStatusPage(snapshot, context = {}) {
  const status = snapshot?.status ?? "unknown";
  const bitbrowserStatus = snapshot?.bitbrowserStatus ?? "unknown";
  const cloudUser = context.cloudUser ?? null;
  const tokenConfigured = !!context.tokenConfigured;
  const componentsReady = !!cloudUser && localAgentReady(status) && bitbrowserReady(bitbrowserStatus) && !!snapshot?.mainUserIdMasked;
  const ready = componentsReady && tokenConfigured;
  const reason = !cloudUser
    ? "请先登录运营平台"
    : !localAgentReady(status)
      ? "本机服务未启动，请重启应用后再检查。"
      : !bitbrowserReady(bitbrowserStatus)
        ? "未检测到可用的比特浏览器，请先启动比特浏览器后再检测。"
        : !snapshot?.mainUserIdMasked
          ? "未读取到比特浏览器账号，请确认比特浏览器已登录后重新检测。"
          : !tokenConfigured
            ? "本机执行凭证未配置，任务无法领取；请重新登录或重启应用后重试。"
            : "本机环境已就绪，可以扫描窗口、配置代理和检查账号。";

  return {
    primaryStatus: STATUS_LABELS[status] ?? STATUS_LABELS.unknown,
    localStatusKey: status,
    agentId: snapshot?.agentId ?? "Not registered",
    cloudStatusText: cloudUser ? `已登录：${cloudUser.username}` : "未登录",
    bitbrowserStatusText: BITBROWSER_LABELS[bitbrowserStatus] ?? BITBROWSER_LABELS.unknown,
    // 面板行各说各话：本机服务行只看服务自身，比特浏览器行只看比特浏览器自身，
    // 执行凭证行只回答本进程有没有 —— 一行坏了不歪曲另一行的事实。
    serviceText: !localAgentReady(status)
      ? "未运行"
      : `已连接，${snapshot?.currentTaskId ? `当前任务 ${snapshot.currentTaskId}` : "当前无任务"}`,
    bitbrowserText:
      bitbrowserReady(bitbrowserStatus) && snapshot?.mainUserIdMasked ? "已登录指定账号" : BITBROWSER_LABELS[bitbrowserStatus] ?? BITBROWSER_LABELS.unknown,
    credentialText: tokenConfigured ? "已配置" : "未配置",
    runtimeText: [snapshot?.operatingSystem, snapshot?.cpuArchitecture, snapshot?.agentVersion].filter(Boolean).join(" / ") || "未上报",
    ready,
    trustStatus: ready ? "normal" : "warning",
    trustText: ready ? "可执行本机浏览器操作" : (componentsReady ? "本机执行凭证未配置" : "本机环境不可用"),
    trustReason: reason,
    canRefresh: true,
    actions: [
      {
        id: "start",
        label: "Start",
        enabled: !["running", "starting"].includes(status),
      },
      {
        id: "stop",
        label: "Stop",
        enabled: !["stopped", "stopping", "unknown"].includes(status),
      },
      {
        id: "refresh",
        label: "Refresh",
        enabled: true,
      },
    ],
  };
}
