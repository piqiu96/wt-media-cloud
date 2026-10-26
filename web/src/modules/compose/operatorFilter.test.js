import { describe, expect, it } from 'vitest'
import { missingOperatorOption, operatorIdFromOption } from './operatorFilter.js'

describe('operator filter value', () => {
  // 清空下拉的真实形状：undefined 不是空串（TDesign clear 的行为）。
  it('treats every empty shape as no operator', () => {
    expect(operatorIdFromOption(undefined)).toBe(0)
    expect(operatorIdFromOption(null)).toBe(0)
    expect(operatorIdFromOption('')).toBe(0)
    expect(operatorIdFromOption(3)).toBe(0)
    expect(operatorIdFromOption({ value: 'op:3' })).toBe(0)
  })

  it('reads a member id only from a well-formed op: option', () => {
    expect(operatorIdFromOption('op:3')).toBe(3)
    expect(operatorIdFromOption('op:03')).toBe(3)
    expect(operatorIdFromOption('op:0')).toBe(0)
    expect(operatorIdFromOption('op:-1')).toBe(0)
    expect(operatorIdFromOption('op:2.5')).toBe(0)
    expect(operatorIdFromOption('op:abc')).toBe(0)
    expect(operatorIdFromOption('op:')).toBe(0)
    expect(operatorIdFromOption('op:9007199254740993')).toBe(0)
    expect(operatorIdFromOption('mine')).toBe(0)
  })
})

describe('missing operator option', () => {
  const options = [{ value: 'op:3', label: 'pika' }]

  it('only reports a non-empty value that has no matching option', () => {
    expect(missingOperatorOption(undefined, options)).toBe(false)
    expect(missingOperatorOption(null, options)).toBe(false)
    expect(missingOperatorOption('', options)).toBe(false)
    expect(missingOperatorOption('op:3', options)).toBe(false)
    expect(missingOperatorOption('op:9', options)).toBe(true)
  })

  it('keeps an untrusted value when the option list failed to load', () => {
    expect(missingOperatorOption('op:3', [])).toBe(true)
    expect(missingOperatorOption('op:3', undefined)).toBe(true)
  })
})
