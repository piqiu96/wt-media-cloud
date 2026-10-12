import { describe, expect, it } from 'vitest'
import {
  expectDerivedScrollWidth,
  expectExplicitColumnWidths,
  expectMinuteResolutionTimestamps,
  pageReader,
} from '../../../shared/testing/listPageConventions.js'

// 量具与判据在 shared/testing，目录与列数留在这里 —— 与素材库两页同一套规则，
// 从第一张列表页起就受约束，见 materials/pages/ListPageConventions.test.js 的说明。
const read = pageReader(import.meta.url)

// 11 列：ID / 策略名称 / 游戏 / 内容类型 / 模板 / 画幅 / 可覆盖参数 / 当前版本 /
// 修改时间 / 状态 / 操作。分母写死，列数对不上立刻失败而不是空转。
const TABLES = [{ file: 'ComposeStrategyPage.vue', name: 'columns', count: 11 }]

describe('compose strategy list page conventions', () => {
  it('declares an explicit width on every table column instead of only a minWidth', () => {
    expectExplicitColumnWidths(read, TABLES)
  })

  it('derives the horizontal scroll width on every table', () => {
    expectDerivedScrollWidth(read, ['ComposeStrategyPage.vue'])
  })

  it('drops seconds from every list timestamp', () => {
    expectMinuteResolutionTimestamps(read, ['ComposeStrategyPage.vue'])
  })
})
