export const PublicationStatus = Object.freeze({
  Pending: "pending",
  Queued: "queued",
  Running: "running",
  AwaitingHuman: "awaiting_human",
  Uncertain: "uncertain",
  Reviewing: "reviewing",
  Failed: "failed",
  Published: "published",
  Cancelled: "cancelled",
  Abandoned: "abandoned",
});

export const AttemptStage = Object.freeze({
  Claimed: "claimed",
  Preparing: "preparing",
  Ready: "ready",
  Submitting: "submitting",
  AwaitingHuman: "awaiting_human",
  ConnectionLost: "connection_lost",
  Finished: "finished",
});

export const publicationStatuses = Object.freeze({
  [PublicationStatus.Pending]: "待执行",
  [PublicationStatus.Queued]: "排队中",
  [PublicationStatus.Running]: "执行中",
  [PublicationStatus.AwaitingHuman]: "等待人工",
  [PublicationStatus.Uncertain]: "结果待确认",
  [PublicationStatus.Reviewing]: "平台审核中",
  [PublicationStatus.Failed]: "发布失败",
  [PublicationStatus.Published]: "已发布",
  [PublicationStatus.Cancelled]: "已取消",
  [PublicationStatus.Abandoned]: "已放弃",
});

export const attemptStageLabels = Object.freeze({
  [AttemptStage.Claimed]: "已领取",
  [AttemptStage.Preparing]: "准备资源",
  [AttemptStage.Ready]: "资源及账号已核对",
  [AttemptStage.Submitting]: "正在提交",
  [AttemptStage.AwaitingHuman]: "等待人工",
  [AttemptStage.ConnectionLost]: "连接中断，等待核实",
  [AttemptStage.Finished]: "执行已结束",
});
