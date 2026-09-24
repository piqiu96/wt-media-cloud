import { describe, expect, it } from 'vitest'

import {
  createLocalLogsPage,
  levelLabel,
  levelOption,
  levelTheme,
  LOG_LEVELS,
  NO_LEVEL,
  UNCLASSIFIED_LABEL,
} from './apps/desktop/features/local-logs/local-logs-view.js'

describe('the level filter vocabulary', () => {
  /**
   * Every option but the last names a *floor*, not a match.
   *
   * The Rust side's `filter_min` keeps entries at the chosen level or louder, so
   * an option labelled 「警告」 would promise an exact match the command does not
   * make. The wording is the assertion.
   */
  it('says each option means that level and louder', () => {
    const labels = LOG_LEVELS.map((option) => option.label)

    expect(labels).toEqual([
      '全部（含跟踪）',
      '调试及以上',
      '信息及以上',
      '警告及以上',
      '仅错误',
    ])
    // The floor wording, on every option that has one.
    for (const option of LOG_LEVELS.slice(1)) {
      expect(option.label).toMatch(/及以上|仅/)
    }
  })

  it('lists the levels in severity order, weakest first', () => {
    expect(LOG_LEVELS.map((option) => option.value)).toEqual([
      'trace',
      'debug',
      'info',
      'warn',
      'error',
    ])
  })

  it('resolves a known value to its option and refuses an unknown one', () => {
    expect(levelOption('warn')).toEqual({ value: 'warn', label: '警告及以上' })
    expect(levelOption('verbose')).toBeNull()
    expect(levelOption(null)).toBeNull()
    expect(levelOption(undefined)).toBeNull()
    // The wire's 「no filter」 is `null`, and `NO_LEVEL` is the page's spelling of
    // the same thing — the two must not drift into two different absences.
    expect(NO_LEVEL).toBeNull()
  })
})

describe('one line level', () => {
  it('spells the five levels and the absence of one', () => {
    expect(levelLabel('error')).toBe('错误')
    expect(levelLabel('warn')).toBe('警告')
    expect(levelLabel('info')).toBe('信息')
    expect(levelLabel('debug')).toBe('调试')
    expect(levelLabel('trace')).toBe('跟踪')
    expect(levelLabel(null)).toBe(UNCLASSIFIED_LABEL)
    expect(levelLabel(undefined)).toBe(UNCLASSIFIED_LABEL)
    // An unknown token is shown as itself rather than dropped: a reader that
    // learned a new level must not produce blank gutters.
    expect(levelLabel('critical')).toBe('critical')
  })

  /**
   * An unclassified line is not an error.
   *
   * It is a line this reader could not attribute — the beginning of a file, or a
   * record written before any level was — and colouring it red would report a
   * problem in the log that the log does not have.
   */
  it('does not colour an unclassified line as a failure', () => {
    expect(levelTheme(null)).toBe('default')
    expect(levelTheme('info')).toBe('default')
    expect(levelTheme('warn')).toBe('warning')
    expect(levelTheme('error')).toBe('danger')
  })
})

