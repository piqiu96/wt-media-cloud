import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

// 走查五轮（2026-09-30）：素材详情的底部冒出一对「取消 / 确认」，用户看得到，但
// **任何源码断言都看不见**——它不是本仓写的，是 TDesign Drawer 的 footer 默认值
// （`props.footer` 默认 `true`，不给插槽就渲染 `getDefaultFooter()` 的 cancel/confirm）。
// 素材抽屉自己那条 `expect(template).not.toContain('>取消</t-button>')` 因此一直是绿的。
//
// 事后盘点全站 12 个抽屉，10 个显式关掉了或给了自己的页脚，2 个漏了。漏一个就长一对
// 取消/确认，而这类缺陷只会在人打开抽屉时暴露。所以把「每个抽屉都必须对页脚表态」
// 钉成一条站级约定：给 `#footer` 插槽，或显式写 `footer` 属性。
//
// 只管 `t-drawer`。`t-dialog` 有同样默认值，但确认对话框要的**正是**那对取消/确认，
// 一并扫会造出一堆假阳性。

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

/** 开标签可能跨行，属性值里也可能有 `>`（模板串、箭头函数）。按引号配对找真正的收尾。 */
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

/** 取出 `#footer` 插槽本身。页脚里会有嵌套的 `<template v-if>`，按第一个
    `</template>` 切会把动作条从第一个分支处剪断，所以按标签配对找收尾。 */
function footerSlot(block) {
  const at = block.search(/<template\s+#footer\b/)
  if (at === -1) return null
  let i = at + openingTag(block, at).length
  let depth = 1
  while (depth > 0) {
    const nextOpen = block.indexOf('<template', i)
    const nextClose = block.indexOf('</template>', i)
    if (nextClose === -1) return block.slice(at)
    if (nextOpen !== -1 && nextOpen < nextClose) {
      depth += 1
      i = nextOpen + '<template'.length
    } else {
      depth -= 1
      i = nextClose + '</template>'.length
    }
  }
  return block.slice(at, i)
}

/** 每个抽屉说清：它给自己的页脚，还是显式关掉；都没有就是漏了。 */
export function auditDrawers() {
  const found = []
  for (const file of vueFiles(SRC)) {
    const text = readFileSync(file, 'utf8')
    let from = 0
    for (;;) {
      const at = text.indexOf('<t-drawer', from)
      if (at === -1) break
      from = at + 9
      const tag = openingTag(text, at)
      const end = text.indexOf('</t-drawer>', at)
      const block = text.slice(at + tag.length, end === -1 ? text.length : end)
      found.push({
        file: file.replace(SRC, ''),
        line: text.slice(0, at).split('\n').length,
        tag,
        hasFooterProp: /\bfooter\s*=/.test(tag),
        hasFooterSlot: /<template\s+#footer\b/.test(block),
        footerText: footerSlot(block),
      })
    }
  }
  return found
}

describe('drawer footer convention', () => {
  const drawers = auditDrawers()

  // 分母先钉住：扫不到抽屉的扫描器会让下面那条断言在空集上静默通过。
  // 实测 2026-09-30 全站 12 个（materials 1、contentpool 3、profiles 2、proxy 1、
  // accounts 1、transfer 1、users 1、shared 模板 1）。
  it('scans every drawer in the app', () => {
    expect(drawers.length, '扫描器没找到抽屉，这条约定就是空转').toBeGreaterThanOrEqual(12)
    expect(new Set(drawers.map((d) => d.file)).size).toBeGreaterThanOrEqual(8)
  })

  it('never leaves a drawer on the framework default footer pair', () => {
    const missing = drawers
      .filter((d) => !d.hasFooterProp && !d.hasFooterSlot)
      .map((d) => `${d.file}:${d.line}`)
    expect(
      missing,
      'TDesign 的 drawer footer 默认是 true，不表态就会渲染「取消 / 确认」',
    ).toEqual([])
  })

  // 2026-09-30 用户裁定：关闭一律走抽屉右上角的 ×，页脚只放业务动作。素材详情与本机
  // 扫描结果两个抽屉原本各有一颗「关闭」占着页脚靠左的位置，撤掉之后关闭入口只剩一个，
  // 也就不会再出现「同一个抽屉，右上角一个 ×、左下角一个关闭」。
  it('keeps every drawer footer on business actions, never a 关闭 button', () => {
    const withFooter = drawers.filter((d) => d.footerText)
    // 分母跟另一条读数对齐：`hasFooterSlot` 是不经切片的正则。切片器坏掉时（返回 null，
    // 或者返回一段垃圾串）两个数就对不上，而不会安静地变成「没有抽屉带页脚」。
    expect(
      withFooter.length,
      '切片器给出的页脚数与正则数对不上，这条约定就是空转',
    ).toBe(drawers.filter((d) => d.hasFooterSlot).length)
    expect(withFooter.length, '一个带页脚的抽屉都没扫到').toBeGreaterThanOrEqual(3)
    const offenders = withFooter
      .filter((d) => />\s*关闭\s*</.test(d.footerText))
      .map((d) => `${d.file}:${d.line}`)
    expect(offenders, '关闭请走抽屉右上角的 ×，页脚只留给业务动作').toEqual([])
  })

  // 页脚那颗「关闭」撤了之后，标题栏右上角的 × 就是唯一的明确关闭入口，而它**不会
  // 自己出现**：本仓装的 tdesign-vue-next 1.20.3 里 `closeBtn` 没有 default
  // （同版本的 `dialog` 写着 `default: true`，`drawer` 没写），Vue 对声明了 Boolean
  // 又没有 default 的 prop 取值 `false`，于是渲染处那句 `props2.closeBtn && …` 为假。
  // 2026-09-30 用 headless Chrome 渲染确认过：不写 `close-btn` 的抽屉 DOM 里没有
  // `.t-drawer__close-btn`，写了 `:close-btn="true"` 才有。
  it('enables the header close button on every drawer, which does not default on', () => {
    const off = drawers
      .filter((d) => !/:close-btn\s*=\s*"true"/.test(d.tag))
      .map((d) => `${d.file}:${d.line}`)
    expect(off, '不写 :close-btn="true" 的抽屉没有关闭入口').toEqual([])
  })
})
