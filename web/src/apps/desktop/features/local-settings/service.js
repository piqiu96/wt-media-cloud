// The 本机设置 page's window onto the machine the app is running on.
//
// The same shape as `local-agent/service.js`, and for the same reason: the page
// never talks to a port, a path or a filesystem — it names a **command** and the
// Rust bridge answers. Every command here is one CHG-058 added for this page
// (`local_settings_*`), or one of T-05/T-06/T-07's (`local_storage_*`,
// `local_log_*`, `local_*_cleanup`, `local_diagnostic_export`, `local_open_place`).
//
// ## Argument names are camelCase on this side, snake_case in Rust
//
// Tauri v2 renames command arguments on the way in: the Rust parameter `save_dir`
// is passed by the WebView as `saveDir`, `min_level` as `minLevel`. The names
// below are therefore the *wire* spelling, not a choice — a service that sent
// `save_dir` would reach a command whose `save_dir` is `None`, and the page would
// silently store nothing. `localSettingsService.test.js` pins the exact objects.
//
// ## A place is a label, not a directory
//
// `openPlace(place)` sends one of the three words the Rust side knows
// (`"desktop"`, `"agent"`, `"data"`) and never a path. The two log labels are
// literally `Source::label()`'s, so the viewer hands the same string to
// `logTail` and to this method and both mean the same tree. `local-settings-view.js`
// owns the list; this module does not re-spell it.

export const LOCAL_SETTINGS_COMMANDS = Object.freeze({
  getSettings: "local_settings_get",
  setSettings: "local_settings_set",
  pushSaveDirectory: "local_push_save_directory",
  storageUsage: "local_storage_usage",
  logFiles: "local_log_files",
  logTail: "local_log_tail",
  cacheCleanup: "local_cache_cleanup",
  logCleanup: "local_log_cleanup",
  diagnosticExport: "local_diagnostic_export",
  openPlace: "local_open_place",
  migrationPlan: "local_save_dir_migration_plan",
  moveSavedFiles: "local_move_saved_files",
  deleteSavedFiles: "local_delete_saved_files",
  addSearchDirectory: "local_pick_search_directory",
});

/** The two components whose logs this page can show, in the order it shows them. */
export const LOG_SOURCES = Object.freeze(["desktop", "agent"]);

/** How many lines a view asks for unless the person picks otherwise. */
export const DEFAULT_TAIL_LINES = 500;

function clone(value) {
  return JSON.parse(JSON.stringify(value));
}

function asObject(value) {
  return value && typeof value === "object" ? value : {};
}

function asArray(value) {
  return Array.isArray(value) ? value : [];
}

function asCount(value) {
  const number = Number(value ?? 0);
  return Number.isFinite(number) ? number : 0;
}

/**
 * A count that stays `null` when it was not measured.
 *
 * `asCount` above is right for a count that is a count; a *size* is different —
 * a missing one must not become `0`, because `0` is a measurement and the two are
 * summed into 「需要多少空间」 (see `formatBytes` in `local-settings-view.js`).
 */
function asNullableCount(value) {
  if (value === null || value === undefined || value === "") {
    return null;
  }
  const number = Number(value);
  return Number.isFinite(number) ? number : null;
}

/** The operator's settings as the page reads them. */
export function normalizeSettingsView(value) {
  const source = asObject(value);
  return {
    // `null` is 「未设置」 and is not the same as `""`. Kept as `null` through the
    // normalizer so the page cannot render an empty box that looks filled in.
    saveDir: source.save_dir ?? null,
    file: String(source.file ?? ""),
  };
}

/**
 * What the Agent says about the save location, **read back from the Agent**.
 *
 * `saveDir` is the Agent's own answer rather than an echo of what Desktop just
 * stored, and the two can disagree: the Agent may have been down when the choice
 * was made. What a download depends on is 「Agent 现在会往哪里写」, so that is
 * what this carries — and `writable` says whether a task could write there right
 * now, which is a different question from whether the directory exists.
 */
export function normalizeSaveDirectoryFacts(value) {
  const source = asObject(value);
  return {
    saveDir: source.save_dir ?? null,
    writable: source.writable === true,
    freeBytes: asCount(source.free_bytes),
  };
}

/** One log file as the listing describes it. */
export function normalizeLogFile(value) {
  const source = asObject(value);
  return {
    name: String(source.name ?? ""),
    kind: String(source.kind ?? "other"),
    bytes: asCount(source.bytes),
    // Seconds, or `null` when the filesystem would not say. The page formats it
    // in this machine's timezone (the user's ruling); the Rust side deliberately
    // does not.
    modifiedSeconds: source.modified_seconds ?? null,
  };
}

