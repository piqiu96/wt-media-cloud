import { existsSync, readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

import { desktopRoutes } from './apps/desktop/router.ts'

function read(relativePath) {
  return readFileSync(new URL(relativePath, import.meta.url), 'utf8')
}

const children = desktopRoutes.flatMap((route) => route.children ?? [])
const settings = children.find((route) => route.name === 'LocalSettings')

/**
 * The 本机设置 page is reachable, in all three places that have to agree.
 *
 * A page is not wired by existing: the route has to be declared, its **name** has
 * to be in `main.ts`'s no-auth allowlist, and the navigation entry has to point
 * at the route's path. Getting two of the three right produces a page that
 * redirects to the login screen, or one that cannot be reached at all — and
 * neither failure says which of the three is missing, which is why this asserts
 * the agreement rather than the presence of any one of them.
 *
 * The route table is imported rather than grepped, so what is asserted is the
 * object the app navigates by rather than a string that resembles it. That works
 * without a DOM because the table is exported separately from
 * `createDesktopRouter`, which needs a browser history.
 */
describe('the 本机设置 route', () => {
  it('is declared with the name the allowlist uses', () => {
    expect(settings, 'no route named LocalSettings').toBeDefined()
    // Relative under the layout route, which is what makes the app's address
    // `/settings` rather than `/settings/`.
    expect(settings.path).toBe('settings')
  })

  it('exists in the no-auth allowlist, by name', () => {
    const main = read('./apps/desktop/main.ts')

    // The same name, in the guard. A route that is not in this list sends a
    // person to the login screen, which looks like a broken page rather than a
    // missing entry.
    expect(main).toMatch(/to\.name === "LocalSettings"/)
  })

  it('is reachable from the sidebar, at the path the router serves', () => {
    const layout = read('./layout/AppLayout.vue')

    expect(layout).toMatch(/path: '\/settings'/)
    // The sidebar's path is absolute and the router's is relative under the
    // layout route; the two spellings have to name one address, so the absolute
    // form is the router's with a leading slash.
    expect(layout).toContain(`path: '/${settings.path}'`)
  })

  it('is listed under 桌面环境, after the two pages that were already there', () => {
    const layout = read('./layout/AppLayout.vue')
    const desktopItems = layout.slice(layout.indexOf('const desktopItems'))

    expect(desktopItems.indexOf("path: '/agent'")).toBeGreaterThan(-1)
    expect(desktopItems.indexOf("path: '/logs'")).toBeGreaterThan(-1)
    expect(desktopItems.indexOf("path: '/settings'")).toBeGreaterThan(
      desktopItems.indexOf("path: '/logs'")
    )
    // Still inside the one desktop group rather than in a group of its own.
    expect(desktopItems.indexOf("path: '/settings'")).toBeLessThan(
      desktopItems.indexOf('adminOnlyPaths')
    )
  })

  /**
   * Every component the desktop router names resolves to a file on disk.
   *
   * A dynamic `import()` of a path that does not exist is a build error in the
   * packaged app and a runtime one in dev, and both messages name a module id
   * rather than the page. Walking the declared table covers all of them, not just
   * this change's row.
   *
   * **What is read out of the loader is Vite's own id, not the source path.**
   * The runner rewrites `import("./x.vue")` into
   * `__vite_ssr_dynamic_import__("/src/apps/…/x.vue")` — already resolved
   * against the project root — so the id here is the one Vite produced, and the
   * assertion is that a file is at that place. Both spellings are accepted
   * because which one appears is the bundler's business, not this test's; the id
   * is the part that means something either way.
   */
  it('names a component file that exists, for every desktop route', () => {
    const loaders = children
      .map((route) => route.components?.default ?? route.component)
      .filter((loader) => typeof loader === 'function')

    // Exact rather than a floor: a floor catches only a table that has been emptied,
    // not one that lost a route, and the walk below would then cover one page fewer
    // without saying so.
    expect(loaders.length, 'desktop routes carrying a loader (measured 20)').toBe(20)

    const projectRoot = new URL('../', import.meta.url)
    let checked = 0
    for (const loader of loaders) {
      const match = /(?:\bimport|__vite_ssr_dynamic_import__)\(\s*(?:"|')(.+?)(?:"|')\s*\)/.exec(
        loader.toString()
      )
      if (!match) continue
      const id = match[1]
      // A root-absolute id is Vite's, resolved against the project root; a
      // relative one is the source's, resolved against the router's directory.
      const target = id.startsWith('/')
        ? new URL(`.${id}`, projectRoot)
        : new URL(id, new URL('./apps/desktop/', import.meta.url))
      const resolved = ['', '.ts', '.js', '.vue'].some((extension) =>
        existsSync(`${fileURLToPath(target)}${extension}`)
      )
      expect(resolved, `${id} does not exist`).toBe(true)
      checked += 1
    }
    // The loop has to have looked at something: a bundler that stopped putting a
    // readable id in the loader would make every assertion above vacuous, and
    // this is the line that says so rather than passing quietly. The first draft
    // of this test had a regex that matched nothing and passed for the wrong
    // reason, which is what this guard was for.
    expect(checked).toBe(loaders.length)
  })

  /** The positive control for the check above: a path that is not there fails it. */
  it('would notice a component path that is not there', () => {
    const base = new URL('./apps/desktop/features/local-settings/', import.meta.url)

    expect(existsSync(fileURLToPath(new URL('NoSuchPage.vue', base)))).toBe(false)
    // And the page this change added is the one that is there, so the two
    // answers differ for the right reason.
    expect(existsSync(fileURLToPath(new URL('LocalSettingsPage.vue', base)))).toBe(true)
  })
})

/**
 * Every name a desktop page imports from a sibling module is really exported.
 *
 * **This exists because the build caught something the suite could not.** The
 * rewritten viewer imported `logKindLabel` from `local-logs-view.js`, where it is
 * not defined — it lives in `local-settings-view.js`. All 164 tests passed and
 * `npm run build:desktop` failed, because the unit suite never compiles a `.vue`
 * file: it imports the plain-JS view modules directly, which is the whole reason
 * the page logic lives there. A misspelled or misplaced specifier is therefore a
 * build error and nothing else, and a build is too slow to be the only thing
 * looking.
 *
 * The check is narrow on purpose: relative specifiers out of these pages, and
 * the module's real export list — no attempt to resolve bare specifiers or to
 * type-check anything, both of which the bundler does better.
 */
describe('desktop page imports', () => {
  const pages = [
    './apps/desktop/features/local-settings/LocalSettingsPage.vue',
    './apps/desktop/features/local-logs/LocalLogsPage.vue',
    './apps/desktop/features/local-agent/AgentStatusPage.vue',
  ]

  /** The script block, so a name appearing in a template does not count. */
  function scriptOf(source) {
    const match = /<script[^>]*>([\s\S]*?)<\/script>/.exec(source)
    return match ? match[1] : ''
  }

  function relativeImports(script) {
    const found = []
    const pattern = /import\s*\{([^}]+)\}\s*from\s*(?:"|')(\.[^"']+)(?:"|')/g
    let match
    while ((match = pattern.exec(script)) !== null) {
      const names = match[1]
        .split(',')
        .map((name) => name.trim().split(/\s+as\s+/)[0].trim())
        .filter(Boolean)
      found.push({ names, specifier: match[2] })
    }
    return found
  }

  it('finds imports to check, in every page', () => {
    const totals = pages.map((path) => relativeImports(scriptOf(read(path))).length)

    // A regex that stopped matching would make every assertion below vacuous.
    for (const [index, total] of totals.entries()) {
      expect(total, pages[index]).toBeGreaterThan(0)
    }
  })

  it('imports only names the target module actually exports', async () => {
    const checked = []

    for (const path of pages) {
      const pageUrl = new URL(path, import.meta.url)
      for (const { names, specifier } of relativeImports(scriptOf(read(path)))) {
        const moduleUrl = new URL(specifier, pageUrl)
        if (moduleUrl.pathname.endsWith('.vue')) continue
        const moduleExports = await import(/* @vite-ignore */ moduleUrl.href)
        for (const name of names) {
          expect(
            Object.keys(moduleExports),
            `${specifier} does not export ${name} (imported by ${path})`
          ).toContain(name)
          checked.push(`${path} -> ${specifier}#${name}`)
        }
      }
    }

    // The same guard, one level down: the loop has to have checked something.
    expect(checked.length).toBeGreaterThan(0)
  })
})
