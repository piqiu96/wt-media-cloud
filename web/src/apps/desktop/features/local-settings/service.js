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
  storageUsage: "local_storage_usage",
  logFiles: "local_log_files",
  logTail: "local_log_tail",
  cacheCleanup: "local_cache_cleanup",
  logCleanup: "local_log_cleanup",
  diagnosticExport: "local_diagnostic_export",
  openPlace: "local_open_place",
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
