import { describe, expect, it, vi } from 'vitest'

import {
  createLocalSettingsService,
  createMockInvoke,
  DEFAULT_TAIL_LINES,
  LOG_SOURCES,
  LOCAL_SETTINGS_COMMANDS,
  normalizeLogTail,
  normalizeMigrationPlan,
  normalizeMigrationReport,
  normalizeSearchDirectoryView,
  normalizeSettingsView,
} from './apps/desktop/features/local-settings/service.js'

/**
 * The exact command names, pinned.
 *
 * These are the Rust side's function names, and `main.rs` registers them by
 * full path. A rename on either side is a page that silently does nothing —
 * `invoke` rejects on an unknown command, and the page's error banner is the
 * only place it would show — so the spelling is asserted here rather than
 * discovered in a build.
 */
describe('local settings command names', () => {
  it('spells every command the way the Rust side registers it', () => {
    expect(LOCAL_SETTINGS_COMMANDS).toEqual({
      getSettings: 'local_settings_get',
      setSettings: 'local_settings_set',
      pushSaveDirectory: 'local_push_save_directory',
      storageUsage: 'local_storage_usage',
      logFiles: 'local_log_files',
      logTail: 'local_log_tail',
      cacheCleanup: 'local_cache_cleanup',
      logCleanup: 'local_log_cleanup',
      diagnosticExport: 'local_diagnostic_export',
      openPlace: 'local_open_place',
      migrationPlan: 'local_save_dir_migration_plan',
      moveSavedFiles: 'local_move_saved_files',
      deleteSavedFiles: 'local_delete_saved_files',
      addSearchDirectory: 'local_pick_search_directory',
    })
  })

  it('names the two components the viewer lists', () => {
    expect(LOG_SOURCES).toEqual(['desktop', 'agent'])
  })
})

/**
 * The invoke arguments, exactly.
 *
 * Tauri v2 renames arguments between the two sides: Rust's `save_dir` is the
 * WebView's `saveDir`, `min_level` is `minLevel`. A service that sent the
 * snake_case spelling would reach a command whose argument is `None`, store
 * nothing, and answer success — so the objects are pinned rather than left to
 * `localAgentService.test.js`'s sibling assertions to imply.
 */
