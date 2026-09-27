import { describe, expect, it } from 'vitest'

import {
  FILE_CHOICES,
  choiceLabel,
  chosenNames,
  createLocalSettingsPage,
  describeCleanup,
  describeDiagnostic,
  describeMigration,
  describeMigrationPlan,
  describePush,
  formatBytes,
  formatModified,
  keptReasonLabel,
  logKindLabel,
  migrationRows,
} from './apps/desktop/features/local-settings/local-settings-view.js'

describe('byte formatting', () => {
  it('shows bytes below a kilobyte as bytes', () => {
    expect(formatBytes(0)).toBe('0 B')
    expect(formatBytes(1)).toBe('1 B')
    expect(formatBytes(1023)).toBe('1023 B')
  })

  it('scales by 1024 and drops the decimal where it is noise', () => {
    expect(formatBytes(1024)).toBe('1.0 KB')
    expect(formatBytes(1536)).toBe('1.5 KB')
    // Below ten the decimal is informative; at and above it, rounding is what a
    // person reads.
    expect(formatBytes(10 * 1024)).toBe('10 KB')
    expect(formatBytes(1024 * 1024)).toBe('1.0 MB')
    expect(formatBytes(512 * 1024 * 1024 * 1024)).toBe('512 GB')
  })

  /**
   * A number that is not a number is 「未知」, never 「0 B」.
   *
   * The Rust side refuses to answer with a measurement it could not take, so
   * this branch is unreachable through the service — which is exactly why it is
   * asserted: if it ever becomes reachable, the failure must not look like an
   * empty directory.
   */
  it('refuses to render a value it cannot read as zero', () => {
    expect(formatBytes(undefined)).toBe('未知')
    expect(formatBytes(null)).toBe('未知')
    expect(formatBytes('abc')).toBe('未知')
    expect(formatBytes(-1)).toBe('未知')
  })
})

describe('modification time formatting', () => {
  const seconds = 1_774_000_000

  it('renders seconds as this machine local time', () => {
    const rendered = formatModified(seconds)
    const date = new Date(seconds * 1000)

    expect(rendered).toMatch(/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/)
    // Read back as local time, it is the same instant. A UTC rendering passes
    // the shape check above and fails here, which is the point: the user's
    // ruling is that both sides keep the machine's own timezone.
    expect(new Date(rendered.replace(' ', 'T')).getTime()).toBe(date.getTime())
  })

  /**
   * The stamp is built from the **local** getters, provably.
   *
   * A one-line swap of `getFullYear()` for `getUTCFullYear()` renders identical
   * strings except across a year boundary, so a test that only round-trips an
   * ordinary instant cannot see the difference. This one finds an instant in the
   * machine's *own* zone whose local year and UTC year differ, and asserts the
   * rendering follows the local one.
   *
   * The candidates straddle the UTC year boundary from both sides, so one of
   * them differs in any zone with a non-zero offset, east or west. On a machine
   * whose clock is UTC no such instant exists: the two spellings coincide, the
   * swap is a true equivalence, and this test says so instead of passing for a
   * reason it cannot distinguish from having no coverage at all.
   */
  it('builds the stamp from the local getters, not the UTC ones', () => {
    const pad = (number) => String(number).padStart(2, '0')
    const render = (date) =>
      `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ` +
      `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`

    const candidates = [
      Date.UTC(2026, 11, 31, 23, 59, 30),
      Date.UTC(2027, 0, 1, 0, 0, 30),
      Date.UTC(2026, 11, 31, 0, 0, 30),
      Date.UTC(2027, 0, 1, 23, 59, 30),
    ]
      .map((millis) => new Date(millis))
      .filter((date) => date.getFullYear() !== date.getUTCFullYear());

    if (candidates.length === 0) {
      // A UTC clock: the local and UTC getters are the same function there, so
      // there is nothing for this test to tell apart.
      expect(new Date(seconds * 1000).getTimezoneOffset()).toBe(0)
      return
    }

    for (const date of candidates) {
      const rendered = formatModified(Math.floor(date.getTime() / 1000))
      expect(rendered).toBe(render(date))
      // The control: the other spelling really does produce something else for
      // this instant, so the assertion above is not comparing a string with
      // itself.
      expect(render(date)).not.toBe(
        `${date.getUTCFullYear()}-${pad(date.getUTCMonth() + 1)}-${pad(date.getUTCDate())} ` +
          `${pad(date.getUTCHours())}:${pad(date.getUTCMinutes())}:${pad(date.getUTCSeconds())}`
      )
    }
  })

  it('pads every field to two digits', () => {
    // A timestamp whose local parts are single-digit in every zone west of
    // Greenwich and in several east of it.
    const rendered = formatModified(Date.UTC(2026, 0, 2, 3, 4, 5) / 1000)
    const [, time] = rendered.split(' ')
    for (const part of time.split(':')) {
      expect(part).toHaveLength(2)
    }
  })

  it('says 未知 rather than inventing a date', () => {
    expect(formatModified(null)).toBe('未知')
    expect(formatModified(undefined)).toBe('未知')
    expect(formatModified(0)).toBe('未知')
    expect(formatModified('not a number')).toBe('未知')
  })
})

