export const CATEGORY_TYPE_LABELS = {
  game: '游戏',
  ai: 'AI',
  anime: '二次元',
}

export const CATEGORY_TYPE_OPTIONS = Object.entries(CATEGORY_TYPE_LABELS).map(([value, label]) => ({ value, label }))

export function categoryTypeLabel(value, fallback = '—') {
  return CATEGORY_TYPE_LABELS[value] || fallback
}

/** 只保留当前仍可被新业务选择的类目；历史记录仍使用全量目录解析名称。 */
export function enabledCategories(categories) {
  if (!Array.isArray(categories)) return []
  return categories.filter((category) => category?.enabled === true)
}
