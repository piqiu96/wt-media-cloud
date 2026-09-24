import { describe, expect, it } from 'vitest'

import {
  createLocalSettingsPage,
  describeCleanup,
  describeDiagnostic,
  formatBytes,
  formatModified,
  logKindLabel,
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