describe('log file kinds', () => {
  it('names the three kinds the listing can report', () => {
    expect(logKindLabel('live')).toBe('正在写入')
    expect(logKindLabel('archive')).toBe('已归档')
    expect(logKindLabel('other')).toBe('无法识别')
    // A kind this build does not know reads as unrecognised rather than as the
    // empty string, which would look like a rendering bug.
    expect(logKindLabel('something-new')).toBe('无法识别')
  })
})

/**
 * The four outcomes of a cleanup.
 *
 * 「已释放 0 字节」is the same string for three of them, which is the whole
 * reason the report carries counts — so each outcome is asserted by its *theme*
 * as well as its text, because a page that said the right words in the wrong
 * colour would still be telling a person the wrong thing.
 */
describe('cleanup reporting', () => {
  function report(overrides = {}) {
    return {
      label: 'cache',
      directory: '/Users/operator/Library/Caches/WTMedia/Desktop',
      freedBytes: 0,
      filesRemoved: 0,
      directoriesRemoved: 0,
      kept: [],
      failures: [],
      ...overrides,
    }
  }

  it('reports work done as success, naming the directory', () => {
    const said = describeCleanup(report({ freedBytes: 4096, filesRemoved: 2 }))

    expect(said.theme).toBe('success')
    expect(said.text).toContain('4.0 KB')
    expect(said.text).toContain('2 个文件')
    expect(said.text).toContain('/Users/operator/Library/Caches/WTMedia/Desktop')
  })

  /**
   * Nothing to do is a real outcome, and it still names what was kept.
   *
   * `kept` is not a failure list, but it is also not nothing: a person who
   * pressed a cleanup button and read 「没有需要清理的文件」 over a directory that
   * does contain a live log is owed the reason — the live file was deliberately
   * left alone.
   */
  it('reports nothing to do as information, naming what was kept', () => {
    const said = describeCleanup(report())

    expect(said.theme).toBe('info')
    expect(said.text).toBe('没有需要清理的文件')
    expect(said.detail).toBe('')

    const withKept = describeCleanup(
      report({ kept: [{ name: 'desktop.log', reason: 'live' }] })
    )
    expect(withKept.theme).toBe('info')
    expect(withKept.text).toBe('没有需要清理的文件')
    expect(withKept.detail).toContain('保留了 1 项')
    expect(withKept.detail).toContain('desktop.log')
  })

  it('reports a partial cleanup as a warning with the failures listed', () => {
    const said = describeCleanup(
      report({
        freedBytes: 1024,
        filesRemoved: 1,
        failures: [{ path: '/cache/locked.bin', reason: 'Permission denied (os error 13)' }],
      })
    )

    expect(said.theme).toBe('warning')
    expect(said.text).toContain('1 个文件')
    expect(said.detail).toContain('/cache/locked.bin')
    // The operating system's own words, passed through rather than summarised.
    expect(said.detail).toContain('Permission denied')
  })

  /**
   * A cleanup that removed nothing *because it failed* is not a cleanup that had
   * nothing to do. Both have `freedBytes === 0`, and only the failure count tells
   * them apart — so this is the case that would go wrong first.
   */
  it('does not report a wholly failed cleanup as an empty one', () => {
    const said = describeCleanup(
      report({ failures: [{ path: '/cache/locked.bin', reason: 'Permission denied' }] })
    )

    expect(said.theme).toBe('error')
    expect(said.text).toContain('没有完成')
    expect(said.text).not.toBe('没有需要清理的文件')
  })

  it('names what was deliberately kept, without calling it a failure', () => {
    const said = describeCleanup(
      report({ freedBytes: 512, filesRemoved: 1, kept: [{ name: 'desktop.log', reason: 'live' }] })
    )

    expect(said.theme).toBe('success')
    expect(said.detail).toContain('desktop.log')
    expect(said.detail).toContain('保留')
  })
})

