// 操作列宽度按这一页（或这一角色）实际会出现的按钮清单推导，不再逐页写死。
// 尺寸全部来自 tdesign token：按钮 size=s 的字号 12px、左右 padding 各 7px、边框 2px；
// t-space gap 8px；size=s 表格单元格左右 padding 各 8px。中文一个字 ≈ 一个字号宽。
const BUTTON_TEXT_PX = 12
const BUTTON_CHROME_PX = 16
const BUTTON_GAP_PX = 8
const CELL_CHROME_PX = 16
// loading 态按钮前面多一个 spinner。
const LOADING_CHROME_PX = 18
const MIN_WIDTH = 80

export function opColumnWidth(labels, { loading = false } = {}) {
  const buttons = [...labels].map((label) => [...String(label)].length * BUTTON_TEXT_PX + BUTTON_CHROME_PX)
  const content = buttons.reduce((sum, width) => sum + width, 0) + Math.max(labels.length - 1, 0) * BUTTON_GAP_PX
  return Math.max(content + CELL_CHROME_PX + (loading ? LOADING_CHROME_PX : 0), MIN_WIDTH)
}
