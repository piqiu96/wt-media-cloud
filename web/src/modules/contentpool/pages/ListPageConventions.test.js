import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { formatDateTime } from '../../../shared/utils/datetime.js'

const read = (name) => readFileSync(new URL(`./${name}`, import.meta.url), 'utf8')

// 取出 `const <name> = [ ... ]` 的数组字面量内容，按方括号配平（列声明里没有嵌套数组）。
function arrayLiteral(source, name) {
  const start = source.indexOf(`const ${name} = [`)
  if (start === -1) return null
  const open = source.indexOf('[', start)
  let depth = 0
  for (let i = open; i < source.length; i += 1) {
    if (source[i] === '[') depth += 1
    else if (source[i] === ']') {
      depth -= 1
      if (depth === 0) return source.slice(open + 1, i)
    }
  }
  return null
}

function fnBody(source, name) {
  const start = source.indexOf(`function ${name}(`)
  if (start === -1) return null
  const end = source.indexOf('\n}', start)
  return end === -1 ? null : source.slice(start, end)
}

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
  // 这是本类 bug 唯一的闸门，理由见下。
  //
  // 实测：TDesign 内层 <table> 没有宽度，表格宽度完全由列声明推导（取 col.width ?? col.minWidth）。
  // minWidth 在 WebKit 的 fixed 布局下不产生确定列宽，这类列于是变成「剩余空间列」，
  // 可用宽度不足时全部缺口由它们承担：内容池「内容」列声明 340 实测被压到 91.5、
  // 「来源」声明 260 压到 100；挖掘任务「任务名称」220→76、「任务来源」160→76.5、
  // 「发现结果」240→76.5。声明 width 的列则逐列精确。
  //
  // Chrome 认 minWidth（同一份产物在无头 Chrome 里量出来是 340），所以 Chrome 模拟台
  // 对这种 bug 天然失明 —— 这条静态断言才是能在 CI 里挡住复发的判据。
  it('declares an explicit width on every table column instead of only a minWidth', () => {
    for (const { file, name, count } of TABLES) {
      const block = arrayLiteral(read(file), name)
      expect(block, `${file} 里找不到 ${name}`).not.toBeNull()

      const entries = [...block.matchAll(/\{[^{}]*\}/g)].map((m) => m[0])
      // 分母可见：列数对不上说明正则没抽对，立刻失败而不是空转。
      expect(entries, `${file} ${name}`).toHaveLength(count)

      const missing = entries.filter((entry) => !/\bwidth:\s*\d+/.test(entry))
      expect(missing, `${file} ${name} 有列没写 width`).toEqual([])
      // minWidth 写了也不生效，留着只会误导下一个人。
      expect(block, `${file} ${name}`).not.toContain('minWidth:')
    }
  })

  // 横向滚动宽度全部由列宽推导，三页都不许再出现写死的 x。
  it('derives the horizontal scroll width on every table', () => {
    for (const file of ['ContentPoolPage.vue', 'DiscoveryStrategiesPage.vue', 'CrawlTasksPage.vue']) {
      const source = read(file)
      expect(source, file).not.toContain(':scroll="{ x:')
      expect(source, file).toContain(':scroll="tableScroll"')
      expect(source, file).toContain('const tableScroll = computed')
    }
  })

  it('drops seconds from every list timestamp', () => {
    for (const file of ['ContentPoolPage.vue', 'DiscoveryStrategiesPage.vue', 'CrawlTasksPage.vue']) {
      const source = read(file)
      expect(source, file).toContain("from '../../../shared/utils/datetime.js'")
      // 各自实现一份日期格式化正是漂移的来源（其中两份带秒）。
      expect(source, file).not.toContain('function dateLabel(')
      expect(source, file).not.toContain('{ hour12: false }')
    }
  })

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

  // 计数用「·」串成一行时，宽度不够会在任意位置折断，数字和标签被拆散。
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
