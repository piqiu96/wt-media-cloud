import { describe, expect, it } from 'vitest'
import { BOTTOM_EPSILON, BOTTOM_ZONE, HOME_NAV_SECTIONS, resolveActiveNav } from './scrollSpy.js'

const VH = 900

// 通用 fixture:文档坐标,顺序与真实 /home 一致
const TOPS = [
  { id: 'hero', top: 0 },
  { id: 'product', top: 800 },
  { id: 'download', top: 1700 },
  { id: 'pricing', top: 2400 },
  { id: 'contact', top: 2600 },
]
const resolve = (scrollY, { scrollHeight = 3200, sectionTops = TOPS } = {}) =>
  resolveActiveNav({ scrollY, viewportHeight: VH, scrollHeight, sectionTops })

// 真实尾部 fixture:contact 后还有 ~249px 尾高(卡片+footer),
// 构造出「slack=60 时 contact 顶仍低于 0.67vh 判定线」——只有距底强制线能点亮联系。
const TAIL_TOPS = [
  { id: 'hero', top: 0 },
  { id: 'product', top: 800 },
  { id: 'download', top: 3500 },
  { id: 'pricing', top: 4200 },
  { id: 'contact', top: 4451 },
]
const resolveTail = (scrollY) => resolve(scrollY, { scrollHeight: 4700, sectionTops: TAIL_TOPS })

describe('resolveActiveNav', () => {
  it('hero 与 product 区都高亮产品', () => {
    expect(resolve(0)).toBe('product')
    expect(resolve(900)).toBe('product')
  })

  it('download / pricing 区分别高亮', () => {
    expect(resolve(1400)).toBe('download')
    expect(resolve(2100)).toBe('pricing')
  })

  it('进入近底区间后判定线上移,contact 提前点亮', () => {
    expect(resolve(2110)).toBe('pricing')
    expect(resolve(2150)).toBe('contact')
  })

  it('距底 BOTTOM_EPSILON 内强制 contact(旧逻辑 -2 在此失败)', () => {
    expect(resolveTail(3740)).toBe('contact')
    expect(resolveTail(3739)).toBe('pricing')
    expect(resolveTail(3800)).toBe('contact')
  })

  it('反向滚动时激活态逐级恢复', () => {
    expect([2300, 2100, 1400, 0].map(resolve)).toEqual(['contact', 'pricing', 'download', 'product'])
  })

  it('整页一屏即视为已在底部(pin:有意行为)', () => {
    expect(resolve(0, { scrollHeight: 800 })).toBe('contact')
  })
})

describe('scrollSpy 常量', () => {
  it('section→nav 映射与导航四项一致', () => {
    expect(HOME_NAV_SECTIONS.map((s) => s.id)).toEqual(['hero', 'product', 'download', 'pricing', 'contact'])
    expect(HOME_NAV_SECTIONS.map((s) => s.nav)).toEqual(['product', 'product', 'download', 'pricing', 'contact'])
  })

  it('强制线必须严格小于近底区间(clamp 落点不得落入强制圈)', () => {
    expect(BOTTOM_EPSILON).toBeLessThan(BOTTOM_ZONE)
  })
})