describe('log viewer state', () => {
  /**
   * Two trees, both holding a file named `error.log`.
   *
   * The shared name is the point, not decoration: selection is a pair of
   * (tree, file), and a fixture whose second tree were empty would be satisfied
   * by matching the file name alone. A mutation that dropped the tree from the
   * comparison survived this fixture before the shared name was added.
   */
  function listing() {
    return {
      trees: [
        {
          source: 'desktop',
          directory: '/logs/desktop',
          files: [
            { name: 'desktop.log', kind: 'live', bytes: 512, modifiedSeconds: 1_774_000_000 },
            { name: 'desktop.log.2026-09-24-23', kind: 'archive', bytes: 4096, modifiedSeconds: null },
            { name: 'error.log', kind: 'other', bytes: 64, modifiedSeconds: null },
          ],
        },
        {
          source: 'agent',
          directory: '/logs/agent',
          files: [
            { name: 'agent.log', kind: 'live', bytes: 256, modifiedSeconds: null },
            { name: 'error.log', kind: 'live', bytes: 128, modifiedSeconds: null },
          ],
        },
      ],
    }
  }

  function tail(overrides = {}) {
    return {
      source: 'desktop',
      name: 'desktop.log',
      path: '/logs/desktop/desktop.log',
      minLevel: null,
      linesRead: 3,
      linesShown: 3,
      truncated: false,
      lines: [{ level: 'info', continues: false, text: 'one' }],
      ...overrides,
    }
  }

  it('marks the selected file and reports the directory it came from', () => {
    const page = createLocalLogsPage({
      listing: listing(),
      source: 'desktop',
      name: 'desktop.log',
      tail: tail(),
    })

    expect(page.selectedDirectory).toBe('/logs/desktop')
    expect(page.selectedPath).toBe('/logs/desktop/desktop.log')
    expect(page.canView).toBe(true)
    expect(page.trees[0].files[0].selected).toBe(true)
    expect(page.trees[0].files[1].selected).toBe(false)
    expect(page.trees[0].files[2].selected).toBe(false)
  })

  /**
   * The same file name in the other tree is not the same file.
   *
   * Both trees hold an `error.log`, so a selection that matched on the name
   * alone would mark two rows — and the one the person clicked is then not
   * distinguishable from the one they did not.
   */
  it('selects a file by its tree and its name together', () => {
    const desktop = createLocalLogsPage({
      listing: listing(),
      source: 'desktop',
      name: 'error.log',
      tail: tail({ source: 'desktop', name: 'error.log', path: '/logs/desktop/error.log' }),
    })
    const agent = createLocalLogsPage({
      listing: listing(),
      source: 'agent',
      name: 'error.log',
      tail: tail({ source: 'agent', name: 'error.log', path: '/logs/agent/error.log' }),
    })

    const flagged = (page) =>
      page.trees.flatMap((tree) => tree.files.filter((file) => file.selected).map((file) => `${tree.source}/${file.name}`))

    expect(flagged(desktop)).toEqual(['desktop/error.log'])
    expect(flagged(agent)).toEqual(['agent/error.log'])
    expect(desktop.selectedDirectory).toBe('/logs/desktop')
    expect(agent.selectedDirectory).toBe('/logs/agent')
  })

  it('keeps the listing order it was given rather than sorting again', () => {
    const page = createLocalLogsPage({ listing: listing(), source: 'desktop', name: 'desktop.log' })

    expect(page.trees[0].files.map((file) => file.name)).toEqual([
      'desktop.log',
      'desktop.log.2026-09-24-23',
      'error.log',
    ])
    expect(page.trees[1].files.map((file) => file.name)).toEqual(['agent.log', 'error.log'])
  })

  /**
   * The filter quoted is the one the answer was under, not the one that was
   * asked for.
   *
   * A filter that was requested and not applied is worse than no filter: the
   * page would say 「仅错误」 over a full log. Only the answer knows.
   */
  it('quotes the applied filter from the answer, not from the request', () => {
    const applied = createLocalLogsPage({
      listing: listing(),
      source: 'desktop',
      name: 'desktop.log',
      tail: tail({ minLevel: 'warn' }),
      level: 'error',
    })
    const ignored = createLocalLogsPage({
      listing: listing(),
      source: 'desktop',
      name: 'desktop.log',
      tail: tail({ minLevel: null }),
      level: 'error',
    })

    expect(applied.appliedFilterText).toBe('警告及以上')
    expect(ignored.appliedFilterText).toBe('未筛选')
  })

  it('reports both counts as the command gave them, without recounting', () => {
    const page = createLocalLogsPage({
      listing: listing(),
      source: 'desktop',
      name: 'desktop.log',
      // Deliberately inconsistent with `lines`: a page that recounted locally
      // would hide the disagreement rather than show it.
      tail: tail({ linesRead: 900, linesShown: 4 }),
    })

    expect(page.countsText).toBe('读取 900 行，显示 4 行')
    expect(page.hiddenCount).toBe(896)
    expect(page.hiddenNotice).toContain('896')
  })

  /**
   * The hidden notice says what a filter does *not* hide.
   *
   * Lines the reader could not classify are kept whatever the filter says, so
   * 「筛选隐藏了 N 行」alone would leave a person believing the filter is total.
   */
  it('says unclassified lines are not affected by the filter', () => {
    const page = createLocalLogsPage({
      listing: listing(),
      source: 'desktop',
      name: 'desktop.log',
      tail: tail({ linesRead: 10, linesShown: 2, minLevel: 'warn' }),
    })

    expect(page.hiddenNotice).toContain('未分级')
    expect(page.hiddenNotice).toContain('始终显示')
  })

  it('says nothing about hiding when nothing is hidden', () => {
    const page = createLocalLogsPage({
      listing: listing(),
      source: 'desktop',
      name: 'desktop.log',
      tail: tail(),
    })

    expect(page.hiddenNotice).toBe('')
    expect(page.hiddenCount).toBe(0)
  })

  /**
   * A truncation is a statement about which end of the file is on screen.
   *
   * Without it, a short-looking view of a long file reads as a short file.
   */
  it('says the view is the tail when the byte ceiling ended the read', () => {
    const truncated = createLocalLogsPage({
      listing: listing(),
      source: 'desktop',
      name: 'desktop.log',
      tail: tail({ truncated: true }),
    })
    const whole = createLocalLogsPage({
      listing: listing(),
      source: 'desktop',
      name: 'desktop.log',
      tail: tail(),
    })

    expect(truncated.truncatedNotice).toContain('尾部')
    expect(whole.truncatedNotice).toBe('')
  })

  it('renders a continuation as an indent rather than as a second record', () => {
    const page = createLocalLogsPage({
      listing: listing(),
      source: 'desktop',
      name: 'desktop.log',
      tail: tail({
        lines: [
          { level: 'error', continues: false, text: 'a record' },
          { level: null, continues: true, text: 'its second line' },
        ],
      }),
    })

    expect(page.lines[0].indent).toBe(false)
    expect(page.lines[0].levelText).toBe('错误')
    expect(page.lines[1].indent).toBe(true)
    // A continuation has no gutter label of its own: it belongs to the record
    // above, and a repeated badge would suggest a second event.
    expect(page.lines[1].levelText).toBe('')
  })

  it('says which file is open before anything has been read', () => {
    const page = createLocalLogsPage({ listing: listing() })

    expect(page.canView).toBe(false)
    expect(page.hasLines).toBe(false)
    expect(page.countsText).toBe('')
    expect(page.selectedName).toBe('')
    expect(page.selectedDirectory).toBe('')
  })

  it('offers the trees even when both are empty', () => {
    const page = createLocalLogsPage({
      listing: { trees: [{ source: 'desktop', directory: '/logs/desktop', files: [] }] },
    })

    expect(page.hasTrees).toBe(true)
    expect(page.trees[0].files).toEqual([])
  })

  it('renders before a listing has arrived', () => {
    const page = createLocalLogsPage()

    expect(page.trees).toEqual([])
    expect(page.hasTrees).toBe(false)
    expect(page.filterOptions).toBe(LOG_LEVELS)
  })
})
