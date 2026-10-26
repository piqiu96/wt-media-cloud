// 负责人下拉清空后，TDesign 会把 v-model 写成 undefined（不是空串）；页面里任何
// 直接读 `.startsWith` 的地方都会在“清空再筛选”时抛错。
export function operatorIdFromOption(value) {
  if (typeof value !== 'string' || !value.startsWith('op:')) return 0
  const id = Number(value.slice(3))
  return Number.isSafeInteger(id) && id > 0 ? id : 0
}

export function missingOperatorOption(value, options) {
  if (typeof value !== 'string' || value === '') return false
  return !(options || []).some((option) => option.value === value)
}
