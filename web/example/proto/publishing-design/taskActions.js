// Isolated prototype adapter. Production must supply authenticated actor and executor evidence.
import { setResult } from "./relationships.js";
import { canCancelTask } from "./taskExperience.js";
export function publicWork(platform, value) {
  let u;
  try {
    u = new URL(value);
  } catch {
    throw Error("请填写最终公开作品链接");
  }
  if (u.protocol !== "https:" || u.username || u.password || u.port)
    throw Error("作品链接须为有效 HTTPS 地址");
  const rules = {
    B站: [/^(www\.)?bilibili\.com$/, /^\/video\/(BV[a-zA-Z0-9]+|av\d+)\/?$/],
    小红书: [
      /^(www\.)?xiaohongshu\.com$/,
      /^\/(?:explore|discovery\/item)\/([a-fA-F0-9]{24})\/?$/,
    ],
    百家号: [/^baijiahao\.baidu\.com$/, /^\/s\/?$/],
  };
  const rule = rules[platform];
  if (!rule || !rule[0].test(u.hostname)) throw Error("链接与任务平台不匹配");
  const match = u.pathname.match(rule[1]);
  const id = platform === "百家号" ? u.searchParams.get("id") : match?.[1];
  if (!match || !id || !/^[a-zA-Z0-9]+$/.test(id))
    throw Error("请使用具体作品链接，不能使用账号主页、投稿或审核页面");
  return {
    id,
    url:
      platform === "百家号"
        ? `https://baijiahao.baidu.com/s?id=${id}`
        : `https://${platform === "B站" ? "www.bilibili.com/video/" : "www.xiaohongshu.com/explore/"}${id}`,
  };
}
export function verifyResult(db, t, input, actor) {
  if (!["结果待确认", "平台审核中", "待补链接"].includes(t.status)) throw Error("任务状态已变化，请重新打开");
  if (t.status !== "结果待确认" && input.status !== "已发布") throw Error("仅能补录公开发布结果");
  if (!actor) throw Error("缺少已认证核实人，无法提交");
  const attempt = t.attemptHistory?.at(-1);
  if (
    !attempt ||
    !Number.isInteger(attempt.number) ||
    attempt.number < 1 ||
    input.attempt !== attempt.number
  )
    throw Error("执行尝试已变化，请重新核实");
  const { status, evidence = "", reason = "", url = "" } = input;
  if (!["已发布", "平台审核中", "执行失败", "结果待确认"].includes(status))
    throw Error("请选择核实结论");
  let work;
  if (status === "已发布") {
    if (!input.publicAt || !Number.isFinite(Date.parse(input.publicAt)) || Date.parse(input.publicAt) > Date.now()) throw Error("请选择有效的实际发布时间");
    work = publicWork(t.platform, url);
  }
  if (status === "平台审核中" && !evidence.trim())
    throw Error("请填写投稿编号或有效审核证据");
  if (
    status === "执行失败" &&
    (!reason.trim() ||
      !evidence.trim() ||
      !attempt.ended ||
      !t.executorEvidence?.confirmedUnpublished ||
      t.executorEvidence?.attempt !== attempt.number)
  )
    throw Error("需要失败原因、核实依据和执行器确认的终止及未发布证据");
  const at = new Date().toISOString();
  if (work) {
    db.works ||= [];
    let entity = db.works.find(
      (w) => w.platform === t.platform && w.platformId === work.id,
    );
    if (!entity) {
      entity = {
        id: crypto.randomUUID(),
        platform: t.platform,
        platformId: work.id,
        url: work.url,
        accountId: t.accountId,
        sourceTaskIds: [],
      };
      db.works.push(entity);
    }
    entity.sourceTaskIds ||= [];
    if (!entity.sourceTaskIds.includes(t.id)) entity.sourceTaskIds.push(t.id);
    t.workId = entity.id;
    t.workUrl = work.url;
    t.platformId = work.id;
  }
  t.verifications ||= [];
  t.verifications.push({
    actor,
    at,
    attempt: attempt.number,
    status,
    evidence,
    reason,
    url: work?.url || "",
    source: "manual",
  });
  t.verifiedAt = at;
  t.verifiedBy = actor;
  setResult(t, status, {
    publicAt: status === "已发布" ? new Date(input.publicAt).toISOString() : null,
    evidence: reason
      ? `${reason}；${evidence}`
      : evidence || work?.url || "仍无法判断",
  });
  if (status === "执行失败") t.retrySafe = true;
}
export function cancelPublication(t, reason = "") {
  if (!canCancelTask(t))
    throw Error(
      "无法证明执行已安全停止，未终止任务；请等待执行器证据或核实结果",
    );
  t.cancelReason = reason;
  t.cancelledAt = new Date().toISOString();
  setResult(t, "已取消", { evidence: reason || "终止任务；保留资源占用" });
}
export function processHuman(t, result) {
  if (t.status !== "等待人工") throw Error("任务状态已变化");
  if (result === "later") return;
  if (result === "submitted") {
    setResult(t, "结果待确认", {
      evidence: "人工反馈可能已提交，需核实平台结果",
    });
    return;
  }
  if (
    result !== "validated" ||
    !t.executorEvidence?.safeToResume ||
    t.executorEvidence?.attempt !== t.attemptHistory?.at(-1)?.number ||
    t.submittedAt
  )
    throw Error("尚未取得执行器验证结果，保留等待人工状态");
  setResult(t, "执行中", { evidence: "执行器确认验证完成，继续原尝试" });
}
