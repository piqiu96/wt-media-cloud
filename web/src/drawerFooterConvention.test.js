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
        hasFooterProp: /\bfooter\s*=/.test(tag),
        hasFooterSlot: /<template\s+#footer\b/.test(block),
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
})