describe('local settings service invoke arguments', () => {
  it('reads settings and storage with no arguments at all', async () => {
    const invoke = vi.fn(async (command) => {
      if (command === LOCAL_SETTINGS_COMMANDS.getSettings) {
        return { save_dir: null, file: '/tmp/settings.toml' }
      }
      return { available_bytes: 1, cache_bytes: 2, logs: [] }
    })
    const service = createLocalSettingsService({ invoke })

    await service.getSettings()
    await service.storageUsage()

    expect(invoke).toHaveBeenNthCalledWith(1, 'local_settings_get')
    expect(invoke).toHaveBeenNthCalledWith(2, 'local_storage_usage')
    // One argument: no options object, so a future `{ args: … }` wrapper cannot
    // arrive unnoticed.
    expect(invoke.mock.calls[1]).toHaveLength(1)
  })

  it('sends the chosen directory as camelCase saveDir', async () => {
    const invoke = vi.fn(async () => ({ save_dir: '/Users/operator/Movies', file: '/tmp/s.toml' }))
    const service = createLocalSettingsService({ invoke })

    await service.setSaveDir('/Users/operator/Movies')

    expect(invoke).toHaveBeenCalledWith('local_settings_set', {
      saveDir: '/Users/operator/Movies',
    })
  })

  /**
   * `null` and `""` are different requests.
   *
   * The first means 「没有选择」 and the second means an empty path, which the
   * Rust side refuses. A caller clearing the choice must not be able to produce
   * the second by accident, and an empty input must not be turned into the
   * first — so both spellings are asserted, including the whitespace-only one
   * that a trimmed input would otherwise become `''`.
   */
  it('keeps clearing the choice distinct from choosing an empty path', async () => {
    const invoke = vi.fn(async (command, args = {}) => ({
      save_dir: args.saveDir ?? null,
      file: '/tmp/s.toml',
    }))
    const service = createLocalSettingsService({ invoke })

    await service.setSaveDir(null)
    await service.setSaveDir(undefined)
    await service.setSaveDir('')

    expect(invoke.mock.calls[0][1]).toEqual({ saveDir: null })
    expect(invoke.mock.calls[1][1]).toEqual({ saveDir: null })
    // The empty string is passed through as a value, not converted to `null`:
    // which of the two it is has to be the caller's decision, because the
    // refusal a person needs to see depends on it.
    expect(invoke.mock.calls[2][1]).toEqual({ saveDir: '' })
  })

  it('sends the tail request with camelCase minLevel and the line ceiling', async () => {
    const invoke = vi.fn(async () => ({
      source: 'desktop',
      name: 'desktop.log',
      path: '/logs/desktop.log',
      min_level: 'warn',
      lines_read: 1,
      lines_shown: 1,
      truncated: false,
      lines: [],
    }))
    const service = createLocalSettingsService({ invoke })

    await service.logTail({ source: 'desktop', name: 'desktop.log', minLevel: 'warn' })
    await service.logTail({ source: 'agent', name: 'agent.log' })

    expect(invoke).toHaveBeenNthCalledWith(1, 'local_log_tail', {
      source: 'desktop',
      name: 'desktop.log',
      lines: DEFAULT_TAIL_LINES,
      minLevel: 'warn',
    })
    // No filter is `null`, not `''`: the Rust side parses the string and refuses
    // a spelling it does not know, so an empty string would be a refusal rather
    // than 「no filter」.
    expect(invoke.mock.calls[1][1]).toEqual({
      source: 'agent',
      name: 'agent.log',
      lines: DEFAULT_TAIL_LINES,
      minLevel: null,
    })
  })

  it('sends a place label, never a path', async () => {
    const invoke = vi.fn(async () => '/opened')
    const service = createLocalSettingsService({ invoke })

    for (const place of LOG_SOURCES.concat('data')) {
      await service.openPlace(place)
    }

    expect(invoke.mock.calls.map((call) => call[1])).toEqual([
      { place: 'desktop' },
      { place: 'agent' },
      { place: 'data' },
    ])
    // The whole point of the vocabulary: no argument here can name a directory.
    for (const call of invoke.mock.calls) {
      expect(call[1].place).not.toMatch(/[/\\]/)
    }
  })

  /**
   * The listing is normalized on the way out, tree by tree and file by file.
   *
   * A wrapper that returned the answer's `trees` untouched would hand the page
   * `modified_seconds` where it reads `modifiedSeconds`, and every mtime would
   * render as 「未知」 — which looks like a filesystem that would not say, not
   * like a missing conversion.
   */
  it('normalizes every file in the listing, not just the tree around it', async () => {
    const invoke = vi.fn(async () => ({
      trees: [
        {
          source: 'desktop',
          directory: '/logs/desktop',
          files: [{ name: 'desktop.log', kind: 'live', bytes: 512, modified_seconds: 1_774_000_000 }],
        },
      ],
    }))
    const service = createLocalSettingsService({ invoke })

    const listing = await service.logFiles()

    expect(Object.keys(listing.trees[0]).sort()).toEqual(['directory', 'files', 'source'])
    expect(listing.trees[0].files[0]).toEqual({
      name: 'desktop.log',
      kind: 'live',
      bytes: 512,
      modifiedSeconds: 1_774_000_000,
    })
    expect(listing.trees[0].files[0]).not.toHaveProperty('modified_seconds')
  })

  it('sends a log cleanup for one named tree', async () => {
    const invoke = vi.fn(async () => ({
      label: 'desktop',
      directory: '/logs',
      freed_bytes: 0,
      files_removed: 0,
      directories_removed: 0,
      kept: [],
      failures: [],
    }))
    const service = createLocalSettingsService({ invoke })

    await service.logCleanup('agent')

    expect(invoke).toHaveBeenCalledWith('local_log_cleanup', { source: 'agent' })
  })

  /**
   * The migration commands take a list of **names**, and nothing else.
   *
   * That is the whole boundary of this feature: no directory, no path. A service
   * that grew a `directory` argument would be handing the WebView the ability to
   * name a location, which is the one thing the Rust side refuses to accept.
   */
  it('sends the migration commands a list of names and nothing else', async () => {
    const invoke = vi.fn(async () => ({}))
    const service = createLocalSettingsService({ invoke })

    await service.migrationPlan(['a.mp4', 'b.mp4'])
    await service.moveSavedFiles(['a.mp4'])
    await service.deleteSavedFiles(['b.mp4'])

    expect(invoke).toHaveBeenCalledWith('local_save_dir_migration_plan', { names: ['a.mp4', 'b.mp4'] })
    expect(invoke).toHaveBeenCalledWith('local_move_saved_files', { names: ['a.mp4'] })
    expect(invoke).toHaveBeenCalledWith('local_delete_saved_files', { names: ['b.mp4'] })
    // The whole argument object, not just the key it was expected to carry.
    for (const call of invoke.mock.calls) {
      expect(Object.keys(call[1])).toEqual(['names'])
    }
  })

  /**
   * Pushing the location carries nothing; it is not another `set`.
   *
   * And what comes back is the **Agent's own report**, not the directory the
   * command was given — the command takes no directory at all, so a page that
   * echoed one would be reporting its own input as the result. The three fields
   * are the Agent's answer, normalized.
   */
  it('pushes the stored location with no arguments and reads the Agent back', async () => {
    const invoke = vi.fn(async () => ({
      save_dir: '/saved/files',
      writable: true,
      free_bytes: 4096,
    }))
    const service = createLocalSettingsService({ invoke })

    await expect(service.pushSaveDir()).resolves.toEqual({
      saveDir: '/saved/files',
      writable: true,
      freeBytes: 4096,
    })
    // The whole call, so a second argument (a path the page would have had to know)
    // cannot appear without this failing.
    expect(invoke.mock.calls[0]).toEqual(['local_push_save_directory'])
  })

  /**
   * An Agent that has not stored it, or cannot write there, is not a success.
   *
   * `writable` is `true` only when the Agent said so — a missing field must not
   * read as 「可写」, which is the same rule as `null` never becoming `0`.
   */
  it('does not read an absent writable flag as a writable directory', async () => {
    const service = createLocalSettingsService({
      invoke: vi.fn(async () => ({ save_dir: null })),
    })

    await expect(service.pushSaveDir()).resolves.toEqual({
      saveDir: null,
      writable: false,
      freeBytes: 0,
    })
  })

  /**
   * Naming a search directory carries nothing, and **cancelling is not an
   * answer**.
   *
   * The dialog is opened on the Rust side, so this command takes no argument at
   * all — the page never learns the directory by sending it, only by being told
   * what was picked. And a cancelled dialog comes back as `null`: a service that
   * normalized it into `{picked: '', added: false, …}` would hand the page an
   * answer to render, and the page would say 「已经在查找范围里了」 about a directory
   * nobody named. Both halves are the whole call, so an argument appearing here
   * fails this test.
   */
  it('opens the search-directory picker with no arguments and reads the answer back', async () => {
    const invoke = vi.fn(async () => ({
      picked: '/old/files',
      added: true,
      dropped: '/gone',
      searched: 3,
    }))
    const service = createLocalSettingsService({ invoke })

    await expect(service.pickSearchDirectory()).resolves.toEqual({
      picked: '/old/files',
      added: true,
      dropped: '/gone',
      searched: 3,
    })
    expect(invoke.mock.calls[0]).toEqual(['local_pick_search_directory'])
  })

  it('answers a cancelled picker with nothing rather than with an empty answer', async () => {
    for (const cancelled of [null, undefined]) {
      const service = createLocalSettingsService({
        invoke: vi.fn(async () => cancelled),
      })

      await expect(service.pickSearchDirectory()).resolves.toBeNull()
    }
  })
})

