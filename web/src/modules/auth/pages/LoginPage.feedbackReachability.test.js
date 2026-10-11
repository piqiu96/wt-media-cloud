import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const page = readFileSync(new URL('./LoginPage.vue', import.meta.url), 'utf8')

/** 取出 watcher 整体；`\n})` 只匹配列首收尾，吃不到缩进的 `scrollIntoView(…)` 闭括号。 */
function watcherBlock(text) {
  const match = text.match(/watch\(\[error, replaceNeeded\],[\s\S]*?\n\}\)/)
  if (!match) throw new Error('找不到 watcher：watch([error, replaceNeeded] …')
  return match[0]
}

/**
 * 桌面端登录卡是绝对定位的视觉稿布局：错误/替换提示一出现，卡片向下长高、提交按钮
 * 被推出视口，而唯一的滚动通道挂在 `:has()` 上，用户发现不了——这就是线上「点了登录
 * 之后卡住」的完整成因。所以提示出现时必须由程序把提交按钮带回可视区；`nearest` 保证
 * 按钮本来可见时不发生任何滚动。
 */
describe('login feedback keeps the submit button reachable', () => {
  it('watches both feedback states and scrolls the primary button into view', () => {
    const watcher = watcherBlock(page)
    expect(watcher).toContain('nextTick')
    expect(watcher).toContain("scrollIntoView({ block: 'nearest' })")
  })

  it('scrolls the submit button itself, not the card or the shell', () => {
    expect(page).toContain('ref="primaryButton"')
    expect(watcherBlock(page)).toContain('primaryButton.value?.scrollIntoView')
  })
})
