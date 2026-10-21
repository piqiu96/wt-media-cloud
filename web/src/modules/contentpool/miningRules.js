// 挖掘策略的「转素材规则」与合成策略的优先级规则同形：优先级 + 过滤/命中 +
// 全部/任一 + 条件。库里的 material_rules 是规则组数组，表单里的发布日期条件
// 用时间戳承载（t-date-picker value-type="time-stamp"），保存时换回 RFC3339。
//
// 历史配置把 auto_material / material_rule / like_threshold / favorite_threshold
// 直接摊在 config 上；resolveMaterialRules 在读取时翻译成同样形状，保存后后端
// 只写 material_rules。
import { formatDateTime } from '../../shared/utils/datetime.js'

export function defaultCondition() { return { field: 'published_at', operator: 'recent_days', value: 1 } }

export function ruleFormValue(condition) {
  if (condition.field === 'published_at' && ['gte', 'lt'].includes(condition.operator)) return Date.parse(condition.value)
  return condition.value
}

// 逐层重建而不是 structuredClone：传进来的是列表 ref 里的 Vue 代理，
// 克隆代理会抛 DataCloneError。
export function displayRules(rules) {
  return rules.map((rule) => ({
    ...rule,
    conditions: rule.conditions.map((condition) => ({ ...condition, value: ruleFormValue(condition) })),
  }))
}

export function saveRules(rules) {
  return rules.map((rule) => ({
    ...rule,
    conditions: rule.conditions.map((condition) => ({
      ...condition,
      value: condition.field === 'published_at' && ['gte', 'lt'].includes(condition.operator)
        ? new Date(condition.value).toISOString()
        : Number(condition.value),
    })),
  }))
}

export function resolveMaterialRules(config) {
  if (Array.isArray(config?.material_rules)) return config.material_rules
  if (!config?.auto_material) return []
  const conditions = []
  const like = Number(config.like_threshold || 0)
  const favorite = Number(config.favorite_threshold || 0)
  if (like > 0) conditions.push({ field: 'like_count', operator: 'gte', value: like })
  if (favorite > 0) conditions.push({ field: 'favorite_count', operator: 'gte', value: favorite })
  if (!conditions.length) return []
  return [{
    priority: 1,
    action: 'match',
    logic: String(config.material_rule || '').toUpperCase() === 'OR' ? 'any' : 'all',
    conditions,
  }]
}

function formatThreshold(value) {
  const n = Number(value || 0)
  if (n >= 10000) {
    const w = n / 10000
    return `${Number.isInteger(w) ? w : w.toFixed(1).replace(/\.0$/, '')}万`
  }
  if (n >= 1000) {
    const k = n / 1000
    return `${Number.isInteger(k) ? k : k.toFixed(1).replace(/\.0$/, '')}千`
  }
  return String(n)
}

function conditionSummary(condition) {
  const value = condition.value
  if (condition.field === 'published_at') {
    if (condition.operator === 'recent_hours') return `近${value}小时`
    if (condition.operator === 'recent_days') return `近${value}天`
    return `${condition.operator === 'gte' ? '发布≥' : '发布<'}${formatDateTime(value)}`
  }
  const field = condition.field === 'favorite_count' ? '藏' : '赞'
  const operator = condition.operator === 'eq' ? '=' : condition.operator === 'lte' ? '≤' : '≥'
  return `${field}${operator}${formatThreshold(value)}`
}

export function materialRulesSummary(config) {
  const rules = resolveMaterialRules(config)
  if (!rules.length) return '不自动转素材'
  return rules
    .slice()
    .sort((left, right) => (Number(left.priority) || 0) - (Number(right.priority) || 0))
    .map((rule) => {
      const joiner = rule.logic === 'any' ? ' 或 ' : ' 且 '
      return `${rule.action === 'filter' ? '过滤' : '命中'}：${rule.conditions.map(conditionSummary).join(joiner)}`
    })
    .join('；')
}
