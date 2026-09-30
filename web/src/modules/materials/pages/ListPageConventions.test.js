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

// 走查四轮把封面/标题/来源平台三列合成一个「素材」格，8 列降到 6 列；走查五轮又加回
// 两列（素材状态、使用情况，规范 §7.2 的第二个状态维度），素材库回到 8 列。
// 走查七轮我的素材换了列但不加列数（游戏并入「素材」格副行，腾出的位置给「使用状态」），
// 仍是 6 列。分母留在这里，列数对不上就立刻失败，而不是让宽度断言在一个抽不到的数组上静默空转。
const TABLES = [
  { file: 'MaterialLibraryPage.vue', name: 'columns', count: 8 },
  { file: 'MyMaterialsPage.vue', name: 'columns', count: 6 },
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
