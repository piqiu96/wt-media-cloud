import { describe, expect, it } from 'vitest'
import {
  expectDerivedScrollWidth,
  expectExplicitColumnWidths,
  expectMinuteResolutionTimestamps,
  pageReader,
} from '../../../shared/testing/listPageConventions.js'

// 量具与判据在 shared/testing（抽出来就是为了这里能用同一套），**目录与列数留在这里**。
// 这两个页面是本 CHG 新增的，它们从第一天起就受同一条规则约束：内容池那三页当年是
// 各写各的，于是日期格式化出现了三份、其中两份带秒。
const read = pageReader(import.meta.url)

const FILES = ['MaterialLibraryPage.vue', 'MyMaterialsPage.vue']

const TABLES = [
  { file: 'MaterialLibraryPage.vue', name: 'columns', count: 8 },
  { file: 'MyMaterialsPage.vue', name: 'columns', count: 8 },
]

describe('material list page conventions', () => {
  it('declares an explicit width on every table column instead of only a minWidth', () => {
    expectExplicitColumnWidths(read, TABLES)
  })

  it('derives the horizontal scroll width on every table', () => {
    expectDerivedScrollWidth(read, FILES)
  })

  it('drops seconds from every list timestamp', () => {
    expectMinuteResolutionTimestamps(read, FILES)
  })
})
