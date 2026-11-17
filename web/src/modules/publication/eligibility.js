export function accountUnavailable(account, category) {
  if (account.platform !== "baidu") return "当前阶段仅支持百度";
  if (!account.enabled) return "账号已停用";
  if (
    account.identification_status !== "identified" ||
    !account.platform_account_id
  )
    return "尚未识别真实账号";
  if (!account.browser_profile_id) return "尚未绑定浏览器窗口";
  if (account.platform_login_status !== "normal") return "请先检查账号登录状态";
  if (!(account.category_ids || []).map(String).includes(String(category)))
    return "账号不支持当前类目";
  return "";
}
export function outputUnavailable(output, category, account) {
  if (String(output.category_id) !== String(category)) return "成片类目不匹配";
  if (account && String(output.team_id) !== String(account.team_id))
    return "成片团队不匹配";
  if (!output.platforms?.includes("baidu")) return "成片不适用于百度";
  if (output.publish_status === "archived") return "成片已归档";
  if (output.video?.status !== "ready")
    return output.video?.reason || "视频文件不可用";
  return "";
}
export const publicationStatuses = {
  pending: "待执行",
  queued: "排队中",
  running: "执行中",
  awaiting_human: "等待人工",
  uncertain: "结果待确认",
  reviewing: "平台审核中",
  failed: "发布失败",
  published: "已发布",
  cancelled: "已取消",
  abandoned: "已放弃",
};
export function contentProblem(form) {
  if (!form.category_id || !form.media_account_id || !form.composite_output_id)
    return "请选择发布类目、账号和成片";
  if (!form.content.description.trim()) return "请填写作品描述";
  if (!form.content.cover_file_id) return "请选择发布封面";
  if (!form.content.declaration) return "请按内容事实选择创作声明";
  if (!form.content.confirmed) return "请核对并确认当前文案、封面和声明";
  return "";
}

// Only an explicit business rejection proves that the save did not happen.
export function definitivePublicationRejection(error) {
  return [18001, 18002, 18003, 18004, 18005].includes(error?.errcode)
}
