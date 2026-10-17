import { describe, expect, it } from 'vitest'
import { buildOverridePayload, effectiveConfig, prefillOverrides, validateOverrides } from './taskOverrides.js'

// 执行器发布的 basic_cut v1 的样子：开始/结束必填且有下限，插入秒数可选。
const FIELDS = [
  { key: 'start_seconds', label: '开始秒数', type: 'integer', minimum: 0, required: true },
  { key: 'end_seconds', label: '结束秒数', type: 'integer', minimum: 1, required: true },
  { key: 'insert_seconds', label: '插入秒数', type: 'integer', minimum: 0, required: false },
]
const STRATEGY = { start_seconds: 0, end_seconds: 30 }

describe('prefill overrides', () => {
  it('fills what the strategy saved and leaves the rest blank', () => {
    expect(prefillOverrides(FIELDS, STRATEGY)).toEqual({ start_seconds: 0, end_seconds: 30 })
  })

  // 0 是填了的：把它当空会让人以为策略里的 0 没生效。
  it('keeps a saved zero and invents nothing for a key the strategy never set', () => {
    const prefilled = prefillOverrides(FIELDS, STRATEGY)
    expect(prefilled.start_seconds).toBe(0)
    expect(Object.hasOwn(prefilled, 'insert_seconds')).toBe(false)
  })

  it('survives a strategy without config and coerces a numeric field', () => {
    expect(prefillOverrides(FIELDS, null)).toEqual({})
    expect(prefillOverrides([{ key: 'note', type: 'string' }], { note: '原文' })).toEqual({ note: '原文' })
    // 策略里存成字符串的数字要变回数字：输入框要的是数字，字符串提交上去会被 schema 拒。
    expect(prefillOverrides([{ key: 'end_seconds', type: 'integer' }], { end_seconds: '30' })).toEqual({ end_seconds: 30 })
  })
})

describe('effective config', () => {
  it('lets a filled override win over the strategy value', () => {
    expect(effectiveConfig(FIELDS, STRATEGY, { end_seconds: 45 })).toEqual({ start_seconds: 0, end_seconds: 45 })
  })

  it('falls back to the strategy for a cleared field', () => {
    expect(effectiveConfig(FIELDS, STRATEGY, { end_seconds: '' })).toEqual({ start_seconds: 0, end_seconds: 30 })
  })
})

describe('the payload carries only what actually differs', () => {
  it('drops a value equal to the strategy value and keeps a real override', () => {
    expect(buildOverridePayload(FIELDS, STRATEGY, { start_seconds: 0, end_seconds: 45 })).toEqual({ end_seconds: 45 })
  })

  it('treats clearing a field as falling back to the strategy, not as an empty override', () => {
    expect(buildOverridePayload(FIELDS, STRATEGY, { start_seconds: '', end_seconds: null, insert_seconds: undefined })).toEqual({})
    expect(buildOverridePayload(FIELDS, STRATEGY, { end_seconds: '   ' })).toEqual({})
  })

  it('sends a numeric string as a number and a strategy-less field as-is', () => {
    expect(buildOverridePayload(FIELDS, STRATEGY, { end_seconds: '45' })).toEqual({ end_seconds: 45 })
    expect(buildOverridePayload(FIELDS, {}, { insert_seconds: 12 })).toEqual({ insert_seconds: 12 })
  })
})

describe('validation speaks Chinese and judges the effective value', () => {
  // 线上那次报错的原样：开始 3 秒、结束 1 秒。错挂在「结束秒数」上 —— 要改的通常是它。
  it('catches the online mistake and puts the error on the end field', () => {
    expect(validateOverrides(FIELDS, STRATEGY, { start_seconds: 3, end_seconds: 1 }))
      .toEqual({ end_seconds: '「结束秒数」必须大于「开始秒数」' })
  })

  // 策略里存着 start 0 / end 30，运营只把开始往后推过 30：结束那一格一字未动，自己看着
  // 没有问题，生效值却已经反了，服务端一定会拒 —— 所以要按**生效值**判，不是只看改过的格子。
  it('catches a strategy value the override made invalid without touching that field', () => {
    expect(validateOverrides(FIELDS, STRATEGY, { start_seconds: 45 }).end_seconds).toBe('「结束秒数」必须大于「开始秒数」')
    expect(validateOverrides(FIELDS, STRATEGY, { start_seconds: 30 }).end_seconds).toBe('「结束秒数」必须大于「开始秒数」')
  })

  it('wants the two ends strictly apart, so equal is refused too', () => {
    expect(validateOverrides(FIELDS, STRATEGY, { start_seconds: 15, end_seconds: 15 }).end_seconds).toBeTruthy()
  })

  it('asks for a required field the strategy did not fill either', () => {
    expect(validateOverrides(FIELDS, {}, {})).toEqual({ start_seconds: '请填写「开始秒数」', end_seconds: '请填写「结束秒数」' })
  })

  it('does not ask for a required field the strategy already fills', () => {
    expect(validateOverrides(FIELDS, STRATEGY, {})).toEqual({})
  })

  it('rejects a value below the minimum, a non-number and a fraction', () => {
    expect(validateOverrides(FIELDS, STRATEGY, { end_seconds: -5 }).end_seconds).toBe('「结束秒数」不能小于 1')
    expect(validateOverrides(FIELDS, STRATEGY, { end_seconds: 'abc' }).end_seconds).toBe('「结束秒数」必须是数字')
    expect(validateOverrides(FIELDS, STRATEGY, { end_seconds: 12.5 }).end_seconds).toBe('「结束秒数」必须是整数')
  })

  it('keeps the concrete reason instead of replacing it with the pair rule', () => {
    // -5 同时违反「不低于 1」和「大于开始秒数」；告诉运营下限，他才改得动。
    expect(validateOverrides(FIELDS, STRATEGY, { end_seconds: -5 }).end_seconds).not.toBe('「结束秒数」必须大于「开始秒数」')
  })
})
