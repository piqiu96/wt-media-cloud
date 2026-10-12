// schema → 表单字段描述。策略编辑器与合成任务创建对话框共用同一执行器 schema，
// 字段清单、类型与下限都来自 schema（executor.BasicCutV1() 发布），页面不写死。
export function schemaFields(schema) {
  const properties = schema?.properties
  if (!properties || typeof properties !== 'object') return []
  const required = Array.isArray(schema.required) ? schema.required : []
  return Object.entries(properties).map(([key, def]) => ({
    key,
    label: def.title || key,
    type: def.type || 'string',
    minimum: def.minimum,
    required: required.includes(key),
  }))
}
