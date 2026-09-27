import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(
  new URL('./apps/desktop/features/local-settings/LocalSettingsPage.vue', import.meta.url),
  'utf8'
)

/**
 * The migration dialog's rules live in `local-settings-view.js` and are tested
 * there; this file checks that the page **uses** them.
 *
 * A computed rule nobody reads is invisible, and the failure is silent: the
 * dialog would render rows with no decision buttons on them, or a footer with a
 * count that never changes, and every unit test of the rule would still pass.
 * The page cannot be mounted in this suite (no DOM), so what is asserted here is
 * the text that connects the two — the same approach as
 * `DownloadCentreDrawer.test.js`.
 */
describe('the migration dialog is wired to its rules', () => {
  it('renders the rows the view module built, with a badge and a decision each', () => {
    expect(source).toContain(':data="migrationRowsFor"')
    // The state a person reads: the module's label and tone, not a second set of
    // conditions in the template.
    expect(source).toContain(':tone="row.statusTone"')
    expect(source).toContain(':label="row.statusLabel"')
    // The decision, and the guard that keeps a button off a row that has nothing
    // to decide.
    expect(source).toContain('v-if="row.selectable"')
    expect(source).toContain('@click="choose(row.name, option)"')
    expect(source).toContain(':theme="row.choice === option ? \'primary\' : \'default\'"')
    // Every choice is offered, including 保留 — the default has to be clickable
    // for a mistaken 搬运 to be taken back.
    expect(source).toContain('v-for="option in CHOICE_OPTIONS"')
    expect(source).toContain('{{ choiceLabel(option) }}')
  })

  /**
   * Two actions, two buttons, each carrying **its own** count.
   *
   * One confirm button performing whichever action the rows happened to hold
   * would make a deletion and a move the same press. The counts are what says how
   * many files are about to be touched.
   */
  it('has one button per action, each showing how many files it would touch', () => {
    expect(source).toContain('@click="runMigration(FILE_CHOICES.move)"')
    expect(source).toContain('@click="runMigration(FILE_CHOICES.delete)"')
    expect(source).toContain('{{ movableNames.length }}')
    expect(source).toContain('{{ deletableNames.length }}')
    expect(source).toContain(':disabled="!!migrationBusy || !movableNames.length"')
    expect(source).toContain(':disabled="!!migrationBusy || !deletableNames.length"')
  })

  // The names that are sent are the ones that were chosen, and the report is
  // rendered by the view module rather than written into the component.
  it('carries out the chosen action on exactly the chosen names', () => {
    expect(source).toContain('chosenNames(migrationRowsFor.value, FILE_CHOICES.move)')
    expect(source).toContain('chosenNames(migrationRowsFor.value, FILE_CHOICES.delete)')
    expect(source).toContain('service.deleteSavedFiles(names)')
    expect(source).toContain('service.moveSavedFiles(names)')
    expect(source).toContain('describeMigration(report, action)')
    expect(source).toContain('outcome.value = describeMigration(report, action)')
  })

  /**
   * The order: store, push, scan, then show.
   *
   * The dialog must not open before the location is stored — the scan is about
   * the location that is now chosen, and a later failure would leave the page
   * asking about a directory the settings file does not have. The push comes
   * before the scan because it is the same change seen from the other side.
   */
  it('stores the location, pushes it, then asks about the files', () => {
    const save = source.slice(source.indexOf('async function saveSaveDir()'))
    const body = save.slice(0, save.indexOf('\n}'))

    const set = body.indexOf('service.setSaveDir(')
    const push = body.indexOf('pushSaveDir()')
    const reload = body.indexOf('await reload()')
    const ask = body.indexOf('openMigration()')

    expect(set).toBeGreaterThan(-1)
    expect(push).toBeGreaterThan(set)
    expect(reload).toBeGreaterThan(push)
    expect(ask).toBeGreaterThan(reload)
  })

  /**
   * A failed read and an empty answer are different sentences.
   *
   * 「这台机器还没有下载过文件」 shown over a failed history request is how a
   * person concludes their downloads are gone. So the catch sets the reason and
   * never the empty flag, and the empty sentence is bound to its own flag.
   */
  it('shows why the list could not be read instead of an empty one', () => {
    const open = source.slice(source.indexOf('async function openMigration()'))
    const body = open.slice(0, open.indexOf('\n}'))

    expect(body).toContain('migrationEmpty.value = true')
    expect(body).toContain('migrationError.value = error?.message')
    // The empty flag is set on the empty answer, and the catch — read on its own —
    // must not set it. Asserting 「before the catch」 would not say this: the flag's
    // first occurrence is in the `try`, so a catch that set it too would still
    // come after one.
    const caught = body.slice(body.indexOf('catch ('))
    expect(caught).not.toContain('migrationEmpty.value = true')
    expect(caught).toContain('migrationError.value =')
    // And the template shows the reason with its own branch, before the empty
    // sentence, so the two cannot both render.
    expect(source).toContain('v-if="migrationError"')
    expect(source).toContain('v-else-if="migrationEmpty"')
  })

  /**
   * The Agent push is a note, not the page's error banner.
   *
   * Storing the choice did not fail, and the choice is not lost — the Agent reads
   * the settings file when it next starts. Rendering it as an error would report
   * a failure that did not happen.
   */
  it('reports a failed push without reporting a failed save', () => {
    expect(source).toContain('pushNote.value =')
    expect(source).toContain('v-if="pushNote"')
    // The push is not the run() path, which is what sets `failure`.
    const push = source.slice(source.indexOf('async function pushSaveDir()'))
    expect(push.slice(0, push.indexOf('\n}'))).not.toContain('failure.value')
  })

  // Nothing in this dialog may offer to move a file that the plan did not list:
  // the plan is the only source of names, and `chosenNames` only ever reads the
  // rows it produced.
  it('cannot name a file the plan did not mention', () => {
    expect(source).not.toContain('local_path')
    expect(source).toMatch(/const names = await loadCandidateNames\(\)/)
    expect(source).toMatch(/await service\.migrationPlan\(names\)/)
  })
})
