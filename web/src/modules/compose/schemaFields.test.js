import { describe, expect, it } from 'vitest'
import { schemaFields } from './schemaFields.js'

// 执行器 schema 存进 MySQL 的 JSON 列后，键序会被按「键名长度、再按字节」重排：
// start_seconds（13 字节）掉到 end_seconds（11 字节）后面。页面上于是成了「先结束、后开始」，
// 运营照着顺序填就把两个值填反了。执行器 schema 本身不能改（已发布版本拒绝 schema 变更，
// 存量策略也按 schema 匹配），所以顺序在展示层定一次。
const AS_STORED = {
  type: 'object',
  properties: {
    end_seconds: { type: 'integer', title: '结束秒数', minimum: 1 },
    start_seconds: { type: 'integer', title: '开始秒数', minimum: 0 },
  },
  required: ['start_seconds', 'end_seconds'],
}

describe('schema fields', () => {
  it('puts start before end no matter how the keys came back', () => {
    expect(schemaFields(AS_STORED).map((field) => field.key)).toEqual(['start_seconds', 'end_seconds'])
  })

  it('keeps parameters it does not know after the known ones, in schema order', () => {
    const schema = { properties: { zeta: {}, start_seconds: {}, alpha: {}, end_seconds: {} } }
    expect(schemaFields(schema).map((field) => field.key)).toEqual(['start_seconds', 'end_seconds', 'zeta', 'alpha'])
  })

  it('reads the label, type, minimum and required flag off the schema', () => {
    const [start, end] = schemaFields(AS_STORED)
    expect(start).toEqual({ key: 'start_seconds', label: '开始秒数', type: 'integer', minimum: 0, required: true })
    expect(end.minimum).toBe(1)
    expect(end.required).toBe(true)
  })

  it('leaves out of required whatever the schema does not require', () => {
    const schema = { properties: { insert_seconds: { type: 'integer', title: '插入秒数' } } }
    expect(schemaFields(schema)).toEqual([{ key: 'insert_seconds', label: '插入秒数', type: 'integer', minimum: undefined, required: false }])
  })

  it('falls back to the key as the label and to string as the type', () => {
    expect(schemaFields({ properties: { note: {} } })[0]).toMatchObject({ label: 'note', type: 'string' })
  })

  it('is empty for a missing or shapeless schema', () => {
    expect(schemaFields(undefined)).toEqual([])
    expect(schemaFields({})).toEqual([])
    expect(schemaFields({ properties: null })).toEqual([])
  })
})
