import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

// 走查七轮（2026-09-30）：用户带提示词指出筛选区「不要只靠 placeholder 表达字段含义」——
// 运营选完值以后就记不得这一格在筛什么了。同一批还给了一条项目级的约定：所有 FilterPanel
// 都用「字段标题 + 控件」，标题在控件**左侧**（用户在同轮的两个版式里选了左侧）。
//
// 这件事单靠改页面是留不住的：下一个新页面照样会写出 `<t-input placeholder="搜索名称" />`，
// 而它看上去没有任何问题。所以把约定钉成一条站级测试，扫所有含 `filter-row` 的页面。
//
// 扫描的是源码结构，不是渲染结果：`.filter-field` 这个类名就是「标题 + 控件」的落点，
// 页面上它的写法是 `<label class="filter-field"><span>标题</span><t-input … /></label>`。

const SRC = new URL('./', import.meta.url).pathname

function vueFiles(dir) {
  const out = []
  for (const name of readdirSync(dir)) {
    const full = join(dir, name)
    if (statSync(full).isDirectory()) out.push(...vueFiles(full))
    else if (name.endsWith('.vue')) out.push(full)
  }
  return out
}

/** 开标签可能跨行，属性值里也可能有 `>`。按引号配对找真正的收尾。 */
function openingTag(text, start) {
  let quote = null
  for (let i = start; i < text.length; i += 1) {
    const ch = text[i]
    if (quote) {
      if (ch === quote) quote = null
    } else if (ch === '"' || ch === "'" || ch === '`') {
      quote = ch
    } else if (ch === '>') {
      return text.slice(start, i + 1)
    }
  }
  return text.slice(start)
}

/** 从 `class="filter-row"` 所在的那个 div 取到与它配对的 `</div>`。 */
function filterRowBlock(text, marker) {
  const at = text.indexOf(marker)
  const tagStart = text.lastIndexOf('<div', at)
  const tag = openingTag(text, tagStart)
  const body = tagStart + tag.length
  let depth = 1
  let i = body
  while (i < text.length && depth > 0) {
    const open = text.indexOf('<div', i)
    const close = text.indexOf('</div>', i)
    if (close === -1) return text.slice(body)
    if (open !== -1 && open < close) {
      depth += 1
      i = open + 4
    } else {
      depth -= 1
      i = close + 6
    }
  }
  return text.slice(body, i)
}

/** 一个筛选控件的标题：同一行里最近的那个 `<label class="filter-field">` 的 `<span>`。 */
function titleOf(block, controlAt) {
  const labelAt = block.lastIndexOf('<label', controlAt)
  if (labelAt === -1) return null
  if (block.slice(labelAt, controlAt).includes('</label>')) return null
  const labelTag = openingTag(block, labelAt)
  if (!labelTag.includes('filter-field')) return null
  const span = /<span\b[^>]*>([^<]+)<\/span>/.exec(block.slice(labelAt, controlAt))
  return span ? span[1].trim() : null
}

export function auditFilterRows() {
  const rows = []
  for (const file of vueFiles(SRC)) {
    const text = readFileSync(file, 'utf8')
    let from = 0
    for (;;) {
      const at = text.indexOf('class="filter-row"', from)
      if (at === -1) break
      from = at + 1
      const block = filterRowBlock(text, 'class="filter-row"')
      const controls = []
      const controlRe = /<t-(input|select)\b/g
      let control
      while ((control = controlRe.exec(block)) !== null) {
        const tag = openingTag(block, control.index)
        const placeholder = /\bplaceholder="([^"]*)"/.exec(tag)
        controls.push({
          kind: control[1],
          title: titleOf(block, control.index),
          placeholder: placeholder ? placeholder[1] : null,
        })
      }
      const buttons = []
      const buttonRe = /<t-button\b/g
      let button
      while ((button = buttonRe.exec(block)) !== null) {
        const tag = openingTag(block, button.index)
        const label = /<t-button\b[^>]*>\s*([^<\s][^<]*?)\s*<\/t-button>/.exec(
          block.slice(button.index, button.index + tag.length + 40)
        )
        if (label) buttons.push({ text: label[1], tag })
      }
      rows.push({
        file: file.replace(SRC, ''),
        line: text.slice(0, at).split('\n').length,
        controls,
        buttons,
      })
    }
  }
  return rows
}

const rows = auditFilterRows()

/** 一个页面可能有多条筛选行，拼起来一起看。 */
function perPage(rows) {
  const map = new Map()
  for (const row of rows) {
    const entry = map.get(row.file) ?? { controls: [], buttons: [] }
    entry.controls.push(...row.controls)
    entry.buttons.push(...row.buttons)
    map.set(row.file, entry)
  }
  return map
}

describe('filter panel convention', () => {
  // 分母先钉住：扫不到筛选行的扫描器会让下面每条断言在空集上静默通过。
  // 实测 2026-09-30 全站 7 页（素材库、我的素材、内容池 crawl-tasks / 挖掘策略、
  // 用户管理、运营分组、游戏管理）、19 个控件（8 输入框 + 11 Select）。
  // 两个下限都取实测值而不是一个好看的整数：写成 15 的话，少掉一整页的筛选行照样是绿的。
  it('scans every filter row in the app', () => {
    expect(rows.length, '扫描器没找到筛选行，这条约定就是空转').toBeGreaterThanOrEqual(7)
    expect(
      rows.reduce((sum, row) => sum + row.controls.length, 0),
      '筛选控件数',
    ).toBeGreaterThanOrEqual(19)
  })

  it('gives every filter control a visible field title', () => {
    const missing = []
    for (const row of rows) {
      for (const control of row.controls) {
        if (!control.title) {
          missing.push(`${row.file}:${row.line} ${control.kind}(placeholder=${control.placeholder ?? '无'})`)
        }
      }
    }
    expect(
      missing,
      '筛选控件必须有一个可见的字段标题——只靠 placeholder，选完值就不知道这一格在筛什么了',
    ).toEqual([])
  })

  it('shows 全部 as the default an empty select falls back to', () => {
    const offenders = []
    for (const row of rows) {
      for (const control of row.controls) {
        if (control.kind !== 'select') continue
        if (control.placeholder !== '全部') {
          offenders.push(
            `${row.file}:${row.line} 「${control.title ?? '无标题'}」的 placeholder 是 ${control.placeholder ?? '无'}，不是「全部」`,
          )
        }
      }
    }
    expect(offenders, 'Select 空值的默认展示应该是「全部」——拿字段名当 placeholder，选完就只剩下值了').toEqual([])
  })

  it('pairs 查询 with 重置, primary first', () => {
    const offenders = []
    for (const [file, entry] of perPage(rows)) {
      const query = entry.buttons.filter((b) => b.text === '查询')
      const reset = entry.buttons.filter((b) => b.text === '重置')
      if (query.length !== 1) offenders.push(`${file}：查询按钮 ${query.length} 个`)
      if (reset.length !== 1) offenders.push(`${file}：重置按钮 ${reset.length} 个`)
      if (query.length === 1 && !/theme="primary"/.test(query[0].tag)) {
        offenders.push(`${file}：查询不是 Primary`)
      }
      if (reset.length === 1 && /theme="primary"/.test(reset[0].tag)) {
        offenders.push(`${file}：重置不该是 Primary（规范 §4.3）`)
      }
    }
    expect(offenders, '规范 §4.3：筛选区统一使用「查询」「重置」，且同一区域最多一个 Primary').toEqual([])
  })
})
