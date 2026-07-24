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
  const componentsReady = !!cloudUser && localAgentReady(status) && bitbrowserReady(bitbrowserStatus) && !!snapshot?.mainUserId;
  const bound = componentsReady && !!snapshot?.nodeId;
  const reason = !cloudUser
    ? "请先登录运营平台"
    : !localAgentReady(status)
      ? "本机执行服务未运行，请先启动后再检测。"
      : !bitbrowserReady(bitbrowserStatus)
        ? "未检测到可用的比特浏览器，请先启动比特浏览器后再检测。"
        : !snapshot?.mainUserId
          ? "未读取到比特浏览器账号，请确认比特浏览器已登录后重新检测。"
          : !snapshot?.nodeId
            ? "当前电脑尚未完成可信绑定，暂不能执行本机浏览器相关操作。"
            : "当前电脑已完成可信绑定，可以扫描窗口、配置代理和检查账号。";

  return {
    primaryStatus: STATUS_LABELS[status] ?? STATUS_LABELS.unknown,
    localStatusKey: status,
    nodeId: snapshot?.nodeId ?? "",
    agentId: snapshot?.agentId ?? "Not registered",
    cloudStatusText: cloudUser ? `已登录：${cloudUser.username}` : "未登录",
    bitbrowserStatusText: BITBROWSER_LABELS[bitbrowserStatus] ?? BITBROWSER_LABELS.unknown,
    mainUserText: snapshot?.mainUserId ? "已读取" : "未读取",
    nodeText: snapshot?.nodeId ? "已完成可信绑定" : "尚未完成可信绑定",
    runtimeText: [snapshot?.operatingSystem, snapshot?.cpuArchitecture, snapshot?.agentVersion].filter(Boolean).join(" / ") || "未上报",
    trustStatus: bound ? "normal" : "warning",
    trustText: bound ? "可执行本机浏览器操作" : (componentsReady ? "尚未完成可信绑定" : "本机环境不可用"),
    trustReason: reason,
    canRefresh: true,
    canBindTrustedNode: !!context.canBindTrustedNode && componentsReady,
    bindActionText: bound ? "确认并刷新本机可信环境" : "绑定当前比特浏览器账号",
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
