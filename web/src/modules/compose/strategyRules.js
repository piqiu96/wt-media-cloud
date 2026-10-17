// 合成策略的优先级规则在「库里的形状」与「表单里的形状」之间来回转换。
// 单独成模块是为了能直接对响应式输入做单测：编辑器打开时传进来的 row 来自列表 ref，
// 它的 priority_rules 是 Vue 代理——搬不动代理的克隆方式会让「编辑」点了没反应。
export function defaultCondition() { return { field: 'published_at', operator: 'recent_days', value: 1 } }

export function ruleFormValue(condition) {
  if (condition.field === 'published_at' && ['gte', 'lt'].includes(condition.operator)) return Date.parse(condition.value)
  return condition.value
}

// 表单拿到的规则必须是新对象（改到一半取消不能把列表行也改了），但**不能**用
// structuredClone：传进来的是列表 ref 里的 Vue 代理，克隆代理会抛 DataCloneError。
// 逐层重建即可——规则和条件都是平铺字段。
export function displayRules(rules) {
  return rules.map((rule) => ({
    ...rule,
    conditions: rule.conditions.map((condition) => ({ ...condition, value: ruleFormValue(condition) })),
  }))
}

export function saveRules(rules) {
  return rules.map((rule) => ({ ...rule, conditions: rule.conditions.map((condition) => ({
    ...condition,
    value: condition.field === 'published_at' && ['gte', 'lt'].includes(condition.operator)
      ? new Date(condition.value).toISOString() : Number(condition.value),
  })) }))
}
