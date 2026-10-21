import { reactive } from 'vue'
import { describe, expect, it } from 'vitest'
import { defaultCondition, displayRules, materialRulesSummary, resolveMaterialRules, saveRules } from './miningRules.js'

const libraryRow = () => reactive({
  material_rules: [
    { priority: 1, action: 'filter', logic: 'any', conditions: [{ field: 'published_at', operator: 'recent_hours', value: 24 }] },
    { priority: 2, action: 'match', logic: 'all', conditions: [{ field: 'like_count', operator: 'gte', value: 1000 }] },
  ],
})

describe('mining material rules', () => {
  it('把库里的响应式规则搬进表单，表单与那一行不共享对象', () => {
    const row = libraryRow()
    const display = displayRules(row.material_rules)

    expect(display).toEqual([
      { priority: 1, action: 'filter', logic: 'any', conditions: [{ field: 'published_at', operator: 'recent_hours', value: 24 }] },
      { priority: 2, action: 'match', logic: 'all', conditions: [{ field: 'like_count', operator: 'gte', value: 1000 }] },
    ])
    expect(display[0]).not.toBe(row.material_rules[0])
    display[0].conditions[0].value = 48
    expect(row.material_rules[0].conditions[0].value).toBe(24)
  })

  it('发布时间在表单里是时间戳，回存时还原成 ISO；计数保持数字', () => {
    const row = reactive({
      material_rules: [{ priority: 1, action: 'match', logic: 'all', conditions: [{ field: 'published_at', operator: 'gte', value: '2026-01-01T00:00:00Z' }] }],
    })
    const display = displayRules(row.material_rules)
    expect(display[0].conditions[0].value).toBe(Date.parse('2026-01-01T00:00:00Z'))

    const payload = saveRules(display)
    expect(payload[0].conditions[0].value).toBe('2026-01-01T00:00:00.000Z')
    expect(saveRules([{ priority: 1, action: 'match', logic: 'all', conditions: [{ field: 'like_count', operator: 'gte', value: '20000' }] }])[0].conditions[0].value).toBe(20000)
  })

  it('新条件默认从发布时间近 1 天下手', () => {
    expect(defaultCondition()).toEqual({ field: 'published_at', operator: 'recent_days', value: 1 })
  })

  it('优先读 material_rules，空数组表示只采集', () => {
    expect(resolveMaterialRules({ material_rules: [] })).toEqual([])
    const rules = [{ priority: 1, action: 'match', logic: 'all', conditions: [{ field: 'like_count', operator: 'gte', value: 1 }] }]
    expect(resolveMaterialRules({ material_rules: rules, auto_material: false })).toBe(rules)
  })

  it('把旧配置翻译成一组命中规则', () => {
    expect(resolveMaterialRules({ auto_material: false, like_threshold: 100 })).toEqual([])
    expect(resolveMaterialRules({ auto_material: true })).toEqual([])
    expect(resolveMaterialRules({
      auto_material: true,
      material_rule: 'OR',
      like_threshold: 10000,
      favorite_threshold: '500',
    })).toEqual([{
      priority: 1,
      action: 'match',
      logic: 'any',
      conditions: [
        { field: 'like_count', operator: 'gte', value: 10000 },
        { field: 'favorite_count', operator: 'gte', value: 500 },
      ],
    }])
    expect(resolveMaterialRules({ auto_material: true, material_rule: 'AND', like_threshold: 100 })[0].logic).toBe('all')
  })

  it('摘要按优先级展示过滤/命中组', () => {
    expect(materialRulesSummary({})).toBe('不自动转素材')
    expect(materialRulesSummary({ material_rules: [] })).toBe('不自动转素材')
    expect(materialRulesSummary({
      material_rules: [
        { priority: 2, action: 'match', logic: 'all', conditions: [{ field: 'like_count', operator: 'gte', value: 10000 }, { field: 'favorite_count', operator: 'gte', value: 500 }] },
        { priority: 1, action: 'filter', logic: 'any', conditions: [{ field: 'published_at', operator: 'recent_hours', value: 24 }] },
      ],
    })).toBe('过滤：近24小时；命中：赞≥1万 且 藏≥500')
  })

  it('旧配置的摘要与清单一致', () => {
    expect(materialRulesSummary({ auto_material: true, material_rule: 'OR', like_threshold: 1000, favorite_threshold: 2000 }))
      .toBe('命中：赞≥1千 或 藏≥2千')
  })
})
