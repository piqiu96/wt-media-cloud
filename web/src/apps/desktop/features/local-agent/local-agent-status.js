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
  unknown: "未知",
});

function localAgentReady(status) {
  return ["idle", "running"].includes(status);
}

function bitbrowserReady(status) {
  return status === "normal";
}

export function createLocalAgentStatusPage(snapshot, context = {}) {
  const status = snapshot?.status ?? "unknown";
  const bitbrowserStatus = snapshot?.bitbrowserStatus ?? "unknown";
  const cloudUser = context.cloudUser ?? null;
  const executable = !!cloudUser && localAgentReady(status) && bitbrowserReady(bitbrowserStatus) && !!snapshot?.mainUserId;
  const reason = !cloudUser
    ? "请先登录Cloud账号"
    : !localAgentReady(status)
      ? "Local Agent未就绪"
      : !bitbrowserReady(bitbrowserStatus)
        ? "BitBrowser不可用或身份不可验证"
        : !snapshot?.mainUserId
          ? "未读取到BitBrowser主账号"
          : "当前电脑可以执行M2本地敏感操作";

  return {
    primaryStatus: STATUS_LABELS[status] ?? STATUS_LABELS.unknown,
    localStatusKey: status,
    nodeId: snapshot?.nodeId ?? "",
    agentId: snapshot?.agentId ?? "Not registered",
    cloudStatusText: cloudUser ? `已登录：${cloudUser.username}` : "未登录",
    bitbrowserStatusText: BITBROWSER_LABELS[bitbrowserStatus] ?? BITBROWSER_LABELS.unknown,
    mainUserText: snapshot?.mainUserId || "未读取",
    runtimeText: [snapshot?.operatingSystem, snapshot?.cpuArchitecture, snapshot?.agentVersion].filter(Boolean).join(" / ") || "未上报",
    trustStatus: executable ? "normal" : "warning",
    trustText: executable ? "本机环境可信" : "本机环境不可执行",
    trustReason: reason,
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
