import { describe, it, expect } from 'vitest'
import { descriptionLayout } from './descriptionLayout.js'
describe('description layout uses actual container and readable field density', () => {
  const short = ['ID', '地区', '状态', '额度'].map(label => ({ label }))
  it('keeps sparse or narrow details in one column', () => {
    expect(descriptionLayout(short.slice(0, 2), 760).columns).toBe(1)
    expect(descriptionLayout(short, 639).columns).toBe(1)
    expect(descriptionLayout(short, 640).columns).toBe(2)
  })
  it('gives long values a full row without inflating short field density', () => {
    expect(descriptionLayout([...short, { label: '认证密码' }, { label: '运营备注' }], 752).spans).toEqual([1, 1, 1, 1, 2, 2])
    expect(descriptionLayout([{ label: '名称' }, { label: '地址' }, ...short.slice(0, 2)], 752).columns).toBe(1)
  })
  it('supports a bare full attribute from Vue as well as boolean binding', () => {
    expect(descriptionLayout([...short, { label: '扩展字段', full: '' }], 752).spans.at(-1)).toBe(2)
    expect(descriptionLayout([...short, { label: '扩展字段', full: false }], 752).spans.at(-1)).toBe(1)
  })
  it('allows an explicit full row for unusual labels and recomputes after resize', () => {
    const fields = [...short, { label: '扩展字段', full: true }]
    expect(descriptionLayout(fields, 752).spans.at(-1)).toBe(2)
    expect(descriptionLayout(fields, 500).spans).toEqual([1, 1, 1, 1, 1])
  })
})