/** One component's log tree. */
export function normalizeLogTree(value) {
  const source = asObject(value);
  return {
    source: String(source.source ?? ""),
    directory: String(source.directory ?? ""),
    files: asArray(source.files).map(normalizeLogFile),
  };
}

/** One line of a log, with the level that applies to it. */
export function normalizeLogEntry(value) {
  const source = asObject(value);
  return {
    level: source.level ?? null,
    continues: source.continues === true,
    text: String(source.text ?? ""),
  };
}

/** The end of one log file, as asked for. */
export function normalizeLogTail(value) {
  const source = asObject(value);
  return {
    source: String(source.source ?? ""),
    name: String(source.name ?? ""),
    path: String(source.path ?? ""),
    // A filter that was asked for and is not applied is worse than no filter, so
    // the answer's own spelling — not the request's — is what the page shows.
    minLevel: source.min_level ?? null,
    linesRead: asCount(source.lines_read),
    linesShown: asCount(source.lines_shown),
    truncated: source.truncated === true,
    lines: asArray(source.lines).map(normalizeLogEntry),
  };
}

/** Free space, the cache, and each log tree. */
export function normalizeStorageUsage(value) {
  const source = asObject(value);
  return {
    availableBytes: asCount(source.available_bytes),
    cacheBytes: asCount(source.cache_bytes),
    logs: asArray(source.logs).map((entry) => {
      const tree = asObject(entry);
      return {
        source: String(tree.source ?? ""),
        directory: String(tree.directory ?? ""),
        bytes: asCount(tree.bytes),
        files: asCount(tree.files),
      };
    }),
  };
}

/** One cleanup's report. */
export function normalizeCleanupReport(value) {
  const source = asObject(value);
  return {
    label: String(source.label ?? ""),
    directory: String(source.directory ?? ""),
    freedBytes: asCount(source.freed_bytes),
    filesRemoved: asCount(source.files_removed),
    directoriesRemoved: asCount(source.directories_removed),
    kept: asArray(source.kept).map((entry) => {
      const kept = asObject(entry);
      return { name: String(kept.name ?? ""), reason: String(kept.reason ?? "") };
    }),
    failures: asArray(source.failures).map((entry) => {
      const failure = asObject(entry);
      return { path: String(failure.path ?? ""), reason: String(failure.reason ?? "") };
    }),
  };
}

/** One diagnostic export's report. */
export function normalizeDiagnosticReport(value) {
  const source = asObject(value);
  return {
    path: String(source.path ?? ""),
    checksumPath: String(source.checksum_path ?? ""),
    bytes: asCount(source.bytes),
    sha256: String(source.sha256 ?? ""),
    createdAt: String(source.created_at ?? ""),
    entries: asArray(source.entries).map((entry) => {
      const item = asObject(entry);
      return {
        name: String(item.name ?? ""),
        bytes: asCount(item.bytes),
        truncated: item.truncated === true,
      };
    }),
    omitted: asArray(source.omitted).map((entry) => {
      const item = asObject(entry);
      return { name: String(item.name ?? ""), reason: String(item.reason ?? "") };
    }),
    agentState: String(source.agent_state ?? ""),
    failedRecords: asCount(source.failed_records),
  };
}

/**
 * One saved file as the migration plan counts it.
 *
 * `bytes` is `null` when the size could not be read — and it is *not* turned into
 * `0` here either (AC-06): the plan's `needed_bytes` is a sum, and a missing size
 * silently worth nothing is how a space check passes on a disk that is full.
 */
export function normalizeSavedFile(value) {
  const source = asObject(value);
  return {
    name: String(source.name ?? ""),
    directory: String(source.directory ?? ""),
    bytes: asNullableCount(source.bytes),
  };
}

/** A known save directory that could not be listed, and why. */
export function normalizeUnreadableDir(value) {
  const source = asObject(value);
  return {
    directory: String(source.directory ?? ""),
    reason: String(source.reason ?? ""),
  };
}

/**
 * What moving the saved files would do, before anything is touched.
 *
 * `movable` / `already_there` / `missing` carry a record per name rather than a
 * bare name because the dialog has to show a size beside each one (「逐个列出名字
 * 与体积」); a second command for that would re-run the same directory walk and
 * could answer about a different moment.
 */