/**
 * The answers, normalized.
 *
 * The Rust DTOs are snake_case throughout and the Vue layer reads camelCase.
 * Every conversion is asserted on a value that would be wrong if the field were
 * merely copied — a `null` that must stay `null`, a missing key that must become
 * a default rather than `undefined`.
 */
describe('local settings normalization', () => {
  it('keeps an unchosen save directory as null rather than an empty string', () => {
    expect(normalizeSettingsView({ save_dir: null, file: '/tmp/s.toml' })).toEqual({
      saveDir: null,
      file: '/tmp/s.toml',
    })
    expect(normalizeSettingsView({ save_dir: '/x', file: '/tmp/s.toml' }).saveDir).toBe('/x')
    // A response that was not the expected shape must not become 「chosen」.
    expect(normalizeSettingsView(undefined).saveDir).toBeNull()
    expect(normalizeSettingsView({}).file).toBe('')
  })

  it('carries the tail counts and the truncation flag across unchanged', () => {
    const tail = normalizeLogTail({
      source: 'agent',
      name: 'agent.log',
      path: '/logs/agent.log',
      min_level: 'error',
      lines_read: 900,
      lines_shown: 12,
      truncated: true,
      lines: [{ level: null, continues: true, text: 'and so on' }],
    })

    expect(tail).toEqual({
      source: 'agent',
      name: 'agent.log',
      path: '/logs/agent.log',
      minLevel: 'error',
      linesRead: 900,
      linesShown: 12,
      truncated: true,
      lines: [{ level: null, continues: true, text: 'and so on' }],
    })
  })

  /**
   * A size that was not read stays `null`, and here that is load-bearing.
   *
   * `needed_bytes` is a sum. If an unmeasured size became `0` on the way in, the
   * space check would compare a total that quietly left a file out against the
   * free space, and could pass on a disk that cannot hold the move.
   */
  it('keeps an unreadable file size absent instead of calling it zero', () => {
    const plan = normalizeMigrationPlan({
      to: '/new',
      free_bytes: 100,
      needed_bytes: 50,
      movable: [
        { name: 'a.mp4', directory: '/old', bytes: 50 },
        { name: 'b.mp4', directory: '/old', bytes: null },
        { name: 'c.mp4', directory: '/old' },
      ],
      already_there: [],
      missing: [],
      unreadable: [],
    })

    expect(plan.movable.map((file) => file.bytes)).toEqual([50, null, null])
    expect(plan.freeBytes).toBe(100)
    expect(plan.neededBytes).toBe(50)
  })

  it('carries the plan under the page spellings', () => {
    const plan = normalizeMigrationPlan({
      to: '/new',
      free_bytes: 1,
      needed_bytes: 2,
      movable: [{ name: 'a.mp4', directory: '/old', bytes: 2 }],
      already_there: [{ name: 'b.mp4', directory: '/new', bytes: 3 }],
      missing: [{ name: 'c.mp4' }],
      unreadable: [{ directory: '/gone', reason: 'Permission denied (os error 13)' }],
    })

    expect(Object.keys(plan).sort()).toEqual([
      'alreadyThere', 'freeBytes', 'missing', 'movable', 'neededBytes', 'to', 'unreadable',
    ])
    expect(plan.alreadyThere[0].name).toBe('b.mp4')
    expect(plan.unreadable[0]).toEqual({ directory: '/gone', reason: 'Permission denied (os error 13)' })
    // A missing file has no directory and no size, and neither is invented.
    expect(plan.missing[0]).toEqual({ name: 'c.mp4', directory: '', bytes: null })
  })

  /**
   * A failure names the file, not a path.
   *
   * `normalizeCleanupReport` carries `path` because a cleanup walks one known
   * directory. A migration does not: the file may have come from any of the
   * known locations, so a path would be a second, ambiguous answer to 「which
   * file」 — and the person picked it by name.
   */
  it('identifies a failed file by name, and does not invent a path', () => {
    const report = normalizeMigrationReport({
      directory: '/new',
      moved: ['a.mp4'],
      deleted: [],
      missing: ['z.mp4'],
      kept: [{ name: 'b.mp4', reason: 'target_exists' }],
      failures: [{ name: 'c.mp4', reason: 'EXDEV and the copy failed' }],
    })

    expect(report.moved).toEqual(['a.mp4'])
    expect(report.missing).toEqual(['z.mp4'])
    expect(report.kept[0]).toEqual({ name: 'b.mp4', reason: 'target_exists' })
    expect(report.failures[0]).toEqual({ name: 'c.mp4', reason: 'EXDEV and the copy failed' })
    expect(report.failures[0]).not.toHaveProperty('path')
    // An answer that never arrived must not read as 「nothing happened」 either —
    // every list is present and empty.
    expect(normalizeMigrationReport(null)).toEqual({
      directory: '', moved: [], deleted: [], missing: [], kept: [], failures: [],
    })
  })

  /**
   * The three facts the search-directory answer carries, and the one it must not
   * invent.
   *
   * `dropped` is `null` when nothing was evicted — not `''`, which is a path and
   * would render as a directory with no name; and `added` is `true` only when the
   * Rust side said so, because 「已经在里面了」 read as 「加进去了」 is a person
   * looking for a change that did not happen.
   */
  it('keeps the search-directory answer in the page spellings, and null as null', () => {
    expect(
      normalizeSearchDirectoryView({
        picked: '/old/files',
        added: true,
        dropped: null,
        searched: 2,
      })
    ).toEqual({ picked: '/old/files', added: true, dropped: null, searched: 2 })

    expect(normalizeSearchDirectoryView({ picked: '/old/files' })).toEqual({
      picked: '/old/files',
      added: false,
      dropped: null,
      searched: 0,
    })
    // An answer that never arrived must not read as 「加进去了」.
    expect(normalizeSearchDirectoryView(undefined).added).toBe(false)
    expect(normalizeSearchDirectoryView(undefined).picked).toBe('')
  })

  it('does not invent a level for a line that has none', () => {
    const tail = normalizeLogTail({
      source: 'desktop',
      name: 'desktop.log',
      lines: [{ level: null, continues: false, text: 'before the first record' }],
    })

    // `null`, not `'info'`: naming a level here would let the page render a
    // classification the reader never made.
    expect(tail.lines[0].level).toBeNull()
    expect(tail.minLevel).toBeNull()
  })
})

