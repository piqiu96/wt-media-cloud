import { reactive } from 'vue'
import { describe, expect, it } from 'vitest'
import { defaultCondition, displayRules, saveRules } from './strategyRules.js'

// 编辑器的输入就是列表行本身：rows 是 ref，row.priority_rules 是 Vue 代理。
// 这里用 reactive 复刻那个输入形态——搬不动代理的克隆方式会抛 DataCloneError，
// 页面上表现为「点编辑没反应」（抽屉不出现、只有一条 console 报错）。
const libraryRow = () => reactive({
  priority_rules: [
    { priority: 1, action: 'filter', logic: 'all', conditions: [{ field: 'favorite_count', operator: 'lte', value: 500 }] },
    { priority: 2, action: 'match', logic: 'any', conditions: [{ field: 'published_at', operator: 'recent_days', value: 60 }] },
  ],
})

describe('compose strategy rules', () => {
  it('把库里的响应式规则搬进表单，表单与那一行不共享对象', () => {
    const row = libraryRow()
    const display = displayRules(row.priority_rules)

    expect(display).toEqual([
      { priority: 1, action: 'filter', logic: 'all', conditions: [{ field: 'favorite_count', operator: 'lte', value: 500 }] },
      { priority: 2, action: 'match', logic: 'any', conditions: [{ field: 'published_at', operator: 'recent_days', value: 60 }] },
    ])
    expect(display[0]).not.toBe(row.priority_rules[0])
    display[0].conditions[0].value = 42
    expect(row.priority_rules[0].conditions[0].value).toBe(500)
  })

  // 发布时间的两档运算符在表单里是时间戳（日期选择器要数字），回存必须还原成 ISO。
  it('发布时间在表单里是时间戳，回存时还原成 ISO；计数保持数字', () => {
    const row = reactive({
      priority_rules: [{ priority: 1, action: 'match', logic: 'all', conditions: [{ field: 'published_at', operator: 'gte', value: '2026-01-01T00:00:00Z' }] }],
    })
    const display = displayRules(row.priority_rules)
    const stamp = Date.parse('2026-01-01T00:00:00Z')
    expect(display[0].conditions[0].value).toBe(stamp)

    const payload = saveRules(display)
    expect(payload[0].conditions[0].value).toBe('2026-01-01T00:00:00.000Z')
    expect(saveRules([{ priority: 1, action: 'match', logic: 'all', conditions: [{ field: 'like_count', operator: 'gte', value: '20000' }] }])[0].conditions[0].value).toBe(20000)
  })

  it('新条件默认从发布时间近 1 天下手', () => {
    expect(defaultCondition()).toEqual({ field: 'published_at', operator: 'recent_days', value: 1 })
  })
})
