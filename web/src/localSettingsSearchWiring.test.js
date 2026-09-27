import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(
  new URL('./apps/desktop/features/local-settings/LocalSettingsPage.vue', import.meta.url),
  'utf8'
)

/** One function's body, so an assertion is about that handler and not the file. */
function bodyOf(name) {
  const start = source.indexOf(`async function ${name}()`)
  expect(start, `no handler named ${name}`).toBeGreaterThan(-1)
  const body = source.slice(start)
  return body.slice(0, body.indexOf('\n}'))
}

/**
 * The 「添加查找位置」 entry, and the one thing it must not do.
 *
 * The rules live in `local-settings-view.js` and are tested there; the service
 * call is tested in `localSettingsService.test.js`. What is left, and what
 * nothing else can check, is that the page connects them — a rule nobody reads
 * and a command nobody calls both look exactly like a working page from the
 * outside.
 *
 * The load-bearing assertion here is the **negative** one. This entry exists
 * because a directory can be searched without being where new files go, and the
 * one plausible wrong implementation is to reuse the save-location handler:
 * storing the picked directory through `setSaveDir` would make 「这个目录也作为查找
 * 位置」 quietly mean 「以后的下载也放这儿」, moving where material lands for anyone
 * who pressed it. That mistake is invisible in the handler's own success path —
 * the button would light up and the outcome would read plausibly — so it is
 * asserted by what the body does *not* call.
 */
describe('the 添加查找位置 entry is wired to the picker', () => {
  it('is the only one of the two pickers on the page', () => {
    expect(source).toContain('@click="addSearchDirectory"')
    expect(source).toContain('添加查找位置')
    // The save-location picker belongs to the Agent-facing flow; this page writes
    // the draft through an input and a 保存 button, and there must not be a second
    // button that also picks a save directory.
    expect(source).not.toContain('local_pick_save_directory')
  })

  it('calls the picker and renders the rule module\'s answer', () => {
    const body = bodyOf('addSearchDirectory')

    expect(body).toContain('service.pickSearchDirectory()')
    expect(body).toContain('describeSearchDirectory(answer)')
    expect(body).toContain('outcome.value = describeSearchDirectory(answer)')
  })

  /**
   * A cancelled dialog renders **nothing**, and a failure still renders its own
   * banner.
   *
   * `run` answers `null` for both, so the two are told apart by what came before
   * them: a failure has set `failure`, which is the page's error alert, and a
   * cancel has set nothing. Returning before `outcome` is assigned is what keeps
   * a cancelled picker from printing a sentence about a directory nobody chose.
   */
  it('returns before rendering anything when the dialog was cancelled', () => {
    const body = bodyOf('addSearchDirectory')

    const guard = body.indexOf('if (!answer)')
    const render = body.indexOf('outcome.value =')

    expect(guard).toBeGreaterThan(-1)
    expect(guard).toBeLessThan(render)
    expect(body.slice(guard, render)).toContain('return')
  })

  /**
   * And it does not touch the save location — checked with its positive control.
   *
   * The control is the other handler on the same card, which *does* store the
   * choice: if the slice above had grabbed the wrong function, or the pattern had
   * stopped matching, `saveSaveDir` would fail the same assertion that
   * `addSearchDirectory` passes. Both are asserted in the one test so that a
   * slice that emptied out cannot pass quietly.
   */
  it('does not store or push a save location', () => {
    const picked = bodyOf('addSearchDirectory')
    const stored = bodyOf('saveSaveDir')

    expect(picked).not.toContain('setSaveDir')
    expect(picked).not.toContain('pushSaveDir')
    // The positive control: the same two patterns, in the handler that must hit
    // them. A regex or a slice that matched nothing would take this down too.
    expect(stored).toContain('setSaveDir')
    expect(stored).toContain('pushSaveDir()')
  })

  /**
   * The page says out loud that searching is not saving.
   *
   * Two buttons on one card can both be read as 「换个目录」, and the hint above the
   * 搬运 button already says 「换位置不会自动搬运」 — this is the other half of that
   * pair, and it is the sentence that keeps the new entry from being understood as
   * a second way to change where downloads go.
   */
  it('says the new downloads keep going to the saved location', () => {
    expect(source).toContain('以后的下载仍然保存到上面的位置')
  })
})
