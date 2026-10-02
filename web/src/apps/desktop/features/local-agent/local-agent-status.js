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

export function createLocalAgentStatusPage(snapshot, context = {}) {
  const status = snapshot?.status ?? "unknown";
  const bitbrowserStatus = snapshot?.bitbrowserStatus ?? "unknown";
  const cloudUser = context.cloudUser ?? null;
  const componentsReady = !!cloudUser && localAgentReady(status) && bitbrowserReady(bitbrowserStatus) && !!snapshot?.mainUserId;
  const bound = componentsReady && !!snapshot?.nodeId;
  const reason = !cloudUser
    ? "请先登录运营平台"
    : !localAgentReady(status)
      ? "本机服务未启动，请重启应用后再检查。"
      : !bitbrowserReady(bitbrowserStatus)
        ? "未检测到可用的比特浏览器，请先启动比特浏览器后再检测。"
        : !snapshot?.mainUserId
          ? "未读取到比特浏览器账号，请确认比特浏览器已登录后重新检测。"
          : !snapshot?.nodeId
            ? "本机服务尚未连接云端，请到「个人信息」页检查设备绑定。"
            : "本机执行节点已连接，可以扫描窗口、配置代理和检查账号。";

  return {
    primaryStatus: STATUS_LABELS[status] ?? STATUS_LABELS.unknown,
    localStatusKey: status,
    nodeId: snapshot?.nodeId ?? "",
    agentId: snapshot?.agentId ?? "Not registered",
    cloudStatusText: cloudUser ? `已登录：${cloudUser.username}` : "未登录",
    bitbrowserStatusText: BITBROWSER_LABELS[bitbrowserStatus] ?? BITBROWSER_LABELS.unknown,
    // 面板行各说各话：本机服务行只看服务自身（在运行且已连节点），
    // 比特浏览器行只看比特浏览器自身——一个组件坏了不歪曲另一行的事实。
    serviceText: !localAgentReady(status)
      ? "未运行"
      : !snapshot?.nodeId
        ? "未连接云端"
        : `已连接，${snapshot?.currentTaskId ? `当前任务 ${snapshot.currentTaskId}` : "当前无任务"}`,
    bitbrowserText:
      bitbrowserReady(bitbrowserStatus) && snapshot?.mainUserId ? "已登录指定账号" : BITBROWSER_LABELS[bitbrowserStatus] ?? BITBROWSER_LABELS.unknown,
    mainUserText: snapshot?.mainUserId ? "已读取" : "未读取",
    nodeText: snapshot?.nodeId ? "当前会话已连接" : "当前会话未连接",
    runtimeText: [snapshot?.operatingSystem, snapshot?.cpuArchitecture, snapshot?.agentVersion].filter(Boolean).join(" / ") || "未上报",
    bound: componentsReady && !!snapshot?.nodeId,
    trustStatus: bound ? "normal" : "warning",
    trustText: bound ? "可执行本机浏览器操作" : (componentsReady ? "当前会话待连接" : "本机环境不可用"),
    trustReason: reason,
    canRefresh: true,
    canBindTrustedNode: !!context.canBindTrustedNode && componentsReady,
    bindActionText: bound ? "刷新本机执行状态" : "连接当前会话",
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