export function normalizeMigrationPlan(value) {
  const source = asObject(value);
  return {
    to: String(source.to ?? ""),
    freeBytes: asCount(source.free_bytes),
    neededBytes: asCount(source.needed_bytes),
    movable: asArray(source.movable).map(normalizeSavedFile),
    alreadyThere: asArray(source.already_there).map(normalizeSavedFile),
    missing: asArray(source.missing).map(normalizeSavedFile),
    unreadable: asArray(source.unreadable).map(normalizeUnreadableDir),
  };
}

/** What one move or delete did. Both commands answer in this one shape. */
export function normalizeMigrationReport(value) {
  const source = asObject(value);
  return {
    directory: String(source.directory ?? ""),
    moved: asArray(source.moved).map(String),
    deleted: asArray(source.deleted).map(String),
    missing: asArray(source.missing).map(String),
    kept: asArray(source.kept).map((entry) => {
      const kept = asObject(entry);
      return { name: String(kept.name ?? ""), reason: String(kept.reason ?? "") };
    }),
    failures: asArray(source.failures).map((entry) => {
      const failure = asObject(entry);
      // The name, not a path: the whole point of this command family is that no
      // path crosses it, and a failure names the file the person picked.
      return { name: String(failure.name ?? ""), reason: String(failure.reason ?? "") };
    }),
  };
}

/**
 * What naming a directory for searching did.
 *
 * `dropped` stays `null` rather than becoming an empty string when nothing was
 * evicted: 「没有挤掉任何一个」 and 「答案里没有这一项」 are different, and only the
 * first is a fact the Rust side knows. `searched` is **that** process's count —
 * the page is never sent the list it would have to count itself, which is what
 * keeps 「共查找 N 个位置」 from being a number this side invented.
 */
export function normalizeSearchDirectoryView(value) {
  const source = asObject(value);
  return {
    picked: String(source.picked ?? ""),
    added: source.added === true,
    dropped: source.dropped ?? null,
    searched: asCount(source.searched),
  };
}

// Real Tauri invoke-based service.
// Falls back to mock when Tauri is unavailable (Vite dev mode).
//
// **The mock is an `invoke`, not a service.** It answers in the *wire* spelling
// (`min_level`, `lines_shown`) and the normalizers below are applied to it
// exactly as they are to the Rust bridge's answers — so there is one definition
// of the shape a page sees, and a browser preview cannot render a page that the
// packaged app would leave blank. The first draft built the mock as a second
// service returning camelCase, which is how that bug would have shipped: the
// preview looked right and the app showed `undefined` counts.
export function createLocalSettingsService({ invoke } = {}) {
  const call = typeof invoke === "function" ? invoke : createMockInvoke();

  return {
    async getSettings() {
      return normalizeSettingsView(await call(LOCAL_SETTINGS_COMMANDS.getSettings));
    },
    /**
     * Store a chosen save directory, or clear it with `null`.
     *
     * `null` and `""` are different requests: the first means 「没有选择」, the
     * second means a path that is empty — which the Rust side refuses. The
     * conversion is explicit here rather than left to `??` so the two cannot be
     * confused by a caller passing an empty string from an input.
     */
    async setSaveDir(path) {
      return normalizeSettingsView(
        await call(LOCAL_SETTINGS_COMMANDS.setSettings, {
          saveDir: path === null || path === undefined ? null : String(path),
        })
      );
    },
    /**
     * Tell the Agent where downloads should go from now on.
     *
     * Storing the choice (`setSaveDir`) only writes this app's own settings file;
     * the machine that actually writes the files learns the directory from this
     * push, and it is a **separate** command because it can fail on its own (the
     * Agent may not be running). The page treats that failure as non-fatal: the
     * next agent start pushes the same value again.
     */
    async pushSaveDir() {
      return normalizeSaveDirectoryFacts(await call(LOCAL_SETTINGS_COMMANDS.pushSaveDirectory));
    },
    /**
     * Name a directory to search **without** moving where new downloads go.
     *
     * The picker is opened on the Rust side, so this is the only call in the
     * service that can answer with **nothing**: a cancelled dialog arrives as
     * `null`, and that is not an answer. It is kept distinct from an answer all
     * the way up — rendering 「已加入」 for it would report a change that did not
     * happen, and rendering 「已经在里面了」 would invent a directory nobody named.
     */
    async pickSearchDirectory() {
      const answer = await call(LOCAL_SETTINGS_COMMANDS.addSearchDirectory);
      return answer === null || answer === undefined
        ? null
        : normalizeSearchDirectoryView(answer);
    },
    /** What moving the named files to the chosen directory would involve. */
    async migrationPlan(names) {
      return normalizeMigrationPlan(
        await call(LOCAL_SETTINGS_COMMANDS.migrationPlan, { names })
      );
    },
    /** Move the named files into the chosen directory. Nothing else is touched. */
    async moveSavedFiles(names) {
      return normalizeMigrationReport(
        await call(LOCAL_SETTINGS_COMMANDS.moveSavedFiles, { names })
      );
    },
    /** Delete the named files. Regular files only, and only the named ones. */
    async deleteSavedFiles(names) {
      return normalizeMigrationReport(
        await call(LOCAL_SETTINGS_COMMANDS.deleteSavedFiles, { names })
      );
    },
    async storageUsage() {
      return normalizeStorageUsage(await call(LOCAL_SETTINGS_COMMANDS.storageUsage));
    },
    async logFiles() {
      const answer = asObject(await call(LOCAL_SETTINGS_COMMANDS.logFiles));
      return { trees: asArray(answer.trees).map(normalizeLogTree) };
    },
    async logTail({ source, name, lines = DEFAULT_TAIL_LINES, minLevel = null }) {
      return normalizeLogTail(
        await call(LOCAL_SETTINGS_COMMANDS.logTail, {
          source,
          name,
          lines,
          minLevel,
        })
      );
    },
    async cacheCleanup() {
      return normalizeCleanupReport(await call(LOCAL_SETTINGS_COMMANDS.cacheCleanup));
    },
    async logCleanup(source) {
      return normalizeCleanupReport(
        await call(LOCAL_SETTINGS_COMMANDS.logCleanup, { source })
      );
    },
    async diagnosticExport() {
      return normalizeDiagnosticReport(await call(LOCAL_SETTINGS_COMMANDS.diagnosticExport));
    },
    /** Returns the directory that was opened, as a string. */
    async openPlace(place) {
      return String(await call(LOCAL_SETTINGS_COMMANDS.openPlace, { place }));
    },
  };
}