describe('diagnostic export reporting', () => {
  function exported(overrides = {}) {
    return {
      path: '/Users/operator/Downloads/wt-media-diagnostic-20260924-225501.tar.gz',
      checksumPath: '/Users/operator/Downloads/wt-media-diagnostic-20260924-225501.tar.gz.sha256',
      bytes: 8192,
      sha256: 'a'.repeat(64),
      createdAt: '2026-09-24T22:55:01',
      entries: [{ name: 'logs/desktop/desktop.log', bytes: 512, truncated: true }],
      omitted: [{ name: 'logs/agent/big.log', reason: 'archive_full' }],
      agentState: 'answered',
      failedRecords: 2,
      ...overrides,
    }
  }

  it('says where the archive is and what is in it', () => {
    const said = describeDiagnostic(exported())

    expect(said.theme).toBe('success')
    expect(said.text).toContain('wt-media-diagnostic-20260924-225501.tar.gz')
    expect(said.detail).toContain('8.0 KB')
    expect(said.detail).toContain('a'.repeat(64))
    expect(said.detail).toContain('另有 1 个未收录')
    expect(said.agentUnreachable).toBe(false)
  })

  /**
   * An unreachable Agent still produces a bundle, so this is a fact about the
   * export rather than a failure — but a person attaching it needs to know which
   * one they have.
   */
  it('flags a bundle made without the Agent', () => {
    const said = describeDiagnostic(exported({ agentState: 'unreachable' }))

    expect(said.theme).toBe('success')
    expect(said.agentUnreachable).toBe(true)
    expect(said.detail).toContain('不可达')
  })
})

describe('settings page state', () => {
  it('says 未设置 rather than a guessed default', () => {
    const page = createLocalSettingsPage({
      settings: { saveDir: null, file: '/data/settings.toml' },
    })

    expect(page.saveDirText).toBe('未设置')
    expect(page.saveDirSet).toBe(false)
    expect(page.fileText).toBe('/data/settings.toml')
    expect(page.fileSet).toBe(true)
  })

  it('shows a chosen directory as it is stored', () => {
    const page = createLocalSettingsPage({
      settings: { saveDir: '/Users/operator/Movies/WTMedia', file: '/data/settings.toml' },
    })

    expect(page.saveDirText).toBe('/Users/operator/Movies/WTMedia')
    expect(page.saveDirSet).toBe(true)
  })

  it('describes each log tree with its own directory and size', () => {
    const page = createLocalSettingsPage({
      usage: {
        availableBytes: 512 * 1024 * 1024 * 1024,
        cacheBytes: 1024 * 1024,
        logs: [
          { source: 'desktop', directory: '/logs/desktop', bytes: 2048, files: 3 },
          { source: 'agent', directory: '/logs/agent', bytes: 0, files: 0 },
        ],
      },
    })

    expect(page.availableText).toBe('512 GB')
    expect(page.cacheText).toBe('1.0 MB')
    expect(page.logTrees).toEqual([
      { source: 'desktop', directory: '/logs/desktop', bytesText: '2.0 KB', filesText: '3 个文件', isEmpty: false },
      // An empty tree says so rather than showing 「0 B」: same measurement,
      // different sentence, and only one of them reads as a state.
      { source: 'agent', directory: '/logs/agent', bytesText: '0 B', filesText: '0 个文件', isEmpty: true },
    ])
  })

  it('renders before either answer has arrived', () => {
    const page = createLocalSettingsPage()

    expect(page.saveDirText).toBe('')
    expect(page.fileText).toBe('')
    expect(page.logTrees).toEqual([])
    expect(page.availableText).toBe('')
  })
})

