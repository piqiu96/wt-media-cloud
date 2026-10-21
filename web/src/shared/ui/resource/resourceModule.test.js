import { existsSync, readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const resourceRoot = new URL('./', import.meta.url)
const read = (file) => readFileSync(new URL(file, resourceRoot), 'utf8')

describe('operation resource design primitives', () => {
  it('loads the shared token and resource styles from both application entrypoints', () => {
    const cloudEntry = readFileSync(new URL('../../../apps/cloud/main.ts', import.meta.url), 'utf8')
    const desktopEntry = readFileSync(new URL('../../../apps/desktop/main.ts', import.meta.url), 'utf8')

    expect(cloudEntry).toContain('design-token.css')
    expect(cloudEntry).toContain('resource-module.css')
    expect(desktopEntry).toContain('design-token.css')
    expect(desktopEntry).toContain('resource-module.css')
  })

  it('provides token-backed presentation components for resource pages', () => {
    for (const file of ['ResourcePageHeader.vue', 'ResourceCard.vue', 'ResourceStatGrid.vue', 'ResourceStatusBadge.vue', 'MetricList.vue']) {
      expect(existsSync(new URL(file, resourceRoot))).toBe(true)
    }

    expect(read('ResourcePageHeader.vue')).toContain("title: { type: String, required: true }")
    expect(read('ResourceCard.vue')).toContain('<slot />')
    expect(read('ResourceStatGrid.vue')).toContain("items: { type: Array, required: true }")
    expect(read('ResourceStatusBadge.vue')).toContain("tone: { type: String, default: 'neutral' }")
    expect(read('../../../styles/design-token.css')).toContain('--wt-primary: #2563EB')

    // MetricList 竖排：容器是 column 方向，每项各占一行。
    const metrics = read('MetricList.vue')
    expect(metrics).toContain("items: { type: Array, default: () => [] }")
    expect(metrics).toContain('v-for="item in items"')
    expect(metrics).toMatch(/\.metric-list \{[^}]*flex-direction: column/)
    expect(metrics).toMatch(/\.metric-list__item \{[^}]*white-space: nowrap/)
  })

  it('keeps resource filters adaptive while table operations remain pinned', () => {
    const styles = read('../../styles/resource-module.css')

    expect(styles).not.toContain('.filter-bar > .t-space')
    expect(styles).toContain('flex-wrap: wrap')
    expect(styles).toContain('width: max-content')
    expect(styles).toContain('.t-table__fixed-right')
  })

  // 自适应筛选栏：字段是 .filter-bar 直接子元素时的布局规则——主条件（--wide）
  // 可伸可缩，普通条件固定基准宽，动作组居右。
  it('provides the adaptive filter-bar layout for direct-child filter fields', () => {
    const styles = read('../../styles/resource-module.css')

    expect(styles).toMatch(/\.filter-bar \{[^}]*display: flex/)
    expect(styles).toMatch(/\.filter-bar \{[^}]*flex-wrap: wrap/)
    expect(styles).toMatch(/\.filter-bar > \.wt-filter-field \{[^}]*flex: 0 1 200px/)
    expect(styles).toMatch(/\.filter-bar > \.wt-filter-field--wide \{[^}]*flex: 1 1 280px/)
    expect(styles).toMatch(/\.filter-bar > \.wt-filter-actions \{[^}]*margin-left: auto/)
  })

  it('renders metric cards as controls only when the page supports metric filtering', () => {
    const source = read('ResourceStatGrid.vue')
    expect(source).toContain("interactive: { type: Boolean, default: false }")
    expect(source).toContain("interactive ? 'button' : 'div'")
  })

  // t-input / t-select 渲染出来的根元素是 .t-input__wrap / .t-select__wrap——挂在组件上的
  // .t-input / .t-select 类在内层，`> .t-input` 这类直接子选择器永远匹配不到。TDesign 只给
  // 这两个根 width:100%，不给收缩许可：字段基准宽扣掉标签后小于 input 的 min-content 时，
  // 整个控件会溢出字段、盖住右边字段的标签（合成策略页筛选栏实测）。控件根必须能缩。
  it('lets the real tdesign control roots shrink inside a filter field', () => {
    const styles = read('../../styles/resource-module.css')

    expect(styles).toMatch(/\.wt-filter-field > \.t-input__wrap,\s*\n\.wt-resource-page \.wt-filter-field > \.t-select__wrap \{[^}]*flex: 1 1 auto/)
    expect(styles).toMatch(/\.wt-filter-field > \.t-input__wrap,\s*\n\.wt-resource-page \.wt-filter-field > \.t-select__wrap \{[^}]*min-width: 0/)
    // 死选择器（匹配不到任何真实 DOM）不留着误导下一个人。
    expect(styles).not.toMatch(/\.wt-filter-field > \.t-input \{/)
    expect(styles).not.toMatch(/\.wt-filter-field > \.t-select \{/)
  })
})
