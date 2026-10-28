import { describe, expect, it } from 'vitest'
import { opColumnWidth } from './opColumnWidth.js'

// 期望值直接由 tdesign token 推出（推导见 opColumnWidth.js 顶部注释）。
// token 或按钮字号变了，这里的数字会先红。
describe('opColumnWidth', () => {
  it('grows with button count: a 2-3 button column is far narrower than the old hand-picked widths', () => {
    expect(opColumnWidth(['详情'])).toBe(80)
    expect(opColumnWidth(['详情', '编辑', '复制'])).toBe(152)
    expect(opColumnWidth(['下载', '详情', '去发布', '归档'], { loading: true })).toBe(230)
  })

  it('answers a loading button with extra room for the spinner', () => {
    expect(opColumnWidth(['下载', '详情'], { loading: true })).toBe(122)
    expect(opColumnWidth(['下载', '详情'], { loading: true })).toBe(opColumnWidth(['下载', '详情']) + 18)
  })

  it('fits five text actions without reserving five outlined-button frames', () => {
    const labels = ['详情', '编辑', '检测', '设置配额', '删除']
    expect(opColumnWidth(labels, { variant: 'text' })).toBe(244)
    expect(opColumnWidth(labels, { variant: 'text', primary: true })).toBe(256)
    expect(opColumnWidth(labels, { variant: 'text', loading: true })).toBe(262)
    expect(opColumnWidth(labels, { variant: 'text', loading: true, primary: true })).toBe(274)
    expect(opColumnWidth(labels)).toBe(272)
  })

  it('never returns less width for more buttons, and bottoms out above the header text', () => {
    expect(opColumnWidth(['详情', '取消'])).toBeGreaterThan(opColumnWidth(['详情']))
    expect(opColumnWidth(['详情'])).toBe(80)
  })
})
