// 执行类错误码 → 用户能照做的下一步。
//
// 按 `code` 分支而不是解析文案：同一条消息可能来自 Cloud、Tauri 或 Agent，
// 文案会变，码不会。未收录的码退回原始消息，至少保留排查线索。
const EXECUTION_ERROR_TEXT = Object.freeze({
  BIT_NOT_BOUND: '尚未绑定比特浏览器账号，请到「个人信息」页完成绑定。',
  BIT_MAIN_USER_MISMATCH: '比特浏览器登录账号与系统绑定账号不一致，请到「个人信息」页确认换绑。',
  BIT_BROWSER_OFFLINE: '比特浏览器未运行或不可达，请先启动比特浏览器后重试。',
  bit_browser_offline: '比特浏览器未运行或无法连接，请先启动比特浏览器并完成登录，然后重试。',
  bitbrowser_response_error: '比特浏览器未能完成操作，请检查比特浏览器的运行状态后重试；若持续失败，请联系管理员。',
  agent_unreachable: '本机服务无法连接，请重新启动 Desktop 后重试。',
  PROFILE_NOT_FOUND: '未找到对应的浏览器环境，请重新关联浏览器窗口。',
  PROFILE_ASSIGNED_TO_OTHER_USER: '该浏览器环境属于其他用户，请选择自己的窗口。',
  PROFILE_LOGIN_INVALID: '浏览器环境未登录目标平台账号，请先在窗口中登录。',
  FILE_PREPARE_FAILED: '本地文件准备失败，请检查下载目录与磁盘空间后重试。',
  EXECUTION_INTERRUPTED: '任务执行被中断，可以重试。',
  RESULT_UNKNOWN: '任务结果不明，请先检查平台实际状态，再决定是否重新执行。',
  STALE_ATTEMPT: '这次执行已被更新的执行取代，结果未采用。',
  DESKTOP_LOGIN_REQUIRED: '登录状态已失效，请重新登录。',
  AUTH_DESKTOP_REPLACED: '你的账号已在另一台电脑登录，请重新登录。',
  AUTH_AGENT_TOKEN_INVALID: '本机执行凭证无效，请重新登录后重试。',
  AUTH_AGENT_TOKEN_EXPIRED: '本机执行凭证已过期，请重新登录后重试。',
})

const TIMEOUT_TEXT = 'BitBrowser窗口操作超时：Local Agent 已请求 BitBrowser，但 BitBrowser 未在限定时间内返回。请查看 Agent 日志确认是否已延迟打开，或稍后重试。'

export function executionErrorMessage(error, fallback = '本机操作失败，请查看 Agent 日志。') {
  const message = String(error?.message || error || '')
  const text = EXECUTION_ERROR_TEXT[error?.code]
  if (text) return text
  if (message.includes('timed out') || message.includes('timeout')) return TIMEOUT_TEXT
  return message || fallback
}
