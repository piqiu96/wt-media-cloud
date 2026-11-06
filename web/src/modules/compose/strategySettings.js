import { schemaFields } from './schemaFields.js'

export const OUTPUT_PLATFORMS = [
  { value: 'bilibili', label: 'B站' }, { value: 'baidu', label: '百度' }, { value: 'xiaohongshu', label: '小红书' },
]
const clone = (value) => JSON.parse(JSON.stringify(value))
export function settingFields(schema, defaults = {}) {
  return schemaFields(schema).map((field) => ({ ...field, ...schema.properties[field.key], default: defaults[field.key] ?? schema.properties[field.key].default, label: schema.properties[field.key].title || field.label }))
}
export function initialParams(fields) {
  return Object.fromEntries(fields.map((field) => [field.key, field.default ?? (['integer', 'number'].includes(field.type) ? field.minimum ?? 0 : field.type === 'boolean' ? false : '')]))
}
export function newSetting(fields, ordinal = 1) {
  return { key: crypto.randomUUID(), name: `成片设置 ${ordinal}`, platforms: [], copies: 1, params: initialParams(fields) }
}
export function copySetting(setting) {
  return { ...clone(setting), key: crypto.randomUUID(), name: `${setting.name} 副本` }
}
export function strategySettings(strategy) {
  if (strategy.output_settings?.length) return clone(strategy.output_settings)
  return [{ key: 'default', name: '默认成片', platforms: [], copies: 1, params: clone(strategy.config || {}) }]
}
export function validGuideURL(value) {
  try {
    const url = new URL(value)
    if (['https:', 'http:'].includes(url.protocol) && /^\/api\/v1\/file-resources\/[a-fA-F0-9]{64}$/.test(url.pathname) && !url.search && !url.hash && !url.username && !url.password) return true
    return url.protocol === 'https:' && !!url.hostname && !url.username && !url.password && !url.hash && (!url.port || url.port === '443')
  } catch { return false }
}
export function validateSettings(settings, fields) {
  const errors = {}
  if (!settings.length) return { settings: '至少保留一组成片设置' }
  for (const group of settings) {
    const issue = {}
    if (!String(group.name || '').trim()) issue.name = '请输入成片设置名称'
    if (!group.platforms?.length || group.platforms.some((value) => !OUTPUT_PLATFORMS.some((option) => option.value === value))) issue.platforms = '请选择适用平台'
    if (!Number.isInteger(group.copies) || group.copies < 1 || group.copies > 20) issue.copies = '每条素材生成 1～20 份'
    for (const field of fields) {
      const value = group.params[field.key]
      if (value === null || value === undefined || value === '') {
        if (field.required) issue[field.key] = `请填写${field.label}`
        continue
      }
      if (['integer', 'number'].includes(field.type)) {
        if (typeof value !== 'number' || !Number.isFinite(value) || (field.type === 'integer' && !Number.isInteger(value)) || (field.minimum !== undefined && value < field.minimum) || (field.maximum !== undefined && value > field.maximum)) issue[field.key] = `请填写有效的${field.label}`
      } else if (field.type === 'boolean' && typeof value !== 'boolean') issue[field.key] = '请选择有效值'
      else if ((field.format === 'uri' || field.key === 'guide_video_url') && !validGuideURL(value)) issue[field.key] = '请填写文件管理中的视频链接或 HTTPS 视频地址'
    }
    if (Object.keys(issue).length) errors[group.key] = issue
  }
  return errors
}
export function platformText(values) {
  return (values || []).map((value) => OUTPUT_PLATFORMS.find((option) => option.value === value)?.label || value).join('、') || '未设置'
}