/**
 * The 搬运 dialog.
 *
 * This is the only place the page touches files the person did not just create,
 * so the rules live here — in a plain module a test can import — rather than in
 * the `.vue` file. What is asserted below is mostly about *not* offering things:
 * not offering an action on a file that is already in place, not calling a file
 * gone when a directory could not be read, and not starting with anything
 * selected.
 */
/**
 * The note under the save-location field after a push.
 *
 * It is written from the **Agent's answer**, and the three branches are three
 * different states of the machine — which is why they are three sentences rather
 * than one with the directory substituted in.
 */
describe('push reporting', () => {
  it('reports the directory the Agent will actually use, with its free space', () => {
    const said = describePush({ saveDir: '/new', writable: true, freeBytes: 1024 ** 3 })

    expect(said.theme).toBe('info')
    expect(said.text).toContain('/new')
    // The Agent's own number, formatted here — not a bare byte count.
    expect(said.text).toContain('1.0 GB')
  })

  // Stored but unusable is a warning, not a success: the next download will fail
  // and the person should hear it from this page rather than from a failed task.
  it('warns when the Agent has the directory but cannot write there', () => {
    const said = describePush({ saveDir: '/new', writable: false, freeBytes: 10 })

    expect(said.theme).toBe('warning')
    expect(said.text).toContain('/new')
    expect(said.text).toContain('写不进去')
  })

  /**
   * An Agent that reports no directory at all after a push is not 「已推送」.
   *
   * This is the branch the first draft of this page got wrong: it rendered the
   * command's return value with `String(...)`, so this case showed
   * 「[object Object]」 and every case looked like a success. The answer is a
   * read-back, so 「没有记下」 is a real outcome and needs its own sentence.
   */
  it('does not call it pushed when the Agent reports no directory', () => {
    const said = describePush({ saveDir: null, writable: false, freeBytes: 0 })

    expect(said.theme).toBe('warning')
    expect(said.text).toContain('没有记下')
    expect(said.text).not.toContain('null')
  })
})

