// 社媒账号登录状态的展示文案。状态码来自后端模型，页面只做映射。
export function loginStatusText(status) {
  return {
    unknown: '待检查',
    normal: '登录正常',
    not_logged_in: '未登录',
    verification_needed: '需要验证码',
    expired: '已失效',
    restricted: '受限',
    account_mismatch: '账号不一致',
    environment_error: '环境异常',
  }[status] || status || '-'
}
