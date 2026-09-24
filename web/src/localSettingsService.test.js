import { describe, expect, it, vi } from 'vitest'

import {
  createLocalSettingsService,
  createMockInvoke,
  DEFAULT_TAIL_LINES,
  LOG_SOURCES,
  LOCAL_SETTINGS_COMMANDS,
  normalizeLogTail,
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
      storageUsage: 'local_storage_usage',
      logFiles: 'local_log_files',
      logTail: 'local_log_tail',
      cacheCleanup: 'local_cache_cleanup',
      logCleanup: 'local_log_cleanup',
      diagnosticExport: 'local_diagnostic_export',
      openPlace: 'local_open_place',
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
