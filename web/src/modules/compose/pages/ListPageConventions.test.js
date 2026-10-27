import { describe, expect, it } from 'vitest'
import {
  expectDerivedScrollWidth,
  expectExplicitColumnWidths,
  expectMinuteResolutionTimestamps,
  pageReader,
} from '../../../shared/testing/listPageConventions.js'

// 合成两页与素材/内容池页受同一条列表规范约束：列宽显式、滚动宽推导、时间无秒。
const read = pageReader(import.meta.url)

const FILES = ['ComposeStrategyPage.vue', 'ComposeTasksPage.vue', 'MyOutputsPage.vue']

const TABLES = [
  { file: 'ComposeStrategyPage.vue', name: 'columns', count: 9 },
  { file: 'ComposeTasksPage.vue', name: 'columns', count: 11 },
  { file: 'MyOutputsPage.vue', name: 'columns', count: 11 },
]

describe('compose list page conventions', () => {
  it('declares an explicit width on every table column instead of only a minWidth', () => {
    expectExplicitColumnWidths(read, TABLES)
  })

  it('derives the horizontal scroll width on every table', () => {
    expectDerivedScrollWidth(read, FILES)
  })

  it('drops seconds from every list timestamp', () => {
    expectMinuteResolutionTimestamps(read, FILES)
  })

  // t-space 等宽网格分不出主次；这三页的筛选栏走自适应布局（AdminUI 规范 §2.1
  // 高频条件优先），字段是 .filter-bar 的直接子元素，宽度由修饰类定。
  it('lays every filter bar out adaptively instead of the equal-width t-space grid', () => {
    for (const file of FILES) {
      expect(read(file), file).not.toMatch(/filter-bar"[^>]*>\s*<t-space/)
    }
  })
})
