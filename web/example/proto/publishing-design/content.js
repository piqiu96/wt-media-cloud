// Explicitly simulated suggestions. Each response is bound to input AND field revisions.
export const contentFields = [
  "title",
  "shortTitle",
  "topics",
  "tags",
  "description",
];
export const fingerprint = (r) =>
  JSON.stringify([
    r.assetId,
    r.accountId,
    r.platform,
    r.category,
    r.templateId,
    r.templateVersion,
    r.coverSource,
    r.inputVersion || 0,
  ]);
export function initCandidate(r) {
  r.candidateId ||= crypto.randomUUID();
  r.inputVersion ||= 1;
  r.fieldVersions ||= {};
  r.ai ||= {};
  return r;
}
export function editField(r, field, value) {
  initCandidate(r);
  if (JSON.stringify(r[field]) === JSON.stringify(value)) return;
  r[field] = value;
  r.fieldVersions[field] = (r.fieldVersions[field] || 0) + 1;
  r.version = (r.version || 0) + 1;
  r.contentApproved = false;
  if (
    [
      "shortTitle",
      "subTitle",
      "coverSource",
      "templateId",
      "templateDocument",
      "coverMode",
    ].includes(field)
  )
    invalidateCover(r);
}
export function invalidateCover(r) {
  r.coverVariants = {};
  r.coverApproved = false;
  r.coverSignature = null;
  r.coverRender = "待重新合成";
}
export function requestSuggestion(r, fields = contentFields) {
  initCandidate(r);
  const token = crypto.randomUUID();
  r.ai.latest ||= {};
  for (const f of fields) r.ai.latest[f] = token;
  return {
    candidateId: r.candidateId,
    input: fingerprint(r),
    versions: { ...r.fieldVersions },
    fields,
    token,
  };
}
export function receiveSuggestion(r, request, values, error = null) {
  if (r.candidateId !== request.candidateId) return false;
  r.ai.suggestions ||= {};
  for (const f of request.fields) {
    if (r.ai.latest[f] !== request.token) continue;
    r.ai.suggestions[f] = {
      value: values?.[f],
      error,
      request,
      expired:
        fingerprint(r) !== request.input ||
        (r.fieldVersions[f] || 0) !== (request.versions[f] || 0),
      status: error ? "失败" : "待采用",
    };
  }
  return true;
}
export function adopt(r, field) {
  const s = r.ai?.suggestions?.[field];
  if (!s || s.error || s.status === "已采用") return false;
  if (
    s.request.input !== fingerprint(r) ||
    (r.fieldVersions[field] || 0) !== (s.request.versions[field] || 0) ||
    r.ai.latest[field] !== s.request.token
  ) {
    s.expired = true;
    s.status = "已过期";
    return false;
  }
  editField(r, field, s.value);
  s.status = "已采用";
  return true;
}
export function demoValues(r, style = "实战攻略") {
  return {
    title:
      r.category + "：" + (style === "简洁" ? "实战要点" : "这次转点怎么选"),
    shortTitle: r.category === "火影忍者" ? "连招进阶指南" : "转点实战技巧",
    topics: "#" + r.category + " #游戏攻略",
    tags: r.category + ",游戏攻略",
    description: "结合本条成片整理操作与路线思路。",
  };
}
