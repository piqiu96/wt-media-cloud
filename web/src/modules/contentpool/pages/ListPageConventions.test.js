import { describe, expect, it } from 'vitest'
import { formatDateTime } from '../../../shared/utils/datetime.js'
import {
  expectDerivedScrollWidth,
  expectExplicitColumnWidths,
  expectMinuteResolutionTimestamps,
  fnBody,
  pageReader,
} from '../../../shared/testing/listPageConventions.js'

// 量具与判据在 shared/testing，**目录与列数留在这里** —— 这是本目录的事实。
const read = pageReader(import.meta.url)

const FILES = ['ContentPoolPage.vue', 'DiscoveryStrategiesPage.vue', 'CrawlTasksPage.vue']

// 三张列表页的每一张表。数字是「列数」——写死是为了让分母可见：
// 抽不到列或列数变了都会直接失败，而不是安静地变成一条空转的断言。
const TABLES = [
  { file: 'ContentPoolPage.vue', name: 'columns', count: 15 },
  { file: 'ContentPoolPage.vue', name: 'resultColumns', count: 9 },
  { file: 'DiscoveryStrategiesPage.vue', name: 'columns', count: 12 },
  { file: 'CrawlTasksPage.vue', name: 'columns', count: 11 },
  { file: 'CrawlTasksPage.vue', name: 'resultColumns', count: 7 },
]

describe('content pool list page conventions', () => {
  it('declares an explicit width on every table column instead of only a minWidth', () => {
    expectExplicitColumnWidths(read, TABLES)
  })

  // 横向滚动宽度全部由列宽推导，三页都不许再出现写死的 x。
  it('derives the horizontal scroll width on every table', () => {
    expectDerivedScrollWidth(read, FILES)
  })

  it('drops seconds from every list timestamp', () => {
    expectMinuteResolutionTimestamps(read, FILES)
  })

  // 判据本身也要被验：上一行断言的是「页面导入了同一个格式化器」，而格式化器
  // 自己被改坏时页面一行业不用动 —— 这四条钉的是它的输出。
  it('formats timestamps to the minute and renders midnight as 00:xx', () => {
    // 用本地时间构造，避免断言依赖运行环境的时区。
    expect(formatDateTime(new Date(2026, 8, 23, 16, 42, 54))).toBe('2026/9/23 16:42')
    // hour12: false 在部分实现下会把午夜渲染成 24:05。
    expect(formatDateTime(new Date(2026, 8, 23, 0, 5, 0))).toBe('2026/9/23 00:05')
    // 最宽的一刻：150px 定宽按这个长度算的。
    expect(formatDateTime(new Date(2026, 11, 31, 23, 59, 59))).toBe('2026/12/31 23:59')
    expect(formatDateTime(null)).toBe('-')
    expect(formatDateTime('')).toBe('-')
  })

  it('embeds a minute-resolution timestamp in the task name', () => {
    const body = fnBody(read('CrawlTasksPage.vue'), 'compactTime')
    expect(body).not.toBeNull()
    expect(body).not.toContain('getSeconds')
    expect(body).toContain('getMinutes')
  })
})

// 计数用「·」串成一行时，宽度不够会在任意位置折断，数字和标签被拆散。
// 这三条钉的是「这一页把指标块渲染成竖排」的具体函数名，属于本目录的事实。
describe('content pool metric blocks', () => {
  it('renders metric blocks vertically instead of joining them with a dot', () => {
    for (const [file, fn] of [
      ['ContentPoolPage.vue', 'interactionLabel'],
      ['CrawlTasksPage.vue', 'foundStats'],
      ['DiscoveryStrategiesPage.vue', 'latestStats'],
    ]) {
      const source = read(file)
      const body = fnBody(source, fn)
      expect(body, `${file} 里找不到 ${fn}`).not.toBeNull()
      expect(body, `${file} ${fn}`).not.toContain('·')
      expect(body, `${file} ${fn}`).toContain('label:')
      expect(body, `${file} ${fn}`).toContain('value:')
      expect(source, file).toContain('<MetricList')
    }
  })
})
