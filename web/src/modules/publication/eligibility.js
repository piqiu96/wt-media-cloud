import { IdentificationStatus, PlatformLoginStatus } from "../accounts/status.js";
import { OutputPublishStatus, OutputVideoStatus } from "../compose/outputStatus.js";

export function accountUnavailable(account, category, userId, platform = 'baidu') {
  if (userId && String(account.user_id) !== String(userId)) return "只能选择本人账号创建发布任务";
  if (account.platform !== platform || !['baidu', 'bilibili'].includes(platform)) return platform === 'baidu' ? "当前阶段仅支持百度" : "账号发布平台不匹配";
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
export function outputUnavailable(output, category, account, platform = account?.platform || 'baidu') {
  if (String(output.category_id) !== String(category)) return "成片类目不匹配";
  if (account && String(output.team_id) !== String(account.team_id))
    return "成片团队不匹配";
  if (!output.platforms?.includes(platform)) return platform === 'bilibili' ? "成片不适用于B站" : "成片不适用于百度";
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
  if (form.platform === 'bilibili') {
    if (!String(content.title || '').trim() || content.title.length > 80 || /[\r\n]/.test(content.title)) issues.push({ field: 'title', message: '请填写80字以内的单行标题' });
    if (String(content.description || '').length > 2000) issues.push({ field: 'description', message: '简介不能超过2000字' });
    const options = content.bilibili || {};
    if (!Number.isSafeInteger(Number(options.partition_id)) || Number(options.partition_id) <= 0 || !options.partition_name?.trim()) issues.push({ field: 'partition', message: '请选择当前B站分区' });
    const tags = options.tags;
    if (!Array.isArray(tags) || !tags.length || tags.length > 10 || tags.some(tag => typeof tag !== 'string' || !tag.trim() || tag.length > 20 || /[,，\r\n]/.test(tag)) || new Set(tags).size !== tags.length) issues.push({ field: 'tags', message: '请填写1至10个不重复的标签，每个不超过20字' });
    if (!['内容无需标注', '含AI生成内容', '含虚构演绎内容', '内容含营销信息', '个人观点，仅供参考', '内容为转载'].includes(options.statement)) issues.push({ field: 'declaration', message: '请按内容事实选择创作声明' });
    if (options.statement === '内容为转载' && (!options.source?.trim() || options.source.length > 200 || options.rights_reserved)) issues.push({ field: 'source', message: '转载须填写200字以内来源说明，不能同时声明自制权利' });
    if (!['public', 'private'].includes(options.visibility)) issues.push({ field: 'visibility', message: '请选择可见范围' });
    if (!content.space_cover_file_id || content.space_cover_file_id === '0') issues.push({ field: 'space', message: '请选择或生成16:9个人空间封面' });
  } else {
  if (!String(content.description || "").trim()) issues.push({ field: "text", message: "请填写发布标题" });
  else if (publicationTextLength(content.description,content.topics)>50) issues.push({ field: "text", message: "百度发布标题与话题合计不能超过50字（含分隔空格），请缩短后再确认" });
  if (!content.cover_file_id) issues.push({ field: "cover", message: "请选择发布封面" });
  if (!content.portrait_cover_file_id) issues.push({ field: "portrait", message: "请选择或生成3:4竖版封面" });
  if (!content.declaration) issues.push({ field: "declaration", message: "请按内容事实选择创作声明" });
  }
  if (form.platform === 'bilibili' && (!content.cover_file_id || content.cover_file_id === '0')) issues.push({ field: 'cover', message: '请选择4:3首页推荐封面' });
  if (!content.confirmed) issues.push({ field: "confirmation", message: "请核对并确认当前文案、封面和声明" });
  return issues;
}
export function contentIssue(form) { return contentIssues(form)[0] || null; }

export function contentProblem(form) { return contentIssue(form)?.message || ""; }

// Only an explicit business rejection proves that the save did not happen.
export function definitivePublicationRejection(error) {
  return [18001, 18002, 18003, 18004, 18005].includes(error?.errcode)
}
