import { validateOverrides } from './taskOverrides.js'
export function groupOverridePayload(groups, draft, fields) {
  return groups.map((group) => ({ key: group.key, params: Object.fromEntries(fields.filter((field) => field.editable === true).filter((field) => Object.hasOwn(draft[group.key] || {}, field.key) && draft[group.key][field.key] !== group.params[field.key]).map((field) => [field.key, draft[group.key][field.key]])) })).filter((group) => Object.keys(group.params).length)
}
export function validateTaskGroup(group, draft, fields) {
  const errors = validateOverrides(fields, group.params, draft)
  for (const field of fields.filter((field) => field.editable === true)) {
    const value = draft[field.key]
    if (field.maximum_from_strategy && Number(value) > Number(group.params[field.key])) errors[field.key] = `「${field.label}」只能缩短，不能超过策略的 ${group.params[field.key]} 秒`
    if (field.required && (value === '' || value === null || value === undefined)) errors[field.key] = `请填写「${field.label}」`
    if (typeof field.maximum === 'number' && Number(value) > field.maximum) errors[field.key] = `「${field.label}」不能大于 ${field.maximum}`
  }
  return errors
}