describe('migration dialog rows', () => {
  function plan(overrides = {}) {
    return {
      to: '/new',
      freeBytes: 1_000_000,
      neededBytes: 500,
      movable: [{ name: 'a.mp4', directory: '/old', bytes: 500 }],
      alreadyThere: [{ name: 'b.mp4', directory: '/new', bytes: 300 }],
      missing: [{ name: 'c.mp4', directory: '', bytes: null }],
      unreadable: [],
      ...overrides,
    }
  }

  // The order is the priority: what needs deciding first, then what is already
  // fine, then what is not there.
  it('lists the files that need a decision before the ones that do not', () => {
    expect(migrationRows(plan()).map((row) => row.status)).toEqual([
      'movable', 'already_there', 'missing',
    ])
  })

  /**
   * **Nothing starts selected.** The default for every file is 保留.
   *
   * Moving and deleting act on the person's own files, and a dialog that opened
   * with 「搬运」 already chosen turns one press of the confirm button into a bulk
   * file operation nobody asked for. The cost of the safe default is a few
   * clicks; the cost of the other one is somebody's files.
   */
  it('starts every file at 保留, with nothing to carry out', () => {
    const rows = migrationRows(plan())

    expect(rows.map((row) => row.choice)).toEqual(['keep', 'keep', 'keep'])
    expect(chosenNames(rows, FILE_CHOICES.move)).toEqual([])
    expect(chosenNames(rows, FILE_CHOICES.delete)).toEqual([])
    // And the words the buttons show come from one place.
    expect(rows.map((row) => choiceLabel(row.choice))).toEqual(['保留', '保留', '保留'])
  })

  it('carries a decision through to the name it was made about', () => {
    const rows = migrationRows(plan(), { 'a.mp4': FILE_CHOICES.move })

    expect(chosenNames(rows, FILE_CHOICES.move)).toEqual(['a.mp4'])
    expect(chosenNames(rows, FILE_CHOICES.delete)).toEqual([])
  })

  // A file already in the chosen directory has nothing to decide, and a file that
  // is not there cannot be acted on. Offering either one buttons gets an action
  // whose only possible outcome is a report saying nothing happened.
  it('offers no buttons on a row that has nothing to decide', () => {
    const rows = migrationRows(plan(), { 'a.mp4': FILE_CHOICES.move, 'b.mp4': FILE_CHOICES.delete })

    expect(rows.map((row) => row.selectable)).toEqual([true, false, false])
    // Even a caller that marked them is not obeyed.
    expect(chosenNames(rows, FILE_CHOICES.delete)).toEqual([])
    expect(chosenNames(rows, FILE_CHOICES.move)).toEqual(['a.mp4'])
  })

  it('shows a size that was not read as 未知 rather than as zero', () => {
    const rows = migrationRows(plan())

    expect(rows[0].bytesText).toBe('500 B')
    expect(rows[2].bytesText).toBe('未知')
  })

  /**
   * 「未找到」 and 「已不存在」 are different claims, and only one of them is
   * available after a directory could not be read.
   *
   * A file may be sitting in the directory nobody could open. Telling a person it
   * no longer exists is how they re-download 230 MB they still have.
   */
  it('will not call a file gone while a known directory went unread', () => {
    const whole = migrationRows(plan())
    expect(whole[2].statusLabel).toBe('已不存在')

    const partial = migrationRows(plan({
      unreadable: [{ directory: '/locked', reason: 'Permission denied (os error 13)' }],
    }))
    expect(partial[2].statusLabel).toBe('未找到')
    // The rows that *were* measured keep their own labels either way.
    expect(partial[0].statusLabel).toBe('待搬运')
    expect(partial[1].statusLabel).toBe('已在当前位置')
  })

  it('names a kept file by the reason the Rust side recorded', () => {
    expect(keptReasonLabel('target_exists')).toBe('目标位置已有同名文件')
    expect(keptReasonLabel('same_directory')).toBe('已经在目标位置')
    // An unrecognised token is passed through rather than replaced: a reason
    // nobody translated is still more use than a blank.
    expect(keptReasonLabel('something_new')).toBe('something_new')
  })
})

describe('migration plan summary', () => {
  function plan(overrides = {}) {
    return {
      to: '/new',
      freeBytes: 1_000_000,
      neededBytes: 500,
      movable: [{ name: 'a.mp4', directory: '/old', bytes: 500 }],
      alreadyThere: [],
      missing: [],
      unreadable: [],
      ...overrides,
    }
  }

  it('says where the files would go and how much room they need', () => {
    const said = describeMigrationPlan(plan())

    expect(said.theme).toBe('info')
    expect(said.text).toContain('/new')
    expect(said.detail).toContain('1 个文件')
    expect(said.detail).toContain('500 B')
    expect(said.shortfall).toBe(false)
  })

  // A shortfall is information, not a refusal — the dialog stays usable, because
  // the person may be about to free the space or to move the files somewhere else.
  it('flags a target without room without refusing the move', () => {
    const said = describeMigrationPlan(plan({ freeBytes: 100, neededBytes: 500 }))

    expect(said.shortfall).toBe(true)
    expect(said.theme).toBe('warning')
    expect(said.text).toContain('1 个文件')
  })

  // The total is a lower bound once a size is missing, and it says so: a number
  // that quietly left out the unmeasured file is a space check that passes on a
  // disk that is full.
  it('calls the total a lower bound when a size could not be read', () => {
    const said = describeMigrationPlan(plan({
      movable: [
        { name: 'a.mp4', directory: '/old', bytes: 500 },
        { name: 'b.mp4', directory: '/old', bytes: null },
      ],
    }))

    expect(said.detail).toContain('至少')
    expect(said.detail).toContain('1 个文件读不到体积')
  })

  /**
   * A directory that could not be listed is named, with its reason.
   *
   * Almost every other sentence in this dialog is a claim about a file. This one
   * is the caveat that makes 「未找到」 less than a fact, so it has to be visible
   * rather than folded into a count.
   */
  it('names the directories it could not read, and why', () => {
    const said = describeMigrationPlan(plan({
      missing: [{ name: 'c.mp4', directory: '', bytes: null }],
      unreadable: [{ directory: '/locked', reason: 'Permission denied (os error 13)' }],
    }))

    expect(said.detail).toContain('/locked')
    expect(said.detail).toContain('Permission denied (os error 13)')
    expect(said.detail).toContain('在查得到的目录里没找到')
    expect(said.theme).toBe('warning')
  })

  it('says there is nothing to move when there is nothing to move', () => {
    const said = describeMigrationPlan(plan({ movable: [], neededBytes: 0 }))

    expect(said.text).toBe('没有需要搬运的文件')
  })
})

