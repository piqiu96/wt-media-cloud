// 平台代码与展示文案的唯一归属。后端、数据库与合同只存稳定英文代码；
// 页面一律通过这里取中文名，不再各页各写一份映射，也不直接渲染代码。
// 小红书（xhs）目前没有采集/发布能力，只在这里占位，避免以后再加一套命名。
export const PLATFORM_LABELS = {
  douyin: '抖音',
  bilibili: 'B站',
  xhs: '小红书',
  baidu: '百度',
}

export function platformLabel(value, fallback = '-') {
  if (!value) return fallback
  return PLATFORM_LABELS[value] || fallback
}

export function platformOptions(codes) {
  return codes.map((value) => ({ value, label: PLATFORM_LABELS[value] || value }))
}

// 能力范围：挖掘只接抖音；社媒账号只接 B站/百度；开户三平台都接。
export const MINING_PLATFORMS = ['douyin']
export const ACCOUNT_PLATFORMS = ['bilibili', 'baidu']
export const ACCOUNT_OPENING_PLATFORMS = ['douyin', 'bilibili', 'baidu']

export const MINING_PLATFORM_OPTIONS = platformOptions(MINING_PLATFORMS)
export const ACCOUNT_PLATFORM_OPTIONS = platformOptions(ACCOUNT_PLATFORMS)
export const ACCOUNT_OPENING_PLATFORM_OPTIONS = platformOptions(ACCOUNT_OPENING_PLATFORMS)