/**
 * The dev fallback.
 *
 * The mock stands in for `invoke`, not for the service, so it answers in the
 * Rust side's spelling and goes through the same normalizers the bridge's
 * answers do. That is what these assert: a preview page and a packaged page see
 * one shape. The first draft had the mock *be* a second service returning
 * camelCase — which would have shipped a preview that looked right and an app
 * whose line counts were `undefined`.
 */
describe('local settings mock bridge', () => {
  it('is reachable when no invoke function was supplied', async () => {
    const service = createLocalSettingsService()

    await expect(service.getSettings()).resolves.toBeTypeOf('object')
  })

  it('answers in the Rust side vocabulary, not the page vocabulary', async () => {
    const mockInvoke = createMockInvoke()

    const settings = await mockInvoke(LOCAL_SETTINGS_COMMANDS.getSettings)
    const listing = await mockInvoke(LOCAL_SETTINGS_COMMANDS.logFiles)
    const tail = await mockInvoke(LOCAL_SETTINGS_COMMANDS.logTail, {
      source: 'desktop',
      name: 'desktop.log',
    })

    expect(Object.keys(settings).sort()).toEqual(['file', 'save_dir'])
    expect(Object.keys(tail)).toContain('lines_shown')
    expect(Object.keys(tail)).toContain('min_level')
    expect(tail).not.toHaveProperty('linesShown')
    expect(Object.keys(listing.trees[0].files[0])).toContain('modified_seconds')
    expect(listing.trees[0].files[0]).not.toHaveProperty('modifiedSeconds')
  })

  /**
   * And the normalizers are on that path, so what the page gets is camelCase
   * with the absent values still absent.
   */
  it('normalizes its own answers through the same service path', async () => {
    const service = createLocalSettingsService()

    const tail = await service.logTail({ source: 'desktop', name: 'desktop.log' })

    expect(tail.minLevel).toBeNull()
    expect(tail.linesRead).toBeGreaterThan(0)
    expect(tail.linesShown).toBe(tail.linesRead)
    expect(Number.isFinite(tail.linesShown)).toBe(true)
  })

  it('marks every answer as a mock', async () => {
    const service = createLocalSettingsService()

    const settings = await service.getSettings()
    const listing = await service.logFiles()

    expect(settings.file).toMatch(/^\/mock\//)
    for (const tree of listing.trees) {
      expect(tree.directory).toMatch(/^\/mock\//)
    }
  })

  it('round-trips a chosen directory and a cleared one', async () => {
    const service = createLocalSettingsService()

    expect((await service.getSettings()).saveDir).toBeNull()
    expect((await service.setSaveDir('/Users/mock/Movies')).saveDir).toBe('/Users/mock/Movies')
    expect((await service.setSaveDir(null)).saveDir).toBeNull()
  })

  /**
   * The stand-in's picker answers with a **directory**, and its second answer is
   * a different one.
   *
   * A mock that always cancelled would leave the page's success and no-change
   * branches undrawable in a preview, which is the one thing a preview exists to
   * check; one that always added would make 「已经在查找范围里了」 unreachable. So
   * both answers are asserted, from one service, plus the count that has to move
   * with the chosen directory — the Rust side counts the union of the choice and
   * the history, and a mock counting only the history would print one location
   * too few.
   */
  it('names a search directory once, then reports the second attempt as no change', async () => {
    const service = createLocalSettingsService()

    const first = await service.pickSearchDirectory()
    const second = await service.pickSearchDirectory()

    expect(first.picked).toMatch(/^\/mock\//)
    expect(first.added).toBe(true)
    expect(first.dropped).toBeNull()
    expect(first.searched).toBe(1)
    expect(second).toEqual({ ...first, added: false })

    await service.setSaveDir('/Users/mock/Movies')
    expect((await service.pickSearchDirectory()).searched).toBe(2)
  })

  /**
   * The mock filters like the command does — at the level *or louder* — and
   * reports the filter it applied as `null` when it applied none.
   *
   * The expected levels are spelled out rather than bounded by a membership
   * check: 「every line is warn or error」 is satisfied by `=== 'warn'` too, and
   * the difference between a floor and an exact match is exactly the record
   * above the floor. That is the assertion, so the mock has an `error` line.
   */
  it('filters a mock tail at the chosen level or louder', async () => {
    const service = createLocalSettingsService()

    const unfiltered = await service.logTail({ source: 'desktop', name: 'desktop.log' })
    const filtered = await service.logTail({
      source: 'desktop',
      name: 'desktop.log',
      minLevel: 'warn',
    })

    expect(unfiltered.minLevel).toBeNull()
    expect(unfiltered.lines.map((entry) => entry.level)).toEqual(['info', 'warn', 'error'])
    expect(filtered.minLevel).toBe('warn')
    expect(filtered.lines.map((entry) => entry.level)).toEqual(['warn', 'error'])
    expect(filtered.linesShown).toBe(2)
    // The read count is the whole file's, before the filter: it is what lets the
    // page say how much the filter hid.
    expect(filtered.linesRead).toBe(3)
  })

  /**
   * An unknown command rejects, as a real `invoke` does. A mock that answered
   * `undefined` would let a page with a misspelled command name pass in preview
   * and fail in the app.
   */
  it('refuses a command it does not implement', async () => {
    const mockInvoke = createMockInvoke()

    await expect(mockInvoke('local_nonexistent')).rejects.toThrow(/unknown command/)
  })

  it('returns the directory it opened, as the real command does', async () => {
    const service = createLocalSettingsService()

    await expect(service.openPlace('data')).resolves.toBe('/mock/opened/data')
  })
})
