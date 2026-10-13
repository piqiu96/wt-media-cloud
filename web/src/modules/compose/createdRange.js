function localMidnight(day) {
  const parts = day.split('-').map(Number)
  if (parts.length !== 3 || parts.some((part) => !Number.isInteger(part))) throw new Error('请选择有效的日期范围')
  const date = new Date(parts[0], parts[1] - 1, parts[2])
  if (date.getFullYear() !== parts[0] || date.getMonth() !== parts[1] - 1 || date.getDate() !== parts[2]) {
    throw new Error('请选择有效的日期范围')
  }
  return date
}

export function createdRangeParams(range) {
  if (!Array.isArray(range) || !range[0] || !range[1]) return {}
  const from = localMidnight(range[0])
  const before = localMidnight(range[1])
  before.setDate(before.getDate() + 1)
  if (from >= before) throw new Error('结束日期不能早于开始日期')
  return { created_from: from.toISOString(), created_before: before.toISOString() }
}
