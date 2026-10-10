// 官网页 Scroll Spy 判定:由各 section 文档坐标与视口几何推导当前导航激活项。
// HomePage 只负责收集 rect,判定集中在此,便于纯函数测试。
export const HOME_NAV_SECTIONS = [
  { id: 'hero', nav: 'product' },
  { id: 'product', nav: 'product' },
  { id: 'download', nav: 'download' },
  { id: 'pricing', nav: 'pricing' },
  { id: 'contact', nav: 'contact' },
]

// 近底区间:进入后判定线上移到视口 67%,避免矮 section(定价/联系)永远够不到判定线。
export const BOTTOM_ZONE = 180

// 距底强制「联系」线。必须 < BOTTOM_ZONE:点击「定价」的 clamp 落点是 maxScroll -
// BOTTOM_ZONE,若落点落入强制圈,刚点完定价就会高亮联系。
export const BOTTOM_EPSILON = 60

const navById = new Map(HOME_NAV_SECTIONS.map((section) => [section.id, section.nav]))

export function resolveActiveNav({ scrollY, viewportHeight, scrollHeight, sectionTops }) {
  const maxScroll = Math.max(0, scrollHeight - viewportHeight)
  const marker = scrollY >= maxScroll - BOTTOM_ZONE
    ? viewportHeight * 0.67
    : Math.min(viewportHeight * 0.38, 360)
  let current = 'product'
  for (const { id, top } of sectionTops) {
    const nav = navById.get(id)
    if (nav && nav !== 'product' && top - scrollY <= marker) current = nav
  }
  if (scrollY + viewportHeight >= scrollHeight - BOTTOM_EPSILON) current = 'contact'
  return current
}
