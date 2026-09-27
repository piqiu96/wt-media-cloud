// 列表页那一套约定的共用量具与共用的判据。
//
// 它们原先长在 `modules/contentpool/pages/ListPageConventions.test.js` 里，而那里的 `read()`
// 是**目录相对**的（`new URL('./' + name, import.meta.url)`）—— 于是它只能量自己那个目录里的
// 页面。新目录（素材库、我的素材）要验同一套规则，唯一省事的做法是把 helper 抄一份，而抄一份
// 正是这些规则漂移的方式：所谓「三张列表页的规则」，当年就是三份各写各的日期格式化。
//
// 所以：**量具与判据在这里，分母留在各自的测试里**。「这个目录有哪几个页面、各几列」是那个
// 目录的事实，不是共用事实；写死在调用方才能让「抽不到列」立刻失败，而不是安静地空转。
//
// 用法（见两个目录里同名的 `ListPageConventions.test.js`）：
//
//   const read = pageReader(import.meta.url)
//   expectExplicitColumnWidths(read, [{ file: 'XxxPage.vue', name: 'columns', count: 10 }])
import { readFileSync } from 'node:fs'
import { expect } from 'vitest'

/** 目录相对的读取器。传 `import.meta.url`，读的就是调用方所在的那个目录。 */
export function pageReader(base) {
  return (name) => readFileSync(new URL(`./${name}`, base), 'utf8')
}

// 取出 `const <name> = [ ... ]` 的数组字面量内容，按方括号配平（列声明里没有嵌套数组）。
export function arrayLiteral(source, name) {
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

/** 取出 `function <name>(` 到顶格 `}` 的那一段。箭头函数不算数，见各处的用法。 */
export function fnBody(source, name) {
  const start = source.indexOf(`function ${name}(`)
  if (start === -1) return null
  const end = source.indexOf('\n}', start)
  return end === -1 ? null : source.slice(start, end)
}

/** 列声明里的每个 `{...}` 条目（列声明里没有嵌套对象）。 */
export function columnEntries(block) {
  return [...block.matchAll(/\{[^{}]*\}/g)].map((m) => m[0])
}

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
export function expectExplicitColumnWidths(read, tables) {
  for (const { file, name, count } of tables) {
    const block = arrayLiteral(read(file), name)
    expect(block, `${file} 里找不到 ${name}`).not.toBeNull()

    const entries = columnEntries(block)
    // 分母可见：列数对不上说明正则没抽对，立刻失败而不是空转。
    expect(entries, `${file} ${name}`).toHaveLength(count)

    const missing = entries.filter((entry) => !/\bwidth:\s*\d+/.test(entry))
    expect(missing, `${file} ${name} 有列没写 width`).toEqual([])
    // minWidth 写了也不生效，留着只会误导下一个人。
    expect(block, `${file} ${name}`).not.toContain('minWidth:')
  }
}

/** 横向滚动宽度必须由列宽推导，不许再出现写死的 x。 */
export function expectDerivedScrollWidth(read, files) {
  for (const file of files) {
    const source = read(file)
    expect(source, file).not.toContain(':scroll="{ x:')
    expect(source, file).toContain(':scroll="tableScroll"')
    expect(source, file).toContain('const tableScroll = computed')
  }
}

/** 时间列走同一个格式化器，且不带秒。 */
export function expectMinuteResolutionTimestamps(read, files) {
  for (const file of files) {
    const source = read(file)
    expect(source, file).toContain("from '../../../shared/utils/datetime.js'")
    // 各自实现一份日期格式化正是漂移的来源（其中两份带秒）。
    expect(source, file).not.toContain('function dateLabel(')
    expect(source, file).not.toContain('{ hour12: false }')
  }
}
