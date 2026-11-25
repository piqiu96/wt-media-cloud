import { IdentificationStatus, PlatformLoginStatus } from "../accounts/status.js";
import { OutputPublishStatus, OutputVideoStatus } from "../compose/outputStatus.js";

export function accountUnavailable(account, category, userId) {
  if (userId && String(account.user_id) !== String(userId)) return "只能选择本人账号创建发布任务";
  if (account.platform !== "baidu") return "当前阶段仅支持百度";
  if (!account.enabled) return "账号已停用";
  if (
    account.identification_status !== IdentificationStatus.Identified ||
    !account.platform_account_id
  )
    return "尚未识别真实账号";
  if (!account.browser_profile_id) return "尚未绑定浏览器窗口";
  if (account.platform_login_status !== PlatformLoginStatus.Normal) return "请先检查账号登录状态";
  if (!(account.category_ids || []).map(String).includes(String(category)))
    return "账号不支持当前类目";
  return "";
}
export function outputUnavailable(output, category, account) {
  if (String(output.category_id) !== String(category)) return "成片类目不匹配";
  if (account && String(output.team_id) !== String(account.team_id))
    return "成片团队不匹配";
  if (!output.platforms?.includes("baidu")) return "成片不适用于百度";
  if (output.occupied === true) return "该成片已被当前平台的任务占用";
  if (output.publish_status === OutputPublishStatus.Archived) return "成片已归档";
  if (output.video?.status !== OutputVideoStatus.Ready)
    return output.video?.reason || "视频文件不可用";
  return "";
}
export function publicationTextLength(description,topics) {return Array.from(description || "").length + (topics ? 1+Array.from(topics).length : 0);}
export function contentIssues(form) {
  const issues = [], content = form.content || {};
  if (!form.category_id || !form.media_account_id || !form.composite_output_id)
    issues.push({ field: "resource", message: "请选择发布类目、账号和成片" });
  if (!String(content.description || "").trim()) issues.push({ field: "text", message: "请填写发布标题" });
  else if (publicationTextLength(content.description,content.topics)>50) issues.push({ field: "text", message: "百度发布标题与话题合计不能超过50字（含分隔空格），请缩短后再确认" });
  if (!content.cover_file_id) issues.push({ field: "cover", message: "请选择发布封面" });
  if (!content.portrait_cover_file_id) issues.push({ field: "portrait", message: "请选择或生成3:4竖版封面" });
  if (!content.declaration) issues.push({ field: "declaration", message: "请按内容事实选择创作声明" });
  if (!content.confirmed) issues.push({ field: "confirmation", message: "请核对并确认当前文案、封面和声明" });
  return issues;
}
export function contentIssue(form) { return contentIssues(form)[0] || null; }

export function contentProblem(form) { return contentIssue(form)?.message || ""; }

// Only an explicit business rejection proves that the save did not happen.
export function definitivePublicationRejection(error) {
  return [18001, 18002, 18003, 18004, 18005].includes(error?.errcode)
}
