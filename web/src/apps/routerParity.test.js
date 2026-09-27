import { existsSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

import { cloudRoutes } from './cloud/router.ts'
import { desktopRoutes } from './desktop/router.ts'

// 两张路由表都是导出的（不是在 `create*Router` 里现搭的），正是为了这里能导入它们
// 而不是 grep 那些长得像路由的字符串。导入还需要跑得起 TS 与 `.vue` 加载器 ——
// 可以：加载器只是 `() => import(...)` 这个函数本身，断言的是它的**文本**，不要求值。

const childrenOf = (routes) => routes.flatMap((route) => route.children ?? [])
const cloudChildren = childrenOf(cloudRoutes)
const desktopChildren = childrenOf(desktopRoutes)

function loaderSource(route) {
  const loader = route.components?.default ?? route.component
  if (typeof loader !== 'function') return null
  // Vite 会把 `import("./x.vue")` 改写成 `__vite_ssr_dynamic_import__("/src/…/x.vue")`，
  // 两种拼法都接受 —— 出现哪一种由打包器决定，不是这条断言的事。取到的是**模块 id**。
  const match = /(?:\bimport|__vite_ssr_dynamic_import__)\(\s*(?:"|')(.+?)(?:"|')\s*\)/.exec(String(loader))
  return match ? match[1] : null
}

function byPath(children) {
  return new Map(children.map((route) => [route.path, route]))
}

const cloudByPath = byPath(cloudChildren)
const desktopByPath = byPath(desktopChildren)

/**
 * 「Cloud Web 与 Desktop WebView 复用同一批页面」这句话，此前只写在两句注释里。
 *
 * 注释说的是一件事，而它可以是假的：两条路由各改各的，把同一个地址指向两个不同的
 * 组件文件，页面外表照常工作，两端的运营看到的却是两个不同的东西 —— 没有任何构建、
 * 测试或日志会提这件事。这条断言把那句话变成可执行的：**同一个 path，两端解析到
 * 同一个模块 id**。
 */
describe('cloud and desktop serve the same pages', () => {
  const sharedPaths = [...cloudByPath.keys()].filter((path) => desktopByPath.has(path))

  it('has shared paths to compare at all', () => {
    // 分母。空集会让下面每一条断言都空转通过。
    expect(sharedPaths.length).toBeGreaterThan(15)
    // 本 CHG 动过的两条必须在里面，否则「比过了」比的是别的东西。
    expect(sharedPaths).toContain('material-library')
    expect(sharedPaths).toContain('my-material')
  })

  it('resolves every shared path to the same component file on both ends', () => {
    const checked = []
    const differing = []
    for (const path of sharedPaths) {
      const cloud = loaderSource(cloudByPath.get(path))
      const desktop = loaderSource(desktopByPath.get(path))
      if (cloud === null || desktop === null) continue
      checked.push(path)
      if (cloud !== desktop) differing.push(`${path}: cloud ${cloud} / desktop ${desktop}`)
    }

    // 与上一条同一个理由：循环必须真的比过东西，否则读数是「0 处不同」而分母是 0。
    expect(checked.length).toBeGreaterThan(15)
    expect(differing).toEqual([])
  })

  // 阳性对照：这套比较**能**看出不同。拿一对已知不同的（都是真实存在的页面，
  // 且不在共享集里）验一次，否则「0 处不同」也可能只是比较器对谁都报相等。
  it('would notice two paths that resolve to different files', () => {
    const cloudLibrary = loaderSource(cloudByPath.get('material-library'))
    const desktopContentPool = loaderSource(desktopByPath.get('content-pool'))

    expect(cloudLibrary).toBeTruthy()
    expect(desktopContentPool).toBeTruthy()
    expect(cloudLibrary).not.toBe(desktopContentPool)
  })
})

/**
 * 素材库与我的素材从「占位页／内容池投影」换成了真实页面。
 *
 * 这条钉的是**已经被替换掉**：旧的 `/material-library` 指向 `ContentPoolPage.vue`
 * （页面内用 `route.path` 判断自己该渲染成素材库），`/my-material` 指向
 * `ComingSoon.vue`。改动若只落在一端，另一端会安静地继续显示旧东西。
 */
describe('the two material pages are real pages on both ends', () => {
  for (const [name, loader] of [
    ['material-library', 'modules/materials/pages/MaterialLibraryPage.vue'],
    ['my-material', 'modules/materials/pages/MyMaterialsPage.vue'],
  ]) {
    it(`${name} points at ${loader} in both tables`, () => {
      for (const [end, table] of [['cloud', cloudByPath], ['desktop', desktopByPath]]) {
        const source = loaderSource(table.get(name))
        expect(source, `${end} ${name}`).toBeTruthy()
        expect(source, `${end} ${name}`).toContain(loader)
        expect(source, `${end} ${name}`).not.toContain('ComingSoon.vue')
        expect(source, `${end} ${name}`).not.toContain('ContentPoolPage.vue')
      }
    })
  }
})

/**
 * 两端各自不该有的页面。这两条原先也只是注释（「不包含 /agent, /logs」「不包含 /users」）。
 * 一个地址在错的树上存在，不会报错：它会渲染出一个那个端根本进不到的服务器的页面，
 * 或者一个运营无从管理的管理页。
 */
describe('the two ends keep their own exclusions', () => {
  it('keeps the desktop-only pages out of the cloud table', () => {
    for (const path of ['agent', 'logs', 'settings']) {
      expect(cloudByPath.has(path), path).toBe(false)
      expect(desktopByPath.has(path), path).toBe(true)
    }
  })

  it('keeps the cloud admin pages out of the desktop table', () => {
    for (const path of ['users', 'operation-teams', 'games']) {
      expect(desktopByPath.has(path), path).toBe(false)
      expect(cloudByPath.has(path), path).toBe(true)
    }
  })
})

/** 每一条声明的路由都必须落在一个真的存在的文件上（两张表一起走）。 */
describe('every declared component exists', () => {
  it('names a file on disk for every route in both tables', () => {
    // Vite 的 id 是相对 **web/** 这一层解析的（`/src/…`），而本文件在 `web/src/apps/`，
    // 所以是两级向上，不是一级。
    const projectRoot = new URL('../../', import.meta.url)
    const ends = [
      ['cloud', cloudChildren, new URL('./cloud/', import.meta.url)],
      ['desktop', desktopChildren, new URL('./desktop/', import.meta.url)],
    ]

    let checked = 0
    for (const [end, children, base] of ends) {
      for (const route of children) {
        const id = loaderSource(route)
        if (!id) continue
        const target = id.startsWith('/')
          ? new URL(`.${id}`, projectRoot)
          : new URL(id, base)
        const resolved = ['', '.ts', '.js', '.vue'].some((extension) =>
          existsSync(`${fileURLToPath(target)}${extension}`)
        )
        expect(resolved, `${end} ${route.path} → ${id} 不存在`).toBe(true)
        checked += 1
      }
    }

    // 同上的分母守卫：正则改瞎之后这条会连同上面一起红，而不是安静地什么都不查。
    expect(checked).toBeGreaterThan(30)
  })
})
