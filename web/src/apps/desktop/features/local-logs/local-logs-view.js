// What the 本地日志 viewer derives from the listing and the tail it was given.
//
// Pure and separable, like `local-agent-status.js` and for the same reason: the
// rules here — which filter a button means, what a truncation is a statement
// about, what to say when a filter is hiding lines — are the part worth
// asserting, and they cannot be asserted through a mounted component.
//
// ## Three things the page must not claim
//
// 1. **That a filter is a match.** `filter_min` on the Rust side keeps entries at
//    the chosen level *or louder*, so `warn` means 「警告及以上」, not 「只有警告」.
//    The options say so.
// 2. **That a filter hid everything.** A line before the first record that
//    carried a level has no level, and those lines are kept whatever the filter
//    says — a file whose first lines are unclassifiable is not empty under a
//    filter. `linesRead` and `linesShown` come from the answer, so the two counts
//    the page shows are the ones it was actually given rather than a recount
//    here.
// 3. **That the tail shown is the whole file.** `truncated` means the byte
//    ceiling ended the read, so what is on screen is the *end* of the file; the
//    notice says so rather than leaving a person to assume the file is short.

/** The levels a person can filter by, in the order the page lists them. */
export const LOG_LEVELS = Object.freeze([
  { value: "trace", label: "全部（含跟踪）" },
  { value: "debug", label: "调试及以上" },
  { value: "info", label: "信息及以上" },
  { value: "warn", label: "警告及以上" },
  { value: "error", label: "仅错误" },
]);

/** No filter: everything that was read is shown. */
export const NO_LEVEL = null;

/** How the page spells a level that the tail did not classify. */
export const UNCLASSIFIED_LABEL = "未分级";

const LEVEL_LABELS = Object.freeze({
  trace: "跟踪",
  debug: "调试",
  info: "信息",
  warn: "警告",
  error: "错误",
});

/**
 * The filter option a value means, or `null` when it is not one of them.
 *
 * `null` covers two different things on purpose — the wire's 「no filter」 and a
 * spelling the page does not know — because both have the same safe rendering:
 * show what came back and do not claim a filter is applied. The command's own
 * `min_level` is what the notice quotes, never this function's guess.
 */
export function levelOption(value) {
  return LOG_LEVELS.find((option) => option.value === value) ?? null;
}

/** What a level looks like on one line. */
export function levelLabel(level) {
  if (level === null || level === undefined || level === "") {
    return UNCLASSIFIED_LABEL;
  }
  return LEVEL_LABELS[level] ?? level;
}

/** The theme a line's level is rendered with. */
export function levelTheme(level) {
  switch (level) {
    case "error":
      return "danger";
    case "warn":
      return "warning";
    case "info":
      return "default";
    case "debug":
    case "trace":
      return "default";
    default:
      // An unclassified line is not an error and must not be coloured like one:
      // it is a line this reader could not attribute, which is a fact about the
      // reader and not about the log.
      return "default";
  }
}

/**
 * The file list, flattened across both trees.
 *
 * The order within a tree is the listing's own — live file, then archives newest
 * first — because the reader already sorted it, and re-sorting here would be a
 * second opinion that can disagree with the first.
 */
export function createLocalLogsPage({
  listing = null,
  source = null,
  name = null,
  tail = null,
  level = NO_LEVEL,
} = {}) {
  const trees = (listing?.trees ?? []).map((tree) => ({
    source: tree.source,
    directory: tree.directory,
    files: tree.files.map((file) => ({
      ...file,
      selected: tree.source === source && file.name === name,
    })),
  }));

  const chosenTree = trees.find((tree) => tree.source === source) ?? null;
  const chosenFile = chosenTree?.files.find((file) => file.name === name) ?? null;

  const appliedLevel = tail?.minLevel ?? null;
  const appliedOption = levelOption(appliedLevel);
  const hidden = tail ? Math.max(0, tail.linesRead - tail.linesShown) : 0;

  return {
    trees,
    hasTrees: trees.length > 0,
    selectedSource: source ?? "",
    selectedName: name ?? "",
    // The directory the file came from, so a person can tell two same-named
    // files from two trees apart without opening anything.
    selectedDirectory: chosenTree?.directory ?? "",
    selectedPath: tail?.path ?? "",
    canView: Boolean(source && name),
    filterValue: level ?? NO_LEVEL,
    filterOptions: LOG_LEVELS,
    // The filter the answer was actually under, quoted from the answer rather
    // than from the request: a filter that was asked for and not applied is
    // worse than no filter, and only the answer knows.
    appliedFilterText: appliedOption
      ? appliedOption.label
      : appliedLevel
        ? appliedLevel
        : "未筛选",
    lines: (tail?.lines ?? []).map((entry, index) => ({
      key: `${index}`,
      // An indented continuation says 「this is the same record」, which is the
      // only thing the flag means.
      indent: entry.continues,
      level: entry.level,
      levelText: entry.continues ? "" : levelLabel(entry.level),
      levelTheme: levelTheme(entry.level),
      text: entry.text,
    })),
    hasLines: (tail?.lines ?? []).length > 0,
    // Both counts, both from the answer. A page that recomputed them would be
    // able to disagree with the command about how much it hid.
    countsText: tail ? `读取 ${tail.linesRead} 行，显示 ${tail.linesShown} 行` : "",
    hiddenCount: hidden,
    // Said only when a filter is genuinely hiding something, and it names the
    // two facts that make it true: the count, and that unclassified lines are
    // not among them.
    hiddenNotice:
      hidden > 0
        ? `筛选隐藏了 ${hidden} 行；未分级的行不受筛选影响，始终显示。`
        : "",
    // A window, not the whole file. Says which end.
    truncatedNotice: tail?.truncated
      ? "文件较长，这里显示的是尾部；更早的内容没有读出。"
      : "",
  };
}
