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

// Vite 的根相对 id 是相对 **web/** 这一层解析的（`/src/…`），而本文件在 `web/src/apps/`。
const projectRoot = new URL('../../', import.meta.url)
const cloudBase = new URL('./cloud/', import.meta.url)
const desktopBase = new URL('./desktop/', import.meta.url)

function loaderSource(route) {
  const loader = route.components?.default ?? route.component
  if (typeof loader !== 'function') return null
  // Vite 会把 `import("./x.vue")` 改写成 `__vite_ssr_dynamic_import__("/src/…/x.vue")`，
  // 两种拼法都接受 —— 出现哪一种由打包器决定，不是这条断言的事。取到的是**模块 id**。
  const match = /(?:\bimport|__vite_ssr_dynamic_import__)\(\s*(?:"|')(.+?)(?:"|')\s*\)/.exec(String(loader))
  return match ? match[1] : null
}

/**
 * 模块 id → 磁盘文件。
 *
 * 相对 id 要按**各自的路由目录**解析：同一份组件在 Cloud 表里写作
 * `./pages/users/UsersPage.vue`，在 Desktop 表里写作 `../cloud/pages/users/UsersPage.vue`，
 * 两者是同一个文件。比较原始 id 会把它们报成不同，比较解析后的文件才是那句话的意思。
 */
function loaderFile(route, base) {
  const id = loaderSource(route)
  if (!id) return null
  const target = id.startsWith('/') ? new URL(`.${id}`, projectRoot) : new URL(id, base)
  return fileURLToPath(target)
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
 * 同一个文件**（相对 id 按各自的路由目录解析，拼法不同不算不同）。
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
      const cloud = loaderFile(cloudByPath.get(path), cloudBase)
      const desktop = loaderFile(desktopByPath.get(path), desktopBase)
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
    const cloudLibrary = loaderFile(cloudByPath.get('material-library'), cloudBase)
    const desktopContentPool = loaderFile(desktopByPath.get('content-pool'), desktopBase)

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
describe('the material pages are real pages on both ends', () => {
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

describe('the compose task page is a real page on both ends', () => {
  const loader = 'modules/compose/pages/ComposeTasksPage.vue'
  for (const [end, table] of [['cloud', cloudByPath], ['desktop', desktopByPath]]) {
    it(`${end} compose points at ${loader}`, () => {
      const source = loaderSource(table.get('compose'))
      expect(source, end).toBeTruthy()
      expect(source, end).toContain(loader)
      expect(source, end).not.toContain('ComingSoon.vue')
    })
  }
})

// CHG079 把占位守卫钉在这里，条件是「模板还不能执行」；CHG080 用真实 FFmpeg 验证了
// `basic_cut` v1，CHG081 的策略页因此翻掉这个占位。与上面素材库两条同型：钉住**已经被
// 替换掉**，防止只改一端或哪天退回占位页。
describe('the compose strategy page is a real page on both ends', () => {
  const loader = 'modules/compose/pages/ComposeStrategyPage.vue'
  for (const [end, table] of [['cloud', cloudByPath], ['desktop', desktopByPath]]) {
    it(`${end} compose-strategy points at ${loader}`, () => {
      const source = loaderSource(table.get('compose-strategy'))
      expect(source, end).toBeTruthy()
      expect(source, end).toContain(loader)
      expect(source, end).not.toContain('ComingSoon.vue')
    })
  }
})

// CHG-20261013-084 把成片管理从占位页换成「我的成片」真实页面。与上面素材库两条
// 同型：钉住**已经被替换掉**，防止只改一端或哪天退回占位页。
describe('the my outputs page is a real page on both ends', () => {
  const loader = 'modules/compose/pages/MyOutputsPage.vue'
  for (const [end, table] of [['cloud', cloudByPath], ['desktop', desktopByPath]]) {
    it(`${end} finished-media points at ${loader}`, () => {
      const source = loaderSource(table.get('finished-media'))
      expect(source, end).toBeTruthy()
      expect(source, end).toContain(loader)
      expect(source, end).not.toContain('ComingSoon.vue')
    })
  }
})

/**
 * 两端的路由表边界。原先这两条只是注释（「不包含 /logs, /settings」「不包含 /users」）。
 * 一个地址在错的树上存在，不会报错：它会渲染出一个那个端根本进不到的服务器的页面。
 */
describe('the two ends keep their route tables straight', () => {
  it('keeps the desktop-only pages out of the cloud table', () => {
    for (const path of ['logs', 'settings']) {
      expect(cloudByPath.has(path), path).toBe(false)
      expect(desktopByPath.has(path), path).toBe(true)
    }
  })

  // 系统管理页原先钉的是「桌面端不得出现这三个地址」。2026-10-27 起桌面端对管理员与
  // 高级运营开放系统管理，规则换了个方向：地址两端都有，但必须落在同一个组件文件上，
  // 不许各写一套。角色可见性由 `shared/utils/roles.js` 与两端 main.ts 的守卫决定，
  // 不靠路由表缺项来隐藏。
  it('serves the system management pages from one file on both ends', () => {
    for (const path of ['users', 'operation-teams', 'games']) {
      expect(cloudByPath.has(path), path).toBe(true)
      expect(desktopByPath.has(path), path).toBe(true)
      const cloud = loaderFile(cloudByPath.get(path), cloudBase)
      const desktop = loaderFile(desktopByPath.get(path), desktopBase)
      expect(cloud, `cloud ${path}`).toContain(`/cloud/pages/users/`)
      expect(desktop, `desktop ${path}`).toBe(cloud)
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