/**
 * How many directories the search space holds, mirroring the Rust side's
 * `MAX_KNOWN_SAVE_DIRS`. The mock keeps the same bound so that a preview can
 * reach the 「挤掉了一个」 answer; the number itself is the Rust side's, and this
 * is a copy of it rather than a second definition of the rule.
 */
const MAX_SEARCH_DIRS = 8;

/**
 * The dev-mode stand-in for the Rust bridge.
 *
 * Every answer is self-evidently a mock — the directories and file names are
 * nobody's, and the settings file is under `/mock` — because a mock that
 * pretended otherwise would let a browser preview look correct while its real
 * bindings were never exercised.
 *
 * It answers in the **Rust side's spelling**, deliberately: snake_case keys and
 * camelCase arguments, so it is a stand-in for `invoke` rather than for the
 * service, and the normalizers are on the path in both modes.
 */
export function createMockInvoke() {
  let saveDir = null;
  const file = "/mock/WTMedia/Desktop/settings.toml";
  // The pretend search history, newest first and bounded like the Rust side's
  // (`MAX_KNOWN_SAVE_DIRS`), so a preview draws the eviction branch the app would
  // rather than growing a list the app caps. It starts empty so that all three
  // answers the page has to render — added / already there / evicted — are
  // reachable by pressing the button more than once.
  let searches = [];
  // The pretend download history, and one older save directory holding two of
  // them. `old: false` is a file already in the chosen directory; `bytes: null`
  // is one whose size could not be read.
  const oldDir = "/mock/Movies/WTMedia-old";
  const saved = [
    { name: "演示素材-42.mp4", old: true, bytes: 230 * 1024 * 1024 },
    { name: "演示素材-43.mp4", old: true, bytes: null },
    { name: "成片-7.mp4", old: false, bytes: 48 * 1024 * 1024 },
  ];
  // One known directory this stand-in cannot list. It is here on purpose: with
  // every directory readable the page never draws the branch that says 「有位置
  // 没查成」, and that branch is the difference between 「已不存在」 and 「未找到」.
  const unreadable = [{ directory: "/mock/Movies/WTMedia-perm", reason: "Permission denied (os error 13)" }];
  const trees = [
    {
      source: "desktop",
      directory: "/mock/Logs/WTMedia/Desktop",
      files: [
        { name: "desktop.log", kind: "live", bytes: 512, modified_seconds: null },
        { name: "desktop.log.2026-09-24-23", kind: "archive", bytes: 4096, modified_seconds: null },
      ],
    },
    {
      source: "agent",
      directory: "/mock/Logs/WTMedia/Agent",
      files: [{ name: "agent.log", kind: "live", bytes: 256, modified_seconds: null }],
    },
  ];
  // Three levels, one of them *above* the filter a page is most likely to try —
  // a mock whose only two records were `info` and `warn` would answer a `warn`
  // filter identically whether it filtered by floor or by exact match, and the
  // page's wording is about the floor.
  const records = [
    { level: "info", continues: false, text: "2026-09-24T23:00:00 [INFO] mock: started" },
    { level: "warn", continues: false, text: "2026-09-24T23:00:01 [WARN] mock: nothing real" },
    { level: "error", continues: false, text: "2026-09-24T23:00:02 [ERROR] mock: a failure" },
  ];
  // The mock implements the same floor semantics as `reader::filter_min`: the
  // chosen level *or louder*, and an unclassified line is kept whatever the
  // filter says. A mock that filtered differently would let the page's wording
  // be checked against a rule the real command does not have.
  const order = ["trace", "debug", "info", "warn", "error"];
  const atLeast = (entry, min) =>
    entry.level === null || order.indexOf(entry.level) >= order.indexOf(min);

  return async function mockInvoke(command, args = {}) {
    switch (command) {
      case LOCAL_SETTINGS_COMMANDS.getSettings:
        return clone({ save_dir: saveDir, file });
      case LOCAL_SETTINGS_COMMANDS.setSettings:
        saveDir = args.saveDir ?? null;
        return clone({ save_dir: saveDir, file });
      case LOCAL_SETTINGS_COMMANDS.addSearchDirectory: {
        // A folder dialog cannot be answered from a browser, so the stand-in
        // names the one directory its pretend machine used to use instead of
        // answering `null`: a mock that always cancelled would make both of the
        // page's other branches undrawable, which is the one thing a preview
        // exists to check. Its effect on *visibility* is not modelled — the plan
        // below lists that directory's files already, because on a real machine
        // it is in the history by the time this picker is needed at all.
        const picked = oldDir;
        const searched = () =>
          saveDir ? [saveDir, ...searches.filter((dir) => dir !== saveDir)] : [...searches];
        if (searched().includes(picked)) {
          return clone({ picked, added: false, dropped: null, searched: searched().length });
        }
        const grown = [picked, ...searches];
        const dropped = grown.length > MAX_SEARCH_DIRS ? grown[MAX_SEARCH_DIRS] : null;
        searches = grown.slice(0, MAX_SEARCH_DIRS);
        return clone({ picked, added: true, dropped, searched: searched().length });
      }
      case LOCAL_SETTINGS_COMMANDS.pushSaveDirectory:
        // Refuses while nothing is chosen, exactly as the Rust side does — a mock
        // that answered 「记下了」 for a push that carried nothing would let the
        // page report success for a location the Agent does not have.
        if (!saveDir) {
          throw new Error("还没有选择下载保存位置");
        }
        return clone({ save_dir: saveDir, writable: true, free_bytes: 512 * 1024 * 1024 * 1024 });
      case LOCAL_SETTINGS_COMMANDS.migrationPlan:
        return clone(mockPlan(saved, args.names, saveDir, oldDir, unreadable));
      case LOCAL_SETTINGS_COMMANDS.moveSavedFiles:
        return clone(mockMigrate(saved, args.names, saveDir, "move"));
      case LOCAL_SETTINGS_COMMANDS.deleteSavedFiles:
        return clone(mockMigrate(saved, args.names, saveDir, "delete"));
      case LOCAL_SETTINGS_COMMANDS.storageUsage:
        return clone({
          available_bytes: 512 * 1024 * 1024 * 1024,
          cache_bytes: 1024 * 1024,
          logs: trees.map((tree) => ({
            source: tree.source,
            directory: tree.directory,
            bytes: tree.files.reduce((sum, item) => sum + item.bytes, 0),
            files: tree.files.length,
          })),
        });
      case LOCAL_SETTINGS_COMMANDS.logFiles:
        return clone({ trees });
      case LOCAL_SETTINGS_COMMANDS.logTail: {
        const chosen = args.minLevel ? records.filter((entry) => atLeast(entry, args.minLevel)) : records;
        return clone({
          source: args.source,
          name: args.name,
          path: `/mock/Logs/WTMedia/${args.source}/${args.name}`,
          min_level: args.minLevel ?? null,
          lines_read: records.length,
          lines_shown: chosen.length,
          truncated: false,
          lines: chosen.slice(0, args.lines ?? DEFAULT_TAIL_LINES),
        });
      }
      case LOCAL_SETTINGS_COMMANDS.cacheCleanup:
        return clone(mockCleanup("cache", "/mock/Caches/WTMedia/Desktop"));
      case LOCAL_SETTINGS_COMMANDS.logCleanup:
        return clone(
          mockCleanup(
            args.source,
            trees.find((tree) => tree.source === args.source)?.directory ??
              `/mock/Logs/WTMedia/${args.source}`
          )
        );
      case LOCAL_SETTINGS_COMMANDS.diagnosticExport:
        return clone({
          path: "/mock/Downloads/wt-media-diagnostic-20260924-230000.tar.gz",
          checksum_path: "/mock/Downloads/wt-media-diagnostic-20260924-230000.tar.gz.sha256",
          bytes: 8192,
          sha256: "0".repeat(64),
          created_at: "2026-09-24T23:00:00",
          entries: [{ name: "logs/desktop/desktop.log", bytes: 512, truncated: true }],
          omitted: [{ name: "logs/agent/big.log", reason: "archive_full" }],
          agent_state: "answered",
          failed_records: 0,
        });
      case LOCAL_SETTINGS_COMMANDS.openPlace:
        return `/mock/opened/${args.place}`;
      default:
        // A real `invoke` rejects on an unknown command; a mock that answered
        // `undefined` would let a page pass in preview while every call failed
        // in the app.
        throw new Error(`mock invoke got an unknown command: ${command}`);
    }
  };
}