describe('migration reporting', () => {
  function report(overrides = {}) {
    return {
      directory: '/new',
      moved: [],
      deleted: [],
      missing: [],
      kept: [],
      failures: [],
      ...overrides,
    }
  }

  it('reports a completed move as success, naming where the files went', () => {
    const said = describeMigration(report({ moved: ['a.mp4', 'b.mp4'] }), FILE_CHOICES.move)

    expect(said.theme).toBe('success')
    expect(said.text).toContain('/new')
    expect(said.text).toContain('2 个文件')
  })

  // A deletion has no destination. Naming the chosen directory beside it would
  // read as 「these were deleted from there」, which is not what the report says.
  it('does not name a destination for a deletion', () => {
    const said = describeMigration(report({ deleted: ['a.mp4'] }), FILE_CHOICES.delete)

    expect(said.theme).toBe('success')
    expect(said.text).toContain('已删除')
    expect(said.text).not.toContain('/new')
  })

  /**
   * A move that failed *and* moved nothing is not a move that had nothing to do.
   *
   * The two are indistinguishable from the count alone — which is why the same
   * shape as `describeCleanup` is used here rather than a single sentence with
   * the number in it.
   */
  it('does not report a wholly failed move as an empty one', () => {
    const said = describeMigration(report({ failures: [{ name: 'a.mp4', reason: 'read-only' }] }), FILE_CHOICES.move)

    expect(said.theme).toBe('error')
    expect(said.text).toContain('没有完成')
    expect(said.text).not.toContain('没有文件需要')
    expect(said.detail).toContain('a.mp4')
    expect(said.detail).toContain('read-only')
  })

  it('reports a partial move as a warning with the failures listed', () => {
    const said = describeMigration(report({
      moved: ['a.mp4'],
      failures: [{ name: 'b.mp4', reason: 'read-only' }],
    }), FILE_CHOICES.move)

    expect(said.theme).toBe('warning')
    expect(said.text).toContain('部分完成')
    expect(said.detail).toContain('b.mp4')
  })

  // Kept files are not failures, and they come with their reason — a file left
  // behind because the target already had that name is the one case a person
  // needs to know about before they assume the move did nothing.
  it('names what was kept, with the reason, without calling it a failure', () => {
    const said = describeMigration(report({
      moved: ['a.mp4'],
      kept: [{ name: 'b.mp4', reason: 'target_exists' }],
      missing: ['z.mp4'],
    }), FILE_CHOICES.move)

    expect(said.theme).toBe('success')
    expect(said.detail).toContain('b.mp4')
    expect(said.detail).toContain('目标位置已有同名文件')
    expect(said.detail).toContain('z.mp4')
  })

  it('reports nothing to do as information rather than as success', () => {
    const said = describeMigration(report({ kept: [{ name: 'a.mp4', reason: 'same_directory' }] }))

    expect(said.theme).toBe('info')
    expect(said.text).toBe('没有文件需要搬运')
    expect(said.detail).toContain('已经在目标位置')
  })
})
