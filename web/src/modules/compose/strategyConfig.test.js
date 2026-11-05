import { describe, expect, it } from 'vitest'
import { templateLabel } from './strategyConfig.js'
describe('bound template label', () => {
  it('retains the bound version when the related drawer has no catalog', () => {
    expect(templateLabel([], 7, 2)).toBe('模板 7 v2')
    expect(templateLabel([{id:7,name:'基础切段',version:3}],7,2)).toBe('基础切段 v2')
  })
  it('keeps the ordinary catalog label and missing-id fallback', () => {
    expect(templateLabel([{id:7,name:'基础切段',version:3}],7)).toBe('基础切段 v3')
    expect(templateLabel([],null)).toBe('—')
  })
})