/**
 * The stand-in's migration plan, in the Rust side's spelling.
 *
 * `needed_bytes` sums only the sizes that were read; a file with an unreadable
 * size contributes nothing and the page shows it as 未知 rather than as 0 B —
 * the sum is then a lower bound, which is why the dialog says 「至少」.
 */
function mockPlan(saved, names, saveDir, oldDir, unreadable) {
  const asked = Array.isArray(names) ? names : [];
  const at = (entry) => (entry.old ? oldDir : saveDir ?? "/mock/Movies/WTMedia");
  const found = saved.filter((entry) => asked.includes(entry.name));
  const movable = found.filter((entry) => entry.old);
  return {
    to: saveDir ?? "/mock/Movies/WTMedia",
    free_bytes: 512 * 1024 * 1024 * 1024,
    needed_bytes: movable.reduce((sum, entry) => sum + (entry.bytes ?? 0), 0),
    movable: movable.map((entry) => ({ name: entry.name, directory: at(entry), bytes: entry.bytes })),
    already_there: found
      .filter((entry) => !entry.old)
      .map((entry) => ({ name: entry.name, directory: at(entry), bytes: entry.bytes })),
    // Asked about and not on this pretend disk: 「未找到」, not an error.
    missing: asked
      .filter((name) => !saved.some((entry) => entry.name === name))
      .map((name) => ({ name, directory: "", bytes: null })),
    unreadable,
  };
}

