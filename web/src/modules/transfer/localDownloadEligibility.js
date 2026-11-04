export function localDownloadReason(user) {
  if (!user) return '正在确认本机下载权限';
  return ['operator', 'senior_operator'].includes(user.role) ? '' : '本机下载仅对运营和高级运营开放，当前账号不能执行本机下载';
}
