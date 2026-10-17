// schema → 表单字段描述。策略编辑器与合成任务创建对话框共用同一执行器 schema，
// 字段清单、类型与下限都来自 schema（executor.BasicCutV1() 发布），页面不写死。
//
// 字段顺序在展示层定一次：MySQL 的 JSON 列按键名长度、再按字节重排对象键，schema 读回来
// 时 start_seconds（13 字节）会掉到 end_seconds（11 字节）后面，页面上就成了「先结束、
// 后开始」。执行器 schema 本身不能动（已发布版本拒绝 schema 变更，存量策略也按 schema
// 匹配），所以这里给出人读顺序；不在表里的参数保持 schema 原序，排在其后。
const PARAM_ORDER = ['start_seconds', 'end_seconds', 'insert_seconds']

function paramRank(key) {
  const index = PARAM_ORDER.indexOf(key)
  return index === -1 ? PARAM_ORDER.length : index
}

export function schemaFields(schema) {
  const properties = schema?.properties
  if (!properties || typeof properties !== 'object') return []
  const required = Array.isArray(schema.required) ? schema.required : []
  return Object.entries(properties).map(([key, def]) => ({
    key,
    label: def.title || key,
    type: def.type || 'string',
    format: def.format,
    minimum: def.minimum,
    required: required.includes(key),
  })).sort((left, right) => paramRank(left.key) - paramRank(right.key))
}