/** The stand-in's move/delete, which really does change what a later call sees. */
function mockMigrate(saved, names, saveDir, action) {
  const asked = new Set(Array.isArray(names) ? names : []);
  const moved = [];
  const deleted = [];
  const kept = [];
  for (const entry of saved) {
    if (!asked.has(entry.name)) continue;
    if (action === "delete") {
      deleted.push(entry.name);
      entry.removed = true;
      continue;
    }
    if (!entry.old) {
      // Already where it was asked to go: kept, and not counted as moved. The
      // same distinction the Rust side draws.
      kept.push({ name: entry.name, reason: "same_directory" });
      continue;
    }
    entry.old = false;
    moved.push(entry.name);
  }
  const missing = [...asked].filter((name) => !saved.some((entry) => entry.name === name));
  const survivors = saved.filter((entry) => !entry.removed);
  saved.length = 0;
  saved.push(...survivors);
  return {
    directory: saveDir ?? "/mock/Movies/WTMedia",
    moved,
    deleted,
    missing,
    kept,
    failures: [],
  };
}

function mockCleanup(label, directory) {
  return {
    label,
    directory,
    freed_bytes: 4096,
    files_removed: 1,
    directories_removed: 0,
    kept: [{ name: `${label}.live`, reason: "live" }],
    failures: [],
  };
}
